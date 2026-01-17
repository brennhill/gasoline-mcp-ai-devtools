# QA & Architecture Review v2

**Reviewer:** Principal Engineer Agent (Opus 4.5)
**Date:** 2026-01-28
**Codebase:** Gasoline MCP v5.2 (branch: `next`, commit: `ca2472f`)
**Scope:** Full codebase — Go server (18 source files), Chrome Extension (3 JS files)

---

## Executive Summary

Gasoline MCP is a well-architected MCP server and browser extension with strong foundations: zero-dependency Go stdlib server, consistent ring buffer eviction, proper mutex-guarded concurrency, and thoughtful memory management with three-tier eviction. The 4-tool MCP constraint enforces clean API design.

This review identified **3 critical**, **6 high**, **11 medium**, and **8 low** severity issues across security, performance, memory, and cross-cutting concerns. The most impactful findings are:

1. **Path traversal gap in HAR export** — `isPathSafe()` lacks symlink resolution, unlike the SARIF export's `resolveExistingPath()`, creating an inconsistent security boundary.
2. **ReDoS risk in user-supplied regex patterns** — `ai_noise.go` compiles user-provided regex without complexity validation, enabling denial of service.
3. **Unescaped user input in generated test scripts** — `codegen.go` injects URL-derived `testName` directly into JavaScript `test()` calls without escaping.

Overall code quality is high. The patterns below are meant to harden an already solid codebase.

---

## Critical Issues

### C-1: HAR Export Path Traversal via Symlink (Security)

**File:** `cmd/dev-console/export_har.go:264-278`
**Category:** Security — Path Traversal
**Severity:** Critical

**Description:** The `isPathSafe()` function validates file paths using only `filepath.Clean()` and prefix checks, without resolving symlinks. An attacker-controlled MCP client could create a symlink at `/tmp/evil -> /etc/` and then request export to `/tmp/evil/passwd`, bypassing the path check.

In contrast, `export_sarif.go:282-294` implements `resolveExistingPath()` which calls `filepath.EvalSymlinks()` recursively through parent directories — a robust defense against symlink-based traversal.

```go
// export_har.go:264 — VULNERABLE: no symlink resolution
func isPathSafe(path string) bool {
    cleaned := filepath.Clean(path)
    if filepath.IsAbs(cleaned) {
        if strings.HasPrefix(cleaned, "/tmp") { return true }
        tmpDir := os.TempDir()
        return strings.HasPrefix(cleaned, tmpDir)
    }
    return !strings.Contains(cleaned, "..")
}

// export_sarif.go:282 — SECURE: resolves symlinks
func resolveExistingPath(path string) string {
    path = filepath.Clean(path)
    resolved, err := filepath.EvalSymlinks(path)
    if err == nil { return resolved }
    parent := filepath.Dir(path)
    if parent == path { return path }
    return filepath.Join(resolveExistingPath(parent), filepath.Base(path))
}
```

**Similar occurrences:** Only these two export paths exist. SARIF is already hardened.

**Recommended fix:** Replace `isPathSafe()` with a function that mirrors SARIF's `resolveExistingPath()` approach. Alternatively, extract the SARIF path validation into a shared `safepath` helper and use it in both exports.

---

### C-2: ReDoS via User-Supplied Regex in Noise Rules (Security)

**File:** `cmd/dev-console/ai_noise.go:509-524`
**Category:** Security — Denial of Service
**Severity:** Critical

**Description:** When users add custom noise rules via the `configure` MCP tool, the `MessageRegex`, `SourceRegex`, and `URLRegex` fields are compiled directly with `regexp.Compile()` without any complexity validation. A pathological regex like `(a+)+$` matched against `"aaaaaaaaaaaaaaaaaaaaaaaaaab"` can cause exponential backtracking, blocking the goroutine for minutes.

```go
// ai_noise.go:509
re, err := regexp.Compile(r.MatchSpec.MessageRegex) // No complexity check
```

Go's `regexp` package uses RE2 semantics which prevents catastrophic backtracking by design (linear-time matching). However, Go's `regexp` is still RE2-based and complex patterns with many alternations or large character classes can cause significant constant-factor slowdowns. The `AutoDetect()` method at line 767 also holds a write lock for the entire analysis + recompile cycle, amplifying latency.

