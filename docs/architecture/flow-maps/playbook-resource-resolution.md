---
doc_type: flow_map
flow_id: playbook-resource-resolution
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/handler_dispatch.go:handleResourcesRead
  - cmd/dev-console/bridge_fastpath.go:handleFastPath
code_paths:
  - cmd/dev-console/mcp_resources.go
  - cmd/dev-console/playbooks.go
  - cmd/dev-console/playbooks_performance.go
  - cmd/dev-console/playbooks_accessibility.go
  - cmd/dev-console/playbooks_security.go
  - cmd/dev-console/playbooks_automation.go
  - cmd/dev-console/playbooks_resolver.go
test_paths:
  - cmd/dev-console/handler_unit_test.go
  - cmd/dev-console/bridge_fastpath_unit_test.go
  - cmd/dev-console/bridge_faststart_extended_test.go
  - cmd/dev-console/playbooks_content_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Playbook Resource Resolution

## Scope

Covers MCP `resources/list`, `resources/templates/list`, and `resources/read` resolution for capability index, guide, quickstart, and playbooks.

## Entrypoints

- `handleResourcesRead` serves canonical resource content in normal handler path.
- `handleFastPath` serves `resources/read` immediately in bridge fast-path mode.

## Primary Flow

1. Client requests resource templates and discovers `gasoline://playbook/{capability}/{level}`.
2. Client calls `resources/read` with a URI.
3. `resolveResourceContent` routes URI family (`capabilities`, `guide`, `quickstart`, `playbook`, `demo`).
4. Playbook URIs are normalized via `resolvePlaybookKey` and capability aliases.
5. Canonical URI and markdown content are returned in MCP resource result.
6. Fast-path requests record telemetry success/failure counters in bridge metrics.

## Error and Recovery Paths

- Unknown resource URI returns MCP not-found error (`-32002`).
- Invalid playbook capability/shape resolves to not-found instead of ambiguous fallback.
- Fast-path path emits telemetry for both success and failure to support diagnostics.

## State and Contracts

- `playbooks` map is built from capability-specific sets with duplicate-key panic guard.
- Canonical capability aliases must remain stable for compatibility (`security_audit` -> `security`).
- Resource payloads are text/markdown and intended for token-efficient LLM retrieval.

## Code Paths

- `cmd/dev-console/mcp_resources.go`
- `cmd/dev-console/playbooks.go`
- `cmd/dev-console/playbooks_performance.go`
- `cmd/dev-console/playbooks_accessibility.go`
- `cmd/dev-console/playbooks_security.go`
- `cmd/dev-console/playbooks_automation.go`
- `cmd/dev-console/playbooks_resolver.go`

## Test Paths

- `cmd/dev-console/handler_unit_test.go`
- `cmd/dev-console/bridge_fastpath_unit_test.go`
- `cmd/dev-console/bridge_faststart_extended_test.go`
- `cmd/dev-console/playbooks_content_test.go`

## Edit Guardrails

- Keep URI normalization in `playbooks_resolver.go`; avoid scattered alias logic.
- Keep playbook content modular by capability file; do not reintroduce monolithic map blobs.
- Preserve canonical URI behavior and test coverage before adding new capabilities.
