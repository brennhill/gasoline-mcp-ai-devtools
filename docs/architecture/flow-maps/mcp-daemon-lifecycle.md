---
doc_type: flow_map
flow_id: mcp-daemon-lifecycle
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/main.go:dispatchMode
  - cmd/dev-console/main_connection_mcp.go:runMCPMode
code_paths:
  - cmd/dev-console/main_connection_mcp.go
  - cmd/dev-console/main_connection_mcp_bootstrap.go
  - cmd/dev-console/main_connection_mcp_upgrade.go
  - cmd/dev-console/main_connection_mcp_shutdown.go
  - cmd/dev-console/daemon_lifecycle.go
  - cmd/dev-console/daemon_lock_file.go
  - cmd/dev-console/server_routes.go
test_paths:
  - cmd/dev-console/main_connection_coverage_test.go
  - cmd/dev-console/main_connection_diag_test.go
  - cmd/dev-console/main_connection_pid_contract_test.go
  - cmd/dev-console/daemon_lifecycle_policy_test.go
  - cmd/dev-console/runtime_mode_test.go
---

# MCP Daemon Lifecycle

## Scope

Covers daemon startup, HTTP bind, PID/lock persistence, upgrade watcher wiring, and shutdown semantics.

## Entrypoints

- `dispatchMode` selects daemon vs bridge runtime.
- `runMCPMode` orchestrates daemon boot and lifecycle.

## Primary Flow

1. `dispatchMode` calls `runMCPMode` in daemon mode.
2. `runMCPMode` initializes capture and routes.
3. Startup loops begin (`startVersionCheckLoop`, screenshot limiter cleanup).
4. Upgrade monitoring is configured (`configureBinaryUpgradeMonitoring`).
5. Startup policy + stale PID + port preflight checks run.
6. HTTP server is created and bound (`startHTTPServer`).
7. PID file and daemon lock are persisted.
8. Runtime blocks on `awaitShutdownSignal`.

## Error and Recovery Paths

- Port bind failures return early with lifecycle logging.
- Stale PID with owner mismatch is removed before bind retry.
- Unexpected HTTP listener exit triggers synthetic shutdown to avoid zombie daemon.
- Upgrade detection writes marker and triggers SIGTERM for controlled restart.

## State and Contracts

- `binaryUpgradeState` is shared read state for health/handler warnings.
- PID file + daemon lock reflect active owner and are cleared on shutdown.
- `httpDone` channel is the listener liveness signal.

## Code Paths

- `cmd/dev-console/main_connection_mcp.go`
- `cmd/dev-console/main_connection_mcp_bootstrap.go`
- `cmd/dev-console/main_connection_mcp_upgrade.go`
- `cmd/dev-console/main_connection_mcp_shutdown.go`
- `cmd/dev-console/daemon_lifecycle.go`
- `cmd/dev-console/daemon_lock_file.go`

## Test Paths

- `cmd/dev-console/main_connection_coverage_test.go`
- `cmd/dev-console/main_connection_diag_test.go`
- `cmd/dev-console/main_connection_pid_contract_test.go`
- `cmd/dev-console/daemon_lifecycle_policy_test.go`
- `cmd/dev-console/runtime_mode_test.go`

## Edit Guardrails

- Keep `runMCPMode` orchestration-only; push details into helper files.
- Any change in lifecycle ordering must update this flow map and tests above.
- Preserve structured lifecycle event names for diagnostics compatibility.
