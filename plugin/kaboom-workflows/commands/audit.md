---
name: kaboom/audit
description: Run the Kaboom Phase 1 audit for the current tracked site and return a six-lane local report.
argument: focus_prompt
allowed-tools:
  - mcp__kaboom__observe
  - mcp__kaboom__analyze
  - mcp__kaboom__interact
  - mcp__kaboom__configure
---

# /kaboom/audit — Phase 1 Product Audit

Run the Kaboom audit workflow against the current tracked site. If the client does not support namespaced commands, fall back to `/audit` with the same workflow.

This is the product-shaped Phase 1 audit: one tracked site, one local run, one polished report.

## Preconditions

- Start from the current tracked site or current tracked page.
- Run `configure(what:"health")` first.
- If there is no tracked site, stop and tell the operator to track the site before auditing.
- Respect the user's focus prompt when provided, but always cover the full six-lane baseline.

## Workflow

1. Confirm health and tracked-site context:
Run `configure(what:"health")`. If disconnected or no tracked site is available, stop and report the blocker.

2. Capture a baseline:
Run `analyze(what:"page_issues", summary:true)` and capture an opening screenshot.

3. Build a quick page map:
Use `interact(what:"explore_page")` and `interact(what:"list_interactive")` to identify the main routes, flows, CTAs, and risky states worth sampling.

4. Audit the six lanes:
- Functionality
- UX Polish
- Accessibility
- Performance
- Release Risk
- SEO

5. Gather supporting evidence:
Use screenshots plus targeted follow-up analysis for issues you find. Pull console/runtime, network, accessibility, performance, link health, security, and structural evidence as needed.

6. Synthesize one local report:
Return a polished Phase 1 report in the exact structure below.

## Lane Guidance

### Functionality

Check broken flows, API failures, console/runtime errors, state mismatches, and flaky interactions.

### UX Polish

Check confusing copy, awkward states, missing empty/loading/error states, visual inconsistency, and rough interaction details.

### Accessibility

Check keyboard flow, labels, semantics, contrast, focus treatment, and obvious WCAG failures.

### Performance

Check slow routes, heavy assets, layout instability, and delayed interaction response.

### Release Risk

Check broken links, security footguns, noisy errors, missing guardrails, and issues likely to surface in production.

### SEO

Check titles, descriptions, heading structure, crawl/indexing mistakes, canonicals, and weak discoverability signals.

## Report Format

Use this exact section order:

```md
# Kaboom Audit Report: [site]
## Overall Score
## Lane Scores
## Executive Summary
## Top Findings
## Fast Wins
## Ship Blockers
## Coverage And Limits
```

## Report Requirements

- Include all six lane names explicitly in `## Lane Scores`.
- Rank findings by severity and user impact.
- Use clear, plain-language recommendations tied to evidence.
- Mark any inferred technical conclusions as inferred.
- If a lane could not be fully assessed, explain the limit in `## Coverage And Limits`.
- Keep the final output client-ready and readable, not raw telemetry.
