---
feature: analyze-tool
version: v7.0-rev2
---

# `analyze` Tool — Comprehensive QA Plan

## Testing Strategy Overview

### Test Pyramid

| Layer | Count | Focus |
|-------|-------|-------|
| Unit (Go) | ~50 tests | Request validation, response schemas, timeout logic |
| Unit (Extension) | ~40 tests | axe-core integration, analyzers, message passing |
| Integration | ~20 tests | End-to-end flows, error propagation |
| Edge Cases | ~60 tests | Failure modes, resource exhaustion, state corruption |
| Performance | ~15 tests | Benchmarks, load scenarios |
| Security | ~10 tests | Redaction, permission enforcement |

### Test Locations

- **Go Server:** `cmd/dev-console/analyze_test.go`
- **Extension:** `tests/extension/analyze.test.js`, `tests/extension/analyze-*.test.js`
- **Integration:** `tests/integration/analyze_test.go`

---

## Unit Tests: Go Server

**Location:** `cmd/dev-console/analyze_test.go`

### 1. Tool Registration

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-GO-001 | `analyze` tool appears in MCP tools/list | Tool present with correct name |
| A-GO-002 | Tool has correct parameter schema | action (required), scope, selector, tab_id, force_refresh |
| A-GO-003 | Tool description matches spec | Description mentions "structured findings" |

### 2. Request Validation

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-GO-010 | Valid action `audit` accepted | Request creates pending query |
| A-GO-011 | Valid action `memory` accepted | Request creates pending query |
| A-GO-012 | Valid action `security` accepted | Request creates pending query |
| A-GO-013 | Valid action `regression` accepted | Request creates pending query |
| A-GO-014 | Invalid action `render` rejected | Error: "render deferred to future release" |
| A-GO-015 | Invalid action `bundle` rejected | Error: "bundle deferred to future release" |
| A-GO-016 | Invalid action `invalid_xyz` rejected | Error: "unknown_action" with valid values list |
| A-GO-017 | Empty action rejected | Error: "action required" |
| A-GO-018 | Null arguments rejected | Error: "invalid arguments" |
| A-GO-019 | tab_id as string "0" coerced to int | Request succeeds |
| A-GO-020 | force_refresh defaults to false | Pending query has force_refresh=false |

### 3. Scope Validation

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-GO-030 | `audit` with scope `accessibility` | Valid |
| A-GO-031 | `audit` with scope `performance` | Valid |
| A-GO-032 | `audit` with scope `full` | Valid |
| A-GO-033 | `audit` with invalid scope | Error with valid scope list |
| A-GO-034 | `memory` with scope `snapshot` | Valid |
| A-GO-035 | `memory` with scope `compare` | Valid |
| A-GO-036 | `security` with no scope (defaults to full) | Valid, scope set to "full" |
| A-GO-037 | `regression` with scope `baseline` | Valid |
| A-GO-038 | `regression` with scope `compare` | Valid |
| A-GO-039 | `regression` with scope `clear` | Valid |

### 4. Pending Query Handling

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-GO-050 | Analyze request creates pending query | Query appears in GetPendingQueries() |
| A-GO-051 | Correlation ID is UUID format | Matches UUID regex pattern |
| A-GO-052 | Correlation IDs are unique | 100 requests = 100 unique IDs |
| A-GO-053 | Query type is "analyze" | Type field matches |
| A-GO-054 | Query includes action and scope | Params JSON contains both |
| A-GO-055 | Query timeout matches action | audit.accessibility = 15s, memory = 5s |
| A-GO-056 | Expired queries cleaned up | After timeout, query not in pending list |
| A-GO-057 | Max pending queries enforced (5) | 6th query evicts oldest |
| A-GO-058 | Per-tab mutex enforced | Second analyze on same tab queued |

### 5. Concurrency Pattern Selection

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-GO-060 | `audit.accessibility` uses async pattern | Returns immediately with correlation_id |
| A-GO-061 | `audit.full` uses async pattern | Returns immediately with correlation_id |
| A-GO-062 | `memory.snapshot` uses blocking pattern | Waits for result (WaitForResult) |
| A-GO-063 | `security.headers` uses blocking pattern | Waits for result |
| A-GO-064 | Blocking pattern respects 5s timeout | Returns timeout error after 5s |
| A-GO-065 | Async pattern returns pending status | status="pending", includes correlation_id |

