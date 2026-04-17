---
name: audit
description: Use when the user wants a full Kaboom quality audit of a web app or tracked site, with one polished report covering Functionality, UX Polish, Accessibility, Performance, Release Risk, and SEO.
---

# Kaboom Audit

Use this skill for the product-shaped Phase 1 audit workflow.

Goal: audit the current tracked page or tracked site, cover the six required lanes, capture evidence, and return one polished local report.

## Non-Negotiables

- Run `configure(what:"health")` first.
- Start with `analyze(what:"page_issues", summary:true)` for a quick baseline.
- Begin from the current tracked page or tracked site when possible.
- If no tracked site exists, stop and tell the user to track the site before running the audit.
- Keep the audit local-only. No watch mode, history, hosted reports, or team workflow in this skill.
- Always say what was covered and what was not covered.

## Inputs To Request

- Optional focus prompt
- Any flows or pages that deserve extra scrutiny
- Any actions that should be avoided

If the user does not specify these, start from the current tracked page, map the obvious routes and CTAs, and avoid destructive actions.

## Workflow

1. Run `configure(what:"health")`.
2. Run `analyze(what:"page_issues", summary:true)`.
3. Capture a starting screenshot.
4. Discover the page structure with `interact(what:"explore_page")` and `interact(what:"list_interactive")`.
5. Audit these six lanes:
- Functionality
- UX Polish
- Accessibility
- Performance
- Release Risk
- SEO
6. Collect follow-up evidence with targeted screenshots and observe/analyze calls.
7. Produce one polished Kaboom audit report.

## Report Contract

Use this section order:

1. `# Kaboom Audit Report: [site]`
2. `## Overall Score`
3. `## Lane Scores`
4. `## Executive Summary`
5. `## Top Findings`
6. `## Fast Wins`
7. `## Ship Blockers`
8. `## Coverage And Limits`

## Lane Expectations

- Functionality: broken flows, API failures, console/runtime errors, state mismatches, flaky interactions
- UX Polish: confusing copy, awkward states, weak empty/loading/error states, visual inconsistency, rough interaction details
- Accessibility: keyboard flow, labels, semantics, contrast, focus treatment, obvious WCAG failures
- Performance: slow routes, heavy assets, layout instability, slow interaction response
- Release Risk: broken links, security footguns, noisy errors, missing guardrails, issues likely to surface in production
- SEO: titles, descriptions, heading structure, canonicals, crawl/indexing mistakes, weak discoverability signals

## Output Rules

- Include all six lanes even if some have limited evidence.
- Rank findings by severity and user impact.
- Keep recommendations concrete and tied to evidence.
- Mark inferred technical conclusions as inferred.
- Keep the final output readable and client-ready rather than dumping raw tool output.
