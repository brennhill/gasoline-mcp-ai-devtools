# Principal Architect Review

**Date:** 2026-01-28
**Reviewer:** Principal Software Architect (via Claude Opus 4.5)
**Scope:** Full codebase — Go server, Chrome Extension, MCP protocol, API design
**Branch:** `next` (commit `edb1899`)

## Executive Summary

Gasoline MCP is a well-architected system with strong fundamentals: the 4-tool MCP constraint keeps the API surface manageable, the structured error system with recovery hints is genuinely LLM-friendly, and memory management is multi-layered with sensible eviction policies. However, the codebase has accumulated complexity — the `observe` tool now dispatches to 25+ modes, JSON field naming is inconsistent (mixed camelCase and snake_case), and there are goroutine leak vectors in the query wait path. The most critical finding is the `WaitForResult` goroutine that holds `c.mu.Lock()` indefinitely if the timeout fires before the goroutine exits.

---

## API Design

### Strengths

1. **4-tool constraint is excellent** (`tools.go:1156-1167`). The `observe`/`generate`/`configure`/`interact` taxonomy maps cleanly to read/create/configure/act semantics. This prevents tool sprawl that confuses LLMs.

2. **Structured errors with recovery hints** (`tools.go:348-389`). Every error includes `error` (machine code), `message` (human), `retry` (action instruction), and optional `hint`. The format `"Error: missing_param -- Add the 'what' parameter and call again"` plus JSON body gives LLMs both a readable summary and structured data. This is among the best error patterns I have seen for MCP tools.

3. **`_meta.data_counts` in tool schemas** (`tools.go:662-677`). Exposing live buffer counts (errors, logs, network_bodies, etc.) in the tool listing means the LLM knows what data exists before making a call. This eliminates wasted round-trips.

4. **Consistent dispatch pattern** (`tools.go:1173-1256`). Each composite tool follows the same shape: parse args, validate required param, switch on mode, return response. Predictable for maintenance and testing.

5. **Dual response format strategy** (`tools.go:186-220`). `mcpMarkdownResponse` for flat tabular data (errors, logs, actions) and `mcpJSONResponse` for nested data (websocket_status, vitals, page). This matches how LLMs parse data most effectively.

### Issues

1. **Observe mode sprawl: 25 modes** (`tools.go:684`). The `what` enum has grown to 25 values: `errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, api, accessibility, changes, timeline, error_clusters, history, security_audit, third_party_audit, security_diff, command_result, pending_commands, failed_commands`. While the 4-tool constraint is correct, the observe tool is becoming a god-function. The description alone is ~2000 characters. An LLM reading this enum will struggle to pick the right mode.

2. **Parameter explosion in observe schema** (`tools.go:686-813`). The observe tool accepts ~30 parameters, most applicable to only 1-2 modes. Parameters like `checks`, `first_party_origins`, `custom_lists`, `compare_from`, `compare_to` are highly mode-specific. This creates an opaque schema where an LLM cannot tell which parameters apply to which mode without parsing the description strings.

3. **Inconsistent "action" parameter naming** (`tools.go:942` vs `tools.go:1107`). Both `configure` and `interact` use `action` as their dispatch parameter, but `observe` uses `what` and `generate` uses `format`. This asymmetry is minor but forces the LLM to remember different dispatch keys per tool. Consider standardizing to `mode` across all four tools.

4. **configure has a stale enum** (`tools.go:1338`). The error hint lists `capture, record_event` as valid actions, but the `enum` in the schema (`tools.go:944`) does not include `capture` or `record_event`. Meanwhile the switch at `tools.go:1353-1355` handles them. The schema enum and the actual dispatch are out of sync.

5. **Tool description lengths** (`tools.go:660-661`, `tools.go:817-819`, `tools.go:935-937`, `tools.go:1100-1102`). Each tool description is 1000-2000 characters, totaling roughly 6000+ characters of tool descriptions. This consumes significant context window for every MCP interaction. The anti-pattern examples in each description, while helpful, could be moved to a resource endpoint instead.

