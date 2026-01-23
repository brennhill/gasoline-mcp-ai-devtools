# Gasoline CI — Technical Specification

## Status: Specification

---

## Overview

Gasoline CI extends Gasoline's browser observability from local development into CI/CD pipelines. The same capture logic that powers the Chrome extension runs in headless browsers during automated testing, giving teams:

- **Console/network/WebSocket capture** during E2E test runs
- **Automatic failure context** — every failed test gets full browser state
- **AI-powered failure analysis** — MCP tools diagnose failures without human intervention
- **Zero test code changes** — observability is injected at the infrastructure level

---

## Architecture

```
                     LOCAL DEV                                CI/CD
              ┌─────────────────────┐              ┌──────────────────────────┐
              │  Chrome Extension    │              │  Option A: Script Inject │
              │  inject.js           │              │  gasoline-ci.js          │
              │  content.js          │              │  (addInitScript)         │
              │  background.js       │              │                          │
              └────────┬────────────┘              │  Option B: Extension     │
                       │                           │  --load-extension=...    │
                       │ HTTP POST                 │                          │
                       ▼                           │  Option C: CDP           │
              ┌─────────────────────┐              │  Runtime.evaluate        │
              │  Gasoline Server     │◄─────────────┘                          │
              │  localhost:7890       │                         │               │
              │                      │                         │ HTTP POST     │
              │  /logs               │◄────────────────────────┘               │
              │  /websocket-events   │                                         │
              │  /network-bodies     │              ┌──────────────────────────┐
              │  /snapshot (NEW)     │◄─────────────│  gasoline-report CLI     │
              │  /clear (NEW)        │              │  (AI failure analysis)   │
              └──────────┬──────────┘              └──────────────────────────┘
                         │ MCP (stdio)
                         ▼
              ┌─────────────────────┐
              │  AI Coding Agent     │
              │  (Claude, Cursor...) │
              └─────────────────────┘
```

### Three Capture Options

| Option | Mechanism | Fidelity | Setup Complexity | Use Case |
|--------|-----------|----------|-----------------|----------|
| **A: Script Injection** | `page.addInitScript()` | High (console, fetch, WS) | Lowest | Playwright/Puppeteer teams |
| **B: Extension Loading** | `--load-extension` | Full (identical to local dev) | Medium | Teams wanting exact parity |
| **C: CDP Direct** | `Runtime.evaluate` | Highest (runtime-level) | Highest | Custom frameworks, Selenium |

All three options POST captured data to the same Gasoline server endpoints. The server doesn't care how data arrives — it exposes the same MCP tools regardless of source.

---

## Component 1: `gasoline-ci.js` (Standalone Capture Script)

### Purpose

A self-contained JavaScript file extracted from `inject.js` that runs in any browser context without Chrome extension APIs. It replaces `window.postMessage` with direct HTTP POST to the Gasoline server.

### Extraction Strategy

The core capture logic in `inject.js` is already pure JavaScript with no Chrome API dependencies. The only extension coupling is the message-passing layer:

```javascript
// Current inject.js transport (extension-dependent):
window.postMessage({ type: 'DEV_CONSOLE_LOG', payload }, '*')

// CI transport (direct HTTP):
fetch('http://127.0.0.1:7890/logs', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ entries: [payload] })
})
```

### File: `packages/gasoline-ci/gasoline-ci.js`

