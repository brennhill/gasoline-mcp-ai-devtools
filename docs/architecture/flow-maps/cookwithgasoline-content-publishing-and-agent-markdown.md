---
doc_type: flow_map
flow_id: cookwithgasoline-content-publishing-and-agent-markdown
status: active
last_reviewed: 2026-03-04
owners:
  - Brenn
feature_ids:
  - feature-cookwithgasoline-content-platform
entrypoints:
  - cookwithgasoline.com/src/content/docs/index.mdx
  - cookwithgasoline.com/src/pages/[...slug].md.ts
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
  - scripts/docs/check-reference-schema-sync.mjs
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

# Cookwithgasoline Content Publishing and Agent Markdown Flow

## Scope

Covers complete homepage theme/layout replacement and messaging updates (including centered hero flame-only favicon-style flicker, reference-page readability fixes, schema-synced tool-mode coverage, section spacing/overflow hardening for full-page scroll rhythm, and 100x-style split solutions panels with in-panel synthetic visuals + Gasoline-themed callout overlays), workflow discovery, split discovery surfaces for date-driven release-note `blog` and topic-driven `articles`, tool-reference navigation, and automatic per-route markdown mirrors for agent consumption.

## Entrypoints

- Splash homepage in `cookwithgasoline.com/src/content/docs/index.mdx`
- Agent markdown mirror route in `cookwithgasoline.com/src/pages/[...slug].md.ts`
- Content contract gate in `scripts/docs/check-cookwithgasoline-content-contract.mjs`
- Reference/schema sync gate in `scripts/docs/check-reference-schema-sync.mjs`

## Primary Flow

1. Starlight loads docs/blog/articles entries from `cookwithgasoline.com/src/content/docs/*` through `docsLoader()`.
2. Site navigation and information architecture are defined in `cookwithgasoline.com/astro.config.mjs`.
3. Splash pages render reusable components for marketing and discovery (`Landing.astro`, `WorkflowLibrary.astro`, `ArticlesLibrary.astro`).
4. Every docs/blog/articles slug is mirrored as `/<slug>.md` via `src/pages/[...slug].md.ts`.
5. `<link rel="alternate" type="text/markdown">` in `Head.astro` points each HTML route to its markdown mirror.
6. `llms.txt` and `llms-full.txt` enumerate markdown/HTML URLs from `src/utils/markdownPaths.ts`.
7. CI executes `docs:ci` to enforce feature bundle completeness, content contract compliance, and schema-to-reference mode coverage.

## Error and Recovery Paths

| Condition | Behavior |
| --- | --- |
| Missing slug in markdown mirror route | Returns markdown `404` response |
| Missing title/description on changed docs file | CI failure with explicit frontmatter error |
| Reference page missing key sections | CI failure (`Quick Reference`, `Common Parameters`) |
| Reference page missing a live schema mode/action | CI failure from `check-reference-schema-sync.mjs` |
| Blog post missing date/authors/tags | CI failure with required key list |
| Outdated evergreen content appears under `/blog/*` | IA violation; evergreen guides must live under `/articles/*` |
| Articles page not grouped by topic | IA regression; `/articles/` no longer reflects tag-grouped sections |

## State and Contracts

- **SEO contract:** Changed docs/blog/articles files require `title` and `description` frontmatter.
- **LLM contract:** Every docs/blog/articles page has deterministic markdown at `*.md` and is discoverable in `llms.txt`.
- **Reference contract:** Pages under `/reference/` keep predictable section anchors and cover live schema modes/actions.
- **Backward compatibility:** Legacy `/markdown/<slug>.md` routes remain available.

## Code Paths

- `cookwithgasoline.com/astro.config.mjs`
- `cookwithgasoline.com/src/components/Head.astro`
- `cookwithgasoline.com/src/components/Landing.astro`
- `cookwithgasoline.com/src/components/WorkflowLibrary.astro`
- `cookwithgasoline.com/src/components/ArticlesLibrary.astro`
- `cookwithgasoline.com/src/data/workflows.ts`
- `cookwithgasoline.com/src/pages/[...slug].md.ts`
- `cookwithgasoline.com/src/pages/index.md.ts`
- `cookwithgasoline.com/src/pages/llms.txt.ts`
- `cookwithgasoline.com/src/pages/llms-full.txt.ts`
- `cookwithgasoline.com/src/pages/markdown/[...slug].md.ts`
- `cookwithgasoline.com/src/utils/markdownPaths.ts`
- `scripts/docs/check-cookwithgasoline-content-contract.mjs`
- `scripts/docs/check-reference-schema-sync.mjs`
- `scripts/docs/check-feature-bundles.js`
- `.github/workflows/ci.yml`

## Test Paths

- `scripts/docs/check-cookwithgasoline-content-contract.mjs`
- `scripts/docs/check-reference-schema-sync.mjs`
- `scripts/docs/check-feature-bundles.js`

## Edit Guardrails

1. New docs/blog/articles routes must resolve to both HTML and `*.md` outputs.
2. Changes to markdown URL generation must update `llms.txt` and `llms-full.txt` behavior.
3. Reference pages must preserve `## Quick Reference` and `## Common Parameters` headings.
4. Reference pages must keep mode/action sections synchronized with `internal/schema/*`.
5. Keep legacy `/markdown/*` compatibility until consumers are migrated.
6. Any site IA refactor must update this flow map and its feature index in the same PR.
