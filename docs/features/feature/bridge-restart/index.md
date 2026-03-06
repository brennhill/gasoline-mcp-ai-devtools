---
doc_type: feature_index
feature_id: feature-bridge-restart
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-02-18
code_paths:
  - cmd/dev-console/bridge.go
  - cmd/dev-console/tools_configure.go
  - cmd/dev-console/tools_schema.go
test_paths:
  - cmd/dev-console/bridge_test.go
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

## Requirement IDs

- FEATURE_BRIDGE_RESTART_001: Force-restart daemon from bridge when unresponsive
- FEATURE_BRIDGE_RESTART_002: Daemon-side restart via self-SIGTERM when responsive
- FEATURE_BRIDGE_RESTART_003: Recovery from frozen (SIGSTOP'd) daemon processes

## Code and Tests

| File | Purpose |
|------|---------|
| `cmd/dev-console/bridge.go` | `extractToolAction()`, `forceKillOnPort()`, `handleBridgeRestart()` |
| `cmd/dev-console/tools_configure.go` | `toolConfigureRestart()` daemon-side handler |
| `cmd/dev-console/tools_schema.go` | Schema: `restart` in configure action enum + oneOf |
| `cmd/dev-console/bridge_test.go` | Unit tests for `extractToolAction()` |