**Similar occurrences:** Three regex fields per rule (lines 509, 515, 521). Auto-detected rules use `regexp.QuoteMeta()` (safe, lines 803, 839, 889).

**Recommended fix:** Add a regex complexity validator: limit pattern length (e.g., 512 chars), reject nested quantifiers, and add a compilation timeout. Since Go's RE2 prevents true exponential backtracking, the practical severity depends on input patterns, but defense-in-depth is warranted for a localhost service accepting MCP tool calls.

---

### C-3: Unescaped Test Name in Generated Playwright Scripts (Security)

**File:** `cmd/dev-console/codegen.go:408`
**Category:** Security — Code Injection in Generated Output
**Severity:** Critical

**Description:** The `generateTestScript()` function formats the test name directly into a JavaScript string literal without passing it through `escapeJSString()`. The `testName` is derived from a page URL (line 393-404), which is attacker-controlled content.

```go
// codegen.go:408
sb.WriteString(fmt.Sprintf("test('%s', async ({ page }) => {\n", testName))
```

If a page URL contains a single quote (e.g., `https://evil.com/page?name='); process.exit(1); //`), the generated Playwright test script would contain executable injected code. While the user must explicitly run the generated script, the generated output is presented as safe-to-execute.

The Go `escapeJSString()` function exists in the same file and properly handles `\`, `'`, `\n`, `\r`. It is used for other values but missed here.

**Similar occurrences:** The JavaScript version in `reproduction.js:350` does escape the test name: `test('${escapeString(testName)}', ...)`. This confirms the Go version is an oversight.

**Recommended fix:** Apply `escapeJSString()` to `testName`:
```go
sb.WriteString(fmt.Sprintf("test('%s', async ({ page }) => {\n", escapeJSString(testName)))
```

---

## High Issues

### H-1: restoreState() Sets Cookies/Storage from Untrusted State (Security)

**File:** `extension/inject.js:1309-1350`
**Category:** Security — Untrusted Data Injection
**Severity:** High

**Description:** The `restoreState()` function unconditionally clears `localStorage`, `sessionStorage`, and all cookies, then replaces them with values from a `state` object received via the MCP server. This state object is controlled by the MCP client (AI assistant).

```javascript
// inject.js:1311-1316
localStorage.clear()
sessionStorage.clear()
for (const [key, value] of Object.entries(state.localStorage || {})) {
    localStorage.setItem(key, value)
}
```

A compromised or malicious MCP client could inject arbitrary cookies (including session tokens), localStorage values (including auth tokens), and sessionStorage values. The function also navigates to `state.url` at line 1345-1347, potentially redirecting to a phishing page.

The function is gated behind the `VALID_STATE_ACTIONS` whitelist (line 740) and requires the pilot to be enabled, but the data within the state object itself is unvalidated.

**Similar occurrences:** Cookie restoration at line 1332-1336 directly sets `document.cookie` from the state.

**Recommended fix:** Add key/value sanitization — reject keys containing `__proto__`, `constructor`, etc. Consider a configurable allowlist of restorable storage keys. For cookies, validate against a domain allowlist. Add a confirmation step or warning in the MCP response noting that state was restored.

---

### H-2: Incomplete escapeString() in reproduction.js (Security)

**File:** `extension/lib/reproduction.js:405-407`
**Category:** Security — Incomplete Output Encoding
**Severity:** High

**Description:** The JavaScript `escapeString()` function only handles three escape sequences: `\`, `'`, and `\n`. It is missing `\r` (carriage return), `\t` (tab), backtick, and other control characters.

```javascript
// reproduction.js:405-407
function escapeString(str) {
  if (!str) return ''
  return str.replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/\n/g, '\\n')
}
```

This function is used extensively to build Playwright locator expressions (lines 317, 322, 334, 339, 350, 377, 381-396). User-entered text containing carriage returns or template literal backticks would produce syntactically broken or exploitable generated scripts.

The Go counterpart `escapeJSString()` in `codegen.go` properly handles `\r`, demonstrating awareness of this requirement.

**Similar occurrences:** All callers of `escapeString()` in `reproduction.js` are affected (14 call sites from lines 317-396).

