# Multi-Client MCP Architecture: Engineering Review

## Status: Review Complete
## Reviewer: Claude (Principal Go Engineer Review)
## Date: 2026-01-26

---

## Executive Summary

The multi-client MCP architecture spec is well-structured and addresses the core problem correctly. However, the proposed RWMutex + RingBuffer approach has subtle race conditions that will cause cursor corruption under load. The spec also underestimates the complexity of SSE fan-out and overengineers log file separation. This review recommends: (1) replacing the atomics-with-RWMutex hybrid with a cleaner lock-free ring buffer using compare-and-swap, (2) deferring SSE to Phase 4+, (3) simplifying log files to a single append-only JSONL with client tags, and (4) adding explicit lock ordering documentation to prevent deadlocks as the codebase grows.

---

## Section-by-Section Analysis

### 1. Concurrency Model

#### Current Spec Proposal (lines 591-617)

```go
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
```

#### Problem 1: Mixing Atomics with Mutexes

The spec uses `atomic.AddInt64` inside a mutex-protected region. This is redundant and misleading. If you hold the write lock, atomics add overhead without benefit. If you want lock-free reads, the atomic must be readable WITHOUT the lock.

**Race Condition**: Reader calls `ReadFrom(cursor)` holding RLock. Writer concurrently increments `head` atomically. Reader sees partial state: `head` was incremented but `entries[head % capacity]` hasn't been written yet.

#### Recommended Fix: Pure Lock-Free Ring Buffer

```go
type RingBuffer[T any] struct {
    entries  []atomic.Pointer[T] // Each slot is an atomic pointer
    head     atomic.Int64        // Write position
    tail     atomic.Int64        // Oldest valid position (for wrap detection)
    capacity int64
}

func (rb *RingBuffer[T]) Write(entry T) {
    for {
        head := rb.head.Load()
        slot := head % rb.capacity

        // Store the entry atomically
        rb.entries[slot].Store(&entry)

        // Advance head with CAS (allows concurrent writers)
        if rb.head.CompareAndSwap(head, head+1) {
            // Update tail if we wrapped
            if head >= rb.capacity {
                rb.tail.Store(head - rb.capacity + 1)
            }
            return
        }
        // CAS failed, retry
    }
}

func (rb *RingBuffer[T]) ReadFrom(cursor int64) ([]T, int64) {
    head := rb.head.Load()
    tail := rb.tail.Load()

    // Clamp cursor to valid range
    if cursor < tail {
        cursor = tail
    }
    if cursor >= head {
        return nil, cursor
    }

    result := make([]T, 0, head-cursor)
    for i := cursor; i < head; i++ {
        entry := rb.entries[i % rb.capacity].Load()
        if entry != nil {
            result = append(result, *entry)
        }
    }
    return result, head
}
```

**Trade-off**: This uses more memory (pointer per slot) but eliminates all lock contention between readers and writers. For Gasoline's workload (high-frequency writes from extension, concurrent reads from multiple clients), this is the right choice.

#### Alternative: Simpler RWMutex-Only Approach

If the added complexity of lock-free is undesirable for MVP, use pure RWMutex without atomics:

```go
type RingBuffer[T any] struct {
    mu       sync.RWMutex
    entries  []T
    head     int64 // NOT atomic - always access under lock
    capacity int
}

func (rb *RingBuffer[T]) Write(entry T) {
    rb.mu.Lock()
    rb.entries[rb.head % int64(rb.capacity)] = entry
    rb.head++
    rb.mu.Unlock()
}

func (rb *RingBuffer[T]) ReadFrom(cursor int64) ([]T, int64) {
    rb.mu.RLock()
    defer rb.mu.RUnlock()

    if cursor >= rb.head {
        return nil, cursor
    }

    // Clamp to available data
    oldest := rb.head - int64(rb.capacity)
    if oldest < 0 {
        oldest = 0
    }
    if cursor < oldest {
        cursor = oldest
    }

    result := make([]T, rb.head-cursor)
    for i := cursor; i < rb.head; i++ {
        result[i-cursor] = rb.entries[i % int64(rb.capacity)]
    }
    return result, rb.head
}
```

