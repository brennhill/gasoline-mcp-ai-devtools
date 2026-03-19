---
name: automate
description: Use when user asks to automate browser actions, click buttons, fill forms, navigate pages, run a sequence of browser steps, or build a repeatable workflow.
---

# Gasoline Automate

Use this skill for reliable task execution in the browser where flakiness must be minimized.

## Inputs To Request

- Target workflow
- Start URL
- Credentials or auth preconditions
- Success condition

## Workflow

1. Validate preconditions:
Confirm tracked tab, auth state, and feature flags.

2. Plan robust selectors:
Prefer semantic selectors and stable attributes over brittle CSS chains.

3. Execute in small steps:
After each interact call, inspect command_result and page state.

4. Apply bounded recovery:
Use retry once with alternate selector or wait strategy.

5. Confirm success:
Return explicit pass/fail with final observed state.

## Output Contract

- `plan`
- `execution_log`
- `retries`
- `final_status`
