# Gasoline v4 - Technical Specification

## Overview

v4 adds four new capabilities to Gasoline, each exposed as MCP tools that AI coding assistants can invoke on demand:

1. **WebSocket Monitoring** - Passive capture of WebSocket lifecycle and messages
2. **Network Response Bodies** - Capture request/response payloads for API debugging
3. **Live DOM Queries** - On-demand DOM state inspection
4. **Accessibility Audit** - Run axe-core and surface violations

---

## Architecture Changes

```
┌─────────────────────────────────────────────────────────────────┐
│                         Browser                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Inject Script (v4)                        ││
│  │                                                              ││
│  │  WebSocket ─────┐  (intercept constructor + send/message)    ││
│  │  fetch/XHR ─────┤  (capture req/res bodies)                  ││
│  │  DOM ───────────┤  (query on demand via message)             ││
│  │  axe-core ──────┤  (run audit on demand)                     ││
│  └─────────────────┼───────────────────────────────────────────┘│
│                    │ postMessage                                  │
│  ┌─────────────────▼───────────────────────────────────────────┐│
│  │              Content Script                                  ││
│  │  - Routes captures to background                             ││
│  │  - Handles on-demand requests (DOM query, a11y audit)        ││
│  └─────────────────┬───────────────────────────────────────────┘│
│                    │ chrome.runtime.sendMessage                   │
│  ┌─────────────────▼───────────────────────────────────────────┐│
│  │            Background Service Worker                         ││
│  │  - Batches passive captures (WebSocket, network bodies)      ││
│  │  - Proxies on-demand requests to content script              ││
│  └─────────────────┬───────────────────────────────────────────┘│
└────────────────────┼─────────────────────────────────────────────┘
                     │ HTTP
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Gasoline Server (Go)                           │
│                                                                   │
│  Passive endpoints (extension pushes):                            │
│    POST /logs              - Existing log entries                  │
│    POST /websocket-events  - WebSocket lifecycle + messages        │
│    POST /network-bodies    - Request/response payloads             │
│                                                                   │
│  On-demand endpoints (MCP triggers, server asks extension):       │
│    POST /query-dom         - Request DOM state from extension      │
│    POST /run-a11y-audit    - Request accessibility audit           │
│    GET  /dom-result        - Poll for DOM query result             │
│    GET  /a11y-result       - Poll for audit result                 │
│                                                                   │
│  MCP Tools:                                                       │
│    get_websocket_events    - Return captured WS events             │
│    get_network_bodies      - Return captured request/response data │
│    query_dom               - Query live DOM state                  │
│    run_accessibility_audit - Run axe-core audit                    │
└─────────────────────────────────────────────────────────────────┘
```

### On-Demand Request Flow

For tools that query live browser state (DOM queries, accessibility audit), the flow is:

1. AI calls MCP tool (e.g., `query_dom`)
2. Server receives the request and stores it as a pending query
3. Extension polls server for pending queries (via existing connection or SSE)
4. Extension executes the query in the content script
5. Extension POSTs the result back to the server
6. Server returns the result to the MCP tool caller

**Polling interval**: Extension checks for pending queries every 1 second.
**Timeout**: On-demand queries timeout after 10 seconds if no response from extension.

---

## Feature 1: WebSocket Monitoring

### Purpose

Capture WebSocket connection lifecycle and message payloads so the AI can debug real-time features (chat, live updates, multiplayer, etc.) without the user manually copying network tab data.

### Capture Strategy

Intercept the `WebSocket` constructor in the inject script to wrap all WebSocket instances:

