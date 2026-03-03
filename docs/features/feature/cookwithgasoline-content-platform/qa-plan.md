---
doc_type: qa_plan
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
last_reviewed: 2026-03-03
owners:
  - Brenn
---

# QA Plan

## Automated Gates

1. `npm run docs:check:strict`
2. `npm run docs:lint:content-contract`
3. `npm run docs:lint:reference-schema-sync`
4. `npm run docs:ci`
5. `(cd cookwithgasoline.com && npm run build)`

## Manual Checks

1. Verify homepage hero spacing/centering at desktop and mobile breakpoints.
2. Verify light/dark theme readability for nav, sidebar, TOC, and body text.
3. Verify `/reference/*` pages render all documented modes/actions.
4. Verify `.md` mirrors resolve for docs, blog, and articles routes.

## Regression Focus

- Reference docs drift when schema enums change.
- Visual regressions in top hero layout and section spacing.
- Missing metadata/frontmatter on changed docs/blog/articles files.

## Linked Architecture

- Canonical flow map: [flow-map.md](./flow-map.md)
