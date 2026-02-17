---
doc_type: feature_index
feature_id: feature-link-health
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-02-17
code_paths:
  - cmd/dev-console/tools_analyze.go
  - src/lib/link-health.ts
  - src/background/pending-queries.ts
  - src/content/message-handlers.ts
  - src/inject/message-handlers.ts
test_paths:
  - cmd/dev-console/tools_analyze_validation_test.go
---

# Link Health

## TL;DR
- Status: shipped
- Tool: `analyze`
- Modes: `what:"link_health"` and `what:"link_validation"`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
