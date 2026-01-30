# Large Data Issue - Analysis & Solutions

## Problem Statement

**Issue**: `observe({what: "network_waterfall"})` returns >440K characters, exceeding MCP token limits

**Root Cause**: Network waterfall ring buffer accumulates data from all browser tabs (43 tabs currently open), with configurable capacity of 1000 entries (default).

**Impact**:
- Cannot view network waterfall data
- MCP response token limit exceeded
- AI cannot analyze network traffic

---

## Current State

### What EXISTS ✅

1. **Buffer Clearing**
   ```javascript
   configure({action: "clear"})
   ```
   - ✅ Clears browser console logs
   - ❌ Does NOT clear network waterfall
   - ❌ Does NOT clear other buffers (websocket, actions)

2. **Filtering** (Applied but still processes all data)
   - `url: "substring"` - Filter by URL
   - `status_min: 400` - Minimum status code
   - `status_max: 499` - Maximum status code
   - `method: "POST"` - Filter by HTTP method

3. **Ring Buffer** (Auto-eviction)
   - networkWaterfall: 1000 entries (configurable capacity)
   - wsEvents: 500 entries
   - networkBodies: 100 entries
   - enhancedActions: 50 entries

### What's MISSING ❌

1. **Pagination**
   - No `offset` parameter
   - No `limit` parameter
   - No `page` / `page_size` parameters

2. **Selective Buffer Clearing**
   ```javascript
   // These DON'T exist:
   configure({action: "clear", buffer: "network"})
   configure({action: "clear", buffer: "websocket"})
   configure({action: "clear", buffer: "actions"})
   ```

3. **Streaming/Chunking**
   - No ability to stream results
   - No chunked responses
   - All-or-nothing data return

4. **Query Capabilities**
   - No SQL-like queries
   - No aggregation functions
   - No GROUP BY / ORDER BY / COUNT

5. **Embedded DB**
   - No SQLite integration
   - No indexing
   - No persistent storage

---

## Solutions (Ordered by Priority)

### 1. Add Pagination (HIGH PRIORITY) ⭐⭐⭐

**Impact**: IMMEDIATE FIX for token limit issues

**Implementation**:
```go
// cmd/dev-console/tools.go
func (h *ToolHandler) toolGetNetworkWaterfall(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
    var arguments struct {
        URL       string `json:"url"`
        Method    string `json:"method"`
        StatusMin int    `json:"status_min"`
        StatusMax int    `json:"status_max"`
        Offset    int    `json:"offset"`  // NEW
        Limit     int    `json:"limit"`   // NEW (default 100)
    }

    // Apply filters
    filtered := applyFilters(allEntries, arguments)

    // Apply pagination
    if arguments.Limit == 0 {
        arguments.Limit = 100  // Default limit
    }

    start := arguments.Offset
    end := min(start + arguments.Limit, len(filtered))

    if start >= len(filtered) {
        return emptyResult()
    }

    page := filtered[start:end]

    // Return with pagination metadata
    return paginatedResponse(page, start, end, len(filtered))
}
```

**Usage**:
```javascript
// Get first 100 entries
observe({what: "network_waterfall", limit: 100})

// Get next 100 entries
observe({what: "network_waterfall", offset: 100, limit: 100})

// Get entries 200-300
observe({what: "network_waterfall", offset: 200, limit: 100})
```

**Benefits**:
- ✅ Solves token limit issue immediately
- ✅ Minimal code changes
- ✅ Backward compatible (limit defaults to 100)
- ✅ Works with existing filters

**Effort**: 2-4 hours

---

### 2. Add Buffer-Specific Clear (HIGH PRIORITY) ⭐⭐⭐

**Impact**: Allows clearing specific buffers without restarting server

