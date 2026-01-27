# Multi-Client MCP Architecture

## Status: Draft
## Author: Claude + Brenn
## Date: 2025-01-26

---

## Problem Statement

Currently, each MCP process starts its own HTTP server on port 7890. This means:
- Only one project can use Gasoline at a time
- Switching projects requires killing the server
- The browser extension must reconnect when the server restarts

**Goal**: Allow multiple MCP clients (different Claude Code sessions in different repos) to share a single HTTP server and browser extension connection.

---

## Architecture Overview

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Claude Code    │     │  Claude Code    │     │  Claude Code    │
│  (Project A)    │     │  (Project B)    │     │  (Project C)    │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │ stdio                 │ stdio                 │ stdio
         ▼                       ▼                       ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  MCP Client     │     │  MCP Client     │     │  MCP Client     │
│  (--connect)    │     │  (--connect)    │     │  (--connect)    │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │ HTTP (localhost:7890)
                                 ▼
                    ┌────────────────────────┐
                    │    Gasoline Server     │
                    │    (single instance)   │
                    │                        │
                    │  ┌──────────────────┐  │
                    │  │  Shared State    │  │
                    │  │  - Log buffer    │  │
                    │  │  - Network bodies│  │
                    │  │  - WS events     │  │
                    │  │  - Actions       │  │
                    │  └──────────────────┘  │
                    │                        │
                    │  ┌──────────────────┐  │
                    │  │  Per-Client State│  │
                    │  │  - Checkpoints   │  │
                    │  │  - Noise rules   │  │
                    │  │  - Pending queries│ │
                    │  │  - KV store      │  │
                    │  └──────────────────┘  │
                    └───────────┬────────────┘
                                │
                                ▼
                    ┌────────────────────────┐
                    │   Browser Extension    │
                    └────────────────────────┘
