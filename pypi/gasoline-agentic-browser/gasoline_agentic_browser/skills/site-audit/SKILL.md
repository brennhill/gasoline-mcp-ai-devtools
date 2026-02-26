---
name: site-audit
description: Perform a comprehensive logged-in site/product audit with full menu capture, page-by-page and feature-by-feature analysis, usability findings, and reproducibility notes.
---

# Gasoline Site Audit

Use this skill when the user wants a full-product, screenshot-backed audit of a site or web app after login.
Goal: maximize meaningful coverage inside strict scope/budget limits, then produce a detailed report with strengths, weaknesses, risks, and technical notes.

## Non-Negotiables

- Capture the full menu system first: top nav, side nav, footer nav, user/account menu, overflow/context menus, and role-based visibility differences.
- Deliver both:
1. page-by-page breakdown
2. feature-by-feature breakdown
- Include a dedicated usability report section with clear positives, pain points, and severity.
- Produce a reproducibility packet detailed enough that another team could reconstruct the app/site structure and core flows.

## Inputs To Request

- Start URL
- Audit boundary (allowed domains and path prefixes)
- Logged-in readiness (which account/role is already authenticated)
- Page budget and time budget
- Priority flows and high-value areas
- Exclusions (logout, destructive admin actions, billing changes, etc.)

## Workflow

1. Define scope and budget:
Set strict crawl bounds to avoid unbounded exploration. Use best-effort exhaustive discovery within those bounds.

2. Confirm authenticated starting state:
Begin only after the user/session is logged in and on the intended start page.

3. Build a discovery queue:
Seed from global navigation, sidebars, footers, in-page links, and major CTAs.
De-duplicate by canonical URL (ignore tracking query params unless they change behavior).

4. Capture complete menu inventory:
Expand and record every menu and submenu before broad traversal.
For each menu item, capture label, destination, visibility rules, and behavior (navigation/action/modal).

5. Traverse pages and key states:
Visit queued pages, then discover more URLs from each page.
Also capture state-level views (tabs, modals, drawers, filters, accordions, pagination).

6. Capture per-page visual evidence:
For each page/state, capture:
- top-of-page screenshot
- mid-page screenshot (after scroll)
- bottom-of-page screenshot (after full scroll)
- full-page screenshot when available
Always scroll back to top before leaving the page.

7. Capture per-page diagnostics:
Collect page summary, console/runtime errors, warning/error logs, failing network requests, and notable performance clues.

8. Build feature-by-feature inventory:
Record each major feature and its key interactions:
- forms (validation, error/success states)
- search/filter/sort
- navigation/menu behavior
- data tables/lists/pagination
- uploads/downloads
- onboarding/settings/profile/account flows
For each feature, capture:
- entry points (menus/pages/CTAs)
- preconditions and permissions
- step-by-step interaction flow
- system response and state transitions
- dependencies on APIs or backend behavior
- observed quality issues and evidence

9. Analyze UX and accessibility:
Assess hierarchy, clarity, affordances, feedback, error handling, empty/loading states, consistency, and accessibility basics (labels, focus flow, landmarks, contrast signals).

10. Extract technical implementation signals:
Infer likely stack and implementation details from observable evidence:
- framework/runtime hints
- API styles (REST/GraphQL), endpoint patterns, status failures
- caching/lazy-loading/pagination behavior
- client rendering patterns and potential performance bottlenecks
Mark inferred items explicitly as inferred.

11. Build reproducibility packet:
Compile route map, menu map, feature map, key user flows, major states, and implementation clues so another team can reproduce the product structure and behavior.

12. Synthesize findings into a final report:
Rank issues by severity and user impact. Include concrete evidence references (screenshots, logs, endpoints, page URLs).

## Per-Page Checklist

For every audited page/state, capture all of:

- `page_id` and canonical URL
- Page purpose in 1-3 sentences
- Primary user jobs on that page
- Major UI regions/components
- Outbound links/routes and what they appear to do
- Key interactions attempted and outcomes
- UX strengths
- UX issues/gaps
- Accessibility observations
- Technical observations (observed + inferred)
- Screenshot references (top/mid/bottom/full)
- Errors/warnings/network failures tied to that page

## Feature Checklist

For every feature, capture all of:

- `feature_id` and feature name
- Business purpose / user goal
- Entry points (menus, routes, CTAs)
- Preconditions and permission requirements
- End-to-end interaction steps
- Success path, failure path, and edge states
- Data dependencies and external/system dependencies
- UX/usability quality notes
- Accessibility notes
- Technical observations (observed + inferred)
- Evidence references (screenshots, logs, network traces)

## Menu Capture Checklist

- Global top navigation (all primary items)
- Side navigation and nested branches
- Footer navigation and utility links
- User/account/profile dropdowns
- Overflow/context menus attached to key components
- Role-dependent menu differences (if visible)
- For each item: label, location, destination/action, and evidence screenshot

## Coverage Rules

- Prefer breadth first, then depth:
Cover all unique routes first, then deepen interaction coverage on high-value routes.
- Avoid infinite traps:
Cap repeated patterns (calendar days, endless feeds, faceted URL explosions).
- Respect safety boundaries:
Do not execute destructive actions unless explicitly approved.
- Track coverage gaps:
If blocked by permissions, role restrictions, or time budget, list what was not covered and why.

## Output Contract

- `site_map`
- `coverage_summary`
- `menu_inventory`
- `page_dossiers`
- `feature_inventory`
- `interaction_results`
- `usability_report`
- `ux_findings`
- `accessibility_findings`
- `technical_observations`
- `reproducibility_packet`
- `issue_inventory` (severity, impact, evidence, owner suggestion)
- `strengths` (what is working well)
- `priority_backlog` (ranked fixes with expected impact)
- `open_questions_and_gaps`

## Final Report Template

Use this section structure in order for every full site audit. If a section is not applicable, write `Not observed` with a short reason.

1. Executive summary
2. Scope, constraints, and audit method
3. Coverage summary (routes discovered, routes audited, interactions tested, gaps)
4. Full menu inventory and navigation architecture
5. Page-by-page dossiers (with screenshot references)
6. Feature-by-feature dossiers (flows, dependencies, quality)
7. Usability report (clarity, friction, consistency, severity)
8. UX and accessibility assessment
9. Technical implementation notes (observed vs inferred)
10. Reproducibility packet (enough to reconstruct app/site behavior)
11. What is good (strengths)
12. What is bad (issues, anti-patterns, risks)
13. Prioritized remediation plan
14. Appendix: evidence index (screenshots, logs, failed requests, notable traces)
