---
doc_type: flow_map
flow_id: feature-doc-frontmatter-freshness-gates
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
entrypoints:
  - scripts/docs/check-feature-bundles.js:runCheckFeatureBundlesCLI
  - .github/workflows/ci.yml:javascript
code_paths:
  - scripts/docs/check-feature-bundles.js
  - .github/workflows/ci.yml
  - package.json
  - scripts/lint-documentation.py
test_paths:
  - scripts/docs/check-feature-bundles.test.mjs
---

# Feature Doc Frontmatter and Freshness Gates

## Scope

Covers CI enforcement for `docs/features/**` metadata quality: required frontmatter keys and non-stale `last_reviewed` values.

## Entrypoints

1. `npm run docs:check:strict` executes `check-feature-bundles.js`.
2. CI JavaScript job runs the strict feature-doc gate as a blocking step.

## Primary Flow

1. Discover feature directories under `docs/features` using predicate rules (`feature/`, `bug/`, and legacy allowed paths).
2. Validate required bundle files: `index.md`, `product-spec.md`, `tech-spec.md`, `qa-plan.md`.
3. In strict mode, enforce frontmatter keys (`doc_type`, `feature_id`, `last_reviewed`) on each required file.
4. Enforce freshness window for `last_reviewed` (default 30 days) under feature docs.
5. Emit actionable issue list and fail process on any violations.
6. CI blocks merge when strict gate fails.

## Error and Recovery Paths

1. Missing required files/frontmatter keys cause immediate hard failures.
2. Invalid `last_reviewed` format (`YYYY-MM-DD`) fails validation with explicit format guidance.
3. Stale review dates fail with age threshold details.
4. Freshness enforcement can be temporarily toggled via `DOCS_STRICT_FEATURE_FRESHNESS=0` for controlled migrations.

## State and Contracts

1. `DOCS_STRICT_FRONTMATTER=1` enables full file-level frontmatter enforcement.
2. `DOCS_FEATURE_FRESHNESS_DAYS` defines freshness threshold (default 30).
3. `docs:check` remains developer-friendly; CI uses strict mode for merge blocking.

## Migration Plan (Non-Feature Docs)

1. Phase 1 (current): strict enforcement only for `docs/features/**`; non-feature docs continue warning-only via `lint-documentation.py`.
2. Phase 2: add dedicated strict subsets for `docs/architecture/**` and `docs/standards/**` with per-domain freshness thresholds.
3. Phase 3: converge onto repo-wide strict frontmatter/freshness gates once warning volume is near zero.

## Code Paths

- `scripts/docs/check-feature-bundles.js`
- `.github/workflows/ci.yml`
- `package.json`
- `scripts/lint-documentation.py`

## Test Paths

- `scripts/docs/check-feature-bundles.test.mjs`

## Edit Guardrails

1. Keep feature-dir discovery predicates backward-compatible to avoid silently dropping legacy feature folders.
2. Any frontmatter key changes must update script validation, docs conventions, and test fixtures together.
3. Do not weaken CI gate severity for `docs/features/**` without updating migration plan and explicit issue tracking.