```

---

## Data Classification

### Shared State (all clients see the same data)

| Data | Rationale |
|------|-----------|
| Console logs | Browser produces one stream; all clients need visibility |
| Network requests/bodies | Same — browser state is global |
| WebSocket events | Same |
| Enhanced actions | User actions are global to the browser |
| Performance snapshots | Same |
| DOM state | Live DOM is shared |

### Per-Client State (isolated by client ID)

| Data | Rationale |
|------|-----------|
| Checkpoints | "before-fix" in Project A ≠ "before-fix" in Project B |
| Noise rules | Project A may want to suppress different patterns than B |
| KV store | Persistent storage is project-specific |
| Pending DOM queries | Response routing — query from A must return to A |
| Pending a11y audits | Same |
| Read cursors | "What's new since I last checked" is per-client |

---

## Client Identity

### Option A: Auto-generated UUID
```
MCP client generates UUID on startup, includes in all HTTP requests.
Header: X-Gasoline-Client: abc123
```

**Pros**: Simple, no config needed
**Cons**: New ID on every restart — orphan state accumulates

### Option B: Project-derived ID
```
Hash of working directory or project name.
Header: X-Gasoline-Client: sha256(cwd)[:12]
```

**Pros**: Same project = same ID across restarts
**Cons**: Two terminals in same project = same ID (may be desired?)

### Option C: User-specified ID
```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline",
      "args": ["--connect", "--client-id", "project-a"]
    }
  }
}
```

**Pros**: Explicit control
**Cons**: User burden

### Recommendation: Option B with fallback to A

1. If `--client-id` provided → use it
2. Else if stdin is from Claude Code → derive from CWD (Claude Code sets this)
3. Else → generate UUID

---

## Query Routing

DOM queries and a11y audits are request/response — the response must return to the correct client.

### Current Flow
```
1. MCP client calls query_dom(selector)
2. Server adds query to pendingQueries[]
3. Extension polls /pending-queries, gets query
4. Extension runs query, POSTs to /dom-result
5. Server stores result in queryResults[]
6. MCP client polls for result
```

### Multi-Client Flow
```
1. MCP client A calls query_dom(selector) with X-Gasoline-Client: A
2. Server adds query to pendingQueries with clientId: A
3. Extension polls /pending-queries (no change — gets all pending)
4. Extension runs query, POSTs to /dom-result with queryId
5. Server stores result with clientId: A
6. Only client A can retrieve this result
```

**Key change**: Query results are keyed by (queryId, clientId). Client B cannot retrieve Client A's results.

---

## Read Cursors (Preventing Duplicate Reads)

### Problem
Client A reads logs at t=100. Client B reads logs at t=100. Both see the same errors. Both try to fix the same bug.

### Solution: Per-Client Read Cursors

Each client tracks "last read position" in shared buffers:

```go
type ClientState struct {
    ID              string
    LogCursor       int64     // index into log buffer
    NetworkCursor   int64
    ActionsCursor   int64
    LastSeen        time.Time // for cleanup
}
```

When Client A calls `observe(what: "errors")`:
1. Server returns entries[A.LogCursor:]
2. Server updates A.LogCursor = len(entries)

Client B calling `observe(what: "errors")` gets its own view based on B.LogCursor.

### First-Read Behavior

New client (cursor = 0) sees all buffered data. This is correct — they need context.

### Clear Behavior

`configure(action: "clear")` options:
1. **Clear shared buffer** — affects all clients (destructive)
2. **Reset my cursor** — only affects calling client (safe)

Recommendation: Default to resetting cursor. Add `scope: "all"` param for destructive clear.

---

## Checkpoint Namespacing

Checkpoints are namespaced by client ID automatically:

```
Client A: checkpoint "before" → stored as "A:before"
Client B: checkpoint "before" → stored as "B:before"
```

Client A calling `analyze(target: "changes", checkpoint: "before")` resolves to "A:before".

Cross-client access (if ever needed): `checkpoint: "B:before"` with explicit prefix.

---

## Noise Rule Isolation

Each client has its own noise rule set:

```go
type ClientState struct {
    NoiseRules []NoiseRule
}
```

When filtering logs for Client A, apply A's noise rules only.

### Shared Noise Rules (Future)

Some patterns are universally noisy (React DevTools, etc.). Future enhancement:
- `scope: "global"` — applies to all clients
- `scope: "client"` — default, per-client

---

## Connection Lifecycle

### MCP Client Startup (--connect mode)

```
1. Parse --connect flag
2. Check if server is running: GET http://localhost:7890/health
3. If not running → error: "No Gasoline server running. Start one with: gasoline --server"
4. Register client: POST /clients { clientId: "..." }
5. Enter MCP stdio loop, forwarding tool calls to HTTP server
6. On stdin EOF → unregister: DELETE /clients/{clientId}
```

### Server Startup (--server mode)

No change from current behavior. Server starts, waits for connections.

### Client Cleanup

When MCP client disconnects:
1. Per-client state is retained for `--client-ttl` (default: 1 hour)
2. After TTL, state is purged
3. Allows reconnection without losing checkpoints

---

## HTTP API Changes

### New Endpoints

```
POST   /clients              Register a client
DELETE /clients/{id}         Unregister a client
GET    /clients              List active clients (admin/debug)
```

### Modified Endpoints

All existing endpoints accept `X-Gasoline-Client` header:

```
GET /logs                    Returns logs from client's cursor position
POST /pending-queries        Now includes clientId in query object
GET /dom-result/{queryId}    Only returns if clientId matches
```

### MCP-over-HTTP

`POST /mcp` already exists. With `X-Gasoline-Client` header, it routes to correct client state.

---

## Configuration

### Server Flags

```
--server              Run HTTP server only (existing)
--client-ttl <dur>    How long to keep disconnected client state (default: 1h)
```

### Client Flags (new)

```
--connect             Connect to existing server instead of starting one
--client-id <id>      Explicit client identifier
--port <n>            Server port to connect to (default: 7890)
```

### MCP Config Examples

**Current (starts own server):**
```json
{
  "mcpServers": {
    "gasoline": {
      "command": "/path/to/gasoline"
    }
  }
}
```

**New (connects to shared server):**
```json
{
  "mcpServers": {
    "gasoline": {
      "command": "/path/to/gasoline",
      "args": ["--connect"]
    }
  }
}
```

---

## Race Condition Analysis

### Scenario 1: Concurrent Log Reads
- A reads logs, B reads logs simultaneously
- **Safe**: Each has own cursor, reads don't interfere

### Scenario 2: Read During Write
- Extension POSTs new logs while A is reading
- **Safe**: Cursor-based reads use snapshot semantics

### Scenario 3: Concurrent DOM Queries
- A and B both query `.error-message`
- **Safe**: Each query has unique ID, results routed by (queryId, clientId)

### Scenario 4: Clear vs Read Race
- A clears logs while B is reading
- **Depends on implementation**:
  - If clear resets cursors → B sees empty on next read (surprising but consistent)
  - If clear only resets caller's cursor → B unaffected (recommended)

### Scenario 5: Checkpoint Name Collision
- A creates "test", B creates "test"
- **Safe**: Namespaced as "A:test" and "B:test"

### Scenario 6: Server Restart
- Server restarts, clients reconnect
- **Issue**: In-memory state lost
- **Mitigation**: Persist critical state (checkpoints, noise rules) to disk

---

## Implementation Phases

### Phase 1: Connect Mode (MVP)
- [ ] Add `--connect` flag
- [ ] Client identity via CWD hash
- [ ] Basic state isolation (checkpoints, noise rules)
- [ ] Query routing by clientId

### Phase 2: Cursor-Based Reads
- [ ] Per-client read cursors for logs, network, actions
- [ ] Cursor persistence across reconnects

### Phase 3: Robustness
- [ ] Client TTL and cleanup
- [ ] State persistence to disk
- [ ] Health endpoint shows connected clients

### Phase 4: Polish
- [ ] `/clients` admin endpoint
- [ ] Cross-client checkpoint access
- [ ] Global vs client noise rules

---

## Open Questions

1. **Should the extension know about clients?**
   - Current: Extension talks to server, server manages clients
   - Alternative: Extension could route data to specific clients
   - Recommendation: Keep extension simple, server handles multiplexing

2. **What happens if two projects have same CWD hash?**
   - Unlikely but possible (symlinks, etc.)
   - Could add process PID to hash, but then restarts get new ID
   - Recommendation: Accept this edge case, offer `--client-id` escape hatch

3. **Should we support multiple servers on different ports?**
   - Use case: Complete isolation between work and personal projects
   - Recommendation: Out of scope. Use different ports manually if needed.

4. **Real-time sync between clients?**
   - Use case: Pair programming, both see same errors
   - Recommendation: Out of scope. Clients poll independently.

---

## Rejected Alternatives

### Alternative A: Separate Ports per Client
Each MCP client gets its own port (7890, 7891, 7892...).

**Rejected because**: Extension can only connect to one port. Would need extension changes or multiple extension instances.

### Alternative B: WebSocket Multiplexing
Server maintains WebSocket to each MCP client for real-time push.

**Rejected because**: Over-engineered. Polling-based MCP is fine. Adds complexity.

### Alternative C: Shared State Only
No per-client isolation. All clients see everything.

**Rejected because**: Checkpoint collisions, noise rule conflicts, confusing UX.

---

## Success Criteria

1. User can run `gasoline --server` once
2. Multiple projects can add `"args": ["--connect"]` to `.mcp.json`
3. Each project's Claude sees browser data
4. Checkpoints and noise rules don't interfere between projects
5. Closing one project doesn't affect others

---

## Extended Requirements (v2)

### Tab-Based Routing

**Problem**: Multiple Claude sessions may be working on different browser tabs simultaneously.

```
Client A: Debugging React app on localhost:3000 (Tab 1)
Client B: Debugging API server on localhost:4000 (Tab 2)
```

Both clients currently see ALL browser data mixed together.

#### Option 1: Tab Affinity (Explicit)
```json
// Client A config
{ "args": ["--connect", "--tab-filter", "localhost:3000"] }
```

Client A only sees logs/network from URLs matching filter. Extension tags all data with source tab ID.

#### Option 2: Tab Affinity (Automatic)
Extension tracks which tab is "active" when client connects. Client auto-binds to that tab.

**Problem**: What if user switches tabs?

#### Option 3: Tab Selection via Tool
New tool: `select_tab(url_pattern: "localhost:3000")` — client explicitly chooses which tab(s) to monitor.

```go
type ClientState struct {
    TabFilters []string  // URL patterns this client cares about
}
```

#### Recommendation: Option 3

Most flexible. Client can:
- Monitor all tabs (default)
- Filter to specific tab(s)
- Change filter mid-session

---

### Full Duplex Communication

**Current**: MCP is request/response. Client asks, server answers.

**Goal**: Server can push events to clients without polling.

#### Use Cases
1. **Real-time error alerts**: New console error → push to all clients
2. **DOM change notifications**: Element appeared → push to waiting client
3. **WebSocket event streaming**: Don't buffer, stream live

#### Architecture Options

##### A. Server-Sent Events (SSE)
```
Client A connects: GET /events?client=A (keep-alive)
Server pushes: data: {"type":"error","message":"..."}\n\n
```

**Pros**: Simple, HTTP-based, works with MCP stdio proxy
**Cons**: One-way only (client → server still needs HTTP)

##### B. WebSocket per Client
```
Client A connects: ws://localhost:7890/ws?client=A
Bidirectional messaging
```

**Pros**: True full-duplex
**Cons**: More complex, may not work well with MCP stdio

##### C. Long-Polling with Comet
```
Client A: GET /poll?client=A&timeout=30s
Server holds connection until event or timeout
Client immediately reconnects
```

**Pros**: Works everywhere, no special protocols
**Cons**: Latency, connection overhead

##### D. Hybrid: SSE for Push, HTTP for Commands
```
MCP client spawns two connections:
1. stdio → HTTP POST /mcp (commands)
2. background goroutine → GET /events (receive pushes)
```

**Recommendation**: Start with Option D (Hybrid)

MCP stdio loop handles tool calls. Background SSE connection receives pushes. Pushes are queued and returned on next tool call if SSE unavailable.

---

### Multiple Log Files

**Problem**: Single `.gasoline/` log file gets polluted when multiple clients are active.

#### Current State
```
.gasoline/
  gasoline.log       # All events, all clients