```javascript
/**
 * Gasoline CI - Standalone browser capture for CI/CD pipelines
 *
 * This file is auto-generated from inject.js with the transport layer
 * replaced. Do NOT edit directly — edit inject.js and run the build.
 *
 * Usage:
 *   await page.addInitScript({ path: require.resolve('@anthropic/gasoline-ci') })
 *
 * Environment:
 *   GASOLINE_PORT (default: 7890)
 *   GASOLINE_HOST (default: 127.0.0.1)
 *   GASOLINE_TEST_ID - current test identifier for correlation
 */

(function() {
  'use strict';

  // === Configuration ===
  const GASOLINE_HOST = '127.0.0.1';
  const GASOLINE_PORT = 7890;
  const BASE_URL = `http://${GASOLINE_HOST}:${GASOLINE_PORT}`;

  // Batch settings (avoid flooding server)
  const BATCH_INTERVAL_MS = 100;  // Flush every 100ms
  const MAX_BATCH_SIZE = 50;      // Max entries per flush

  // === Shared constants (from inject.js) ===
  const MAX_STRING_LENGTH = 10240;
  const MAX_RESPONSE_LENGTH = 5120;
  const MAX_DEPTH = 10;
  const SENSITIVE_HEADERS = ['authorization', 'cookie', 'set-cookie', 'x-auth-token'];

  // === Batching Layer ===
  let logBatch = [];
  let wsBatch = [];
  let networkBatch = [];
  let flushTimer = null;

  function scheduleFlush() {
    if (flushTimer) return;
    flushTimer = setTimeout(flush, BATCH_INTERVAL_MS);
  }

  function flush() {
    flushTimer = null;

    if (logBatch.length > 0) {
      const entries = logBatch.splice(0, MAX_BATCH_SIZE);
      sendToServer('/logs', { entries });
    }
    if (wsBatch.length > 0) {
      const events = wsBatch.splice(0, MAX_BATCH_SIZE);
      sendToServer('/websocket-events', { events });
    }
    if (networkBatch.length > 0) {
      const bodies = networkBatch.splice(0, MAX_BATCH_SIZE);
      sendToServer('/network-bodies', { bodies });
    }

    // If there's still data, schedule another flush
    if (logBatch.length > 0 || wsBatch.length > 0 || networkBatch.length > 0) {
      scheduleFlush();
    }
  }

  function sendToServer(endpoint, data) {
    // Use sendBeacon for reliability (won't be cancelled on page unload)
    const url = `${BASE_URL}${endpoint}`;
    const body = JSON.stringify(data);

    if (navigator.sendBeacon) {
      navigator.sendBeacon(url, new Blob([body], { type: 'application/json' }));
    } else {
      // Fallback to fetch (fire-and-forget)
      fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
        keepalive: true
      }).catch(() => {}); // Swallow errors — never interfere with the app
    }
  }

  // === Transport Adapter ===
  // Replaces window.postMessage with server POST
  function emit(type, payload) {
    switch (type) {
      case 'DEV_CONSOLE_LOG':
        logBatch.push(payload);
        break;
      case 'GASOLINE_WS':
        wsBatch.push(payload);
        break;
      case 'GASOLINE_NETWORK_BODY':
        networkBatch.push(payload);
        break;
      // Enhanced actions and web vitals go to logs
      case 'GASOLINE_ENHANCED_ACTION':
      case 'GASOLINE_WEB_VITALS':
        logBatch.push({ type, ...payload });
        break;
    }
    scheduleFlush();
  }

  // === safeSerialize (identical to inject.js) ===
  function safeSerialize(value, depth = 0, seen = new WeakSet()) {
    if (value === null) return null;
    if (value === undefined) return undefined;
    const type = typeof value;
    if (type === 'string') {
      return value.length > MAX_STRING_LENGTH
        ? value.slice(0, MAX_STRING_LENGTH) + '... [truncated]'
        : value;
    }
    if (type === 'number' || type === 'boolean') return value;
    if (type === 'function') return `[Function: ${value.name || 'anonymous'}]`;
    if (type === 'symbol') return value.toString();
    if (type === 'bigint') return value.toString() + 'n';
    if (depth >= MAX_DEPTH) return '[max depth reached]';
    if (value instanceof Error) {
      return { name: value.name, message: value.message, stack: value.stack };
    }
    if (value instanceof RegExp) return value.toString();
    if (value instanceof Date) return value.toISOString();
    if (typeof value === 'object') {
      if (seen.has(value)) return '[Circular]';
      seen.add(value);
      if (Array.isArray(value)) {
        return value.slice(0, 100).map(v => safeSerialize(v, depth + 1, seen));
      }
      if (value instanceof HTMLElement || value instanceof Node) {
        return `[${value.constructor.name}: ${value.tagName || value.nodeName}]`;
      }
      const result = {};
      const keys = Object.keys(value).slice(0, 50);
      for (const key of keys) {
        try { result[key] = safeSerialize(value[key], depth + 1, seen); }
        catch { result[key] = '[unserializable]'; }
      }
      return result;
    }
    return String(value);
  }

  // === Console Capture ===
  function installConsoleCapture() {
    const levels = ['log', 'warn', 'error', 'info', 'debug'];
    const originals = {};

    levels.forEach(level => {
      originals[level] = console[level];
      console[level] = function(...args) {
        originals[level].apply(console, args);

        emit('DEV_CONSOLE_LOG', {
          level,
          message: args.map(a => typeof a === 'string' ? a : safeSerialize(a)).join(' '),
          args: args.map(a => safeSerialize(a)),
          timestamp: new Date().toISOString(),
          url: window.location.href,
          source: 'console'
        });
      };
    });
  }

  // === Exception Capture ===
  function installExceptionCapture() {
    window.addEventListener('error', (event) => {
      emit('DEV_CONSOLE_LOG', {
        level: 'error',
        message: event.message || 'Unknown error',
        args: [safeSerialize(event.error)],
        timestamp: new Date().toISOString(),
        url: window.location.href,
        source: 'exception',
        stack: event.error?.stack,
        filename: event.filename,
        lineno: event.lineno,
        colno: event.colno
      });
    });

    window.addEventListener('unhandledrejection', (event) => {
      const reason = event.reason;
      emit('DEV_CONSOLE_LOG', {
        level: 'error',
        message: reason?.message || String(reason) || 'Unhandled Promise rejection',
        args: [safeSerialize(reason)],
        timestamp: new Date().toISOString(),
        url: window.location.href,
        source: 'unhandledrejection',
        stack: reason?.stack
      });
    });
  }

  // === Fetch/XHR Capture ===
  function installNetworkCapture() {
    const originalFetch = window.fetch;

    window.fetch = async function(input, init = {}) {
      const url = typeof input === 'string' ? input : input.url;
      const method = init.method || (input.method ? input.method : 'GET');
      const startTime = Date.now();

      let requestBody = null;
      if (init.body) {
        try {
          requestBody = typeof init.body === 'string'
            ? init.body.slice(0, MAX_RESPONSE_LENGTH)
            : '[non-string body]';
        } catch { requestBody = '[unreadable]'; }
      }

      try {
        const response = await originalFetch.apply(this, arguments);
        const duration = Date.now() - startTime;

        // Capture network body for non-2xx or all if enabled
        if (response.status >= 400) {
          const clone = response.clone();
          try {
            const text = await clone.text();
            emit('GASOLINE_NETWORK_BODY', {
              url,
              method: method.toUpperCase(),
              status: response.status,
              requestBody,
              responseBody: text.slice(0, MAX_RESPONSE_LENGTH),
              duration,
              timestamp: new Date().toISOString(),
              requestHeaders: filterHeaders(init.headers),
              responseHeaders: filterHeaders(Object.fromEntries(response.headers.entries()))
            });
          } catch {}
        }

        // Always log errors to console stream
        if (response.status >= 400) {
          emit('DEV_CONSOLE_LOG', {
            level: response.status >= 500 ? 'error' : 'warn',
            message: `${method.toUpperCase()} ${url} → ${response.status}`,
            timestamp: new Date().toISOString(),
            url: window.location.href,
            source: 'network',
            metadata: { status: response.status, duration }
          });
        }

        return response;
      } catch (error) {
        emit('DEV_CONSOLE_LOG', {
          level: 'error',
          message: `${method.toUpperCase()} ${url} → Network Error: ${error.message}`,
          timestamp: new Date().toISOString(),
          url: window.location.href,
          source: 'network',
          metadata: { error: error.message, duration: Date.now() - startTime }
        });
        throw error;
      }
    };
  }

  // === WebSocket Capture ===
  function installWebSocketCapture() {
    const OriginalWebSocket = window.WebSocket;

    window.WebSocket = function(url, protocols) {
      const ws = new OriginalWebSocket(url, protocols);
      const id = (typeof crypto !== 'undefined' && crypto.randomUUID)
        ? crypto.randomUUID()
        : 'ws_' + Date.now() + '_' + Math.random().toString(36).slice(2);

      emit('GASOLINE_WS', { event: 'connecting', id, url, ts: new Date().toISOString() });

      ws.addEventListener('open', () => {
        emit('GASOLINE_WS', { event: 'open', id, url, ts: new Date().toISOString() });
      });

      ws.addEventListener('message', (event) => {
        const data = typeof event.data === 'string' ? event.data : '[binary]';
        emit('GASOLINE_WS', {
          event: 'message',
          id, url,
          direction: 'incoming',
          data: data.slice(0, MAX_STRING_LENGTH),
          size: event.data.length || 0,
          ts: new Date().toISOString()
        });
      });

      ws.addEventListener('close', (event) => {
        emit('GASOLINE_WS', {
          event: 'close', id, url,
          code: event.code,
          reason: event.reason,
          ts: new Date().toISOString()
        });
      });

      ws.addEventListener('error', () => {
        emit('GASOLINE_WS', { event: 'error', id, url, ts: new Date().toISOString() });
      });

      // Wrap send
      const originalSend = ws.send.bind(ws);
      ws.send = function(data) {
        const payload = typeof data === 'string' ? data : '[binary]';
        emit('GASOLINE_WS', {
          event: 'message',
          id, url,
          direction: 'outgoing',
          data: payload.slice(0, MAX_STRING_LENGTH),
          size: data.length || 0,
          ts: new Date().toISOString()
        });
        return originalSend(data);
      };

      return ws;
    };

    // Preserve static properties
    window.WebSocket.CONNECTING = OriginalWebSocket.CONNECTING;
    window.WebSocket.OPEN = OriginalWebSocket.OPEN;
    window.WebSocket.CLOSING = OriginalWebSocket.CLOSING;
    window.WebSocket.CLOSED = OriginalWebSocket.CLOSED;
    window.WebSocket.prototype = OriginalWebSocket.prototype;
  }

  // === Helpers ===
  function filterHeaders(headers) {
    if (!headers) return {};
    const filtered = {};
    const entries = headers instanceof Headers
      ? Array.from(headers.entries())
      : Object.entries(headers);
    for (const [key, value] of entries) {
      if (SENSITIVE_HEADERS.includes(key.toLowerCase())) {
        filtered[key] = '[REDACTED]';
      } else {
        filtered[key] = value;
      }
    }
    return filtered;
  }

  // === Lifecycle Hooks ===

  // Flush on page unload
  window.addEventListener('beforeunload', () => flush());

  // Signal to server that this page loaded (useful for test correlation)
  sendToServer('/logs', {
    entries: [{
      level: 'info',
      message: '[gasoline-ci] Capture initialized',
      timestamp: new Date().toISOString(),
      url: window.location.href,
      source: 'gasoline-ci',
      metadata: {
        testId: window.__GASOLINE_TEST_ID || null,
        captureVersion: '1.0.0'
      }
    }]
  });

  // === Install All Captures ===
  installConsoleCapture();
  installExceptionCapture();
  installNetworkCapture();
  installWebSocketCapture();

})();
```

### Build Process

The CI script is generated from `inject.js` via a build script that:

1. Extracts the pure-JS capture functions (`safeSerialize`, `installConsoleCapture`, etc.)
2. Replaces the `window.postMessage` transport with the HTTP batching layer
3. Removes Chrome extension-specific code (settings listener, `chrome.runtime.getURL`)
4. Wraps in IIFE for isolation
5. Adds the CI-specific configuration header

```bash
# Build command
node scripts/build-ci-script.js

