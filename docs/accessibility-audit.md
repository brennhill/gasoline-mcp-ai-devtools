---
title: "Accessibility Audit"
description: "Run accessibility audits on your web app via MCP. AI assistants identify WCAG violations, missing alt text, contrast issues, and ARIA problems — then fix them."
keywords: "accessibility audit MCP, axe-core MCP tool, WCAG violations, accessibility testing AI, a11y audit browser, WCAG compliance"
permalink: /accessibility-audit/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Surface accessibility violations. Let your AI fix them."
toc: true
toc_sticky: true
---

Gasoline's `run_accessibility_audit` tool surfaces WCAG violations as actionable findings your AI can fix.

## <i class="fas fa-exclamation-circle"></i> The Problem

Accessibility is important but easy to overlook during development. Running audits manually, interpreting violations, and knowing how to fix them adds friction. Most developers defer a11y until it becomes a compliance requirement.

With Gasoline, your AI runs the audit and fixes the issues in one step.

## <i class="fas fa-cogs"></i> How It Works

1. <i class="fas fa-terminal"></i> Your AI calls `run_accessibility_audit`
2. <i class="fas fa-search"></i> Extension runs axe-core against the live page
3. <i class="fas fa-list"></i> Violations returned with severity, elements, and remediation guidance

```json
// AI runs audit on the login form
{
  "scope": "#login-form",
  "tags": ["wcag2a", "wcag2aa"]
}

// Response:
{
  "violations": [{
    "id": "label",
    "impact": "critical",
    "description": "Form elements must have labels",
    "nodes": [{
      "target": ["input#email"],
      "html": "<input type=\"email\" id=\"email\" placeholder=\"Email\">"
    }]
  }]
}
```

## <i class="fas fa-tools"></i> MCP Tool Parameters

| Parameter | Description |
|-----------|-------------|
| `scope` | CSS selector to scope the audit (default: full page) |
| `tags` | WCAG tags to test: `wcag2a`, `wcag2aa`, `wcag2aaa`, `best-practice` |

## <i class="fas fa-clipboard-list"></i> What's Reported

Each violation includes:

| Field | Description |
|-------|-------------|
| <i class="fas fa-tag"></i> Rule ID | axe-core rule identifier (e.g., `color-contrast`) |
| <i class="fas fa-exclamation-circle"></i> Impact | Critical, serious, moderate, or minor |
| <i class="fas fa-align-left"></i> Description | Human-readable explanation |
| <i class="fas fa-code"></i> Affected elements | CSS selectors and HTML snippets |
| <i class="fas fa-external-link-alt"></i> Help URL | Link to detailed remediation guidance |

## <i class="fas fa-layer-group"></i> Supported WCAG Levels

| Tag | Coverage |
|-----|----------|
| `wcag2a` | Level A (minimum compliance) |
| `wcag2aa` | Level AA (standard compliance) |
| `wcag2aaa` | Level AAA (enhanced compliance) |
| `best-practice` | Industry best practices beyond WCAG |

## <i class="fas fa-search"></i> Common Findings

| Category | Examples |
|----------|---------|
| <i class="fas fa-image"></i> Images | Missing alt text, decorative images not marked |
| <i class="fas fa-palette"></i> Color | Insufficient contrast ratio |
| <i class="fas fa-edit"></i> Forms | Missing labels, unlabeled inputs |
| <i class="fas fa-project-diagram"></i> ARIA | Invalid roles, missing required attributes |
| <i class="fas fa-sitemap"></i> Structure | Missing landmarks, heading order |
| <i class="fas fa-keyboard"></i> Keyboard | Not focusable, missing focus indicators |

## <i class="fas fa-brain"></i> The AI Advantage

Your AI doesn't just report violations — it fixes them. Because it has access to both the violation details (element selectors, rule descriptions) and your source code, it can:

- Add missing ARIA labels to form inputs
- Fix color contrast ratios in CSS
- Add alt text to images
- Correct heading hierarchy
- Add keyboard navigation handlers
- Re-run the audit to verify the fix

## <i class="fas fa-fire-alt"></i> Use Cases

### Pre-Commit Checks

> "Run an accessibility audit on this page before I commit."

Catch issues before they reach production.

### Component Development

> "Check the accessibility of this new modal component."

Scope the audit to just the component you're building.

### Compliance Remediation

> "Fix all critical accessibility issues on this page."

Your AI audits, identifies violations, and applies fixes in one flow.

### Post-Refactor Verification

> "Did my refactor break any accessibility?"

Quick audit after UI changes catches regressions before users hit them.
