# Gasoline Architecture

## System Overview

Gasoline is a two-component system: a **Chrome Extension** that captures browser state and a **Go Server** that stores data and exposes it to AI assistants via the Model Context Protocol (MCP).

```
┌──────────────────────────────────────────────────────────────┐
│                          Browser                              │
│                                                               │
│  inject.js (page context)                                     │
│    ├── Console capture (log, warn, error, info, debug)        │
│    ├── Network error capture (fetch/XHR 4xx/5xx)              │
│    ├── Exception capture (onerror, onunhandledrejection)       │
│    ├── WebSocket monitoring (v4)                              │
│    ├── Network body capture (v4)                              │
│    ├── DOM query execution (v4, on-demand)                    │
│    └── Accessibility audit (v4, on-demand)                    │
│         │                                                     │
│         │ window.postMessage                                  │
│         ▼                                                     │
│  content.js (content script context)                          │
│    └── Routes messages between inject.js and background.js    │
│         │                                                     │
│         │ chrome.runtime.sendMessage                          │
│         ▼                                                     │
│  background.js (service worker)                               │
│    ├── Log batching and debounce                              │
│    ├── Server health checking                                 │
│    ├── Badge updates                                          │
│    ├── Pending query polling (v4)                             │
│    └── Error grouping                                         │
│                                                               │
└────────────┬──────────────────────────────────────────────────┘
             │ HTTP (localhost:7890)
             ▼
┌──────────────────────────────────────────────────────────────┐
│                    Gasoline Server (Go)                        │
│                                                               │
│  HTTP Endpoints:                                              │
│    GET  /health              Server status and stats          │
│    POST /logs                Receive log entries               │
│    DELETE /logs              Clear all logs                    │
│    POST /websocket-events    Receive WS events (v4)           │
│    POST /network-bodies      Receive network bodies (v4)      │
│    GET  /pending-queries     Extension polls for queries (v4)  │
│    POST /dom-result          Extension posts DOM result (v4)   │
│    POST /a11y-result         Extension posts a11y result (v4)  │
│                                                               │
│  MCP Protocol (stdin/stdout, JSON-RPC 2.0):                   │
│    get_browser_errors        Filter error-level entries        │
│    get_browser_logs          Return all log entries            │
│    clear_browser_logs        Clear log buffer                  │
│    get_websocket_events      Return WS events (v4)            │
│    get_websocket_status      Return connection states (v4)     │
│    get_network_bodies        Return network bodies (v4)        │
│    query_dom                 On-demand DOM query (v4)          │
│    get_page_info             On-demand page summary (v4)       │
│    run_accessibility_audit   On-demand a11y audit (v4)         │
│                                                               │
│  Storage:                                                     │
│    In-memory ring buffers (concurrent-safe via RWMutex)        │
│    JSONL file persistence (~/gasoline-logs.jsonl)              │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

## Execution Modes

The Go binary operates in three modes:

1. **MCP Mode** (default when stdin is piped): HTTP server in background goroutine, MCP protocol on stdin/stdout. Used by Claude Code, Cursor, etc.
2. **Server Mode** (`--server` flag): Foreground HTTP server with status banner. Used for development.
3. **Daemon Mode** (default when stdin is TTY): Spawns HTTP server as background process, prints PID, exits.

## Data Flow: Passive Capture

```
Page Event → inject.js (serialize) → postMessage → content.js →
  chrome.runtime.sendMessage → background.js (batch) →
  HTTP POST /logs → Server (store in ring buffer + write JSONL)
```

## Data Flow: On-Demand Query (v4)

```
AI calls MCP tool → Server creates pending query →
  Extension polls GET /pending-queries → finds query →
  content.js executes in page context →
  Extension POSTs result to /dom-result or /a11y-result →
  Server returns result to MCP caller
```

## Memory Management

All buffers are bounded by both entry count AND memory:

| Buffer | Max Entries | Max Memory | Eviction |
|--------|------------|------------|----------|
| Log entries | 1000 (default) | N/A | Oldest first |
| WebSocket events | 500 | 4MB | Oldest first |
| Network bodies | 100 | 8MB | Oldest first |
| Connection tracker | 20 active + 10 closed | 2MB | Oldest closed first |
| Pending queries | 5 | 1KB | Oldest first |

## Concurrency Model

- Server uses `sync.RWMutex` for all shared state
- Extension uses single-threaded JS event loop (no concurrency issues)
- Background service worker handles one message at a time
- On-demand queries block MCP caller until result or timeout (10s)

## Security Model

- Server binds to `127.0.0.1` only (never exposed to network)
- Extension strips sensitive headers (Authorization, Cookie, tokens)
- Network body capture is OFF by default (opt-in)
- axe-core loads from extension bundle (no external network calls)
- All data stays on localhost (never sent to external services)

## Extension Permissions

- `activeTab` - Query active tab for on-demand features
- `scripting` - Dynamic script injection for axe-core
- `storage` - Persist user settings
- Host permission for localhost:7890 only

## Distribution

Go binary is compiled for 5 platforms and wrapped in NPM packages:
- `gasoline-mcp` (main package, installs platform-specific optional dep)
- `@nicepipes/gasoline-darwin-arm64`
- `@nicepipes/gasoline-darwin-x64`
- `@nicepipes/gasoline-linux-arm64`
- `@nicepipes/gasoline-linux-x64`
- `@nicepipes/gasoline-win32-x64`

Users install via `npx gasoline-mcp` in their MCP config.