```

#### Proposed Structure
```
.gasoline/
  server.log         # Server lifecycle events
  clients/
    abc123.log       # Client A's session log
    def456.log       # Client B's session log
  shared/
    console.log      # Browser console (shared)
    network.log      # Network requests (shared)
    websocket.log    # WebSocket events (shared)
```

#### Log Rotation
- Per-client logs: Rotate on client disconnect or size limit (10MB)
- Shared logs: Rotate hourly or on size limit (50MB)
- Retention: Keep last 7 days

#### Log Format
```jsonl
{"ts":"2025-01-26T12:00:00Z","client":"abc123","type":"tool_call","tool":"observe","args":{"what":"errors"}}
{"ts":"2025-01-26T12:00:01Z","client":"abc123","type":"tool_result","entries":15}
```

---

### Concurrency Model

#### Current Implementation
```go
// Single mutex protects all state
var mu sync.Mutex
var logs []LogEntry
var network []NetworkEntry
```

#### Problems at Scale
1. **Lock contention**: Extension POSTs block client reads
2. **No reader parallelism**: Multiple clients reading = serial
3. **Buffer growth**: Unbounded slices cause GC pressure

#### Proposed: Per-Buffer RWMutex + Ring Buffers

```go
type SharedBuffers struct {
    Logs     *RingBuffer[LogEntry]     // RWMutex internally
    Network  *RingBuffer[NetworkEntry]
    Actions  *RingBuffer[ActionEntry]
    WS       *RingBuffer[WSEvent]
}

