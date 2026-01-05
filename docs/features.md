---
title: "What Gasoline Captures"
description: "Gasoline captures console logs, network errors, exceptions, WebSocket events, network bodies, user actions, Web Vitals, and generates Playwright tests, PR summaries, and accessibility reports."
keywords: "browser log capture, console error monitoring, network error capture, exception tracking, WebSocket monitoring, Web Vitals, performance regression, API schema, accessibility audit, Playwright test generation"
permalink: /features/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Every signal your browser emits. Captured. Analyzed. Acted on."
toc: true
toc_sticky: true
---

Gasoline passively observes your browser, analyzes performance, and generates code — everything an AI needs to diagnose and fix issues autonomously.

## <i class="fas fa-terminal"></i> Console Logs

<img src="/assets/images/sparky/features/sparky-confused-dizzy-web.webp" alt="Sparky looking overwhelmed by stack traces" style="float: right; width: 160px; margin: 0 0 20px 20px; border-radius: 8px;" />

All `console` API calls captured with full argument serialization:

| Method | Level |
|--------|-------|
| `console.error()` | error |
| `console.warn()` | warn |
| `console.log()` | info |
| `console.info()` | info |
| `console.debug()` | debug |

Objects, arrays, and Error instances are fully serialized — not just `[Object object]`.

## <i class="fas fa-wifi"></i> Network Errors

<img src="/assets/images/sparky/features/sparky-fighting-bugs-web.webp" alt="Sparky fighting network bugs" style="float: left; width: 160px; margin: 0 20px 20px 0; border-radius: 8px;" />

Failed API calls (4xx and 5xx) captured with:

- HTTP method and URL
- Response status code
- Response body (the actual error payload)
- Request duration (ms)

Your AI sees _why_ the API call failed, not just _that_ it failed.

<img src="/assets/images/sparky/features/sparky-firefighter-tough-web.webp" alt="Sparky as firefighter ready for action" style="float: right; width: 160px; margin: 0 0 20px 20px; border-radius: 8px;" />

## <i class="fas fa-bomb"></i> Exceptions

Uncaught errors and unhandled promise rejections with:

- Error message
- Full stack trace
- Source file, line, and column
- Source map resolution (minified → original)

<img src="/assets/images/sparky/features/sparky-digital-surfing-web.webp" alt="Sparky riding WebSocket waves" style="float: left; width: 160px; margin: 0 20px 20px 0; border-radius: 8px;" />

## <i class="fas fa-plug"></i> WebSocket Events

[Full details →](/websocket-monitoring/)

- Connection lifecycle (open, close, error)
- Message payloads (sent and received)
- Adaptive sampling for high-frequency streams
- Per-connection rates and schemas

## <i class="fas fa-exchange-alt"></i> Network Bodies

[Full details →](/network-bodies/)

- Request payloads (POST/PUT/PATCH)
- Response payloads for debugging
- On-demand — doesn't record everything

## <i class="fas fa-mouse-pointer"></i> User Actions

Full interaction recording with 6 action types:

| Action | What's Captured |
|--------|----------------|
| <i class="fas fa-hand-pointer"></i> **click** | Element selector, URL, timestamp |
| <i class="fas fa-keyboard"></i> **input** | Field selector, value (passwords redacted) |
| <i class="fas fa-key"></i> **keypress** | Key name, target element |
| <i class="fas fa-compass"></i> **navigate** | Destination URL |
| <i class="fas fa-list-ul"></i> **select** | Selected option, target element |
| <i class="fas fa-arrows-alt-v"></i> **scroll** | Scroll position (throttled) |

Smart selector priority: `data-testid` > `role` > `aria-label` > text > `id` > CSS path

## <i class="fas fa-camera"></i> Screenshots

Auto-captured on error as JPEG:

- Rate limited: 5s between captures, 10/session max
- JPEG quality 80%
- Triggered by exceptions and console errors

## <i class="fas fa-brain"></i> AI Context Enrichment

Errors enriched with framework-aware context:

- Component ancestry (React, Vue, Svelte)
- Relevant app state snapshots
- Custom annotations via [`window.__gasoline.annotate()`](/developer-api/)