**Implementation**:
```go
// cmd/dev-console/tools.go
func (h *ToolHandler) toolClearBuffer(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
    var arguments struct {
        Buffer string `json:"buffer"`  // "network", "websocket", "actions", "all"
    }

    switch arguments.Buffer {
    case "network":
        h.capture.mu.Lock()
        h.capture.networkWaterfall = nil
        h.capture.mu.Unlock()
        return success("Network waterfall cleared")

    case "websocket":
        h.capture.mu.Lock()
        h.capture.wsEvents = nil
        h.capture.wsAddedAt = nil
        h.capture.mu.Unlock()
        return success("WebSocket events cleared")

    case "actions":
        h.capture.mu.Lock()
        h.capture.enhancedActions = nil
        h.capture.actionAddedAt = nil
        h.capture.mu.Unlock()
        return success("Actions cleared")

    case "all":
        h.clearAllBuffers()
        return success("All buffers cleared")

    default:
        return error("Invalid buffer. Use: network, websocket, actions, or all")
    }
}
```

**Usage**:
```javascript
configure({action: "clear", buffer: "network"})
configure({action: "clear", buffer: "websocket"})
configure({action: "clear", buffer: "actions"})
configure({action: "clear", buffer: "all"})
```

**Benefits**:
- ✅ Granular buffer control
- ✅ No server restart needed
- ✅ Useful for testing/debugging
- ✅ Prevents memory bloat

**Effort**: 1-2 hours

---

### 3. Add Server-Side Aggregation (MEDIUM PRIORITY) ⭐⭐

**Impact**: Reduce data volume before returning to MCP

**Implementation**:
```go
func (h *ToolHandler) toolGetNetworkStats(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
    var arguments struct {
        GroupBy string `json:"group_by"`  // "host", "status", "method"
    }

    stats := aggregateNetworkData(h.capture.networkWaterfall, arguments.GroupBy)

    return JSONRPCResponse{
        JSONRPC: "2.0",
        ID: req.ID,
        Result: mcpJSONResponse("Network statistics", stats),
    }
}
```

**Usage**:
```javascript
// Get stats by host
observe({what: "network_stats", group_by: "host"})
→ {
    "localhost:3000": {count: 150, avg_duration: 45ms, errors: 3},
    "api.example.com": {count: 50, avg_duration: 120ms, errors: 0}
  }

// Get stats by status code
observe({what: "network_stats", group_by: "status"})
→ {
    "200": {count: 180},
    "404": {count: 15},
    "500": {count: 5}
  }
```

**Benefits**:
- ✅ Compact representation of large datasets
- ✅ Useful for overview/analysis
- ✅ No token limit issues

**Effort**: 4-6 hours

---

### 4. Add Embedded SQLite (LOW PRIORITY) ⭐

**Impact**: Advanced querying, but significant complexity

**Why NOT Recommended (Yet)**:
- ❌ High complexity (schema design, migrations, indexes)
- ❌ Performance overhead (serialization to/from DB)
- ❌ Persistence questions (in-memory vs on-disk?)
- ❌ Adds external dependency (CGO for mattn/go-sqlite3 or modernc.org/sqlite)
- ⚠️ Premature optimization (pagination solves 90% of the problem)

**When to Reconsider**:
- If pagination + aggregation aren't sufficient
- If you need complex queries (JOIN, subqueries, window functions)
- If you want persistent storage across server restarts
- If you need full-text search capabilities

**Effort**: 20-40 hours (initial implementation + testing)

---

## Recommended Immediate Actions

### Phase 1: Quick Wins (1-2 days) ✅

1. **Add Pagination to network_waterfall**
   - Add `offset` and `limit` parameters
   - Default limit: 100 entries
   - Return pagination metadata (total, offset, limit)

2. **Add Buffer-Specific Clear**
   - Extend `configure({action: "clear", buffer: "..."})`
   - Support: network, websocket, actions, all

3. **Document New Features**
   - Update UAT test plan
   - Update MCP tool descriptions
   - Add examples to README

### Phase 2: Enhancements (1 week)

4. **Add Aggregation/Stats Endpoints**
   - `observe({what: "network_stats", group_by: "host"})`
   - `observe({what: "websocket_stats"})`
   - `observe({what: "action_stats"})`

