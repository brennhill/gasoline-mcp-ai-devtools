---
doc_type: feature_index
feature_id: feature-framework-selector-resilience
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - scripts/smoke-tests/29-framework-selector-resilience.sh
  - scripts/smoke-tests/build-framework-fixtures.mjs
  - scripts/smoke-tests/framework-fixtures/react-entry.jsx
  - scripts/smoke-tests/framework-fixtures/vue-entry.js
  - scripts/smoke-tests/framework-fixtures/SmokeSvelteApp.svelte
  - scripts/smoke-tests/framework-fixtures/next-app/pages/index.jsx
  - scripts/smoke-test.sh
test_paths:
  - scripts/smoke-tests/29-framework-selector-resilience.sh
---

# Framework Selector Resilience

## TL;DR
- Status: shipped
- Surface: smoke module `29-framework-selector-resilience.sh`
- Scope: real-framework fixture coverage for hard automation failure modes

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map: `flow-map.md`

## Canonical Note
This feature makes the fixture coverage itself a first-class capability: Gasoline now validates automation resilience against real React, Vue, Svelte, and Next.js pages that model hydration races, overlays, remount churn, async delayed UI, and virtualized content.