## <i class="fas fa-tachometer-alt"></i> Web Vitals

Core Web Vitals captured and assessed against Google thresholds:

| Metric | Good | Poor |
|--------|------|------|
| **FCP** (First Contentful Paint) | < 1.8s | ≥ 3.0s |
| **LCP** (Largest Contentful Paint) | < 2.5s | ≥ 4.0s |
| **CLS** (Cumulative Layout Shift) | < 0.1 | ≥ 0.25 |
| **INP** (Interaction to Next Paint) | < 200ms | ≥ 500ms |

Plus: Load Time, TTFB, DomContentLoaded, DomInteractive, request count, transfer size, long tasks, and total blocking time.

## <i class="fas fa-chart-line"></i> Performance Regression Detection

Automatic baseline computation and regression alerting:

- Baselines computed from rolling averages (weighted 80/20 after 5 samples)
- Regression thresholds: >50% load time, >50% LCP, >100% transfer size
- **Causal diffing** — identifies which resource changes caused the regression
- Actionable recommendations generated automatically
- Push-based alerts when regressions are detected

## <i class="fas fa-project-diagram"></i> API Schema Inference

Auto-discovers your API structure from captured network traffic:

- Groups requests by endpoint pattern (normalizes dynamic segments)
- Infers request/response shapes from observed payloads
- Exports as OpenAPI stub or compact gasoline format
- Configurable minimum observation count before including an endpoint

## <i class="fas fa-history"></i> Session Checkpoints

Save and compare browser state over time:

- **Named checkpoints** — save state at meaningful moments (up to 20)
- **Auto-checkpoint** — implicit baseline for continuous monitoring
- **Timestamp queries** — check state at any ISO 8601 time
- **Compressed diffs** — only changed data returned (console, network, WebSocket, actions)
- Deduplication and fingerprinting for efficient comparisons

## <i class="fas fa-filter"></i> Noise Filtering

Keep your AI focused on real issues:

- **Auto-detect** — identifies repetitive patterns and suggests dismissal
- **Rule-based** — add regex patterns for known noise (extensions, analytics, etc.)
- **One-off dismiss** — quickly silence a specific pattern
- Categories: console, network, WebSocket
- Pre-built rules filter common browser extension noise

## <i class="fas fa-play-circle"></i> Reproduction Scripts

User actions → runnable Playwright scripts:

- Multi-strategy selectors (data-testid > aria > role > CSS)
- Click, input, scroll, keyboard, select, navigate events
- Base URL rewriting for environment portability
- Error context embedding
- Configurable last-N actions scope

## <i class="fas fa-vial"></i> Test Generation

Full Playwright test scripts with configurable assertions:

- **Network assertions** — wait for responses, validate status codes
- **Response shape assertions** — verify JSON property structure
- **Console error detection** — assert no unexpected errors
- Custom test names and base URL substitution

## <i class="fas fa-file-alt"></i> PR Summaries

Performance impact reports for pull requests:

- Before/after comparison table (Load Time, FCP, LCP, CLS, Bundle Size)
- Delta and percentage change calculations
- Error tracking (fixed vs new errors)
- Compact one-liner format for git hook annotations

## <i class="fas fa-file-archive"></i> HAR Export

Standard HTTP Archive format export:

- Filter by URL substring, HTTP method, status code range
- Compatible with Chrome DevTools, Charles Proxy, and other HAR viewers
- Includes request/response headers and bodies

## <i class="fas fa-universal-access"></i> Accessibility Audits

WCAG compliance checking with standard output:

- axe-core engine with configurable rule tags
- Scope to specific page sections via CSS selectors
- Cached results with force-refresh option
- **SARIF export** — standard Static Analysis Results Interchange Format
- File save or inline JSON return

## <i class="fas fa-database"></i> Persistent Session Memory

Key-value store that persists across tool calls:

- Namespaced storage (save, load, list, delete)
- Session context loading from previous runs
- Stats and usage reporting

## <i class="fas fa-sliders-h"></i> Extension Controls

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
