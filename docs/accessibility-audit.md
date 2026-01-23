---
title: "Accessibility Audit"
description: "Run accessibility audits on your web app via MCP. AI assistants can identify WCAG violations, missing alt text, contrast issues, and ARIA problems."
keywords: "accessibility audit MCP, axe-core MCP tool, WCAG violations, accessibility testing AI, a11y audit browser"
permalink: /accessibility-audit/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Surface accessibility violations. Let your AI fix them."
toc: true
toc_sticky: true
---

Gasoline's `run_accessibility_audit` tool surfaces WCAG violations as actionable findings your AI can fix.

## <i class="fas fa-cogs"></i> How It Works

1. <i class="fas fa-terminal"></i> Your AI calls `run_accessibility_audit`
2. <i class="fas fa-search"></i> Extension audits the live page
3. <i class="fas fa-list"></i> Violations returned with severity, elements, and fixes

## <i class="fas fa-universal-access"></i> Scope Options

- **Full page** — audit everything (no arguments)
- **Scoped** — pass a CSS selector to audit just one section

## <i class="fas fa-clipboard-list"></i> What's Reported

Each violation includes:

| Field | Description |
|-------|-------------|
| <i class="fas fa-tag"></i> Rule ID | The accessibility rule violated |
| <i class="fas fa-exclamation-circle"></i> Severity | Critical, serious, moderate, or minor |
| <i class="fas fa-align-left"></i> Description | What the issue is |
| <i class="fas fa-code"></i> Affected elements | Which DOM elements have the problem |
| <i class="fas fa-check"></i> Fix suggestion | How to resolve the violation |

## <i class="fas fa-search"></i> Common Findings

| Category | Examples |
|----------|---------|
| <i class="fas fa-image"></i> Images | Missing alt text, decorative images not marked |
| <i class="fas fa-palette"></i> Color | Insufficient contrast ratio |
| <i class="fas fa-edit"></i> Forms | Missing labels, unlabeled inputs |
| <i class="fas fa-project-diagram"></i> ARIA | Invalid roles, missing required attributes |
| <i class="fas fa-sitemap"></i> Structure | Missing landmarks, heading order |
| <i class="fas fa-keyboard"></i> Keyboard | Not focusable, missing focus indicators |

## <i class="fas fa-fire-alt"></i> Use Cases

- Catch a11y issues during development (before code review)
- Let AI suggest ARIA fixes alongside functional code
- Audit after UI changes to prevent regressions
- Scope to components: _"Audit just the login form"_