### LLM-Friendliness Findings

- **Good:** Error codes are self-describing snake_case strings (`invalid_json`, `missing_param`, `extension_timeout`). An LLM can pattern-match on these without documentation.
- **Good:** Empty result responses include actionable hints: "Network body capture is OFF. To enable, call: configure({action: 'capture', settings: {network_bodies: 'true'}})" (`network.go:217-218`). This teaches the LLM how to fix the state.
- **Concern:** The `observe` response format varies dramatically by mode: markdown tables for `errors/logs/actions`, JSON for `vitals/page/websocket_status`, plain text for empty results. An LLM cannot predict the response shape without knowing the mode.
- **Concern:** The word "action" is overloaded: `configure({action: "capture"})` vs `interact({action: "execute_js"})` vs `EnhancedAction.Type` vs `noise_action` vs `store_action` vs `session_action` vs `streaming_action`. Seven different uses of "action" in the API surface.

---

## Data Structures

### Strengths

1. **Dual-lock architecture** (`types.go:484` and `types.go:540`). Main `mu sync.RWMutex` for capture data and separate `resultsMu sync.RWMutex` for async command results. This avoids contention between the hot ingest path and the async command polling path. Well-designed.

2. **Parallel timestamp tracking** (`types.go:490-491`, `495-496`, `499-500`). Each ring buffer has a companion `[]time.Time` array (`wsAddedAt`, `networkAddedAt`, `actionAddedAt`) enabling TTL-based filtering at read time without modifying the buffer. This is an elegant approach to time-windowed eviction.

3. **Composed sub-structs** (`types.go:565-573`). Breaking `A11yCache`, `PerformanceStore`, `MemoryState`, `SessionTracker`, `SchemaStore`, `CSPGenerator` into named sub-structs keeps the Capture type organized despite its 50+ fields.

4. **Reasonable buffer sizes** (`types.go:400-429`). 500 WS events, 100 network bodies, 50 actions, 1000 waterfall entries. Memory limits: 4MB WS, 8MB network bodies, 50MB hard cap, 100MB critical. These are well-tuned for a developer tools context.

### Issues

1. **Mixed JSON field naming conventions** (`types.go`). The codebase mixes camelCase and snake_case extensively:
   - camelCase: `initiatorType`, `startTime`, `fetchStart`, `responseEnd`, `transferSize`, `decodedBodySize`, `encodedBodySize`, `pageURL`, `openedAt`, `closedAt`, `closeCode`, `messageRate`, `lastMessage`, `perSecond`, `requestBody`, `responseBody`, `contentType`, `inputType`, `fromUrl`, `toUrl`
   - snake_case: `binary_format`, `format_confidence`, `session_id`, `client_id`, `request_body`, `response_body`, `response_status`, `duration_ms`, `tab_id`, `correlation_id`, `completed_at`, `created_at`, `detected_at`, `delta_ms`, `size_bytes`

   This inconsistency means an LLM consuming responses cannot predict field names. The pattern appears to be: extension-origin types use camelCase (matching JavaScript convention) while server-side types use snake_case. However, `NetworkBody` mixes both (`requestBody` camelCase, `binary_format` snake_case) in the same struct (`types.go:191-206`).

2. **Capture struct is too large** (`types.go:483-574`). At ~90 lines and 50+ fields, the Capture struct has become a monolith. Fields like `currentTestID`, `trackingEnabled`, `trackedTabID`, `trackedTabURL`, `trackingUpdated`, `pollingLog`, `pollingLogIndex`, `httpDebugLog`, `httpDebugLogIndex`, `extensionSession`, `sessionChangedAt`, `pilotEnabled`, `pilotUpdatedAt` are all under the same mutex despite being logically independent. This increases contention.

