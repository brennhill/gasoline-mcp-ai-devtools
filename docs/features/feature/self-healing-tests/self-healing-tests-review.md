# Technical Review: Agentic CI/CD Loop (Self-Healing Tests)

**Reviewer:** Principal Engineer
**Date:** 2026-01-26
**Spec Location:** `/docs/ai-first/tech-spec-agentic-cicd.md`
**Supporting Spec:** `/docs/gasoline-ci-specification.md`

---

## Executive Summary

The Agentic CI/CD spec is architecturally sound for its stated goal: orchestrating existing Gasoline primitives into autonomous AI workflows. The clear separation between Gasoline (observation layer) and Phase 8 (agent behavior patterns) is correct. However, the spec conflates two distinct implementation efforts: (1) the Gasoline CI infrastructure (new endpoints, capture script, Playwright fixture) which requires significant server changes, and (2) the Claude Code skills which are purely compositional. The CI infrastructure spec has several performance, concurrency, and data contract risks that require attention before implementation.

---

## 1. Critical Issues (Must Fix Before Implementation)

### 1.1 Race Condition in Multi-Worker Test ID Tagging

**Location:** Gasoline CI Spec, "Multi-Worker Isolation" section (lines 1359-1410)

**Problem:** The spec describes two competing mechanisms for test ID tagging:
1. Client-side: `window.__GASOLINE_TEST_ID` included in every POST payload
2. Server-side: `/test-boundary` sets "current test" context; subsequent entries inherit it

These conflict. The server-side approach (`/test-boundary`) uses a single `currentTestID` field (line 697-702), which breaks under concurrent workers. Worker A calls `/test-boundary start`, then Worker B calls `/test-boundary start`, and now all entries from both workers get Worker B's test ID.

**Required Fix:** Remove the server-side `currentTestID` approach entirely. The client-side `test_id` in the POST payload is the only viable approach for concurrent workers. Update the spec to clarify:
- `/test-boundary` should only be used for event logging, not for implicit tagging
- All test ID correlation must be explicit via payload field

### 1.2 Snapshot Atomicity Causes Lock Contention Deadlock Risk

**Location:** Gasoline CI Spec, "Snapshot Atomicity" section (lines 1456-1469)

**Problem:** The proposed implementation acquires multiple locks:
```go
s.mu.RLock()
capture.mu.RLock()
defer s.mu.RUnlock()
defer capture.mu.RUnlock()
```

This introduces deadlock risk if any other code path acquires these locks in the opposite order. The existing codebase (`tools.go` line 262-270, `captureStateAdapter`) already acquires `server.mu.RLock()` then iterates entries. If `Capture` methods internally acquire `server.mu`, deadlock occurs.

**Required Fix:** Establish and document a global lock ordering: always `server.mu` before `capture.mu`. Audit all existing code paths. Better: use a single snapshot function that copies all data under one lock, then processes outside the lock:

```go
func (s *Server) handleSnapshot(...) {
    // Capture all data under a single lock acquisition cycle
    serverData := s.copyEntriesUnderLock()
    captureData := capture.copyDataUnderLock()

    // Process outside locks
    response := buildSnapshotResponse(serverData, captureData)
    json.NewEncoder(w).Encode(response)
}
```

### 1.3 Memory Pressure from Unbounded Test ID Tracking

**Location:** Gasoline CI Spec, Multi-Worker Isolation section

**Problem:** The spec says "The server tracks multiple active test IDs simultaneously" but provides no eviction policy. With parallel CI workers, flaky test retries, and long-running suites, test IDs accumulate. Each entry stores the test ID string, and filtering by test ID requires scanning all entries.

**Required Fix:** Implement bounded test ID tracking:
- Maximum 100 active test IDs (configurable)
- LRU eviction when limit reached
- Test IDs expire after 1 hour of inactivity
- Add `test_id` index for O(1) lookup instead of O(n) scan

### 1.4 `gasoline-ci.js` Build Script is Fragile

**Location:** Gasoline CI Spec, lines 1486-1637

