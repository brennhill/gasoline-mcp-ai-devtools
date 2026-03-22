---
doc_type: feature_index
feature_id: feature-browser-extension-enhancement
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-21
code_paths:
  - src/popup.ts
  - src/popup/logo-motion.ts
  - src/options.ts
  - src/lib/daemon-http.ts
  - extension/popup.html
  - extension/popup.css
  - extension/options.html
test_paths:
  - tests/extension/logo-motion.test.js
  - tests/extension/sync-client.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-03-21
---

# Browser Extension Enhancement

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/browser-extension-enhancement`
- The popup header now uses the shared idle-motion STRUM icon and escalates to the stronger strum asset only on hover.

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

- `src/popup.ts` initializes popup-side UI wiring, including the shared STRUM logo hover behavior.
- `src/popup/logo-motion.ts` keeps popup logo motion consistent with the hover island asset split.
- `src/options.ts` uses shared daemon request/header helpers for health checks and active-codebase config sync.
- `src/lib/daemon-http.ts` defines the canonical extension-client header and JSON request init contract.
