---
doc_type: flow_map
flow_id: interact-action-surface-registry
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - internal/schema/interact_actions.go
  - internal/tools/configure/mode_specs_interact.go
code_paths:
  - internal/schema/interact_actions.go
  - internal/schema/interact_properties_dispatch.go
  - internal/tools/configure/mode_specs_interact.go
  - cmd/browser-agent/tools_schema_parity_test.go
  - scripts/docs/check-reference-schema-sync.mjs
test_paths:
  - internal/schema/interact_test.go
  - internal/tools/configure/mode_specs_test.go
  - cmd/browser-agent/tools_schema_parity_test.go
  - cmd/browser-agent/tools_interact_navigate_document_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Action Surface Registry

## Scope

Defines the canonical interact action surface once, then derives schema enums and `describe_capabilities` mode specs from that same registry.

## Entrypoints

- `internal/schema/interact_actions.go` owns `interactActionSpecs` and derives `interactActions`.
- `internal/tools/configure/mode_specs_interact.go` builds interact mode specs directly from `schema.InteractActionSpecs()`.

## Primary Flow

1. Add or edit an action in `interactActionSpecs`.
2. `interactActions` is derived from the registry for schema `what/action` enums.
3. `describe_capabilities` interact mode specs are generated from the same registry.
4. Runtime dispatch parity tests enforce schema enum vs handler surface consistency.
5. Reference docs sync checks validate action docs coverage against the canonical schema source.

## Error and Recovery Paths

- Missing registry hint/duplicate name is caught by `internal/schema/interact_test.go`.
- Schema enum vs runtime dispatch drift is caught by `cmd/browser-agent/tools_schema_parity_test.go`.
- Docs reference drift is caught by `scripts/docs/check-reference-schema-sync.mjs`.

## State and Contracts

- Canonical source: `internal/schema/interact_actions.go` (`interactActionSpecs`).
- Derived contracts:
  - `interact.what/action` enum in schema.
  - `configure(what="describe_capabilities", tool="interact")` mode hints/params.
- Backward-compatible alias actions (`state_*`) stay in the same registry and remain documented.

## Code Paths

- `internal/schema/interact_actions.go`
- `internal/schema/interact_properties_dispatch.go`
- `internal/tools/configure/mode_specs_interact.go`
- `cmd/browser-agent/tools_schema_parity_test.go`
- `scripts/docs/check-reference-schema-sync.mjs`

## Test Paths

- `internal/schema/interact_test.go`
- `internal/tools/configure/mode_specs_test.go`
- `cmd/browser-agent/tools_schema_parity_test.go`
- `cmd/browser-agent/tools_interact_navigate_document_test.go`

## Edit Guardrails

- Add a new interact action by editing `interactActionSpecs` first.
- Implement runtime behavior in `cmd/browser-agent` after schema/metadata registration.
- Keep parity tests green before merging.
- Update feature docs pointer/index and this flow map whenever action surface changes.
- Cross-reference: `docs/features/feature/interact-explore/index.md` and `docs/features/feature/interact-explore/flow-map.md`.
