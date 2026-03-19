---
name: regression-test
description: Use when user has fixed a bug and needs a test that proves it stays fixed, or when a failure needs to be captured as a reproducible test case.
---

# Gasoline Regression Test

Use this skill after a bug is confirmed and should never reappear.

## Inputs To Request

- Failure scenario and expected outcome
- Test layer preference (unit, integration, e2e)
- Existing test file or suite location

## Workflow

1. Reproduce failure deterministically:
Capture exact trigger and expected assertion.

2. Pick narrowest test layer:
Prefer unit/integration before full e2e unless browser behavior is required.

3. Build stable fixtures:
Use fixed inputs, bounded timing, and mocked unstable dependencies.

4. Write fail-first assertion:
Ensure test fails on pre-fix behavior and targets the real defect.

5. Add guard assertion:
Assert adjacent behavior to prevent overfitting.

6. Verify repeatedly:
Run targeted tests multiple times to catch flakiness.

## Output Contract

- `test_location`
- `failure_assertion`
- `regression_guard`
- `stability_controls`
- `verification_results`