# Outputs:
#   packages/gasoline-ci/gasoline-ci.js       (development, with comments)
#   packages/gasoline-ci/gasoline-ci.min.js   (production, minified)
```

### Build Script: `scripts/build-ci-script.js`

```javascript
#!/usr/bin/env node
/**
 * Extracts capture logic from inject.js and builds gasoline-ci.js
 *
 * This ensures CI and extension capture logic stays in sync.
 * Any divergence between inject.js and gasoline-ci.js is a bug.
 */

import { readFileSync, writeFileSync } from 'fs';
import { join } from 'path';

const INJECT_PATH = join(import.meta.dirname, '../extension/inject.js');
const OUTPUT_PATH = join(import.meta.dirname, '../packages/gasoline-ci/gasoline-ci.js');

// Extract function bodies by name
function extractFunction(source, name) {
  // ... implementation
}

// Build the CI script by composing extracted functions + CI transport
function build() {
  const injectSource = readFileSync(INJECT_PATH, 'utf-8');

  // Extract shared functions
  const safeSerialize = extractFunction(injectSource, 'safeSerialize');
  // ... etc

  // Compose with CI transport header + IIFE wrapper
  const output = composeScript(safeSerialize, /* ... */);

  writeFileSync(OUTPUT_PATH, output);
  console.log(`Built gasoline-ci.js (${output.length} bytes)`);
}

build();
```

### Sync Verification

A CI check ensures `gasoline-ci.js` stays in sync with `inject.js`:

```bash
# In CI pipeline:
node scripts/build-ci-script.js
git diff --exit-code packages/gasoline-ci/gasoline-ci.js || {
  echo "ERROR: gasoline-ci.js is out of sync with inject.js"
  echo "Run 'node scripts/build-ci-script.js' and commit the result"
  exit 1
}
```

---

## Component 2: Server Changes

### New Endpoint: `GET /snapshot`

Returns ALL current state in a single response — logs, WebSocket events, network bodies, and performance data. Designed for test teardown: grab everything that happened during a test.

```go
// GET /snapshot
// Query params:
//   since=<ISO8601>  - Only entries after this timestamp (optional)
//   test_id=<string> - Only entries with this test_id metadata (optional)

