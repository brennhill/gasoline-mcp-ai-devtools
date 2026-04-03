# Kaboom Phase 1 Audit Design

## Goal

Define the first product-shaped `Audit` experience for Kaboom.

Phase 1 should turn Kaboom from "a powerful set of MCP browser tools" into "an AI QA copilot for vibe-coded apps" by shipping one strong local audit workflow and one polished local report.

## Decision Summary

The approved direction is:

- Kaboom should position itself as an AI QA copilot for vibe-coded apps
- Phase 1 is audit-first, not fix-first
- the audit should work against any tracked web app, not only local or preview environments
- the primary human entry point should be an `Audit` button shown after a site is tracked
- the primary agent entry point should be a `/kaboom/audit` skill
- both entry points should invoke the same underlying audit orchestrator
- the audit should score six lanes:
  - functionality
  - UX polish
  - accessibility
  - performance
  - release risk
  - SEO
- the output should be one polished local report
- recurring audits, watch mode, hosted reports, history, team workflow, and white-labeling are Pro features and are out of scope for Phase 1

## Problem Statement

Kaboom already has strong primitives for observing and analyzing browser behavior, but the product story is still tool-centric. Users can inspect logs, run audits, and automate the browser, yet the most important user outcome is still too manual:

- figure out what feels broken
- figure out what feels rough or low-quality
- prioritize what to fix next
- communicate those findings in a professional way

The target users for the new product direction are:

1. solo indie builders shipping fast with AI
2. small startup teams cleaning up post-vibe-code apps
3. agencies and freelancers cleaning up client products

Those users do not primarily want another low-level tool surface. They want Kaboom to review a site like a strong QA and product reviewer, then hand back a high-trust audit artifact.

## Core Product Promise

Phase 1 should make this promise:

> Point Kaboom at any tracked web app and get a professional quality audit that tells you what feels broken, rough, risky, or unshippable.

Internally, this is the "post-loveable" direction: Kaboom helps people clean up the mess caused by vibe-coded products.

Externally, the messaging should avoid anchoring on other brands and stay close to:

- AI QA copilot for vibe-coded apps
- audit-first cleanup for messy AI-built products
- professional quality audits for products shipped fast with AI

## Scope

### In Scope

- one local/manual audit run against a tracked site
- human entry point via an `Audit` button on the tracked-site surface
- agent entry point via a `/kaboom/audit` skill
- one shared audit orchestrator
- app exploration before evaluation
- six audit lanes
- evidence capture during the audit
- one polished local report

### Out Of Scope

- recurring audits
- watch mode
- historical comparisons
- hosted reports or share links
- artifact hosting
- projects, comments, assignments, or collaboration
- white-label or agency branding
- CI and release automation
- automatic fixing as the primary workflow

## User Experience

### Human Entry Point

Kaboom already has a "track this site" workflow. Phase 1 should extend that existing mental model rather than introducing a separate audit setup flow.

The human workflow should be:

1. user clicks `Track this site`
2. once a site is tracked, Kaboom shows an `Audit` button
3. the user clicks `Audit`
4. Kaboom runs the audit against the currently tracked site
5. Kaboom produces a polished local report

This keeps the product simple:

- `Track` means "connect Kaboom to this app"
- `Audit` means "review this tracked app like an AI QA copilot"

### Agent Entry Point

Kaboom should ship a `/kaboom/audit` skill as the high-level agent interface for the same workflow.

The skill should:

- require a tracked site or guide the user to track one first
- accept an optional focus prompt
- invoke the same audit orchestrator as the UI button
- produce the same report structure

This prevents agents from needing to manually stitch together `observe`, `interact`, and `analyze` calls for the core product promise.

### Optional Focus Prompt

The default audit should be one-click useful, but the workflow may accept a short focus prompt such as:

- "Audit onboarding and signup"
- "Focus on polish and trust issues"
- "Review this like a client-facing QA pass"

Phase 1 should keep this prompt lightweight. The prompt is a focus hint, not a complex audit configuration surface.

## Audit Workflow

Both the `Audit` button and `/kaboom/audit` skill should drive the same workflow:

1. Confirm a tracked site exists
2. Build a quick map of the app
3. Explore likely routes and core flows
4. Run the six audit lanes
5. Capture evidence throughout the run
6. Synthesize findings into one local report

### 1. Confirm A Tracked Site Exists

If no site is tracked, Kaboom should not start the audit. It should instruct the user or agent to track a site first.

### 2. Build A Quick Map Of The App

Before deep evaluation, the audit should identify:

- key routes or pages
- primary navigation structure
- likely critical flows
- obvious entry points such as homepage, signup, login, pricing, dashboard, checkout, or settings

This map should be fast and heuristic-driven. Phase 1 does not need a full crawler or a persistent site model.

### 3. Explore Likely Routes And Core Flows

Kaboom should behave like a strong QA reviewer, not a static rules engine.

The audit should explore:

- obvious click paths
- key user journeys
- common state transitions
- visible loading, empty, success, and error states when available

