---
doc_type: feature_index
feature_id: feature-gasoline-ci
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - Makefile
  - .github/workflows/ci.yml
  - .golangci.yml
  - scripts/generate-wire-types.js
  - scripts/docs/check-feature-bundles.js
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
  - scripts/docs/check-reference-schema-sync.mjs
  - scripts/lint-documentation.py
  - package.json
test_paths:
  - scripts/docs/check-feature-bundles.test.mjs
  - cmd/browser-agent/tools_schema_parity_test.go
  - cmd/browser-agent/tools_interact_navigate_document_test.go
  - cmd/browser-agent/tools_contract_enforcement_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Gasoline Ci

## TL;DR

- Status: proposed
- Tool: observe, generate
- Mode/Action: observe(errors, logs, network_waterfall, network_bodies, websocket_events, performance, timeline), generate(har, sarif)
- Location: `docs/features/feature/gasoline-ci`
- Fast Gate: `make verify-llm` (typical warm-cache runtime ~60-120s)
- Added Gates: docs integrity (`docs:lint:integrity`) and Go import boundaries (`depguard`)

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
