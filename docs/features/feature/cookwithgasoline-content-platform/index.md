---
doc_type: feature_index
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
feature_type: feature
owners:
  - Brenn
last_reviewed: 2026-03-03
code_paths:
  - cookwithgasoline.com/astro.config.mjs
  - cookwithgasoline.com/src/components/Head.astro
  - cookwithgasoline.com/src/components/Landing.astro
  - cookwithgasoline.com/src/components/WorkflowLibrary.astro
  - cookwithgasoline.com/src/components/ArticlesLibrary.astro
  - cookwithgasoline.com/src/data/workflows.ts
  - cookwithgasoline.com/src/pages/[...slug].md.ts
  - cookwithgasoline.com/src/pages/index.md.ts
  - cookwithgasoline.com/src/pages/llms.txt.ts
  - cookwithgasoline.com/src/pages/llms-full.txt.ts
  - cookwithgasoline.com/src/pages/markdown/[...slug].md.ts
  - cookwithgasoline.com/src/styles/custom.css
  - cookwithgasoline.com/src/utils/markdownPaths.ts
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
  - .github/workflows/ci.yml
test_paths:
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
---

# Cookwithgasoline Content Platform

## TL;DR

- Status: in_progress
- Scope: homepage redesign, workflow discovery page, articles index, and automated markdown mirrors for all docs/blog pages
- Primary guardrail: CI content-contract linter for changed docs/blog files

## Specs

- Flow Map: [flow-map.md](./flow-map.md)

## Code and Tests

- Content and layout code paths are listed in frontmatter.
- CI gate is enforced through `docs:lint:content-contract` in `.github/workflows/ci.yml`.
