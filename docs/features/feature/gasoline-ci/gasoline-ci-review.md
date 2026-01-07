# Gasoline CI Specification - Technical Review

**Reviewer:** Principal Engineer
**Date:** 2026-01-26
**Spec Version:** As of commit 859db4e

---

## Executive Summary

The Gasoline CI spec presents a sound high-level architecture for extending browser observability into CI/CD pipelines. However, the implementation details reveal **three critical concerns**: (1) the `gasoline-ci.js` script duplicates rather than extracts from `inject.js`, creating divergence risk despite claims of sync; (2) the `/test-boundary` endpoint introduces global mutable state without solving the fundamental multi-worker race condition; (3) the snapshot endpoint lacks pagination and will timeout on large test suites. The spec is implementable with the fixes below, but shipping without addressing the critical issues will cause production failures at scale.

---

## 1. Performance Analysis

### 1.1 Critical: `/snapshot` Lacks Pagination (Section: Component 2)

**Problem:** The spec shows `GET /snapshot` returning ALL logs, WebSocket events, network bodies, and actions in a single response. For a 10-worker CI suite running 500 tests, this could easily be 50,000+ entries. The current server already struggles at 1,000 entries (see `types.go:327` `maxWSEvents = 500`).

```go
// Spec lines 596-627: handleSnapshot aggregates everything
snapshot := SnapshotResponse{
    Logs:            logs,           // Could be 10,000+
    WebSocket:       wsEvents,       // Could be 5,000+
    NetworkBodies:   networkBodies,  // Could be 2,000+
}
```

**Impact:**
- JSON serialization will block the HTTP server goroutine for seconds
- Memory spike during marshal (double the data in memory)
- Response timeout for large snapshots
- GC pressure from large intermediate allocations

**Recommendation:** Add required pagination:
```go
type SnapshotRequest struct {
    Since    string `json:"since"`     // ISO8601 timestamp
    TestID   string `json:"test_id"`   // Filter
    Offset   int    `json:"offset"`    // Pagination
    Limit    int    `json:"limit"`     // Default 100, max 500
}
```

### 1.2 High: Batching Without Backpressure (Section: Component 1, Lines 119-150)

**Problem:** The CI script batches entries and flushes every 100ms, but has no backpressure mechanism. If the server is slow or unreachable, batches accumulate unbounded in `logBatch`, `wsBatch`, `networkBatch`.

```javascript
// Spec lines 120-123
let logBatch = [];
let wsBatch = [];
let networkBatch = [];  // No size limit!
```

**Impact:**
- Memory leak in browser context during test execution
- Lost data when `sendBeacon` fails silently (line 158)
- No visibility into capture health

**Recommendation:** Add batch size limits and drop policy:
```javascript
const MAX_PENDING_BATCHES = 500;

function emit(type, payload) {
    switch (type) {
        case 'GASOLINE_LOG':
            if (logBatch.length < MAX_PENDING_BATCHES) {
                logBatch.push(payload);
            } // else: drop oldest or newest based on priority
            break;
    }
}
```

### 1.3 Medium: Memory Calculation Mismatch

The spec claims `< 5MB` memory budget for the CI script (line 1329), but the existing `inject.js` already imports from 10+ modules (`lib/serialize.js`, `lib/context.js`, etc. - see `inject.js:14-129`). The CI script either needs to inline all dependencies or accept a larger footprint.

---

## 2. Concurrency Analysis

### 2.1 Critical: Test Boundary Race Condition (Section: Component 2, Lines 680-716)

**Problem:** The spec proposes `currentTestID` as server state set by `/test-boundary`. With parallel workers, this creates a classic race:

```
Worker 1: POST /test-boundary {test_id: "A", action: "start"}
Worker 2: POST /test-boundary {test_id: "B", action: "start"}  // Overwrites!
Worker 1: POST /logs {...}  // Tagged with "B", not "A"
```

The spec acknowledges this in "Multi-Worker Isolation" (lines 1359-1410) but the solution (`window.__GASOLINE_TEST_ID`) only works if entries are tagged **client-side**, not server-side.

**Impact:** Test isolation is completely broken under parallelism - the primary use case.

**Recommendation:** Remove server-side `currentTestID`. Require `test_id` on every POST:
```go
// Every endpoint accepts test_id in payload
type LogPayload struct {
    Entries []LogEntry `json:"entries"`
    TestID  string     `json:"test_id"` // REQUIRED for CI mode
}
```

### 2.2 High: Lock Ordering in Snapshot (Section: Component 2, Lines 1457-1473)

**Problem:** The spec shows acquiring multiple locks for snapshot atomicity:

