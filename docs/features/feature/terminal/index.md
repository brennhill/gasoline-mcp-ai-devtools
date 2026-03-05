---
doc_type: feature_index
feature_id: feature-terminal
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - cmd/dev-console/terminal_handlers.go
  - cmd/dev-console/terminal_server.go
  - cmd/dev-console/terminal_assets/terminal.html
  - extension/content/ui/terminal-widget.js
  - src/content/ui/terminal-widget.ts
  - src/content/ui/tracked-hover-launcher.ts
  - internal/pty/manager.go
  - internal/pty/session.go
test_paths:
  - cmd/dev-console/terminal_handlers_test.go
  - tests/extension/terminal-widget.test.js
  - internal/pty/manager_test.go
  - internal/pty/session_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Terminal

## TL;DR
- Status: shipped
- In-browser terminal widget that embeds a PTY-backed shell via iframe
- Runs on a **dedicated HTTP server** at `main_port + 1` (e.g., 7891) for isolation
- Singleton session shared across all tabs via `chrome.storage.session`
- Three UI states: **open**, **minimized**, **closed** — all persisted across page refreshes
- Header redraw control (`↻`) resets widget geometry and reloads iframe graphics without killing the PTY session
- Annotation auto-send now uses a typing-aware write queue: if the user is active in terminal, writes wait until ~1.5s idle
- Queued submit is reconnect-safe: if WS drops before Enter, submit waits until connection is back
- WebSocket frame writes are serialized per-connection to prevent concurrent writer frame interleaving
- Scrollback buffer capped at 256 KB for memory safety
- Canonical flow map: [terminal-server-isolation.md](../../../architecture/flow-maps/terminal-server-isolation.md)
- Feature flow-map pointer: [flow-map.md](./flow-map.md)

---

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)

## Architecture

### Dedicated Terminal Server

The terminal runs on its own `http.Server` on **port+1** (e.g., main daemon on 7890, terminal on 7891). This isolates:

- **Timeouts**: Main server has `WriteTimeout: 65s` for MCP blocking tools. Terminal server has `WriteTimeout: 0` for long-lived WebSocket connections.
- **Middleware**: Main server uses `AuthMiddleware` for API key validation. Terminal server uses its own session token validation (no AuthMiddleware).
- **Failure isolation**: If the main server has issues, the terminal keeps running. If the terminal server dies, the main daemon logs it but keeps serving MCP.

### Port Assignment

| Server | Port | Purpose |
|--------|------|---------|
| Main daemon | `PORT` (default 7890) | MCP, capture, health, diagnostics |
| Terminal | `PORT + 1` (default 7891) | Terminal HTML, static assets, WebSocket, session lifecycle |

The terminal port is surfaced in:
- `/health` HTTP response as `terminal_port`
- MCP `configure(what: "health")` response in `server.terminal_port`
- Startup lifecycle logs

### Port Conflict Handling

If port+1 is already in use at startup:
- **Logged loudly** to stderr with actionable instructions
- **Lifecycle event** `terminal_server_bind_failed` logged
- **Main daemon continues** — terminal is non-essential
- `/health` response omits `terminal_port` (signals terminal unavailable)
- MCP health returns `terminal_port: 0`

If the terminal server dies at runtime:
- Logged as `terminal_server_died`
- `terminal_port` set to 0
- Main daemon is **not** affected

---

## UI Widget State Machine

The widget has three visual states, tracked by the `TerminalUIState` type:

```
              toggleTerminal()                hideTerminal()
    ┌─────────────────────────┐      ┌───────────────────────────┐
    │                         ▼      │                           ▼
 CLOSED ──toggleTerminal()──► OPEN ──┤──toggleMinimize()──► MINIMIZED
    ▲                                │                           │
    │         exitTerminalSession()  │   toggleMinimize()        │
    └────────────────────────────────┘◄──────────────────────────┘
    ▲                                                            │
    └─────────exitTerminalSession()──────────────────────────────┘
```

### State Descriptions

