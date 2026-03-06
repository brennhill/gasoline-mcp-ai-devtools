---
doc_type: feature_index
feature_id: feature-interact-explore
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-02-17
code_paths:
  - cmd/dev-console/tools_interact.go
  - cmd/dev-console/tools_interact_draw.go
  - cmd/dev-console/tools_interact_upload.go
  - src/background/pending-queries.ts
  - src/background/query-execution.ts
  - src/content/runtime-message-listener.ts
test_paths:
  - cmd/dev-console/tools_interact_rich_test.go
  - cmd/dev-console/tools_interact_upload_test.go
  - cmd/dev-console/tools_interact_state_test.go
---

# Interact Tool

## TL;DR
- Status: shipped
- Tool: `interact`
- Mode key: `action`
- Contract source: `cmd/dev-console/tools_schema.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`

## Canonical Note
This feature documents the shipped `interact` action surface (not a batched `interact.explore` action).
