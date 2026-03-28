---
name: performance
description: Use when user reports slow page load, asks about Core Web Vitals, wants before/after performance comparison, or needs to profile a specific interaction.
---

# Gasoline Performance

Use this skill for performance triage, regression checks, and optimization validation.

## Inputs To Request

- Feature or page URL
- Baseline branch or expected budget
- Critical user journey steps
- Success budget (for example LCP, TTI, or API latency)

## Workflow

1. Define scenario:
List exact navigation and interaction sequence.

2. Capture baseline:
Collect network waterfall, web vitals, and action timeline.

3. Run candidate path:
Repeat the same sequence and collect the same artifacts.

4. Compare deltas:
Report largest regressions by metric and by request.

5. Prioritize fixes:
Call out top bottleneck, likely cause, and lowest-risk optimization.

## Output Contract

- `scenario`
- `baseline`
- `candidate`
- `regressions`
- `recommended_fix`
