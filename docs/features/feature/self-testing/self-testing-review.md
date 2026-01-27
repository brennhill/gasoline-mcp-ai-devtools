# Review: AI Self-Testing Infrastructure (tech-spec-self-testing.md)

## Executive Summary

This spec defines three features -- HTTP GET endpoints for captured data, an HTTP-accessible MCP endpoint, and a Playwright UAT harness -- that together enable AI agents to self-verify Gasoline's behavior. The GET endpoints and HTTP MCP bridge are sound, low-risk additions that fill real gaps. The Playwright harness introduces external dependencies (@playwright/test, @anthropic-ai/sdk) that conflict with the project's zero-dependency philosophy, and the spec underspecifies concurrency safety for the new read endpoints.

## Critical Issues (Must Fix Before Implementation)

### 1. HTTP MCP Endpoint Creates an Unauthenticated RPC Gateway

**Section:** "Feature 33: HTTP MCP Endpoint"

The `/mcp` endpoint exposes the entire MCP tool surface over HTTP with no authentication. The spec says "Optional API key if Feature 20 implemented" -- but Feature 20 does not exist yet, meaning the initial implementation has zero auth.

While the server binds to localhost, any process on the machine can now invoke any MCP tool via HTTP. This includes `execute_javascript` (arbitrary JS execution in the browser), `browser_action` (navigation control), and `manage_state` (page state manipulation). A malicious local process, a compromised npm script running in a terminal, or a browser-based SSRF attack targeting `localhost:7890/mcp` could invoke these tools.

The existing stdio MCP interface requires the caller to be the direct parent process (the AI agent spawns Gasoline). The HTTP endpoint removes this boundary.

**Fix:** Implement API key authentication before shipping the `/mcp` endpoint. Generate a random key at server startup, print it to stderr (visible to the spawning process), and require `Authorization: Bearer <key>` on all `/mcp` requests. This is a single afternoon of work and closes the most obvious attack vector.

Additionally, the AI Web Pilot tools (`execute_javascript`, `browser_action`) should be gated behind both the API key AND the extension toggle. The HTTP endpoint should not bypass the toggle check.

### 2. GET Endpoints Expose Ring Buffer Under Read Lock Without Snapshot

**Section:** "Feature 32: HTTP GET for Captured Data"

The implementation sketch shows:

```go
entries := h.capture.GetActions(limit, since)
```

The `Capture` struct uses `sync.RWMutex` for all shared state. The GET endpoints will take a read lock, iterate the ring buffer, filter by `since`, and serialize to JSON. During this time, POST endpoints from the extension are blocked (they need a write lock). If the JSON serialization is slow (1000 entries, complex objects), extension POST requests will queue up, causing the extension's background.js to timeout and potentially lose data.

The existing MCP tools avoid this because stdio is single-threaded -- one tool call at a time. HTTP endpoints are concurrent. Multiple simultaneous GET requests (e.g., a test script polling in a loop) could starve extension writes.

**Fix:** Copy the relevant slice under the lock, release the lock, then serialize. The pattern should be:

```go
func (v *Capture) GetActions(limit int, since int64) []EnhancedAction {
    v.mu.RLock()
    // Copy slice under lock
    snapshot := make([]EnhancedAction, len(v.enhancedActions))
    copy(snapshot, v.enhancedActions)
    v.mu.RUnlock()
    // Filter and limit outside lock
    return filterActions(snapshot, limit, since)
}
```

This is the standard pattern used throughout the codebase (see `GetWebSocketEvents` in `websocket.go`). The spec should mandate it for all five new endpoints.

### 3. Playwright Harness Requires `headless: false` -- Breaks CI

**Section:** "Feature 34: AI-Runnable Playwright Harness" -- Extension Loading

The spec correctly notes Chrome extensions require headed mode:

```javascript
headless: false, // Extensions require headed mode
```

This means the UAT harness cannot run in standard CI environments (GitHub Actions, GitLab CI, Docker) without a virtual display server (Xvfb). The spec does not mention this limitation. A developer running `node scripts/uat-runner.js` in CI will get a cryptic Chromium launch failure.

**Fix:** Document the headed-mode requirement prominently. Provide a `--ci` flag that uses Xvfb (on Linux) or skips extension-dependent tests. Better yet, split the test scenarios into two tiers:
- **Tier 1 (server-only):** Tests that only need the HTTP endpoints (`/captured/*`, `/mcp`). These run headless, no extension.
- **Tier 2 (full E2E):** Tests that require the extension. These require headed mode and are run locally or in CI with Xvfb.