```go
// Spec lines 1459-1461
s.mu.RLock()
capture.mu.RLock()  // Second lock while holding first
defer s.mu.RUnlock()
defer capture.mu.RUnlock()
```

The existing codebase has `Server.mu` and `Capture.mu` as separate mutexes. Acquiring them in different orders elsewhere would deadlock. I verified the existing code (`main.go:505-540`, `websocket.go:21-56`) doesn't hold both locks simultaneously, but this pattern is fragile.

**Recommendation:** Either:
1. Document lock ordering invariant: "Always acquire `Server.mu` before `Capture.mu`"
2. Or use a single lock for snapshot (copy data under one lock, release, then build response)

### 2.3 Medium: Graceful Shutdown Data Loss (Section: Lines 1419-1481)

The spec proposes 500ms shutdown timeout, but existing `gracePeriod` in `main.go:1006-1012` is only 100ms. If CI runner sends SIGTERM during high-volume capture, in-flight HTTP requests may not complete.

---

## 3. Data Contract Analysis

### 3.1 Critical: Payload Parity Claim is False (Section: Lines 1251-1261)

**Problem:** The spec claims identical payload format between extension and CI script, but the code shows divergence:

Extension (`inject.js` via `lib/bridge.js`):
```javascript
// Posts via window.postMessage with type wrapper
window.postMessage({ type: 'GASOLINE_LOG', payload }, '*')
```

CI Script (spec lines 172-189):
```javascript
// Posts directly to server with different structure
function emit(type, payload) {
    switch (type) {
        case 'GASOLINE_LOG':
            logBatch.push(payload);  // Just payload, no wrapper
```

The server endpoint (`main.go:1464-1486`) expects `{ entries: [...] }`, but the extension's `background.js` reformats the postMessage payload. The CI script must match that reformatting.

**Recommendation:** Extract the transformation logic to a shared function, or document the exact wire format with JSON schemas.

### 3.2 High: Missing Version Negotiation

Neither the CI script nor the server has version negotiation. When the capture format changes, old CI scripts will silently produce incompatible data. The spec mentions `captureVersion: '1.0.0'` (line 452) but this is informational only.

**Recommendation:** Add server-side validation:
```go
const supportedCaptureVersions = []string{"1.0.0"}

func validateCaptureVersion(version string) error {
    for _, v := range supportedCaptureVersions {
        if v == version { return nil }
    }
    return fmt.Errorf("unsupported capture version: %s", version)
}
```

### 3.3 Medium: Type Safety Across Go/JS Boundary

The spec defines Go structs (`SnapshotResponse`, `TestBoundary`) but no TypeScript types for the Playwright fixture. The `GasolineSnapshot` interface (lines 798-809) has `logs: any[]` which defeats type safety.

**Recommendation:** Generate TypeScript types from Go structs, or define JSON Schema as the source of truth.

---

## 4. Error Handling Analysis

### 4.1 High: Silent Failures in CI Script (Lines 152-167)

```javascript
function sendToServer(endpoint, data) {
    // ...
    fetch(url, { ... }).catch(() => {}); // Swallow ALL errors
}
```

This makes debugging CI capture failures impossible. A test fails, no Gasoline data appears, and there's no indication why.

**Recommendation:** At minimum, log to console in debug mode:
```javascript
.catch((err) => {
    if (window.__GASOLINE_DEBUG) {
        console.warn('[gasoline-ci] Server send failed:', err.message);
    }
});
```

### 4.2 High: No Retry Logic for Transient Failures

The CI script uses `sendBeacon` which returns `true/false` but doesn't retry on failure. Network blips during test execution will cause permanent data loss.

**Recommendation:** Implement exponential backoff for critical data (errors, test boundaries):
```javascript
async function sendCritical(endpoint, data, retries = 3) {
    for (let i = 0; i < retries; i++) {
        try {
            const res = await fetch(url, { ... });
            if (res.ok) return true;
        } catch { /* continue */ }
        await sleep(100 * Math.pow(2, i));
    }
    return false;
}
```

### 4.3 Medium: Reporter Fails on Server Unreachable (Lines 1011-1035)

The `gasoline report` command queries `/snapshot` but has no timeout or retry. If the server crashed during tests, the report command hangs indefinitely.

---

## 5. Security Analysis

### 5.1 High: CORS Allows Any Origin (Existing Issue Amplified)

The existing CORS middleware (`main.go:648-660`) allows `*`. For local dev this is acceptable, but in CI environments where the runner may have network access to other services, this becomes a vector for exfiltration.

**Recommendation:** For CI mode, restrict to localhost only:
```go
func ciCorsMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if strings.HasPrefix(origin, "http://127.0.0.1") ||
           strings.HasPrefix(origin, "http://localhost") {
            w.Header().Set("Access-Control-Allow-Origin", origin)
        }
        // ...
    }
}
```

