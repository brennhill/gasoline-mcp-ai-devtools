---
name: api-validation
description: Validate API behavior against expected contracts using captured requests, responses, and schema checks.
---

# Gasoline API Validation

Use this skill when endpoint behavior, payload shape, or status handling is in question.

## Inputs To Request

- Operation(s) or endpoint(s)
- Expected status codes and response shape
- Auth/session expectations

## Workflow

1. Define checks:
List each operation with expected request and response constraints.

2. Collect traffic:
Use `observe(network_waterfall)` and `observe(network_bodies)` for matching requests.

3. Validate contracts:
Run `analyze(api_validation)` for each operation and capture mismatches.

4. Check auth and headers:
Confirm required cookies/tokens/headers and error-path behavior.

5. Classify mismatch type:
Schema drift, status drift, auth failure, routing mismatch, or serialization issue.

6. Recommend minimum corrective change:
Prefer server/client contract alignment with explicit tests.

## Output Contract

- `operations_checked`
- `mismatches`
- `risk_level`
- `recommended_fix`
- `follow_up_tests`
