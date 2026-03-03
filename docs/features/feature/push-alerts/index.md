---
doc_type: feature_index
feature_id: feature-push-alerts
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - cmd/dev-console/alerts.go
  - cmd/dev-console/streaming.go
  - cmd/dev-console/tools_configure_runtime_impl.go
  - internal/streaming/stream.go
  - internal/streaming/stream_emit.go
  - internal/streaming/types.go
  - internal/streaming/alerts_buffer.go
  - internal/identity/mcp.go
test_paths:
  - internal/streaming/stream_test.go
  - internal/streaming/alerts_test.go
  - cmd/dev-console/alerts_unit_test.go
---

# Push Alerts

## TL;DR

- Status: shipped
- Tool: observe
- Mode/Action: alert system
- Location: `docs/features/feature/push-alerts`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Related Architecture

- [Push Alert Notification Emission](../../../architecture/flow-maps/push-alert-notification-emission.md)

## Requirement IDs

- FEATURE_PUSH_ALERTS_001
- FEATURE_PUSH_ALERTS_002
- FEATURE_PUSH_ALERTS_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