**Recommendation for MVP**: Use the pure RWMutex approach. It's simpler, correct, and sufficient for the expected load (< 1000 events/sec). Benchmark before optimizing to lock-free.

#### Problem 2: Lock Ordering

The current codebase has multiple mutex holders that could deadlock:

1. `Server.mu` - protects log entries
2. `Capture.mu` - protects network/ws/action buffers
3. `CheckpointManager.mu` - protects checkpoints
4. `NoiseConfig.mu` - protects noise rules

The spec adds:
5. `ClientRegistry.mu` - protects client map
6. `ClientState.mu` - per-client state

**Deadlock Scenario**:
```
Goroutine A: Lock ClientRegistry.mu -> Lock Capture.mu (to get cursor)
Goroutine B: Lock Capture.mu -> Lock ClientRegistry.mu (to check client exists)
```

**Recommended Lock Ordering** (add to spec):

```
1. ClientRegistry.mu (outer)
2. ClientState.mu (per-client, non-overlapping)
3. Capture.mu (buffer access)
4. Server.mu (log access)
5. CheckpointManager.mu (inner)
6. NoiseConfig.mu (inner)
```

Document this in a `LOCKING.md` file and enforce via code review. Consider adding `go-deadlock` to CI for detection.

---

### 2. State Management Across Clients

#### The Spec's Approach (lines 79-88, 617-645)

Per-client state includes:
- Cursors (log, network, ws, actions)
- Checkpoints
- Noise rules
- Tab filters
- Pending queries

#### Problem: Cursor Persistence Across Reconnects

The spec says cursors should persist for `--client-ttl` (default 1 hour). But cursors are indices into ring buffers. If the buffer wraps while the client is disconnected, the cursor becomes invalid.

**Example**:
1. Client A at cursor 500 disconnects
2. 2000 new entries arrive (buffer capacity 1000)
3. Client A reconnects, cursor 500 now points to garbage

**Recommended Fix**: Store cursors as (position, timestamp) pairs:

```go
type BufferCursor struct {
    Position  int64     // Ring buffer position
    Timestamp time.Time // When this position was valid
}

func (c *BufferCursor) Resolve(buffer *RingBuffer) int64 {
    tail := buffer.Tail()

    // If our position is still in the buffer, use it
    if c.Position >= tail {
        return c.Position
    }

    // Position was evicted - find nearest valid position by timestamp
    // (requires buffer to store timestamps, which Capture already does)
    return buffer.FindPositionAtTime(c.Timestamp)
}
```

This gracefully handles wraparound by falling back to timestamp-based resolution.

#### Memory Overhead Analysis

Per-client state memory estimate:

| Component | Size |
|-----------|------|
| 4 cursors (log, network, ws, actions) | 32 bytes |
| 20 checkpoints (max) @ ~200 bytes each | 4 KB |
| 100 noise rules (max) @ ~300 bytes each | 30 KB |
| 5 tab filters @ ~100 bytes each | 500 bytes |
| Pending query channels | ~1 KB |
| **Total per client** | **~36 KB** |