### 6. Result Handling

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-GO-070 | Result correlated to correct query | Query ID matches |
| A-GO-071 | Result removes query from pending | GetPendingQueries() no longer includes it |
| A-GO-072 | Partial results handled | status="partial", warnings included |
| A-GO-073 | Error results formatted correctly | mcpStructuredError schema |
| A-GO-074 | WaitForResult returns result directly | No polling needed |
| A-GO-075 | Result cache expires after 10s | force_refresh bypasses cache |

### 7. Endpoint Tests

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-GO-080 | POST /analyze-result accepts valid result | 200 OK |
| A-GO-081 | POST /analyze-result with missing correlation_id | 400 Bad Request |
| A-GO-082 | POST /analyze-result body > 5MB | 413 Payload Too Large |
| A-GO-083 | GET /pending-queries includes analyze queries | Type "analyze" in response |

---

## Unit Tests: Extension

**Location:** `tests/extension/analyze*.test.js`

### 1. Dispatcher Routing (analyze.js)

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EXT-001 | Action `audit` routes to runAudit | Function called |
| A-EXT-002 | Action `memory` routes to runMemoryAnalysis | Function called |
| A-EXT-003 | Action `security` routes to runSecurityAnalysis | Function called |
| A-EXT-004 | Action `regression` routes to runRegressionAnalysis | Function called |
| A-EXT-005 | Unknown action returns error | { error: "unknown_action" } |

### 2. axe-core Lazy Loading (analyze-audit.js)

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EXT-020 | First audit triggers axe load | AXE_STATE changes UNLOADED → LOADING → LOADED |
| A-EXT-021 | Second audit reuses loaded axe | No second script injection |
| A-EXT-022 | AXE_STATE.status transitions correctly | State machine followed |
| A-EXT-023 | Load failure sets AXE_STATE to FAILED | status="FAILED", lastError set |
| A-EXT-024 | Retry after 5s on FAILED state | Second attempt after 5s |
| A-EXT-025 | Concurrent audit calls during LOADING | All wait for same loadPromise |
| A-EXT-026 | Script injected via chrome.runtime.getURL | Uses bundled axe.min.js |

### 3. axe-core Execution

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EXT-030 | axe.run() called with document | Default context |
| A-EXT-031 | axe.run() called with selector scope | { include: [selector] } |
| A-EXT-032 | Results transformed to Gasoline schema | findings[], summary{} |
| A-EXT-033 | Violations include affected elements | selector, html, fix |
| A-EXT-034 | WCAG reference included | { wcag: "1.4.3", level: "AA" } |
| A-EXT-035 | Nodes capped at 10 per violation | A11Y_MAX_NODES_PER_VIOLATION |
| A-EXT-036 | HTML snippets truncated | DOM_QUERY_MAX_HTML (200 chars) |

### 4. Memory Analysis (analyze-memory.js)

**Limitation:** Uses `performance.memory` (Chrome-only, deprecated, basic stats only). For full heap profiling, use DevTools.

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EXT-050 | Heap snapshot captures performance.memory | heap_used_mb, heap_total_mb |
| A-EXT-051 | performance.memory unavailable detected | error: "memory_api_unavailable" |
| A-EXT-052 | DOM node count captured | dom_node_count field populated |
| A-EXT-053 | Snapshot includes timestamp | ISO 8601 format |
| A-EXT-054 | Compare requires prior baseline | error: "no_baseline" if missing |
| A-EXT-055 | Compare calculates delta | heap_growth_mb, heap_growth_pct |
| A-EXT-056 | Findings generated for significant growth | severity: "high" if >25% growth |

### 5. Security Analysis (analyze-security.js)

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EXT-070 | Headers audit checks CSP | missing_csp finding if absent |
| A-EXT-071 | Headers audit checks HSTS | missing_hsts finding if absent |
| A-EXT-072 | Cookie flags checked | HttpOnly, Secure, SameSite |
| A-EXT-073 | Insecure cookie identified | severity: "medium" |
| A-EXT-074 | localStorage keys scanned | Patterns: auth, token, key, secret |
| A-EXT-075 | Sensitive key found | severity: "low", affected: ["auth_token"] |
| A-EXT-076 | localStorage VALUES never returned | Only key names in response |
| A-EXT-077 | Cookie values redacted | value="[REDACTED]" |

