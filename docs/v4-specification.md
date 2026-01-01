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

  // Track lifecycle
  ws.addEventListener('open', () => emit('ws:open', { id, url }));
  ws.addEventListener('close', (e) => emit('ws:close', { id, url, code: e.code, reason: e.reason }));
  ws.addEventListener('error', () => emit('ws:error', { id, url }));

  // Track incoming messages
  ws.addEventListener('message', (e) => emit('ws:message', {
    id, url, direction: 'incoming', data: truncate(e.data)
  }));

  // Intercept send
  const origSend = ws.send.bind(ws);
  ws.send = (data) => {
    emit('ws:message', { id, url, direction: 'outgoing', data: truncate(data) });
    origSend(data);
  };

  return ws;
};
window.WebSocket.prototype = OriginalWebSocket.prototype;
```

### Log Entry Format

```jsonl
{"ts":"2024-01-15T10:30:00.000Z","type":"websocket","event":"open","id":"uuid","url":"wss://api.example.com/ws","tabId":123}
{"ts":"2024-01-15T10:30:01.000Z","type":"websocket","event":"message","id":"uuid","direction":"incoming","data":"{\"type\":\"chat\",\"msg\":\"hello\"}","size":32}
{"ts":"2024-01-15T10:30:02.000Z","type":"websocket","event":"message","id":"uuid","direction":"outgoing","data":"{\"type\":\"ping\"}","size":16}
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

### Limits

- **Message body truncation**: 4KB per message. If exceeded, truncate and add `"truncated": true`.
- **Max tracked connections**: 20 concurrent. Oldest connection evicted if exceeded.
- **Buffer size**: 200 WebSocket events in memory ring buffer (separate from main log rotation).
- **Binary messages**: Captured as `"[Binary: ${size} bytes]"` — not base64 encoded.

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

### Performance Budget

| Feature | Overhead (idle) | Overhead (active) |
|---------|-----------------|-------------------|
| WebSocket monitoring | ~0.1ms per message | Negligible |
| Network bodies | ~2ms per request (clone + read) | Up to 50ms for large responses |
| DOM queries | Zero (on-demand only) | 10-100ms depending on selector complexity |
| Accessibility audit | Zero (on-demand only) | 1-5s for full page audit |

### New MCP Tools Summary

| Tool | Type | Description |
|------|------|-------------|
| `get_websocket_events` | Passive | Return buffered WebSocket events |
| `get_network_bodies` | Passive | Return buffered network request/response data |
| `query_dom` | On-demand | Query live DOM state by CSS selector |
| `get_page_info` | On-demand | Get page structure summary |
| `run_accessibility_audit` | On-demand | Run axe-core accessibility audit |
