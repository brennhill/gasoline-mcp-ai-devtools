---
doc_type: flow_map
flow_id: interact-navigate-and-document
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/tools_interact_workflow_navigate_document.go:handleNavigateAndDocument
code_paths:
  - cmd/dev-console/tools_interact_workflow_navigate_document.go
  - cmd/dev-console/tools_interact_response_helpers.go
  - cmd/dev-console/tools_interact_dispatch.go
  - cmd/dev-console/tools_pending_query_enqueue.go
  - internal/tools/interact/workflow.go
  - internal/schema/interact_actions.go
  - internal/schema/interact_properties_output_batch.go
  - internal/schema/interact_properties_targeting.go
  - internal/tools/configure/mode_specs_interact.go
test_paths:
  - cmd/dev-console/tools_interact_navigate_document_test.go
  - cmd/dev-console/tools_pending_query_enqueue_test.go
  - cmd/dev-console/tools_interact_screenshot_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Navigate and Document

## Scope

Covers `interact(what:"navigate_and_document")`: click-driven navigation with optional URL-change and DOM-stability waits, followed by structured page-context enrichment and stage-level workflow tracing.

## Entrypoints

- `handleNavigateAndDocument` orchestrates click -> waits -> response enrichment.

## Primary Flow

1. Parse workflow flags (`wait_for_url_change`, `wait_for_stable`, `timeout_ms`, `stability_ms`).
2. Snapshot pre-click tracked URL from capture state.
3. Execute click via the existing DOM primitive path (`handleDOMPrimitive(..., "click")`).
4. If enabled and baseline URL exists, poll tracked page URL until it differs or times out.
5. If enabled, run `wait_for_stable` via the existing DOM primitive handler.
6. Append compact page context (`url`, `title`, `tab_id`) to both legacy text content and `metadata.page_context`.
7. Attach normalized stage trace envelope to `metadata.workflow_trace` (`trace_id`, `status`, `stages[]` with timing/status).
8. Caller-level composables (`include_screenshot`, `include_interactive`) run after workflow completion in `toolInteract`.

## Error and Recovery Paths

- Invalid JSON returns `ErrInvalidJSON`.
- Missing click target propagates existing click validation error (`selector`/`element_id`/`index` required).
- Explicit `tab_id` without active tracking returns `ErrInvalidParam`; the workflow requires tracked-tab context for deterministic waits.
- `tab_id` mismatch (different from tracked tab) returns `ErrInvalidParam`; workflow waits/context are tracked-tab scoped.
- URL-change timeout returns `ErrExtTimeout` with guidance to increase timeout or disable URL-change waiting.
- Queue saturation returns fail-fast `queue_full` structured errors before waiting, via shared enqueue helper.
- Stability wait failure propagates the structured `wait_for_stable` error.
- Failed paths still return `metadata.workflow_trace` so callers can identify the exact failing stage.

## State and Contracts

- URL-change detection reads tracked tab URL from capture state, falling back to observe page context when needed.
- If `tab_id` is provided, an active tracked tab is required and it must match the tracked tab for deterministic workflow postconditions.
- If `timeout_ms` is provided, wait stages consume a shared remaining-budget window (not independent full timeouts per stage).
- All extension-dispatched sub-actions use shared enqueue semantics (`enqueuePendingQuery`) so queue saturation is surfaced consistently.
- Workflow defaults are conservative for navigation use-cases: URL-change wait enabled, stability wait enabled.
- Response enrichment is additive: base action result is preserved and page context text block is appended for backward compatibility.
- Machine-readable context contract: `metadata.page_context`.
- Stage trace contract: `metadata.workflow_trace` with deterministic stage names/order.

## Code Paths

- `cmd/dev-console/tools_interact_workflow_navigate_document.go`
- `cmd/dev-console/tools_interact_response_helpers.go`
- `cmd/dev-console/tools_interact_dispatch.go`
- `internal/tools/interact/workflow.go`
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
- Keep workflow trace envelope structure stable (`trace_id`, `status`, `stages[].{stage,started_at,completed_at,duration_ms,status,error}`).
- Preserve snake_case parameter naming and mode-spec/schema parity when adding workflow knobs.
