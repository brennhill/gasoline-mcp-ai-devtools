---
doc_type: feature_index
feature_id: feature-browser-extension-enhancement
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - src/options.ts
  - src/lib/daemon-http.ts
  - extension/options.html
test_paths:
  - tests/extension/sync-client.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Browser Extension Enhancement

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/browser-extension-enhancement`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_BROWSER_EXTENSION_ENHANCEMENT_001
- FEATURE_BROWSER_EXTENSION_ENHANCEMENT_002
- FEATURE_BROWSER_EXTENSION_ENHANCEMENT_003

## Code and Tests

- `src/options.ts` uses shared daemon request/header helpers for health checks and active-codebase config sync.
- `src/lib/daemon-http.ts` defines the canonical extension-client header and JSON request init contract.