3. **A11yCache uses slice for LRU order tracking** (`types.go:439`). The `cacheOrder []string` field requires O(n) operations for removal and deduplication (`queries.go:762-769`). With `maxA11yCacheEntries = 10` this is acceptable, but the pattern does not scale.

4. **`connectionState` uses string timestamps** (`types.go:368`). Fields `openedAt` and `lastAt` in `directionStats` are strings, not `time.Time`. This means timestamp comparison requires parsing on every access. The choice was likely made for JSON serialization, but it forces string formatting in hot paths.

---

## Performance

### Strengths

1. **Two-phase injection** (`inject.js:1079-1111`). Phase 1 installs lightweight API + PerformanceObservers immediately. Phase 2 (heavy interceptors: console, fetch, WS, error handlers) defers until after page load + 100ms settling time. This directly addresses the "extension must not degrade browsing" requirement.

2. **Memory enforcement on every ingest** (`memory.go:149`). `enforceMemory()` is called at the top of `AddNetworkBodies`, `AddEnhancedActions`, and presumably `AddWSEvents`. This means memory pressure is checked with every data addition, not just on periodic timers. The periodic timer (`memory.go:335-350`) is a safety net.

3. **Single-pass eviction** (`network.go:92-110`). `evictNBForMemory` calculates how many entries to drop in one pass to avoid O(n^2) re-scanning. Good.

4. **Bounded observer goroutines** (`network.go:71-87`). The `observeSem` channel (capacity 4 at `types.go:515`) caps concurrent schema/CSP observer goroutines. The `default` branch drops observations under load rather than accumulating goroutines.

5. **Efficient circular buffers for debug logs** (`queries.go:26-28`, `queries.go:33-36`). Fixed-size 50-entry circular buffers for polling and HTTP debug logs avoid allocation per entry.

### Issues

1. **`calcWSMemory` and `calcNBMemory` are O(n) on every ingest** (`memory.go:80-105`). Both iterate the entire buffer to sum sizes. With 500 WS events and 100 network bodies this is fast, but it is called from `enforceMemory()` which runs on every ingest. Under high throughput (1000 events/sec), this adds up. A running total maintained on add/evict would be O(1).

2. **`evictBuffers` calls `calcTotalMemory` multiple times** (`memory.go:213`, `memory.go:233`). After evicting network bodies, it re-calculates total memory before deciding whether to evict WS events. Each call is O(n). This could be optimized to maintain a running total.

3. **`setA11yCacheEntry` allocates a new slice on every call** (`queries.go:762-769`). The deduplication loop creates `newOrder` from scratch every time a cache entry is set. This is a minor allocation but the pattern could use a proper LRU container.

4. **WS event rate calculation iterates `recentTimes` slice** (`types.go:379`). The `recentTimes []time.Time` field is used for rolling-window rate calculation, which requires trimming timestamps outside the window on every calculation. No cap on slice growth between trims is visible in the type definition.

5. **Extension fetch wrapper clones response for error bodies** (`inject.js:218`). On every non-ok fetch response, `response.clone()` is called and the full body read. For APIs that legitimately return 4xx (e.g., validation errors), this doubles memory usage per response. The `MAX_RESPONSE_LENGTH` truncation mitigates this, but the clone still buffers the full response before truncation.

---

## Correctness

### Strengths

1. **Comprehensive test suite** — 58 test files covering: race conditions (`race_test.go`), rate limiting (`rate_limit_test.go`), memory management (`memory_test.go`), multi-client isolation (`multi_client_test.go`), TTL filtering (`ttl_test.go`), security boundaries (`security_boundary_test.go`), API schema conformance (`api_schema_test.go`), temporal graphs (`temporal_graph_test.go`), and end-to-end checkpoint tests (`ai_checkpoint_e2e_test.go`). This is an exceptionally thorough test suite for a project of this size.

2. **Race condition build tag** (`race_test.go`). The `//go:build race` tag enables race-specific test behavior, showing the team runs the race detector in CI.

