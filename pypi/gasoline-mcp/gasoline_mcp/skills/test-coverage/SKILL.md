---
name: test-coverage
description: Raise test coverage with targeted tests for untested branches and risk-heavy files without changing production behavior.
---

# Gasoline Test Coverage

Use this skill to systematically increase coverage while preserving behavior.

## Inputs To Request

- Coverage target
- Scope (packages/files)
- Test runtime budget

## Workflow

1. Measure current coverage:
Collect total and per-file data.

2. Rank gaps:
Prioritize low-coverage files with high risk or high churn.

3. Add focused tests:
Cover error paths, boundary checks, and branch conditions.

4. Validate stability:
Run fast suite first, then full suite if needed.

5. Summarize progress:
Report before/after per file and residual gaps.

## Output Contract

- `baseline`
- `new_tests`
- `coverage_delta`
- `remaining_gaps`