**Recommended fix:** Extend `escapeString()` to match or exceed the Go version:
```javascript
function escapeString(str) {
  if (!str) return ''
  return str.replace(/\\/g, '\\\\')
    .replace(/'/g, "\\'")
    .replace(/\n/g, '\\n')
    .replace(/\r/g, '\\r')
    .replace(/\t/g, '\\t')
    .replace(/`/g, '\\`')
}
```

---

### H-3: Missing Body Size Limit on POST /api/extension-status (Security)

**File:** `cmd/dev-console/status.go:56`
**Category:** Security — Denial of Service
**Severity:** High

**Description:** The `HandleExtensionStatus()` endpoint uses `json.NewDecoder(r.Body).Decode()` directly without wrapping `r.Body` in `http.MaxBytesReader` or `io.LimitReader`. A malicious client could send a multi-gigabyte POST body, causing excessive memory allocation.

```go
// status.go:56
if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
```

Other endpoints in the codebase consistently apply body limits:
- `settings.go:134`: `io.LimitReader(r.Body, 1024*10)` (10KB)
- `helpers.go:56-60`: `readIngestBody()` applies `maxPostBodySize`

**Similar occurrences:** This is the only unprotected endpoint found. All ingest endpoints use `readIngestBody()`.

**Recommended fix:** Wrap the body with `http.MaxBytesReader`:
```go
r.Body = http.MaxBytesReader(w, r.Body, 10*1024) // 10KB limit
```

---

### H-4: AutoDetect() Holds Write Lock During Full Buffer Analysis (Performance)

**File:** `cmd/dev-console/ai_noise.go:767-768`
**Category:** Performance — Lock Contention
**Severity:** High

**Description:** `AutoDetect()` acquires a write lock (`nc.mu.Lock()`) at line 768 and holds it for the entire analysis of console entries, network bodies, and WebSocket events. This blocks all concurrent noise filter checks (`IsNoise()`, `IsNetworkNoise()`, `IsWSNoise()`) which acquire read locks on the same mutex.

During auto-detection, the function iterates all entries to count frequencies, creates proposals, and compiles new regex rules — all under the write lock. For large buffers (500+ WS events, 100+ network bodies), this could block noise filtering for tens of milliseconds.

**Similar occurrences:** `AddRules()` also holds a write lock during regex compilation but processes smaller batches.

**Recommended fix:** Perform analysis and frequency counting outside the write lock using copies of the rules slice. Only acquire the write lock when applying high-confidence proposals at the end.

---

### H-5: Dual Mutex Nesting in ai_noise.go (Concurrency)

**File:** `cmd/dev-console/ai_noise.go:698-711`
**Category:** Concurrency — Potential Deadlock
**Severity:** High

**Description:** The `recordMatch()` and `recordSignal()` methods acquire `nc.statsMu` and are called from filter methods (`IsNoise`, `IsNetworkNoise`, `IsWSNoise`) which hold `nc.mu.RLock`. This creates a nested lock ordering: `mu.RLock -> statsMu.Lock`.

Meanwhile, `GetStatistics()` at line 714 acquires only `statsMu`, and `AutoDetect()` acquires only `mu.Lock`. This ordering is consistent (mu always before statsMu), so there is no actual deadlock risk. However, if any future code path acquires `statsMu` then `mu`, a deadlock would occur.

**Recommended fix:** Document the lock ordering invariant `mu -> statsMu` in a comment on the `NoiseConfig` struct. Consider consolidating into a single mutex if the performance benefit of dual locks is not measured.

---

### H-6: ClearAll() Does Not Reset Performance Data (Memory)

**File:** `cmd/dev-console/ci.go:247-263`
**Category:** Memory — Incomplete Cleanup
**Severity:** High

**Description:** `ClearAll()` resets WebSocket events, network bodies, enhanced actions, and connection state, but does not reset the `perf` field (performance snapshots and baselines). In CI test isolation scenarios (the primary use case for `/clear`), stale performance data from a previous test run would persist and potentially corrupt regression detection for the next test.

```go
func (c *Capture) ClearAll() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.wsEvents = nil
    c.wsAddedAt = nil
    // ... other resets ...
    // MISSING: c.perf = performanceData{} or equivalent
}
```

**Similar occurrences:** `evictCritical()` in `memory.go` also uses nil assignment for cleanup but does not touch `perf`.

**Recommended fix:** Add `c.perf = performanceData{}` (or a targeted reset of `snapshots`, `baselines`, and `orderKeys`) to `ClearAll()`. If perf data should survive clears (by design), document this decision.

---

## Medium Issues

### M-1: O(n) Full Scan for GetConsoleErrors/GetConsoleWarnings (Performance)

**File:** `cmd/dev-console/tools.go:617-641`
**Category:** Performance — Algorithmic Complexity
**Severity:** Medium

**Description:** Both `GetConsoleErrors()` and `GetConsoleWarnings()` iterate the entire `server.entries` slice on every call, performing a string type assertion per entry. These are called from `computeDataCounts()` (line 705-727) which is called on every `tools/list` MCP request.

```go
func (a *captureStateAdapter) GetConsoleErrors() []SnapshotError {
    a.server.mu.RLock()
    defer a.server.mu.RUnlock()
    var errors []SnapshotError
    for _, entry := range a.server.entries {
        if level, _ := entry["level"].(string); level == "error" {
            // ...
        }
    }
    return errors
}
```

With the default log buffer at 1000 entries, this is two O(1000) scans per `tools/list` call.

**Similar occurrences:** `computeDataCounts()` at line 705 also scans `server.entries` for error counts inline (lines 708-712), duplicating the scan.

**Recommended fix:** Maintain running counters (`errorCount`, `warningCount`) that are updated on `addEntries()` and `evictEntries()`. This converts O(n) scans to O(1) lookups.

---

### M-2: toolsList() Rebuilds Tool Definitions on Every Call (Performance)

**File:** `cmd/dev-console/tools.go:729-731`
**Category:** Performance — Unnecessary Allocation
**Severity:** Medium

**Description:** `toolsList()` constructs the full array of 4 MCP tool definitions (~500 lines of schema) on every `tools/list` JSON-RPC call. The tool schemas are static; only the `data_counts` in `_meta` change. This allocates ~dozens of maps and slices per call.

```go
func (h *ToolHandler) toolsList() []MCPTool {
    errorCount, logCount, ... := h.computeDataCounts()
    return []MCPTool{ /* ~500 lines of schema construction */ }
}
```

**Similar occurrences:** None — this is the only tool list construction.

**Recommended fix:** Cache the tool definitions as a package-level `sync.Once` or struct field. On each call, only update the `_meta.data_counts` values. Alternatively, construct the static schema once and deep-copy only the `Meta` field per call.

---

### M-3: Sequential Lock Acquisition in computeDataCounts() (Performance)

**File:** `cmd/dev-console/tools.go:705-727`
**Category:** Performance — Lock Contention
**Severity:** Medium

**Description:** `computeDataCounts()` acquires `server.mu.RLock` then releases it, then acquires `capture.mu.RLock`. While this avoids deadlock (locks are never held simultaneously), it means the data counts are not atomically consistent — the log count could change between the two lock acquisitions.

```go
h.MCPHandler.server.mu.RLock()
logCount = len(h.MCPHandler.server.entries)
// ... scan for errors
h.MCPHandler.server.mu.RUnlock()

