---
doc_type: feature_index
feature_id: feature-analyze-tool
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-02-17
code_paths:
  - cmd/dev-console/tools_analyze.go
  - cmd/dev-console/tools_analyze_annotations.go
  - cmd/dev-console/tools_security.go
  - src/background/pending-queries.ts
  - src/content/message-handlers.ts
  - src/inject/message-handlers.ts
test_paths:
  - cmd/dev-console/tools_analyze_validation_test.go
  - cmd/dev-console/tools_analyze_route_test.go
  - cmd/dev-console/tools_analyze_handler_test.go
---

# Analyze Tool

## TL;DR
- Status: shipped
- Tool: `analyze`
- Mode key: `what`
- Contract source: `cmd/dev-console/tools_schema.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`

## Canonical Note
`analyze` is the active analysis surface. `analyze({what:"dom"})` is the canonical DOM query API.
