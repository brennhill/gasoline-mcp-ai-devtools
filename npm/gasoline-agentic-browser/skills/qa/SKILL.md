---
name: qa
description: Use when user says "QA my app", "find all bugs", "check my app for problems", "debug my whole app", "what's broken", or wants a comprehensive quality review of their web application.
---

# Gasoline QA

Use this skill when the user wants you to act as a QA tester — systematically walk through their web app, find everything that's broken, and report back with a prioritized list of issues and evidence.

This is the one-command alternative to manually directing individual debug/observe/analyze calls. The user says "QA my app" and you do the rest.

## Non-Negotiables

- Run `configure(what:"health")` first to verify the daemon and extension are connected.
- Start every QA session with `analyze(what:"page_issues", summary:true)` for a fast baseline.
- Take a screenshot before and after every page navigation.
- Never execute destructive actions (delete, submit payment, reset) unless the user explicitly approves.
- If you hit a login page, stop and ask the user to log in manually. Resume after they confirm.
- Always report what was checked AND what was not checked. Partial coverage is fine; silent gaps are not.
- Ask the user when behavior is ambiguous ("is this button supposed to do X?") rather than guessing.

## Inputs To Request

- Target URL (or "the page I'm on" if already tracked)
- Scope: full app or specific pages/flows
- Anything that should NOT be touched (admin, billing, destructive actions)

If the user doesn't provide these, use sensible defaults: start from the current tracked page, explore everything reachable, and avoid destructive actions.

## Workflow

1. **Pre-flight check:**
Run `configure(what:"health")` to verify daemon and extension connectivity. If unhealthy, stop and help the user fix the connection before proceeding.

2. **Quick scan — get the baseline:**
Run `analyze(what:"page_issues", summary:true)` to surface all detectable issues on the current page in one call: console errors, network failures, a11y violations, and security findings.
Take a screenshot with `observe(what:"screenshot")` to assess visual state.

3. **Discover navigation:**
Run `interact(what:"explore_page")` and `interact(what:"list_interactive")` to find all navigable links and interactive elements.
Build a queue of pages to visit from nav links, sidebar, footer, and major CTAs.

4. **Walk each page:**
For each page in the queue:
- `interact(what:"navigate", url:"<page_url>", wait_for_stable:true)`
- `observe(what:"screenshot")` — capture visual state
- `analyze(what:"page_issues", summary:true)` — run the full check sweep
- `interact(what:"list_interactive")` — find interactive elements
- Click key interactive elements (buttons, forms, tabs) and observe what happens
- Note any issues found with their severity and evidence

5. **Handle auth walls:**
If you encounter a login page during navigation, stop and tell the user: "I've hit a login page at [URL]. Please log in, then tell me to continue."
Do not attempt to fill credentials.

6. **Gather deep evidence for each issue:**
For issues found in steps 2-4, gather targeted evidence:
- Console errors: `observe(what:"error_bundles")` for pre-assembled error context
- Network failures: `observe(what:"network_bodies", status_min:400)` for response details
- DOM issues: `analyze(what:"dom", selector:"<problem_area>")` for element state
- Visual issues: reference the screenshots taken during walkthrough

7. **Ask about ambiguous behavior:**
If something looks wrong but could be intentional, ask the user. Don't guess.
Examples: "This button doesn't seem to do anything — is that expected?" or "The API returns a 404 at /api/v2/settings — is this endpoint supposed to exist?"

8. **Produce the QA report:**
Organize findings by severity (critical → high → medium → low):
- For each issue: what's wrong, where it is, evidence (screenshot, error log, network trace)
- Summary of what was checked and what was not
- Total issue counts by category and severity

9. **Offer to fix (if source code is accessible):**
If the user's source code is available in this session, ask: "Want me to start fixing these? I'll begin with the critical/high issues."
If yes, work through the list one by one. After each fix, re-run `analyze(what:"page_issues")` on the affected page to verify the fix landed.
If source code is not accessible, present the report as the deliverable.

## Coverage Rules

- Breadth first: visit all unique pages before deep-diving into any single page.
- Cap exploration: if the app has 50+ pages, focus on the main navigation paths and flag the rest as "not visited."
- Avoid infinite traps: skip calendar widgets, infinite scroll feeds, and URL-parameterized pagination beyond 2 pages.
- Track what you visited: maintain a simple list of URLs checked so you can report coverage.
- Pacing: use `wait_for_stable:true` on navigate calls and allow pages to fully load before running `analyze`. Rapid-fire tool calls on unstable pages produce incomplete data.

## Output Contract

- `issue_inventory` — all issues found, each with: severity, category, page_url, description, evidence
- `page_coverage` — list of pages visited and pages discovered but not visited
- `checks_summary` — which check categories ran on each page (console, network, a11y, security)
- `priority_backlog` — issues ranked by severity with suggested fix order
