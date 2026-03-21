---
title: What.gasoline Captures
description: .gasoline captures console logs, network errors, exceptions, WebSocket events, network bodies, user actions, Web Vitals, and generates Playwright tests, PR summaries, and accessibility reports."
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['features']
---

STRUM is an open-source MCP server that passively observes your browser, analyzes performance, and generates code — everything an AI coding assistant needs to diagnose and fix issues autonomously. Zero dependencies. Localhost only.

## Console Logs

All `console` API calls captured with full argument serialization:

| Method | Level |
|--------|-------|
| `console.error()` | error |
| `console.warn()` | warn |
| `console.log()` | info |
| `console.info()` | info |
| `console.debug()` | debug |

Objects, arrays, and Error instances are fully serialized — not just `[Object object]`.

## Network Errors

Failed API calls (4xx and 5xx) captured with:

- HTTP method and URL
- Response status code
- Response body (the actual error payload)
- Request duration (ms)

Your AI sees _why_ the API call failed, not just _that_ it failed.

## Exceptions

Uncaught errors and unhandled promise rejections with:

- Error message
- Full stack trace
- Source file, line, and column
- Source map resolution (minified → original)

## WebSocket Events

[Full details →](/reference/observe/#websocket_events)

- Connection lifecycle (open, close, error)
- Message payloads (sent and received)
- Adaptive sampling for high-frequency streams
- Per-connection rates and schemas

## Network Bodies

[Full details →](/reference/observe/#network_bodies)

- Request payloads (POST/PUT/PATCH)
- Response payloads for debugging
- On-demand — doesn't record everything

## User Actions

Full interaction recording with 6 action types:

| Action | What's Captured |
|--------|----------------|
| **click** | Element selector, URL, timestamp |
| **input** | Field selector, value (passwords redacted) |
| **keypress** | Key name, target element |
| **navigate** | Destination URL |
| **select** | Selected option, target element |
| **scroll** | Scroll position (throttled) |

Smart selector priority: `data-testid` > `role` > `aria-label` > text > `id` > CSS path

## Screenshots

Auto-captured on error as JPEG:

- Rate limited: 5s between captures, 10/session max
- JPEG quality 80%
- Triggered by exceptions and console errors

## AI Context Enrichment

Errors enriched with framework-aware context:

- Component ancestry (React, Vue, Svelte)
- Relevant app state snapshots
- Custom annotations via `window.__gasoline.annotate()`

## Web Vitals

Core Web Vitals captured and assessed against Google thresholds:

| Metric | Good | Poor |
|--------|------|------|
| **FCP** (First Contentful Paint) | < 1.8s | ≥ 3.0s |
| **LCP** (Largest Contentful Paint) | < 2.5s | ≥ 4.0s |
| **CLS** (Cumulative Layout Shift) | < 0.1 | ≥ 0.25 |
| **INP** (Interaction to Next Paint) | < 200ms | ≥ 500ms |

Plus: Load Time, TTFB, DomContentLoaded, DomInteractive, request count, transfer size, long tasks, and total blocking time.

## Performance Regression Detection

Automatic baseline computation and regression alerting:

- Baselines computed from rolling averages (weighted 80/20 after 5 samples)
- Regression thresholds: >50% load time, >50% LCP, >100% transfer size
- **Causal diffing** — identifies which resource changes caused the regression
- Actionable recommendations generated automatically
- Push-based alerts when regressions are detected

## API Schema Inference

Auto-discovers your API structure from captured network traffic:

- Groups requests by endpoint pattern (normalizes dynamic segments)
- Infers request/response shapes from observed payloads
- Exports as OpenAPI stub or compact gasoline format
- Configurable minimum observation count before including an endpoint

## Session Checkpoints

Save and compare browser state over time:

- **Named checkpoints** — save state at meaningful moments (up to 20)
- **Auto-checkpoint** — implicit baseline for continuous monitoring
- **Timestamp queries** — check state at any ISO 8601 time
- **Compressed diffs** — only changed data returned (console, network, WebSocket, actions)
- Deduplication and fingerprinting for efficient comparisons

## Noise Filtering

Keep your AI focused on real issues:

- **Auto-detect** — identifies repetitive patterns and suggests dismissal
- **Rule-based** — add regex patterns for known noise (extensions, analytics, etc.)
- **One-off dismiss** — quickly silence a specific pattern
- Categories: console, network, WebSocket
- Pre-built rules filter common browser extension noise

## Reproduction Scripts

User actions → runnable Playwright scripts:

- Multi-strategy selectors (data-testid > aria > role > CSS)
- Click, input, scroll, keyboard, select, navigate events
- Base URL rewriting for environment portability
- Error context embedding
- Configurable last-N actions scope

## Test Generation

Full Playwright test scripts with configurable assertions:

- **Network assertions** — wait for responses, validate status codes
- **Response shape assertions** — verify JSON property structure
- **Console error detection** — assert no unexpected errors
- Custom test names and base URL substitution

## PR Summaries

Performance impact reports for pull requests:

- Before/after comparison table (Load Time, FCP, LCP, CLS, Bundle Size)
- Delta and percentage change calculations
- Error tracking (fixed vs new errors)
- Compact one-liner format for git hook annotations

## HAR Export

Standard HTTP Archive format export:

- Filter by URL substring, HTTP method, status code range
- Compatible with Chrome DevTools, Charles Proxy, and other HAR viewers
- Includes request/response headers and bodies

## Accessibility Audits

WCAG compliance checking with standard output:

- axe-core engine with configurable rule tags
- Scope to specific page sections via CSS selectors
- Cached results with force-refresh option
- **SARIF export** — standard Static Analysis Results Interchange Format
- File save or inline JSON return

## Persistent Session Memory

Key-value store that persists across tool calls:

- Namespaced storage (save, load, list, delete)
- Session context loading from previous runs
- Stats and usage reporting

## Extension Controls

The popup lets you dial the heat:

| Setting | Options |
|---------|---------|
| **Capture level** | Errors Only · Warnings+ · All Logs |
| **WebSocket** | Lifecycle only · Include messages |
| **Network waterfall** | On / Off |
| **Performance marks** | On / Off |
| **User actions** | On / Off |
| **Screenshot on error** | On / Off |
| **Source maps** | On / Off |
| **Domain filters** | Allowlist specific sites |
