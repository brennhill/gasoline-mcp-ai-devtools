# Screenshot Archival & Query — Spec Review

**Review Date:** 2026-01-30
**Spec Version:** 1.0 (docs/screenshot-archival-and-query.md)
**Reviewer:** Principal Engineer (Claude Code)
**Status:** `NEEDS REVISION` — 3 critical issues, 5 high-priority recommendations

---

## Executive Summary

The screenshot archival and query specification presents a **well-structured feature** for design regression tracking, but introduces several **critical architectural tensions** around concurrency (SQLite single-writer), data consistency (LLM-supplied arbitrary metadata), and complexity growth (storage abstraction, query engine). The design is viable but requires explicit guardrails on metadata validation, SQLite transaction isolation, and memory pressure handling before implementation.

---

## Critical Issues (Must Fix Before Implementation)

### 1. Unbounded Screenshot Metadata Causes Memory Spike
**Section:** 2, 5
**Risk Level:** CRITICAL (DoS vector)

**Problem:**
- Spec allows LLM to include "arbitrary metadata" without size limits
- Each screenshot is 200–500 KB (image) + unbounded JSON metadata
- If LLM includes 100 KB of metadata per screenshot: 100 captures × 100 KB = 10 MB single-session memory bloat
- Combined with 5 viewports: 500 MB for 100 multi-viewport audits

**Fix:**
- Cap arbitrary metadata to **5 KB per screenshot** (firm limit)
- Server rejects screenshots with metadata > 5 KB
- Metadata field names must match `[a-z_][a-z0-9_]*` (lowercase alphanumeric + underscore)
- Document: "LLM-supplied metadata is limited to 5KB; excess truncated with warning"

**Implementation:**
```go
const MaxMetadataBytes = 5000
if len(metadataJSON) > MaxMetadataBytes {
    return fmt.Errorf("metadata exceeds %d bytes", MaxMetadataBytes)
}
```

---

### 2. Path Traversal via Component/Variant/Viewport Metadata
**Section:** 2, 4
**Risk Level:** CRITICAL (File system escape)

**Problem:**
- LLM-supplied `component`, `variant`, `viewport` fields are used in filename construction
- Example attack: `metadata: {component: "../../secrets"}` → filename `myapp-../../secrets-desktop.jpg`
- `filepath.Join()` may escape `.gasoline/screenshots/` directory

**Fix:**
- Sanitize ALL user-supplied fields that go into filenames:
  ```go
  filename = fmt.Sprintf("%s-%s-%s-%s-%sx%s-%s.jpg",
      sanitizeForFilename(sitename),
      sanitizeForFilename(component),      // ← ADD
      sanitizeForFilename(variant),        // ← ADD
      sanitizeForFilename(viewport),       // ← ADD
      width, height,
      isoTimestamp)
  ```
- Add security test: Verify that `metadata: {component: "../../secret"}` does NOT escape `.gasoline/screenshots/`

**Implementation:** Extend existing `sanitizeForFilename()` function in main.go to allow alphanumeric + underscores only.

---

### 3. File Write / SQLite Insert Race Condition Creates Orphaned Screenshots
**Section:** 3, 4
**Risk Level:** CRITICAL (Data inconsistency)

**Problem:**
- If `os.WriteFile()` succeeds but SQLite `INSERT` fails (or vice versa):
  - Image exists on disk, no DB row → orphaned screenshot
  - DB row exists, file doesn't → 404 when fetching image
  - Query returns fewer results than actual files

**Scenario:**
```go
err := os.WriteFile(filepath, imageData, 0o644)  // ✓ Succeeds
_, err = db.Exec(`INSERT INTO screenshots (...)`) // ✗ Fails (DB full, locked, etc.)
// Now: Image on disk, no index entry. Query misses it.
```

**Fix:**
- Use **atomic writes** with temp files:
  ```go
  tmpPath := filepath.Join(dir, "."+filename+".tmp")
  if err := os.WriteFile(tmpPath, imageData, 0o644); err != nil {
      return err  // No partial file on disk
  }
  if err := os.Rename(tmpPath, finalPath); err != nil {
      os.Remove(tmpPath)  // Cleanup temp
      return err
  }
  ```
- Implement `RebuildScreenshotIndex()` CLI tool:
  - Walks `.gasoline/screenshots/` directory
  - Reads `.json` sidecars
  - Rebuilds SQLite index from disk (source of truth)
  - Handles corruption recovery

**Implementation:**
```bash
gasoline rebuild-screenshot-index [--path .gasoline/screenshots]
```

---

## High-Priority Recommendations

### 4. Missing Query Indexes Cause Full Table Scans
**Section:** 5
**Status:** ✅ APPROVED — Index strategy now specified in spec

**Spec Updated:**
- `idx_component_variant_timestamp` — Primary index for component-based queries and `latest_per_variant` lookups
- `idx_url_timestamp` — For URL-filtered queries with time ranges
- `idx_viewport` — For viewport filtering
- `idx_timestamp DESC` — For time range queries without component filter
- `idx_filepath UNIQUE` — Unique constraint prevents duplicate storage