3. **Query result isolation** (`queries.go:220-242`). The `GetQueryResult` and `WaitForResult` functions enforce strict client ID isolation: a result belonging to client A cannot be retrieved by client B, and legacy results (no client ID) cannot be retrieved by new-style clients. This prevents cross-client data leakage.

4. **Atomic setQueryResultIfExists** (`queries.go:496-538`). Single lock acquisition to check existence, store result, and remove from pending. Eliminates TOCTOU race.

5. **Content script origin validation** (`content.js:99-100`). `event.source === window` check prevents cross-frame message injection. The tab tracking system (`content.js:19-50`) ensures only the explicitly tracked tab sends data to the server.

### Issues

1. **Goroutine leak in `WaitForResult`** (`queries.go:253-312`). When the timeout fires (`queries.go:309`), the outer `select` returns, but the inner goroutine (`queries.go:258`) is still blocked on `c.queryCond.Wait()` at line 299 while holding `c.mu.Lock()`. The goroutine will remain alive until either: (a) a matching result arrives and wakes it via `Broadcast()`, or (b) the program exits. The ticker goroutine (`queries.go:263`) also continues running because `done` channel is never closed from the timeout path.

   **Impact:** Under sustained timeout conditions (e.g., extension disconnected), each timed-out WaitForResult leaves behind a goroutine holding a read-intention on the mutex. The ticker goroutine also leaks, broadcasting every 100ms forever.

2. **`generateCorrelationID` uses weak randomness** (`queries.go:853-854`). The correlation ID format is `corr-<timestamp-millis>-<lower-32-bits-of-UnixNano>`. Both `time.Now().UnixMilli()` and `time.Now().UnixNano()` are called separately, creating a TOCTOU gap where the two timestamps may differ. More importantly, the "random" component (`UnixNano()&0xFFFFFFFF`) is not random at all -- it is just the lower 32 bits of the nanosecond clock. Two calls within the same nanosecond produce the same ID. Use `crypto/rand` or `math/rand` for the random component.

3. **`cleanExpiredQueries` goroutine spawned per query** (`queries.go:132-141`). Every call to `CreatePendingQueryWithClient` spawns a cleanup goroutine that sleeps for `timeout + 1s`. With 5 concurrent queries, that is 5 goroutines sleeping. The goroutine acquires `c.mu.Lock()` after waking, which means 5 goroutines contend for the lock simultaneously. A single periodic cleanup timer would be more efficient.

4. **`startResultCleanup` ticker never stops** (`queries.go:933-970`). The ticker-based goroutine runs forever with no stop mechanism. If the `Capture` instance is garbage collected while the goroutine is running, it creates a reference cycle preventing collection. The `StartMemoryEnforcement` function (`memory.go:335-349`) correctly returns a stop function, but `startResultCleanup` does not.

5. **Silent JSON unmarshal errors in tool handlers** (`queries.go:554`, `queries.go:657`, `actions.go:108`, `network.go:186`). Multiple tool handlers use `_ = json.Unmarshal(args, &arguments)` with a comment "Optional args - zero values are acceptable defaults." While this is intentional, it means a typo in parameter names (e.g., `lmit` instead of `limit`) is silently ignored. The LLM gets no feedback that its parameter was wrong.

6. **Stale pending query not cleaned when result arrives after timeout** (`queries.go:186-212`). `SetQueryResult` removes the query from `pendingQueries` and stores in `queryResults`. But if the timeout already fired in `WaitForResult`, the result sits in `queryResults` until the 60-second cleanup sweep (`queries.go:164-168`). This is technically correct but wastes memory for up to 60 seconds.

---

## Security

### Strengths

1. **DNS rebinding protection** (`main.go:845-905`). Three-layer defense: Host header validation (rejects non-localhost hosts), Origin validation (rejects non-local, non-extension origins), and CORS echo (never uses wildcard `*`). This is a solid implementation of the MCP security spec requirements (H-2/H-3).

