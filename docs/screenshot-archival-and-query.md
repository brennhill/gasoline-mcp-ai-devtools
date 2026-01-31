# Screenshot Archival & Query — Feature Spec

## Overview

Gasoline will automatically capture, archive, and index screenshots of web pages across multiple viewports. Screenshots will be semantically named and indexed in a SQLite database, allowing the LLM to query and retrieve screenshots by component, viewport, date, variant, or custom metadata.

This enables:
- **Design regression tracking** — Compare component appearance over time
- **Reference management** — Quickly fetch baseline screenshots for comparison
- **Variant archival** — Track light mode vs dark mode, A/B variants, etc.
- **LLM-driven analysis** — LLM retrieves screenshots and performs visual diffs

## Goals

1. Automate screenshot capture across responsive breakpoints
2. Create queryable archive of screenshots with rich metadata
3. Support future extensibility (external databases, cloud storage)
4. Zero external dependencies in runtime

## Non-Goals

- Pixel-level diff analysis (LLM does visual comparison)
- Automatic regression detection (LLM interprets results)
- Screen recording or video capture
- Image compression/optimization

---

## Design

### 1. Auto-Capture & Archival

When the LLM calls `observe({what: 'design-audit'})` or `observe({what: 'screenshots'})`:

1. **Extension captures in parallel** at configured viewports (desktop, tablet, mobile)
   - All 3 viewports captured simultaneously via `Promise.all()`
   - Images saved to temp files on disk
   - Metadata prepared for each

2. **Extension batch uploads** to server:
   ```javascript
   POST /screenshots {
     "timestamp": "2026-01-30T14:32:15Z",
     "screenshots": [
       {
         "viewport": {"name": "desktop", "width": 1280, "height": 720},
         "image": "base64-encoded-jpeg",
         "metadata": {...}
       },
       {
         "viewport": {"name": "tablet", "width": 768, "height": 1024},
         "image": "base64-encoded-jpeg",
         "metadata": {...}
       },
       {
         "viewport": {"name": "mobile", "width": 375, "height": 667},
         "image": "base64-encoded-jpeg",
         "metadata": {...}
       }
     ],
     "metadata": {
       "component": "dashboard",
       "variant": "lightmode",
       "page_title": "Dashboard",
       "custom_field": "valueFromLLM"  // LLM can add arbitrary metadata (max 5KB)
     }
   }
   ```

3. **Server batch inserts**:
   - Write all image files atomically (temp → final rename)
   - Single SQLite transaction for all rows
   - Single lock acquisition instead of 3 separate writes
   - Target latency: ~1000-1500ms for 3 viewports (vs 3300ms serialized)

### 2. Storage Structure

```
.gasoline/
├── screenshots/
│   ├── app-dashboard-desktop-1280x720-2026-01-30T14-32-15Z.jpg
│   ├── app-dashboard-desktop-1280x720-2026-01-30T14-32-15Z.json
│   ├── app-dashboard-tablet-768x1024-2026-01-30T14-32-15Z.jpg
│   ├── app-dashboard-tablet-768x1024-2026-01-30T14-32-15Z.json
│   ├── app-dashboard-mobile-375x667-2026-01-30T14-32-15Z.jpg
│   └── app-dashboard-mobile-375x667-2026-01-30T14-32-15Z.json
└── .index.db  # SQLite database
```

### 3. Filename Convention

```
{sitename}-{component}-{viewport}-{width}x{height}-{iso8601-timestamp}.jpg
```