h.capture.mu.RLock()
networkCount = len(h.capture.networkBodies)
// ... other counts
h.capture.mu.RUnlock()
```

For informational `_meta` data this is acceptable, but worth documenting.

**Similar occurrences:** `toolSecurityAudit()` acquires multiple locks sequentially. `captureStateAdapter` methods acquire one lock at a time.

**Recommended fix:** Document the intentional non-atomicity with a comment. If atomic consistency is needed, consider a snapshot approach.

---

### M-4: appendAndPrune() Allocates New Slice Per WebSocket Message (Performance)

**File:** `cmd/dev-console/websocket.go:231-244`
**Category:** Performance — Allocation Pressure
**Severity:** Medium

**Description:** `appendAndPrune()` creates a new `[]time.Time` slice on every call, even when no pruning is needed. For high-frequency WebSocket connections (rate tracking), this generates garbage on every message.

```go
func appendAndPrune(times []time.Time, t time.Time) []time.Time {
    // ...
    surviving := make([]time.Time, len(times)-start) // always allocates
    copy(surviving, times[start:])
    // ...
}
```

**Similar occurrences:** The pattern of creating new slices on eviction is used consistently throughout the codebase (memory.go, network.go) and is good practice for GC. However, `appendAndPrune` is called per-message rather than per-eviction-cycle.

**Recommended fix:** Use in-place pruning when `start == 0` (no entries to prune): return `append(times, t)` directly. Only allocate a new slice when actually pruning.

---

### M-5: O(n) removeFromSlice on Every Snapshot Update (Performance)

**File:** `cmd/dev-console/helpers.go:41-51`
**Category:** Performance — Algorithmic Complexity
**Severity:** Medium

**Description:** `removeFromSlice()` performs a linear scan and allocates a new slice for every removal. It is used in performance.go for LRU eviction of snapshots/baselines and in websocket.go for connection close tracking.

```go
func removeFromSlice(slice []string, item string) []string {
    for i, v := range slice {
        if v == item {
            newSlice := make([]string, len(slice)-1)
            copy(newSlice, slice[:i])
            copy(newSlice[i:], slice[i+1:])
            return newSlice
        }
    }
    return slice
}
```

With `maxPerfSnapshots=20` and `maxPerfBaselines=20`, the O(n) cost is bounded but the allocation per removal is wasteful.

**Similar occurrences:** Used for `connOrder` in websocket.go on connection close. Used for `orderKeys` in performance.go on snapshot/baseline eviction.

**Recommended fix:** Use in-place removal with `copy(slice[i:], slice[i+1:])` and `slice[:len(slice)-1]` to avoid allocation. Alternatively, for ordered eviction, a `container/list` (doubly-linked list from stdlib) provides O(1) removal.

---

### M-6: Unbounded CLS Accumulation in perf-snapshot.js (Memory)

**File:** `extension/lib/perf-snapshot.js`
**Category:** Memory — Unbounded Growth
**Severity:** Medium

**Description:** The Cumulative Layout Shift (CLS) value is accumulated as `clsValue += entry.value` in the PerformanceObserver callback. This value is never reset between SPA navigations. For long-lived SPA sessions, CLS will monotonically increase and never reflect the current navigation's CLS.

This is technically correct per the Web Vitals specification (CLS is cumulative for the page lifecycle), but for SPA debugging purposes, the accumulated value loses diagnostic utility over time.

**Similar occurrences:** Other Web Vitals (LCP, FCP, TTFB) use `last-value` semantics and are naturally bounded.

**Recommended fix:** Track both cumulative and per-navigation CLS. Reset a `navigationCLS` counter on `popstate`/`pushState` events. Report both values in snapshots.

---

### M-7: isAllowedOrigin/isAllowedHost Return True for Empty Values (Security)

**File:** `cmd/dev-console/main.go:826-827, 861-862`
**Category:** Security — Permissive Default
**Severity:** Medium

**Description:** Both `isAllowedOrigin()` and `isAllowedHost()` return `true` when the respective header is empty. The comments document this as intentional (for CLI/curl and HTTP/1.0 clients). However, the `corsMiddleware` at line 896 only checks origin when present (`if origin != "" && !isAllowedOrigin(origin)`), meaning requests without an `Origin` header bypass origin validation entirely.

```go
func isAllowedOrigin(origin string) bool {
    if origin == "" { return true } // Intentional for CLI/curl
    // ...
}
```

This is appropriate for a localhost-only service where the Host header check (line 889) provides the primary DNS rebinding defense. The Origin check is a defense-in-depth layer. No fix needed, but the design decision should be documented.

**Similar occurrences:** The empty-Host case at line 861 is similarly documented as HTTP/1.0 compatibility.

**Recommended fix:** No code change needed. Add a security comment block at the top of `corsMiddleware` explaining the layered defense model and why empty origin/host are accepted.

---

### M-8: Pilot Delegate Methods Double-Marshal JSON (Performance)

**File:** `cmd/dev-console/pilot.go`
**Category:** Performance — Unnecessary Serialization
**Severity:** Medium

**Description:** Multiple pilot handler methods (e.g., `handlePilotManageStateSave`, `handlePilotHighlight`, `handlePilotExecuteJS`) unmarshal the incoming JSON arguments, then marshal a new JSON payload for the WebSocket message. This double-parse pattern is functionally correct but wastes CPU on JSON round-trips.

**Similar occurrences:** This pattern appears in approximately 6-8 pilot delegate methods.

**Recommended fix:** Where the outgoing payload structure matches the incoming one, pass `json.RawMessage` through directly instead of unmarshaling and remarshaling.

---

### M-9: GetSessionTimeline() Uses Insertion Sort O(n^2) (Performance)

**File:** `cmd/dev-console/codegen.go`
**Category:** Performance — Algorithmic Complexity
**Severity:** Medium

**Description:** `GetSessionTimeline()` builds a timeline from multiple sources (actions, network bodies, WebSocket events) and sorts them by timestamp. The sort implementation uses insertion into a sorted position, resulting in O(n^2) complexity.

The timeline is capped at 200 entries, so worst-case is 200^2 = 40,000 comparisons. This is not a practical performance issue at current scale but would not scale if the cap increases.

**Similar occurrences:** `filterTopResources()` in `performance.go` also uses insertion sort but is bounded by `maxResourceFingerprints=50`.

**Recommended fix:** Use `sort.Slice()` from stdlib for O(n log n) sorting. This is a one-line change and eliminates the scaling concern.

---

### M-10: reproduction.js Uses O(n) shift()/unshift() for Buffers (Performance)

**File:** `extension/lib/reproduction.js`
**Category:** Performance — Algorithmic Complexity
**Severity:** Medium

**Description:** The reproduction buffer uses `shift()` for eviction (removing oldest entry) and `computeCssPath()` uses `unshift()` for building CSS path segments. Both operations are O(n) for JavaScript arrays because they require re-indexing all elements.

For the buffer (capped at ~500 actions), `shift()` on eviction moves up to 499 elements. `computeCssPath()` builds paths by prepending segments, calling `unshift()` per DOM level (typically 5-15 levels).

**Similar occurrences:** None in the Go server (uses slice copying with explicit index management).

**Recommended fix:** For buffer eviction, track a start index instead of shifting. For CSS path building, use `push()` + `reverse()` at the end instead of `unshift()` per iteration.

---

### M-11: wrapFetch() Has Duplicated Header Filtering Code (Maintainability)

**File:** `extension/inject.js`
**Category:** Maintainability — Code Duplication
**Severity:** Medium

**Description:** The `wrapFetch()` interceptor contains header filtering logic duplicated in both the success path and the error/catch path. The sensitive header stripping logic (auth, token, cookie) appears in two places that must be kept in sync.

**Similar occurrences:** The header stripping pattern is also used in `settings.go:117-122` (server-side), but with different key detection logic.

**Recommended fix:** Extract a `filterSensitiveHeaders(headers)` helper function and call it from both paths. Align the key detection logic with the server-side version.

---

## Low Issues

### L-1: normalizeResourceURL() Does Manual URL Parsing (Maintainability)

**File:** `cmd/dev-console/performance.go`
**Category:** Maintainability — Reinvented Wheel
**Severity:** Low

**Description:** `normalizeResourceURL()` manually strips query strings and fragments from URLs using `strings.Index`. The Go stdlib `net/url.Parse()` handles this correctly and handles edge cases (e.g., encoded characters, fragments containing `?`).

**Similar occurrences:** `replaceOrigin()` in `codegen.go` also does manual URL parsing without `net/url`.

**Recommended fix:** Use `net/url.Parse()` + reconstruct without query/fragment.

---

### L-2: EnhancedAction Uses camelCase for inputType Field (Consistency)

**File:** `cmd/dev-console/types.go`
**Category:** Consistency — Naming Convention
**Severity:** Low

**Description:** The `EnhancedAction` struct uses `inputType` as a Go field name with a `json:"inputType"` tag. The JSON output uses camelCase (`inputType`) while the broader codebase convention for MCP tool output uses snake_case (`content_type`, `binary_format`, etc.).

**Similar occurrences:** Most other struct JSON tags use snake_case. HAR types use camelCase (correctly, per HAR 1.2 spec).

**Recommended fix:** Standardize to snake_case (`input_type`) in the JSON tag for MCP output. If backward compatibility is a concern, add both tags via a custom marshaler or accept the inconsistency with a comment.

---

### L-3: Settings Fire-and-Forget Disk Persistence (Error Handling)

**File:** `cmd/dev-console/settings.go:243-247`
**Category:** Reliability — Silent Failure
**Severity:** Low

**Description:** `HandleSettings()` persists settings to disk in a fire-and-forget goroutine. If the write fails, the error is only logged to stderr — the HTTP response has already been sent as 200 OK.

```go
go func() {
    if err := c.SaveSettingsToDisk(); err != nil {
        fmt.Fprintf(os.Stderr, "[gasoline] Failed to save settings to disk: %v\n", err)
    }
}()
```

For a settings cache that is only used for startup optimization (5s staleness check), this is acceptable. The settings are always re-sent by the extension on reconnect.

**Similar occurrences:** None — other file writes are synchronous.

**Recommended fix:** No code change needed. The current behavior is appropriate for the use case.

---

### L-4: filterTopResources() Uses Insertion Sort (Performance)

**File:** `cmd/dev-console/performance.go`
**Category:** Performance — Algorithmic Complexity
**Severity:** Low

**Description:** `filterTopResources()` uses insertion sort to find the top N resources by transfer size. Bounded by `maxResourceFingerprints=50`, so worst-case is 50^2 = 2,500 comparisons.

**Similar occurrences:** See M-9 (`GetSessionTimeline()`).

**Recommended fix:** No change needed at current scale. If `maxResourceFingerprints` increases, switch to `sort.Slice()`.

---

### L-5: calcRate() Iterates All Timestamps in Rate Window (Performance)

**File:** `cmd/dev-console/websocket.go:247`
**Category:** Performance — Algorithmic Complexity
**Severity:** Low

**Description:** `calcRate()` iterates all timestamps in the rate window to count recent events. The timestamps are chronologically ordered, so a binary search (`sort.Search`) could find the cutoff point in O(log n).

**Similar occurrences:** None.

**Recommended fix:** Use `sort.Search()` for the cutoff index. Practical impact is negligible at current buffer sizes.

---

### L-6: Extension Browser Permission Scope (Security)

**File:** `extension/manifest.json` (not directly reviewed)
**Category:** Security — Principle of Least Privilege
**Severity:** Low

**Description:** The Chrome extension operates as MV3 with content script injection. The `VALID_SETTINGS` whitelist (inject.js:728-738) properly restricts which settings can be modified via postMessage. The `VALID_STATE_ACTIONS` set (line 740) restricts state operations.

These are good security boundaries. The observation is that the extension's full permission set should be reviewed separately against the principle of least privilege.

**Recommended fix:** Audit the manifest.json permissions to ensure only necessary permissions are requested.

---

### L-7: Script Output Cap at 50KB (Robustness)

**File:** `cmd/dev-console/codegen.go`
**Category:** Robustness — Output Truncation
**Severity:** Low

**Description:** Generated Playwright scripts are capped at 50KB. For long session recordings with many actions, the script will be silently truncated. The user receives no indication that the generated script is incomplete.

**Recommended fix:** Add a comment at the truncation point: `// [Script truncated at 50KB — {remaining} actions omitted]`.

