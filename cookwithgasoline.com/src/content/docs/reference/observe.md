---
title: "observe() — Read Browser State"
description: "Complete reference for the observe tool. 23 modes for reading errors, network traffic, WebSocket messages, user actions, recordings, and more."
---

The `observe` tool reads the current browser state. It's your AI's eyes into the browser — errors, network traffic, WebSocket messages, performance metrics, visual state, and more.

**Always call `observe()` before `interact()` or `generate()`** to give the AI context about the current page.

## Quick Reference

```js
observe({what: "errors"})                    // Console errors
observe({what: "error_bundles"})             // Errors with full context
observe({what: "logs", min_level: "warn"})   // Console output
observe({what: "network_bodies", url: "/api", status_min: 400})  // Failed API calls
observe({what: "websocket_events", last_n: 10})                  // WebSocket messages
observe({what: "screenshot"})                // Page screenshot
observe({what: "vitals"})                    // Web Vitals (LCP, CLS, INP)
observe({what: "recordings"})               // Recording metadata
observe({what: "log_diff_report", original_id: "rec-1", replay_id: "rec-2"})  // Compare recordings
```

## Common Parameters

These parameters work across multiple modes:

| Parameter | Type | Description |
|-----------|------|-------------|
| `what` | string (required) | Which mode to use (see sections below) |
| `limit` | number | Maximum entries to return |
| `last_n` | number | Return only the last N items |
| `url` | string | Filter by URL substring |
| `after_cursor` | string | Backward pagination — entries older than cursor |
| `before_cursor` | string | Forward pagination — entries newer than cursor |
| `since_cursor` | string | All entries newer than cursor (inclusive, no limit) |
| `restart_on_eviction` | boolean | Auto-restart if cursor expired from buffer overflow |

---

## Errors & Logs

### `errors`

Console errors with deduplication. The starting point for any debugging session.

```js
observe({what: "errors"})
```

Returns deduplicated errors with stack traces, source locations, and occurrence counts.

### `error_bundles`

Pre-assembled debugging context per error. Each bundle includes the error plus the network requests, user actions, and console logs that happened near it.

```js
observe({what: "error_bundles", window_seconds: 5})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `window_seconds` | number | 3 | How far back to look for related context (max 10) |

This is the most powerful debugging mode — it gives the AI a complete incident report instead of a bare stack trace.

### `logs`

All console output (log, warn, error, info, debug) with level filtering.

```js
observe({what: "logs", min_level: "warn", limit: 50})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `min_level` | string | Minimum level: `debug` < `log` < `info` < `warn` < `error` |
| `limit` | number | Maximum entries to return |

### `extension_logs`

Internal Gasoline extension debug logs. **Not** browser console output — use `logs` for that. Only useful for troubleshooting the Gasoline extension itself.

```js
observe({what: "extension_logs"})
```

---

## Network

### `network_waterfall`

Resource timing data for all network requests (XHR, fetch, scripts, stylesheets, images). Shows URL, method, status, duration, and size.

```js
observe({what: "network_waterfall", limit: 30})
observe({what: "network_waterfall", url: "/api"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | number | Maximum entries |
| `url` | string | Filter by URL substring |

:::note
This captures **all** network requests. For request/response bodies, use `network_bodies` (which only captures `fetch()` calls).
:::

### `network_bodies`

Full request and response payloads for `fetch()` calls. Filtered by URL, method, or status code.

```js
observe({what: "network_bodies", url: "/api/users", status_min: 400})
observe({what: "network_bodies", method: "POST", limit: 5})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | Filter by URL substring |
| `method` | string | Filter by HTTP method (GET, POST, etc.) |
| `status_min` | number | Minimum status code (e.g., 400 for errors only) |
| `status_max` | number | Maximum status code |
| `limit` | number | Maximum entries |

**Buffer:** 100 recent requests, 8 MB total memory. Auth headers are always stripped.

<!-- Screenshot: Example network_bodies output showing a failed API call with request and response payloads -->

---

## WebSocket

### `websocket_events`

Captured WebSocket messages and lifecycle events (open, close, error).

