---
doc_type: feature_index
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
feature_type: feature
owners:
  - Brenn
last_reviewed: 2026-03-04
code_paths:
  - cookwithgasoline.com/astro.config.mjs
  - cookwithgasoline.com/public/images/solutions-seo-signal.svg
  - cookwithgasoline.com/src/content/docs/articles/*.md
  - cookwithgasoline.com/src/content/docs/reference/index.md
  - cookwithgasoline.com/src/content/docs/reference/observe.md
  - cookwithgasoline.com/src/content/docs/reference/analyze.md
  - cookwithgasoline.com/src/content/docs/reference/interact.md
  - cookwithgasoline.com/src/content/docs/reference/configure.md
  - cookwithgasoline.com/src/content/docs/reference/generate.md
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
  - scripts/docs/check-reference-schema-sync.mjs
  - scripts/docs/check-feature-bundles.js
  - .github/workflows/ci.yml
test_paths:
  - scripts/docs/check-feature-bundles.js
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
  - scripts/docs/check-reference-schema-sync.mjs
---

# Cookwithgasoline Content Platform

## TL;DR

- Status: in_progress
- Scope: full homepage theme reset to a update theme to more modern layout, plus hero flame-only favicon-style flicker, reference-page readability fixes (heading dedupe + contrast), schema-synced reference mode/action coverage, scroll-rhythm spacing/overflow hardening, workflow discovery, split `blog` (date-driven) vs `articles` (topic-driven), and automated markdown mirrors for docs/blog/articles pages. Home solutions now use five 100x-style split panels with large left-side visual mocks, Gasoline-themed annotation callouts, and right-side CTA/copy blocks.
- Current IA policy: `/blog/*` is release-note history, `/articles/*` is evergreen topic content.
- Primary guardrail: CI docs contracts (`content-contract` + reference schema sync + feature bundle strict check)

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Code and Tests

- Content and layout code paths are listed in frontmatter.
- CI gate is enforced through `docs:ci` in `.github/workflows/ci.yml`.