The goal is not exhaustive coverage. The goal is enough coverage to produce a high-signal quality review.

### 4. Run The Six Audit Lanes

The audit should score and evaluate these lanes:

1. Functionality
2. UX Polish
3. Accessibility
4. Performance
5. Release Risk
6. SEO

Lane definitions:

- Functionality: broken flows, API failures, console errors, state mismatches, flaky interactions
- UX Polish: confusing copy, awkward states, weak empty/loading/error states, visual inconsistency, rough interaction details
- Accessibility: keyboard flow, labels, semantics, contrast, focus treatment, obvious WCAG failures
- Performance: slow routes, heavy assets, layout instability, slow interaction response
- Release Risk: broken links, security footguns, noisy errors, missing guardrails, issues likely to surface in production
- SEO: missing or weak titles and descriptions, heading misuse, crawl/indexing mistakes, bad canonicals, thin internal linking, structurally weak pages

### 5. Capture Evidence Throughout The Run

The audit should capture evidence while exploring rather than only summarizing after the fact.

Evidence types may include:

- screenshots
- console findings
- network failures
- notable UI states
- accessibility findings
- performance findings
- broken links
- structural SEO problems

The evidence model should support report-quality presentation rather than raw event dumps.

### 6. Synthesize Findings Into One Local Report

Phase 1 should end with one report artifact, not a pile of raw tool outputs.

## Report Contract

The local report should feel polished and client-grade.

Minimum sections:

- overall score
- six lane scores
- executive summary
- top findings ranked by severity and impact
- screenshot-backed evidence where relevant
- explanation of why each issue matters
- recommended next action for each issue
- fast wins section
- ship blockers section when applicable

### Severity Model

Findings should use a simple severity model:

- Critical
- High
- Medium
- Low

Each finding should also carry:

- user impact
- fix effort

This keeps the report focused on prioritization rather than raw issue volume.

## Product Differentiation

Kaboom should not feel like a generic scanner.

The audit should distinguish itself by combining:

- runtime evidence
- journey exploration
- standards-based checks
- product judgment

The desired voice of the report is:

- plain language
- high trust
- non-jargony
- clear about what matters and why

The bar is not merely "detected missing alt text" or "found broken link." The audit should be able to say, in effect:

- this flow technically works but feels low-trust
- this state is confusing and likely to cause drop-off
- this app is shippable only after a short set of blockers is fixed

That judgment layer is the core product value.

## Architecture Direction

Phase 1 should introduce one shared audit orchestrator rather than embedding audit logic separately in UI and agent flows.

At a minimum, the implementation should separate:

- audit entry points
- audit orchestration
- lane evaluators
- evidence collection
- report schema
- report rendering

### Entry Points

Two entry points call the same underlying audit flow:

- tracked-site `Audit` button
- `/kaboom/audit` skill

### Orchestrator

The orchestrator should:

- validate prerequisites
- drive site mapping and exploration
- call lane evaluators
- aggregate findings and evidence
- produce the final report payload

### Lane Evaluators

Lane evaluators should reuse existing Kaboom capabilities wherever possible instead of building a parallel stack.

Expected reuse areas:

- console and runtime observation
- network failures and response inspection
- accessibility analysis
- performance analysis
- link health analysis
- SEO checks where feasible
- interaction/exploration primitives

### Report Schema

Phase 1 should define a canonical audit report structure so UI and agent outputs stay aligned.

The schema should cover:

- metadata
- audit scope
- lane scores
- findings
- evidence references
- summaries
- recommended actions

### Report Renderer

The first renderer can remain local, but it must be intentionally designed. Phase 1 should avoid raw JSON or loosely formatted terminal dumps as the primary user-facing artifact.

## Success Criteria

Phase 1 is successful if:

- a tracked site can be audited from one obvious UI action
- an agent can run the same workflow through `/kaboom/audit`
- the audit produces a coherent report instead of fragmented tool output
- the six audit lanes are represented clearly
- the report helps a builder decide what to fix next
- the experience feels like product review and QA judgment, not only linting

## Risks

- The audit may become a thin wrapper around existing tools and fail to feel product-shaped
- The exploration pass may overreach and become fragile if it tries to cover too many journeys
- UX polish findings may feel vague unless the report voice is explicit and concrete
- SEO can sprawl into a separate product area if Phase 1 does not keep it practical
- Too much configuration will weaken the one-click value of the `Audit` button

## Open Questions

- How much route exploration should happen automatically before the audit becomes too slow or unpredictable?
- What is the right default balance between breadth of site coverage and depth of journey coverage?
- Should the first report renderer live in the extension UI, the terminal, a generated local HTML report, or a combination of these?
- What minimal focus-prompt grammar is useful without creating a settings-heavy workflow?

## Next Step

After this design is approved in written form, the next planning step should define the concrete Phase 1 implementation plan for:

- the audit orchestrator
- the report schema
- the shared entry-point contract
- the initial lane evaluators
- the first report renderer
