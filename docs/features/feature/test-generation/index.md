---
doc_type: feature_index
feature_id: feature-test-generation
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - cmd/dev-console/tools_generate_testgen_handler.go
  - cmd/dev-console/testgen_aliases.go
  - cmd/dev-console/testgen_provider_adapter.go
  - cmd/dev-console/testgen_classify.go
  - cmd/dev-console/testgen_heal.go
  - cmd/dev-console/testgen.go
  - cmd/dev-console/tools_generate.go
  - internal/schema/generate.go
test_paths:
  - cmd/dev-console/testgen_context_test.go
  - cmd/dev-console/testgen_generate_test.go
  - cmd/dev-console/testgen_heal_test.go
  - cmd/dev-console/testgen_classify_dispatch_test.go
  - internal/testgen/generate_test.go
  - internal/testgen/helpers_test.go
  - internal/schema/invariants_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation

## TL;DR

- Status: proposed
- Tool: generate
- Mode/Action: [test_from_context, test_heal, test_classify]
- Location: `docs/features/feature/test-generation`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_TEST_GENERATION_001
- FEATURE_TEST_GENERATION_002
- FEATURE_TEST_GENERATION_003

## Code and Tests

- Sub-handler wiring: `cmd/dev-console/tools_generate_testgen_handler.go`
- Context dispatch: `cmd/dev-console/testgen.go`
- Alias/contracts: `cmd/dev-console/testgen_aliases.go`
- Provider delegation: `cmd/dev-console/testgen_provider_adapter.go`
- Heal and classify handlers: `cmd/dev-console/testgen_heal.go`, `cmd/dev-console/testgen_classify.go`
- Generate tool schema contract: `internal/schema/generate.go`
- Core behavior tests: `cmd/dev-console/testgen_context_test.go`, `cmd/dev-console/testgen_generate_test.go`, `cmd/dev-console/testgen_heal_test.go`, `cmd/dev-console/testgen_classify_dispatch_test.go`
- Schema invariants: `internal/schema/invariants_test.go`
