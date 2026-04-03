---
name: qa
description: Use when user says "QA my app", "find all bugs", "check my app for problems", "debug my whole app", "what's broken", or wants a comprehensive quality review of their web application.
---

# Kaboom QA

Use this skill as a compatibility entrypoint for the Kaboom audit workflow.

If the environment supports slash commands, route the session to `/kaboom/audit`.
If namespaced slash commands are not available, route it to `/audit`.
If slash commands are unavailable entirely, follow the `audit` skill contract directly.

This wrapper exists so older `qa` references still land on the single Phase 1 audit methodology instead of maintaining a second QA workflow.

## Redirect Rules

- Treat `qa` as an alias for the Kaboom audit workflow.
- Start from the current tracked page or tracked site when possible.
- Run the same six-lane audit:
- Functionality
- UX Polish
- Accessibility
- Performance
- Release Risk
- SEO
- Return the same report sections expected by the `audit` skill:
- Overall Score
- Lane Scores
- Executive Summary
- Top Findings
- Fast Wins
- Ship Blockers
- Coverage And Limits