```js
observe({what: "websocket_events", last_n: 20})
observe({what: "websocket_events", connection_id: "ws-3", direction: "incoming"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `connection_id` | string | Filter by specific WebSocket connection |
| `direction` | string | `incoming` or `outgoing` |
| `last_n` | number | Return only the last N events |
| `limit` | number | Maximum entries |

**Buffer:** 500 events max, 4 MB limit. High-frequency streams are adaptively sampled.

### `websocket_status`

Live connection health — active connections, message rates, duration.

```js
observe({what: "websocket_status"})
```

Returns connection ID, URL, state (open/closed), duration, and message rates per direction.

---

## Page State

### `page`

Current URL and page title.

```js
observe({what: "page"})
```

### `tabs`

All browser tabs with their URLs and titles.

```js
observe({what: "tabs"})
```

### `screenshot`

Captures a screenshot of the current viewport. The AI receives the image and can reason about visual state — broken layouts, stuck spinners, hidden elements.

```js
observe({what: "screenshot"})
```

<!-- Screenshot: Example of observe screenshot output showing a web page capture -->

### `pilot`

Current state of AI Web Pilot — whether browser control is enabled or disabled.

```js
observe({what: "pilot"})
```

---

## User Actions

### `actions`

Recorded user interactions — clicks, typing, navigation, scrolling. Each action includes a timestamp, the element targeted, and the selector used.

```js
observe({what: "actions", last_n: 10})
observe({what: "actions", limit: 50})
```

Useful for understanding what the user did before an error occurred, or for generating reproduction scripts.

---

## Performance

### `vitals`

Core Web Vitals: LCP (Largest Contentful Paint), CLS (Cumulative Layout Shift), INP (Interaction to Next Paint), FCP (First Contentful Paint).

```js
observe({what: "vitals"})
```

Each metric includes the value, the rating (good/needs-improvement/poor), and thresholds.

---

## Recordings

### `saved_videos`

List recorded browser session videos.

```js
observe({what: "saved_videos"})
```

### `recordings`

Recording metadata — lists all recordings with their IDs, timestamps, and duration.

```js
observe({what: "recordings"})
```

### `recording_actions`

Actions captured during a specific recording session.

```js
observe({what: "recording_actions"})
```

### `playback_results`

Results from a recording playback session.

```js
observe({what: "playback_results"})
```

### `log_diff_report`

Compare error states between two recordings. Useful for verifying that a bug fix actually resolved the issue.

```js
observe({what: "log_diff_report", original_id: "rec-abc", replay_id: "rec-xyz"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `original_id` | string | Original recording ID (baseline) |
| `replay_id` | string | Replay recording ID (comparison) |

---

## Timeline & Aggregation

### `timeline`

Merged chronological view of all events — errors, network requests, user actions, console logs — interleaved by timestamp.

```js
observe({what: "timeline"})
observe({what: "timeline", include: ["errors", "network"]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `include` | array | Categories to include (e.g., `errors`, `network`, `actions`, `logs`) |

---

## Async Commands

### `command_result`

Retrieve the result of a previously-issued async command by correlation ID.

```js
observe({what: "command_result", correlation_id: "abc123"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `correlation_id` | string | The ID returned when the async command was issued |

### `pending_commands`

List commands that are still waiting for execution by the extension.

```js
observe({what: "pending_commands"})
```

### `failed_commands`

List commands that failed during execution.

```js
observe({what: "failed_commands"})
```

---

## Pagination

All buffer-backed modes support cursor-based pagination. The response includes a `metadata` object with cursor values:

```json
{
  "data": [...],
  "metadata": {
    "cursor": "1707325200:42",
    "has_more": true
  }
}
```

Pass the cursor back on the next request:

```js
// Get next page (older entries)
observe({what: "logs", after_cursor: "1707325200:42"})

// Get new entries since last check
observe({what: "errors", since_cursor: "1707325200:42"})
```

If the cursor expires (buffer overflowed since last read), set `restart_on_eviction: true` to automatically restart from the oldest available entry instead of erroring.