**Problem:** The build script uses regex extraction (`extractFunction` with `// === {name} ===` markers). This is extremely brittle:
- Adding comments or refactoring `inject.js` silently breaks CI script
- No automated verification that extracted functions are complete
- Regex can't handle nested functions or closures properly

**Required Fix:** Either:
1. **Option A (Recommended):** Use a proper AST parser (esprima, acorn) to extract functions
2. **Option B:** Maintain `gasoline-ci.js` as the canonical source, with `inject.js` importing from it
3. **Option C:** Add comprehensive integration tests that verify behavioral equivalence between extension and CI script

At minimum, add a CI step that runs both extension and CI script against the same test page and asserts identical captured payloads.

---

## 2. Performance Concerns

### 2.1 `sendBeacon` Reliability in CI Context

**Location:** Gasoline CI Spec, lines 153-168

**Problem:** `navigator.sendBeacon` has a size limit (typically 64KB) and silently fails when exceeded. In a CI context with verbose logging or large network bodies, this will cause data loss without any indication.

**Recommendation:**
- Check payload size before using `sendBeacon`
- Fall back to `fetch` with `keepalive: true` for large payloads
- Add a size warning in the server logs when payloads are truncated

```javascript
function sendToServer(endpoint, data) {
    const body = JSON.stringify(data);
    if (body.length > 60000 && navigator.sendBeacon) {
        console.warn('[gasoline-ci] Payload exceeds sendBeacon limit, using fetch');
        fetch(url, { method: 'POST', body, keepalive: true }).catch(() => {});
        return;
    }
    // ... existing logic
}
```

### 2.2 `/snapshot` Performance Under Load

**Location:** Gasoline CI Spec, lines 546-629

**Problem:** The `/snapshot` endpoint serializes ALL captured data (logs, WebSocket events, network bodies, actions) in a single response. With 1000 log entries + 500 WS events + 100 network bodies, the response could be 10MB+. This blocks the HTTP handler goroutine and causes GC pressure.

**Recommendation:**
- Add pagination: `/snapshot?offset=0&limit=100`
- Add field selection: `/snapshot?fields=logs,stats` (omit heavy fields)
- Consider streaming via chunked transfer encoding for large snapshots
- Set a response size limit (e.g., 5MB) with truncation warning

### 2.3 `computeStats` Linear Scan

**Location:** Gasoline CI Spec, line 615

**Problem:** `computeStats(logs, wsEvents, networkBodies)` scans all entries to count errors, warnings, and failures. This is O(n) per snapshot request.

**Recommendation:** Maintain running counters in the server, updated on insert. `computeStats` becomes O(1) lookup of pre-computed values.

```go
type Server struct {
    // ... existing
    stats struct {
        sync.RWMutex
        errorCount    int
        warningCount  int
        networkErrors int // status >= 400
    }
}
```

---

## 3. Data Contract Risks

### 3.1 Test ID Type Inconsistency

**Location:** Multiple sections

**Problem:** The spec shows `test_id` as a string in some places and omits it in others. The `SnapshotResponse` struct (line 553) has `TestID string` but the POST `/logs` body (line 1387) shows it as a top-level field. This creates ambiguity about where the test ID lives in the data model.

**Recommendation:** Standardize:
- `test_id` is ALWAYS in the POST body (never in headers, never server-derived)
- `test_id` is copied onto each entry at storage time
- `test_id` appears in all response types that include entries

### 3.2 Timestamp Format Inconsistency

**Location:** Throughout spec

**Problem:** The spec uses `RFC3339Nano` in some places and ISO 8601 in others. The `since` parameter (line 588) uses `time.RFC3339Nano`, but the JavaScript client generates `new Date().toISOString()` which is RFC3339 without nanoseconds.

**Recommendation:** Accept both formats in the server, always return RFC3339Nano. Document the format explicitly in the API contract.

### 3.3 Breaking Change Risk for Existing Clients

**Location:** Gasoline CI Spec, Backwards Compatibility section

