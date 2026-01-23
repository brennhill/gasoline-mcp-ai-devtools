---
title: "WebSocket Monitoring"
description: "Monitor WebSocket connections, messages, and lifecycle events in real time. Adaptive sampling handles high-frequency streams without browser overhead."
keywords: "WebSocket debugging, WebSocket monitoring tool, WebSocket MCP, real-time WebSocket capture, WebSocket connection tracking"
permalink: /websocket-monitoring/
toc: true
toc_sticky: true
---

Gasoline captures WebSocket connection lifecycle and message payloads, making real-time communication debuggable by AI assistants.

## What Gets Captured

### Connection Lifecycle

- **Open** — when a WebSocket connection is established (URL, protocols)
- **Close** — when a connection closes (code, reason, clean/dirty)
- **Error** — connection errors

### Message Payloads

- **Sent messages** — data your app sends to the server
- **Received messages** — data arriving from the server
- **Direction tagging** — each message is labeled `sent` or `received`

## Adaptive Sampling

High-frequency WebSocket streams (e.g., live data feeds, game state) can produce thousands of messages per second. Gasoline uses adaptive sampling to keep overhead under 0.1ms per message:

- Low-frequency connections: all messages captured
- High-frequency streams: statistically sampled
- Connection lifecycle events are always captured (never sampled)

## MCP Tools

### `get_websocket_events`

Query captured WebSocket events with filters:

- **URL filter** — only events for a specific WebSocket URL
- **Connection ID** — events for a specific connection
- **Direction** — `sent`, `received`, or both

### `get_websocket_status`

Get current connection states:

- Active connections and their URLs
- Message rates (messages/second)
- Message schemas (inferred structure of payloads)
- Connection duration

## Extension Settings

WebSocket capture has two modes:

1. **Lifecycle only** — captures open/close/error events (minimal overhead)
2. **Include messages** — captures message payloads with adaptive sampling

Toggle in the extension popup under "WebSocket Monitoring."

## Use Cases

- Debug real-time features (chat, notifications, live updates)
- Monitor WebSocket reconnection behavior
- Inspect message formats between client and server
- Identify connection instability (frequent close/reopen cycles)
