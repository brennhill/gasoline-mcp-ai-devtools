---
doc_type: feature_index
feature_id: feature-config-profiles
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - cmd/dev-console/tools_configure.go
  - cmd/dev-console/tools_configure_registry.go
  - cmd/dev-console/tools_configure_session_handler.go
  - cmd/dev-console/tools_configure_state_impl.go
  - cmd/dev-console/tools_configure_sessions.go
test_paths:
  - cmd/dev-console/tools_configure_handler_test.go
  - cmd/dev-console/tools_configure_session_test.go
---

# Config Profiles

## TL;DR

- Status: proposed
- Tool: configure
- Mode/Action: profiles
- Location: `docs/features/feature/config-profiles`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_CONFIG_PROFILES_001
- FEATURE_CONFIG_PROFILES_002
- FEATURE_CONFIG_PROFILES_003

## Code and Tests

- Configure dispatch and action registry:
  - `cmd/dev-console/tools_configure.go`
  - `cmd/dev-console/tools_configure_registry.go`
- Session/store sub-handler and implementations:
  - `cmd/dev-console/tools_configure_session_handler.go`
  - `cmd/dev-console/tools_configure_state_impl.go`
  - `cmd/dev-console/tools_configure_sessions.go`
- Tests:
  - `cmd/dev-console/tools_configure_handler_test.go`
  - `cmd/dev-console/tools_configure_session_test.go`
