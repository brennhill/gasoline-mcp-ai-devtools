---
title: "What Gasoline Captures"
description: "Gasoline captures console logs, network errors, exceptions, WebSocket events, network request/response bodies, user actions, and screenshots from your browser."
keywords: "browser log capture, console error monitoring, network error capture, exception tracking, WebSocket monitoring, screenshot on error"
permalink: /features/
toc: true
toc_sticky: true
---

Gasoline passively observes your browser and captures everything an AI coding assistant needs to debug issues.

## Console Logs

All `console` API calls are captured with full arguments:

- `console.log()` — general output
- `console.warn()` — warnings
- `console.error()` — errors
- `console.info()` — informational
- `console.debug()` — debug output

Arguments are serialized including objects, arrays, and Error instances.

## Network Errors

Failed API calls (4xx and 5xx responses) are captured with:

- HTTP method and URL
- Response status code
- Response body (for debugging API errors)
- Request duration

## Exceptions

Uncaught errors and unhandled promise rejections are captured with:

- Error message
- Full stack trace
- Source file, line, and column
- Source map resolution (when enabled)

## WebSocket Events

[Full details →](/websocket-monitoring/)

- Connection lifecycle (open, close, error)
- Message payloads (sent and received)
- Adaptive sampling for high-frequency streams
- Per-connection message rates and schemas

## Network Bodies

[Full details →](/network-bodies/)

- Request payloads (POST/PUT/PATCH bodies)
- Response payloads for API debugging
- On-demand capture (doesn't record everything)

## User Actions

Recent user interactions are buffered and attached to errors:

- Click events with target element selectors
- Input events (values redacted by default)
- Scroll events (throttled)
- Keyboard events
- Multi-strategy selectors (data-testid, aria, role, CSS path)

## Screenshots

Auto-captured on error, saved as JPEG files on disk:

- Rate limited (5s between captures, 10 per session max)
- Configurable quality (JPEG 80%)
- Triggered by exceptions and console errors

## AI Context Enrichment

Errors can be enriched with framework-aware context:

- Component ancestry (React, Vue, Svelte)
- Relevant app state snapshots
- Custom annotations via `window.__gasoline.annotate()`

## Reproduction Scripts

Captured user actions can be converted to runnable Playwright tests:

- Multi-strategy selectors (data-testid > aria > role > CSS path)
- Click, input, scroll, keyboard, and select events
- Generated via `window.__gasoline.generateScript()`

## Extension Settings

The extension popup lets you control what gets captured:

- **Capture level** — Errors Only, Warnings+, or All Logs
- **WebSocket monitoring** — lifecycle only or include messages
- **Network waterfall** — timing data for all requests
- **Performance marks** — `performance.mark()` and `measure()`
- **User actions** — click/input/scroll buffer
- **Screenshot on error** — auto-capture on exceptions
- **Source maps** — resolve minified stack traces
- **Domain filters** — only capture from specific sites