2. **Server binds to 127.0.0.1 only** (`main.go:2001`). `net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))` ensures the server is not accessible from the network. This is the correct approach.

3. **MaxBytesReader on all ingest endpoints** (`helpers.go:61`, `queries.go:447`, `extension_logs.go:30`, `main.go:593`). Every POST endpoint applies `http.MaxBytesReader(w, r.Body, maxPostBodySize)` (5MB limit) before reading the body. This prevents memory exhaustion from oversized requests.

4. **File write path traversal protection** (`export_sarif.go:300-333`). The `saveSARIFToFile` function resolves symlinks via `filepath.EvalSymlinks`, checks that the resolved path is under CWD or temp directory, and rejects all other paths. This correctly prevents `../../etc/passwd` style attacks.

5. **Password redaction on ingest** (`actions.go:32-33`). Password input values are redacted to `[redacted]` at capture time, before storage. This means even if buffer data is leaked, passwords are never stored.

6. **Auth header stripping** (`queries.go:335-336`). HTTP debug logging redacts headers containing "auth" or "token" in their name.

7. **Rate limiting with circuit breaker** (`rate_limit.go:57`, `helpers.go:56-68`). Both per-second rate limiting (1000 events/sec threshold) and a circuit breaker pattern with exponential backoff protect against DoS from a misbehaving extension or malicious page.

### Issues

1. **Three `postMessage` calls use `'*'` as target origin** (`extension/lib/perf-snapshot.js:260`, `extension/lib/reproduction.js:239`, `extension/inject.js:1265`). While most `postMessage` calls correctly use `window.location.origin`, three use `'*'`. Since these messages are sent within the same window (inject.js to content.js), the risk is limited -- a malicious iframe listening for `GASOLINE_PERFORMANCE_SNAPSHOT`, `GASOLINE_ENHANCED_ACTION`, or `GASOLINE_HIGHLIGHT_RESPONSE` could intercept performance data, user action data, and highlight bounding box coordinates. The content script validates `event.source === window`, which partially mitigates this, but the inconsistency should be fixed.

2. **`executeJavaScript` uses `new Function()` in page context** (`inject.js:643`). This is inherently a powerful capability -- the AI Web Pilot can execute arbitrary JavaScript on any tracked page. The security model relies entirely on: (a) the pilot toggle being disabled by default, (b) the user explicitly enabling it. However, there is no additional sandboxing or CSP enforcement once enabled. A malicious LLM provider could instruct the AI to execute exfiltration scripts. The pilot gate check (`pilot.go:77-145`) is the only defense.

3. **`isAllowedOrigin` returns `true` for any `chrome-extension://` origin** (`main.go:831`). The check `strings.HasPrefix(origin, "chrome-extension://")` accepts requests from ANY Chrome extension, not just the Gasoline MCP extension. A malicious extension could make requests to the Gasoline MCP server if it knows the port. To fix this, validate the extension ID in the origin matches the expected Gasoline MCP extension ID.

4. **Empty Origin header accepted** (`main.go:826-828`). `isAllowedOrigin` returns `true` for empty origin, which is correct for CLI/curl usage but also allows requests from contexts that strip Origin headers (e.g., some browser-based tools, server-side requests that have been proxied). Combined with the Host header check, this is acceptable but worth documenting explicitly.

5. **`HandleSettings` and `HandleExtensionStatus` accept any POST from localhost** (`main.go:1638`, `main.go:1553`). These endpoints update server state (pilot toggle, tracking status) and are protected only by the CORS middleware. Any local process that knows the port can toggle the AI Web Pilot on or call other mutation endpoints. There is no authentication or authorization beyond localhost restriction.

