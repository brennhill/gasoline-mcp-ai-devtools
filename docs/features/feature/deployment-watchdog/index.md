---
doc_type: feature_index
feature_id: feature-deployment-watchdog
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - cmd/dev-console/main_connection_mcp_upgrade.go
  - cmd/dev-console/binary_watcher.go
  - cmd/dev-console/binary_watcher_marker.go
  - cmd/dev-console/health_response_builders.go
  - cmd/dev-console/server_routes_health_diagnostics.go
  - cmd/dev-console/handler_tools_call_postprocess.go
test_paths:
  - cmd/dev-console/binary_watcher_test.go
---

# Deployment Watchdog

## TL;DR

- Status: proposed
- Tool: configure, observe
- Mode/Action: watchdog, deployment_status
- Location: `docs/features/feature/deployment-watchdog`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_DEPLOYMENT_WATCHDOG_001
- FEATURE_DEPLOYMENT_WATCHDOG_002
- FEATURE_DEPLOYMENT_WATCHDOG_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
