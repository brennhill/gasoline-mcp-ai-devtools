---
doc_type: architecture_flow_map
feature_id: feature-framework-selector-resilience
status: shipped
owners: []
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Framework Selector Resilience Smoke Flow

## Scope

Build and run real-framework smoke fixtures that validate hard browser automation cases across React, Vue, Svelte, and Next.js.

## Entrypoints

1. `scripts/smoke-test.sh --only 29`
2. `scripts/smoke-tests/29-framework-selector-resilience.sh`
3. `npm run smoke:build-framework-fixtures`
4. `npm run smoke:framework-parity`

## Feature Docs

1. `docs/features/feature/framework-selector-resilience/index.md`
2. `docs/features/feature/framework-selector-resilience/flow-map.md`

## Primary Flow

1. Module `29-framework-selector-resilience.sh` calls `build-framework-fixtures.mjs` once.
2. Builder compiles React/Vue/Svelte fixture bundles and exports a static Next.js fixture app.
3. Builder writes generated assets into `cmd/dev-console/testpages/frameworks/`.
4. Smoke harness serves generated pages via local harness (`http://127.0.0.1:<port>/frameworks/...`).
5. For each framework page, smoke test verifies:
   - `analyze({what:"page_structure"})` detects expected framework.
   - Hydration gate is completed (`#hydrated-ready`).
   - Overlay interception is handled (`Accept Cookies`).
   - Async delayed UI (`Async Save`) is discoverable and actionable.
   - Lazy/virtualized target (`Framework Target 80`) appears after scroll.
   - Route remount churn keeps actions stable via stale `element_id` recovery.
   - Selector churn resilience remains stable across configurable refresh cycles.
6. Repeat controls:
   - `FRAMEWORK_RESILIENCE_FULL_REPEATS` repeats the full scenario per framework.
   - `FRAMEWORK_SELECTOR_REFRESH_CYCLES` controls per-run refresh loops.

## Error and Recovery Paths

1. Fixture build failure: module fails fast with diagnostics path.
2. Framework detection mismatch: module reports exact framework expected vs returned payload.
3. Hydration/overlay race failures: `wait_for` and explicit overlay dismissal gates prevent false positives.
4. Stale handle mismatch after remount: failure includes stale `element_id` and command payload.
5. Virtualized/async target timeout: failure includes command payload and timeout context.

## State and Contracts

1. Generated fixture output contract:
   - `cmd/dev-console/testpages/frameworks/react.html + react.bundle.js`
   - `cmd/dev-console/testpages/frameworks/vue.html + vue.bundle.js`
   - `cmd/dev-console/testpages/frameworks/svelte.html + svelte.bundle.js`
   - `cmd/dev-console/testpages/frameworks/next/` static export
2. Required fixture selectors and semantics:
   - `#hydrated-ready`, `#selector-token`, `#mount-token`, `#async-result`, `#deep-result`
   - `text=Accept Cookies`, `text=Profile Tab`, `text=Settings Tab`
   - `placeholder=Enter name`, `text=Submit Profile`, `text=Async Save`, `text=Framework Target 80`
3. Framework detection contract:
   - React → `"React"`
   - Vue → `"Vue"`
   - Svelte → `"Svelte"`
   - Next fixture → `"Next.js"`
4. Repeat contract:
   - Defaults: `FRAMEWORK_RESILIENCE_FULL_REPEATS=1`, `FRAMEWORK_SELECTOR_REFRESH_CYCLES=3`
   - Parity gate: `FRAMEWORK_RESILIENCE_FULL_REPEATS=3`, `FRAMEWORK_SELECTOR_REFRESH_CYCLES=3`

## Code Paths

1. `scripts/smoke-tests/29-framework-selector-resilience.sh`
2. `scripts/smoke-tests/build-framework-fixtures.mjs`
3. `scripts/smoke-tests/framework-fixtures/react-entry.jsx`
4. `scripts/smoke-tests/framework-fixtures/vue-entry.js`
5. `scripts/smoke-tests/framework-fixtures/SmokeSvelteApp.svelte`
6. `scripts/smoke-tests/framework-fixtures/next-app/pages/index.jsx`
7. `scripts/smoke-tests/framework-fixtures/next-app/next.config.mjs`
8. `scripts/smoke-test.sh`
9. `package.json` (`smoke:framework-parity`)
10. `scripts/smoke-tests/framework-fixtures/README.md`

## Test Paths

1. `scripts/smoke-test.sh --only 29`
2. `scripts/smoke-tests/29-framework-selector-resilience.sh`
3. `npm run smoke:framework-parity`

## Edit Guardrails

1. Keep fixture semantics stable (`placeholder=Enter name`, `Submit Profile`, tab labels) so smoke selectors stay deterministic.
2. Keep hard-case markers (`#hydrated-ready`, `#async-result`, `#deep-result`) backward-compatible.
3. When adding a framework fixture, update:
   - builder output
   - module `29` framework matrix
   - feature docs + this flow map
4. Generated bundle artifacts are build outputs; do not hand-edit.
