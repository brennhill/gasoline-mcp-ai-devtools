---
feature: context-streaming
status: proposed
version: null
tool: configure
mode: streaming
authors: []
created: 2026-01-26
updated: 2026-01-28
---

# Context Streaming

> Push-based telemetry notifications from the Gasoline server to the MCP client, enabling real-time AI awareness of browser events without polling.

## Problem

Today, AI coding agents using Gasoline must repeatedly call `observe()` to discover what happened in the browser. This polling model has three fundamental problems:

1. **Latency.** A new 500 error or performance regression sits in the server buffer until the AI happens to poll. In a typical coding loop, the AI only calls `observe()` when it thinks something might have changed --- meaning seconds or minutes of delay before it notices a problem.

2. **Wasted tokens.** Most `observe()` calls return "nothing new." The AI is spending tool calls (and the user is spending tokens) on empty polls. In a 30-minute debugging session, an agent might make 20+ observe calls where only 3 contain actionable data.

3. **Missed context windows.** Rapid-fire events (e.g., a cascade of 5xx errors during a deploy) can fill and overflow the ring buffer before the AI polls. The AI sees a truncated view of what happened.

Push-based streaming solves all three: the server tells the AI when something significant happens, the moment it happens.

## Solution

Context Streaming adds an opt-in active notification mode to the existing push-alerts infrastructure. When an AI agent enables streaming via `configure({action: "streaming", ...})`, the Gasoline server begins emitting MCP notifications over the stdio transport whenever significant browser events occur. The AI receives these notifications between tool calls, gaining real-time awareness without polling.

The feature builds on two existing systems:

- **Push Alerts (passive mode, shipped):** Alerts are already accumulated in an `AlertBuffer` and piggybacked onto `observe()` responses as an `_alerts` content block. This continues to work unchanged.

- **Context Streaming (active mode, this feature):** When enabled, the same alerts that would accumulate in the buffer are ALSO emitted as MCP `notifications/message` to stdout in real time. The AI does not need to call any tool to receive them.

The key insight: passive mode (piggybacking) is the safe default that works with every MCP client. Active mode (push notifications) is an opt-in upgrade for clients that can process unsolicited notifications.

## User Stories

- As an AI coding agent, I want to be notified immediately when a new JavaScript error appears in the browser so that I can investigate the error in the context of the code change I just made, without having to remember to poll.

- As an AI coding agent, I want to be notified when a network request returns a 5xx status so that I can proactively check the backend code I just modified.

- As an AI coding agent, I want to be notified when a performance regression is detected (e.g., LCP doubled) so that I can correlate it with the CSS or JavaScript change I just deployed to the dev server.

- As an AI coding agent, I want to control exactly which event categories I receive notifications for so that I am not flooded with irrelevant events while debugging a specific issue.

- As a developer using Gasoline, I want streaming to be off by default so that it does not produce unexpected output or confuse MCP clients that do not support unsolicited notifications.

- As an AI coding agent, I want to disable streaming instantly when notifications become noisy so that I can return to focused work without distraction.

## MCP Interface

**Tool:** `configure`
**Mode/Action:** `streaming`

Context Streaming is accessed via the existing `configure` tool with `action: "streaming"`. The streaming-specific sub-action is passed via the `streaming_action` parameter to avoid conflicts with the top-level `action` parameter.

### Request: Enable Streaming

```json
{
  "tool": "configure",
  "arguments": {
    "action": "streaming",
    "streaming_action": "enable",
    "events": ["errors", "network_errors", "performance"],
    "throttle_seconds": 5,
    "severity_min": "warning"
  }
}
```

### Request: Disable Streaming

```json
{
  "tool": "configure",
  "arguments": {
    "action": "streaming",
    "streaming_action": "disable"
  }
}
```

### Request: Check Status

```json
{
  "tool": "configure",
  "arguments": {
    "action": "streaming",
    "streaming_action": "status"
  }
}
```

### Response: Enable

```json
{
  "status": "enabled",
  "config": {
    "enabled": true,
    "events": ["errors", "network_errors", "performance"],
    "throttle_seconds": 5,
    "severity_min": "warning",
    "url": ""
  }
}
```

### Response: Disable

```json
{
  "status": "disabled",
  "pending_cleared": 3
}
```

### Response: Status

```json
{
  "config": {
    "enabled": true,
    "events": ["errors", "network_errors", "performance"],
    "throttle_seconds": 5,
    "severity_min": "warning",
    "url": ""
  },
  "notify_count": 7,
  "pending": 2
}
```

### MCP Notification Format (Pushed to Client)