---

### L-8: JSON Encoder Error Ignored in GET /api/extension-status (Error Handling)

**File:** `cmd/dev-console/status.go:46`
**Category:** Error Handling — Ignored Error
**Severity:** Low

**Description:** `json.NewEncoder(w).Encode(status)` has its error suppressed with `//nolint:errcheck`. If the client disconnects mid-response, the error is silently dropped.

```go
json.NewEncoder(w).Encode(status) //nolint:errcheck
```

This is standard practice for HTTP handlers where the only possible error is a broken connection, which the handler cannot recover from anyway.

**Similar occurrences:** Line 70 has the same pattern.

**Recommended fix:** No code change needed. The `//nolint:errcheck` annotation is appropriate.

---

## Cross-cutting Patterns

### Pattern 1: Consistent Slice Eviction via New Allocation

**Files:** `memory.go`, `network.go`, `websocket.go`, `helpers.go`

The codebase consistently uses `make([]T, newLen)` + `copy()` for eviction rather than re-slicing (`slice = slice[n:]`). This is correct: re-slicing pins the underlying array in memory, preventing GC of evicted elements. The `evictCritical()` function in `memory.go` goes further with `= nil` assignment.

**Verdict:** Good pattern. No change needed.

### Pattern 2: Lock Ordering — server.mu before capture.mu