type SnapshotResponse struct {
    Timestamp      string                 `json:"timestamp"`
    TestID         string                 `json:"test_id,omitempty"`
    Logs           []LogEntry             `json:"logs"`
    WebSocket      []WebSocketEvent       `json:"websocket_events"`
    NetworkBodies  []NetworkBody          `json:"network_bodies"`
    EnhancedActions []EnhancedAction      `json:"enhanced_actions,omitempty"`
    Stats          SnapshotStats          `json:"stats"`
}

type SnapshotStats struct {
    TotalLogs        int `json:"total_logs"`
    ErrorCount       int `json:"error_count"`
    WarningCount     int `json:"warning_count"`
    NetworkFailures  int `json:"network_failures"` // status >= 400
    WSConnections    int `json:"ws_connections"`
}
```

#### Implementation

```go
func (s *Server) handleSnapshot(v4 *V4Server) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "GET" {
            jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
            return
        }

        since := r.URL.Query().Get("since")
        testID := r.URL.Query().Get("test_id")

        var sinceTime time.Time
        if since != "" {
            var err error
            sinceTime, err = time.Parse(time.RFC3339Nano, since)
            if err != nil {
                jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid since timestamp"})
                return
            }
        }

        // Gather logs
        logs := s.getEntries()
        if !sinceTime.IsZero() {
            logs = filterEntriesSince(logs, sinceTime)
        }
        if testID != "" {
            logs = filterEntriesByTestID(logs, testID)
        }

        // Gather v4 data
        var wsEvents []WebSocketEvent
        var networkBodies []NetworkBody
        var enhancedActions []EnhancedAction
        if v4 != nil {
            wsEvents = v4.getWebSocketEvents(WebSocketEventFilter{})
            networkBodies = v4.getNetworkBodies(NetworkBodyFilter{})
            enhancedActions = v4.getEnhancedActions()
        }

        // Compute stats
        stats := computeStats(logs, wsEvents, networkBodies)

        snapshot := SnapshotResponse{
            Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
            TestID:          testID,
            Logs:            logs,
            WebSocket:       wsEvents,
            NetworkBodies:   networkBodies,
            EnhancedActions: enhancedActions,
            Stats:           stats,
        }

        jsonResponse(w, http.StatusOK, snapshot)
    }
}
```

### New Endpoint: `DELETE /clear` (or `POST /clear`)

Resets all buffers. Called between tests to ensure isolation.

```go
// POST /clear
// Body (optional):
//   { "preserve_config": true }  - Keep noise rules/settings, clear data only
//
// Response:
//   { "cleared": true, "entries_removed": 47 }

func (s *Server) handleClear(v4 *V4Server) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" && r.Method != "DELETE" {
            jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
            return
        }

        previousCount := s.getEntryCount()
        s.clearEntries()

        if v4 != nil {
            v4.clearAll()
        }

        jsonResponse(w, http.StatusOK, map[string]interface{}{
            "cleared":         true,
            "entries_removed": previousCount,
        })
    }
}

// V4Server.clearAll resets all v4 buffers
func (v4 *V4Server) clearAll() {
    v4.mu.Lock()
    defer v4.mu.Unlock()

    v4.wsEvents = v4.wsEvents[:0]
    v4.networkBodies = v4.networkBodies[:0]
    v4.enhancedActions = v4.enhancedActions[:0]
    v4.connections = make(map[string]*connectionState)
    v4.closedConns = v4.closedConns[:0]
    v4.connOrder = v4.connOrder[:0]
}
```

### New Endpoint: `POST /test-boundary`

Marks test boundaries for correlation. The server tags all subsequent entries with the test ID until the next boundary.

```go
// POST /test-boundary
// Body:
//   { "test_id": "login-flow-happy-path", "action": "start" | "end" }
//
// Response:
//   { "test_id": "login-flow-happy-path", "action": "start", "timestamp": "..." }

type TestBoundary struct {
    TestID    string    `json:"test_id"`
    Action    string    `json:"action"` // "start" or "end"
    Timestamp time.Time `json:"timestamp"`
}

// Server tracks current test context
type Server struct {
    // ... existing fields ...
    currentTestID string // Set by /test-boundary start, cleared by end
    testMu        sync.RWMutex
}
```

### Route Registration

```go
func setupHTTPRoutes(server *Server, v4 *V4Server) {
    // ... existing routes ...

    // CI-specific routes
    http.HandleFunc("/snapshot", corsMiddleware(server.handleSnapshot(v4)))
    http.HandleFunc("/clear", corsMiddleware(server.handleClear(v4)))
    http.HandleFunc("/test-boundary", corsMiddleware(server.handleTestBoundary))
}
```

### Backwards Compatibility

These new endpoints are purely additive:
- No existing endpoint behavior changes
- No existing types change
- The `/logs` DELETE method already clears logs (preserved)
- New endpoints return 404 on older servers (clients can feature-detect)

---

## Component 3: `@gasoline/playwright` Fixture Package

### Purpose

A Playwright fixture that automatically:
1. Starts the Gasoline server before the test suite
2. Injects `gasoline-ci.js` into every page
3. Clears state between tests
4. Captures snapshots on failure
5. Attaches browser state to test reports

### Package: `packages/gasoline-playwright/`

```
packages/gasoline-playwright/
├── package.json
├── index.ts          # Fixture definition
├── reporter.ts       # Custom reporter (optional)
├── types.ts          # TypeScript types
└── README.md
```

### `package.json`

```json
{
  "name": "@anthropic/gasoline-playwright",
  "version": "1.0.0",
  "description": "Playwright fixture for Gasoline CI browser observability",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "peerDependencies": {
    "@playwright/test": ">=1.40.0"
  },
  "dependencies": {
    "@anthropic/gasoline-ci": "^1.0.0"
  }
}
```

### `index.ts` — Fixture Definition

```typescript
import { test as base, expect, Page, TestInfo } from '@playwright/test';
import { ChildProcess, spawn } from 'child_process';
import { resolve } from 'path';

