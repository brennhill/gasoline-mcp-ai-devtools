---
doc_type: flow_map
flow_id: interact-navigate-and-document
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/tools_interact_workflow_navigate_document.go:handleNavigateAndDocument
code_paths:
  - cmd/dev-console/tools_interact_workflow_navigate_document.go
  - cmd/dev-console/tools_interact_response_helpers.go
  - cmd/dev-console/tools_interact_dispatch.go
  - internal/schema/interact_actions.go
  - internal/schema/interact_properties_output_batch.go
  - internal/schema/interact_properties_targeting.go
  - internal/tools/configure/mode_specs_interact.go
test_paths:
  - cmd/dev-console/tools_interact_navigate_document_test.go
  - cmd/dev-console/tools_interact_screenshot_test.go
---

# Interact Navigate and Document

## Scope

Covers `interact(what:"navigate_and_document")`: click-driven navigation with optional URL-change and DOM-stability waits, followed by page-context enrichment.

## Entrypoints

- `handleNavigateAndDocument` orchestrates click -> waits -> response enrichment.

## Primary Flow

1. Parse workflow flags (`wait_for_url_change`, `wait_for_stable`, `timeout_ms`, `stability_ms`).
2. Snapshot pre-click tracked URL from capture state.
3. Execute click via the existing DOM primitive path (`handleDOMPrimitive(..., "click")`).
4. If enabled and baseline URL exists, poll tracked page URL until it differs or times out.
5. If enabled, run `wait_for_stable` via the existing DOM primitive handler.
6. Append compact page context (`url`, `title`, `tab_id`) to the response body.
7. Caller-level composables (`include_screenshot`, `include_interactive`) run after workflow completion in `toolInteract`.

## Error and Recovery Paths

- Invalid JSON returns `ErrInvalidJSON`.
- Missing click target propagates existing click validation error (`selector`/`element_id`/`index` required).
- URL-change timeout returns `ErrExtTimeout` with guidance to increase timeout or disable URL-change waiting.
- Stability wait failure propagates the structured `wait_for_stable` error.

## State and Contracts

- URL-change detection reads tracked tab URL from capture state, falling back to observe page context when needed.
- Workflow defaults are conservative for navigation use-cases: URL-change wait enabled, stability wait enabled.
- Response enrichment is additive: base action result is preserved and page context is appended.

## Code Paths

- `cmd/dev-console/tools_interact_workflow_navigate_document.go`
- `cmd/dev-console/tools_interact_response_helpers.go`
- `cmd/dev-console/tools_interact_dispatch.go`
- `internal/schema/interact_actions.go`
- `internal/schema/interact_properties_output_batch.go`
- `internal/schema/interact_properties_targeting.go`
- `internal/tools/configure/mode_specs_interact.go`

## Test Paths

- `cmd/dev-console/tools_interact_navigate_document_test.go`
- `cmd/dev-console/tools_interact_screenshot_test.go`

## Edit Guardrails

- Keep click execution delegated to existing DOM primitive handlers; do not fork click semantics inside the workflow.
- Keep page-context extraction in response helpers to reuse across workflows.
- Preserve snake_case parameter naming and mode-spec/schema parity when adding workflow knobs.
