---
name: qa
description: Use when user says "QA my app", "find all bugs", "check my app for problems", "debug my whole app", "what's broken", or wants a comprehensive quality review of their web application.
---

# Kaboom QA

Use this skill when the user wants you to act as a QA tester: systematically walk through their web app, find what's broken, and report back with prioritized issues and evidence.

This is the one-command alternative to manually directing individual debug, observe, or analyze calls.

## Non-Negotiables

- Run `configure(what:"health")` first to verify the daemon and extension are connected.
- Start every QA session with `analyze(what:"page_issues", summary:true)` for a fast baseline.
- Take a screenshot before and after every page navigation.
- Never execute destructive actions unless the user explicitly approves.
- If you hit a login page, stop and ask the user to log in manually.
- Always report what was checked and what was not checked.
- Ask the user when behavior is ambiguous instead of guessing.

## Inputs To Request

- Target URL, or the current tracked page
- Scope: full app or specific pages/flows
- Anything that should not be touched

If the user does not provide these, use sensible defaults: start from the current tracked page, explore everything reachable, and avoid destructive actions.

## Workflow

1. Run `configure(what:"health")`.
2. Run `analyze(what:"page_issues", summary:true)`.
3. Capture `observe(what:"screenshot")`.
4. Discover navigation with `interact(what:"explore_page")` and `interact(what:"list_interactive")`.
5. Walk each page, capturing screenshots and running `analyze(what:"page_issues", summary:true)`.
6. Gather deeper evidence for issues with targeted observe/analyze calls.
7. Ask about ambiguous behavior.
8. Produce a severity-ranked QA report with coverage notes.

## Output Contract

- `issue_inventory`
- `page_coverage`
- `checks_summary`
- `priority_backlog`