export interface GasolineOptions {
  /** Port for the Gasoline server (default: 7890) */
  gasolinePort: number;
  /** Whether to start the server automatically (default: true) */
  gasolineAutoStart: boolean;
  /** Path to the gasoline binary (default: uses npx) */
  gasolineBinary: string;
  /** Capture mode: 'errors-only' | 'all' (default: 'errors-only') */
  gasolineCaptureMode: 'errors-only' | 'all';
  /** Whether to attach snapshot to report on failure (default: true) */
  gasolineAttachOnFailure: boolean;
}

export interface GasolineFixture {
  /** Get current snapshot of all captured data */
  getSnapshot: (since?: string) => Promise<GasolineSnapshot>;
  /** Clear all captured data (call between tests) */
  clear: () => Promise<void>;
  /** Mark test boundary for correlation */
  markTest: (testId: string, action: 'start' | 'end') => Promise<void>;
}

export interface GasolineSnapshot {
  timestamp: string;
  logs: any[];
  websocket_events: any[];
  network_bodies: any[];
  stats: {
    total_logs: number;
    error_count: number;
    warning_count: number;
    network_failures: number;
    ws_connections: number;
  };
}

// Fixtures
export const test = base.extend<GasolineOptions & { gasoline: GasolineFixture }>({
  gasolinePort: [7890, { option: true }],
  gasolineAutoStart: [true, { option: true }],
  gasolineBinary: ['npx gasoline-mcp', { option: true }],
  gasolineCaptureMode: ['errors-only', { option: true }],
  gasolineAttachOnFailure: [true, { option: true }],

  gasoline: async ({ gasolinePort, gasolineAutoStart, gasolineAttachOnFailure, page }, use, testInfo) => {
    const baseUrl = `http://127.0.0.1:${gasolinePort}`;

    // Inject capture script into every page
    await page.addInitScript({
      path: require.resolve('@anthropic/gasoline-ci')
    });

    // Mark test start
    const testId = `${testInfo.titlePath.join(' > ')}`;
    await fetch(`${baseUrl}/test-boundary`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ test_id: testId, action: 'start' })
    }).catch(() => {}); // Server might not be running yet

    const fixture: GasolineFixture = {
      getSnapshot: async (since?: string) => {
        const params = new URLSearchParams();
        if (since) params.set('since', since);
        params.set('test_id', testId);
        const res = await fetch(`${baseUrl}/snapshot?${params}`);
        return res.json();
      },
      clear: async () => {
        await fetch(`${baseUrl}/clear`, { method: 'POST' });
      },
      markTest: async (id: string, action: 'start' | 'end') => {
        await fetch(`${baseUrl}/test-boundary`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ test_id: id, action })
        });
      }
    };

    await use(fixture);

    // After test: if failed and attachOnFailure, grab snapshot
    if (testInfo.status !== testInfo.expectedStatus && gasolineAttachOnFailure) {
      const snapshot = await fixture.getSnapshot();

      // Attach to Playwright report
      await testInfo.attach('gasoline-snapshot', {
        body: JSON.stringify(snapshot, null, 2),
        contentType: 'application/json'
      });

      // Also attach a human-readable summary
      const summary = formatFailureSummary(snapshot);
      await testInfo.attach('gasoline-summary', {
        body: summary,
        contentType: 'text/plain'
      });
    }

    // Mark test end
    await fetch(`${baseUrl}/test-boundary`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ test_id: testId, action: 'end' })
    }).catch(() => {});

    // Clear between tests
    await fixture.clear();
  }
});

function formatFailureSummary(snapshot: GasolineSnapshot): string {
  const lines: string[] = [
    `=== Gasoline Failure Context ===`,
    `Captured at: ${snapshot.timestamp}`,
    ``,
    `--- Stats ---`,
    `Total logs: ${snapshot.stats.total_logs}`,
    `Errors: ${snapshot.stats.error_count}`,
    `Warnings: ${snapshot.stats.warning_count}`,
    `Network failures: ${snapshot.stats.network_failures}`,
    `WebSocket connections: ${snapshot.stats.ws_connections}`,
  ];

  if (snapshot.stats.error_count > 0) {
    lines.push('', '--- Errors ---');
    for (const log of snapshot.logs.filter(l => l.level === 'error')) {
      lines.push(`  [${log.source}] ${log.message}`);
      if (log.stack) lines.push(`    ${log.stack.split('\n')[0]}`);
    }
  }

  if (snapshot.stats.network_failures > 0) {
    lines.push('', '--- Network Failures ---');
    for (const body of snapshot.network_bodies.filter(b => b.status >= 400)) {
      lines.push(`  ${body.method} ${body.url} → ${body.status}`);
      if (body.responseBody) {
        lines.push(`    ${body.responseBody.slice(0, 200)}`);
      }
    }
  }

  return lines.join('\n');
}

export { expect };
```

### Usage in Tests

```typescript
// playwright.config.ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  use: {
    gasolinePort: 7890,
    gasolineAutoStart: true,
    gasolineAttachOnFailure: true,
  },
});

// tests/checkout.spec.ts
import { test, expect } from '@anthropic/gasoline-playwright';

test('checkout flow completes', async ({ page, gasoline }) => {
  await page.goto('/checkout');
  await page.fill('#email', 'test@example.com');
  await page.click('#submit');

  // Wait for success
  await expect(page.locator('.success')).toBeVisible();

  // Optional: assert no errors occurred
  const snapshot = await gasoline.getSnapshot();
  expect(snapshot.stats.error_count).toBe(0);
  expect(snapshot.stats.network_failures).toBe(0);
});
```

---

## Component 4: `gasoline-report` CLI Tool

### Purpose

Post-run analysis tool that reads Gasoline snapshots and produces:
- Human-readable failure reports
- AI-consumable context for automated triage
- JUnit/JSON output for CI dashboards

### Usage

```bash
# Basic: analyze failures from last test run
gasoline-report --format=text

# JSON output for CI integration
gasoline-report --format=json --output=gasoline-results.json

# AI-friendly output (designed for LLM consumption)
gasoline-report --format=ai-context --test-id="checkout flow"

# Pipe to AI for automated diagnosis
gasoline-report --format=ai-context | claude-code --message "Diagnose these test failures"
```

### Implementation (Go, part of main binary)

The reporter is a subcommand of the gasoline binary:

```bash
gasoline report [options]
```

```go
// cmd/dev-console/report.go

