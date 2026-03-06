---
title: "Analyze Executable Examples"
description: "Runnable examples for every analyze mode with response shapes and failure fixes."
last_verified_version: "0.7.12"
last_verified_date: 2026-03-05
normalized_tags: ['reference', 'examples', 'analyze']
---

# Analyze Executable Examples

Each section provides one runnable baseline call, expected response shape, and one failure example with a concrete fix. Use these as copy/paste starters and then adjust for your page or workflow.

## Quick Reference

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "dom"
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

### `dom`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "dom",
    "selector": ".error-banner"
  }
}
```

#### Expected response shape

```json
{
  "what": "dom",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "dom",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `performance`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "performance"
  }
}
```

#### Expected response shape

```json
{
  "what": "performance",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `performance`.

### `accessibility`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "accessibility",
    "scope": "#main",
    "tags": [
      "wcag2a",
      "wcag2aa"
    ]
  }
}
```

#### Expected response shape

```json
{
  "what": "accessibility",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "scope": "#main",
    "tags": [
      "wcag2a",
      "wcag2aa"
    ]
  }
}
```

Fix: Use a valid analyze mode value, e.g. `accessibility`.

### `error_clusters`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "error_clusters"
  }
}
```

#### Expected response shape

```json
{
  "what": "error_clusters",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `error_clusters`.

### `navigation_patterns`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "navigation_patterns"
  }
}
```

#### Expected response shape

```json
{
  "what": "navigation_patterns",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `navigation_patterns`.

### `security_audit`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "security_audit",
    "checks": [
      "credentials",
      "pii"
    ]
  }
}
```

#### Expected response shape

```json
{
  "what": "security_audit",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "checks": [
      "credentials",
      "pii"
    ]
  }
}
```

Fix: Use a valid analyze mode value, e.g. `security_audit`.

### `third_party_audit`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "third_party_audit"
  }
}
```

#### Expected response shape

```json
{
  "what": "third_party_audit",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `third_party_audit`.

### `link_health`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "link_health",
    "domain": "example.com"
  }
}
```

#### Expected response shape

```json
{
  "what": "link_health",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "domain": "example.com"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `link_health`.

### `link_validation`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "link_validation",
    "urls": [
      "https://example.com",
      "https://example.com/docs"
    ]
  }
}
```

#### Expected response shape

```json
{
  "what": "link_validation",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "urls": [
      "https://example.com",
      "https://example.com/docs"
    ]
  }
}
```

Fix: Use a valid analyze mode value, e.g. `link_validation`.

### `page_summary`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "page_summary",
    "timeout_ms": 10000
  }
}
```

#### Expected response shape

```json
{
  "what": "page_summary",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "page_summary",
    "timeout_ms": "10000"
  }
}
```

Fix: Use `timeout_ms` as a number of milliseconds.

### `annotations`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "annotations",
    "wait": true,
    "timeout_ms": 60000
  }
}
```

#### Expected response shape

```json
{
  "what": "annotations",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "annotations",
    "wait": true,
    "timeout_ms": "10000"
  }
}
```

Fix: Use `timeout_ms` as a number of milliseconds.

### `annotation_detail`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "annotation_detail",
    "correlation_id": "ann_123"
  }
}
```

#### Expected response shape

```json
{
  "what": "annotation_detail",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "correlation_id": "ann_123"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `annotation_detail`.

### `api_validation`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "api_validation",
    "operation": "analyze"
  }
}
```

#### Expected response shape

```json
{
  "what": "api_validation",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "operation": "analyze"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `api_validation`.

### `draw_history`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "draw_history"
  }
}
```

#### Expected response shape

```json
{
  "what": "draw_history",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `draw_history`.

### `draw_session`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "draw_session",
    "file": "draw-session-2026-03-05.json"
  }
}
```

#### Expected response shape

```json
{
  "what": "draw_session",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "file": "draw-session-2026-03-05.json"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `draw_session`.

### `computed_styles`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "computed_styles",
    "selector": "button[type=\"submit\"]"
  }
}
```

#### Expected response shape

```json
{
  "what": "computed_styles",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "computed_styles",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `forms`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "forms"
  }
}
```

#### Expected response shape

```json
{
  "what": "forms",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `forms`.

### `form_state`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "form_state",
    "selector": "form#checkout"
  }
}
```

#### Expected response shape

```json
{
  "what": "form_state",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "form_state",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `form_validation`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "form_validation"
  }
}
```

#### Expected response shape

```json
{
  "what": "form_validation",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `form_validation`.

### `data_table`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "data_table",
    "selector": "table"
  }
}
```

#### Expected response shape

```json
{
  "what": "data_table",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "data_table",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `visual_baseline`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "visual_baseline",
    "name": "home-baseline"
  }
}
```

#### Expected response shape

```json
{
  "what": "visual_baseline",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "name": "home-baseline"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `visual_baseline`.

### `visual_diff`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "visual_diff",
    "baseline": "home-baseline",
    "name": "home-current"
  }
}
```

#### Expected response shape

```json
{
  "what": "visual_diff",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "baseline": "home-baseline",
    "name": "home-current"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `visual_diff`.

### `visual_baselines`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "visual_baselines"
  }
}
```

#### Expected response shape

```json
{
  "what": "visual_baselines",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `visual_baselines`.

### `navigation`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "navigation"
  }
}
```

#### Expected response shape

```json
{
  "what": "navigation",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `navigation`.

### `page_structure`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "page_structure"
  }
}
```

#### Expected response shape

```json
{
  "what": "page_structure",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `page_structure`.

### `audit`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "audit",
    "categories": [
      "performance",
      "accessibility"
    ]
  }
}
```

#### Expected response shape

```json
{
  "what": "audit",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode",
    "categories": [
      "performance",
      "accessibility"
    ]
  }
}
```

Fix: Use a valid analyze mode value, e.g. `audit`.

### `feature_gates`

#### Minimal call

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "feature_gates"
  }
}
```

#### Expected response shape

```json
{
  "what": "feature_gates",
  "status": "completed",
  "result": {
    "summary": "Analysis completed",
    "findings": []
  }
}
```

#### Failure example and fix

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid analyze mode value, e.g. `feature_gates`.