**Test:** Run query planner verification:
```go
rows, _ := db.Query("EXPLAIN QUERY PLAN SELECT ... WHERE component='button' AND variant='primary' ORDER BY timestamp DESC LIMIT 1")
// Verify it uses idx_component_variant_timestamp, NOT full table scan
```

---

### 5. SQLite Contention Under Parallel Viewport Captures
**Section:** 2, Concurrency
**Status:** ✅ APPROVED — Batch upload strategy now in spec

**Spec Updated (Section 1):**
- Extension captures all 3 viewports in parallel via `Promise.all()`
- Images saved to temp files on disk (no SQLite involvement)
- Single batch POST `/screenshots` with all viewports + shared metadata
- Server batch inserts all rows in single SQLite transaction
- Single lock acquisition instead of 3 serialized writes

**Implementation Pattern:**
```javascript
// Extension: Parallel capture, single batch upload
const [desktop, tablet, mobile] = await Promise.all([
  captureViewport(1280, 720),
  captureViewport(768, 1024),
  captureViewport(375, 667)
]);

POST /screenshots {
  timestamp: "2026-01-30T14:32:15Z",
  screenshots: [desktop, tablet, mobile],  // ← All viewports in one POST
  metadata: { ... }
}
```

```go
// Server: Single transaction for all inserts
func (store *SQLiteStore) StoreScreenshotBatch(screenshots []*Screenshot) error {
    tx, _ := db.BeginTx(ctx, nil)
    for _, ss := range screenshots {
        tx.Exec(`INSERT INTO screenshots ...`, ...)
    }
    return tx.Commit()  // Single lock acquisition for all 3 rows
}
```

**Target:** 1000–1500ms total for 3-viewport capture (vs 3300ms serialized)

---

### 6. Concurrency Design: Lock Contention with Capture.mu
**Section:** 2, Concurrency
**Status:** ✅ APPROVED — Separate mutex recommended for Phase 1

**Implementation Recommendation:**
```go
type Capture struct {
    mu              sync.RWMutex       // Existing: for logs, network, events
    screenshotMu    sync.RWMutex       // NEW: for screenshots only
}

func (c *Capture) HandleScreenshot(w http.ResponseWriter, r *http.Request) {
    c.screenshotMu.Lock()  // ← Separate lock, doesn't block observers
    defer c.screenshotMu.Unlock()

    // File I/O, SQLite write
    store.SaveScreenshotBatch(screenshots)
}
```

**Benefit:** File I/O (100–200ms) doesn't block WebSocket event ingestion or other observers.

**Test:** Measure observer latency under concurrent screenshot capture. Should be <10ms.

---

### 7. Query Pattern `latest_per_variant` Is Ad-Hoc, Not Generalizable
**Section:** 6
**Severity:** MEDIUM
**Impact:** Future schema friction when adding `latest_per_component`, `latest_per_url`, etc.

**Problem:**
- Spec defines `latest_per_variant: true` as a special case
- Phase 2 will likely need `latest_per_component`, `latest_per_url`, etc.
- Current schema doesn't generalize

**Recommendation:**
- Document `latest_per_variant` as shorthand for "GROUP BY variant, ORDER BY timestamp DESC"
- Phase 2: Generalize to `groupBy: 'component' | 'variant' | 'url' | undefined`
- Clarify response: When `groupBy` is specified, return **one entry per group** (not all entries)

---

### 8. Spec Missing Query Result Bounds
**Section:** 6
**Status:** ✅ APPROVED — Query limits now defined in spec

**Spec Updated:**
- **Default `limit`:** 10 (reasonable for LLM visual processing)
- **Maximum `limit`:** 100 (upper bound to prevent memory spike)
- Server enforces: If `limit > 100`, reject with 400 Bad Request
- Server enforces: If `limit` not specified, apply default 10
- Response includes `total_available` (actual count without limit) for pagination awareness

**Example:**
```json
{
  "screenshots": [...],  // 10 results (default limit)
  "count": 10,
  "total_available": 125,  // ← LLM knows there are 115 more
  "query": {"limit": 10}
}
```

---

### 9. Cleanup Policy & Retention Now Fully Specified
**Section:** Configuration
**Status:** ✅ APPROVED — Cleanup policy now in spec

**Spec Updated:**
```json
{
  "screenshots": {
    "retention": {
      "max_age_days": null,        // null = unlimited (no age-based cleanup)
      "max_disk_bytes": 5368709120 // 5GB default, triggers cleanup when exceeded
    }
  }
}
```

**Cleanup Behavior:**
1. **Age-based cleanup:** Only if `max_age_days` is set (e.g., 30 days)
2. **Space-based cleanup:** If directory exceeds `max_disk_bytes`, runs immediately (oldest first)
3. **Both enabled:** Satisfies both constraints (delete oldest until both conditions met)
4. **Daily job:** Runs at 2 AM UTC to check both conditions

**Disk Usage Warnings:**
- API/query responses include `disk_usage_percent` and optional `cleanup_warning`
- Warning triggers when usage > 80% of configured max:
  ```json
  {
    "disk_usage_bytes": 4294967296,
    "disk_usage_percent": 80,
    "cleanup_warning": "Screenshot storage at 80% capacity. Cleanup scheduled for 2:00 AM UTC."
  }
  ```
