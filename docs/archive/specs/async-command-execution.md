# Async Command Execution Specification

**Status:** Approved for implementation
**Created:** 2026-01-27
**Motivation:** Prevent MCP server hangs on long-running browser commands, provide better observability for LLMs

## Problem

Current architecture blocks MCP stdin loop waiting for extension responses:
- `interact({action: "execute_js"})` hangs up to 10s waiting for result
- Extension polls every 1-2s, adding latency
- Long-running JS (5-10s) causes timeouts
- No visibility into command execution status

## Solution: Async Command Architecture

### Core Principle
**Server never hangs.** Commands are queued immediately, extension processes asynchronously, LLM polls for results.

---

## Phase 1: Command Submission (< 1ms)

### Server generates correlation_id and queues command

```go
// MCP request
interact({action: "execute_js", script: "longRunningTask()"})

// Server immediately:
correlationID := fmt.Sprintf("corr-%d-%s", time.Now().UnixMilli(), randomHex(8))
pendingQuery := PendingQuery{
    ID:            generateQueryID(),      // internal dedup
    CorrelationID: correlationID,          // LLM tracks this
    Type:          "browser_action",
    Params:        json.RawMessage(...),
    CreatedAt:     time.Now(),
}
c.pendingQueries = append(c.pendingQueries, pendingQuery)

// Immediate MCP response (never blocks)
return JSONRPCResponse{
    Result: mcpJSONResponse(
        "Command queued for execution",
        json.RawMessage(fmt.Sprintf(`{
            "status": "queued",
            "correlation_id": "%s",
            "message": "Extension will process on next poll (1-2s). Use observe({what: 'command_result', correlation_id: '%s'}) to check status."
        }`, correlationID, correlationID))
    )
}
```

---

## Phase 2: Extension Poll & Mandatory Response (< 2s)

### Extension receives correlation_id from server

```http
GET /pending-queries
Response:
[
  {
    "id": "query-uuid-123",
    "correlation_id": "corr-1738012345678-a1b2c3d4",
    "type": "browser_action",
    "params": {"action": "execute_js", "script": "..."}
  }
]
```

### Extension MUST respond within 2s

**Fast path (< 2s execution):**
```http
POST /query-result
{
  "correlation_id": "corr-1738012345678-a1b2c3d4",
  "status": "complete",
  "result": {"success": true, "data": "result value"}
}
```

**Slow path (> 2s execution):**
```http
POST /query-result (within 2s)
{
  "correlation_id": "corr-1738012345678-a1b2c3d4",
  "status": "pending"
}

POST /query-result (later, when done or after 10s timeout)
{
  "correlation_id": "corr-1738012345678-a1b2c3d4",
  "status": "complete", // or "timeout"
  "result": {...} // or error
}
```

---

## Phase 3: LLM Result Retrieval

### New observe mode: `command_result`

```javascript
observe({what: "command_result", correlation_id: "corr-1738012345678-a1b2c3d4"})

// Returns one of:
{
  "status": "pending",
  "message": "Command still executing in browser"
}

{
  "status": "complete",
  "result": {"success": true, "data": "..."},
  "completed_at": "2026-01-27T10:15:30Z"
}

{
  "status": "timeout",
  "error": "JavaScript execution exceeded 10s",
  "failed_at": "2026-01-27T10:15:40Z"
}

{
  "status": "expired",
  "error": "Result expired after 60s (not retrieved in time)",
  "expired_at": "2026-01-27T10:16:00Z"
}
```

### New observe mode: `pending_commands`

```javascript
observe({what: "pending_commands"})

// Returns:
{
  "pending": [
    {"correlation_id": "corr-123", "created_at": "...", "command": "execute_js"}
  ],
  "completed": [
    {"correlation_id": "corr-456", "completed_at": "...", "duration_ms": 1523}
  ],
  "failed": [
    {"correlation_id": "corr-789", "error": "timeout", "failed_at": "..."}
  ]
}
```

### New observe mode: `failed_commands`

```javascript
observe({what: "failed_commands"})

// Returns recent failures:
[
  {
    "correlation_id": "corr-789",
    "error": "extension_no_response",
    "expired_at": "2026-01-27T10:15:45Z",
    "hint": "Extension did not respond within 3s of receiving command"
  },
  {
    "correlation_id": "corr-012",
    "error": "execution_timeout",
    "expired_at": "2026-01-27T10:16:10Z",
    "hint": "JavaScript execution exceeded 10s"
  }
]
```

---

## Phase 4: Server State Management

### Data Structures

```go
type CommandResult struct {
    CorrelationID string
    Status        string // "pending", "complete", "timeout"
    Result        json.RawMessage
    Error         string
    CompletedAt   time.Time
    CreatedAt     time.Time
}

type Capture struct {
    // ... existing fields ...

    // Async command tracking
    completedResults map[string]*CommandResult  // correlation_id → result
    failedCommands   []*CommandResult            // circular buffer, max 100
    resultsMu        sync.RWMutex
}
```