type ReportConfig struct {
    Format   string // "text", "json", "ai-context", "junit"
    Output   string // file path or "-" for stdout
    TestID   string // filter to specific test
    Since    string // ISO8601 timestamp
    Port     int    // server port to query
    Severity string // minimum severity: "error", "warn", "info"
}

type ReportEntry struct {
    TestID      string          `json:"test_id"`
    Status      string          `json:"status"` // "pass", "fail", "error"
    Errors      []ReportError   `json:"errors,omitempty"`
    NetworkFails []ReportNetwork `json:"network_failures,omitempty"`
    WSErrors    []ReportWS      `json:"ws_errors,omitempty"`
    Duration    string          `json:"duration,omitempty"`
}

func runReport(cfg ReportConfig) error {
    // 1. Query server for snapshot
    snapshot, err := fetchSnapshot(cfg.Port, cfg.Since, cfg.TestID)
    if err != nil {
        return fmt.Errorf("failed to fetch snapshot: %w", err)
    }

    // 2. Classify and group entries
    report := classifyEntries(snapshot)

    // 3. Format output
    switch cfg.Format {
    case "text":
        return formatText(report, cfg.Output)
    case "json":
        return formatJSON(report, cfg.Output)
    case "ai-context":
        return formatAIContext(report, cfg.Output)
    case "junit":
        return formatJUnit(report, cfg.Output)
    default:
        return fmt.Errorf("unknown format: %s", cfg.Format)
    }
}
```

### AI Context Format

The `ai-context` format is designed to be token-efficient while providing maximum diagnostic value:

```
## Test Failure: checkout flow completes
### Browser Errors (3)
1. [exception] TypeError: Cannot read property 'id' of null
   at CheckoutForm.submit (checkout.tsx:42)
   at HTMLFormElement.onSubmit (checkout.tsx:15)

2. [network] POST /api/orders → 500
   Request: {"items":[{"id":1,"qty":2}],"email":"test@example.com"}
   Response: {"error":"Internal Server Error","details":"null pointer: user.address"}

3. [network] POST /api/analytics → 0 (network error)
   [likely noise: analytics endpoint, non-critical]

### Network Timeline
  0ms: GET /checkout → 200 (42ms)
 50ms: GET /api/cart → 200 (89ms)
120ms: POST /api/orders → 500 (234ms) ← FAILURE
350ms: POST /api/analytics → 0 (timeout)

### Diagnosis Hints
- Primary failure: POST /api/orders returned 500
- Root cause likely: "null pointer: user.address" — user address data missing
- The analytics error is unrelated (network timeout on non-critical endpoint)
```

---

## Component 5: GitHub Action

### Purpose

Wraps the Gasoline server lifecycle and reporter for GitHub Actions CI:

```yaml
# .github/workflows/e2e.yml
name: E2E Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: anthropic/gasoline-ci@v1
        with:
          port: 7890

      - run: npx playwright test

      - uses: anthropic/gasoline-ci/report@v1
        if: failure()
        with:
          format: ai-context
          output: gasoline-report.md

      - uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: gasoline-report
          path: gasoline-report.md
```

### Action: `anthropic/gasoline-ci`

```yaml
# action.yml
name: 'Gasoline CI'
description: 'Start Gasoline server for browser observability in CI'
inputs:
  port:
    description: 'Server port'
    default: '7890'
  version:
    description: 'Gasoline version'
    default: 'latest'
bruns:
  using: 'composite'
  steps:
    - name: Install Gasoline
      shell: bash
      run: npm install -g gasoline-mcp@${{ inputs.version }}

    - name: Start server
      shell: bash
      run: |
        gasoline --port=${{ inputs.port }} &
        echo "GASOLINE_PID=$!" >> $GITHUB_ENV
        # Wait for server to be ready
        for i in {1..30}; do
          curl -s http://127.0.0.1:${{ inputs.port }}/health && break
          sleep 0.1
        done
```

### Action: `anthropic/gasoline-ci/report`

```yaml
# report/action.yml
name: 'Gasoline Report'
description: 'Generate failure report from Gasoline CI data'
inputs:
  format:
    description: 'Report format: text, json, ai-context, junit'
    default: 'ai-context'
  output:
    description: 'Output file path'
    default: 'gasoline-report.md'
  port:
    description: 'Server port'
    default: '7890'
runs:
  using: 'composite'
  steps:
    - shell: bash
      run: gasoline report --format=${{ inputs.format }} --output=${{ inputs.output }} --port=${{ inputs.port }}
```

---

## Component 6: Extension Loading in CI

### Purpose

For teams that want exact parity with local development — the same extension running in CI headless Chrome.

### Playwright Configuration

```typescript
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';
import { resolve } from 'path';

export default defineConfig({
  use: {
    // Load Gasoline extension in CI
    ...devices['Desktop Chrome'],
    launchOptions: {
      args: [
        `--load-extension=${resolve('./node_modules/gasoline-mcp/extension')}`,
        '--disable-extensions-except=' + resolve('./node_modules/gasoline-mcp/extension'),
      ],
    },
    // Must use non-headless for extensions (Playwright limitation)
    headless: false,
  },
  // Use xvfb-run on Linux CI for headless-like behavior
  // xvfb-run npx playwright test
});
```

### CI Considerations

| Concern | Solution |
|---------|----------|
| Extensions require non-headless | Use `xvfb-run` on Linux CI |
| Extension needs to find server | Same `127.0.0.1:7890` — no config needed |
| Extension popup not tested | Popup is irrelevant in CI |
| Extension permissions | `--load-extension` grants all manifest permissions |
| Performance overhead | Extension adds < 5ms per page load — negligible in E2E |

### GitHub Actions with xvfb

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: anthropic/gasoline-ci@v1

      - name: Install xvfb
        run: sudo apt-get install -y xvfb

      - name: Run tests with extension
        run: xvfb-run npx playwright test
```

---

## Backwards Compatibility Guarantees

### Principle: Additive Only