- LLM can monitor and adjust capture frequency if approaching limit

**Implementation:**
```go
func (s *ScreenshotStore) cleanup(ctx context.Context) error {
    config := s.config.Screenshots.Retention

    // Age-based: delete if max_age_days is set
    if config.MaxAgeDays > 0 {
        cutoff := time.Now().AddDate(0, 0, -config.MaxAgeDays)
        s.db.Exec("DELETE FROM screenshots WHERE timestamp < ?", cutoff)
    }

    // Space-based: delete oldest if exceeding limit
    usage := getDirectorySize(s.config.Screenshots.Path)
    if usage > config.MaxDiskBytes {
        remaining := usage - config.MaxDiskBytes
        s.db.Query("SELECT id FROM screenshots ORDER BY timestamp ASC").Each(func(id string) {
            if remaining <= 0 { return }
            // Delete oldest, decrement remaining
        })
    }
}
```

---

## Implementation Roadmap

### Phase 1: Core Screenshot Archival (Weeks 2-3)

**Week 1: Spec Finalization & Design**
- ✓ Complete this review
- Address all critical issues (metadata cap, path traversal, atomic writes)
- Design lock hierarchy (separate screenshot mutex if needed)
- Finalize SQLite schema with all indexes

**Week 2: Server Implementation**
1. Add `ScreenshotEntry`, `QueryParams`, `QueryResult` types to `types.go`
2. Implement `ScreenshotStore` directly (no interface yet; Phase 2 refactors to interface)
   - `Store(screenshot *Screenshot) error` — Atomic file write + SQLite insert
   - `Query(query *QueryParams) ([]*Screenshot, error)` — Execute queries with proper indexes
3. Implement handlers
   - `POST /screenshots` — Parse, validate metadata, store (with batch support)
   - Extend `observe()` to support `what: 'screenshots'` with query params
4. Add cleanup job + disk monitoring
5. Implement `RebuildScreenshotIndex()` CLI tool

**Week 3: Extension + Tests**
1. Extension: Parallel viewport capture + batch upload
2. Integration with pending queries (follow existing pattern in `communication.js`)
3. Test suite: 2500+ LOC
   - Unit: filename sanitization, metadata validation, query filters, indexes
   - Integration: capture → store → query → delete lifecycle
   - Security: SQL injection fuzzing, path traversal, metadata injection
   - Stress: 10K screenshots, query latency < 500ms

**Deliverables:**
- SQLite schema with indexes
- Atomic file I/O with temp files
- Query engine supporting all 7 patterns from Spec Section 6
- Extension integration
- CLI recovery tool
- Full test coverage

---

### Phase 2: Enterprise Backends (Future)
1. Refactor to `ScreenshotStore` interface
2. Implement `RemotePostgresStore`
3. Implement `CloudS3Store` (S3 for images, Postgres for metadata)
4. Generalize `latest_per_variant` to `groupBy` parameter
5. Add advanced query filters (pattern matching, exclusions, state filtering)

---

## Critical Files for Implementation

- **cmd/dev-console/main.go** — HTTP routes, screenshot handler, atomic file writes
- **cmd/dev-console/types.go** — Screenshot types, constants, buffer limits
- **cmd/dev-console/queries.go** — Query handlers, pending query integration
- **extension/background/communication.js** — Parallel capture, batch upload
- **.claude/refs/architecture.md** — Document new SQLite layer, concurrency guarantees

---

## Sign-Off

| Item | Status | Notes |
|------|--------|-------|
| **1. Metadata size limit (5KB)** | ✅ APPROVED | Spec enforces max 5KB, validates field names |
| **2. Path traversal sanitization** | ✅ APPROVED | All fields (component, variant, viewport) sanitized |
| **3. Atomic file writes** | ✅ APPROVED | Temp-file-then-rename pattern with recovery tool |
| **4. Query indexes** | ✅ APPROVED | Composite indexes specified: `idx_component_variant_timestamp`, `idx_url_timestamp` |
| **5. Batch uploads** | ✅ APPROVED | Extension parallel capture, single batch POST, server transaction |
| **6. Separate screenshot mutex** | ✅ APPROVED | Recommended for Phase 1 to avoid blocking observers |
| **7. Generalize latest_per_variant** | ✅ APPROVED | Documented as shorthand, Phase 2 will generalize to `groupBy` |
| **8. Query limits (10 default, 100 max)** | ✅ APPROVED | Spec enforces bounds, response includes `total_available` |
| **9. Cleanup policy (5GB space, unlimited age)** | ✅ APPROVED | Config-based, daily job, disk usage warnings in responses |

---

**Overall:** `APPROVED` — All critical issues and high-priority recommendations addressed in spec. Ready for implementation.

**Reviewer:** Principal Engineer (Claude Code), 2026-01-30 (Updated 2026-01-30)
**Next Step:** Schedule implementation kick-off. Target: 2 weeks (Week 2-3) for Phase 1 delivery.