**Files:** `tools.go:706-723`, `tools.go` (toolSecurityAudit), `captureStateAdapter` methods

The observed lock ordering is: `server.mu.RLock()` -> release -> `capture.mu.RLock()`. Locks are never held simultaneously, preventing deadlocks but sacrificing atomic consistency. Within `ai_noise.go`, the ordering is `mu -> statsMu`.

**Verdict:** Safe but fragile. Document the ordering invariant.

### Pattern 3: Structured Error Responses with Retry Hints

**Files:** `tools.go` (all tool handlers)

MCP tool responses consistently use `mcpStructuredError(errCode, message, retryHint)` for error cases. This provides actionable information to AI assistants. The error codes are well-categorized (`ErrPathNotAllowed`, `ErrExportFailed`, etc.).

**Verdict:** Excellent pattern. Consistent across all handlers.

### Pattern 4: File I/O Outside Lock Scope

**Files:** `main.go` (addEntries), `settings.go` (SaveSettingsToDisk), `export_har.go`, `export_sarif.go`

File I/O operations are consistently performed outside mutex-held regions. `addEntries()` collects data under lock, releases, then writes. `SaveSettingsToDisk()` reads state under RLock, releases, then writes.

**Verdict:** Good pattern. Prevents I/O latency from inflating lock hold times.