Gasoline CI MUST NOT change any existing behavior. The extension and server must work identically whether or not CI features are present.

### Server Compatibility Matrix

| Change | Existing Behavior | CI Behavior | Risk |
|--------|-------------------|-------------|------|
| New `/snapshot` endpoint | Returns 404 (doesn't exist) | Returns aggregated state | **None** — new endpoint |
| New `/clear` endpoint | Use `DELETE /logs` | Clears all buffers at once | **None** — new endpoint, DELETE /logs still works |
| New `/test-boundary` endpoint | N/A | Marks test correlation | **None** — new endpoint |
| Existing `POST /logs` | Accepts entries array | Same — CI script uses same format | **None** — same contract |
| Existing `POST /websocket-events` | Accepts events array | Same — CI script uses same format | **None** — same contract |
| Existing `POST /network-bodies` | Accepts bodies array | Same — CI script uses same format | **None** — same contract |
| Existing `GET /health` | Returns status + entry count | Same | **None** — unchanged |
| Existing MCP tools | Work via stdio and HTTP | Same — available regardless of data source | **None** — unchanged |

### Extension Compatibility

| Change | Risk | Mitigation |
|--------|------|-----------|
| No extension code changes | None | CI uses separate `gasoline-ci.js`, not the extension |
| Extension in CI via `--load-extension` | Minimal | Standard Chrome extension loading, no modification |
| Same server, two data sources | Low | Server already handles concurrent POST from any source |

### Data Format Compatibility

The CI capture script produces the exact same JSON payloads as the extension:

```javascript
// Extension's background.js sends:
{ "entries": [{ "level": "error", "message": "...", "timestamp": "...", "url": "..." }] }

// gasoline-ci.js sends identical format:
{ "entries": [{ "level": "error", "message": "...", "timestamp": "...", "url": "..." }] }
```

### Test Strategy for Compatibility

```go
// main_test.go additions

func TestSnapshotEndpoint_NewRoute(t *testing.T) {
    // Verify /snapshot exists and returns correct structure
}

func TestClearEndpoint_ClearsAllBuffers(t *testing.T) {
    // Verify /clear resets both v3 logs and v4 buffers
}

func TestClearEndpoint_ExistingDeleteLogsStillWorks(t *testing.T) {
    // Verify DELETE /logs still works independently
}

func TestSnapshotEndpoint_ReturnsEmptyOnFreshServer(t *testing.T) {
    // Verify no crash on empty state
}

func TestTestBoundary_CorrelatesEntries(t *testing.T) {
    // Verify entries are tagged with test_id
}

func TestCIAndExtension_ConcurrentWrites(t *testing.T) {
    // Simulate CI script and extension writing simultaneously
    // Verify no data loss, no race conditions
}

func TestExistingEndpoints_UnchangedBehavior(t *testing.T) {
    // Run all existing endpoint tests to verify no regression
    // This is the most critical test — run the entire existing test suite
}
```

```javascript
// extension-tests/ci-compat.test.js

import { test } from 'node:test';
import assert from 'node:assert';

test('gasoline-ci.js produces same payload format as extension', () => {
  // Compare output schemas
});

test('extension message types match CI emit types', () => {
  // DEV_CONSOLE_LOG, GASOLINE_WS, GASOLINE_NETWORK_BODY all handled
});

test('safeSerialize output matches between extension and CI', () => {
  // Same edge cases handled identically
});
```

---

## Performance Budgets

### CI Capture Script

| Metric | Budget | Rationale |
|--------|--------|-----------|
| Script injection overhead | < 5ms | Must not slow page load in tests |
| Per-console intercept | < 0.1ms | Same as extension budget |
| Per-fetch intercept | < 0.5ms | Async clone, no blocking |
| Batch flush interval | 100ms | Balance between latency and server load |
| Max memory (script) | < 5MB | Ring buffers, not unbounded arrays |
| Beacon/fetch fire-and-forget | < 1ms | Never blocks test execution |

### Server Under CI Load

| Metric | Budget | Rationale |
|--------|--------|-----------|
| POST /logs throughput | > 1000 entries/sec | Handles fast test suites |
| GET /snapshot latency | < 50ms for 1000 entries | Fast enough for per-test teardown |
| POST /clear latency | < 10ms | Near-instant reset |
| Concurrent test workers | Up to 10 | Playwright's default parallel workers |
| Memory under CI load | < 100MB | 10 workers × max entries |

### Race Condition Safety

The server already uses `sync.RWMutex` for all buffer access. CI introduces no new concurrency patterns:

- Multiple test workers POST to the same server: already handled by HTTP server goroutines
- `/snapshot` reads while workers write: RLock for snapshot, Lock for writes
- `/clear` during active writes: Lock ensures atomicity

---

## Testing Strategy

### Unit Tests (Go)

```go
// cmd/dev-console/ci_test.go

func TestSnapshotEndpoint(t *testing.T) {
    t.Run("returns empty snapshot on fresh server", func(t *testing.T) { ... })
    t.Run("returns all entry types", func(t *testing.T) { ... })
    t.Run("filters by since timestamp", func(t *testing.T) { ... })
    t.Run("filters by test_id", func(t *testing.T) { ... })
    t.Run("stats are computed correctly", func(t *testing.T) { ... })
    t.Run("concurrent reads during writes", func(t *testing.T) { ... })
}

func TestClearEndpoint(t *testing.T) {
    t.Run("clears all buffers", func(t *testing.T) { ... })
    t.Run("returns entry count before clear", func(t *testing.T) { ... })
    t.Run("existing DELETE /logs still works", func(t *testing.T) { ... })
    t.Run("concurrent clear and write", func(t *testing.T) { ... })
}

func TestTestBoundary(t *testing.T) {
    t.Run("marks test start", func(t *testing.T) { ... })
    t.Run("marks test end", func(t *testing.T) { ... })
    t.Run("entries between boundaries get test_id", func(t *testing.T) { ... })
    t.Run("overlapping test boundaries", func(t *testing.T) { ... })
}

func TestReportCommand(t *testing.T) {
    t.Run("text format output", func(t *testing.T) { ... })
    t.Run("json format output", func(t *testing.T) { ... })
    t.Run("ai-context format output", func(t *testing.T) { ... })
    t.Run("junit format output", func(t *testing.T) { ... })
    t.Run("filters by severity", func(t *testing.T) { ... })
}
```

### Unit Tests (JavaScript)

```javascript
// extension-tests/gasoline-ci.test.js

import { test } from 'node:test';
import assert from 'node:assert';

test('gasoline-ci.js batching', async (t) => {
  await t.test('batches entries until flush interval', () => { ... });
  await t.test('flushes immediately at MAX_BATCH_SIZE', () => { ... });
  await t.test('flushes on beforeunload', () => { ... });
});

test('gasoline-ci.js capture parity', async (t) => {
  await t.test('console.error produces same payload as extension', () => { ... });
  await t.test('fetch 500 produces same payload as extension', () => { ... });
  await t.test('WebSocket message produces same payload as extension', () => { ... });
  await t.test('unhandled exception produces same payload as extension', () => { ... });
});

test('gasoline-ci.js isolation', async (t) => {
  await t.test('does not leak into global scope', () => { ... });
  await t.test('does not interfere with page fetch', () => { ... });
  await t.test('does not interfere with page WebSocket', () => { ... });
  await t.test('swallows server connection errors silently', () => { ... });
});
```

### Integration Tests (E2E)

```javascript
// e2e-tests/ci-integration.test.js

import { test, expect } from '@playwright/test';

test.describe('Gasoline CI Integration', () => {
  test('captures console errors via script injection', async ({ page }) => {
    await page.addInitScript({ path: './packages/gasoline-ci/gasoline-ci.js' });
    await page.goto('data:text/html,<script>console.error("test error")</script>');
    await page.waitForTimeout(200); // Wait for batch flush

    const res = await fetch('http://127.0.0.1:7890/snapshot');
    const snapshot = await res.json();
    expect(snapshot.logs.some(l => l.message.includes('test error'))).toBe(true);
  });

  test('captures network failures', async ({ page }) => {
    await page.addInitScript({ path: './packages/gasoline-ci/gasoline-ci.js' });
    await page.goto('data:text/html,<script>fetch("/api/missing")</script>');
    await page.waitForTimeout(200);

    const res = await fetch('http://127.0.0.1:7890/snapshot');
    const snapshot = await res.json();
    expect(snapshot.stats.network_failures).toBeGreaterThan(0);
  });

  test('/clear resets state between tests', async ({ page }) => {
    await fetch('http://127.0.0.1:7890/logs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entries: [{ level: 'error', message: 'old' }] })
    });

    await fetch('http://127.0.0.1:7890/clear', { method: 'POST' });

    const res = await fetch('http://127.0.0.1:7890/snapshot');
    const snapshot = await res.json();
    expect(snapshot.stats.total_logs).toBe(0);
  });

  test('existing extension behavior unaffected', async ({ page }) => {
    // Load extension via --load-extension
    // Verify it still posts to /logs in the same format
    // Verify MCP tools still work
  });
});
```

---

## Security Considerations

### CI-Specific Risks

| Risk | Mitigation |
|------|-----------|
| Server exposed on CI runner | Binds to `127.0.0.1` only — same as local dev |
| Sensitive data in test logs | Same `SENSITIVE_HEADERS` filtering as extension |
| Report files contain secrets | `gasoline-report` inherits header redaction |
| Snapshot data in artifacts | Only attached on failure; teams opt-in |
| CI script injection (supply chain) | `gasoline-ci.js` is built from auditable `inject.js` |

### Auth Header Handling

Both the extension and `gasoline-ci.js` use the same `SENSITIVE_HEADERS` list:

```javascript
const SENSITIVE_HEADERS = ['authorization', 'cookie', 'set-cookie', 'x-auth-token'];
```

These headers are replaced with `[REDACTED]` in captured network bodies. This behavior is identical in local dev and CI — no configuration needed.

---

## Rollout Plan

### Phase 1: Server Endpoints

1. Add `/snapshot` endpoint (read-only, aggregates existing data)
2. Add `/clear` endpoint (resets all buffers)
3. Add `/test-boundary` endpoint (test correlation)
4. Full test coverage for new endpoints
5. Verify all existing tests still pass (regression gate)

### Phase 2: Capture Script

1. Create `scripts/build-ci-script.js` (extract from inject.js)
2. Generate `packages/gasoline-ci/gasoline-ci.js`
3. Add sync verification to CI
4. Unit tests for batching and payload parity
5. Integration test: script → server → snapshot

### Phase 3: Playwright Fixture

1. Create `packages/gasoline-playwright/` package
2. Implement fixture with auto-inject, clear, attach
3. Integration tests with real Playwright
4. Documentation and examples

### Phase 4: Reporter + GitHub Action

1. Add `report` subcommand to gasoline binary
2. Implement text, JSON, ai-context, JUnit formats
3. Create GitHub Action wrappers
4. End-to-end test: full CI pipeline

---

## Open Questions

1. **CDP capture option:** Should we provide a CDP-based capture for Selenium/WebDriver users, or is script injection sufficient for v1?
2. **Multi-worker correlation:** When Playwright runs 10 workers against one server, should `/test-boundary` support concurrent test IDs, or should each worker use its own port?
3. **Streaming vs. polling:** Should the reporter stream results in real-time during the test run, or only analyze post-hoc?
4. **Package naming:** `@anthropic/gasoline-ci` or `gasoline-ci` (unscoped)?

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Time to diagnose CI failure | 50% reduction | Compare triage time before/after Gasoline CI |
| False-positive test reruns | 30% reduction | Track reruns that pass on retry (flaky due to timing, not bugs) |
| Token efficiency for AI analysis | < 500 tokens per failure | Measure ai-context output size |
| Server stability under CI load | Zero crashes in 1000 test runs | Soak test with parallel workers |
| Payload parity with extension | 100% schema match | Automated comparison in CI |
| Existing test suite regression | Zero failures | Run full suite after every change |