| State | Visual | PTY Session | Persisted As |
|-------|--------|-------------|-------------|
| **Closed** | Widget hidden (opacity 0, no pointer events) | Stays alive — can reconnect | `'closed'` |
| **Open** | Full widget visible (iframe + header) | Active, WebSocket connected | `'open'` |
| **Minimized** | Only 32px header bar visible, iframe hidden | Active, WebSocket connected | `'minimized'` |

### State Transitions

| Action | Trigger | From → To |
|--------|---------|-----------|
| `toggleTerminal()` | Launcher button click | Closed → Open (starts session if needed) |
| `toggleTerminal()` | Launcher button click | Open → Closed |
| `hideTerminal()` | Close (✕) button | Open/Minimized → Closed |
| `toggleMinimize()` | Minimize (▁) button | Open → Minimized |
| `toggleMinimize()` | Header click or restore (□) button | Minimized → Open |
| `exitTerminalSession()` | Exit (⏻) button | Any → Closed (kills PTY) |
| `restoreTerminalIfNeeded()` | Page load | Restores previous state from persistence |

### Key Distinction: Close vs Exit

- **Close** (`hideTerminal`) — Hides the widget but the PTY session stays alive on the daemon. The user can reopen and reconnect. Used by the ✕ button.
- **Exit** (`exitTerminalSession`) — Kills the PTY process on the daemon (`POST /terminal/stop`), clears persisted session, and unmounts the widget completely. Used by the ⏻ button.

---

## Session Management

### Singleton Session Model

The terminal uses a **singleton session** — one PTY session shared across all tabs in the browser. This is because `chrome.storage.session` (where the session token is persisted) is scoped to the entire extension session, not per-tab.

### Storage Layers

| Storage | Scope | Keys | Purpose |
|---------|-------|------|---------|
| `chrome.storage.session` | Browser session (all tabs) | `TERMINAL_SESSION`, `TERMINAL_UI_STATE` | Active session token + UI state; clears on browser close |
| `chrome.storage.local` | Persistent (survives restart) | `TERMINAL_CONFIG`, `TERMINAL_AI_COMMAND`, `TERMINAL_DEV_ROOT` | User preferences: shell, AI command, dev root path |

### Session Token Flow

```
Extension                          Terminal Server (port+1)
   │                                        │
   ├─ POST /terminal/start ────────────────►│ Creates PTY, returns {session_id, token}
   │◄────────── {session_id, token} ────────┤
   │                                        │
   ├─ Persist token to chrome.storage.session
   │                                        │
   ├─ Open iframe: /terminal?token=...     │
   │     └─ iframe connects WS:            │
   │        /terminal/ws?token=... ────────►│ Validates token, upgrades to WebSocket
   │◄────────── scrollback replay ──────────┤
   │◄────────── live PTY I/O ──────────────►│
```

### Session Persistence Across Page Refresh

1. On every state change, the widget writes `{token, sessionId}` and `uiState` to `chrome.storage.session`.
2. On page load, `restoreTerminalIfNeeded()` reads the persisted state.
3. If a session exists and `uiState !== 'closed'`:
   - Validates the token against the daemon (`GET /terminal/validate?token=...`).
   - If valid: mounts the widget in the persisted UI state (open or minimized).
   - If invalid (daemon restarted, process died): clears stale state and starts a fresh session.

### Session Conflict (409)

If the client calls `POST /terminal/start` with an ID that already exists:
- Server returns HTTP 409 with the existing session's token.
- Client reconnects using the returned token instead of creating a new session.
- This prevents orphaned sessions from accumulating.

### CWD Priority

When starting a session, the working directory is resolved in this order:
1. `dir` from the request body (explicit)
2. `active_codebase` set via MCP/extension (`server.GetActiveCodebase()`)
3. Auto-detected from the first registered MCP client's CWD
4. Falls back to the daemon's working directory

---

## Scrollback and Memory

### Scrollback Buffer (Server-Side)

The daemon maintains a **256 KB ring buffer** per session (`session.go:maxScrollback`). Every byte read from the PTY is appended via `AppendScrollback()`. When the buffer exceeds 256 KB, the oldest bytes are evicted (trimmed from the front).