### 6. Regression Analysis (analyze-regression.js)

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EXT-090 | Baseline captures current state | Stored with timestamp, URL |
| A-EXT-091 | Compare against baseline | comparison.accessibility.new_issues |
| A-EXT-092 | LCP regression detected | lcp_delta_ms > 0, lcp_regression: true |
| A-EXT-093 | New accessibility issues flagged | new_issues array |
| A-EXT-094 | Clear removes baseline | Subsequent compare returns no_baseline |
| A-EXT-095 | verdict field populated | "regression_detected" or "no_regression" |

### 7. CSP Fallback (Tier 2)

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EXT-110 | Tier 1 failure detected as CSP error | isCSPError() returns true |
| A-EXT-111 | Tier 2 uses chrome.debugger.attach | debugger.attach called with tabId |
| A-EXT-112 | Debugger detached after execution | debugger.detach called in finally block |
| A-EXT-113 | DevTools open blocks Tier 2 | error: "axe_csp_blocked_devtools_open" |

---

## Integration Tests

**Location:** `tests/integration/analyze_test.go`

### End-to-End Flows

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-INT-001 | Accessibility audit happy path | Findings returned to AI |
| A-INT-002 | Performance audit happy path | Vitals returned to AI |
| A-INT-003 | Security audit happy path | Findings with severity |
| A-INT-004 | Memory snapshot happy path | Heap stats returned |
| A-INT-005 | Memory compare happy path | Delta calculated |
| A-INT-006 | Regression baseline happy path | Stored successfully |
| A-INT-007 | Regression compare happy path | Verdict returned |
| A-INT-008 | Scoped analysis (selector) | Only scoped elements |
| A-INT-009 | AI Web Pilot disabled | error: "ai_web_pilot_disabled" |
| A-INT-010 | Extension not connected | error: "extension_timeout" |

### Error Propagation

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-INT-020 | axe-core load failure reaches AI | Structured error with guidance |
| A-INT-021 | Analysis timeout reaches AI | status: "partial", warning included |
| A-INT-022 | CSP blocked reaches AI | error: "axe_csp_blocked" |
| A-INT-023 | Invalid selector reaches AI | error: "invalid_selector" |
| A-INT-024 | Page navigated mid-analysis | status: "partial", warning: "page_navigated" |

---

## Edge Case Tests

### 1. Resource Exhaustion

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-001 | Page with 10,000 DOM elements | Completes within timeout |
| A-EDGE-002 | Page with 50,000 DOM elements | Completes or timeout error |
| A-EDGE-003 | axe-core takes 30 seconds | Timeout after 30s |
| A-EDGE-004 | localStorage with 5MB of data | Audit completes, keys only returned |
| A-EDGE-005 | 1000+ cookies | Audit completes, flagged cookies listed |

### 2. Page State Changes

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-020 | Tab closed during analysis | Graceful error: "tab_closed" |
| A-EDGE-021 | Page navigates during analysis | Partial results with warning |
| A-EDGE-022 | Page reloads during analysis | error: "page_navigated" |

### 3. Concurrency Conflicts

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-040 | Two analyze requests same tab simultaneous | Second queued, both complete |
| A-EDGE-041 | Analyze + interact on same tab | Both complete (no conflict) |
| A-EDGE-042 | 10 rapid analyze requests | First 5 queued, rest rejected |

### 4. Invalid Selector Edge Cases

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-060 | Selector matches 0 elements | Summary shows 0 issues |
| A-EDGE-061 | Selector with special chars | Properly escaped or clear error |
| A-EDGE-062 | Selector for shadow DOM element | May not reach shadow DOM (documented) |
| A-EDGE-063 | Selector for iframe content | error: "cross_origin_frame" or audits |