**Problem:** While the spec claims no breaking changes, the addition of `/snapshot` and `/clear` could break clients that enumerate server endpoints. More critically, if `POST /logs` starts accepting `test_id`, existing extension code that doesn't send it will work, but clients that inspect the response may not expect it.

**Recommendation:** Add API versioning now, before shipping:
- `Accept: application/vnd.gasoline.v1+json` header for new clients
- Unversioned requests get legacy behavior
- This enables clean migration path for CI-specific features

---

## 4. Concurrency Issues

### 4.1 Verification Session State Machine Has Race Windows

**Location:** `verify.go`, lines 263-294 (Watch action), 301-336 (Compare action)

**Problem (in existing code, relevant to spec):** The `Watch` and `Compare` methods check status, then modify it. Between the check and modification, another goroutine could change state:

```go
// Watch() at line 279-280
if session.Status == "cancelled" {
    return nil, fmt.Errorf("session %q has been cancelled", sessionID)
}
// <-- Another goroutine could cancel here
session.Status = "watching"
```

**Recommendation:** The existing implementation holds the lock for the entire operation, which is correct but not obvious. Add a comment clarifying this is intentional, and add tests that exercise concurrent Watch/Cancel scenarios.

### 4.2 Extension Polling Race with Clear

**Location:** Gasoline CI Spec, `/clear` endpoint

**Problem:** If `/clear` is called while the extension is mid-POST, the extension's data is lost (expected) but the extension may also POST new data immediately after, which appears in the "cleared" buffer (unexpected).

**Recommendation:** Add a generation counter or epoch timestamp:
- `/clear` increments epoch
- POSTs include their epoch
- Server rejects POSTs with old epochs after a clear

This is low priority for CI (tests control timing) but matters for interactive debugging.

---

## 5. Security Concerns

### 5.1 Test ID Injection

**Location:** Gasoline CI Spec, Multi-Worker Isolation

**Problem:** Test IDs come from client-controlled input (`window.__GASOLINE_TEST_ID`). A malicious or buggy test could set a test ID that matches another worker's ID, causing cross-contamination.

**Recommendation:**
- Server should validate test ID format (alphanumeric + limited punctuation)
- Consider server-generated test IDs via `/test-boundary start` returning a unique ID
- Document that test ID uniqueness is the client's responsibility

### 5.2 Audit Trail in CI

**Location:** Self-Healing Tests spec, Security section (lines 113-117)

**Problem:** The spec mentions "Audit trail of all AI-generated commits" but doesn't specify the audit mechanism. In CI, the audit must persist beyond the ephemeral container.

**Recommendation:** Specify:
- Audit logs written to stdout for CI runner capture
- Structured JSON format for machine parsing
- Include: test name, AI diagnosis, proposed fix, git diff, commit SHA
- Integration with GitHub Actions artifacts

### 5.3 `execute_javascript` in CI Context

**Location:** Not in CI spec, but relevant

**Problem:** The PR Preview Exploration workflow (lines 173-178) uses `execute_javascript` to interact with the page. This is extremely powerful in CI where the "user" enabling AI Web Pilot is an automated process, not a human.

**Recommendation:**
- Explicitly require human approval for workflows that use `execute_javascript`
- Or scope `execute_javascript` in CI to safe read-only operations
- Add CI-specific mode flag that restricts available tools

---

## 6. Maintainability Concerns

### 6.1 Skill Definition Format Undefined

**Location:** Self-Healing Tests spec, lines 247-270

**Problem:** The skill YAML format is shown but not specified:
- Where do skills live? `.claude/skills/`?
- What's the workflow syntax?
- How are triggers defined?
- What's the permission model?

This is critical for Phase 8 but orthogonal to Gasoline changes.

**Recommendation:** Either:
1. Reference an external Claude Code skills specification
2. Mark skill format as TBD with a link to where it will be defined
3. Scope Phase 8 as "manual agent invocation only" for v1

### 6.2 Complexity Budget Exceeded

**Location:** `tools.go`

