---
title: "WebSocket Monitoring"
description: "Monitor WebSocket connections, messages, and lifecycle events in real time. Adaptive sampling handles high-frequency streams without browser overhead."
keywords: "WebSocket debugging, WebSocket monitoring tool, WebSocket MCP, real-time WebSocket capture, WebSocket connection tracking, adaptive sampling"
permalink: /websocket-monitoring/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Real-time WebSocket traffic, captured and queryable by your AI."
toc: true
toc_sticky: true
---

Gasoline captures WebSocket connection lifecycle and message payloads, making real-time communication debuggable by AI assistants.

## <i class="fas fa-exclamation-circle"></i> The Problem

WebSocket-heavy apps (chat, real-time dashboards, collaborative editors) are notoriously hard to debug. Messages fly by too fast to inspect manually, connections drop silently, and by the time you open DevTools the relevant data is gone.

Your AI assistant can't help if it can't see what's happening on the wire. Gasoline captures it all with zero browsing impact.

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

```json
// High-frequency stream (e.g., 200 msg/s stock ticker)
// Gasoline samples automatically:
{
  "sampling": {
    "active": true,
    "rate": 0.1,
    "reason": "high_frequency",
    "totalSeen": 2000,
    "totalCaptured": 200
  }
}
```

Your AI still sees message patterns, schemas, and any errors — without the memory cost of storing every message.

## <i class="fas fa-tools"></i> MCP Tools

### `get_websocket_events`

Get captured messages and lifecycle events with filters:

| Filter | Description |
|--------|-------------|
| <i class="fas fa-link"></i> URL | Events for a specific WebSocket URL |
| <i class="fas fa-fingerprint"></i> Connection ID | Events for a specific connection |
| <i class="fas fa-arrows-alt-h"></i> Direction | `incoming`, `outgoing`, or both |
| <i class="fas fa-list-ol"></i> Limit | Max events to return (default: 50) |

### `get_websocket_status`

Get live connection health:

```json
{
  "connections": [{
    "id": "ws_1",
    "url": "wss://api.example.com/stream",
    "state": "open",
    "duration": "2m30s",
    "messageRate": {
      "incoming": { "perSecond": 45.2, "total": 5420 },
      "outgoing": { "perSecond": 2.1, "total": 252 }
    }
  }]
}
```

## <i class="fas fa-tachometer-alt"></i> Performance Budget

| Metric | Budget |
|--------|--------|
| Handler latency per message | < 0.1ms |
| Memory buffer | 500 events max, 4MB limit |
| Sampling threshold | Activates above 1000 msg/s |
| Impact on page load | Zero (deferred to after load event) |

## <i class="fas fa-sliders-h"></i> Extension Settings

Two capture modes:

1. **Lifecycle only** — open/close/error events (minimal overhead)
2. **Include messages** — message payloads with adaptive sampling

Toggle in the extension popup.

## <i class="fas fa-fire-alt"></i> Use Cases

### Debugging Dropped Connections

> "My WebSocket keeps disconnecting."

Your AI sees the connection lifecycle, error events, and timing patterns to diagnose whether it's a server timeout, network issue, or client-side problem.

### Message Schema Issues

> "The real-time updates stopped working."

Your AI inspects recent messages to see if the server started sending a different payload format.

### Performance Debugging

> "The app is getting slow."

Message rate data reveals if you're receiving too many updates or if message processing is backing up.

### Reconnection Behavior

> "The socket reconnects but data is stale."

Your AI sees the close/reopen cycle and can identify whether the server is sending a full state refresh or just resuming the stream.
