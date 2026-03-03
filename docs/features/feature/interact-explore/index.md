---
doc_type: feature_index
feature_id: feature-interact-explore
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-02
code_paths:
  - cmd/dev-console/tools_interact_dispatch.go
  - cmd/dev-console/tools_interact_browser.go
  - cmd/dev-console/tools_interact_browser_navigation_impl.go
  - cmd/dev-console/tools_interact_browser_script_impl.go
  - cmd/dev-console/tools_interact_browser_tabs.go
  - cmd/dev-console/tools_interact_browser_util_impl.go
  - cmd/dev-console/tools_interact_response_helpers.go
  - cmd/dev-console/tools_interact_draw.go
  - cmd/dev-console/tools_interact_dom.go
  - cmd/dev-console/tools_interact_elements.go
  - cmd/dev-console/tools_interact_storage.go
  - cmd/dev-console/tools_interact_upload.go
  - cmd/dev-console/tools_interact_evidence.go
  - cmd/dev-console/tools_interact_retry_contract_strategy.go
  - cmd/dev-console/tools_interact_retry_contract_state.go
  - cmd/dev-console/tools_interact_retry_contract_response.go
  - cmd/dev-console/tools_interact_workflow_navigate.go
  - cmd/dev-console/tools_interact_workflow_forms.go
  - cmd/dev-console/tools_interact_workflow_a11y_sarif.go
  - cmd/dev-console/tools_interact_workflow_types.go
  - src/background/pending-queries.ts
  - src/background/query-execution.ts
  - src/background/dom-dispatch.ts
  - src/background/dom-types.ts
  - src/background/dom-primitives.ts
  - src/content/runtime-message-listener.ts
test_paths:
  - cmd/dev-console/tools_interact_rich_test.go
  - cmd/dev-console/tools_interact_upload_test.go
  - cmd/dev-console/tools_interact_retry_contract_test.go
  - cmd/dev-console/tools_interact_evidence_test.go
  - cmd/dev-console/tools_interact_state_test.go
  - extension/background/__tests__/dom-dispatch-structured.test.js
  - extension/background/dom-primitives.test.js
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
- Flow Map Pointer: `flow-map.md`

## Canonical Note
This feature documents the shipped `interact` action surface (not a batched `interact.explore` action).

`get_text` supports `structured:true` for hierarchical extraction (for example accordion/list sections), and this option must be forwarded through DOM dispatch into extension primitives.
