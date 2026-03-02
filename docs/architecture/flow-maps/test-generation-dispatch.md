---
doc_type: flow_map
flow_id: test-generation-dispatch
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/tools_generate.go:handleGenerate
  - cmd/dev-console/testgen.go:handleGenerateTestFromContext
  - cmd/dev-console/testgen_heal.go:handleGenerateTestHeal
  - cmd/dev-console/testgen_classify.go:handleGenerateTestClassify
code_paths:
  - cmd/dev-console/tools_generate.go
  - cmd/dev-console/testgen.go
  - cmd/dev-console/testgen_aliases.go
  - cmd/dev-console/testgen_provider_adapter.go
  - cmd/dev-console/testgen_heal.go
  - cmd/dev-console/testgen_classify.go
  - internal/testgen/generate.go
  - internal/testgen/helpers.go
test_paths:
  - cmd/dev-console/testgen_context_test.go
  - cmd/dev-console/testgen_generate_test.go
  - cmd/dev-console/testgen_heal_test.go
  - cmd/dev-console/testgen_classify_dispatch_test.go
  - internal/testgen/generate_test.go
  - internal/testgen/helpers_test.go
---

# Test Generation Dispatch

## Scope

Covers MCP `generate` test-related modes: `test_from_context`, `test_heal`, and `test_classify`.

## Entrypoints

- `handleGenerate` dispatches `generate` calls to test handlers.
- `handleGenerateTestFromContext` handles contextual test generation.
- `handleGenerateTestHeal` handles selector healing workflows.
- `handleGenerateTestClassify` handles failure classification workflows.

## Primary Flow

1. MCP client calls `tools/call` with `name: "generate"`.
2. `handleGenerate` normalizes params and dispatches to test handlers by mode.
3. `handleGenerateTestFromContext` validates context and picks generator:
4. `error` context uses captured logs + actions to reproduce failures.
5. `interaction` context converts action history to Playwright script.
6. `regression` context adds baseline assertions.
7. Data access is mediated through `toolHandlerDataProvider`.
8. Internal logic lives in `internal/testgen/*`.
9. Response is returned through structured MCP JSON + warnings.

## Error and Recovery Paths

- Invalid JSON returns `ErrInvalidJSON`.
- Missing/invalid mode params return `ErrMissingParam` / `ErrInvalidParam`.
- Domain-specific failures are mapped via `testgen.ErrorMappings`.
- Uncertain classification returns `ErrClassificationUncertain`.

## State and Contracts

- Provider methods expose captured entries/actions/network bodies only.
- `testGenContextDispatch` is the source of truth for valid context values.
- Error code/message mapping must remain consistent with client expectations.

## Code Paths

- `cmd/dev-console/testgen.go`
- `cmd/dev-console/testgen_aliases.go`
- `cmd/dev-console/testgen_provider_adapter.go`
- `cmd/dev-console/testgen_heal.go`
- `cmd/dev-console/testgen_classify.go`
- `internal/testgen/generate.go`
- `internal/testgen/helpers.go`

## Test Paths

- `cmd/dev-console/testgen_context_test.go`
- `cmd/dev-console/testgen_generate_test.go`
- `cmd/dev-console/testgen_heal_test.go`
- `cmd/dev-console/testgen_classify_dispatch_test.go`
- `internal/testgen/generate_test.go`
- `internal/testgen/helpers_test.go`

## Edit Guardrails

- Keep dispatch/validation logic separate from provider data access.
- Avoid reintroducing mixed responsibilities into `testgen.go`.
- Update this flow map and listed tests whenever context/action contracts change.
