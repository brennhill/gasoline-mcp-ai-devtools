---
doc_type: qa-plan
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
last_reviewed: 2026-03-03
---

# Cookwithgasoline Content Platform QA Plan

## Automated Gates

1. `npm run docs:lint:content-contract`
2. `npm run docs:ci`
3. Site quality gates in `npm run check`

## Required Scenarios

1. Homepage renders core messaging and workflow/article discovery sections.
2. Workflow and article index components resolve expected content links.
3. Markdown routes return valid content with canonical metadata:
   - `/index.md`
   - `/[...slug].md`
   - `/markdown/[...slug].md`
4. LLM exports are reachable and generated from current content:
   - `/llms.txt`
   - `/llms-full.txt`
5. Content contract linter fails on malformed/invalid changed content.

## Regression Checklist

1. Verify slug resolution and canonical path handling after route/content changes.
2. Verify markdown route handlers keep frontmatter/content signal headers intact.
3. Verify CI blocks merges on content-contract violations.
