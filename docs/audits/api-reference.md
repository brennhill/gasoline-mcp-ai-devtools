---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Gasoline MCP -- Complete API Reference

> Generated from source code audit on 2026-02-14.
> This document covers every API surface: MCP tools, HTTP endpoints, and extension-daemon communication.

---

## Table of Contents

1. [MCP Protocol (JSON-RPC 2.0)](#1-mcp-protocol-json-rpc-20)
2. [MCP Tools](#2-mcp-tools)
   - [observe](#21-observe)
   - [analyze](#22-analyze)
   - [generate](#23-generate)
   - [configure](#24-configure)
   - [interact](#25-interact)
3. [HTTP API Endpoints](#3-http-api-endpoints)
4. [Extension-to-Daemon Endpoints](#4-extension-to-daemon-endpoints)
5. [MCP Resources](#5-mcp-resources)

---

## 1. MCP Protocol (JSON-RPC 2.0)

### Transport

- **HTTP**: `POST /mcp` -- single endpoint for all MCP JSON-RPC calls
- **Stdio**: JSON-RPC over stdin/stdout (primary transport for MCP clients like Claude, Cursor)

### Protocol Version

Negotiated during `initialize`. Server supports `2024-11-05`.

### MCP Methods

| Method | Purpose |
|--------|---------|
| `initialize` | Handshake: negotiate protocol version, exchange capabilities |
| `initialized` | Client notification (no response) |
| `ping` | Keepalive (returns `{}`) |
| `tools/list` | List available tools and their schemas |
| `tools/call` | Invoke a tool by name with arguments |
| `resources/list` | List available resources |
| `resources/read` | Read a resource by URI |
| `resources/templates/list` | List resource templates (empty) |
| `prompts/list` | List prompts (empty) |
| `notifications/*` | Client notifications (no response per JSON-RPC 2.0 spec) |

### Rate Limiting

- **Tool calls**: 500 calls per minute (sliding window)
- **JSON-RPC error on exceeded**: code `-32603`, message "Tool call rate limit exceeded"

### Authentication

- **HTTP API key**: Optional `X-Gasoline-Key` header. When `GASOLINE_API_KEY` env is set, all HTTP requests must include the matching key. Returns `401 Unauthorized` on mismatch.
- **CORS**: Origin must be localhost, `chrome-extension://`, or `moz-extension://`. Returns `403 Forbidden` on invalid origin.
- **Host validation**: Host header must be a localhost variant (DNS rebinding protection). Returns `403 Forbidden` on invalid host.
- **Extension trust (TOFU)**: First-seen extension ID is paired and persisted. Subsequent connections must match. Override with `GASOLINE_EXTENSION_ID` or `GASOLINE_FIREFOX_EXTENSION_ID` env vars.

### Error Response Format

All MCP tool errors use a structured format:

```json
{
  "content": [{
    "type": "text",
    "text": "Error: error_code -- Retry instruction\n{\"error\":\"error_code\",\"message\":\"Human-readable message\",\"retry\":\"Action to take\",\"param\":\"field_name\",\"hint\":\"Additional context\"}"
  }],
  "isError": true
}
```

### Error Codes

| Code | Category | Description |
|------|----------|-------------|
| `invalid_json` | Input | Malformed JSON in request |
| `missing_param` | Input | Required parameter not provided |
| `invalid_param` | Input | Parameter value out of range or invalid |
| `unknown_mode` | Input | Unrecognized enum value for `what`/`action`/`format` |
| `path_not_allowed` | Input | File path outside allowed directory |
| `not_initialized` | State | Required subsystem not ready |
| `no_data` | State | No data available (buffer empty, command not found) |
| `pilot_disabled` | State | AI Web Pilot not enabled in extension |
| `os_automation_disabled` | State | OS upload automation flag not set |
| `rate_limited` | State | Too many requests |
| `cursor_expired` | State | Pagination cursor evicted from buffer |
| `extension_timeout` | Communication | Extension did not respond in time |
| `extension_error` | Communication | Extension reported an error |
| `internal_error` | Internal | Server bug (do not retry) |
| `marshal_failed` | Internal | JSON serialization failure |
| `export_failed` | Internal | File export operation failed |

---

## 2. MCP Tools

### 2.1 observe

**Purpose**: Read captured browser state from extension buffers.

**Required parameter**: `what` (string)

#### Observe Modes

##### observe({what: "errors"})

Read browser console errors.

| Parameter | Type | Required | Default | Validation | Description |
|-----------|------|----------|---------|------------|-------------|
| what | string | yes | -- | must be "errors" | Mode selector |
| limit | number | no | 100 | must be > 0 | Max entries to return |
| url | string | no | -- | substring match (case-insensitive) | Filter by URL |

**Success response**:
```json
{
  "errors": [{"message":"...","source":"...","url":"...","line":0,"column":0,"stack":"...","timestamp":"...","tab_id":0}],
  "count": 0,
  "metadata": {"retrieved_at":"...","is_stale":false,"data_age":"0.0s"}
}
```

**Error responses**:
| Condition | Error Code | Message |
|-----------|-----------|---------|
| Missing `what` | `missing_param` | "Required parameter 'what' is missing" |
| Unknown mode | `unknown_mode` | "Unknown observe mode: X" |

##### observe({what: "logs"})

Read browser console logs with cursor pagination.

| Parameter | Type | Required | Default | Validation | Description |
|-----------|------|----------|---------|------------|-------------|
| what | string | yes | -- | must be "logs" | Mode selector |
| limit | number | no | 100 | must be > 0 | Max entries per page |
| level | string | no | -- | exact match | Filter by exact log level |
| min_level | string | no | -- | enum: debug, log, info, warn, error | Minimum log level threshold |
| source | string | no | -- | exact match | Filter by source |
| url | string | no | -- | substring match (case-insensitive) | Filter by URL |
| after_cursor | string | no | -- | base64 cursor from previous response | Forward pagination |
| before_cursor | string | no | -- | base64 cursor from previous response | Backward pagination |
| since_cursor | string | no | -- | base64 cursor | All entries newer than cursor |
| restart_on_eviction | boolean | no | false | -- | Auto-restart if cursor expired |

**Success response**:
```json
{
  "logs": [{"level":"...","message":"...","source":"...","url":"...","line":0,"column":0,"timestamp":"...","tab_id":0}],
  "count": 0,
  "metadata": {"retrieved_at":"...","is_stale":false,"data_age":"0.0s","total":0,"has_more":false,"cursor":"..."}
}
```

**Error responses**:
| Condition | Error Code | Message |
|-----------|-----------|---------|
| Invalid cursor | `invalid_param` | Cursor format error |

##### observe({what: "extension_logs"})

Read internal extension debug logs.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| limit | number | no | 100 | Max entries |
| level | string | no | -- | Filter by level |

##### observe({what: "network_waterfall"})

Read all network requests (Performance API: all resource types).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| limit | number | no | 100 | Max entries |
| url | string | no | -- | URL substring filter |

**Success response**:
```json
{
  "entries": [{"url":"...","initiator_type":"...","duration_ms":0,"start_time":0,"transfer_size":0,"decoded_body_size":0,"encoded_body_size":0,"timestamp":"...","page_url":"..."}],
  "count": 0,
  "metadata": {"retrieved_at":"...","is_stale":false,"data_age":"..."}
}
```

##### observe({what: "network_bodies"})

Read captured fetch() request/response bodies.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| limit | number | no | 100 | Max entries |
| url | string | no | -- | URL substring filter |
| method | string | no | -- | HTTP method filter |
| status_min | number | no | -- | Min HTTP status code |
| status_max | number | no | -- | Max HTTP status code |

##### observe({what: "websocket_events"})

Read WebSocket message events.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| limit | number | no | 100 | Max entries |
| url | string | no | -- | URL filter |
| connection_id | string | no | -- | Filter by connection ID |
| direction | string | no | -- | "incoming" or "outgoing" |

##### observe({what: "websocket_status"})

Read WebSocket connection status.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| url | string | no | -- | URL filter |
| connection_id | string | no | -- | Filter by connection ID |

##### observe({what: "actions"})

Read captured user/AI actions (clicks, navigation, typing, etc.).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| limit | number | no | 100 | Max entries |
| url | string | no | -- | URL filter |

##### observe({what: "vitals"})

Read Web Vitals metrics (LCP, FCP, CLS, etc.).

No additional parameters.

##### observe({what: "page"})

Read current page URL and title.

No additional parameters.

##### observe({what: "tabs"})

Read tracked tab information.

No additional parameters.

##### observe({what: "pilot"})

Read AI Web Pilot status (enabled/disabled, extension connection).

No additional parameters. Server-side mode (no extension needed).

##### observe({what: "timeline"})

Read unified session timeline (actions + errors + network + WebSocket).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| limit | number | no | 50 | Max entries |
| include | string[] | no | all categories | Categories: "actions", "errors", "network", "websocket" |

##### observe({what: "error_bundles"})

Pre-assembled debugging context per error (error + network + actions + logs in a time window).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| limit | number | no | 5 | Max bundles |
| window_seconds | number | no | 3 | Lookback seconds per error (max 10) |
| url | string | no | -- | URL filter |

##### observe({what: "screenshot"})

Capture a screenshot of the tracked tab.

No additional parameters.

**Error responses**:
| Condition | Error Code | Message |
|-----------|-----------|---------|
| No tab tracked | `no_data` | "No tab is being tracked..." |
| Extension timeout | `extension_timeout` | "Screenshot capture timeout" |
| Extension error | `extension_error` | "Screenshot capture failed: ..." |

##### observe({what: "command_result"})

Retrieve the result of an async command.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| correlation_id | string | yes | -- | Correlation ID from previous async call |

**Notes**: For annotation commands (prefix `ann_`), blocks up to 55 seconds waiting for user to finish drawing.

**Error responses**:
| Condition | Error Code | Message |
|-----------|-----------|---------|
| Missing correlation_id | `missing_param` | "Required parameter 'correlation_id' is missing" |
| Command not found | `no_data` | "Command not found: ..." |
| Command expired | `extension_timeout` | "Command ... expired" |
| Command timed out | `extension_timeout` | "Command ... timed out" |

##### observe({what: "pending_commands"})

List all pending, completed, and failed async commands.

No additional parameters. Server-side mode.

##### observe({what: "failed_commands"})

List recent failed/expired async commands.

No additional parameters. Server-side mode.

##### observe({what: "saved_videos"})

List saved video recordings.

No additional parameters. Server-side mode.

##### observe({what: "recordings"})

List flow recordings.

No additional parameters. Server-side mode.

##### observe({what: "recording_actions"})

Get actions from a specific recording.

Server-side mode.

##### observe({what: "log_diff_report"})

Get log diff report.

Server-side mode.

#### Observe modes registered in handler map but NOT in schema enum

| Mode | Status |
|------|--------|
| `api` | Registered in handler, returns "not_implemented" |
| `changes` | Registered in handler, returns "not_implemented" |
| `playback_results` | Registered in handler, NOT in schema enum |

#### Side effects

- Extension disconnect warning prepended when extension is not connected (except server-side modes)
- Pending alerts appended as additional content block

---

### 2.2 analyze

**Purpose**: Trigger active analysis operations.

**Required parameter**: `what` (string)

#### Analyze Modes

##### analyze({what: "dom"})

Query DOM elements via CSS selector.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| selector | string | yes | -- | CSS selector |
| tab_id | number | no | active tab | Target tab |

Returns `correlation_id` for async polling.

##### analyze({what: "performance"})

Get performance snapshots.

No additional parameters.

##### analyze({what: "accessibility"})

Run accessibility audit.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| scope | string | no | -- | CSS selector scope |
| tags | string[] | no | -- | WCAG tags to check |
| force_refresh | boolean | no | false | Bypass cache |

**Error responses**:
| Condition | Error Code | Message |
|-----------|-----------|---------|
| No tab tracked | `no_data` | "No tab is being tracked..." |
| Extension timeout | `extension_timeout` | "A11y audit timeout: ..." |

##### analyze({what: "error_clusters"})

Cluster errors by message similarity.

No additional parameters.

##### analyze({what: "history"})

Analyze navigation history from actions.

No additional parameters.

##### analyze({what: "security_audit"})

Run security scan on captured network data.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| checks | string[] | no | all | Checks: credentials, pii, headers, cookies, transport, auth |
| severity_min | string | no | -- | Min severity: critical, high, medium, low, info |
| url | string | no | -- | URL filter |

##### analyze({what: "third_party_audit"})

Audit third-party dependencies.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| first_party_origins | string[] | no | -- | First-party origins |
| include_static | boolean | no | false | Include static-only origins |
| custom_lists | object | no | -- | Custom domain allow/block lists |

##### analyze({what: "security_diff"})

Compare security snapshots. Currently a stub returning empty differences.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| compare_from | string | no | -- | Baseline snapshot |
| compare_to | string | no | -- | Target snapshot |

##### analyze({what: "link_health"})

Initiate link health check on current page (async).

Returns `correlation_id` for polling.

##### analyze({what: "link_validation"})

Server-side link verification (HEAD/GET with SSRF-safe transport).

| Parameter | Type | Required | Default | Validation | Description |
|-----------|------|----------|---------|------------|-------------|
| urls | string[] | yes | -- | max 1000, must be http/https | URLs to validate |
| timeout_ms | number | no | 15000 | 1000-60000 | Per-URL timeout |
| max_workers | number | no | 20 | 1-100 | Concurrent workers |

##### analyze({what: "annotations"})

Get annotations from last draw mode session.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| wait | boolean | no | false | Block until annotations available |
| session | string | no | -- | Named session for multi-page review |
| timeout_ms | number | no | 300000 | Timeout for wait mode (max 600000) |

##### analyze({what: "annotation_detail"})

Get full computed styles and DOM detail for a specific annotation.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| correlation_id | string | yes | -- | Correlation ID from annotation |

#### Analyze modes registered in handler map but NOT in schema enum

| Mode | Status |
|------|--------|
| `api_validation` | Registered in handler, NOT in schema enum |
| `security_diff` | Registered in handler, NOT in schema enum |
| `draw_history` | Registered in handler, NOT in schema enum |
| `draw_session` | Registered in handler, NOT in schema enum |

---

### 2.3 generate

**Purpose**: Generate artifacts from captured data.

**Required parameter**: `format` (string)

#### Generate Formats

##### generate({format: "reproduction"})

Generate Playwright reproduction script from captured actions.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| error_message | string | no | -- | Error context |
| last_n | number | no | all | Use last N actions |
| base_url | string | no | -- | Replace origin in URLs |
| include_screenshots | boolean | no | false | Add screenshot calls |
| generate_fixtures | boolean | no | false | Generate network fixtures |
| visual_assertions | boolean | no | false | Add visual assertions |

##### generate({format: "test"})

Generate Playwright test from captured actions.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| test_name | string | no | "generated test" | Test name |
| last_n | number | no | all | Use last N actions |
| base_url | string | no | -- | Replace origin |
| assert_network | boolean | no | false | Assert no failed requests |
| assert_no_errors | boolean | no | false | Assert no console errors |
| assert_response_shape | boolean | no | false | Assert response shape |

##### generate({format: "csp"})

Generate Content Security Policy from observed network traffic.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| mode | string | no | "moderate" | strict, moderate, or report_only |
| include_report_uri | boolean | no | false | Include report-uri |
| exclude_origins | string[] | no | -- | Origins to exclude |

##### generate({format: "sarif"})

Export accessibility results as SARIF.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| scope | string | no | -- | CSS selector scope |
| include_passes | boolean | no | false | Include passing rules |
| save_to | string | no | -- | File path to save |

##### generate({format: "har"})

Export network traffic as HAR.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| url | string | no | -- | URL filter |
| method | string | no | -- | HTTP method filter |
| status_min | number | no | -- | Min status code |
| status_max | number | no | -- | Max status code |
| save_to | string | no | -- | File path to save |

##### generate({format: "sri"})

Generate Subresource Integrity hashes.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| resource_types | string[] | no | -- | script, stylesheet |
| origins | string[] | no | -- | Filter origins |

##### generate({format: "visual_test"})

Generate visual test from annotation session.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| test_name | string | no | -- | Test name |
| session | string | no | -- | Named annotation session |

##### generate({format: "annotation_report"})

Generate report from annotation session.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| session | string | no | -- | Named annotation session |

##### generate({format: "annotation_issues"})

Extract issues from annotation session.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| session | string | no | -- | Named annotation session |

##### generate({format: "test_from_context"})

Generate test from current browser context.

##### generate({format: "test_heal"})

Heal/fix a broken test.

##### generate({format: "test_classify"})

Classify test type.

#### Generate formats registered in handler map but NOT in schema enum

| Format | Status |
|--------|--------|
| `test` | Registered in handler, NOT in schema enum |
| `pr_summary` | Registered in handler, NOT in schema enum |
| `har` | Registered in handler, NOT in schema enum |
| `sri` | Registered in handler, NOT in schema enum |

---

### 2.4 configure

**Purpose**: Session settings and utilities.

**Required parameter**: `action` (string)

#### Configure Actions

##### configure({action: "health"})

Check server and extension health status.

No additional parameters.

**Success response**: `MCPHealthResponse` with server, memory, buffers, rate_limiting, audit, pilot sections.

##### configure({action: "clear"})

Reset capture buffers.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| buffer | string | no | "all" | Buffer to clear: all, network, websocket, actions, logs |

##### configure({action: "store"})

Persist session data.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| store_action | string | no | "list" | save, load, list, delete, stats |
| namespace | string | no | -- | Storage grouping |
| key | string | no | -- | Storage key |
| data | object | no | -- | JSON data to persist |

##### configure({action: "load"})

Load session context.

No additional parameters.

##### configure({action: "noise_rule"})

Manage console noise filtering.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| noise_action | string | no | "list" | add, remove, list, reset, auto_detect |
| rules | object[] | no | -- | Noise rules to add (with match_spec) |
| rule_id | string | no | -- | Rule ID to remove |
| pattern | string | no | -- | Regex pattern |
| category | string | no | -- | console, network, websocket |

##### configure({action: "streaming"})

Control push notification streaming.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| streaming_action | string | yes (mapped internally) | -- | enable, disable, status |
| events | string[] | no | ["all"] | Event categories to stream |
| throttle_seconds | integer | no | 5 | Min seconds between notifications (1-60) |
| severity_min | string | no | "warning" | Minimum severity: info, warning, error |

##### configure({action: "test_boundary_start"})

Start a test boundary.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| test_id | string | yes | -- | Test identifier |
| label | string | no | "Test: {test_id}" | Test label |

##### configure({action: "test_boundary_end"})

End a test boundary.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| test_id | string | yes | -- | Test identifier |

##### configure({action: "recording_start"})

Start a flow recording.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| name | string | no | -- | Recording name |
| url | string | no | "about:blank" | URL filter |
| sensitive_data_enabled | boolean | no | false | Include sensitive data |

##### configure({action: "recording_stop"})

Stop a flow recording.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| recording_id | string | no | -- | Recording to stop |

##### configure({action: "playback"})

Replay a recording.

##### configure({action: "log_diff"})

Compare error states over time.

#### Configure actions registered in handler map but NOT in schema enum

| Action | Status |
|--------|--------|
| `diff_sessions` | Registered in handler, NOT in schema enum |
| `audit_log` | Registered in handler, NOT in schema enum |

---

### 2.5 interact

**Purpose**: Browser automation actions. Requires AI Web Pilot extension.

**Required parameter**: `action` (string)

#### Interact Actions

##### interact({action: "navigate"})

Navigate to a URL.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| url | string | yes | -- | Target URL |
| tab_id | number | no | active tab | Target tab ID |

Returns `correlation_id`. Auto-includes `perf_diff`.

##### interact({action: "refresh"})

Refresh the current page.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| tab_id | number | no | active tab | Target tab ID |

Returns `correlation_id`. Auto-includes `perf_diff`.

##### interact({action: "back"})

Navigate back.

Requires Pilot enabled.

##### interact({action: "forward"})

Navigate forward.

Requires Pilot enabled.

##### interact({action: "new_tab"})

Open a new tab.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| url | string | no | -- | URL to open |

##### interact({action: "execute_js"})

Execute JavaScript in the page.

| Parameter | Type | Required | Default | Validation | Description |
|-----------|------|----------|---------|------------|-------------|
| script | string | yes | -- | non-empty | JS code to execute |
| timeout_ms | number | no | 5000 | -- | Execution timeout |
| tab_id | number | no | active tab | -- | Target tab |
| world | string | no | "auto" | auto, main, isolated | JS execution world |

##### interact({action: "highlight"})

Highlight an element with visual indicator.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| selector | string | yes | -- | CSS or semantic selector |
| duration_ms | number | no | 5000 | Highlight duration |
| tab_id | number | no | active tab | Target tab |

##### interact({action: "subtitle"})

Show subtitle text overlay in the browser.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| text | string | yes (can be empty to clear) | -- | Subtitle text |

##### DOM Primitive Actions

All DOM actions share these parameters:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| selector | string | yes | -- | CSS or semantic selector |
| tab_id | number | no | active tab | Target tab |
| timeout_ms | number | no | 5000 | Action timeout |
| analyze | boolean | no | false | Enable perf profiling |

**click**: Click an element. No additional params.

**type**: Type text into an element.
| Parameter | Type | Required |
|-----------|------|----------|
| text | string | yes |
| clear | boolean | no (default false) |

**select**: Select an option.
| Parameter | Type | Required |
|-----------|------|----------|
| value | string | yes |

**check**: Check/uncheck a checkbox.
| Parameter | Type | Required |
|-----------|------|----------|
| checked | boolean | no (default true) |

**get_text**: Get element text content. No additional params.

**get_value**: Get element value. No additional params.

**get_attribute**: Get element attribute.
| Parameter | Type | Required |
|-----------|------|----------|
| name | string | yes |

**set_attribute**: Set element attribute.
| Parameter | Type | Required |
|-----------|------|----------|
| name | string | yes |
| value | string | no |

**focus**: Focus an element. No additional params.

**scroll_to**: Scroll to an element. No additional params.

**wait_for**: Wait for an element to appear. No additional params.

**key_press**: Press a key (text param accepts key names: Enter, Tab, Escape, Backspace, ArrowDown, ArrowUp, Space).

##### interact({action: "list_interactive"})

Discover interactive elements on the page.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| tab_id | number | no | active tab | Target tab |

##### interact({action: "save_state"})

Save current page state as a named snapshot.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| snapshot_name | string | yes | -- | State name |

##### interact({action: "load_state"})

Load a previously saved state.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| snapshot_name | string | yes | -- | State name |
| include_url | boolean | no | false | Navigate to saved URL |

##### interact({action: "list_states"})

List all saved states.

No additional parameters.

##### interact({action: "delete_state"})

Delete a saved state.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| snapshot_name | string | yes | -- | State name |

##### interact({action: "record_start"})

Start screen recording.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| name | string | no | -- | Recording name |
| audio | string | no | -- | Audio mode: tab, mic, both |
| fps | number | no | 15 | FPS (5-60) |

##### interact({action: "record_stop"})

Stop screen recording.

##### interact({action: "upload"})

Upload a file to a form input.

##### interact({action: "draw_mode_start"})

Activate annotation overlay for user drawing.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| session | string | no | -- | Named session for multi-page review |

#### Common error for all Pilot-required actions

| Condition | Error Code | Message |
|-----------|-----------|---------|
| Pilot disabled | `pilot_disabled` | "AI Web Pilot is disabled" |

#### Composable subtitle

Any interact action accepts an optional `subtitle` parameter that queues a subtitle display as a side effect.

---

## 3. HTTP API Endpoints

### GET /

API discovery root.

**Auth**: None (CORS validated)

**Response (200)**:
```json
{"name": "gasoline", "version": "...", "health": "/health", "logs": "/logs"}
```

**Response (404)**: For any path other than `/`:
```json
{"error": "Not found"}
```

---

### GET /health

Server health check.

**Auth**: None (CORS validated)

**Response (200)**:
```json
{
  "status": "ok",
  "version": "...",
  "logs": {"entries": 0, "max_entries": 1000, "log_file": "...", "log_file_size": 0, "dropped_count": 0},
  "available_version": "...",
  "capture": {"available": true, "pilot_enabled": false, "extension_connected": false, "extension_last_seen": "...", "extension_client_id": "..."}
}
```

---

### GET /diagnostics

Debug diagnostics for bug reports.

**Auth**: None (CORS validated)

**Response (200)**: Detailed system info including version, uptime, OS, Go version, goroutines, buffers, WebSocket connections, extension status, circuit breaker state, HTTP debug log.

---

### POST /mcp

MCP JSON-RPC 2.0 endpoint.

**Auth**: CORS validated. Rate limited (500 calls/min).

**Request**: JSON-RPC 2.0 body.

**Response**: JSON-RPC 2.0 response. 204 No Content for notifications.

---

### GET /openapi.json

Embedded OpenAPI 3.1.0 specification.

**Auth**: None (CORS validated)

---

### POST /logs

Ingest log entries from extension.

**Auth**: Extension only (`X-Gasoline-Client` header required)

**Request body**:
```json
{"entries": [{"level": "error", "message": "...", "source": "...", "ts": "...", "url": "..."}]}
```

**Response (200)**:
```json
{"received": 0, "rejected": 0, "entries": 0}
```

**Errors**: 400 (Invalid JSON, Missing entries array), 405 (Method not allowed)

### DELETE /logs

Clear all log entries.

**Auth**: Extension only

**Response (200)**: `{"cleared": true}`

---

### POST /screenshots

Save screenshot from extension.

**Auth**: Extension only

**Rate limit**: 1 per second per client

**Request body**:
```json
{"data_url": "data:image/jpeg;base64,...", "url": "...", "correlation_id": "...", "query_id": "..."}
```

**Response (200)**: `{"filename": "...", "path": "...", "correlation_id": "..."}`

**Errors**: 400 (Invalid JSON, Missing/invalid data URL), 405, 429 (rate limited), 500, 503 (rate limiter capacity)

---

### POST /draw-mode/complete

Receive annotation data from draw mode session.

**Auth**: Extension only

**Request body**:
```json
{
  "screenshot_data_url": "data:...",
  "annotations": [...],
  "element_details": {...},
  "page_url": "...",
  "tab_id": 0,
  "session_name": "...",
  "correlation_id": "..."
}
```

**Validation**: `tab_id` must be > 0.

**Response (200)**:
```json
{"status": "stored", "annotation_count": 0, "screenshot": "...", "warnings": [...]}
```

---

### POST /shutdown

Initiate graceful shutdown via SIGTERM.

**Auth**: Extension only

**Response (200)**: `{"status": "shutting_down", "message": "Server shutdown initiated"}`

---

## 4. Extension-to-Daemon Endpoints

All endpoints require `X-Gasoline-Client` header with value `gasoline-extension`, `gasoline-extension/{version}`, or `gasoline-extension-offscreen`.

### Telemetry Ingestion

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/websocket-events` | POST | WebSocket event ingestion |
| `/websocket-status` | POST | WebSocket connection status |
| `/network-bodies` | POST | Network request/response bodies |
| `/network-waterfall` | POST | Performance API network entries |
| `/enhanced-actions` | POST | User action tracking |
| `/performance-snapshots` | POST | Performance timing snapshots |
| `/query-result` | POST | Async query result delivery |
| `/sync` | POST | Unified sync endpoint |

### Client Registry

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/clients` | GET | List registered clients |
| `/clients` | POST | Register a new client |
| `/clients/{id}` | GET | Get client by ID |
| `/clients/{id}` | DELETE | Unregister client |

### Recording

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/recordings/save` | POST | Save video recording binary |
| `/recordings/storage` | -- | Recording storage management |
| `/recordings/reveal` | POST | Open recording in OS file manager |

### CI / Testing

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/snapshot` | -- | CI snapshot management |
| `/clear` | -- | Buffer clearing |
| `/test-boundary` | -- | Test boundary markers |
| `/telemetry` | GET | Unified telemetry read |

### Upload Automation Stages

| Endpoint | Method | Stage | Auth | Description |
|----------|--------|-------|------|-------------|
| `/api/file/read` | POST | 1 | Extension only | Read file metadata + base64 content |
| `/api/file/dialog/inject` | POST | 2 | Extension only | File dialog injection |
| `/api/form/submit` | POST | 3 | Extension only | Form submission helper |
| `/api/os-automation/inject` | POST | 4 | Extension only + `--enable-os-upload-automation` flag | OS-level file dialog automation |
| `/api/os-automation/dismiss` | POST | 4 | Extension only + `--enable-os-upload-automation` flag | Dismiss dangling file dialog |

---

## 5. MCP Resources

### gasoline://guide

**Name**: Gasoline Usage Guide

**MIME Type**: text/markdown

**Content**: Comprehensive usage guide with tool reference table, key patterns (pagination, async commands, error debugging), common workflows, and tips.

### gasoline://quickstart

**Name**: Gasoline MCP Quickstart

**MIME Type**: text/markdown

**Content**: Short canonical MCP call examples for health, observe, analyze, and recording workflows.

### Resource templates

**Template**: gasoline://demo/{name}

**Description**: Demo scripts for websockets, annotations, recording, and dependency vetting.