### Pattern 5: Escape Function Inconsistency Between Go and JS

**Files:** `codegen.go` (Go `escapeJSString`), `reproduction.js` (JS `escapeString`)

The Go version escapes `\`, `'`, `\n`, `\r`. The JS version only escapes `\`, `'`, `\n`. Both are used to generate Playwright test scripts. This inconsistency means the same user action (e.g., typing text with `\r`) would produce different output depending on whether the server or extension generated the script.

**Verdict:** Bug. Align the JS version to match the Go version (see H-2).

### Pattern 6: Observer Backpressure via Semaphore + Default

**Files:** `network.go:76-91`

The `observeSem` channel (capacity 4) with `select { case sem <- ...: go func()... default: drop }` provides clean backpressure for schema inference and CSP generation. Excess observer notifications are dropped rather than queued, preventing goroutine accumulation.

**Verdict:** Good pattern. Well-bounded concurrency.

---

## Files Reviewed

| File | Lines | Category |
|---|---|---|
| `cmd/dev-console/main.go` | ~2060 | Entry point, HTTP server, CORS, routing |
| `cmd/dev-console/types.go` | ~771 | Shared types, Capture struct, constants |
| `cmd/dev-console/queries.go` | ~1012 | Query dispatch, async commands, a11y cache |
| `cmd/dev-console/memory.go` | ~361 | Three-tier memory eviction |
| `cmd/dev-console/network.go` | ~431 | Network body storage, binary detection |
| `cmd/dev-console/websocket.go` | ~491 | WebSocket tracking, rate limiting |
| `cmd/dev-console/settings.go` | ~251 | Settings persistence, disk cache |
| `cmd/dev-console/status.go` | ~82 | Extension status pings |
| `cmd/dev-console/pilot.go` | ~819 | AI Web Pilot handlers |
| `cmd/dev-console/tools.go` | ~2277 | MCP tool definitions and dispatch |
| `cmd/dev-console/codegen.go` | ~1105 | Playwright script generation |
| `cmd/dev-console/performance.go` | ~1018 | Web Vitals, baselines, regression |
| `cmd/dev-console/ai_noise.go` | ~969 | Noise filtering and auto-detection |
| `cmd/dev-console/export_har.go` | ~327 | HAR 1.2 export |
| `cmd/dev-console/export_sarif.go` | ~354 | SARIF 2.1.0 export |
| `cmd/dev-console/ci.go` | ~264 | CI endpoints, snapshot, clear |
| `cmd/dev-console/helpers.go` | ~60+ | Shared utilities |
| `extension/inject.js` | ~1363 | Main extension orchestration |
| `extension/lib/perf-snapshot.js` | ~276 | Performance snapshot collection |
| `extension/lib/reproduction.js` | ~409 | Reproduction recording and script gen |

**Total lines reviewed:** ~13,700+
