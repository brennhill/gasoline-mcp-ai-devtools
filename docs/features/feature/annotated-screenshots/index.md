---
doc_type: feature_index
feature_id: feature-annotated-screenshots
status: active
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - extension/content/draw-mode.js
  - internal/annotation/store.go
  - cmd/dev-console/tools_analyze_annotations_handlers.go
  - cmd/dev-console/annotation_store.go
test_paths:
  - tests/extension/draw-mode.test.js
  - internal/annotation/store_test.go
  - cmd/dev-console/tools_analyze_annotations_test.go
---

# Annotated Screenshots

## TL;DR

- Status: active
- Tool: analyze
- Mode/Action: annotations, annotation_detail
- Location: `docs/features/feature/annotated-screenshots`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Flow Maps

- [flow-map.md](./flow-map.md) — pointers to canonical flow maps

## Requirement IDs

- FEATURE_ANNOTATED_SCREENSHOTS_001
- FEATURE_ANNOTATED_SCREENSHOTS_002
- FEATURE_ANNOTATED_SCREENSHOTS_003

## Code and Tests

### Extension (DOM capture)
- `extension/content/draw-mode.js` — `buildElementDetail()`, `detectCSSFramework()`, parent_context, siblings

### Go (store + handler)
- `internal/annotation/store.go` — `Detail` struct with ParentContext, Siblings, CSSFramework fields; session TTL = 2 hours
- `cmd/dev-console/tools_analyze_annotations_handlers.go` — detail response enrichment, error correlation, LLM hints

### Tests
- `tests/extension/draw-mode.test.js` — "Element Detail Enrichment" describe block
- `internal/annotation/store_test.go` — `TestStore_SessionTTL_Is2Hours`
- `cmd/dev-console/tools_analyze_annotations_test.go` — enrichment fields, error correlation, hints tests
