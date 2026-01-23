---
title: "Accessibility Audit"
description: "Run accessibility audits on your web app via MCP. AI assistants can identify WCAG violations, missing alt text, contrast issues, and ARIA problems."
keywords: "accessibility audit MCP, axe-core MCP tool, WCAG violations, accessibility testing AI, a11y audit browser"
permalink: /accessibility-audit/
toc: true
toc_sticky: true
---

Gasoline's `run_accessibility_audit` MCP tool lets AI assistants run accessibility audits on the current page or a scoped element, surfacing WCAG violations as actionable findings.

## How It Works

1. Your AI calls `run_accessibility_audit`
2. The extension runs an accessibility check against the live page
3. Violations are returned with severity, affected elements, and fix suggestions

## MCP Tool: `run_accessibility_audit`

Run a full-page audit or scope to a specific element:

- Full page audit (no arguments)
- Scoped audit (pass a CSS selector to limit scope)

## What's Reported

Each violation includes:

- **Rule ID** — the accessibility rule that was violated
- **Severity** — critical, serious, moderate, or minor
- **Description** — what the issue is
- **Affected elements** — which DOM elements have the problem
- **Fix suggestion** — how to resolve the violation

## Common Findings

| Category | Examples |
|----------|---------|
| Images | Missing alt text, decorative images not marked |
| Color | Insufficient contrast ratio |
| Forms | Missing labels, unlabeled inputs |
| ARIA | Invalid roles, missing required attributes |
| Structure | Missing landmarks, heading order issues |
| Keyboard | Elements not focusable, missing focus indicators |

## Use Cases

- Catch accessibility issues during development before code review
- Let AI assistants suggest ARIA fixes alongside functional code
- Audit after UI changes to prevent accessibility regressions
- Scope audits to specific components ("Audit just the login form")