```javascript
const OriginalWebSocket = window.WebSocket;
window.WebSocket = function(url, protocols) {
  const ws = new OriginalWebSocket(url, protocols);
  const id = crypto.randomUUID();
  const conn = createConnectionTracker(id, url);

  // Track lifecycle
  ws.addEventListener('open', () => {
    conn.state = 'open';
    conn.openedAt = Date.now();
    emit('ws:open', { id, url });
  });
  ws.addEventListener('close', (e) => {
    conn.state = 'closed';
    emit('ws:close', { id, url, code: e.code, reason: e.reason });
  });
  ws.addEventListener('error', () => {
    conn.state = 'error';
    emit('ws:error', { id, url });
  });

  // Track incoming messages (with adaptive sampling)
  ws.addEventListener('message', (e) => {
    conn.stats.incoming.count++;
    conn.stats.incoming.bytes += getSize(e.data);
    conn.stats.incoming.lastAt = Date.now();
    conn.stats.incoming.lastPreview = truncate(e.data, 200);

    if (conn.shouldSample('incoming')) {
      emit('ws:message', {
        id, url, direction: 'incoming',
        data: formatPayload(e.data), size: getSize(e.data),
        sampled: conn.getSamplingInfo()
      });
    }
  });

  // Intercept send (with adaptive sampling)
  const origSend = ws.send.bind(ws);
  ws.send = (data) => {
    conn.stats.outgoing.count++;
    conn.stats.outgoing.bytes += getSize(data);
    conn.stats.outgoing.lastAt = Date.now();
    conn.stats.outgoing.lastPreview = truncate(data, 200);

    if (conn.shouldSample('outgoing')) {
      emit('ws:message', {
        id, url, direction: 'outgoing',
        data: formatPayload(data), size: getSize(data),
        sampled: conn.getSamplingInfo()
      });
    }
    origSend(data);
  };

  return ws;
};
window.WebSocket.prototype = OriginalWebSocket.prototype;
```

### Adaptive Sampling

High-frequency WebSocket connections (stock feeds, game state, telemetry) can produce hundreds of messages per second. Logging every message would overwhelm the buffer and provide no useful signal.

**Sampling strategy:**

| Message Rate | Behavior |
|-------------|----------|
| < 10 msg/s | Log every message (no sampling) |
| 10–50 msg/s | Log 1 in N (targeting ~10 logged msg/s) |
| 50–200 msg/s | Log 1 in N (targeting ~5 logged msg/s) + rate stats |
| > 200 msg/s | Log 1 in N (targeting ~2 logged msg/s) + rate stats |

**Rate calculation**: Rolling 5-second window, recalculated every second.

**Always logged regardless of sampling:**
- Connection lifecycle events (open, close, error)
- First 5 messages on a new connection (for schema detection)
- Messages matching a different JSON structure than previous (schema change detection)

**Schema detection**: On the first 5 messages, if all are JSON, extract top-level keys as the "schema fingerprint". If a later message has different keys, log it even if it would be sampled out. This catches events like subscription confirmations, error responses, or protocol changes within a stream.

### Binary Message Handling

Binary WebSocket messages (ArrayBuffer, Blob) are common in financial feeds (protobuf), games (custom binary), and media streams.

| Content | Handling |
|---------|----------|
| JSON (text message) | Capture full payload (subject to 4KB truncation) |
| Text (non-JSON) | Capture full payload (subject to 4KB truncation) |
| Binary < 256 bytes | Hex preview: `"[Binary: 128B] 0a1b2c3d..."` (first 64 bytes as hex) |
| Binary >= 256 bytes | Size + magic bytes: `"[Binary: 4096B, magic: 0a1b2c3d]"` |

The hex preview allows developers to identify protobuf messages, MessagePack, or custom protocols. The AI can recognize common binary format headers.

### Log Entry Format

```jsonl
{"ts":"2024-01-15T10:30:00.000Z","type":"websocket","event":"open","id":"uuid","url":"wss://api.example.com/ws","tabId":123}
{"ts":"2024-01-15T10:30:01.000Z","type":"websocket","event":"message","id":"uuid","direction":"incoming","data":"{\"type\":\"chat\",\"msg\":\"hello\"}","size":32}
{"ts":"2024-01-15T10:30:02.000Z","type":"websocket","event":"message","id":"uuid","direction":"outgoing","data":"{\"type\":\"ping\"}","size":16}
{"ts":"2024-01-15T10:30:02.500Z","type":"websocket","event":"message","id":"uuid","direction":"incoming","data":"{\"sym\":\"AAPL\",\"price\":185.42}","size":34,"sampled":{"rate":"48.2/s","logged":"1/5","window":"5s"}}
{"ts":"2024-01-15T10:30:05.000Z","type":"websocket","event":"close","id":"uuid","code":1000,"reason":"normal closure"}
{"ts":"2024-01-15T10:30:06.000Z","type":"websocket","event":"error","id":"uuid","url":"wss://api.example.com/ws"}
```

### MCP Tool: `get_websocket_events`

**Description**: Returns recent WebSocket events (connections, messages, disconnections).

**Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `connection_id` | string | null | Filter by specific connection UUID |
| `url_filter` | string | null | Filter by URL substring |
| `direction` | string | null | `incoming`, `outgoing`, or null for both |
| `limit` | number | 50 | Max events to return |

**Response**: Array of WebSocket event objects, newest first.

