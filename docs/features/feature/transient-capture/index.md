---
doc_type: feature_index
feature_id: feature-transient-capture
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - cmd/dev-console/tools_async_transient.go
  - src/lib/transient-capture.ts
test_paths:
  - cmd/dev-console/tools_async_transient_test.go
  - internal/tools/observe/handlers_transients_test.go
---

# Transient Capture

## TL;DR
- Status: shipped
- Scope: capture short-lived UI state for debugging/observe workflows

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Canonical Note
Transient capture should prefer event/timeline-safe snapshots and avoid introducing observer-induced UI mutations.
