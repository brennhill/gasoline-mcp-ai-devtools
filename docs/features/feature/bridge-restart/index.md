---
doc_type: feature_index
feature_id: feature-bridge-restart
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-03-02
code_paths:
  - cmd/dev-console/bridge.go
  - cmd/dev-console/bridge_startup.go
  - cmd/dev-console/bridge_startup_orchestration.go
  - cmd/dev-console/bridge_startup_lock.go
  - cmd/dev-console/bridge_startup_state.go
  - cmd/dev-console/bridge_startup_status.go
  - cmd/dev-console/tools_configure.go
  - cmd/dev-console/tools_schema.go
test_paths:
  - cmd/dev-console/bridge_test.go
  - cmd/dev-console/bridge_spawn_race_test.go
  - cmd/dev-console/bridge_startup_lock_test.go
  - cmd/dev-console/bridge_startup_contention_test.go
  - cmd/dev-console/bridge_faststart_extended_test.go
---

# Bridge Restart

## TL;DR

- Status: implemented
- Tool: `configure`
- Action: `restart`
- Location: `docs/features/feature/bridge-restart`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- Test Plan: [test-plan.md](./test-plan.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)
- Canonical Startup Flow: [Bridge Startup Contention and Convergence](../../../architecture/flow-maps/bridge-startup-contention-and-convergence.md)

## Requirement IDs

- FEATURE_BRIDGE_RESTART_001: Force-restart daemon from bridge when unresponsive
- FEATURE_BRIDGE_RESTART_002: Daemon-side restart via self-SIGTERM when responsive
- FEATURE_BRIDGE_RESTART_003: Recovery from frozen (SIGSTOP'd) daemon processes

## Code and Tests

| File | Purpose |
|------|---------|
| `cmd/dev-console/bridge.go` | Startup-aware forwarding for `tools/call` during daemon warm-up |
| `cmd/dev-console/bridge_startup_orchestration.go` | Startup coordinator: leader election, follower wait, stale-lock takeover |
| `cmd/dev-console/bridge_startup_lock.go` | Lock-file startup leadership (`bridge-startup-<port>.lock.json`) |
| `cmd/dev-console/bridge_startup_state.go` | Daemon readiness/failed signaling and respawn behavior |
| `cmd/dev-console/tools_configure.go` | `toolConfigureRestart()` daemon-side handler |
| `cmd/dev-console/tools_schema.go` | Schema: `restart` in configure action enum + oneOf |
| `cmd/dev-console/bridge_test.go` | Unit tests for `extractToolAction()` |
| `cmd/dev-console/bridge_startup_contention_test.go` | Multi-client startup convergence integration test |
