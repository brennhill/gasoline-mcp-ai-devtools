---
doc_type: flow_map
flow_id: bridge-startup-contention-and-convergence
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/bridge_startup_orchestration.go (runBridgeMode, startDaemonSpawnCoordinator)
  - cmd/dev-console/bridge.go (bridgeStdioToHTTPFast)
code_paths:
  - cmd/dev-console/bridge_startup.go
  - cmd/dev-console/bridge_startup_orchestration.go
  - cmd/dev-console/bridge_startup_lock.go
  - cmd/dev-console/bridge_startup_state.go
  - cmd/dev-console/bridge_startup_status.go
  - cmd/dev-console/bridge_forward.go
  - cmd/dev-console/bridge.go
test_paths:
  - cmd/dev-console/bridge_startup_lock_test.go
  - cmd/dev-console/bridge_spawn_race_test.go
  - cmd/dev-console/bridge_startup_contention_test.go
  - cmd/dev-console/bridge_faststart_extended_test.go
---

# Bridge Startup Contention and Convergence

## Scope

Covers bridge-mode daemon startup when multiple MCP clients start concurrently, including startup leadership election, follower convergence, and `tools/call` behavior during warm-up.

## Entrypoints

- `runBridgeMode` initializes `daemonState`, checks for existing daemon, and starts async startup coordination.
- `bridgeStdioToHTTPFast` handles MCP fast-path methods and startup-aware forwarding behavior.

## Primary Flow

1. Bridge checks for an existing compatible daemon with `tryConnectToExisting`.
2. If no compatible daemon exists, startup coordination runs asynchronously.
3. Coordination tries to acquire an exclusive startup lock file for the target port.
4. Lock holder becomes startup leader and spawns daemon immediately.
5. Non-leader bridges wait as followers and poll for peer daemon readiness.
6. If follower wait budget expires, stale/dead startup lock is reclaimed and leadership is retried.
7. `tools/call` requests during `starting` status are forwarded and allowed to converge instead of returning transient startup retry envelopes.
8. HTTP forward path still includes respawn-and-retry-once fallback for true connection failures.

## Error and Recovery Paths

- Non-gasoline service on port: marked failed with actionable error.
- Version mismatch against running gasoline daemon: bridge attempts controlled daemon recycle.
- Startup leader crash/stall: follower reclaims stale startup lock and takes over spawn.
- Forwarding connection error after startup: respawn once and retry request.

## State and Contracts

- Startup lock file (`bridge-startup-<port>.lock.json`) provides single-host startup leadership.
- `daemonState.readyCh` / `failedCh` signal readiness transitions for waiting callers.
- Startup budgets:
  - `daemonStartupGracePeriod` (tools/call warm-up wait)
  - `daemonStartupReadyTimeout` (spawn readiness target)
  - `daemonPeerWaitTimeout` (follower wait budget)
- Startup retry envelope is reserved for non-`tools/call` startup cases and true failure states.

## Code Paths

- `cmd/dev-console/bridge_startup.go`
- `cmd/dev-console/bridge_startup_orchestration.go`
- `cmd/dev-console/bridge_startup_lock.go`
- `cmd/dev-console/bridge_startup_state.go`
- `cmd/dev-console/bridge_startup_status.go`
- `cmd/dev-console/bridge_forward.go`
- `cmd/dev-console/bridge.go`

## Test Paths

- `cmd/dev-console/bridge_startup_lock_test.go`
- `cmd/dev-console/bridge_spawn_race_test.go`
- `cmd/dev-console/bridge_startup_contention_test.go`
- `cmd/dev-console/bridge_faststart_extended_test.go`

## Edit Guardrails

- Keep startup leadership single-owner per port; do not allow unbounded concurrent spawns.
- Preserve MCP fast-path responsiveness for `initialize`, `tools/list`, and `resources/*`.
- Keep startup wait budgets explicit and test-backed.
- Any change to startup error envelopes must preserve `tools/call` convergence behavior under contention.

## Algorithm Classification

This is not RAFT. It is a single-node, lock-file-based leader election with timeout-based failover and retry. There is no distributed quorum, replicated log, or term/commit protocol.
