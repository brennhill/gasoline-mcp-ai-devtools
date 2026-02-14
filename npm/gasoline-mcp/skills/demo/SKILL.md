---
name: demo
description: Build a clean, repeatable product demo script that uses Gasoline to show feature behavior and debug visibility in real time.
---

# Gasoline Demo

Use this skill when preparing customer demos, release demos, or internal walkthroughs.

## Inputs To Request

- Audience type
- Demo environment URL
- Features to highlight
- Max demo length

## Workflow

1. Prepare stable environment:
Confirm extension connected and tracked tab healthy.

2. Build demo script:
Create an ordered step list with expected visible outcomes.

3. Execute with narration:
Use subtitle/action overlays for clarity, one feature per segment.

4. Capture proof artifacts:
Store screenshots, key logs, and command results for each segment.

5. Package final runbook:
Include setup, script, fallback path, and known failure recovery.

## Output Contract

- `demo_goal`
- `step_script`
- `artifacts`
- `fallbacks`
