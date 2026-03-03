---
doc_type: feature_index
feature_id: feature-batch-sequences
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - cmd/dev-console/tools_interact_batch.go
  - cmd/dev-console/tools_configure_sequence.go
  - cmd/dev-console/tools_configure_sequence_replay.go
  - cmd/dev-console/tools_configure_sequence_replay_steps.go
  - internal/schema/interact_properties_output_batch.go
test_paths:
  - cmd/dev-console/tools_interact_batch_test.go
  - cmd/dev-console/tools_configure_sequence_test.go
---

# Batch Sequences

## TL;DR
- Status: proposed
- Tools: `interact`, `configure`
- Actions: `batch`, `save_sequence`, `replay_sequence`

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Design Reference: [design-spec.md](./design-spec.md)

## Canonical Note
Batch execution and reusable configure sequences share step semantics; sequence replay should remain a thin layer over the core batch runner to keep behavior DRY and predictable.