5. **Add Response Size Limits**
   - Warn if response > 100K chars
   - Auto-apply limit to prevent token overflow
   - Suggest pagination in warning message

### Phase 3: Advanced (Future)

6. **Consider SQLite** (only if needed)
   - Evaluate use cases
   - Design schema
   - Implement with feature flag

---

## Implementation Priority

| Feature | Priority | Effort | Impact | Implement? |
|---------|----------|--------|--------|------------|
| Pagination | HIGH | 2-4h | ⭐⭐⭐ | ✅ YES - NOW |
| Buffer Clear | HIGH | 1-2h | ⭐⭐⭐ | ✅ YES - NOW |
| Aggregation | MEDIUM | 4-6h | ⭐⭐ | ✅ YES - SOON |
| SQLite | LOW | 20-40h | ⭐ | ❌ NO - LATER |

---

## Workarounds (Available Now)

### Workaround 1: Use Filtering
```javascript
// Instead of all entries:
observe({what: "network_waterfall"})

// Filter to specific domain:
observe({what: "network_waterfall", url: "localhost:3000"})

// Filter to errors only:
observe({what: "network_waterfall", status_min: 400})
```

### Workaround 2: Restart Server
```bash
# Kill old server
kill <PID>

# Start fresh
./gasoline --port 7890
```

### Workaround 3: Close Browser Tabs
- Close unused tabs (you have 43 open!)
- Reduces network traffic captured
- Reduces buffer size

### Workaround 4: Query Direct HTTP Endpoint
```bash
# Use curl with jq for filtering
curl -s http://localhost:7890/network-waterfall | jq '.[:10]'
```

---

## Code Locations

### Files to Modify

1. **cmd/dev-console/tools.go**
   - Line 1279: `toolGetNetworkWaterfall()` - Add pagination
   - Line 1714: `toolClearBrowserLogs()` - Extend to support buffer param
   - Line 1447: `toolConfigure()` - Add buffer clearing case

2. **cmd/dev-console/types.go**
   - Line 538: `networkWaterfall` field - Already a ring buffer
   - No changes needed (capacity already configurable)

3. **cmd/dev-console/testdata/mcp-tools-list.golden.json**
   - Update tool descriptions with new parameters

4. **docs/core/UAT-TEST-PLAN-V2.md**
   - Add pagination testing
   - Add buffer clearing testing

---

## Testing Plan

### Unit Tests
```go
func TestNetworkWaterfallPagination(t *testing.T) {
    // Test: offset + limit returns correct slice
    // Test: offset beyond length returns empty
    // Test: limit defaults to 100
    // Test: pagination metadata correct
}

func TestClearSpecificBuffer(t *testing.T) {
    // Test: clear network doesn't clear websocket
    // Test: clear all clears everything
    // Test: invalid buffer returns error
}
```

### Integration Tests
```javascript
// Test: Pagination
const page1 = await observe({what: "network_waterfall", limit: 10})
const page2 = await observe({what: "network_waterfall", offset: 10, limit: 10})
assert(page1.data.length === 10)
assert(page2.data.length === 10)

// Test: Buffer clearing
await configure({action: "clear", buffer: "network"})
const result = await observe({what: "network_waterfall"})
assert(result.count === 0)
```

---

## Conclusion

**Recommended Solution**: Implement pagination + buffer clearing (Phase 1)

**Timeline**: 1-2 days
**Effort**: 3-6 hours
**Impact**: Solves 90% of large data issues

Pagination is the right solution because:
- ✅ Simple to implement
- ✅ Minimal code changes
- ✅ Backward compatible
- ✅ Solves token limit problem
- ✅ Standard pattern (used everywhere)
- ✅ Works with existing filters

**DO NOT** implement SQLite yet - it's premature optimization. Pagination will handle your use case.

---

## Next Steps

1. Create feature spec: `docs/features/feature/pagination/TECH_SPEC.md`
2. Implement pagination in `cmd/dev-console/tools.go`
3. Implement buffer clearing
4. Add tests
5. Update documentation
6. Test with large datasets
7. Commit and push

Would you like me to implement pagination now?
