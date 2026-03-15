# Observe Tool Reference

Complete reference for all 31 observe modes available via the `observe` MCP tool.

## Universal Parameters

These parameters apply to **every** observe mode:

| Param | Type | Description |
|-------|------|-------------|
| `what` | string, **required** | The observe mode to invoke |
| `telemetry_mode` | `off` \| `auto` \| `full` | Controls telemetry collection level |
| `limit` | integer (default 100, max 1000) | Maximum number of results to return |
| `after_cursor` | string | Cursor for forward pagination |
| `before_cursor` | string | Cursor for backward pagination |
| `since_cursor` | string | Return only results newer than this cursor |
| `restart_on_eviction` | boolean | Restart streaming if the cursor was evicted |
| `summary` | boolean | Return a summarized response |

---

## errors
Browser console errors.
**Params:** url (string), scope (`current_page` | `all`), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"errors","scope":"current_page"}'
```

## logs
Browser console logs.
**Params:** min_level (`debug` | `log` | `info` | `warn` | `error`), source (string), include_internal (boolean), include_extension_logs (boolean), extension_limit (integer), url (string), scope (string)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"logs","min_level":"warn"}'
```

## extension_logs
Internal extension debug logs.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"extension_logs"}'
```

## network_waterfall
All network requests (HTTP, fetch, XHR).
**Params:** url (string), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"network_waterfall","url":"https://example.com"}'
```

## network_bodies
Fetch request/response bodies.
**Params:** url (string), method (string), status_min (integer), status_max (integer), body_path (string), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"network_bodies","method":"POST","status_min":400}'
```

## websocket_events
WebSocket messages.
**Params:** url (string), connection_id (string), direction (`incoming` | `outgoing`), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"websocket_events","direction":"incoming"}'
```

## websocket_status
WebSocket connection status.
**Params:** connection_id (string), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"websocket_status"}'
```

## actions
User actions recorded.
**Params:** url (string), last_n (integer), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"actions","last_n":10}'
```

## vitals
Web vitals metrics.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"vitals"}'
```

## page
Current page state.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"page"}'
```

## tabs
Open browser tabs.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"tabs"}'
```

## history
Browser history.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"history"}'
```

## pilot
Pilot mode data.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"pilot"}'
```

## timeline
Timeline events.
**Params:** include (array of categories), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"timeline","include":["network","console"]}'
```

## error_bundles
Pre-assembled error context.
**Params:** url (string), scope (string), window_seconds (integer, default 3, max 10), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"error_bundles","window_seconds":5}'
```

## screenshot
Page screenshots.
**Params:** format (`png` | `jpeg`), quality (integer, 1-100, jpeg only), full_page (boolean), selector (string), wait_for_stable (boolean), save_to (string)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"screenshot","format":"png","full_page":true}'
```

## storage
localStorage/sessionStorage/cookies.
**Params:** storage_type (`local` | `session` | `cookies`), key (string), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"storage","storage_type":"local","key":"auth_token"}'
```

## indexeddb
IndexedDB contents.
**Params:** database (string), store (string)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"indexeddb","database":"myDB","store":"users"}'
```

## command_result
Async command results.
**Params:** correlation_id (string)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"command_result","correlation_id":"abc-123"}'
```

## pending_commands
Commands awaiting execution.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"pending_commands"}'
```

## failed_commands
Failed commands.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"failed_commands"}'
```

## saved_videos
Saved video recordings.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"saved_videos"}'
```

## recordings
Recording metadata.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"recordings"}'
```

## recording_actions
Actions within a recording.
**Params:** recording_id (string)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"recording_actions","recording_id":"rec-001"}'
```

## playback_results
Playback results.
**Params:** recording_id (string)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"playback_results","recording_id":"rec-001"}'
```

## log_diff_report
Log diff report.
**Params:** original_id (string), replay_id (string)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"log_diff_report","original_id":"rec-001","replay_id":"rec-002"}'
```

## summarized_logs
Aggregated/grouped logs.
**Params:** min_group_size (integer, default 2)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"summarized_logs","min_group_size":3}'
```

## page_inventory
Inventory of page elements.
**Params:** visible_only (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"page_inventory","visible_only":true}'
```

## transients
Transient UI elements (alerts, toasts).
**Params:** url (string), classification (`alert` | `toast` | `snackbar` | `notification` | `tooltip` | `banner` | `flash`), summary (boolean)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"transients","classification":"toast"}'
```

## inbox
Message inbox.
**Params:** none (universal params only)
**Example:**
```bash
bash scripts/gasoline-call.sh observe '{"what":"inbox"}'
```
