---
title: "analyze() — Active Analysis"
description: "Complete reference for the analyze tool. 15 modes for DOM queries, accessibility audits, security scans, link health, visual annotations, performance snapshots, and more."
---

The `analyze` tool triggers active analysis — DOM queries, accessibility audits, security scans, link health checks, and visual annotations. Unlike `observe` (which reads passive buffers), `analyze` dispatches work to the browser extension and returns results.

:::note[Synchronous Mode]
Tools now block until the extension returns a result (up to 15s). Set `background: true` to return immediately with a `correlation_id`, then poll with `observe({what: "command_result", correlation_id: "..."})`.
:::

## Quick Reference

```js
analyze({what: "dom", selector: ".error-banner"})                    // Query live DOM
analyze({what: "accessibility", scope: "#main", tags: ["wcag2a"]})  // WCAG audit
analyze({what: "security_audit", checks: ["credentials", "pii"]})   // Security scan
analyze({what: "link_health", domain: "example.com"})               // Check all links
analyze({what: "performance"})                                       // Performance snapshot
analyze({what: "error_clusters"})                                    // Group similar errors
analyze({what: "page_summary"})                                      // Page structure
analyze({what: "annotations", session: "review"})                   // Draw mode results
```

## Common Parameters

These parameters work across multiple modes:

| Parameter | Type | Description |
|-----------|------|-------------|
| `what` | string (required) | Which mode to use (see sections below) |
| `sync` | boolean | Wait for result (default: true) |
| `background` | boolean | Return immediately with a correlation_id |
| `telemetry_mode` | string | Telemetry metadata: `off`, `auto`, or `full` |
| `tab_id` | number | Target a specific tab (omit for active tab) |

---

## DOM Queries

### `dom`

Query the live DOM using CSS selectors. Returns element details: tag, attributes, text content, visibility, and children.

```js
analyze({what: "dom", selector: ".error-banner"})
analyze({what: "dom", selector: "nav a", tab_id: 123})
analyze({what: "dom", selector: "[role='alert']", frame: "#app-iframe"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS selector to query |
| `frame` | string/number | Target iframe: CSS selector, 0-based index, or `"all"` |
| `tab_id` | number | Target specific tab (omit for active tab) |

---

## Performance

### `performance`

Performance snapshots with before/after comparison and regression detection.

```js
analyze({what: "performance"})
```

---

## Accessibility

### `accessibility`

WCAG accessibility audit using axe-core. Returns violations, passes, and incomplete checks.

```js
analyze({what: "accessibility"})
analyze({what: "accessibility", scope: "#main-content", tags: ["wcag2a"]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `scope` | string | CSS selector to limit the audit scope |
| `tags` | array | WCAG tags to test (e.g., `wcag2a`, `wcag2aa`, `best-practice`) |
| `force_refresh` | boolean | Bypass cached results |
| `frame` | string/number | Target iframe |

---

## Error Analysis

### `error_clusters`

Groups similar errors together by message pattern. Useful for identifying the most common error categories.

```js
analyze({what: "error_clusters"})
```

---

## Navigation

### `history`

Recent navigation history for the current tab.

```js
analyze({what: "history"})
```

---

## Security

### `security_audit`

Scans captured data for security issues: leaked credentials, PII exposure, insecure headers, cookie misconfiguration, transport security, and auth problems.

```js
analyze({what: "security_audit"})
analyze({what: "security_audit", checks: ["credentials", "pii"], severity_min: "high"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `checks` | array | Which checks to run: `credentials`, `pii`, `headers`, `cookies`, `transport`, `auth` |
| `severity_min` | string | Minimum severity: `critical`, `high`, `medium`, `low`, `info` |

### `third_party_audit`

Analyzes third-party scripts and external dependencies loaded by the page.

```js
analyze({what: "third_party_audit"})
analyze({what: "third_party_audit", first_party_origins: ["https://myapp.com"]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `first_party_origins` | array | Origins to consider first-party |
| `include_static` | boolean | Include origins that only serve static assets |
| `custom_lists` | object | Custom allowed/blocked/internal domain lists |

---

## Link Health

### `link_health`

Browser-based link checker. Navigates links in the extension to detect broken links, CORS issues, and redirect chains. Runs concurrently with configurable worker count.

```js
analyze({what: "link_health", domain: "example.com"})
analyze({what: "link_health", domain: "example.com", max_workers: 5, timeout_ms: 10000})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `domain` | string | Domain to check links for |
| `max_workers` | number | Max concurrent workers |
| `timeout_ms` | number | Timeout per link check |

### `link_validation`

Server-side URL validation with SSRF-safe transport. Validates specific URLs from the MCP server without needing the browser extension.

```js
analyze({what: "link_validation", urls: ["https://example.com/page1", "https://example.com/page2"]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `urls` | array | URLs to validate |

---

## Page Analysis

### `page_summary`

Page structure summary — headings, landmarks, forms, links, and metadata.

```js
analyze({what: "page_summary"})
analyze({what: "page_summary", timeout_ms: 10000, world: "main"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `timeout_ms` | number | Timeout for page analysis |
| `world` | string | Execution world: `auto`, `main`, `isolated` |
| `tab_id` | number | Target specific tab |

### `api_validation`

Infer API schemas from captured traffic and validate consistency. Detects response shape changes, missing fields, and type mismatches.

```js
analyze({what: "api_validation", operation: "analyze"})
analyze({what: "api_validation", operation: "report"})
analyze({what: "api_validation", operation: "clear"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | `analyze` (infer schemas), `report` (show results), `clear` (reset) |
| `ignore_endpoints` | array | URL substrings to exclude from analysis |

---

## Draw Mode & Annotations

### `annotations`

Retrieve annotations from the last draw mode session. Users draw rectangles and type feedback, then press ESC. This mode returns all annotations.

```js
analyze({what: "annotations"})
analyze({what: "annotations", session: "checkout-review"})
analyze({what: "annotations", session: "checkout-review", wait: true, timeout_ms: 300000})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `session` | string | Named session for multi-page annotation review |
| `wait` | boolean | Block until user finishes drawing (default 5 min timeout) |
| `timeout_ms` | number | Max wait time when `wait: true` (max 600000ms / 10 min) |

### `annotation_detail`

Full computed styles and DOM detail for a specific annotation. Use after retrieving annotations to get detailed style information for a specific element.

```js
analyze({what: "annotation_detail", correlation_id: "ann-abc123"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `correlation_id` | string | Annotation correlation ID from `annotations` results |

### `draw_history`

List all past draw mode sessions with metadata.

```js
analyze({what: "draw_history"})
```

### `draw_session`

Get the full data for a specific draw session.

```js
analyze({what: "draw_session", file: "session-2026-02-17.json"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `file` | string | Session filename from `draw_history` results |
