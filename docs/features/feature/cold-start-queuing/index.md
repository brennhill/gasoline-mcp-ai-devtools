---
doc_type: feature_index
feature_id: feature-cold-start-queuing
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - cmd/dev-console/tools_core.go
  - cmd/dev-console/tools_async_helpers.go
  - internal/capture/state.go
test_paths:
  - cmd/dev-console/tools_coldstart_gate_test.go
---

# Cold-Start Queuing

## TL;DR
- Status: implemented
- Scope: extension-readiness gating before tool execution
- Default timeout: 5s

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Canonical Note
Readiness wait occurs once at the gate; async/background paths return queued immediately and should not double-block.
