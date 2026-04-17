---
doc_type: flow_map
flow_id: test-generation-dispatch
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - cmd/browser-agent/tools_generate.go:handleGenerate
  - cmd/browser-agent/testgen.go:testGenHandler.handleGenerateTestFromContext
  - cmd/browser-agent/testgen_heal.go:testGenHandler.handleGenerateTestHeal
  - cmd/browser-agent/testgen_classify.go:testGenHandler.handleGenerateTestClassify
code_paths:
  - cmd/browser-agent/tools_generate.go
  - cmd/browser-agent/tools_generate_testgen_handler.go
  - cmd/browser-agent/testgen.go
  - cmd/browser-agent/testgen_aliases.go
  - cmd/browser-agent/testgen_provider_adapter.go
  - cmd/browser-agent/testgen_heal.go
  - cmd/browser-agent/testgen_classify.go
  - internal/testgen/generate.go
  - internal/testgen/helpers.go
test_paths:
  - cmd/browser-agent/testgen_context_test.go
  - cmd/browser-agent/testgen_generate_test.go
  - cmd/browser-agent/testgen_heal_test.go
  - cmd/browser-agent/testgen_classify_dispatch_test.go
  - internal/testgen/generate_test.go
  - internal/testgen/helpers_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation Dispatch

## Scope

Covers MCP `generate` test-related modes: `test_from_context`, `test_heal`, and `test_classify`.

## Entrypoints

- `handleGenerate` dispatches `generate` calls to test handlers.
- `testGenHandler.handleGenerateTestFromContext` handles contextual test generation.
- `testGenHandler.handleGenerateTestHeal` handles selector healing workflows.
- `testGenHandler.handleGenerateTestClassify` handles failure classification workflows.

## Primary Flow

1. MCP client calls `tools/call` with `name: "generate"`.
2. `handleGenerate` normalizes params and dispatches to `h.testGen()` sub-handler by mode.
3. `testGenHandler.handleGenerateTestFromContext` validates context and picks generator:
4. `error` context uses captured logs + actions to reproduce failures.
5. `interaction` context converts action history to Playwright script.
6. `regression` context adds baseline assertions.
7. Data access is mediated through `toolHandlerDataProvider` bound to `testGenHandler`.
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

- `cmd/browser-agent/tools_generate_testgen_handler.go`
- `cmd/browser-agent/testgen.go`
- `cmd/browser-agent/testgen_aliases.go`
- `cmd/browser-agent/testgen_provider_adapter.go`
- `cmd/browser-agent/testgen_heal.go`
- `cmd/browser-agent/testgen_classify.go`
- `internal/testgen/generate.go`
- `internal/testgen/helpers.go`

## Test Paths

- `cmd/browser-agent/testgen_context_test.go`
- `cmd/browser-agent/testgen_generate_test.go`
- `cmd/browser-agent/testgen_heal_test.go`
- `cmd/browser-agent/testgen_classify_dispatch_test.go`
- `internal/testgen/generate_test.go`
- `internal/testgen/helpers_test.go`

## Edit Guardrails

- Keep dispatch/validation logic separate from provider data access.
- Avoid reintroducing mixed responsibilities into `testgen.go`.
- Update this flow map and listed tests whenever context/action contracts change.
