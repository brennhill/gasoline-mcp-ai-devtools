---
title: How Gasoline MCP Captures WebSocket Messages
date: 2026-02-10
author: Brenn Hill
tags: [WebSocket, MCP, AI, DevTools, Technical]
description: A deep dive into how Gasoline MCP captures full WebSocket lifecycle events and makes them available to AI coding agents.
---

# How Gasoline MCP Captures WebSocket Messages

WebSocket communication is the backbone of modern real-time applications—chat apps, live dashboards, collaborative tools, and more. But when something goes wrong, debugging WebSocket issues can be incredibly frustrating, especially when you're working with AI-generated code.

In this post, I'll explain how Gasoline MCP captures the complete WebSocket lifecycle and makes it available to your AI coding assistant.

## The Challenge: WebSocket Debugging is Hard

Traditional browser dev tools show WebSocket messages, but they have limitations:

1. **No programmatic access** - You can't query WebSocket history from code
2. **Limited context** - Messages are isolated from console logs and network errors
3. **No AI integration** - Your AI assistant can't "see" what's happening
4. **Session-bound** - Close the dev tools panel, and you lose the history

When you're debugging AI-generated code, these limitations compound. The AI can't see the WebSocket traffic, so it can't diagnose issues or suggest fixes.

## Gasoline's Solution: Complete WebSocket Observability

Gasoline MCP captures the entire WebSocket lifecycle through a Chrome extension:

### Connection Events

```typescript
{
  "type": "websocket:connected",
  "url": "wss://api.example.com/socket",
  "timestamp": 1707523200000,
  "tabId": 12345
}
```

### Message Events

```typescript
{
  "type": "websocket:message",
  "direction": "sent|received",
  "payload": { /* JSON data */ },
  "timestamp": 1707523201000,
  "tabId": 12345
}
```

### Error Events

```typescript
{
  "type": "websocket:error",
  "error": "Connection closed unexpectedly",
  "code": 1006,
  "timestamp": 1707523205000,
  "tabId": 12345
}
```

### Disconnection Events

```typescript
{
  "type": "websocket:disconnected",
  "url": "wss://api.example.com/socket",
  "timestamp": 1707523210000,
  "tabId": 12345
}
```

## Under the Hood: How It Works

### 1. Extension Content Script

The Chrome extension injects a content script that patches the native WebSocket API:

```javascript
const originalWebSocket = window.WebSocket;

window.WebSocket = function(...args) {
  const ws = new originalWebSocket(...args);
  
  ws.addEventListener('open', (event) => {
    // Capture connection event
    captureWebSocketEvent('connected', ws, event);
  });
  
  ws.addEventListener('message', (event) => {
    // Capture incoming message
    captureWebSocketEvent('message', ws, {
      direction: 'received',
      payload: event.data
    });
  });
  
  ws.addEventListener('error', (event) => {
    // Capture error event
    captureWebSocketEvent('error', ws, event);
  });
  
  ws.addEventListener('close', (event) => {
    // Capture disconnection event
    captureWebSocketEvent('disconnected', ws, event);
  });
  
  // Capture outgoing messages
  const originalSend = ws.send.bind(ws);
  ws.send = function(data) {
    captureWebSocketEvent('message', ws, {
      direction: 'sent',
      payload: data
    });
    return originalSend(data);
  };
  
  return ws;
};
```

### 2. Background Script Processing

The background script receives WebSocket events from content scripts and adds context:

- Correlates with console logs from the same tab
- Links to network requests (if WebSocket was established via HTTP upgrade)
- Captures DOM snapshots at key moments
- Tracks user actions that triggered WebSocket messages

### 3. MCP Server Integration

The MCP server exposes WebSocket data through several tools:

#### List WebSocket Connections

```json
{
  "name": "list_websockets",
  "description": "List all active WebSocket connections"
}
```

#### Get WebSocket Messages

```json
{
  "name": "get_websocket_messages",
  "description": "Get messages for a specific WebSocket connection",
  "parameters": {
    "connectionId": "string",
    "limit": "number",
    "filter": "sent|received|error"
  }
}
```

#### Search WebSocket Messages

```json
{
  "name": "search_websockets",
  "description": "Search WebSocket messages by content",
  "parameters": {
    "query": "string",
    "connectionId": "string (optional)"
  }
}
```

## AI Integration: Making WebSocket Data Actionable

When your AI assistant has access to WebSocket data, it can:

### 1. Diagnose Connection Issues

```
User: "My chat app isn't receiving messages"

AI (via Gasoline MCP):
- Checks WebSocket connections
- Sees connection was established
- Finds error: "Message payload exceeds size limit"
- Suggests: "The server is rejecting messages over 1MB. Try compressing the payload."
```

### 2. Debug Protocol Errors

```
User: "The real-time dashboard updates are inconsistent"

AI (via Gasoline MCP):
- Examines WebSocket message sequence
- Detects out-of-order messages
- Correlates with console warnings
- Identifies: "Messages are being processed before the previous update completes"
```

### 3. Analyze Performance

```
User: "WebSocket performance is degrading over time"

AI (via Gasoline MCP):
- Measures message latency over time
- Correlates with memory usage
- Finds: "Message queue is growing—consider implementing backpressure"
```

## Real-World Example

Here's how Gasoline MCP helped debug a WebSocket issue in a collaborative editing app:

**The Problem:** Users reported that changes weren't syncing properly.

**The Investigation:**
1. Gasoline captured all WebSocket messages
2. AI analyzed the message sequence
3. Found that "operation" messages were being sent before "lock" messages
4. Correlated with a race condition in the code

**The Fix:**
```javascript
// Before (buggy)
socket.send({ type: 'operation', data: change });
await acquireLock();

// After (fixed)
await acquireLock();
socket.send({ type: 'operation', data: change });
```

## Advanced Features

### Message Filtering

Gasoline lets you filter WebSocket messages by:
- Direction (sent/received)
- Time range
- Content patterns
- Connection ID

### Payload Inspection

JSON payloads are automatically parsed and pretty-printed. Binary payloads are shown with hex dumps.

### Correlation with Other Events

WebSocket events are automatically correlated with:
- Console logs
- Network requests
- User actions
- DOM changes

## Security Considerations

Gasoline takes WebSocket security seriously:

1. **Local only** - All data stays on your machine
2. **No cloud** - Messages are never sent to external servers
3. **Auth stripping** - Authorization headers are automatically removed from captured data
4. **Sensitive data masking** - You can configure patterns to mask sensitive data

## Getting Started

To start capturing WebSocket messages with Gasoline MCP:

```bash
# Install the MCP server
npx gasoline-mcp@6.0.0

# Add to your MCP config
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp"]
    }
  }
}
```

Then download the Chrome extension and start debugging!

## What's Next?

In future posts, I'll cover:
- How Gasoline captures network request/response bodies
- Auto-generating Playwright tests from WebSocket sessions
- Building custom WebSocket analyzers

Have questions or ideas? Join our Discord community or open an issue on GitHub.

---

**Related Posts:**
- [Browser Observability for AI Coding Agents](/blog/browser-observability-for-ai)
- [How Gasoline Captures Network Bodies](/blog/network-bodies-capture)
- [Auto-Generating Playwright Tests from Sessions](/blog/playwright-test-generation)
