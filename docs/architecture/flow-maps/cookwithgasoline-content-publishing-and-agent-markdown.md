---
doc_type: flow_map
flow_id: cookwithgasoline-content-publishing-and-agent-markdown
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
feature_ids:
  - feature-cookwithgasoline-content-platform
entrypoints:
  - cookwithgasoline.com/src/content/docs/index.mdx
  - cookwithgasoline.com/src/pages/[...slug].md.ts
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
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
  - cookwithgasoline.com/src/utils/markdownPaths.ts
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
  - .github/workflows/ci.yml
test_paths:
  - scripts/docs/check-cookwithgasoline-content-contract.mjs
---

# Cookwithgasoline Content Publishing and Agent Markdown Flow

## Scope

Covers homepage, workflow discovery, article discovery, tool-reference navigation, and automatic per-route markdown mirrors for agent consumption.

## Entrypoints

- Splash homepage in `cookwithgasoline.com/src/content/docs/index.mdx`
- Agent markdown mirror route in `cookwithgasoline.com/src/pages/[...slug].md.ts`
- Content contract gate in `scripts/docs/check-cookwithgasoline-content-contract.mjs`

## Primary Flow

1. Starlight loads docs/blog entries from `cookwithgasoline.com/src/content/docs/*` through `docsLoader()`.
2. Site navigation and information architecture are defined in `cookwithgasoline.com/astro.config.mjs`.
3. Splash pages render reusable components for marketing and discovery (`Landing.astro`, `WorkflowLibrary.astro`, `ArticlesLibrary.astro`).
4. Every docs/blog slug is mirrored as `/<slug>.md` via `src/pages/[...slug].md.ts`.
5. `<link rel="alternate" type="text/markdown">` in `Head.astro` points each HTML route to its markdown mirror.
6. `llms.txt` and `llms-full.txt` enumerate markdown/HTML URLs from `src/utils/markdownPaths.ts`.
7. CI executes `docs:lint:content-contract` to enforce format compliance for changed docs/blog files.

## Error and Recovery Paths

| Condition | Behavior |
| --- | --- |
| Missing slug in markdown mirror route | Returns markdown `404` response |
| Missing title/description on changed docs file | CI failure with explicit frontmatter error |
| Reference page missing key sections | CI failure (`Quick Reference`, `Common Parameters`) |
| Blog post missing date/authors/tags | CI failure with required key list |

## State and Contracts

- **SEO contract:** Changed docs/blog files require `title` and `description` frontmatter.
- **LLM contract:** Every docs/blog page has deterministic markdown at `*.md` and is discoverable in `llms.txt`.
- **Reference contract:** Pages under `/reference/` keep predictable section anchors.
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
- `.github/workflows/ci.yml`

## Test Paths

- `scripts/docs/check-cookwithgasoline-content-contract.mjs`

## Edit Guardrails

1. New docs/blog routes must resolve to both HTML and `*.md` outputs.
2. Changes to markdown URL generation must update `llms.txt` and `llms-full.txt` behavior.
3. Reference pages must preserve `## Quick Reference` and `## Common Parameters` headings.
4. Keep legacy `/markdown/*` compatibility until consumers are migrated.
5. Any site IA refactor must update this flow map and its feature index in the same PR.
