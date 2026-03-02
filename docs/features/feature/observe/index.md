---
doc_type: feature_index
feature_id: feature-observe
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-02
code_paths:
  - cmd/dev-console/tools_observe.go
  - cmd/dev-console/tools_observe_registry.go
  - cmd/dev-console/tools_observe_response.go
  - cmd/dev-console/tools_observe_analysis.go
  - cmd/dev-console/tools_shared_queries.go
  - cmd/dev-console/tools_observe_bundling.go
  - cmd/dev-console/observe_filtering.go
  - internal/a11ysummary/summary.go
  - internal/tools/observe/analysis_a11y.go
  - internal/tools/observe/analysis_screenshot.go
  - internal/capture/queries.go
  - internal/capture/sync.go
test_paths:
  - cmd/dev-console/tools_observe_handler_test.go
  - cmd/dev-console/tools_observe_blackbox_test.go
  - cmd/dev-console/tools_observe_audit_test.go
  - cmd/dev-console/tools_observe_screenshot_test.go
  - cmd/dev-console/tools_observe_analysis_test.go
  - internal/a11ysummary/summary_test.go
  - internal/tools/observe/analysis_test.go
  - internal/tools/observe/analysis_save_test.go
---

# Observe

## TL;DR
- Status: shipped
- Tool: `observe`
- Mode key: `what`
- Contract source: `cmd/dev-console/tools_schema.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map: `flow-map.md`

## Canonical Note
`observe` is the passive read surface for captured browser/server state. It is the canonical polling surface for async command completion via `what:"command_result"`.

Accessibility (`what:"accessibility"`) normalizes `summary` counts with canonical keys (`violations`, `passes`, `incomplete`, `inapplicable`) and preserves legacy aliases (`*_count`) for compatibility.
