---
title: "Generate Executable Examples"
description: "Runnable examples for every generate mode with response shapes and failure fixes."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['reference', 'examples', 'generate']
---

# Generate Executable Examples

Each section provides one runnable baseline call, expected response shape, and one failure example with a concrete fix. Use these as copy/paste starters and then adjust for your page or workflow.

## Quick Reference

```json
{
  "tool": "generate",
  "arguments": {
    "what": "reproduction"
  }
}
```

## Common Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| `what` | string | Mode name to execute. |
| `tab_id` | number | Optional target browser tab. |
| `telemetry_mode` | string | Optional telemetry verbosity: `off`, `auto`, `full`. |

## Modes

### `reproduction`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "reproduction",
    "mode": "playwright",
    "include_screenshots": true
  }
}
```

#### Expected response shape

```json
{
  "what": "reproduction",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "reproduction",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "mode": "playwright",
    "include_screenshots": true
  }
}
```

Fix: Use a valid generate mode value, e.g. `reproduction`.

### `test`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "test",
    "test_name": "checkout-smoke"
  }
}
```

#### Expected response shape

```json
{
  "what": "test",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "test",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "test_name": "checkout-smoke"
  }
}
```

Fix: Use a valid generate mode value, e.g. `test`.

### `pr_summary`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "pr_summary"
  }
}
```

#### Expected response shape

```json
{
  "what": "pr_summary",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "pr_summary",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid generate mode value, e.g. `pr_summary`.

### `har`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "har",
    "url": "/api",
    "status_min": 400
  }
}
```

#### Expected response shape

```json
{
  "what": "har",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "har",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "har",
    "url": 404,
    "status_min": 400
  }
}
```

Fix: Use a fully qualified URL string, e.g. `https://example.com`.

### `csp`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "csp",
    "include_report_uri": true
  }
}
```

#### Expected response shape

```json
{
  "what": "csp",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "csp",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "include_report_uri": true
  }
}
```

Fix: Use a valid generate mode value, e.g. `csp`.

### `sri`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "sri",
    "resource_types": [
      "script",
      "stylesheet"
    ]
  }
}
```

#### Expected response shape

```json
{
  "what": "sri",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "sri",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "resource_types": [
      "script",
      "stylesheet"
    ]
  }
}
```

Fix: Use a valid generate mode value, e.g. `sri`.

### `sarif`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "sarif",
    "scope": "#main"
  }
}
```

#### Expected response shape

```json
{
  "what": "sarif",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "sarif",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "scope": "#main"
  }
}
```

Fix: Use a valid generate mode value, e.g. `sarif`.

### `visual_test`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "visual_test",
    "test_name": "landing-visual-check"
  }
}
```

#### Expected response shape

```json
{
  "what": "visual_test",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "visual_test",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "test_name": "landing-visual-check"
  }
}
```

Fix: Use a valid generate mode value, e.g. `visual_test`.

### `annotation_report`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "annotation_report",
    "annot_session": "landing-review"
  }
}
```

#### Expected response shape

```json
{
  "what": "annotation_report",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "annotation_report",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "annot_session": "landing-review"
  }
}
```

Fix: Use a valid generate mode value, e.g. `annotation_report`.

### `annotation_issues`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "annotation_issues",
    "annot_session": "landing-review"
  }
}
```

#### Expected response shape

```json
{
  "what": "annotation_issues",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "annotation_issues",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "annot_session": "landing-review"
  }
}
```

Fix: Use a valid generate mode value, e.g. `annotation_issues`.

### `test_from_context`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "test_from_context",
    "context": "Checkout button click does nothing"
  }
}
```

#### Expected response shape

```json
{
  "what": "test_from_context",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "test_from_context",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "context": "Checkout button click does nothing"
  }
}
```

Fix: Use a valid generate mode value, e.g. `test_from_context`.

### `test_heal`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "test_heal",
    "action": "analyze",
    "test_file": "tests/e2e/checkout.spec.ts"
  }
}
```

#### Expected response shape

```json
{
  "what": "test_heal",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "test_heal",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "action": "analyze",
    "test_file": "tests/e2e/checkout.spec.ts"
  }
}
```

Fix: Use a valid generate mode value, e.g. `test_heal`.

### `test_classify`

#### Minimal call

```json
{
  "tool": "generate",
  "arguments": {
    "what": "test_classify",
    "action": "failure",
    "failure": {
      "test_name": "checkout should submit",
      "error": "Timeout 30000ms exceeded while waiting for selector text=Confirm"
    }
  }
}
```

#### Expected response shape

```json
{
  "what": "test_classify",
  "content": [
    {
      "type": "text",
      "text": "Generated artifact summary"
    }
  ],
  "artifact": {
    "format": "test_classify",
    "path": ".kaboom/reports/sample.out"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "generate",
  "arguments": {
    "what": "not_a_real_mode",
    "action": "failure",
    "failure": {
      "test_name": "checkout should submit",
      "error": "Timeout 30000ms exceeded while waiting for selector text=Confirm"
    }
  }
}
```

Fix: Use a valid generate mode value, e.g. `test_classify`.