On WebSocket reconnect (page refresh), the entire scrollback buffer is replayed to the client in 4 KB chunks, so the user sees prior output immediately.

### xterm.js Scrollback (Client-Side)

The `terminal.html` xterm.js instance has `scrollback: 1500` lines. This is intentionally low — the terminal is for interactive use, not log viewing. The server-side 256 KB buffer handles reconnect replay, so the browser doesn't need to retain deep history. Combined:
- **Reconnect replay**: last 256 KB of raw terminal output (server-side)
- **In-session scroll**: last 1,500 lines of rendered text (browser-side)

### Memory Pressure

- Server-side: 256 KB per session is fixed and bounded. With the singleton model (one session), this is negligible.
- Client-side: xterm.js manages its own memory. The 10,000-line scrollback is the main consumer.
- The WebSocket idle timeout (`terminalWSIdleTimeout = 5 minutes`) closes stale connections that stop sending data, preventing resource leaks.

---

## WebSocket Protocol

### Frame Types

| Direction | Opcode | Content |
|-----------|--------|---------|
| PTY → Browser | Binary (0x2) | Raw terminal output bytes |
| Browser → PTY | Binary (0x2) | Raw keystroke bytes |
| Browser → PTY | Text (0x1) | JSON control messages (e.g., `{"type":"resize","cols":80,"rows":24}`) |
| Both | Ping/Pong (0x9/0xA) | Keep-alive |
| Both | Close (0x8) | Graceful disconnect |

### Reconnect Behavior

