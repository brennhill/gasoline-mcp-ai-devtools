---
doc_type: flow_map
flow_id: network-recording-control
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/tools_configure_network_recording.go:toolConfigureNetworkRecording
code_paths:
  - cmd/dev-console/tools_configure_network_recording.go
  - cmd/dev-console/tools_configure_network_recording_state.go
  - cmd/dev-console/tools_configure_network_recording_filters.go
  - internal/capture/network_bodies.go
test_paths:
  - cmd/dev-console/tools_configure_network_recording_test.go
  - cmd/dev-console/tools_configure_handler_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Network Recording Control

## Scope

Covers `configure(what:"network_recording")` lifecycle and filtering of captured network bodies.

## Entrypoints

- `toolConfigureNetworkRecording` handles `start`, `stop`, and `status` operations.

## Primary Flow

1. Client calls `configure` with `what:"network_recording"`.
2. `start` uses `networkRecordingState.tryStart` to atomically begin recording.
3. `status` reads current state snapshot via `networkRecordingState.info`.
4. `stop` snapshots and clears state via `networkRecordingState.stop`.
5. On stop, handler reads capture network bodies and filters through `collectRecordedRequests`.
6. `matchesRecordingFilter` applies time/domain/method constraints.
7. Response returns recorded request summary, duration, and count.

## Error and Recovery Paths

- Invalid JSON returns `ErrInvalidJSON`.
- Unknown operation returns `ErrInvalidParam` with allowed operations.
- Duplicate start and stop-without-start return actionable `ErrInvalidParam` guidance.

## State and Contracts

- Recording state transitions are mutex-protected and atomic.
- Timestamp parsing supports RFC3339Nano and millisecond epoch fallback.
- Filtering is best-effort: unparseable timestamps are retained rather than dropped.

## Code Paths

- `cmd/dev-console/tools_configure_network_recording.go`
- `cmd/dev-console/tools_configure_network_recording_state.go`
- `cmd/dev-console/tools_configure_network_recording_filters.go`
- `internal/capture/network_bodies.go`

## Test Paths

- `cmd/dev-console/tools_configure_network_recording_test.go`
- `cmd/dev-console/tools_configure_handler_test.go`

## Edit Guardrails

- Keep concurrency/state logic in `*_state.go`; avoid mixing with response serialization.
- Keep body filtering/projection in `*_filters.go` so stop handler remains orchestration-only.
- Preserve operation names and structured error behavior for CLI/MCP compatibility.
