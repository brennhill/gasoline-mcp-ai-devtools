---
doc_type: flow_map
flow_id: recording-control-and-playback
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/tools_configure.go (recording_start|recording_stop|playback|log_diff)
  - cmd/dev-console/tools_observe.go (recordings|recording_actions|playback_results)
code_paths:
  - cmd/dev-console/recording_helpers.go
  - cmd/dev-console/recording_handlers_control.go
  - cmd/dev-console/recording_handlers_query.go
  - cmd/dev-console/recording_handlers_playback.go
  - cmd/dev-console/recording_handlers_logdiff.go
  - internal/recording/playback_engine.go
test_paths:
  - cmd/dev-console/recording_handlers_test.go
  - cmd/dev-console/recording_playback_result_test.go
  - cmd/dev-console/tools_observe_contract_test.go
  - cmd/dev-console/tools_configure_audit_test.go
---

# Recording Control and Playback

## Scope

Covers configure/observe flows for action recording, playback execution, result retrieval, and log-diff reporting.

## Entrypoints

- Configure actions: `recording_start`, `recording_stop`, `playback`, `log_diff`.
- Observe queries: `recordings`, `recording_actions`, `playback_results`.

## Primary Flow

1. `toolConfigureRecordingStart` validates input and calls capture `StartRecording`.
2. `toolConfigureRecordingStop` validates `recording_id` and finalizes capture session.
3. `toolConfigurePlayback` runs `ExecutePlayback` and stores session in `playbackSessions`.
4. `toolGetPlaybackResults` projects stored session into response payload.
5. `toolConfigureLogDiff` and `toolGetLogDiffReport` compare original vs replay IDs.

## Error and Recovery Paths

- Missing required IDs return structured `ErrMissingParam` with actionable retry text.
- Unknown recording or playback session returns `ErrNoData` / `ErrInternal` as appropriate.
- Diff/report operations fail fast on invalid recording IDs.

## State and Contracts

- `playbackSessions` is write/read protected by `playbackMu`.
- `buildPlaybackResult` defines canonical playback completion response fields.
- `appendServerLog` enforces bounded in-memory server log capacity.

## Code Paths

- `cmd/dev-console/recording_helpers.go`
- `cmd/dev-console/recording_handlers_control.go`
- `cmd/dev-console/recording_handlers_query.go`
- `cmd/dev-console/recording_handlers_playback.go`
- `cmd/dev-console/recording_handlers_logdiff.go`

## Test Paths

- `cmd/dev-console/recording_handlers_test.go`
- `cmd/dev-console/recording_playback_result_test.go`
- `cmd/dev-console/tools_observe_contract_test.go`
- `cmd/dev-console/tools_configure_audit_test.go`

## Edit Guardrails

- Keep configure/observe schema compatibility stable (field names and structured errors).
- Any playback response shape change must update `buildPlaybackResult` tests first.
- Do not bypass `playbackMu` when touching `playbackSessions`.
