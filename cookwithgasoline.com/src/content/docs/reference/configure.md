---
title: "configure() — Customize the Session"
description: "Complete reference for the configure tool. 16 actions for noise filtering, persistent storage, recording, playback, streaming, log diff, session diffs, audit log, and more."
---

The `configure` tool manages your Gasoline session — filter noise, store data, manage recordings, compare error states, diff sessions, control streaming, and view audit logs.

## Quick Reference

```js
configure({action: "noise_rule", noise_action: "auto_detect"})       // Auto-filter noise
configure({action: "store", store_action: "save", key: "baseline", data: {...}})  // Save data
configure({action: "clear", buffer: "all"})                          // Clear all buffers
configure({action: "health"})                                        // Server health
configure({action: "recording_start"})                               // Start recording
configure({action: "recording_stop", recording_id: "rec-123"})      // Stop recording
configure({action: "playback", recording_id: "rec-123"})            // Replay recording
configure({action: "log_diff", original_id: "rec-1", replay_id: "rec-2"})  // Compare error states
configure({action: "diff_sessions", session_action: "capture", name: "v1"})  // Session snapshot
configure({action: "audit_log", tool_name: "observe"})               // View tool usage history
```

---

## noise_rule — Filter Irrelevant Errors

Manages noise rules that suppress irrelevant errors — browser extension noise, analytics failures, framework internals.

### Auto-detect noise

```js
configure({action: "noise_rule", noise_action: "auto_detect"})
```

Scans current errors and identifies patterns that are likely noise (extension errors, analytics, third-party scripts). Creates rules automatically.

### Add a manual rule

```js
configure({action: "noise_rule",
           noise_action: "add",
           pattern: "analytics\\.google",
           category: "console",
           reason: "Google Analytics noise"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `noise_action` | string | `add`, `remove`, `list`, `reset`, `auto_detect` |
| `pattern` | string | Regex pattern to match against error messages |
| `category` | string | Buffer: `console`, `network`, or `websocket` |
| `reason` | string | Human-readable explanation of why this is noise |
| `rules` | array | Batch-add multiple rules at once |
| `rule_id` | string | ID of rule to remove (for `remove` action) |

### List current rules

```js
configure({action: "noise_rule", noise_action: "list"})
```

### Remove a rule

```js
configure({action: "noise_rule", noise_action: "remove", rule_id: "rule-123"})
```

### Reset all rules

```js
configure({action: "noise_rule", noise_action: "reset"})
```

---

## store — Persistent Key-Value Storage

Save and load JSON data that persists across sessions. Useful for storing baselines, configuration, or any data the AI needs to remember.

### Save data

```js
configure({action: "store",
           store_action: "save",
           namespace: "baselines",
           key: "homepage-vitals",
           data: {lcp: 1200, cls: 0.05}})
```

### Load data

```js
configure({action: "store",
           store_action: "load",
           namespace: "baselines",
           key: "homepage-vitals"})