- **sitename** — Project name (configurable)
- **component** — Which component/page (from metadata)
- **viewport** — Semantic name (desktop/tablet/mobile) or dimension fallback
- **width}x{height** — Actual dimensions
- **iso8601-timestamp** — When captured (sortable)

Example:
```
myapp-button-primary-desktop-1280x720-2026-01-30T14-32-15Z.jpg
```

### 4. JSON Metadata Sidecar

Each screenshot has a `.json` file with full metadata:

```json
{
  "filename": "myapp-button-primary-desktop-1280x720-2026-01-30T14-32-15Z.jpg",
  "component": "button",
  "variant": "primary",
  "viewport": "desktop",
  "dimensions": {
    "width": 1280,
    "height": 720
  },
  "timestamp": "2026-01-30T14:32:15Z",
  "page_title": "Component Library",
  "url": "http://localhost:3000/components/button",
  "metadata": {
    "variant_id": "primary",
    "state": "default",
    "theme": "lightmode",
    "custom_llm_field": "any value"
  }
}
```

The LLM can include arbitrary metadata when capturing. This gets stored in the sidecar JSON and indexed.

### 5. SQLite Index Schema

```sql
CREATE TABLE screenshots (
  id TEXT PRIMARY KEY,  -- uuid
  filename TEXT NOT NULL,
  filepath TEXT NOT NULL,
  component TEXT,
  variant TEXT,
  viewport TEXT,
  width INTEGER,
  height INTEGER,
  timestamp DATETIME NOT NULL,
  page_title TEXT,
  url TEXT,
  metadata JSON,  -- arbitrary JSON from LLM (max 5KB)
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Core indexes for common query patterns
CREATE INDEX idx_component_variant_timestamp ON screenshots(component, variant, timestamp DESC);
CREATE INDEX idx_url_timestamp ON screenshots(url, timestamp DESC);
CREATE INDEX idx_viewport ON screenshots(viewport);
CREATE INDEX idx_timestamp ON screenshots(timestamp DESC);
CREATE UNIQUE INDEX idx_filepath ON screenshots(filepath);
```

Server maintains this index. Extension doesn't write to DB—only server does (single writer).

**Index Strategy:**
- `idx_component_variant_timestamp` — Primary index for component-based queries and `latest_per_variant` lookups
- `idx_url_timestamp` — For URL-filtered queries with time ranges
- `idx_viewport` — For viewport filtering
- `idx_timestamp` — For time range queries without component filter
- `idx_filepath` — Unique constraint prevents duplicate storage

### 6. Query Interface

LLM queries screenshots via extended `observe()`:

```javascript
// List all screenshots (no query filters)
observe({what: 'screenshots'})

// List all screenshots from a specific URL
observe({
  what: 'screenshots',
  query: {
    url: 'http://localhost:3000/components/button'
  }
})

// List screenshots within a time range
observe({
  what: 'screenshots',
  query: {
    since: '2026-01-25T00:00:00Z',
    until: '2026-01-30T23:59:59Z'
  }
})

// List screenshots from URL within time range
observe({
  what: 'screenshots',
  query: {
    url: 'http://localhost:3000/components',
    since: '2026-01-28',
    until: '2026-01-30'
  }
})

// Get all desktop screenshots of button component
observe({
  what: 'screenshots',
  query: {
    component: 'button',
    viewport: 'desktop'
  }
})

// Get latest screenshot of each button variant
observe({
  what: 'screenshots',
  query: {
    component: 'button',
    latest_per_variant: true
  }
})

// Get latest 5 desktop screenshots
observe({
  what: 'screenshots',
  query: {
    viewport: 'desktop',
    limit: 5
  }
})

// Filter by custom metadata (e.g., theme, state)
observe({
  what: 'screenshots',
  query: {
    component: 'button',
    viewport: 'desktop',
    metadata: {
      theme: 'darkmode',
      state: 'hover'
    }
  }
})

// Get specific variant
observe({
  what: 'screenshots',
  query: {
    component: 'button',
    variant: 'primary',
    viewport: 'mobile'
  }
})
```

### 7. Query Response

```json
{
  "screenshots": [
    {
      "id": "uuid-123",
      "filename": "myapp-button-primary-desktop-1280x720-2026-01-30T14-32-15Z.jpg",
      "filepath": ".gasoline/screenshots/myapp-button-primary-desktop-1280x720-2026-01-30T14-32-15Z.jpg",
      "component": "button",
      "variant": "primary",
      "viewport": "desktop",
      "dimensions": {"width": 1280, "height": 720},
      "timestamp": "2026-01-30T14:32:15Z",
      "metadata": {
        "variant_id": "primary",
        "theme": "lightmode"
      }
    },
    // ... more results
  ],
  "count": 1,
  "total_available": 12,
  "query": {
    "component": "button",
    "viewport": "desktop",
    "limit": 10
  },
  "disk_usage_bytes": 2147483648,
  "disk_usage_percent": 40,
  "cleanup_warning": null  // or "Screenshot storage at 80% capacity. Cleanup scheduled for 2:00 AM UTC."
}
```

**Query Limits:**
- Default `limit`: 10 (reasonable for LLM processing)
- Maximum `limit`: 100 (upper bound)
- If `limit` not specified, server applies default 10
- If `limit > 100`, server rejects with 400 Bad Request
- Response includes `total_available` (actual count without limit) for pagination awareness

**Disk Usage Reporting:**
- Every query/list response includes `disk_usage_bytes`, `disk_usage_percent`, and optional `cleanup_warning`
- Warning appears when usage > 80% of configured max
- LLM can monitor and adjust capture frequency if approaching limit

LLM receives screenshot paths + metadata and can request the actual image files.

---

## Configuration

`.gasoline.design.json` (or provided config):

```json
{
  "screenshots": {
    "enabled": true,
    "path": ".gasoline/screenshots/",
    "viewports": [
      {"name": "desktop", "width": 1280, "height": 720},
      {"name": "tablet", "width": 768, "height": 1024},
      {"name": "mobile", "width": 375, "height": 667}
    ],
    "retention": {
      "max_age_days": null,        // null = unlimited (no age-based cleanup)
      "max_disk_bytes": 5368709120 // 5GB default, triggers cleanup when exceeded
    },
    "auto_capture": true,           // Capture on observe({what: 'design-audit'})
    "sitename": "myapp"
  }
}
```

**Retention Behavior:**
- **Age-based cleanup:** If `max_age_days` is set (e.g., 30), daily job deletes screenshots older than that
- **Space-based cleanup:** If directory exceeds `max_disk_bytes`, cleanup job runs immediately (oldest first)
- **Both enabled:** Cleanup satisfies both constraints (delete oldest until both conditions met)
- **Warnings:** API responses include `disk_usage_percent` and `cleanup_warning` if usage > 80% of limit

**Example Response:**
```json
{
  "screenshots": [...],
  "disk_usage_bytes": 4294967296,
  "disk_usage_percent": 80,
  "cleanup_warning": "Screenshot storage at 80% capacity. Cleanup scheduled for 2:00 AM UTC."
}
```

---

## Implementation Notes

### Server (Go)

- Initialize SQLite DB on first run
- Receive screenshot + metadata from extension
- Insert into SQLite with UUID
- Save image files to disk
- Handle query requests via MCP

### Extension (Chrome)

- Capture screenshots at configured viewports
- Send to server with metadata
- No direct DB access (server owns writes)

### Future Extensibility

Storage abstraction allows swapping backends:

```go
type ScreenshotStore interface {
  Store(screenshot *Screenshot) error
  Query(query *QueryParams) ([]*Screenshot, error)
}

// Phase 1: Localhost SQLite
type LocalSQLiteStore struct { ... }

// Phase 2: Enterprise — remote DB
type RemotePostgresStore struct { ... }

// Phase 2: Enterprise — cloud storage (S3 + metadata DB)
type CloudS3Store struct { ... }
```

Currently: LocalSQLiteStore (bundled, zero deps)
Future: Configurable via `.gasoline.design.json`

---

## Example Workflow

**LLM:**
```
"I need to audit the button component across all viewports and compare it to the baseline from Jan 25."

1. observe({what: 'design-audit', viewports: ['desktop', 'tablet', 'mobile'], metadata: {component: 'button'}})
   → Extension captures 3 screenshots, server stores + indexes

2. observe({what: 'screenshots', query: {component: 'button', date_from: '2026-01-25', limit: 10}})
   → Server returns matching screenshots from DB

3. LLM sees: button-desktop-jan25.jpg, button-desktop-jan30.jpg, etc.
   → LLM visually compares, describes differences
   → LLM fixes code

4. observe({what: 'design-audit', metadata: {component: 'button', variant: 'fixed'}})
   → New screenshot captured, indexed

5. observe({what: 'screenshots', query: {component: 'button', variant: 'fixed'}})
   → Verify fix
```

---

## Success Criteria

- ✅ Screenshots auto-captured in parallel across viewports
- ✅ Batch uploads reduce multi-viewport latency to ~1500ms
- ✅ Atomic file writes prevent orphaned screenshots
- ✅ SQLite with composite indexes for fast queries
- ✅ LLM can retrieve screenshots by component/viewport/date/variant/url
- ✅ Query limits (default 10, max 100) prevent memory spike
- ✅ Space-based cleanup (5GB default) with disk usage warnings
- ✅ Age-based cleanup (unlimited by default) configurable
- ✅ Zero external dependencies
- ✅ Storage abstraction supports future backends (Postgres, S3)

---

## Implementation Notes

**Metadata Size Limit:** All LLM-supplied metadata capped at 5KB per screenshot. Oversized metadata rejected at server.

**Path Traversal Prevention:** All user-supplied fields (component, variant, viewport, sitename) sanitized before inclusion in filenames.

**Atomic Writes:** Screenshot files written to temp file, then renamed to final location. Database row inserted only after file successfully written.

**Batch Uploads:** Extension captures all viewports in parallel, sends single batch POST with all screenshots + shared metadata.

---

## Open Questions / Future Work

**Phase 2+ Enhancements:**
1. **Advanced query filters**
   - Pattern matching: `component: 'button*'` for "button-primary", "button-secondary", etc.
   - Exclude filters: `viewport: {not: 'mobile'}`
   - State filtering: `state: 'hover'` or `state: 'error'`
   - Generalize `latest_per_variant` to `groupBy` parameter

2. **Image compression** — JPEG quality tuning, WebP format support

3. **Diff visualization** — Generate visual diff images for PRs showing what changed between two screenshots

4. **Regression detection** — Automated detection of unexpected visual changes (variant X differs from baseline)

5. **External storage** — S3 + metadata in Postgres for enterprise deployments (keep zero-deps for embedded, abstract storage layer)

6. **Diff analysis** — Extract what changed (color, spacing, font-size) from screenshot pairs

7. **Cleanup strategies** — Pluggable cleanup: oldest-first, least-used, by-component, etc.