6. **Screenshot data URLs decoded without size validation** (`main.go:593-621`). While `MaxBytesReader` limits the POST body to 5MB, the base64-decoded image data could be up to ~3.75MB written to disk. The file is written with `0644` permissions. The filename includes a sanitized hostname and timestamp, but the `sanitizeForFilename` function should be verified for completeness.

---

## Recommendations (Priority Ordered)

### 1. [Critical] Fix `WaitForResult` goroutine leak
**File:** `cmd/dev-console/queries.go:253-312`
The goroutine holding `c.mu.Lock()` cannot be cancelled when the timeout fires. This leaks goroutines and mutex waiters under sustained timeout conditions. Use `context.Context` with cancellation, or restructure to release the lock before the select.

### 2. [High] Fix `generateCorrelationID` weak randomness
**File:** `cmd/dev-console/queries.go:853-854`
Replace `time.Now().UnixNano()&0xFFFFFFFF` with `crypto/rand` or `math/rand/v2`. The current implementation can produce duplicate IDs when called in rapid succession.

### 3. [High] Standardize JSON field naming convention
**File:** `cmd/dev-console/types.go` (throughout)
Choose either camelCase or snake_case for all JSON field names and apply consistently. The current mix of conventions (camelCase for extension-origin data, snake_case for server-side) creates parsing ambiguity for LLMs. Recommendation: adopt snake_case for all MCP tool responses, as it is the JSON convention used by the MCP protocol itself. This would be a breaking change requiring a version bump.

### 4. [High] Restrict `chrome-extension://` origin validation
**File:** `cmd/dev-console/main.go:831`
Validate the specific Gasoline MCP extension ID rather than accepting any `chrome-extension://` origin. The extension ID can be configured via environment variable for development builds.

### 5. [High] Fix `postMessage` target origins
**Files:** `extension/lib/perf-snapshot.js:260`, `extension/lib/reproduction.js:239`, `extension/inject.js:1265`
Change `'*'` to `window.location.origin` for consistency with all other `postMessage` calls in the codebase.

### 6. [Medium] Add `startResultCleanup` stop mechanism
**File:** `cmd/dev-console/queries.go:933-970`
Return a stop function (like `StartMemoryEnforcement` does) to prevent goroutine/reference leaks.

### 7. [Medium] Optimize `calcWSMemory`/`calcNBMemory` to O(1)
**Files:** `cmd/dev-console/memory.go:80-105`
Maintain running memory totals that are updated on add/evict, eliminating the O(n) sum on every ingest call.

### 8. [Medium] Reduce `observe` tool description size
**File:** `cmd/dev-console/tools.go:660-661`
Move anti-pattern examples and detailed mode documentation to MCP resources (accessible via `resources/read`). Keep tool descriptions under 500 characters focusing on the mode enum and parameter list.

### 9. [Medium] Consolidate per-query cleanup goroutines
**File:** `cmd/dev-console/queries.go:132-141`
Replace per-query goroutine spawning with a single periodic cleanup timer (similar to `startResultCleanup`). This eliminates goroutine churn and lock contention from concurrent cleanup goroutines.

### 10. [Low] Standardize dispatch parameter naming
**File:** `cmd/dev-console/tools.go`
Consider renaming all dispatch parameters to `mode` across the four tools: `observe({mode: "errors"})`, `generate({mode: "test"})`, `configure({mode: "noise_rule"})`, `interact({mode: "execute_js"})`. This is a breaking API change but improves consistency.

### 11. [Low] Add unknown parameter warnings
**Files:** Various tool handlers
Instead of silently ignoring `_ = json.Unmarshal(args, &arguments)`, consider validating that provided parameter names are recognized and returning a warning for unknown parameters. This helps LLMs catch typos.

### 12. [Low] Fix `configure` schema/dispatch mismatch
**File:** `cmd/dev-console/tools.go:944`, `tools.go:1341-1371`
Add `capture` and `record_event` to the schema enum, or remove them from the dispatch switch. The current mismatch means an LLM using the schema cannot discover these actions.
