---
doc_type: flow_map
flow_id: interact-batch-sequences
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/tools_interact_batch.go:handleBatch
  - cmd/dev-console/tools_configure_sequence_replay.go:toolConfigureReplaySequence
code_paths:
  - cmd/dev-console/tools_interact_batch.go
  - cmd/dev-console/tools_configure_sequence_replay.go
  - cmd/dev-console/tools_configure_sequence_replay_steps.go
  - cmd/dev-console/tools_configure_sequence_types.go
  - cmd/dev-console/tools_interact_dispatch.go
  - internal/schema/interact_actions.go
  - internal/schema/interact_properties_output_batch.go
  - internal/tools/configure/mode_specs_interact.go
test_paths:
  - cmd/dev-console/tools_interact_batch_test.go
  - cmd/dev-console/tools_configure_sequence_test.go
---

# Interact Batch Sequences

## Scope

Covers ad-hoc multi-step execution via `interact(what:"batch")` and shared replay semantics used by `configure(what:"replay_sequence")`.

## Entrypoints

- `handleBatch` executes inline step arrays in a single `interact` call.
- `toolConfigureReplaySequence` replays saved sequences through the same async step execution pattern.

## Primary Flow

1. Parse and validate step list (`steps`, max 50, each step includes `what`/`action`).
2. Normalize execution controls (`continue_on_error`, `step_timeout_ms`, `stop_after_step`).
3. Acquire `replayMu` to prevent concurrent batch/replay collisions.
4. Force each step into async interact dispatch (`sync:false`, `wait:false`) for deterministic transport behavior.
5. Wait per step for correlation completion up to `step_timeout_ms`; classify each step as `ok`, `queued`, or `error`.
6. Aggregate counters/results (`steps_executed`, `steps_failed`, `steps_queued`, `results`) and emit a status summary.

## Error and Recovery Paths

- Missing/empty/oversized `steps` returns `ErrInvalidParam`.
- Nested batch/replay contention returns "Another batch or sequence is currently executing".
- Per-step failures are recorded and can either halt or continue based on `continue_on_error`.
- Timeout on correlation wait marks step as `queued` (non-terminal async completion path).

## State and Contracts

- Step result shape uses `SequenceStepResult` with stable fields (`step_index`, `action`, `status`, `duration_ms`, optional `correlation_id`/`error`).
- Counter invariants are enforced by tests (`steps_failed <= steps_executed <= steps_total`).
- `include_screenshot` is stripped from step payloads during batch to avoid discarded base64 payload overhead.

## Code Paths

- `cmd/dev-console/tools_interact_batch.go`
- `cmd/dev-console/tools_configure_sequence_replay.go`
- `cmd/dev-console/tools_configure_sequence_replay_steps.go`
- `cmd/dev-console/tools_configure_sequence_types.go`
- `cmd/dev-console/tools_interact_dispatch.go`
- `internal/schema/interact_actions.go`
- `internal/schema/interact_properties_output_batch.go`
- `internal/tools/configure/mode_specs_interact.go`

## Test Paths

- `cmd/dev-console/tools_interact_batch_test.go`
- `cmd/dev-console/tools_configure_sequence_test.go`

## Edit Guardrails

- Keep batch/replay step-status semantics aligned; update both paths if result classification changes.
- Preserve `replayMu` coverage to avoid deadlocks and cross-run state races.
- Keep schema, mode specs, and dispatch support synchronized when adding batch options.
