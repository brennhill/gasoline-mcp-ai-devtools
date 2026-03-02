---
doc_type: feature_index
feature_id: feature-analyze-tool
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-02
code_paths:
  - cmd/dev-console/tools_analyze_dispatch.go
  - cmd/dev-console/tools_analyze_annotations_handlers.go
  - cmd/dev-console/tools_analyze_api_validation.go
  - cmd/dev-console/tools_async_observe_commands.go
  - cmd/dev-console/tools_async_formatting.go
  - cmd/dev-console/tools_security_audit.go
  - internal/annotation/store.go
  - internal/annotation/store_results.go
  - internal/annotation/store_wait.go
  - internal/schema/analyze.go
  - src/background/pending-queries.ts
  - src/content/message-handlers.ts
  - src/inject/message-handlers.ts
test_paths:
  - cmd/dev-console/tools_analyze_annotations_test.go
  - cmd/dev-console/tools_analyze_handler_test.go
  - internal/annotation/store_test.go
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
- Flow Map: `flow-map.md`

## Canonical Note
`analyze` is the active analysis surface. `analyze({what:"dom"})` is the canonical DOM query API.
