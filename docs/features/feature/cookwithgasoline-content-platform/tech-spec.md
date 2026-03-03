---
doc_type: tech_spec
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
last_reviewed: 2026-03-03
owners:
  - Brenn
---

# Tech Spec

## Primary Components

- `cookwithgasoline.com/src/components/Landing.astro`
- `cookwithgasoline.com/src/components/WorkflowLibrary.astro`
- `cookwithgasoline.com/src/components/ArticlesLibrary.astro`
- `cookwithgasoline.com/src/styles/custom.css`
- `cookwithgasoline.com/src/content/docs/articles/*.md`
- `cookwithgasoline.com/src/content/docs/reference/*.md`
- `cookwithgasoline.com/src/pages/[...slug].md.ts`
- `cookwithgasoline.com/src/pages/markdown/[...slug].md.ts`

## Contracts and Validation

- Content contract: `scripts/docs/check-cookwithgasoline-content-contract.mjs`
- Reference schema sync contract: `scripts/docs/check-reference-schema-sync.mjs`
- Feature bundle contract: `scripts/docs/check-feature-bundles.js`

## Data/Content Sources

- Tool mode/action enums are sourced from:
  - `internal/schema/observe.go`
  - `internal/schema/analyze.go`
  - `internal/schema/configure_properties_core.go`
  - `internal/schema/generate.go`
  - `internal/schema/interact_actions.go`

## Failure Modes

- Missing required headings (`Quick Reference`, `Common Parameters`) in reference docs.
- Missing mode/action sections after schema changes.
- Missing required feature bundle docs for this feature directory.

## Linked Architecture

- Canonical flow map: [flow-map.md](./flow-map.md)