**Problem:** `ToolHandler` already has 18 fields (line 122-157). Adding more managers (session manager, verification manager, etc.) increases cognitive load and initialization complexity. The `NewToolHandler` function (lines 159-252) is 94 lines of setup.

**Recommendation:**
- Group related managers into sub-structs (e.g., `SecurityTools`, `SessionTools`)
- Consider a plugin/registry pattern where tools register themselves
- Split `tools.go` into domain-specific files (already partially done)

### 6.3 Test Strategy Incomplete

**Location:** Gasoline CI Spec, Testing Strategy section

**Problem:** The test list (lines 1675-1783) is comprehensive but doesn't specify:
- Test data fixtures (what does a valid snapshot look like?)
- Mocking strategy (how to test without real browser?)
- Performance test methodology (how to verify budget compliance?)

**Recommendation:** Before implementation:
1. Create golden file fixtures for all response types
2. Document mock strategy: use `httptest.Server` + pre-recorded captures
3. Add benchmark tests with explicit assertions (e.g., `b.N > 1000/sec`)

---

## 7. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)

1. **Fix lock ordering bug** - Audit all existing code, document lock order
2. **Implement `/snapshot` endpoint** - Start simple, no filtering
3. **Implement `/clear` endpoint** - Reset all buffers atomically
4. **Add test ID to log entry storage** - Accept `test_id` in POST body, store on entries
5. **Write golden file tests** - Capture expected response shapes

### Phase 2: CI Script (Week 3-4)

1. **Create `gasoline-ci.js` manually** - Don't auto-generate yet
2. **Add integration tests** - Verify parity with extension
3. **Implement batching** - With `sendBeacon` size checking
4. **Add CI sync check** - Fail build if drift detected
5. **Document usage** - Playwright example in README

### Phase 3: Playwright Fixture (Week 5-6)

1. **Create `@gasoline/playwright` package** - Basic fixture
2. **Implement auto-inject** - Via `addInitScript`
3. **Implement attach-on-failure** - Snapshot to test report
4. **Add test ID correlation** - Worker-safe via payload
5. **Write fixture tests** - Against real Playwright

### Phase 4: Reporter & Polish (Week 7-8)

1. **Add `report` subcommand** - text, JSON, ai-context formats
2. **Create GitHub Action** - Wrapper for common patterns
3. **Performance tuning** - Add counters, pagination
4. **Security review** - Input validation, audit logging
5. **Documentation** - Full API reference

### Phase 5: Skills Integration (Future)

1. **Define skill format** - Coordinate with Claude Code team
2. **Implement `/self-heal` skill** - Using existing tools
3. **Add circuit breaker** - Max 3 fix attempts
4. **Human approval gates** - For structural changes

---

## 8. Recommendations Summary

| Priority | Issue | Recommendation |
|----------|-------|----------------|
| P0 | Multi-worker test ID race | Remove server-side `currentTestID`, use payload-only |
| P0 | Lock ordering deadlock risk | Document and enforce global lock order |
| P0 | CI script fragility | Use AST parser or manual maintenance |
| P1 | Memory pressure from test IDs | Add LRU eviction and index |
| P1 | `/snapshot` performance | Add pagination and field selection |
| P1 | Data contract ambiguity | Standardize test ID location and timestamp format |
| P2 | `sendBeacon` size limit | Add size check and fallback |
| P2 | Test ID injection | Validate format, document responsibility |
| P2 | Audit trail persistence | Write to stdout in structured format |
| P3 | Skill format undefined | Reference external spec or mark TBD |
| P3 | Complexity budget | Group managers into sub-structs |

---

## 9. Conclusion

The Agentic CI/CD spec represents the right strategic direction: positioning Gasoline as the observation layer that enables AI-driven development workflows. The technical risks are manageable with the fixes outlined above. The most critical work is in the CI infrastructure (Gasoline CI spec), not in the skill definitions (Agentic spec), because that's where the concurrency and performance risks live.

Recommend proceeding with Phase 1-4 of the roadmap before attempting Phase 8 skill integration. The skills are compositional and low-risk once the primitives are solid.
