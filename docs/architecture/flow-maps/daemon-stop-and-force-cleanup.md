---
doc_type: flow_map
flow_id: daemon-stop-and-force-cleanup
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/main_connection_stop.go:runStopMode
  - cmd/dev-console/main_connection_stop.go:runForceCleanup
code_paths:
  - cmd/dev-console/main_connection_stop.go
  - cmd/dev-console/main_connection_stop_strategies.go
  - cmd/dev-console/main_connection_force_cleanup_strategies.go
test_paths:
  - cmd/dev-console/main_connection_diag_test.go
  - cmd/dev-console/main_connection_coverage_test.go
  - cmd/dev-console/main_connection_pid_contract_test.go
  - cmd/dev-console/test_daemon_cleanup_test.go
---

# Daemon Stop and Force Cleanup

## Scope

Covers graceful single-port daemon shutdown (`--stop`) and broad process cleanup (`--force`) flows.

## Entrypoints

- `runStopMode` orchestrates PID, HTTP shutdown, and process-lookup fallback.
- `runForceCleanup` performs cross-process cleanup for install/upgrade recovery.

## Primary Flow

1. `runStopMode` logs invocation and attempts PID-file fast path first.
2. If PID fast path fails, `stopViaHTTP` calls `/shutdown`.
3. If HTTP shutdown fails, `stopViaProcessLookup` finds and terminates matching PIDs.
4. `runForceCleanup` logs lifecycle audit entry and chooses Unix/Windows cleanup strategy.
5. Platform cleanup sweeps process list, sends TERM/KILL, then clears PID files.
6. Summary output reports killed/failed counts and next-step guidance.

## Error and Recovery Paths

- Missing/stale PID file falls through to HTTP and process lookup fallback.
- Non-responsive daemon after TERM escalates to KILL.
- Platform command failures remain best-effort and always execute PID file cleanup.

## State and Contracts

- PID file ownership is treated as hint-only; runtime checks confirm process liveness.
- `cleanupPIDFiles` provides deterministic cleanup across common daemon ports.
- Human-readable stop output is contract-tested in stop command coverage tests.

## Code Paths

- `cmd/dev-console/main_connection_stop.go`
- `cmd/dev-console/main_connection_stop_strategies.go`
- `cmd/dev-console/main_connection_force_cleanup_strategies.go`

## Test Paths

- `cmd/dev-console/main_connection_diag_test.go`
- `cmd/dev-console/main_connection_coverage_test.go`
- `cmd/dev-console/main_connection_pid_contract_test.go`
- `cmd/dev-console/test_daemon_cleanup_test.go`

## Edit Guardrails

- Keep `runStopMode` orchestration-only; put mechanics in strategy helpers.
- Preserve fallback ordering (PID -> HTTP -> process lookup) for expected operator behavior.
- Keep force cleanup platform branching centralized and deterministic.