With max 10 clients (spec's proposed limit), total overhead is ~360 KB. This is acceptable.

**Recommendation**: The 10-client limit is reasonable. Add a `--max-clients` flag for users who need more (e.g., CI systems).

---

### 3. Data Flow with Multiple Tabs

#### The Spec's Tab Affinity Proposal (lines 448-479)

Three options were proposed. Option 3 (tool-based selection) is correct, but the implementation needs more detail.

#### Recommended Implementation

```go
// Extension tags ALL data with tab ID
type TabTaggedEntry struct {
    TabID    int    `json:"tabId"`
    TabURL   string `json:"tabUrl"` // For debugging
    Payload  any    `json:"payload"`
}

// Server stores tag but doesn't filter at ingest
func (v *Capture) AddWebSocketEvents(events []WebSocketEvent) {
    // Events already have TabID from extension
    // Store as-is, filter at read time
}

// Read-time filtering based on client's tab preferences
func (v *Capture) GetWebSocketEventsForClient(
    client *ClientState,
    filter WebSocketEventFilter,
) []WebSocketEvent {
    events := v.GetWebSocketEvents(filter)

    if len(client.TabFilters) == 0 {
        return events // No filter = all tabs
    }

    var filtered []WebSocketEvent
    for _, e := range events {
        if client.MatchesTabFilter(e.TabID, e.URL) {
            filtered = append(filtered, e)
        }
    }
    return filtered
}
```

**Key Design Decisions**:

1. **Extension knows nothing about clients** - Keeps extension simple. Server multiplexes.
2. **Store everything, filter at read** - Allows clients to change filters without losing data.
3. **URL-based matching, not tab ID** - Tab IDs change on reload. URL patterns are stable.

#### Fan-Out Concern

With N clients and M tabs, naive fan-out is O(N*M) per event. But since:
- N is bounded at 10
- M (active tabs with Gasoline content scripts) is typically 1-3
- Events are batched by extension

The actual overhead is negligible. No optimization needed for MVP.

---

### 4. Full Duplex / SSE Push

#### The Spec's SSE Proposal (lines 497-534)

SSE is proposed for push notifications (new errors, DOM changes).

#### Strong Recommendation: Defer SSE to Phase 4+

**Reasons**:

1. **MCP is request/response** - Claude Code polls. Adding SSE creates a parallel data path that MCP clients don't expect.

2. **Backpressure is hard** - If a client is slow, SSE buffers grow unboundedly. You need:
   - Per-client send buffers with limits
   - Heartbeat/keepalive handling
   - Reconnection logic with cursor resumption
   - Graceful degradation when buffer full

3. **Polling works fine** - Claude Code already polls frequently during active sessions. The latency difference between polling every 100ms and SSE push is imperceptible to users.

4. **Complexity vs. value** - SSE adds ~500 lines of code for minimal UX improvement.

#### If SSE is Pursued (Phase 4+)

```go
type SSEManager struct {
    mu       sync.RWMutex
    clients  map[string]*SSEClient
    bufSize  int // Per-client buffer limit
}

type SSEClient struct {
    id       string
    events   chan SSEEvent
    done     chan struct{}
    lastSend time.Time
}

func (m *SSEManager) Broadcast(event SSEEvent) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, client := range m.clients {
        select {
        case client.events <- event:
            // Sent
        default:
            // Buffer full - client is slow
            // Option 1: Drop event (lossy)
            // Option 2: Close connection (force reconnect)
            // Recommendation: Drop with counter, expose in /health
        }
    }
}

func (m *SSEManager) Handler(w http.ResponseWriter, r *http.Request) {
    clientID := r.Header.Get("X-Gasoline-Client")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    client := m.register(clientID)
    defer m.unregister(clientID)

    // Heartbeat ticker
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case event := <-client.events:
            fmt.Fprintf(w, "data: %s\n\n", event.JSON())
            flusher.Flush()
        case <-ticker.C:
            fmt.Fprintf(w, ": heartbeat\n\n")
            flusher.Flush()
        case <-r.Context().Done():
            return
        case <-client.done:
            return
        }
    }
}
```

**Graceful Degradation**: If SSE unavailable (client behind proxy that buffers), fall back to polling. Track SSE connection state in ClientState:

```go
type ClientState struct {
    // ...
    SSEConnected bool
    SSEDropped   int64 // Events dropped due to slow client
}
```

Expose in `/health` so operators can detect degraded clients.

---

### 5. Log Files

#### The Spec's Proposal (lines 539-569)

```
.gasoline/
  server.log
  clients/
    abc123.log
    def456.log
  shared/
    console.log
    network.log
    websocket.log
```

#### Problem: Over-Engineered for Minimal Benefit

This creates 5+ files that must be coordinated. Concurrent writes to `shared/*.log` need synchronization. Log rotation across multiple files is complex.

#### Recommended: Single Append-Only JSONL

```
~/.gasoline/gasoline.log    # Single file, all events
```

Each line includes client ID for filtering:

```json
{"ts":"2026-01-26T12:00:00Z","client":"abc123","type":"tool_call","tool":"observe"}
{"ts":"2026-01-26T12:00:01Z","client":"def456","type":"console","level":"error","msg":"..."}
{"ts":"2026-01-26T12:00:02Z","client":"","type":"network","url":"..."}  // shared event
```

**Benefits**:
1. Single append-only writer - no coordination needed
2. `grep client=abc123 gasoline.log` for per-client filtering
3. Simple rotation: `mv gasoline.log gasoline.log.1 && gzip gasoline.log.1`
4. Works with existing `tail -f` workflows

**Concurrent Write Handling**:

```go
type Logger struct {
    mu   sync.Mutex
    file *os.File
}

func (l *Logger) Log(entry LogEntry) {
    data, _ := json.Marshal(entry)
    data = append(data, '\n')

    l.mu.Lock()
    l.file.Write(data)
    l.mu.Unlock()
}
```

The mutex is held only for the write syscall (~microseconds). This is sufficient for expected throughput.

#### Rotation Strategy

```go
func (l *Logger) Rotate() error {
    l.mu.Lock()
    defer l.mu.Unlock()

    l.file.Close()

    // Rename current to .1, compress async
    os.Rename(l.path, l.path+".1")
    go func() {
        compressFile(l.path + ".1")
    }()

    // Open new file
    var err error
    l.file, err = os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    return err
}
```

Trigger rotation on size (50MB) or age (24h), checked in background goroutine.

---

### 6. Roadmap Critique

#### Phase 1 (Connect Mode MVP) - Correct

The items listed are the right MVP scope. Add one item:

- [ ] **Document lock ordering** in LOCKING.md

#### Phase 2 (Ring Buffers + Cursors) - Correct but Reorder

Move ring buffer implementation to Phase 1. The current slice-based buffers with position tracking (used by CheckpointManager) are already doing cursor math. Ring buffers are a natural evolution, not a separate phase.

**Revised Phase 1**:
- [ ] `--connect` flag implementation
- [ ] Client registry with CWD-based IDs
- [ ] Ring buffer refactor (unify existing slice buffers)
- [ ] Per-client read cursors
- [ ] Basic state isolation (checkpoints, noise rules)
- [ ] Query routing by clientId

#### Phase 3 (Tab Filtering) - Correct

No changes recommended.

#### Phase 4 (SSE Push) - Defer or Cut

As argued above, SSE provides minimal value for substantial complexity. Either:
- Defer to v7 (post-multi-client stabilization)
- Cut entirely and document "polling is the intended model"

#### Phase 5 (Log Files) - Simplify

Replace proposed multi-file structure with single JSONL as recommended above. This becomes a one-day task instead of a week.

#### Phase 6 (Robustness) - Add Chaos Testing

Add explicit chaos testing items:
- [ ] Test: Server crash with 3 connected clients, verify reconnect
- [ ] Test: Extension disconnect during DOM query, verify timeout
- [ ] Test: Buffer wraparound during client reconnect
- [ ] Test: 1000 events/sec sustained for 60s

---

## Risk Assessment

### High Risk

| Risk | Impact | Mitigation |
|------|--------|------------|
| Cursor corruption after buffer wrap | Client sees garbage/crashes | Use (position, timestamp) cursors with fallback |
| Deadlock from lock ordering violation | Server hangs | Document lock order, add go-deadlock to CI |
| Memory leak from orphan client state | OOM after days | Aggressive TTL enforcement, expose in /health |

### Medium Risk

| Risk | Impact | Mitigation |
|------|--------|------------|
| Client ID collision (same CWD hash) | State confusion | Document in troubleshooting, offer --client-id escape |
| Extension not tagging tab ID correctly | Tab filtering broken | Add tab ID to /diagnostics, test with multi-tab |
| SSE backpressure if implemented | Slow clients cause memory bloat | Bounded buffers with drop policy |

### Low Risk

| Risk | Impact | Mitigation |
|------|--------|------------|
| Log file growth | Disk full | Rotation already planned |
| Query timeout during high load | DOM query fails | Already handled with timeout error |

---

## Alternative Approaches

### Alternative A: Process-Per-Client (Rejected)

Instead of multi-client server, each Claude Code session spawns its own gasoline process on a unique port. Extension connects to multiple ports.

**Why Rejected**: Requires extension changes to manage multiple connections. Violates "extension stays simple" principle.

### Alternative B: Shared Memory IPC (Rejected)

Use mmap'd shared memory for buffers. Clients read directly without HTTP.

**Why Rejected**: Platform-specific (different APIs on macOS/Linux/Windows). HTTP is universal and sufficient.

### Alternative C: SQLite for State (Consider for v7)

Replace in-memory buffers with SQLite. Gives persistence, query flexibility, and battle-tested concurrency.

**Why Deferred**: Adds dependency (violates zero-deps rule). But for v7, consider `modernc.org/sqlite` (pure Go, no CGO).

---

## Final Recommendations

### Prioritized Action Items

1. **[P0] Fix RingBuffer concurrency** - Remove atomic/mutex hybrid, use pure RWMutex for MVP
2. **[P0] Add cursor timestamp fallback** - Prevent corruption after buffer wrap
3. **[P0] Document lock ordering** - Create LOCKING.md, add to PR checklist
4. **[P1] Simplify log files** - Single JSONL with client tags
5. **[P1] Defer SSE** - Move to Phase 4+ or cut entirely
6. **[P2] Add chaos tests** - Buffer wrap, reconnect, high load scenarios
7. **[P2] Add --max-clients flag** - Allow operators to tune (default 10)

### Implementation Order

```
Week 1-2: Phase 1 (connect mode + ring buffers + cursors)
Week 3:   Phase 2 (tab filtering)
Week 4:   Phase 3 (robustness + chaos tests)
Week 5:   Phase 4 (log rotation + polish)
Week 6:   Buffer for issues found in dogfooding
```

SSE, if pursued, becomes Week 7-8 in a follow-up release.

### Success Metrics

| Metric | Target |
|--------|--------|
| Concurrent clients without data loss | 10 |
| Buffer wrap with client reconnect | Cursor resolves correctly |
| Event throughput | 1000/sec sustained |
| Memory per client | < 50 KB |
| Lock contention under load | < 1% CPU in mutex spin |

---

## Appendix: Lock Ordering Reference

```
                    ┌─────────────────────┐
                    │  ClientRegistry.mu  │  (1) Outer
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
    │ ClientState.mu  │ │ ClientState.mu  │ │ ClientState.mu  │  (2) Per-client
    │   (client A)    │ │   (client B)    │ │   (client C)    │
    └────────┬────────┘ └────────┬────────┘ └────────┬────────┘
             │                   │                   │
             └───────────────────┼───────────────────┘
                                 │
                    ┌────────────┴────────────┐
                    ▼                         ▼
          ┌─────────────────┐       ┌─────────────────┐
          │   Capture.mu    │       │   Server.mu     │  (3) Buffer access
          └────────┬────────┘       └────────┬────────┘
                   │                         │
                   └───────────┬─────────────┘
                               │
              ┌────────────────┴────────────────┐
              ▼                                 ▼
    ┌─────────────────────┐          ┌─────────────────┐
    │ CheckpointManager.mu │          │  NoiseConfig.mu │  (4) Inner
    └─────────────────────┘          └─────────────────┘
```

**Rules**:
1. Always acquire locks in numerical order
2. Never hold an outer lock when acquiring an inner lock's callback
3. Per-client locks (2) are independent - can hold A and B simultaneously
4. Release in reverse order

---

*Review complete. Recommend implementing P0 items before proceeding with Phase 1 implementation.*
