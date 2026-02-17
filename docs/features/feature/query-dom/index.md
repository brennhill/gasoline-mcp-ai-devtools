---
doc_type: feature_index
feature_id: feature-query-dom
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-02-17
code_paths:
  - cmd/dev-console/tools_analyze.go
  - src/background/pending-queries.ts
  - src/content/message-handlers.ts
  - src/inject/message-handlers.ts
  - src/lib/dom-queries.ts
test_paths:
  - cmd/dev-console/tools_analyze_handler_test.go
  - cmd/dev-console/tools_analyze_route_test.go
---

# Query DOM

## TL;DR
- Status: shipped
- Tool: `analyze`
- Mode: `what:"dom"`
- Legacy note: `analyze({what:"dom"})` is non-canonical

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