```

### List keys

```js
configure({action: "store", store_action: "list"})
configure({action: "store", store_action: "list", namespace: "baselines"})
```

### Delete a key

```js
configure({action: "store", store_action: "delete", key: "homepage-vitals"})
```

### Storage stats

```js
configure({action: "store", store_action: "stats"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `store_action` | string | `save`, `load`, `list`, `delete`, `stats` |
| `namespace` | string | Logical grouping for keys |
| `key` | string | Storage key |
| `data` | object | JSON data to persist (for `save`) |

---

## clear — Clear Buffers

Remove captured data from memory.

```js
configure({action: "clear", buffer: "all"})
configure({action: "clear", buffer: "network"})
configure({action: "clear", buffer: "logs"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `buffer` | string | `network`, `websocket`, `actions`, `logs`, `all` |

---

## recording_start — Start Recording

Start capturing a browser session. Records user actions and browser state for later playback or comparison.

```js
configure({action: "recording_start"})
```

---

## recording_stop — Stop Recording

Stop an active recording session.

```js
configure({action: "recording_stop", recording_id: "rec-123"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `recording_id` | string | ID of the recording to stop |

---

## playback — Replay Recording

Replay a previously captured recording.

```js
configure({action: "playback", recording_id: "rec-123"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `recording_id` | string | ID of the recording to replay |

---

## log_diff — Compare Error States

Compare error states between two recordings. Useful for verifying that a bug fix resolved the issue, or detecting regressions after a deploy.

```js
configure({action: "log_diff", original_id: "rec-abc", replay_id: "rec-xyz"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `original_id` | string | Original recording ID (baseline) |
| `replay_id` | string | Replay recording ID (comparison) |

---

## telemetry — Configure Telemetry Metadata

Set the global telemetry metadata mode. Individual tool calls can override this with the `telemetry_mode` parameter.

```js
configure({action: "telemetry", telemetry_mode: "auto"})
configure({action: "telemetry", telemetry_mode: "off"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `telemetry_mode` | string | `off`, `auto`, or `full` |

---

## streaming — Real-Time Event Streaming

Enable or disable real-time event notifications. When enabled, Gasoline proactively notifies the AI about errors, performance regressions, and security issues as they happen.

```js
configure({action: "streaming", streaming_action: "enable",
           events: ["errors", "performance", "security"],
           severity_min: "warning",
           throttle_seconds: 5})

configure({action: "streaming", streaming_action: "disable"})
configure({action: "streaming", streaming_action: "status"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `streaming_action` | string | `enable`, `disable`, `status` |
| `events` | array | Categories: `errors`, `network_errors`, `performance`, `user_frustration`, `security`, `regression`, `anomaly`, `ci`, `all` |
| `severity_min` | string | Minimum severity: `info`, `warning`, `error` |
| `throttle_seconds` | integer | Minimum seconds between notifications (1-60) |

---

## health — Server Health

Check the Gasoline server's status, uptime, buffer usage, and connected clients.

```js
configure({action: "health"})
```

No additional parameters. Returns server version, uptime, buffer occupancy, client count, and rate limit status.

---

## test_boundary_start / test_boundary_end — Test Boundaries

Mark the start and end of a test run. Events within boundaries can be correlated for test-specific analysis.

```js
configure({action: "test_boundary_start", test_id: "checkout-flow", label: "Guest Checkout Test"})

// ... run the test ...

configure({action: "test_boundary_end", test_id: "checkout-flow"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `test_id` | string | Unique identifier for the test |
| `label` | string | Human-readable description |

---

## diff_sessions — Session Snapshots & Comparison

Capture named snapshots of the current session state and compare them to detect changes.

### Capture a snapshot

```js
configure({action: "diff_sessions", session_action: "capture", name: "before-deploy"})
```

### Compare two snapshots

```js
configure({action: "diff_sessions",
           session_action: "compare",
           compare_a: "before-deploy",
           compare_b: "after-deploy"})
```

### List snapshots

```js
configure({action: "diff_sessions", session_action: "list"})
```

### Delete a snapshot

```js
configure({action: "diff_sessions", session_action: "delete", name: "old-snapshot"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `session_action` | string | `capture`, `compare`, `list`, `delete` |
| `name` | string | Snapshot name |
| `compare_a` | string | First snapshot (baseline) |
| `compare_b` | string | Second snapshot (comparison target) |

---

## audit_log — MCP Tool Usage History

View a log of all MCP tool calls made during the session. Useful for debugging, compliance, and understanding AI agent behavior.

```js
configure({action: "audit_log"})
configure({action: "audit_log", tool_name: "observe", limit: 20})
configure({action: "audit_log", since: "2026-02-07T10:00:00Z"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `tool_name` | string | Filter by tool name |
| `session_id` | string | Filter by MCP session ID |
| `since` | string | Only entries after this ISO 8601 timestamp |
| `limit` | number | Maximum entries to return |

---

## describe_capabilities — Tool Capability Discovery

Returns a description of the tool's capabilities. Useful for AI agents to understand what actions are available.

```js
configure({action: "describe_capabilities"})
```

No additional parameters.
