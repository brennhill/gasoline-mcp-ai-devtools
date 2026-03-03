---
doc_type: tech-spec
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
last_reviewed: 2026-03-03
---

# Cookwithgasoline Content Platform Tech Spec

## Architecture

1. Astro site configuration and routing:
   - `cookwithgasoline.com/astro.config.mjs`
   - markdown route handlers in `src/pages/`
2. Content and UX components:
   - `src/components/Head.astro`
   - `src/components/Landing.astro`
   - `src/components/WorkflowLibrary.astro`
   - `src/components/ArticlesLibrary.astro`
3. Content data and slug/path utilities:
   - `src/data/workflows.ts`
   - `src/utils/contentSlugs.ts`
   - `src/utils/markdownPaths.ts`
4. Contract enforcement:
   - `scripts/docs/check-cookwithgasoline-content-contract.mjs`
   - wired in CI via `.github/workflows/ci.yml`

## Content Contract

1. Changed docs/blog content must satisfy required frontmatter and structural checks.
2. Markdown mirrors must resolve to stable canonical slugs/paths.
3. LLM export routes must be generated from the same source of truth paths.

## Runtime Flow

1. Source docs/blog entries are read from Astro collections.
2. Slug normalization utilities resolve canonical route paths.
3. Markdown endpoints emit frontmatter-enriched markdown responses.
4. CI runs content contract checks to block invalid content changes.

## Reliability and Guardrails

1. Keep slug/path resolution centralized in `contentSlugs.ts` + `markdownPaths.ts`.
2. Keep content contract checks deterministic and script-driven.
3. Keep markdown route handlers aligned with canonical path generation logic.
