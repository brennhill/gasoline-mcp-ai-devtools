---
doc_type: flow_map
flow_id: llm-fast-verify-gate
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - Makefile:verify-llm
  - .github/workflows/ci.yml:verify-llm
code_paths:
  - Makefile
  - .github/workflows/ci.yml
  - scripts/generate-wire-types.js
  - scripts/lint-documentation.py
  - scripts/docs/check-feature-bundles.js
  - scripts/docs/check-gokaboom-content-contract.mjs
  - scripts/docs/check-reference-schema-sync.mjs
  - cmd/browser-agent/tools_schema_parity_test.go
  - cmd/browser-agent/tools_interact_navigate_document_test.go
  - cmd/browser-agent/tools_contract_enforcement_test.go
test_paths:
  - scripts/docs/check-feature-bundles.test.mjs
  - cmd/browser-agent/tools_schema_parity_test.go
  - cmd/browser-agent/tools_interact_navigate_document_test.go
  - cmd/browser-agent/tools_contract_enforcement_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# LLM Fast Verify Gate

## Scope

Covers the fast, high-signal `verify-llm` quality gate for LLM maintenance loops and optional scoped CI execution on relevant PRs.

## Entrypoints

1. Local command: `make verify-llm`.
2. CI job: `verify-llm` in `.github/workflows/ci.yml`.

## Primary Flow

1. Check wire-schema drift via `scripts/generate-wire-types.js --check`.
2. Enforce scoped docs integrity lint (`docs:lint:integrity` for `docs/features` + `docs/architecture`).
3. Enforce strict feature docs bundle checks (`docs:check:strict`).
4. Enforce docs content contract and reference/schema sync.
5. Run focused Go tests for schema parity and key interact workflow contracts.
6. Return fast pass/fail signal for maintainers before full CI.

## Scoped CI Flow

1. On PRs, detect changed files versus base branch.
2. If changes touch tool/schema/docs surfaces, run `make verify-llm`.
3. Otherwise skip with explicit logging (optional gate behavior).

## Error and Recovery Paths

1. Any gate stage fails fast and exits non-zero.
2. Scoped CI false negatives are mitigated by broad path match patterns over tool/schema/docs roots.
3. Full CI jobs remain authoritative for complete validation beyond this fast gate.

## State and Contracts

1. Runtime target for warm-cache execution is approximately 60-120 seconds.
2. `verify-llm` is additive and does not replace full `ci` checks.
3. Gate is deterministic: no network calls required for pass/fail.

## Code Paths

- `Makefile`
- `.github/workflows/ci.yml`
- `scripts/generate-wire-types.js`
- `scripts/lint-documentation.py`
- `scripts/docs/check-feature-bundles.js`
- `scripts/docs/check-gokaboom-content-contract.mjs`
- `scripts/docs/check-reference-schema-sync.mjs`
- `cmd/browser-agent/tools_schema_parity_test.go`
- `cmd/browser-agent/tools_interact_navigate_document_test.go`
- `cmd/browser-agent/tools_contract_enforcement_test.go`

## Test Paths

- `scripts/docs/check-feature-bundles.test.mjs`
- `cmd/browser-agent/tools_schema_parity_test.go`
- `cmd/browser-agent/tools_interact_navigate_document_test.go`
- `cmd/browser-agent/tools_contract_enforcement_test.go`

## Edit Guardrails

1. Keep `verify-llm` focused; avoid adding long-running soak/fuzz/e2e checks.
2. When adding new tool/schema surfaces, extend path-scope detection and focused test regex in lockstep.
3. Document any runtime increase above 120 seconds and justify added checks.