type RingBuffer[T any] struct {
    mu       sync.RWMutex
    entries  []T
    head     int64  // Write position (atomic)
    capacity int
}

func (rb *RingBuffer[T]) Write(entry T) {
    rb.mu.Lock()
    defer rb.mu.Unlock()
    rb.entries[rb.head % rb.capacity] = entry
    atomic.AddInt64(&rb.head, 1)
}

func (rb *RingBuffer[T]) ReadFrom(cursor int64) ([]T, int64) {
    rb.mu.RLock()
    defer rb.mu.RUnlock()
    // Return entries[cursor:head], new cursor
}
```

**Benefits**:
- Readers don't block each other
- Writers only block briefly
- Fixed memory footprint
- Lock-free cursor reads possible with atomics

#### Client State Isolation

```go
type ClientRegistry struct {
    mu      sync.RWMutex
    clients map[string]*ClientState
}

type ClientState struct {
    mu          sync.Mutex  // Protects this client's state only
    ID          string
    Cursors     BufferCursors
    Checkpoints map[string]Checkpoint
    NoiseRules  []NoiseRule
    TabFilters  []string
    PendingQueries chan QueryResult  // Per-client query results
}
```

Each client has its own mutex — no cross-client blocking.

---

### Data Flow Diagrams

#### Write Path (Extension → Server)
```
Extension POST /logs
    │
    ▼
┌─────────────────┐
│ Parse + Validate│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ RingBuffer.Write│  ← Brief write lock
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Notify SSE      │  ← Non-blocking broadcast
│ subscribers     │
└─────────────────┘
```

#### Read Path (Client → Server)
```
Client calls observe(what: "errors")
    │
    ▼
┌─────────────────┐
│ Get client state│  ← RLock on registry
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Get cursor      │  ← Lock on client state
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ RingBuffer.Read │  ← RLock on buffer
│ from cursor     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Apply noise     │  ← No lock needed (copy)
│ filters         │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Update cursor   │  ← Lock on client state
└────────┬────────┘
         │
         ▼
Return filtered entries
```

#### Query Path (DOM/A11y)
```
Client A: query_dom(".error")
    │
    ▼