### 5.2 Medium: Test Data in Artifacts May Contain Secrets

The spec attaches snapshots to test reports on failure (lines 857-875). These snapshots may contain:
- Request bodies with API keys (not in SENSITIVE_HEADERS)
- Response bodies with tokens
- localStorage dumps from `captureState()`

**Recommendation:** Add explicit redaction before attachment:
```typescript
const snapshot = await fixture.getSnapshot();
const redacted = redactSensitiveData(snapshot); // Apply redaction rules
await testInfo.attach('gasoline-snapshot', { body: JSON.stringify(redacted) });
```

### 5.3 Low: Build Script Path Traversal

The build script (`scripts/build-ci-script.js`, lines 1486-1637) uses user-controlled paths without sanitization. While this is a dev-time tool, it could be exploited in CI if the paths come from environment variables.

---

## 6. Maintainability Analysis

### 6.1 High: Divergent Codebases Despite "Sync" Claims

The spec repeatedly claims `gasoline-ci.js` is "extracted" from `inject.js` and "stays in sync" (lines 89, 468, 527-537). However:

1. The actual `inject.js` is modular (imports from 10+ files in `lib/`)
2. The spec shows a monolithic CI script with duplicated functions
3. The "extraction" is regex-based, not AST-based (lines 1575-1584)

This will inevitably drift. The first time someone refactors `inject.js` structure, the regex extraction breaks silently.

**Recommendation:** Two options:
1. **True extraction:** Use esbuild/rollup to bundle `inject.js` with a CI-specific transport shim
2. **Accept duplication:** Maintain two codebases with shared test suite that verifies parity

### 6.2 Medium: Test Coverage Gaps

The spec lists tests (lines 1673-1783) but critical paths are untested:
- Concurrent test boundary handling
- Memory pressure during CI capture
- Network failure recovery
- Server restart mid-test

**Recommendation:** Add integration tests using Playwright to test the Playwright fixture (meta-testing).

### 6.3 Medium: Complexity Budget Exceeded

The spec adds:
- 3 new HTTP endpoints
- 1 new CLI subcommand
- 1 new npm package (@anthropic/gasoline-ci)
- 1 new Playwright fixture package
- 1 new GitHub Action

This is a significant surface area expansion. Each component needs its own test suite, documentation, and maintenance.

---

## Implementation Roadmap

### Phase 0: Prerequisites (1 week)
1. Add pagination to existing `GetWebSocketEvents`, `getEntries` methods
2. Document lock ordering invariant
3. Add JSON Schema for wire formats

### Phase 1: Server Endpoints (1 week)
1. Implement `/snapshot` with pagination (CRITICAL)
2. Implement `/clear` (low risk, straightforward)
3. **Skip `/test-boundary`** - require client-side tagging instead
4. Add `test_id` support to existing POST endpoints

### Phase 2: CI Capture Script (2 weeks)
1. **Option A (recommended):** Create esbuild config that bundles `inject.js` with CI transport
2. **Option B:** Implement AST-based extraction with verified test parity
3. Add batch size limits and debug logging
4. Add basic retry for critical data

### Phase 3: Playwright Integration (1 week)
1. Implement fixture with proper error handling
2. Add TypeScript types (generate from Go structs)
3. Add redaction before attachment

### Phase 4: Reporter & GitHub Action (1 week)
1. Implement `gasoline report` subcommand
2. Add timeout and retry logic
3. Create GitHub Actions (thin wrappers)

### Phase 5: Validation (1 week)
1. End-to-end test: 10-worker Playwright suite with intentional failures
2. Memory stress test: verify no leaks over 1000+ test runs
3. Failure injection: server restart, network partition

---

## Summary of Required Changes

| Priority | Issue | Section | Fix |
|----------|-------|---------|-----|
| CRITICAL | Snapshot lacks pagination | 1.1 | Add offset/limit params |
| CRITICAL | Test boundary race condition | 2.1 | Require client-side test_id |
| CRITICAL | Payload format divergence | 3.1 | Use build tooling, not regex |
| HIGH | No backpressure in batching | 1.2 | Add MAX_PENDING_BATCHES |
| HIGH | Lock ordering undefined | 2.2 | Document or restructure |
| HIGH | Silent failures | 4.1 | Add debug logging |
| HIGH | CORS too permissive for CI | 5.1 | Restrict to localhost |
| MEDIUM | No version negotiation | 3.2 | Add server validation |
| MEDIUM | Secrets in artifacts | 5.2 | Add redaction layer |

---

**Recommendation:** Do not proceed with implementation until Critical issues are resolved. The spec is 80% of the way to a solid design but will fail in production under parallel test execution without the fixes above.
