# Generate Tool — Mode Reference

All modes require `what` (string). Universal optional params: `telemetry_mode` (off|auto|full), `save_to` (string).

---

## reproduction
Generate bug reproduction script from captured actions.
**Params:** error_message (string), last_n (number), base_url (string), include_screenshots (bool), generate_fixtures (bool), visual_assertions (bool), output_format (kaboom-agentic-browser|playwright), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"reproduction","error_message":"TypeError: undefined is not a function","last_n":10,"output_format":"playwright"}'
```

## test
Generate test script from captured data.
**Params:** test_name (string), assert_network (bool), assert_no_errors (bool), assert_response_shape (bool), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"test","test_name":"login-flow","assert_network":true,"assert_no_errors":true}'
```

## pr_summary
Generate PR summary from captured data.
**Params:** save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"pr_summary"}'
```

## har
Generate HAR file from network data.
**Params:** url (string), method (string), status_min (number), status_max (number), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"har","url":"api.example.com","method":"POST","status_min":400,"status_max":599}'
```

## csp
Generate Content Security Policy.
**Params:** mode (strict|moderate|report_only), include_report_uri (bool), exclude_origins (array of strings), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"csp","mode":"strict","include_report_uri":true,"exclude_origins":["analytics.example.com"]}'
```

## sri
Generate Subresource Integrity hashes.
**Params:** resource_types (array: script|stylesheet), origins (array of strings), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"sri","resource_types":["script","stylesheet"],"origins":["cdn.example.com"]}'
```

## sarif
Generate SARIF static analysis results.
**Params:** scope (string), include_passes (bool), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"sarif","scope":"security","include_passes":false}'
```

## visual_test
Generate visual assertion test.
**Params:** test_name (string), annot_session (string), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"visual_test","test_name":"homepage-layout","annot_session":"sess_abc123"}'
```

## annotation_report
Generate annotation report.
**Params:** annot_session (string), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"annotation_report","annot_session":"sess_abc123"}'
```

## annotation_issues
Generate annotation issues.
**Params:** annot_session (string), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"annotation_issues","annot_session":"sess_abc123"}'
```

## test_from_context
Generate test from error/interaction/regression context.
**Params:** context (required, enum: error|interaction|regression), error_id (string), include_mocks (bool), output_format (file|inline), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"test_from_context","context":"error","error_id":"err_42","include_mocks":true,"output_format":"file"}'
```

## test_heal
Analyze and repair broken test selectors.
**Params:** action (analyze|repair|batch), test_file (string, for analyze), test_dir (string, for batch), broken_selectors (array, for repair), auto_apply (bool, for repair), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"test_heal","action":"analyze","test_file":"tests/login.spec.ts"}'
```

## test_classify
Classify test failures.
**Params:** action (failure|batch), failure (object with error required + test_name, screenshot, trace, duration_ms optional), failures (array of failure objects for batch), save_to (string)
**Example:**
```bash
bash scripts/kaboom-call.sh generate '{"what":"test_classify","action":"failure","failure":{"error":"Element not found: #submit-btn","test_name":"checkout.spec.ts"}}'
```
