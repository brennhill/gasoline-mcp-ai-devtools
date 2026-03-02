---
doc_type: flow_map
flow_id: observe-dispatch-and-augmentation
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/tools_observe.go:toolObserve
  - cmd/dev-console/tools_observe.go:resolveObserveMode
code_paths:
  - cmd/dev-console/tools_observe.go
  - cmd/dev-console/tools_observe_registry.go
  - cmd/dev-console/tools_observe_response.go
  - cmd/dev-console/tools_observe_analysis.go
  - cmd/dev-console/tools_shared_queries.go
  - internal/a11ysummary/summary.go
  - cmd/dev-console/tools_observe_bundling.go
  - internal/tools/observe/
test_paths:
  - cmd/dev-console/tools_observe_handler_test.go
  - cmd/dev-console/tools_observe_blackbox_test.go
  - cmd/dev-console/tools_observe_audit_test.go
  - cmd/dev-console/tools_observe_analysis_test.go
  - internal/a11ysummary/summary_test.go
  - cmd/dev-console/tools_observe_unit_test.go
  - cmd/dev-console/tools_schema_parity_test.go
---

# Observe Dispatch and Augmentation

## Scope

Covers the `observe` tool entrypoint, mode selection, handler dispatch, and post-dispatch response augmentation.

## Entrypoints

- `toolObserve` parses mode params and routes requests.
- `resolveObserveMode` normalizes `what` plus deprecated aliases (`mode`, `action`).

## Primary Flow

1. MCP client sends `tools/call` with `name: "observe"`.
2. `toolObserve` parses request args and validates selector params.
3. `resolveObserveMode` canonicalizes mode and applies alias mapping.
4. Dispatcher looks up canonical mode in `observeHandlers`.
5. Handler executes:
6. Most read modes delegate to `internal/tools/observe`.
7. Async/recording-related modes stay in local handler methods.
8. Response is post-processed:
9. Adds disconnect warning for extension-dependent modes.
10. Appends pending alerts as a second content block.
11. Alias usage warning is appended when deprecated params were used.

## Error and Recovery Paths

- Invalid JSON args return `ErrInvalidJSON`.
- Missing mode returns `ErrMissingParam` with valid mode hint.
- Unknown mode returns `ErrUnknownMode` with canonical mode list.
- Conflicting `what` vs alias values return alias conflict response.

## State and Contracts

- `observeHandlers` is the source of truth for mode availability.
- `serverSideObserveModes` defines which modes skip disconnect warnings.
- Schema parity tests must stay aligned with `observeHandlers` keys.
- Accessibility summary payloads are normalized through `internal/a11ysummary` so canonical keys (`violations`, `passes`, `incomplete`, `inapplicable`) and legacy aliases (`*_count`) remain synchronized.

## Code Paths

- `cmd/dev-console/tools_observe.go`
- `cmd/dev-console/tools_observe_registry.go`
- `cmd/dev-console/tools_observe_response.go`
- `cmd/dev-console/tools_observe_analysis.go`
- `cmd/dev-console/tools_shared_queries.go`
- `cmd/dev-console/tools_observe_bundling.go`
- `internal/a11ysummary/summary.go`
- `internal/tools/observe/`

## Test Paths

- `cmd/dev-console/tools_observe_handler_test.go`
- `cmd/dev-console/tools_observe_blackbox_test.go`
- `cmd/dev-console/tools_observe_audit_test.go`
- `cmd/dev-console/tools_observe_analysis_test.go`
- `internal/a11ysummary/summary_test.go`
- `cmd/dev-console/tools_observe_unit_test.go`
- `cmd/dev-console/tools_schema_parity_test.go`

## Edit Guardrails

- Keep mode registry changes in `tools_observe_registry.go`.
- Keep argument parsing/validation in `tools_observe.go`.
- Keep response decoration in `tools_observe_response.go`.
- Update this flow map and observe feature index when mode keys or file ownership changes.