When streaming is enabled and an event passes all filters, the server writes an MCP notification to stdout:

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/message",
  "params": {
    "level": "warning",
    "logger": "gasoline",
    "data": {
      "category": "network_errors",
      "severity": "error",
      "title": "Server error: POST /api/users -> 500",
      "detail": "Request failed with Internal Server Error",
      "timestamp": "2026-01-28T14:30:00.000Z",
      "source": "anomaly_detector"
    }
  }
}
```

This uses the standard MCP `notifications/message` method, which is part of the MCP specification. No protocol extensions are required.

### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `streaming_action` | string (required) | -- | `"enable"`, `"disable"`, or `"status"` |
| `events` | string[] | `["all"]` | Event categories to stream. Valid values: `"errors"`, `"network_errors"`, `"performance"`, `"user_frustration"`, `"security"`, `"regression"`, `"anomaly"`, `"ci"`, `"all"` |
| `throttle_seconds` | integer | 5 | Minimum seconds between notifications. Range: 1-60 |
| `url_filter` | string | `""` | Only stream events whose URL contains this substring. Empty means no filter |
| `severity_min` | string | `"warning"` | Minimum severity to emit. One of: `"info"`, `"warning"`, `"error"` |

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Streaming is OFF by default. The AI must explicitly call `configure({action: "streaming", streaming_action: "enable"})` to activate it | must |
| R2 | When enabled, significant browser events are pushed as MCP `notifications/message` to stdout without requiring a tool call | must |
| R3 | The AI can configure which event categories to receive via the `events` parameter | must |
| R4 | The AI can set a minimum severity threshold via `severity_min` | must |
| R5 | Notifications are throttled: at most one notification per `throttle_seconds` (default 5s) | must |
| R6 | Notifications are rate-limited: at most 12 per minute, hard cap | must |
| R7 | Notifications are deduplicated: the same event (by dedup key) is not repeated within a 30-second window | must |
| R8 | `configure({action: "streaming", streaming_action: "disable"})` immediately stops all notifications and clears pending state | must |
| R9 | Notification output is serialized with the MCP response stream: no interleaving of JSON messages on stdout | must |
| R10 | Notification context data is redacted using the same rules as tool responses (sensitive headers stripped, body patterns masked) | must |
| R11 | The passive alert mode (piggybacking on `observe` responses) continues to work independently of active streaming | must |
| R12 | The `status` sub-action returns current configuration and emission statistics | should |
| R13 | Events that arrive during a throttle window are batched and the batch is flushed when the window expires | should |
| R14 | The URL filter restricts notifications to events matching a URL substring, applied only to event types where a URL is semantically meaningful (network, performance, security) | should |
| R15 | The `SeenMessages` dedup cache is bounded (max 500 entries) and periodically pruned | should |
| R16 | When the MCP client disconnects (stdin closes), all streaming goroutines shut down cleanly without panics or goroutine leaks | must |

## Non-Goals

- **This feature does NOT replace observe().** Streaming provides real-time alerts for significant events. The AI still uses `observe()` for comprehensive data retrieval (full log history, network waterfall, DOM state, etc.). Streaming is a notification layer, not a data layer.

- **This feature does NOT implement SSE or WebSocket push.** The transport is MCP stdio only. An SSE endpoint (`/events/stream`) for non-MCP consumers (dashboards, custom tooling) is deferred to a future version.

- **This feature does NOT add a 5th MCP tool.** Streaming configuration is accessed via `configure({action: "streaming"})`, adhering to the 4-tool maximum constraint.

- **This feature does NOT stream raw telemetry.** It streams processed alerts (errors, regressions, anomalies). Streaming every console.log or network request would overwhelm the AI. The alert system acts as a significance filter.

- **This feature does NOT persist streaming state.** If the server restarts, streaming configuration is lost. The AI must re-enable streaming after reconnecting. Persistence is a future consideration.

- **Out of scope: Frustration detection.** Detecting user frustration patterns (rage clicks, repeated form submissions) is a related but separate concern. It may feed into the alert system in the future but is not part of this spec.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Alert emission latency (event received to notification written to stdout) | < 5ms |
| Notification JSON serialization | < 0.5ms |
| `configure` tool response time | < 10ms |
| Memory overhead of StreamState (including SeenMessages cache, PendingBatch) | < 500KB |
| Stdout write contention (lock hold time for notification write) | < 1ms |
| Zero impact on passive alert mode performance | No regression |

## Security Considerations

- **Redaction at construction time.** Event context data (URLs, headers, request metadata) is redacted before the notification is emitted, not after. The `StreamEvent` or `Alert` struct should never hold unredacted sensitive data in memory longer than necessary. This prevents accidental leakage through debug logging or crash dumps.

- **Same redaction rules as tool responses.** Sensitive headers (Authorization, Cookie, API keys) are stripped. Request/response bodies follow the same opt-in rules as `observe({what: "network_bodies"})`. URL query parameters with sensitive keys are masked. The existing redaction engine is reused --- no separate implementation.

- **Localhost-only transport.** Notifications are written to stdout (MCP stdio transport), which is a local pipe between the Gasoline process and the MCP client process. No network exposure. No authentication needed for the notification path itself.

- **No amplification attack surface.** The rate limit (12/min) and throttle (5s default) prevent a malicious page from flooding the AI's context window. Even if a page generates thousands of errors per second, the AI receives at most 12 notifications per minute.

- **CI webhook security.** The `/ci-result` endpoint that generates CI alerts (which can trigger streaming notifications) has no authentication. This is acceptable for localhost-only usage but must NOT be exposed through tunnels or containers. This is documented in the push-alerts spec and remains unchanged.

## Edge Cases

- **What happens when streaming is enabled but the extension is disconnected?** Expected: No events arrive at the server, so no notifications are emitted. When the extension reconnects, events flow again and streaming resumes automatically. No special handling needed.

- **What happens when the AI enables streaming but never calls observe()?** Expected: Notifications are emitted to stdout regardless of observe() calls. The passive alert buffer also accumulates alerts independently. If the AI later calls observe(), it gets the piggybacked alerts as usual (those are a separate buffer from the streaming path).

- **What happens when 100 errors arrive in 1 second?** Expected: The first error triggers a notification (if it passes filters). Subsequent errors within the throttle window (5s default) are added to the PendingBatch (capped at 100 entries). When the throttle window expires, the batch is flushed as a single notification (or the next qualifying event triggers the flush). Overflow beyond 100 pending entries is silently dropped.

- **What happens when the MCP client disconnects mid-notification?** Expected: The write to stdout fails. The streaming goroutine detects the closed pipe (write error or context cancellation) and stops emitting. No panic, no goroutine leak. The StreamState is left in an enabled-but-inactive state; it becomes irrelevant once the process shuts down or the next MCP client connects.

- **What happens when streaming is rapidly toggled (enable/disable/enable)?** Expected: Each toggle takes effect immediately under the StreamState mutex. Disable clears all pending state (batch, dedup cache, counters). Re-enable starts fresh. No race conditions because all state transitions are serialized.

- **What happens when the SeenMessages dedup cache reaches its maximum (500 entries)?** Expected: Entries older than 2x the dedup window (60s) are evicted. If still over capacity after eviction, the oldest entries by timestamp are removed. This prevents unbounded memory growth during long sessions with high error diversity.

- **What happens when a notification and a tool response are written to stdout at the same time?** Expected: The `mcpStdoutMu` mutex serializes all stdout writes. The notification goroutine acquires the lock, writes the complete JSON line, releases the lock. The main MCP response loop does the same. No interleaving is possible.

- **What happens when the URL filter is set but the event has no URL (e.g., a console error)?** Expected: The URL filter is only applied to event types where a URL is semantically meaningful: `network_errors`, `performance`, and `security`. For other event types (`errors`, `anomaly`, `ci`, `threshold`), the URL filter is skipped and the event passes through.

## Interaction with Push Alerts (Passive Mode)

Context Streaming and Push Alerts are two delivery mechanisms for the same underlying alert data. They are designed to coexist:

| Aspect | Push Alerts (Passive) | Context Streaming (Active) |
|--------|----------------------|---------------------------|
| Delivery | Piggybacked on `observe()` response | Pushed as MCP notification to stdout |
| Trigger | Next `observe()` call | Immediate (subject to throttle) |
| Default state | Always on | Off (opt-in) |
| Alert buffer | Drained on `observe()` | Not drained by streaming |
| Configuration | None (always attached) | `configure({action: "streaming"})` |
| MCP client requirement | Any client (standard tool response) | Client must handle `notifications/message` |

When both are active, the same alert event does two things:
1. It is added to the `AlertBuffer` (for passive piggybacking on the next `observe()` call)
2. It is passed to `StreamState.EmitAlert()` (for immediate notification if filters pass)

These two paths are independent. Draining the alert buffer on `observe()` does not affect streaming. Disabling streaming does not affect the alert buffer.

## Throttling Strategy

The throttling design prevents the AI from being overwhelmed while preserving timeliness for critical events. Three mechanisms work together:

1. **Throttle window** (`throttle_seconds`, default 5s): After emitting a notification, no further notification is emitted until the window elapses. Events arriving during the window are added to `PendingBatch`.

2. **Rate limit** (12 per minute, fixed): Hard ceiling on notification volume. Even with `throttle_seconds=1`, at most 12 notifications per minute are emitted. This maps to roughly one notification every 5 seconds on average.

3. **Dedup window** (30 seconds, fixed): The same event (identified by `category:title` key) is not emitted again within 30 seconds, regardless of throttle state. This prevents repetitive alerts (e.g., the same API endpoint returning 500 on every request).

**Batch flushing:** When the throttle window expires, if there are pending batched events, the next qualifying event triggers a flush. If no event arrives after the window expires, a background timer flushes the batch after an additional grace period. The exact flush mechanism is an implementation detail.

## Event Categories

The `events` filter parameter maps to the alert categories generated by the existing push-alerts system:

| Event Filter Value | Alert Categories Matched | Typical Trigger |
|-------------------|------------------------|-----------------|
| `errors` | `anomaly`, `threshold` | Console error spike, memory pressure |
| `network_errors` | `anomaly` | Network failure spike |
| `performance` | `regression`, `threshold` | LCP/FCP/CLS regression, budget breach |
| `regression` | `regression` | Performance metric degradation |
| `anomaly` | `anomaly` | Error frequency spike (3x rolling avg) |
| `ci` | `ci` | CI/CD webhook result received |
| `security` | `threshold` | Security audit finding |
| `user_frustration` | `anomaly` | (Future: rage clicks, form timeouts) |
| `all` | All categories | Every alert is eligible |

## Dependencies

- **Depends on:** Push Alerts (shipped) --- provides the `AlertBuffer`, `Alert` struct, alert generation logic (anomaly detection, CI webhook, performance regression detection), and the `addAlert` entry point that bridges to streaming.
- **Depends on:** MCP stdio transport --- the server must write well-formed JSON-RPC notifications to stdout.
- **Depends on:** `mcpStdoutMu` mutex --- the existing stdout serialization mechanism (already in `main.go` and `streaming.go`).
- **Depended on by:** (Future) Frustration detection, deployment watchdog, and any feature that benefits from real-time AI awareness.

## Assumptions

- A1: The MCP client (Claude Code, Cursor, etc.) can receive and process `notifications/message` JSON-RPC messages on its stdin. If the client ignores unknown notifications, streaming is harmless but useless.
- A2: The `notifications/message` method with `level`, `logger`, and `data` params is within the MCP specification (MCP 2024-11-05). No custom method names are used.
- A3: The extension is connected and sending telemetry to the server. Without incoming events, there is nothing to stream.
- A4: The stdout pipe between the Gasoline server and the MCP client remains open for the duration of the streaming session. If it closes, the server detects this via write errors.
- A5: The server process has a single MCP client connection at a time (stdio is inherently point-to-point). Streaming targets that single client.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Batch flush timer implementation | open | The spec calls for flushing pending batches when the throttle window expires, but the current implementation has no background timer for this. Events batched during a quiet period (no subsequent event arrives) may be delayed indefinitely. Need to decide: (a) add a ticker goroutine, (b) accept the tradeoff and document it, or (c) flush on next observe() call as fallback. |
| OI-2 | MCP client notification support verification | open | Which MCP clients actually process `notifications/message`? Claude Code likely does, but Cursor, Continue, and other MCP clients may not. Need to verify client behavior: do they silently ignore unknown notifications, or do they error? If any client errors on unsolicited notifications, streaming must remain strictly opt-in. |
| OI-3 | Streaming across MCP reconnects | open | When `--persist=true` (default), the server survives MCP disconnects. If the AI enabled streaming, disconnects, and a new MCP client connects, should streaming state be preserved or reset? Current implementation has no reconnect awareness. Likely should reset to off on new `initialize` handshake. |
| OI-4 | Notification content richness | open | Current notifications carry `category`, `severity`, `title`, `detail`, `timestamp`, `source`. Should they also carry a `correlation_id` and/or `suggested_action` (e.g., "call observe({what: 'network_waterfall'}) for details")? Richer notifications help the AI act on them; leaner notifications save tokens. |
| OI-5 | Category taxonomy alignment | open | The event filter values (`errors`, `network_errors`, `performance`, etc.) and the alert categories (`regression`, `anomaly`, `ci`, `noise`, `threshold`) are different taxonomies. The mapping table above captures the current design, but this creates a translation layer. Should we unify to a single taxonomy? The current design preserves backwards compatibility with the push-alerts category names. |
| OI-6 | Lock ordering documentation | open | The codebase has 15+ mutexes. Adding `StreamState.mu` requires documented lock acquisition ordering to prevent deadlocks. The review flagged this as critical. Must be documented in `architecture.md` before implementation. |
| OI-7 | Context cancellation on disconnect | open | The review identified that background goroutines (if any, e.g., batch flush timer) need `context.Context` wired to stdin closure. The current implementation does not use context cancellation. Must be addressed if a timer goroutine is added (OI-1). |
