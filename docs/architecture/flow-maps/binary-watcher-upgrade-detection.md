---
doc_type: flow_map
flow_id: binary-watcher-upgrade-detection
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/main_connection_mcp_upgrade.go:installBinaryUpgradeHooks
  - cmd/dev-console/binary_watcher.go:startBinaryWatcher
code_paths:
  - cmd/dev-console/main_connection_mcp_upgrade.go
  - cmd/dev-console/binary_watcher.go
  - cmd/dev-console/config_modes.go
  - cmd/dev-console/binary_watcher_marker.go
  - cmd/dev-console/health_response_builders.go
  - cmd/dev-console/server_routes_health_diagnostics.go
test_paths:
  - cmd/dev-console/binary_watcher_test.go
---

# Binary Watcher Upgrade Detection

## Scope

Covers daemon self-upgrade detection based on on-disk binary changes, version verification, and restart guidance propagation through health/tool responses.

## Entrypoints

1. Startup hook installs binary upgrade callbacks in `installBinaryUpgradeHooks`.
2. `startBinaryWatcher` begins periodic file-change + version-check monitoring.

## Primary Flow

1. Resolve executable path and cache baseline file metadata (mtime + size).
2. Poll at configured watch interval; when metadata changes, verify `--version` and parse version from stdout or stderr.
3. If detected version is newer than current daemon version, set `upgradePending` state and detected timestamp.
4. Emit upgrade warning callback; write marker file for restart handoff.
5. After grace period, trigger controlled shutdown to allow process replacement.
6. On next startup, read-and-clear marker and expose upgrade info in health/tool responses.

## Error and Recovery Paths

1. Missing/invalid executable path: watcher initialization returns `nil` (feature silently disabled).
2. Version command failures/timeouts/invalid output: change is ignored, watcher continues polling.
3. Marker parse failures: invalid marker is discarded and removed to avoid repeated failure loops.

## State and Contracts

1. `BinaryWatcherState` is mutex-protected and exposes thread-safe `UpgradeInfo()`.
2. Per-watcher timing/version dependencies are injected via config to avoid package-global test coupling.
3. `checkForUpgrade` accepts per-state verifier + timeout injection for deterministic tests.
4. Version verification accepts canonical version lines from either stdout or stderr to match CLI `--version` behavior.

## Code Paths

- `cmd/dev-console/main_connection_mcp_upgrade.go`
- `cmd/dev-console/binary_watcher.go`
- `cmd/dev-console/config_modes.go`
- `cmd/dev-console/binary_watcher_marker.go`
- `cmd/dev-console/health_response_builders.go`
- `cmd/dev-console/server_routes_health_diagnostics.go`

## Test Paths

- `cmd/dev-console/binary_watcher_test.go`

## Edit Guardrails

1. Keep watcher config defaults centralized in `binary_watcher.go`; avoid reintroducing mutable package-level timing globals.
2. Prefer injected verifier/timeouts for tests over wall-clock sleeps.
3. Preserve marker-file compatibility (`from_version`, `to_version`, `timestamp`) across upgrades.