### 5. CSP and Injection Failures

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-080 | Strict CSP blocks inline script | Tier 2 fallback attempted |
| A-EDGE-081 | CSP + DevTools open | error: "axe_csp_blocked_devtools_open" |
| A-EDGE-082 | chrome:// page (restricted) | error: "restricted_page" |
| A-EDGE-083 | about:blank page | Audit runs (minimal findings) |

### 6. Timeout Scenarios

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-120 | audit.accessibility at 14.9s | Completes successfully |
| A-EDGE-121 | audit.accessibility at 15.1s | Timeout error |
| A-EDGE-122 | memory.snapshot at 4.9s | Completes |
| A-EDGE-123 | memory.snapshot at 5.1s | Timeout error |

### 7. Cache Edge Cases

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-140 | Second audit within 10s | Returns cached result |
| A-EDGE-141 | Second audit at 10.1s | Fresh audit runs |
| A-EDGE-142 | force_refresh=true within 10s | Fresh audit runs |
| A-EDGE-143 | Different selector, same action | Fresh audit (not cached) |

### 8. Memory API Edge Cases

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-160 | Firefox (no performance.memory) | error: "memory_api_unavailable" |
| A-EDGE-161 | Safari (no performance.memory) | error: "memory_api_unavailable" |

### 9. Regression Analysis Edge Cases

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-EDGE-180 | Compare with no baseline | error: "no_baseline" |
| A-EDGE-181 | Baseline from different URL | Warning: "url_mismatch" |
| A-EDGE-182 | Clear non-existent baseline | Success (idempotent) |

---

## Performance Tests

### Benchmarks

| Test ID | Metric | Target | Page Complexity |
|---------|--------|--------|-----------------|
| A-PERF-001 | audit.accessibility duration | < 3s | Simple (50 elements) |
| A-PERF-002 | audit.accessibility duration | < 5s | Medium (500 elements) |
| A-PERF-003 | audit.accessibility duration | < 15s | Complex (5000 elements) |
| A-PERF-004 | memory.snapshot duration | < 2s | Any |
| A-PERF-005 | security.full duration | < 3s | Any |
| A-PERF-006 | axe-core initial load time | < 500ms | - |
| A-PERF-007 | Cached result retrieval | < 50ms | - |

### Load Scenarios

| Test ID | Scenario | Validation |
|---------|----------|------------|
| A-PERF-020 | 10 rapid audits | All complete, no memory leak |
| A-PERF-021 | 100 audits over 30 minutes | Stable memory, stable timing |
| A-PERF-022 | Main thread blocking | < 100ms during analysis |

---

## Security Tests

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| A-SEC-001 | AI Web Pilot toggle OFF | All analyze calls rejected |
| A-SEC-002 | Cookie values in response | Values redacted |
| A-SEC-003 | localStorage values in response | Never returned, only keys |
| A-SEC-004 | Authorization headers in network data | Stripped entirely |
| A-SEC-005 | Password fields in DOM | Values redacted |
| A-SEC-006 | debugger permission required | Manifest includes permission |
| A-SEC-007 | Cross-origin frame access | Blocked, error returned |
| A-SEC-008 | Result data stays localhost | No external network calls |

---

## Browser Compatibility Tests

| Test ID | Browser | Expected Result |
|---------|---------|-----------------|
| A-COMPAT-001 | Chrome (latest) | Full functionality |
| A-COMPAT-002 | Chrome (latest - 1) | Full functionality |
| A-COMPAT-003 | Edge (Chromium) | Full functionality |
| A-COMPAT-004 | Firefox (MV3) | Audit works, memory unavailable |
| A-COMPAT-005 | Brave | Full functionality |

---

## Regression Testing Protocol

After any changes to analyze tool:

1. **Quick Smoke Test** (2 min)
   ```bash
   go test -short ./cmd/dev-console/... -run Analyze
   node --test tests/extension/analyze*.test.js
   ```

2. **Full Test Suite** (10 min)
   ```bash
   make test
   node --test tests/extension/*.test.js
   ```

3. **Integration Verification** (5 min)
   - Start server: `./dist/gasoline --port 7890`
   - Open test page with known a11y issues
   - Run: `analyze({action: 'audit', scope: 'accessibility'})`
   - Verify findings match expected

4. **Performance Regression Check**
   - Compare audit times to baseline
   - Check memory usage before/after
