---
doc_type: feature_index
feature_id: feature-mcp-persistent-server
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - cmd/dev-console/mcp_identity.go
  - cmd/dev-console/bridge.go
  - cmd/dev-console/bridge_startup_orchestration.go
  - internal/identity/mcp.go
  - internal/util/proc_unix.go
  - internal/util/proc_windows.go
test_paths:
  - cmd/dev-console/bridge_startup_contention_test.go
  - cmd/dev-console/bridge_faststart_extended_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# MCP Persistent Server

## TL;DR
- Status: shipped
- Scope: long-lived daemon lifecycle across client reconnects

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Related Architecture
- [Daemon Stop and Force Cleanup](../../../architecture/flow-maps/daemon-stop-and-force-cleanup.md)
