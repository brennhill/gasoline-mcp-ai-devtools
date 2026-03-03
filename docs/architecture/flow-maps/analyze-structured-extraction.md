---
doc_type: flow_map
flow_id: analyze-structured-extraction
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
---

# Analyze Structured Extraction

## Scope

Structured page data extraction for `analyze` modes `form_state` and `data_table`, designed to replace fragile `execute_js` parsing pipelines.

## Entrypoints

1. `analyze({what:"form_state"})` in `cmd/dev-console/tools_analyze_dispatch.go`.
2. `analyze({what:"data_table"})` in `cmd/dev-console/tools_analyze_dispatch.go`.

## Primary Flow

1. MCP `analyze` dispatch resolves mode to `toolFormState` or `toolDataTable`.
2. Go server enqueues pending query (`form_state` or `data_table`) with correlation metadata.
3. Extension `handlePendingQuery` routes to `registerCommand('form_state'|'data_table')`.
4. Background command sends content-script message (`FORM_STATE_QUERY` or `DATA_TABLE_QUERY`) for the resolved tab.
5. Content script forwards to inject context with nonce-scoped postMessage (`GASOLINE_FORM_STATE_QUERY` / `GASOLINE_DATA_TABLE_QUERY`).
6. Inject handlers execute deterministic extractors:
   - `discoverForms(..., mode:'discover')` for form state.
   - `extractDataTables(...)` for HTML table rows/headers.
7. Structured JSON result returns through content -> background -> `/sync` -> MCP response.

## Error and Recovery Paths

1. Invalid JSON args in Go parsing return `invalid_json` MCP structured errors.
2. Missing content script/inject bridge failures return `form_state_failed` or `data_table_failed`.
3. Inject extraction failures return `form_state_error` or `data_table_error` with message context.

## State and Contracts

1. `form_state` returns `{ forms: [...], count }`.
2. `data_table` returns `{ tables: [...], count }`, with per-table `headers`, `rows`, `row_count`, and `column_count`.
3. `selector` is optional for both modes; `data_table` also supports `max_rows` and `max_cols`.
4. All payload fields are snake_case.

## Code Paths

- `cmd/dev-console/tools_analyze_dispatch.go`
- `cmd/dev-console/tools_analyze_inspect_forms.go`
- `internal/schema/analyze.go`
- `internal/tools/configure/mode_specs_analyze.go`
- `src/background/commands/analyze.ts`
- `src/background/commands/helpers.ts`
- `src/content/runtime-message-listener.ts`
- `src/content/message-handlers.ts`
- `src/inject/message-handlers.ts`
- `src/inject/form-discovery.ts`
- `src/inject/data-table.ts`
- `src/types/runtime-messages.ts`

## Test Paths

- `cmd/dev-console/tools_analyze_structured_extraction_test.go`
- `internal/tools/analyze/forms_test.go`
- `tests/extension/data-table.test.js`

## Edit Guardrails

1. Keep query/mode naming aligned across Go and extension (`form_state`, `data_table`).
2. Preserve nonce-validated inject message forwarding for new extraction modes.
3. Do not route these modes through `execute_js`; keep dedicated extraction paths deterministic.
4. Update analyze schema enum + `describe_capabilities` mode specs whenever extraction modes change.