### MCP Tool: `get_websocket_status`

**Description**: Returns the current state of all tracked WebSocket connections — whether they're open, their message rates, and a preview of recent data. Use this to answer "is my WebSocket connected?" without scanning event history.

**Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `url_filter` | string | null | Filter by URL substring |
| `connection_id` | string | null | Filter by specific connection UUID |

**Response**:
```json
{
  "connections": [
    {
      "id": "uuid-1",
      "url": "wss://feed.example.com/prices",
      "state": "open",
      "openedAt": "2024-01-15T10:30:00.000Z",
      "duration": "5m02s",
      "messageRate": {
        "incoming": { "perSecond": 48.2, "total": 14460, "bytes": 892400 },
        "outgoing": { "perSecond": 0.1, "total": 3, "bytes": 156 }
      },
      "lastMessage": {
        "incoming": {
          "at": "2024-01-15T10:35:02.000Z",
          "age": "0.2s",
          "preview": "{\"sym\":\"AAPL\",\"price\":185.42,\"vol\":1234}"
        },
        "outgoing": {
          "at": "2024-01-15T10:30:01.000Z",
          "age": "5m01s",
          "preview": "{\"action\":\"subscribe\",\"symbols\":[\"AAPL\",\"GOOG\"]}"
        }
      },
      "schema": {
        "detectedKeys": ["sym", "price", "vol", "ts"],
        "messageCount": 14460,
        "consistent": true
      },
      "sampling": {
        "active": true,
        "rate": "1/5",
        "reason": "48.2 msg/s exceeds 10 msg/s threshold"
      }
    },
    {
      "id": "uuid-2",
      "url": "wss://chat.example.com/rooms/general",
      "state": "open",
      "openedAt": "2024-01-15T10:28:00.000Z",
      "duration": "7m02s",
      "messageRate": {
        "incoming": { "perSecond": 0.3, "total": 126, "bytes": 18900 },
        "outgoing": { "perSecond": 0.1, "total": 12, "bytes": 1800 }
      },
      "lastMessage": {
        "incoming": {
          "at": "2024-01-15T10:34:58.000Z",
          "age": "4.2s",
          "preview": "{\"type\":\"message\",\"user\":\"alice\",\"text\":\"sounds good!\"}"
        },
        "outgoing": {
          "at": "2024-01-15T10:34:45.000Z",
          "age": "17.2s",
          "preview": "{\"type\":\"message\",\"text\":\"let's deploy after lunch\"}"
        }
      },
      "schema": {
        "detectedKeys": ["type", "user", "text", "ts"],
        "messageCount": 138,
        "consistent": false,
        "variants": ["message (89%)", "typing (8%)", "presence (3%)"]
      },
      "sampling": {
        "active": false,
        "reason": "0.4 msg/s below 10 msg/s threshold"
      }
    }
  ],
  "closed": [
    {
      "id": "uuid-0",
      "url": "wss://api.example.com/notifications",
      "state": "closed",
      "openedAt": "2024-01-15T10:25:00.000Z",
      "closedAt": "2024-01-15T10:29:30.000Z",
      "closeCode": 1006,
      "closeReason": "",
      "totalMessages": { "incoming": 8, "outgoing": 2 }
    }
  ]
}
```

### Limits

- **Message body truncation**: 4KB per message. If exceeded, truncate and add `"truncated": true`.
- **Max tracked connections**: 20 concurrent. Oldest closed connection evicted if exceeded.
- **Buffer size**: 500 WebSocket events in memory ring buffer (separate from main log rotation).
- **Closed connection history**: Keep last 10 closed connections for debugging reconnection issues.
- **Stats retention**: Per-connection stats kept for 5 minutes after close.

### Extension Settings

New toggle in popup: **"Capture WebSockets"** (default: ON).

---

## Feature 2: Network Response Bodies

### Purpose

Capture request and response bodies for fetch/XHR calls so the AI can see exactly what data the API returned (or what the frontend sent) without the user manually copying from the Network tab.

### Capture Strategy

Extend the existing network interception in inject.js:

```javascript
// Wrap fetch
const origFetch = window.fetch;
window.fetch = async function(input, init) {
  const url = typeof input === 'string' ? input : input.url;
  const method = init?.method || 'GET';
  const requestBody = init?.body ? await readBody(init.body) : null;

  const response = await origFetch.apply(this, arguments);

  // Clone response to read body without consuming it
  const clone = response.clone();
  const responseBody = await readResponseBody(clone);

  emit('network:body', {
    url, method,
    status: response.status,
    requestBody: truncate(requestBody),
    responseBody: truncate(responseBody),
    requestHeaders: sanitizeHeaders(init?.headers),
    responseHeaders: extractHeaders(response.headers),
    contentType: response.headers.get('content-type'),
  });

  return response;
};
```