The `terminal.html` WebSocket has built-in auto-reconnect with exponential backoff (1s → 2s → 4s → ... → 10s max). On reconnect:
1. New WebSocket handshake to `/terminal/ws?token=...`
2. Server replays scrollback buffer
3. Client sends resize control message
4. Server sends `SIGWINCH` to force TUI redraw (even if dimensions haven't changed)

### Connection Status Dot

The iframe sends `postMessage` events to the parent widget:
- `connected` → green dot
- `disconnected` → orange dot
- `exited` → red dot

---

## Extension Integration

### Terminal Server URL Computation

The extension computes the terminal server URL from the base daemon URL:
```typescript
function getTerminalServerUrl(baseUrl: string): string {
  const url = new URL(baseUrl)
  url.port = String(parseInt(url.port || '7890', 10) + TERMINAL_PORT_OFFSET)
  return url.origin
}
```

`TERMINAL_PORT_OFFSET = 1` is defined in `src/lib/constants.ts`.

### CSP Reachability Check

The tracked-tab hover launcher proactively checks if the terminal server is reachable before enabling the terminal button. On non-localhost pages, CSP may block `fetch()` to `localhost`:

```typescript
async function checkTerminalReachable(): Promise<boolean>
```

If the fetch fails (CSP block or server not running), the terminal button is dimmed with `opacity: 0.35` and a tooltip explaining the issue. This replaces the previous static `isLocalPage` regex check.

### PostMessage Bridge

The content script communicates with the terminal iframe via `postMessage`:

| Direction | Message | Purpose |
|-----------|---------|---------|
| Parent → Iframe | `{target: 'gasoline-terminal', command: 'focus'}` | Focus the xterm.js instance |
| Parent → Iframe | `{target: 'gasoline-terminal', command: 'resize'}` | Refit terminal after widget resize |
| Parent → Iframe | `{target: 'gasoline-terminal', command: 'redraw'}` | Soft redraw xterm canvas without iframe/session reload |
| Parent → Iframe | `{target: 'gasoline-terminal', command: 'write', text: '...'}` | Write text to PTY stdin |
| Iframe → Parent | `{source: 'gasoline-terminal', event: 'connected'}` | WebSocket connected |
| Iframe → Parent | `{source: 'gasoline-terminal', event: 'disconnected'}` | WebSocket disconnected |
| Iframe → Parent | `{source: 'gasoline-terminal', event: 'exited'}` | PTY process exited |
| Iframe → Parent | `{source: 'gasoline-terminal', event: 'focus', data: { focused }}` | xterm focus/blur state updates |
| Iframe → Parent | `{source: 'gasoline-terminal', event: 'typing', data: { at }}` | Throttled typing heartbeat timestamp |

Origin validation: parent only accepts messages from the terminal server origin. Iframe sends to `*` (since it doesn't know the parent's origin in advance).

### Queued Write Guard

When `writeToTerminal()` is called (for example from annotation auto-send), the widget now queues writes and applies a focus guard:

1. If terminal is connected and user is idle, write is sent immediately.
2. If terminal has focus and recent typing (< 1.5s), write is deferred.
3. A warning toast is shown (`waiting for user to stop typing`) at a throttled interval.
4. After idle clears, widget soft-redraws terminal, writes text, then sends `\r`.
5. If WebSocket disconnects before submit, queued Enter waits until reconnect, then continues.
6. Focus is returned to xterm after submit.

If the user re-focuses and types again during the auto-submit window, Enter is deferred again until idle.

---

## PTY Layer

### Manager (`internal/pty/manager.go`)

- Manages a map of `sessionID → *Session` and `token → sessionID`
- Tokens are 32-byte cryptographic random hex strings
- Thread-safe: all operations hold `sync.RWMutex`
- `Stop()` removes map entries under lock, then calls `sess.Close()` outside the lock to avoid blocking concurrent reads during slow child process teardown

### Session (`internal/pty/session.go`)

- Wraps a PTY master fd + child process
- `Spawn()`: opens `/dev/ptmx`, grants/unlocks slave, sets initial `winsize`, starts child with `Setsid + Setctty`
- `Close()`: sends `SIGTERM`, closes PTY master, waits up to 2s for child exit, escalates to `SIGKILL` if needed
- `Resize()`: `TIOCSWINSZ` ioctl on the PTY master
- `ForceRedraw()`: sends `SIGWINCH` directly to the child process (used on reconnect when dimensions match but display is stale)
- Environment: inherits from parent process, adds `TERM=xterm-256color`

### Sandbox Detection

If the daemon was spawned by an MCP client's stdio transport, macOS sandbox restrictions may prevent `posix_spawn`/`fork`. The `handleTerminalStart` handler detects this and returns HTTP 503 with a `sandbox_restricted` error, which the widget displays as an actionable overlay with the command to restart the daemon with full permissions.

---

## Routes (on terminal server, port+1)

| Route | Method | Purpose |
|-------|--------|---------|
| `/terminal` | GET | Serve terminal HTML page (embedded in binary) |
| `/terminal/static/` | GET | Serve xterm.js, xterm.css (embedded FS) |
| `/terminal/ws` | GET→101 | WebSocket upgrade for PTY I/O (token-validated) |
| `/terminal/start` | POST | Create a new PTY session (returns token) |
| `/terminal/stop` | POST | Destroy a PTY session (kills process) |
| `/terminal/validate` | GET | Check if a session token maps to a live session |
| `/terminal/config` | GET | List active sessions and count |

Note: `/config/active-codebase` is on the **main** daemon server (not terminal server) — it's not terminal-specific.

---

## Code Paths

| File | Responsibility |
|------|---------------|
| `cmd/dev-console/terminal_server.go` | Dedicated server setup: `setupTerminalMux()`, `startTerminalServer()` |
| `cmd/dev-console/terminal_handlers.go` | All HTTP handlers: page, WS, start, stop, validate, config |
| `cmd/dev-console/terminal_assets/terminal.html` | xterm.js terminal page with WS reconnect and postMessage bridge |
| `src/content/ui/terminal-widget.ts` | Widget UI: state machine, session persistence, iframe lifecycle |
| `src/content/ui/tracked-hover-launcher.ts` | Terminal button in hover launcher + CSP reachability check |
| `src/lib/constants.ts` | `TERMINAL_PORT_OFFSET`, storage keys |
| `internal/pty/manager.go` | Session manager: create, get, destroy, token auth |
| `internal/pty/session.go` | PTY session: spawn, I/O, resize, scrollback, close |
| `cmd/dev-console/main_connection_mcp.go` | Terminal server startup wiring |
| `cmd/dev-console/main_connection_mcp_shutdown.go` | Terminal server graceful shutdown |
