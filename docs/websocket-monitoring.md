---
title: "WebSocket Monitoring"
description: "Monitor WebSocket connections, messages, and lifecycle events in real time. Adaptive sampling handles high-frequency streams without browser overhead."
keywords: "WebSocket debugging, WebSocket monitoring tool, WebSocket MCP, real-time WebSocket capture, WebSocket connection tracking"
permalink: /websocket-monitoring/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Real-time WebSocket traffic, captured and queryable by your AI."
toc: true
toc_sticky: true
---

Gasoline captures WebSocket connection lifecycle and message payloads, making real-time communication debuggable by AI assistants.

## <i class="fas fa-plug"></i> What Gets Captured

### Connection Lifecycle

- <i class="fas fa-sign-in-alt"></i> **Open** — connection established (URL, protocols)
- <i class="fas fa-sign-out-alt"></i> **Close** — connection closed (code, reason, clean/dirty)
- <i class="fas fa-exclamation-triangle"></i> **Error** — connection errors

### Message Payloads

- <i class="fas fa-arrow-up"></i> **Sent messages** — data your app sends
- <i class="fas fa-arrow-down"></i> **Received messages** — data arriving from the server
- <i class="fas fa-tag"></i> **Direction tagging** — each message labeled `sent` or `received`

## <i class="fas fa-chart-line"></i> Adaptive Sampling

High-frequency streams (live data feeds, game state) can produce thousands of messages/second. Gasoline uses adaptive sampling to keep overhead under 0.1ms per message:

- Low-frequency connections: all messages captured
- High-frequency streams: statistically sampled
- Lifecycle events: always captured (never sampled)

## <i class="fas fa-tools"></i> MCP Tools

### `get_websocket_events`

| Filter | Description |
|--------|-------------|
| <i class="fas fa-link"></i> URL | Events for a specific WebSocket URL |
| <i class="fas fa-fingerprint"></i> Connection ID | Events for a specific connection |
| <i class="fas fa-arrows-alt-h"></i> Direction | `sent`, `received`, or both |

### `get_websocket_status`

- Active connections and URLs
- Message rates (messages/second)
- Message schemas (inferred payload structure)
- Connection duration

## <i class="fas fa-sliders-h"></i> Extension Settings

Two capture modes:

1. **Lifecycle only** — open/close/error events (minimal overhead)
2. **Include messages** — message payloads with adaptive sampling

Toggle in the extension popup.

## <i class="fas fa-fire-alt"></i> Use Cases

- Debug real-time features (chat, notifications, live updates)
- Monitor reconnection behavior
- Inspect message formats between client and server
- Identify connection instability (frequent close/reopen cycles)