### Body Reading

```javascript
async function readResponseBody(response) {
  const contentType = response.headers.get('content-type') || '';

  if (contentType.includes('application/json')) {
    return await response.text(); // Keep as string for JSON
  }
  if (contentType.includes('text/')) {
    return await response.text();
  }
  // Binary content: just report size
  const blob = await response.blob();
  return `[Binary: ${blob.size} bytes, type: ${contentType}]`;
}
```

### Log Entry Format

```jsonl
{"ts":"2024-01-15T10:30:00.000Z","type":"network_body","method":"POST","url":"/api/users","status":201,"requestBody":"{\"name\":\"Alice\"}","responseBody":"{\"id\":1,\"name\":\"Alice\"}","contentType":"application/json","duration":142}
{"ts":"2024-01-15T10:30:01.000Z","type":"network_body","method":"GET","url":"/api/products?page=2","status":200,"requestBody":null,"responseBody":"{\"items\":[...],\"total\":50}","contentType":"application/json","duration":89}
```

### MCP Tool: `get_network_bodies`

**Description**: Returns recent network request/response bodies for API debugging.

**Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `url_filter` | string | null | Filter by URL substring |
| `method` | string | null | Filter by HTTP method |
| `status_min` | number | null | Min status code (e.g., 400 for errors only) |
| `status_max` | number | null | Max status code |
| `limit` | number | 20 | Max entries to return |

**Response**: Array of network body objects, newest first.

### Limits

- **Body truncation**: 8KB per request body, 16KB per response body. Truncated entries get `"truncated": true`.
- **Buffer size**: 100 entries in memory ring buffer.
- **Excluded content types**: Images, video, audio, fonts, wasm — logged as `[Binary: size, type]`.
- **Excluded URLs**: Extension's own requests to the Gasoline server.

### Header Sanitization

Strip sensitive headers before capture:
- `Authorization`
- `Cookie` / `Set-Cookie`
- `X-API-Key`
- Any header matching `/token|secret|key|password/i`

### Extension Settings

New toggle in popup: **"Capture Network Bodies"** (default: OFF — opt-in due to potential payload size/sensitivity).

---

## Feature 3: Live DOM Queries

### Purpose

Allow the AI to inspect the current state of the page on demand — what elements exist, their attributes, text content, computed styles, and structure. This eliminates the need for users to manually describe what they see.

### MCP Tool: `query_dom`

**Description**: Query the live DOM state of the active browser tab. Returns matching elements with their attributes, text, and optionally computed styles.

**Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `selector` | string | required | CSS selector to query |
| `include_styles` | boolean | false | Include computed styles for matched elements |
| `include_children` | boolean | false | Include child subtree (depth-limited) |
| `max_depth` | number | 3 | Max child depth when include_children is true |
| `properties` | string[] | null | Specific style properties to include (null = all) |

**Response**:
```json
{
  "url": "http://localhost:3000/dashboard",
  "title": "My App - Dashboard",
  "matches": [
    {
      "selector": "#user-list > li:nth-child(1)",
      "tag": "li",
      "attributes": { "class": "user-item active", "data-id": "42" },
      "text": "Alice Johnson",
      "boundingBox": { "x": 20, "y": 140, "width": 300, "height": 48 },
      "visible": true,
      "styles": { "display": "flex", "color": "rgb(0, 0, 0)" },
      "children": [
        { "tag": "span", "attributes": { "class": "name" }, "text": "Alice Johnson" },
        { "tag": "span", "attributes": { "class": "status" }, "text": "Online" }
      ]
    }
  ],
  "matchCount": 5,
  "returnedCount": 5
}
```

### Execution Flow

1. MCP tool called with selector
2. Server stores pending query with unique ID
3. Extension picks up pending query on next poll (1s interval)
4. Content script runs `document.querySelectorAll(selector)` in the active tab
5. Results serialized and POSTed back to server
6. Server returns results to MCP caller

### MCP Tool: `get_page_info`

**Description**: Get basic page state without a specific selector — URL, title, viewport size, scroll position, and a summary of the page structure.

**Parameters**: None.

