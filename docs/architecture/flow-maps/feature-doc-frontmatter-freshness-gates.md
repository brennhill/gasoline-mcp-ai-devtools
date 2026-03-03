---
doc_type: flow_map
flow_id: feature-doc-frontmatter-freshness-gates
status: active
last_reviewed: 2026-03-04
owners:
  - Brenn
entrypoints:
  - scripts/docs/check-feature-bundles.js:runCheckFeatureBundlesCLI
  - scripts/lint-documentation.py:DocumentLinter.lint_all
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

Covers CI enforcement for docs integrity and `docs/features/**` metadata quality:
1. broken-link/code-reference checks across docs
2. required frontmatter keys and non-stale `last_reviewed` values for feature docs

## Entrypoints

1. `npm run docs:lint:integrity` executes `lint-documentation.py` scoped to `docs/features` + `docs/architecture` (broken links + code refs).
2. `npm run docs:check:strict` executes `check-feature-bundles.js`.
3. CI JavaScript job runs both steps as blocking gates.

## Primary Flow

1. Run scoped docs integrity lint to catch broken links and stale code references in `docs/features` + `docs/architecture`.
2. Discover feature directories under `docs/features` using predicate rules (`feature/`, `bug/`, and legacy allowed paths).
3. Validate required bundle files: `index.md`, `product-spec.md`, `tech-spec.md`, `qa-plan.md`.
4. In strict mode, enforce frontmatter keys (`doc_type`, `feature_id`, `last_reviewed`) on each required file.
5. Enforce freshness window for `last_reviewed` (default 30 days) under feature docs.
6. Emit actionable issue list and fail process on any violations.
7. CI blocks merge when either docs-integrity or strict feature-doc gates fail.

## Error and Recovery Paths

1. Broken links and unresolved code references fail docs-integrity gate.
2. Missing required files/frontmatter keys cause immediate hard failures.
3. Invalid `last_reviewed` format (`YYYY-MM-DD`) fails validation with explicit format guidance.
4. Stale review dates fail with age threshold details.
5. Freshness enforcement can be temporarily toggled via `DOCS_STRICT_FEATURE_FRESHNESS=0` for controlled migrations.

## State and Contracts

1. `DOCS_STRICT_FRONTMATTER=1` enables full file-level frontmatter enforcement.
2. `DOCS_FEATURE_FRESHNESS_DAYS` defines freshness threshold (default 30).
3. `docs:check` remains developer-friendly; CI uses strict mode + docs-integrity lint for merge blocking.

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