### 4. `@anthropic-ai/sdk` Dependency is Unexplained and Likely Unnecessary

**Section:** "Feature 34" -- Dependencies

```json
{
  "devDependencies": {
    "@anthropic-ai/sdk": "^0.30.0",
    "@playwright/test": "^1.40.0"
  }
}
```

The spec lists the Anthropic SDK as a dependency but never explains what it is used for. The harness architecture (build server, spawn, launch browser, test, report) does not require the Anthropic SDK. If the intent is for the AI to call the harness and interpret results, the AI already has the Gasoline MCP tools -- it does not need its own SDK in the test harness.

**Fix:** Remove `@anthropic-ai/sdk` from the dependency list. If there is a genuine use case (e.g., the harness sends test results to an Anthropic API), document it. Otherwise, this is a phantom dependency that adds 10MB+ to `node_modules` for no reason.

### 5. No Rate Limiting on GET Endpoints

**Section:** "Feature 32" -- Security

The spec says GET endpoints are "localhost-only" and inherit existing host permissions. But there is no rate limiting. A test script that polls `GET /captured/network` in a tight loop (e.g., waiting for a specific request to appear) will saturate the server with read locks.

The existing rate limiting (`rate_limit.go`) applies to POST ingestion from the extension. GET endpoints need their own protection.

**Fix:** Apply a simple per-endpoint rate limit: max 10 requests/second per endpoint. Return `429 Too Rapidly` if exceeded. This prevents polling abuse while allowing reasonable test automation.

## Recommendations (Should Consider)

### 1. `GET /captured/*` Response Should Include Buffer Metadata

The response format includes `count`, `oldest_ts`, `newest_ts`, and `truncated`. Add `total_captured` (total entries ever ingested, not just current buffer contents) and `buffer_capacity` (max entries). This lets the caller know if they are seeing all data or just the latest window.

### 2. JSONL Format is Specified But Not Justified

The spec supports `format=jsonl` but the use case is unclear. JSONL is useful for streaming large datasets, but these endpoints return bounded ring buffer contents (max 1000 entries for logs, 100 for network). JSON array is simpler for consumers. If JSONL is kept, use `Transfer-Encoding: chunked` for true streaming rather than buffering the entire response.

### 3. The `/mcp` Endpoint Should Support Batch Requests

JSON-RPC 2.0 specifies batch requests (array of request objects). The implementation sketch only handles single requests. For test automation, batch support eliminates per-request HTTP overhead (important when running 45+ test scenarios).

### 4. Test Fixture Page Should Be Served by Gasoline, Not File Protocol

The spec shows a static HTML file at `scripts/fixtures/test-page.html`. Playwright can load this via `file://`, but the extension's content script may not inject on `file://` URLs (Chrome requires explicit permission). The Gasoline server should serve the fixture page at `GET /test-fixture` (dev builds only) so the extension captures from it normally.

### 5. GasolineAPI Client Swallows Errors

**Section:** "Feature 34" -- API Client

```javascript
return JSON.parse(json.result?.content?.[0]?.text || '{}');
```

This silently returns `{}` for any MCP error, tool not found, or malformed response. Test verification will pass with empty objects instead of failing with informative errors.

**Fix:** Check `json.error` first. If present, throw with the error message. If `result` is missing or malformed, throw with the raw response for debugging.

## Implementation Roadmap

1. **Feature 32: GET endpoints** (1 day): Implement all five GET handlers with copy-under-lock pattern. Add `total_captured` and `buffer_capacity` to response. Add per-endpoint rate limiting (10 req/s).

2. **Feature 33: HTTP MCP endpoint** (1 day): Implement `/mcp` POST handler that routes to existing MCP handler. Add startup-generated API key with `Authorization: Bearer` requirement. Ensure AI Web Pilot toggle is still enforced for pilot tools.

3. **Feature 33 tests** (0.5 days): Test all MCP methods via HTTP. Test auth rejection. Test pilot tool toggle enforcement via HTTP path.

4. **Feature 34: Playwright harness** (2-3 days): Remove `@anthropic-ai/sdk` dependency. Split into Tier 1 (server-only, headless) and Tier 2 (extension, headed). Serve test fixture from Gasoline server. Fix GasolineAPI error handling.

5. **CI integration** (0.5 days): Add Tier 1 tests to `make test`. Document Tier 2 headed-mode requirement. Optionally add Xvfb-based Tier 2 to CI.

Total: ~5-6 days of implementation work.
