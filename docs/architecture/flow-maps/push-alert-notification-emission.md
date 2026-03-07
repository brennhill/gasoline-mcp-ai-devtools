---
doc_type: flow_map
flow_id: push-alert-notification-emission
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - cmd/browser-agent/streaming.go:toolConfigureStreaming
  - internal/streaming/stream_emit.go:EmitAlert
  - internal/streaming/stream_emit.go:FormatMCPNotification
code_paths:
  - cmd/browser-agent/streaming.go
  - cmd/browser-agent/alerts.go
  - cmd/browser-agent/tools_configure_runtime_impl.go
  - internal/streaming/stream.go
  - internal/streaming/stream_emit.go
  - internal/streaming/types.go
  - internal/streaming/alerts_buffer.go
  - internal/identity/mcp.go
test_paths:
  - internal/streaming/stream_test.go
  - internal/streaming/alerts_test.go
  - cmd/browser-agent/alerts_unit_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Push Alert Notification Emission

## Scope

Covers configure-time streaming state changes and runtime MCP notification emission for alert/CI events.

## Entrypoints

1. `toolConfigureStreaming` maps configure action payloads into streaming state updates.
2. `EmitAlert` evaluates filters/rate limits/dedup and conditionally writes notification payloads.
3. `FormatMCPNotification` builds MCP `notifications/message` envelopes.

## Primary Flow

1. Caller enables streaming via `configure` (`streaming_action` -> `action` wrapper).
2. Tool handler updates `StreamState` (`enabled`, `events`, `throttle_seconds`, `severity_min`).
3. Alert producers append alerts to the alert buffer and call `EmitAlert`.
4. `EmitAlert` checks event filters, throttle, dedup windows, and minute limits.
5. If emission is allowed, `FormatMCPNotification` produces MCP JSON with canonical logger identity from shared identity constants.
6. Notification is written to stream writer (never stdout by default).

## Error and Recovery Paths

1. Unknown configure action returns a structured validation error.
2. Filter/rate-limit/dedup misses silently suppress emission while preserving buffer history.
3. JSON marshal or writer errors are non-fatal; stream state remains usable for later alerts.

## State and Contracts

1. `StreamState` lock protects config + counters + dedup state transitions.
2. Notification logger identity is canonicalized as `gasoline-browser-devtools` via `internal/identity`.
3. Writer defaults to `nil` to avoid protocol-breaking stdout writes.
4. Pending batches are bounded (`MaxPendingBatch`) to prevent unbounded memory growth.

## Code Paths

- `cmd/browser-agent/streaming.go`
- `cmd/browser-agent/alerts.go`
- `cmd/browser-agent/tools_configure_runtime_impl.go`
- `internal/streaming/stream.go`
- `internal/streaming/stream_emit.go`
- `internal/streaming/types.go`
- `internal/streaming/alerts_buffer.go`
- `internal/identity/mcp.go`

## Test Paths

- `internal/streaming/stream_test.go`
- `internal/streaming/alerts_test.go`
- `cmd/browser-agent/alerts_unit_test.go`

## Edit Guardrails

1. Keep emission filter decisions in `EmitAlert`; avoid duplicating filter logic in handlers.
2. Preserve canonical logger identity for MCP notifications to keep client-side routing stable.
3. Do not route streaming output to stdout in bridge mode.
