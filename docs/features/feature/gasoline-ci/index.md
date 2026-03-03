---
doc_type: feature_index
feature_id: feature-gasoline-ci
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - Makefile
  - .github/workflows/ci.yml
  - scripts/generate-wire-types.js
  - scripts/docs/check-feature-bundles.js
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
  - scripts/docs/check-reference-schema-sync.mjs
  - scripts/lint-documentation.py
  - package.json
test_paths:
  - scripts/docs/check-feature-bundles.test.mjs
  - cmd/dev-console/tools_schema_parity_test.go
  - cmd/dev-console/tools_interact_navigate_document_test.go
  - cmd/dev-console/tools_contract_enforcement_test.go
---

# Gasoline Ci

## TL;DR

- Status: proposed
- Tool: observe, generate
- Mode/Action: observe(errors, logs, network_waterfall, network_bodies, websocket_events, performance, timeline), generate(har, sarif)
- Location: `docs/features/feature/gasoline-ci`
- Fast Gate: `make verify-llm` (typical warm-cache runtime ~60-120s)

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_GASOLINE_CI_001
- FEATURE_GASOLINE_CI_002
- FEATURE_GASOLINE_CI_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
