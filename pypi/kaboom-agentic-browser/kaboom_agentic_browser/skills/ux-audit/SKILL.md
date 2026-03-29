---
name: ux-audit
description: Use when user asks to review UX quality, check accessibility compliance, audit visual hierarchy, or evaluate information architecture on a page.
---

# Kaboom UX Audit

Use this skill to evaluate usability quality, not just functional correctness.

## Inputs To Request

- Primary user journey
- Target pages or flows
- Device scope (desktop/mobile)

## Workflow

1. Capture visual evidence:
Collect screenshots for key states (default, loading, error, success) on desktop and mobile.

2. Run accessibility checks:
Use `analyze(accessibility)` and verify keyboard/focus flow, labels, landmarks, and contrast.

3. Review information hierarchy:
Evaluate heading order, primary CTA prominence, content scan order, and above-the-fold clarity.

4. Identify interaction ambiguity:
Call out unclear affordances, weak feedback, and confusing error/help text.

5. Prioritize fixes:
Rank by severity and user impact; provide fast wins first.

## Output Contract

- `a11y_issues`
- `hierarchy_findings`
- `clarity_gaps`
- `priority_fixes`
- `validation_plan`