### Cleanup Goroutine (60s TTL)

```go
func (c *Capture) startResultCleanup() {
    ticker := time.NewTicker(10 * time.Second)
    go func() {
        for range ticker.C {
            c.resultsMu.Lock()
            now := time.Now()
            for correlationID, result := range c.completedResults {
                age := now.Sub(result.CompletedAt)
                if age > 60*time.Second {
                    // Move to failed_commands if never retrieved
                    c.failedCommands = append(c.failedCommands, &CommandResult{
                        CorrelationID: correlationID,
                        Status:        "expired",
                        Error:         "Result expired after 60s (LLM never retrieved)",
                        CompletedAt:   result.CompletedAt,
                    })
                    delete(c.completedResults, correlationID)

                    // Log for observability
                    log.Printf("[gasoline] Expired unretrieved result: correlation_id=%s", correlationID)
                }
            }
            c.resultsMu.Unlock()
        }
    }()
}
```

---

## Timeout Behavior

| Timeout | Trigger | Action |
|---------|---------|--------|
| **3s** | Extension never responds to queued command | Mark as `extension_no_response`, add to failed_commands |
| **10s** | Extension sends "pending" but never completes | Extension posts timeout error result |
| **60s** | Result completed but LLM never retrieves | Expire result, move to failed_commands |

---

## Extension Changes Required

### 1. Handle correlation_id in queries
```javascript
// extension/background.js or service-worker.js
async function processPendingQuery(query) {
    const {id, correlation_id, type, params} = query;

    // Start execution
    const startTime = Date.now();
    const promise = executeCommand(type, params);

    // 2s decision point
    const timer = setTimeout(() => {
        if (!completed) {
            // Post "pending" status
            fetch(`${SERVER_URL}/query-result`, {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    correlation_id: correlation_id,
                    status: 'pending'
                })
            });
        }
    }, 2000);

    // Wait for execution (up to 10s total)
    try {
        const result = await Promise.race([
            promise,
            new Promise((_, reject) =>
                setTimeout(() => reject(new Error('Execution timeout')), 10000)
            )
        ]);

        clearTimeout(timer);

        // Post complete result
        fetch(`${SERVER_URL}/query-result`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                correlation_id: correlation_id,
                status: 'complete',
                result: result
            })
        });
    } catch (err) {
        clearTimeout(timer);

        // Post timeout/error result
        fetch(`${SERVER_URL}/query-result`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                correlation_id: correlation_id,
                status: 'timeout',
                error: err.message
            })
        });
    }
}
```

### 2. Update POST /query-result handler
```go
// Accept correlation_id instead of just query ID
type QueryResult struct {
    CorrelationID string          `json:"correlation_id"`
    Status        string          `json:"status"` // "complete", "pending", "timeout"
    Result        json.RawMessage `json:"result,omitempty"`
    Error         string          `json:"error,omitempty"`
}

func (h *HTTPHandler) handleQueryResult(w http.ResponseWriter, r *http.Request) {
    var result QueryResult
    json.NewDecoder(r.Body).Decode(&result)

    h.capture.SetCommandResult(result)
}
```

---

## Backward Compatibility

**Breaking change:** Yes, extension must be updated to handle correlation_id.

**Migration path:**
1. Server generates correlation_id for all commands (new behavior)
2. Extension updated to read correlation_id from queries
3. Old extension versions will fail gracefully (no correlation_id → error)

**Version check:** Bump to v6.0.0 (major version for breaking change)

---

## Testing Requirements

1. **Fast path (< 2s):** Command completes immediately, LLM gets result on first poll
2. **Slow path (> 2s, < 10s):** LLM receives "pending", polls again, gets result
3. **Timeout (> 10s):** Extension posts timeout error, LLM observes failure
4. **Extension disconnect:** Server expires command after 3s, adds to failed_commands
5. **Result expiration:** LLM never retrieves result, expires after 60s
6. **Multiple concurrent commands:** Correlation IDs remain isolated

---

## Implementation Phases

### Phase 1: Server-side infrastructure (this session)
- [ ] Add CorrelationID to PendingQuery
- [ ] Modify interact tool to return correlation_id immediately
- [ ] Add completedResults and failedCommands tracking
- [ ] Add observe modes: command_result, pending_commands, failed_commands
- [ ] Add cleanup goroutine

### Phase 2: Extension updates (next session)
- [ ] Update query processing to handle correlation_id
- [ ] Implement 2s/10s timeout logic
- [ ] Update POST /query-result to use correlation_id

### Phase 3: Testing & documentation
- [ ] Add tests for async behavior
- [ ] Update MCP integration docs
- [ ] Add examples to UAT plan

---

## Open Questions

1. **Result retention:** 60s confirmed as default TTL ✅
2. **Batch retrieval:** `pending_commands` confirmed ✅
3. **Failed commands buffer size:** 100 entries (circular buffer)
4. **Backward compat:** Breaking change, bump to v6.0.0 ✅

