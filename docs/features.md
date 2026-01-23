---
title: "What Gasoline Captures"
description: "Gasoline captures console logs, network errors, exceptions, WebSocket events, network request/response bodies, user actions, and screenshots from your browser."
keywords: "browser log capture, console error monitoring, network error capture, exception tracking, WebSocket monitoring, screenshot on error"
permalink: /features/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Every signal your browser emits. Captured. Organized. Fed to your AI."
toc: true
toc_sticky: true
---

Gasoline passively observes your browser and collects everything an AI needs to diagnose issues.

## <i class="fas fa-terminal"></i> Console Logs

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

Failed API calls (4xx and 5xx) captured with:

- HTTP method and URL
- Response status code
- Response body (the actual error payload)
- Request duration (ms)

Your AI sees _why_ the API call failed, not just _that_ it failed.

## <i class="fas fa-bomb"></i> Exceptions

Uncaught errors and unhandled promise rejections with:

- Error message
- Full stack trace
- Source file, line, and column
- Source map resolution (minified → original)

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

Recent interactions buffered and attached to errors:

- <i class="fas fa-hand-pointer"></i> Click events with element selectors
- <i class="fas fa-keyboard"></i> Input events (values redacted by default)
- <i class="fas fa-arrows-alt-v"></i> Scroll events (throttled)
- Multi-strategy selectors (data-testid > aria > role > CSS path)

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

## <i class="fas fa-play-circle"></i> Reproduction Scripts

User actions → runnable Playwright tests:

- Multi-strategy selectors (data-testid > aria > role > CSS)
- Click, input, scroll, keyboard, select events
- Generated via [`window.__gasoline.generateScript()`](/developer-api/)

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