┌─────────────────┐
│ Generate queryId│
│ + clientId      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Add to pending  │  ← Global pending queue
│ queries         │
└────────┬────────┘
         │
    Extension polls /pending-queries
         │
         ▼
┌─────────────────┐
│ Extension runs  │
│ query in DOM    │
└────────┬────────┘
         │
    Extension POSTs /dom-result
         │
         ▼
┌─────────────────┐
│ Route by        │
│ queryId→clientId│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Send to client  │  ← Via channel or SSE
│ A's result chan │
└─────────────────┘
```

---

### Graceful Degradation

#### Server Crash Recovery
1. Clients detect via `/health` failure
2. Clients enter "disconnected" mode — queue tool calls locally
3. When server returns, clients replay queued calls
4. Cursors reset to 0 (re-read all buffered data)

#### Client Crash Recovery
1. Server notices client TTL expiration
2. Per-client state preserved for `--client-ttl`
3. New client with same ID inherits state (checkpoints, noise rules)
4. Cursors preserved — no duplicate reads

#### Extension Disconnect
1. Server tracks `lastExtensionPoll` timestamp
2. If stale > 5s, mark extension "disconnected"
3. Tool calls return warning: "Extension not connected, data may be stale"
4. DOM queries fail fast with helpful error

---

### Security Considerations

#### Client ID Spoofing
Malicious process could use another client's ID.

**Mitigation**:
- Client IDs are localhost-only (not exposed to network)
- Optional: Sign client ID with shared secret from env var

#### Log File Access
Log files contain potentially sensitive data (network bodies, etc.).

**Mitigation**:
- `.gasoline/` should be gitignored
- Log files created with 0600 permissions
- Optional: Encrypt at rest with user-provided key

#### Resource Exhaustion
Malicious client could exhaust server memory.

**Mitigation**:
- Ring buffers have fixed capacity
- Max clients limit (default: 10)
- Per-client rate limiting on writes

---

### Implementation Roadmap

#### Phase 1: Connect Mode MVP (Week 1-2)
- [ ] `--connect` flag implementation
- [ ] Client registry with CWD-based IDs
- [ ] Basic state isolation (checkpoints, noise rules)
- [ ] Query routing by clientId
- [ ] Unit tests for multi-client scenarios

#### Phase 2: Ring Buffers + Cursors (Week 3)
- [ ] Replace slice buffers with RingBuffer
- [ ] Per-client read cursors
- [ ] Cursor persistence in client state
- [ ] Benchmark: 3 clients, 1000 logs/sec write, concurrent reads

#### Phase 3: Tab Filtering (Week 4)
- [ ] Extension tags data with tab ID
- [ ] `select_tab` tool implementation
- [ ] Tab filter logic in read path
- [ ] Tests: Two clients, two tabs, correct routing

#### Phase 4: SSE Push (Week 5)
- [ ] `/events` SSE endpoint
- [ ] Client-side SSE consumer (background goroutine)
- [ ] Push notifications for new errors
- [ ] Graceful fallback to polling

#### Phase 5: Log Files (Week 6)
- [ ] Per-client log files
- [ ] Shared buffer log files
- [ ] Log rotation
- [ ] `--log-dir` flag

#### Phase 6: Robustness (Week 7-8)
- [ ] Client TTL and cleanup
- [ ] Server crash recovery
- [ ] Health endpoint with client list
- [ ] Integration tests: chaos scenarios

---

### Metrics & Observability

#### Server Metrics
```
gasoline_clients_active          gauge   # Current connected clients
gasoline_buffer_entries{type}    gauge   # Entries in each ring buffer
gasoline_buffer_writes_total     counter # Total writes to buffers
gasoline_client_reads_total      counter # Total reads by clients
gasoline_query_latency_seconds   histogram # DOM query round-trip time
gasoline_sse_connections         gauge   # Active SSE connections
```

#### Health Endpoint Enhancement
```json
GET /health

{
  "status": "healthy",
  "uptime_seconds": 3600,
  "clients": {
    "active": 3,
    "list": [
      {"id": "abc123", "connected_at": "...", "last_seen": "..."},
      {"id": "def456", "connected_at": "...", "last_seen": "..."}
    ]
  },
  "buffers": {
    "logs": {"capacity": 10000, "used": 2500, "head": 12500},
    "network": {"capacity": 5000, "used": 1200, "head": 6200}
  },
  "extension": {
    "connected": true,
    "last_poll": "2025-01-26T12:00:00Z"
  }
}
```
