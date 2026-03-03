---
doc_type: feature_index
feature_id: feature-security-hardening
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - internal/security/security_config_policy.go
  - internal/security/security_config_mode.go
  - internal/security/security_config_audit.go
test_paths:
  - internal/security/security_config_unit_test.go
  - internal/security/security_boundary_test.go
  - internal/security/security_config_path_test.go
---

# Security Hardening

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: security config
- Location: `docs/features/feature/security-hardening`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_SECURITY_HARDENING_001
- FEATURE_SECURITY_HARDENING_002
- FEATURE_SECURITY_HARDENING_003

## Code and Tests

- `internal/security/security_config_policy.go` — manual-only security config mutation guards (`AddToWhitelist`, `SetMinSeverity`, `ClearWhitelist`) with explicit human-review guidance and in-memory audit events.
- `internal/security/security_config_mode.go` — MCP-mode and interactive-terminal gating flags.
- `internal/security/security_config_audit.go` — session-scoped in-memory audit trail for security config actions/attempts.
- `internal/security/security_config_unit_test.go` — manual-only policy and audit-event behavior.