**Response**:
```json
{
  "url": "http://localhost:3000/dashboard",
  "title": "My App - Dashboard",
  "viewport": { "width": 1440, "height": 900 },
  "scroll": { "x": 0, "y": 320 },
  "documentHeight": 2400,
  "forms": [
    { "id": "login-form", "action": "/api/login", "fields": ["email", "password"] }
  ],
  "headings": ["Dashboard", "Recent Activity", "Settings"],
  "links": 24,
  "images": 8,
  "interactiveElements": 15
}
```

### Limits

- **Max elements returned**: 50 per query. If more match, return first 50 + `matchCount` total.
- **Max subtree depth**: 5 levels (even if requested higher).
- **Text truncation**: 500 chars per element's text content.
- **Style properties**: If `properties` not specified and `include_styles` is true, return only layout-relevant properties (display, position, width, height, margin, padding, flex, grid, visibility, opacity, overflow, z-index, color, background-color, font-size).
- **Timeout**: 10 seconds. If extension doesn't respond, return error.

---

## Feature 4: Accessibility Audit

### Purpose

Run automated accessibility checks on the current page and return violations in a format the AI can act on — identifying which elements fail, what rule they break, and how to fix them.

### Implementation

Inject [axe-core](https://github.com/dequelabs/axe-core) (the industry standard a11y testing engine, ~200KB minified) into the page on demand and run a full audit.

### MCP Tool: `run_accessibility_audit`

**Description**: Run an accessibility audit on the current page using axe-core. Returns violations grouped by impact level.

**Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `scope` | string | null | CSS selector to limit audit scope (null = full page) |
| `tags` | string[] | null | axe-core rule tags to run (e.g., `["wcag2a", "wcag2aa"]`) |
| `include_passes` | boolean | false | Include passing checks (verbose) |

**Response**:
```json
{
  "url": "http://localhost:3000/signup",
  "timestamp": "2024-01-15T10:30:00.000Z",
  "summary": {
    "violations": 4,
    "passes": 52,
    "incomplete": 2,
    "inapplicable": 31
  },
  "violations": [
    {
      "id": "color-contrast",
      "impact": "serious",
      "description": "Elements must have sufficient color contrast",
      "helpUrl": "https://dequeuniversity.com/rules/axe/4.8/color-contrast",
      "wcag": ["wcag2aa", "1.4.3"],
      "nodes": [
        {
          "selector": "#signup-form > label:nth-child(2)",
          "html": "<label class=\"form-label subtle\">Email address</label>",
          "failureSummary": "Element has insufficient color contrast of 2.8:1 (foreground: #999999, background: #ffffff, required: 4.5:1)",
          "fix": "Change foreground color to at least #767676 for 4.5:1 contrast ratio"
        }
      ]
    },
    {
      "id": "image-alt",
      "impact": "critical",
      "description": "Images must have alternate text",
      "helpUrl": "https://dequeuniversity.com/rules/axe/4.8/image-alt",
      "wcag": ["wcag2a", "1.1.1"],
      "nodes": [
        {
          "selector": "img.hero-image",
          "html": "<img class=\"hero-image\" src=\"/hero.jpg\">",
          "failureSummary": "Element does not have an alt attribute",
          "fix": "Add an alt attribute describing the image content"
        }
      ]
    }
  ]
}
```

### axe-core Integration

```javascript
// Injected on demand (not on every page load)
async function runAxeAudit(options) {
  // Dynamically load axe-core if not already loaded
  if (!window.axe) {
    const script = document.createElement('script');
    script.src = chrome.runtime.getURL('lib/axe.min.js');
    await new Promise((resolve, reject) => {
      script.onload = resolve;
      script.onerror = reject;
      document.head.appendChild(script);
    });
  }

  const config = {
    runOnly: options.tags || undefined,
    resultTypes: options.include_passes
      ? ['violations', 'passes', 'incomplete']
      : ['violations', 'incomplete'],
  };

  const context = options.scope
    ? { include: [options.scope] }
    : undefined;

  return await window.axe.run(context, config);
}
```

### Bundling

- Bundle `axe-core` minified (~200KB) as `extension/lib/axe.min.js`
- Declared in `manifest.json` under `web_accessible_resources` so it can be injected into pages
- Only loaded when an audit is requested — no performance impact on normal browsing

### Limits

- **Max nodes per violation**: 10. If more, include count in `nodeCount` field.
- **HTML snippet truncation**: 200 chars per node's HTML.
- **Timeout**: 30 seconds for full-page audit (large pages can take time).
- **Caching**: Results cached for 30 seconds per URL — repeated calls within that window return cached results unless `force_refresh: true` is passed.

---

## Shared Concerns

### Extension Manifest Changes

```json
{
  "web_accessible_resources": [{
    "resources": ["lib/axe.min.js"],
    "matches": ["<all_urls>"]
  }],
  "permissions": ["activeTab", "scripting"]
}
```

Note: `scripting` permission needed for dynamic script injection of axe-core.

### New Extension Poll Mechanism

For on-demand features (DOM queries, accessibility audit), the extension needs to know when the server has a pending request. Two options:

**Option A: Polling (simpler, recommended for v4)**
- Background service worker polls `GET /pending-queries` every 1 second
- Returns empty array if nothing pending
- Returns query details when AI has made a request

**Option B: Server-Sent Events (future optimization)**
- Extension opens SSE connection to server
- Server pushes queries as they arrive
- Lower latency but more complex lifecycle management

### Privacy & Security

- **Network bodies**: Off by default. User must explicitly enable.
- **Header sanitization**: Auth headers stripped automatically.
- **Body content**: Never sent to any external service — stays on localhost.
- **DOM queries**: Only run on the active tab, only return text/attributes (no JS execution).
- **axe-core**: Runs entirely client-side, no external calls.

### Performance & SLOs

Gasoline MUST NOT degrade the user's browsing experience. The extension runs in the same process as the user's application, so every millisecond of overhead is directly stealing from their app's frame budget.

#### SLO Targets

| Metric | Target | Hard Limit | Action on Violation |
|--------|--------|------------|---------------------|
| **Page load impact** | < 20ms | 50ms | Defer interception to after `load` event |
| **Main thread block (per intercept)** | < 1ms | 5ms | Move to async / Web Worker |
| **Main thread block (DOM query)** | < 50ms | 200ms | Abort and return partial results |
| **Main thread block (a11y audit)** | < 3s | 10s | Abort with timeout error |
| **Memory usage (extension total)** | < 20MB | 50MB | Evict oldest buffers, disable network bodies |
| **fetch() wrapper overhead** | < 0.5ms sync | 2ms sync | Disable body capture for that request |
| **WebSocket handler overhead** | < 0.1ms per msg | 0.5ms per msg | Increase sampling rate |
| **Background → Server POST** | < 50ms | 200ms | Increase batch interval |
| **Server MCP tool response** | < 200ms | 2s | Return cached/partial data |
| **Server memory usage** | < 30MB | 100MB | Evict oldest entries, reject new bodies |
| **Log file write latency** | < 5ms | 50ms | Buffer writes, async flush |

#### Extension Performance Guardrails

**1. Async-Only Body Reading**

The `fetch()` wrapper must NEVER block on reading the response body synchronously. The interception itself (wrapping the call) is sync and fast, but body reading happens in a microtask after the response is returned to the caller:

```javascript
window.fetch = async function(input, init) {
  // Sync: just start timing (~0.1ms overhead)
  const startTime = performance.now();

  // Call original fetch — user gets their response immediately
  const response = await origFetch.apply(this, arguments);

  // Async: read body in background WITHOUT blocking the caller
  // Schedule body capture as a low-priority task
  if (captureEnabled && shouldCaptureUrl(url)) {
    queueMicrotask(() => captureBody(response.clone(), url, method, startTime));
  }

  return response; // User gets response with zero additional latency
};
```

**2. WebSocket Handler Budget**

Each WebSocket message handler has a **0.1ms budget**. The handler only increments counters and checks the sampling flag — no string processing, no truncation, no serialization on the hot path:

```javascript
// HOT PATH — must complete in < 0.1ms
ws.addEventListener('message', (e) => {
  conn.stats.incoming.count++;        // Increment counter
  conn.stats.incoming.lastAt = now(); // Store timestamp

  if (conn.sampleCounter++ % conn.sampleRate === 0) {
    // COLD PATH — runs only for sampled messages
    // Deferred to requestIdleCallback or setTimeout(0)
    scheduleCapture(id, 'incoming', e.data);
  }
});
```

**3. Deferred Serialization**

Message payloads are NOT serialized (JSON.stringify, truncate, format) on the main thread during the intercept. Instead:

1. Raw reference stored in a pending queue
2. `requestIdleCallback` (or `setTimeout(0)` fallback) processes the queue
3. Serialized entries batched and sent to background service worker

This ensures the user's app never pays for Gasoline's serialization work during a frame.

**4. Memory Pressure Detection**

The extension monitors its own memory usage via `performance.memory` (Chrome-only) or by tracking buffer sizes:

```javascript
const memoryCheck = () => {
  const usage = estimateMemoryUsage();

  if (usage > MEMORY_SOFT_LIMIT) {   // 20MB
    // Reduce buffer sizes by 50%
    wsBuffer.resize(wsBuffer.capacity / 2);
    networkBuffer.resize(networkBuffer.capacity / 2);
    console.debug('[gasoline] Memory pressure: reduced buffers');
  }

  if (usage > MEMORY_HARD_LIMIT) {   // 50MB
    // Disable network body capture entirely
    networkBodiesEnabled = false;
    networkBuffer.clear();
    console.debug('[gasoline] Memory critical: disabled network bodies');
  }
};

// Check every 30 seconds
setInterval(memoryCheck, 30000);
```

**5. Page Load Deferral**

v4 intercepts (WebSocket constructor, fetch body capture) are NOT installed during initial page load. They are deferred until after the `load` event fires:

```javascript
// Content script entry point
if (document.readyState === 'complete') {
  installV4Intercepts();
} else {
  window.addEventListener('load', () => {
    // Additional 100ms delay to avoid competing with app initialization
    setTimeout(installV4Intercepts, 100);
  });
}

// v1-v3 intercepts (console, basic network errors, exceptions) still install immediately
// since they are lightweight and don't clone bodies or wrap constructors
```

This ensures Gasoline adds **zero milliseconds** to the page's Time to Interactive (TTI).

**6. Per-Request Body Capture Budget**

If reading a response body takes longer than 5ms (large JSON, slow clone), abort the capture:

```javascript
async function captureBody(clone, url, method, fetchStart) {
  const captureStart = performance.now();

  try {
    const text = await Promise.race([
      clone.text(),
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error('body_timeout')), 5)
      )
    ]);

    // Only store if we got it fast enough
    emit('network:body', { url, method, responseBody: truncate(text) });
  } catch (e) {
    if (e.message === 'body_timeout') {
      // Log that we skipped this one
      emit('network:body', { url, method, responseBody: '[Skipped: body read exceeded 5ms]' });
    }
  }
}
```

#### Server Performance Guardrails

**1. Memory-Bounded Buffers**

All server-side buffers have hard memory caps, not just entry counts:

| Buffer | Max Entries | Max Memory | Eviction |
|--------|------------|------------|----------|
| WebSocket events | 500 | 4MB | Oldest first |
| Network bodies | 100 | 8MB | Oldest first |
| Pending queries | 5 | 1KB | Oldest first (stale queries) |
| Connection tracker | 20 active + 10 closed | 2MB | Oldest closed first |

When a buffer hits its memory cap (not just entry count), entries are evicted starting from the oldest regardless of how many entries remain.

**2. Disk Write Batching**

Network body entries are NOT written to the JSONL log file individually. Instead:

- Buffer up to 10 entries or 1 second (whichever comes first)
- Write batch in a single `write()` syscall
- Use `O_APPEND` flag — no file locking needed
- If write takes >50ms, skip writing bodies to disk (keep in memory only)

**3. Server Circuit Breaker**

If the extension floods the server (e.g., a page opens 100 WebSocket connections each sending at 1000 msg/s):

| Condition | Action |
|-----------|--------|
| > 1000 events/second received | Respond with `429 Too Many Requests` |
| > 50MB memory usage | Reject new network body POSTs |
| > 100MB memory usage | Clear all buffers, restart in minimal mode |
| Pending query not picked up in 10s | Delete it, return timeout to MCP caller |

**4. Extension → Server Failure Handling**

```
Retry budget: 3 attempts per batch
Backoff: 100ms → 500ms → 2000ms
Circuit breaker: After 5 consecutive failures, pause sending for 30 seconds
Recovery: Single successful POST resets the circuit breaker
Server down: Extension keeps buffering locally (up to memory cap), drains on reconnect
```

The extension NEVER retries indefinitely. If the server is down, the extension silently buffers what it can and discards the rest.

#### Performance Monitoring

The extension exposes its own performance metrics via the existing debug log:

```json
{
  "type": "gasoline:perf",
  "ts": "2024-01-15T10:30:00.000Z",
  "metrics": {
    "memoryUsageMB": 12.4,
    "wsEventsBuffered": 342,
    "networkBodiesBuffered": 67,
    "droppedEvents": 0,
    "samplingActive": true,
    "avgInterceptLatencyMs": 0.08,
    "serverPostSuccessRate": 1.0,
    "circuitBreakerState": "closed"
  }
}
```

This is logged every 60 seconds when debug mode is enabled, and accessible via the existing `Export Debug Log` button.

#### Performance Testing

v4 must include performance SLO tests (extending the existing v3 benchmark suite):

| Test | Method | Pass Criteria |
|------|--------|---------------|
| fetch wrapper overhead | Measure 1000 fetch() calls with/without extension | < 1ms avg added latency |
| WS message throughput | Send 10,000 messages at 1000/s, measure frame drops | 0 dropped frames |
| Memory under load | 20 WS connections × 100 msg/s for 5 min | < 50MB extension memory |
| Page load impact | Lighthouse before/after on reference app | < 50ms TTI difference |
| Body capture large response | fetch() 1MB JSON response | < 10ms sync overhead |
| DOM query on complex page | querySelectorAll('*') on 10,000-node page | < 200ms response |
| a11y audit on complex page | axe.run() on 10,000-node page | < 10s total |
| Server memory under load | 1000 network body POSTs with 16KB each | < 50MB server RSS |

### Removed: DOM Snapshot Enrichment

The DOM snapshot enrichment (`_enrichments: ['domSnapshot']`) has been removed. Rationale:

- **Poor signal-to-noise ratio**: A serialized DOM tree (up to 100 nodes, 100KB) consumes significant LLM context window for minimal debugging value
- **Redundant with `query_dom`**: The on-demand DOM query tool provides targeted, relevant DOM state when needed
- **Context window cost**: 100KB of serialized DOM leaves little room for the actual error context in LLM interactions

The `query_dom` MCP tool remains available for targeted DOM inspection.

### Feature Size Awareness

Features that add significant data to log entries should communicate their cost:

| Feature | Default | Data Cost | Warning |
|---------|---------|-----------|---------|
| Screenshot on Error | OFF | JPEG file on disk | "High-resolution displays will produce large files" |
| Network Waterfall | OFF | ~50KB per error | "Adds ~50KB of timing data per error" |
| Performance Marks | OFF | 2-10KB per error | "Adds timing data to log entries" |
| User Actions | ON | ~2KB per error | No warning (small) |
| Source Maps | OFF | Minimal | No warning |

All large-data features default to OFF and show an explanatory note when enabled.

### Context Annotation Monitoring

#### Purpose

Context annotations (`window.__gasoline.annotate(key, value)`) allow developers to attach arbitrary metadata to error entries. While individual values are capped at 4KB, heavy usage (up to 50 keys × 4KB = 200KB per entry) can silently bloat log entries and consume LLM context.

#### Monitoring Behavior

The extension should track the cumulative size of context annotations and warn when usage is excessive:

1. **Measurement**: On each error entry, calculate the total serialized size of all `_context` data included
2. **Threshold**: If total context size exceeds **20KB** in any single entry, flag as excessive
3. **Frequency tracking**: If 3 or more entries in a 60-second window exceed the threshold, trigger a persistent warning

#### Warning UI

When excessive context usage is detected:

1. The "User Actions" or a new "Context Annotations" indicator in the popup should:
   - Highlight in **orange** (`#d29922`)
   - Show a "!" icon next to the label
   - Display a tooltip: "Context annotations are adding [X]KB per error entry. This may consume significant AI context window. Consider reducing annotation keys or values."

2. The warning should:
   - Persist until the next popup open (re-evaluated on each status check)
   - Clear automatically if usage drops below threshold for 60 seconds
   - Be dismissible by the user

#### Implementation Notes

- Monitoring runs in the background service worker (not inject.js) to avoid page performance impact
- Size calculation happens when entries are batched, not on each `annotate()` call
- The warning is informational only — annotations are never silently dropped or truncated beyond the existing per-value 4KB cap
- Badge color does not change for this warning (reserved for connection status)

### New MCP Tools Summary

| Tool | Type | Description |
|------|------|-------------|
| `get_websocket_events` | Passive | Return buffered WebSocket events |
| `get_websocket_status` | Passive | Return current connection states, rates, and schemas |
| `get_network_bodies` | Passive | Return buffered network request/response data |
| `query_dom` | On-demand | Query live DOM state by CSS selector |
| `get_page_info` | On-demand | Get page structure summary |
| `run_accessibility_audit` | On-demand | Run axe-core accessibility audit |
