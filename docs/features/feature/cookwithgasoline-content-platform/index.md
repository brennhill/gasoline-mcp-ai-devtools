---
doc_type: feature_index
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
feature_type: feature
owners:
  - Brenn
last_reviewed: 2026-03-05
code_paths:
  - package.json
  - .vale.ini
  - .vale/styles/Gasoline/*.yml
  - cookwithgasoline.com/astro.config.mjs
  - cookwithgasoline.com/src/content.config.ts
  - cookwithgasoline.com/public/images/integrations/*.svg
  - cookwithgasoline.com/public/images/landing/*.svg
  - cookwithgasoline.com/public/images/solutions-seo-signal.svg
  - cookwithgasoline.com/src/content/docs/articles/*.md
  - cookwithgasoline.com/src/content/docs/blog/*.md
  - cookwithgasoline.com/src/content/docs/downloads.md
  - cookwithgasoline.com/src/content/docs/guides/start-here-by-role.md
  - cookwithgasoline.com/src/content/docs/guides/tracks/*.md
  - cookwithgasoline.com/src/content/docs/guides/visual-evidence-standards.md
  - cookwithgasoline.com/src/content/docs/guides/seo-analysis.md
  - cookwithgasoline.com/src/content/docs/guides/annotation-skill-terminal-workflow.md
  - cookwithgasoline.com/src/content/docs/index.mdx
  - cookwithgasoline.com/src/content/docs/reference/index.md
  - cookwithgasoline.com/src/content/docs/reference/examples/*.md
  - cookwithgasoline.com/src/content/docs/reference/observe.md
  - cookwithgasoline.com/src/content/docs/reference/analyze.md
  - cookwithgasoline.com/src/content/docs/reference/interact.md
  - cookwithgasoline.com/src/content/docs/reference/configure.md
  - cookwithgasoline.com/src/content/docs/reference/generate.md
  - cookwithgasoline.com/src/components/Footer.astro
  - cookwithgasoline.com/src/components/Head.astro
  - cookwithgasoline.com/src/components/Landing.astro
  - cookwithgasoline.com/src/components/WorkflowLibrary.astro
  - cookwithgasoline.com/src/components/ArticlesLibrary.astro
  - cookwithgasoline.com/src/data/relatedGuides.ts
  - cookwithgasoline.com/src/data/searchSynonyms.ts
  - cookwithgasoline.com/src/data/workflows.ts
  - cookwithgasoline.com/src/pages/[...slug].md.ts
  - cookwithgasoline.com/src/pages/index.md.ts
  - cookwithgasoline.com/src/pages/llms.txt.ts
  - cookwithgasoline.com/src/pages/llms-full.txt.ts
  - cookwithgasoline.com/src/pages/markdown/[...slug].md.ts
  - cookwithgasoline.com/src/pages/search-synonyms.json.ts
  - cookwithgasoline.com/src/styles/custom.css
  - cookwithgasoline.com/src/utils/markdownPaths.ts
  - cookwithgasoline.com/src/utils/siteVersion.ts
  - scripts/docs/check-docs-quality-gates.mjs
  - scripts/docs/check-site-content-ids.mjs
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
  - scripts/docs/check-content-style-contract.mjs
  - scripts/docs/check-downloads-page-contract.mjs
  - scripts/docs/check-landing-layout-contract.mjs
  - scripts/docs/generate-reference-executable-examples.mjs
  - scripts/docs/normalize-site-tags.mjs
  - scripts/docs/sync-verification-metadata.mjs
  - scripts/docs/run-vale-on-changed.mjs
  - scripts/docs/check-reference-schema-sync.mjs
  - scripts/docs/check-feature-bundles.js
  - .github/workflows/ci.yml
test_paths:
  - scripts/docs/check-docs-quality-gates.mjs
  - scripts/docs/check-site-content-ids.mjs
  - scripts/docs/check-feature-bundles.js
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
  - scripts/docs/check-content-style-contract.mjs
  - scripts/docs/check-downloads-page-contract.mjs
  - scripts/docs/check-landing-layout-contract.mjs
  - scripts/docs/generate-reference-executable-examples.mjs
  - scripts/docs/normalize-site-tags.mjs
  - scripts/docs/sync-verification-metadata.mjs
  - scripts/docs/run-vale-on-changed.mjs
  - scripts/docs/check-reference-schema-sync.mjs
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Cookwithgasoline Content Platform

## TL;DR

- Status: in_progress
- Scope: full homepage theme reset to an updated modern layout, plus hero flame-only favicon-style flicker, light-theme header title color tuning, reference-page readability fixes (heading dedupe + contrast), schema-synced reference mode/action coverage, scroll-rhythm spacing/overflow hardening, workflow discovery, split `blog` (date-driven) vs `articles` (topic-driven), automated markdown mirrors for docs/blog/articles pages, downloads-page clarity updates (native binary runtime guidance + expressive-code terminal frame visual cleanup), and large-screen staggered left/right solution panel offsets for better scan rhythm. Home solutions now use five split panels with large left-side visual mocks, Gasoline-themed annotation callouts, and right-side CTA/copy blocks. The integrations band now uses a branded card with real agent logos and a hover-widget concept preview, and CTA/footer links were updated based on annotation feedback.
- Current IA policy: `/blog/*` is release-note history, `/articles/*` is evergreen topic content.
- Versioning policy: single live channel (`latest`) with docs version sourced from repo `VERSION` and rendered globally.
- Primary guardrail: CI docs contracts (`content-contract` + `style-contract` + `Vale` + `downloads-contract` + `landing-layout-contract` + reference schema sync + feature bundle strict check)

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Code and Tests

- Content and layout code paths are listed in frontmatter.
- CI gate is enforced through `docs:ci` in `.github/workflows/ci.yml`.
