# v5.3 UAT Checklist

**Release:** v5.3.0
**Focus:** Pagination + Buffer Clearing + Version Checking + Edge Cases
**Date:** 2026-01-30
**Test Coverage:** EXTRA THOROUGH - Regression prevention focus

---

## Executive Summary

This UAT checklist provides comprehensive test coverage for v5.3.0, with extra emphasis on edge cases, performance, and regression prevention based on issues in previous releases.

**New Features:**
1. **Cursor-based pagination** - Stable pagination for logs, errors, actions, websocket_events with buffer overflow handling
2. **Buffer-specific clearing** - Granular control over clearing network, websocket, actions, logs buffers individually
3. **tracked_tab_id metadata** - All JSON handlers include tracked_tab_id when extension tracking is active
4. **GitHub version checking** - Automatic checks for newer releases with caching and extension badge

**Breaking Changes:**
- `observe({what: "errors"})` now returns JSON format instead of markdown (clients must update)

**Test Categories:**
- ✅ **Critical Feature Tests** - Core v5.3 functionality (4 sections, ~40 tests)
- ✅ **Edge Cases & Regression Tests** - Boundary conditions, error handling (~80 tests)
- ✅ **Performance & Scalability** - Load testing, memory leak detection (~15 tests)
- ✅ **Backward Compatibility** - Ensures old clients still work (~8 tests)
- ✅ **Data Integrity** - Correctness verification (~10 tests)
- ✅ **Error Handling** - Malformed input, validation (~12 tests)
- ✅ **Security & Privacy** - Information disclosure checks (~5 tests)
- ✅ **Multi-Client Isolation** - Concurrent access testing (~5 tests)
- ✅ **Cross-Browser Testing** - Extension compatibility (~3 browsers)
- ✅ **Smoke Tests** - Quick sanity checks (5-minute test)
- ✅ **Critical Path** - End-to-end workflow verification

**Total Test Count:** ~180+ individual test cases

**Estimated Test Time:**
- Quick smoke test: 5 minutes
- Critical features only: 30 minutes
- Full comprehensive UAT: 2-3 hours

---

## Pre-Deployment Checklist

- [ ] All Pre-UAT Quality Gates passed (see main UAT plan)
- [ ] `git status` shows clean working directory
- [ ] Current branch: `next`
- [ ] All commits pushed to remote
- [ ] All Go tests pass: `make test`
- [ ] Extension tests pass: `node --test tests/extension/*.test.js`
- [ ] TypeScript compiles: `make compile-ts`
- [ ] No console errors when loading extension in Chrome
- [ ] Server starts without errors: `./dist/gasoline --port 7890`

---

## Critical Feature Tests (v5.3)

### 1. Cursor-Based Pagination ⭐⭐⭐

**Test with logs handler:**
```javascript
// Get first page (default: last 100 entries)
observe({what: "logs"})
// → Verify: Returns JSON with cursor, count, total, oldest_timestamp, newest_timestamp

// Get next page using after_cursor
observe({what: "logs", after_cursor: "<cursor_from_previous>", limit: 50})
// → Verify: Returns older entries, has_more: true if more data exists

// Get newer entries using before_cursor
observe({what: "logs", before_cursor: "<cursor>", limit: 50})
// → Verify: Returns newer entries

// Get all entries since a timestamp
observe({what: "logs", since_cursor: "<cursor>", limit: 100})
// → Verify: Returns all entries >= cursor timestamp
```

**Test cursor restart on eviction:**
```javascript
// Simulate buffer overflow (add >10k logs to fill buffer, evict old ones)
// Then try to fetch with expired cursor
observe({what: "logs", after_cursor: "<old_evicted_cursor>", restart_on_eviction: true})
// → Verify: Returns cursor_restarted: true, warning message, starts from oldest available
```

**Test with websocket_events:**
```javascript
observe({what: "websocket_events", limit: 10})
// → Verify: Cursor pagination works for WebSocket events

observe({what: "websocket_events", after_cursor: "<cursor>", limit: 10})
// → Verify: Returns older WS events
```

**Test with actions:**
```javascript
observe({what: "actions", limit: 5})
// → Verify: Cursor pagination works for user actions

observe({what: "actions", after_cursor: "<cursor>", limit: 5})
// → Verify: Returns older actions
```

**Test with errors (JSON format):**
```javascript
observe({what: "errors"})
// → Verify: Returns JSON format (not markdown)
// → Verify: Has cursor, count, total fields
// → Verify: errors array with level, message, source, timestamp, sequence

observe({what: "errors", limit: 5})
// → Verify: Pagination works for errors
```

---

### 2. tracked_tab_id Metadata ⭐⭐

**Prerequisites:**
- Open a page in Chrome with Gasoline extension
- Extension should be tracking a specific tab

**Test all JSON observe handlers:**
```javascript
observe({what: "logs"})
// → Verify: Response includes tracked_tab_id field if tracking is active

observe({what: "errors"})
// → Verify: Response includes tracked_tab_id field

observe({what: "actions"})
// → Verify: Response includes tracked_tab_id field

observe({what: "websocket_events"})
// → Verify: Response includes tracked_tab_id field

observe({what: "websocket_status"})
// → Verify: Response includes tracked_tab_id field

observe({what: "network_waterfall"})
// → Verify: Response includes tracked_tab_id field
```

**Test without tracking:**
```javascript
// Disable tracking in extension or navigate to different tab
observe({what: "logs"})
// → Verify: tracked_tab_id field NOT present (or 0)
```

---

### 3. Buffer-Specific Clearing ⭐⭐⭐

**Test network buffer clearing:**
```javascript
// Generate some network traffic (navigate pages, trigger API calls)
observe({what: "network_waterfall"})
// → Note: Count of entries

configure({action: "clear", buffer: "network"})
// → Verify: Returns JSON with cleared: "network", counts: {network_waterfall: N, network_bodies: M}, total_cleared: X

observe({what: "network_waterfall"})
// → Verify: Empty or much fewer entries
```

**Test websocket buffer clearing:**
```javascript
observe({what: "websocket_events"})
// → Note: Count of events

configure({action: "clear", buffer: "websocket"})
// → Verify: Returns cleared: "websocket", counts: {websocket_events: N, websocket_status: M}

observe({what: "websocket_events"})
// → Verify: Empty
```

**Test actions buffer clearing:**
```javascript
observe({what: "actions"})
// → Note: Count of actions

configure({action: "clear", buffer: "actions"})
// → Verify: Returns cleared: "actions", counts: {actions: N}

observe({what: "actions"})
// → Verify: Empty
```

**Test logs buffer clearing:**
```javascript
observe({what: "logs"})
// → Note: Count of logs

configure({action: "clear", buffer: "logs"})
// → Verify: Returns cleared: "logs", counts: {logs: N, extension_logs: M}

observe({what: "logs"})
// → Verify: Empty
```

**Test clear all buffers:**
```javascript
configure({action: "clear", buffer: "all"})
// → Verify: Returns cleared: "all", counts object with all buffer types, total_cleared sum

// Verify all buffers empty
observe({what: "network_waterfall"}) // → Empty
observe({what: "websocket_events"})  // → Empty
observe({what: "actions"})           // → Empty
observe({what: "logs"})              // → Empty
```

**Test backward compatibility:**
```javascript
configure({action: "clear"})
// → Verify: Defaults to buffer: "logs", returns cleared: "logs"
// → Verify: Only logs cleared, other buffers unchanged
```

**Test invalid buffer name:**
```javascript
configure({action: "clear", buffer: "invalid"})
// → Verify: Returns error with isError: true
// → Verify: Error message contains "Invalid buffer"
```

---

### 4. Version Checking ⭐⭐

**Server-side checks:**
```bash
# Start Gasoline server
./dist/gasoline --port 7890

# Check logs for version check messages
# → Expect: "[gasoline] New version available: vX.Y.Z (current: vA.B.C)" if newer version exists
# → OR: No message if current version is latest
```

**Health endpoint check:**
```bash
curl http://localhost:7890/health | jq .
# → Verify: Contains "version" field with current server version
# → Verify: Contains "availableVersion" field if newer version exists (or null)
```

**Extension badge check:**
- Install extension
- Connect to server
- Check extension icon:
  - If newer version available: Should show ⬆ badge
  - If current version latest: No badge
  - Hover over badge: Should show "Gasoline: New version available (vX.Y.Z)"

**Cache behavior:**
```bash
# Wait 10 seconds, check logs
# → Should NOT check GitHub again (6-hour cache)

# Wait 6+ hours (or restart server), check logs
# → Should check GitHub again
```

---

## Backward Compatibility Tests ⭐⭐⭐

**Old MCP clients (no cursor parameters):**
```javascript
observe({what: "logs"})
// → Verify: Works (returns last N entries with cursor metadata)
// → Verify: Response includes new cursor fields (forward compatible)

observe({what: "logs", limit: 100})
// → Verify: Works (returns last 100 entries)
// → Verify: No breaking changes to response structure
```

**Old clear syntax:**
```javascript
configure({action: "clear"})
// → Verify: Works (defaults to buffer: "logs")
// → Verify: Only logs cleared (not all buffers)
// → Verify: Response includes new fields (forward compatible)
```

**Old error handler clients (BREAKING CHANGE):**
```javascript
observe({what: "errors"})
// → Verify: Returns JSON format (NOT markdown table)
// → BREAKING: Clients expecting markdown will need updates
// → Verify: New format includes all necessary error information
// → Verify: cursor, count, total metadata present
```

**API contract stability:**
```javascript
// Verify all existing parameters still work
observe({what: "logs", limit: 50})
observe({what: "network_waterfall", url: "example.com"})
observe({what: "websocket_events", direction: "incoming"})
observe({what: "actions", last_n: 10})

// → Verify: All existing parameters still function
// → Verify: No unexpected parameter deprecations
```

---

## Data Integrity & Correctness Tests ⭐⭐⭐

**Cursor accuracy:**
```javascript
// Generate known dataset
// → 100 logs with specific timestamps and sequences

observe({what: "logs", limit: 50})
// → Save cursor C1 (points to entry 50)

observe({what: "logs", after_cursor: "<C1>", limit: 50})
// → Verify: Returns entries 1-49 (correct older entries)
// → Verify: Does NOT include entry 50 (cursor boundary correct)
// → Verify: Does NOT include entries 51-100 (correct direction)
```

**Sequence monotonicity:**
```javascript
observe({what: "logs"})
// → Verify: Sequence numbers are strictly increasing
// → Verify: No gaps in sequence for buffer contents
// → Verify: First sequence = logTotalAdded - bufferLength + 1
```

**Timestamp consistency:**
```javascript
observe({what: "logs"})
// → Verify: oldest_timestamp <= all entry timestamps <= newest_timestamp
// → Verify: Timestamps in RFC3339 format
// → Verify: Timestamps match actual entry creation times
```

**Count accuracy:**
```javascript
observe({what: "logs"})
// → Verify: count field = actual array length
// → Verify: total field = actual buffer size
// → Verify: count <= total always
```

**has_more accuracy:**
```javascript
// Buffer has 100 entries
observe({what: "logs", limit: 50})
// → Verify: has_more = false (no after_cursor, so no "more" concept)

observe({what: "logs", limit: 100})
// → Verify: has_more = false (all entries returned)

observe({what: "logs", after_cursor: "<middle_cursor>", limit: 10})
// If 20 older entries exist:
// → Verify: has_more = true (more older entries available)

// If only 5 older entries exist:
// → Verify: has_more = false (all older entries returned)
```

**Buffer capacity enforcement:**
```javascript
// Fill logs buffer beyond capacity (add 2000 entries to 1000-capacity buffer)
observe({what: "logs"})
// → Verify: count + total <= configured capacity
// → Verify: Oldest entries evicted (buffer is ring buffer)
// → Verify: Newest entries preserved
```

---

## Error Handling & Validation Tests ⭐⭐⭐

**Malformed JSON requests:**
```javascript
// Send invalid JSON to MCP server
{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": "observe", "arguments": "{INVALID}"}}
// → Verify: Returns JSON-RPC error response
// → Verify: Error code and message present
// → Verify: Server does not crash
```

**Missing required parameters:**
```javascript
observe({})
// → Verify: Error response (missing "what" parameter)

configure({action: "store"})
// → Verify: Error response (missing required store parameters)
```

**Type mismatches:**
```javascript
observe({what: "logs", limit: "abc"})
// → Verify: Handles gracefully (parses as 0 or returns error)

observe({what: "logs", limit: 3.14})
// → Verify: Accepts float, converts to int, or returns error
```

**SQL injection attempts (paranoid):**
```javascript
observe({what: "logs", after_cursor: "'; DROP TABLE logs; --"})
// → Verify: Treated as invalid cursor (parsing fails safely)
// → Verify: No SQL injection possible (we don't use SQL)
```

**Null/undefined handling:**
```javascript
observe({what: "logs", limit: null})
// → Verify: Treats as no limit or returns error

observe({what: "logs", after_cursor: null})
// → Verify: Treats as no cursor or returns error

configure({action: "clear", buffer: null})
// → Verify: Defaults to "logs" or returns error
```

**Very long strings:**
```javascript
observe({what: "logs", after_cursor: "A".repeat(10000)})
// → Verify: Handles gracefully (returns error, no crash)
// → Verify: Response time reasonable (no algorithmic explosion)
```

---

## Security & Privacy Tests ⭐⭐

**tracked_tab_id information disclosure:**
```javascript
// Verify tracked_tab_id does NOT leak sensitive information
observe({what: "logs"})
// → Verify: tracked_tab_id is just a tab ID (integer)
// → Verify: No URL, title, or sensitive tab data leaked
```

**Buffer isolation (multi-user safety):**
```javascript
// Verify buffers are process-global (expected behavior)
// → All MCP sessions see same buffer data
// → This is by design for observability

// But session data should be isolated
configure({action: "store", store_action: "save", key: "secret", data: {token: "ABC"}})
// → Another session should NOT access this data (verify isolation)
```

**Version checking privacy:**
```bash
# Verify version check ONLY contacts GitHub
# → No telemetry sent
# → No user data transmitted
# → Only checks public GitHub releases API
```

---

## Edge Cases & Regression Tests ⭐⭐⭐

### Pagination Edge Cases

**Empty buffer pagination:**
```javascript
configure({action: "clear", buffer: "logs"})
observe({what: "logs"})
// → Verify: Empty array, count: 0, total: 0
// → Verify: No cursor field (or cursor: "")
// → Verify: No crash, clean response

observe({what: "logs", limit: 100})
// → Verify: Same result (empty with limit)

observe({what: "logs", after_cursor: "2026-01-30T10:00:00Z:1"})
// → Verify: Empty result (no error for cursor on empty buffer)
```

**Single entry pagination:**
```javascript
// Generate exactly 1 log entry
observe({what: "logs"})
// → Verify: count: 1, total: 1
// → Verify: Cursor points to that single entry
// → Verify: has_more: false

observe({what: "logs", after_cursor: "<cursor_from_above>"})
// → Verify: Empty result (no older entries)
```

**Limit edge cases:**
```javascript
observe({what: "logs", limit: 0})
// → Verify: Returns all available entries (limit=0 means no limit)

observe({what: "logs", limit: -1})
// → Verify: Handles gracefully (should treat as no limit or return error)

observe({what: "logs", limit: 999999999})
// → Verify: Caps at buffer capacity, does NOT allocate huge memory
// → Verify: Response time < 500ms even with max buffer
```

**Invalid cursor formats:**
```javascript
observe({what: "logs", after_cursor: "invalid"})
// → Verify: Returns error with isError: true
// → Verify: Error message contains "invalid cursor format"

observe({what: "logs", after_cursor: "2026-01-30T10:00:00Z"})
// → Verify: Error (missing sequence part)

observe({what: "logs", after_cursor: "not-a-timestamp:123"})
// → Verify: Error (invalid timestamp)

observe({what: "logs", after_cursor: "2026-01-30T10:00:00Z:abc"})
// → Verify: Error (non-numeric sequence)

observe({what: "logs", after_cursor: "2026-01-30T10:00:00Z:123:extra"})
// → Verify: Error (extra colon creates invalid timestamp)
```

**Timestamp collision handling:**
```javascript
// Generate 10 logs with identical timestamps (same millisecond)
// → Verify: Each gets unique sequence number
// → Verify: Pagination respects sequence ordering
// → Verify: after_cursor correctly uses sequence as tiebreaker
```

**Cursor expiration - critical buffer overflow test:**
```javascript
// Fill buffer to capacity (e.g., 1000+ logs to force eviction)
// Note first entry's cursor
observe({what: "logs"})
// → Save oldest cursor

// Add 2000 more logs to force buffer overflow
// → Verify: Buffer evicts oldest entries

// Try to use old (evicted) cursor WITHOUT restart
observe({what: "logs", after_cursor: "<old_evicted_cursor>"})
// → Verify: Returns error with "cursor expired"
// → Verify: Error mentions sequence gap (lost entries)

// Try to use old cursor WITH restart
observe({what: "logs", after_cursor: "<old_evicted_cursor>", restart_on_eviction: true})
// → Verify: Returns cursor_restarted: true
// → Verify: Warning message present
// → Verify: Returns from oldest available entry (not error)
// → Verify: original_cursor field contains old cursor
```

**Boundary cursor positions:**
```javascript
// Get current data
observe({what: "logs"})
// → Save first and last entry cursors

// Cursor at very beginning
observe({what: "logs", after_cursor: "<first_entry_cursor>"})
// → Verify: Returns empty (no older entries)

// Cursor at very end
observe({what: "logs", after_cursor: "<last_entry_cursor>"})
// → Verify: Returns all but last entry

// before_cursor at beginning
observe({what: "logs", before_cursor: "<first_entry_cursor>"})
// → Verify: Returns all but first entry

// before_cursor at end
observe({what: "logs", before_cursor: "<last_entry_cursor>"})
// → Verify: Returns empty (no newer entries)
```

**since_cursor edge cases:**
```javascript
observe({what: "logs", since_cursor: "<middle_cursor>"})
// → Verify: Includes the cursor entry itself (>=, not >)
// → Verify: Returns all entries from cursor onwards

observe({what: "logs", since_cursor: "<oldest_cursor>"})
// → Verify: Returns entire buffer

observe({what: "logs", since_cursor: "<newest_cursor>"})
// → Verify: Returns only the newest entry
```

**Multiple cursor parameters (error handling):**
```javascript
observe({what: "logs", after_cursor: "cursor1", before_cursor: "cursor2"})
// → Verify: Uses after_cursor (precedence order: after > before > since)
// → Verify: Does NOT error (ignores conflicting parameters)
```

**Pagination across all supported handlers:**
```javascript
// Test pagination works identically for:
observe({what: "logs", limit: 10})
observe({what: "errors", limit: 10})
observe({what: "actions", limit: 10})
observe({what: "websocket_events", limit: 10})

// → Verify: All return cursor, count, total, has_more fields
// → Verify: All respect limit parameter
// → Verify: All support after_cursor, before_cursor, since_cursor
```

---

### Buffer Clearing Edge Cases

**Empty buffer clearing:**
```javascript
// Start with empty buffers
configure({action: "clear", buffer: "all"})
// → Verify: Returns counts: 0 for all buffers, total_cleared: 0
// → Verify: No error, no crash

configure({action: "clear", buffer: "network"})
// → Verify: network_waterfall: 0, network_bodies: 0

configure({action: "clear", buffer: "websocket"})
// → Verify: websocket_events: 0, websocket_status: 0

configure({action: "clear", buffer: "actions"})
// → Verify: actions: 0

configure({action: "clear", buffer: "logs"})
// → Verify: logs: 0, extension_logs: 0
```

**Repeated clearing (idempotency):**
```javascript
// Clear same buffer multiple times
configure({action: "clear", buffer: "network"})
configure({action: "clear", buffer: "network"})
configure({action: "clear", buffer: "network"})
// → Verify: All return counts: 0 after first clear
// → Verify: No errors, no state corruption
```

**Memory verification after clearing:**
```javascript
// Fill network bodies buffer (large payloads)
// → Generate 100+ network requests with large bodies

configure({action: "health"})
// → Note: memory.nbMemoryTotal value (should be > 0)

configure({action: "clear", buffer: "network"})

configure({action: "health"})
// → Verify: memory.nbMemoryTotal = 0 (memory actually freed)
// → Verify: buffers.network_bodies = 0

// Fill websocket buffer
configure({action: "health"})
// → Note: memory.wsMemoryTotal value

configure({action: "clear", buffer: "websocket"})

configure({action: "health"})
// → Verify: memory.wsMemoryTotal = 0
```

**Buffer clearing isolation:**
```javascript
// Add data to all buffers
// → Network: 10 entries
// → WebSocket: 5 entries
// → Actions: 8 entries
// → Logs: 15 entries

configure({action: "clear", buffer: "network"})
// → Verify: Only network cleared

observe({what: "websocket_events"})
// → Verify: Still has 5 entries (not affected)

observe({what: "actions"})
// → Verify: Still has 8 entries

observe({what: "logs"})
// → Verify: Still has 15 entries
```

**Invalid buffer parameter:**
```javascript
configure({action: "clear", buffer: "invalid_buffer_name"})
// → Verify: Returns error with isError: true
// → Verify: Error message contains "Invalid buffer"
// → Verify: No buffers were cleared (check all buffers still have data)

configure({action: "clear", buffer: ""})
// → Verify: Defaults to "logs" (backward compatible)

configure({action: "clear", buffer: null})
// → Verify: Handles gracefully (defaults to logs or returns error)
```

**Case sensitivity:**
```javascript
configure({action: "clear", buffer: "NETWORK"})
// → Verify: Error (case-sensitive, must be lowercase "network")

configure({action: "clear", buffer: "Network"})
// → Verify: Error
```

**Clearing buffers with active tracking:**
```javascript
// Enable tracking (tracked_tab_id should be set)
observe({what: "logs"})
// → Verify: tracked_tab_id present

configure({action: "clear", buffer: "logs"})
// → Verify: Logs cleared successfully

observe({what: "logs"})
// → Verify: tracked_tab_id still present (tracking state not lost)
```

**Total count accuracy:**
```javascript
// Add exact counts to each buffer type
// → network_waterfall: 3 entries
// → network_bodies: 2 entries
// → websocket_events: 4 entries
// → websocket_status: 1 connection
// → actions: 5 entries
// → logs: 10 entries
// → extension_logs: 2 entries

configure({action: "clear", buffer: "all"})
// → Verify: counts.network_waterfall = 3
// → Verify: counts.network_bodies = 2
// → Verify: counts.websocket_events = 4
// → Verify: counts.websocket_status = 1
// → Verify: counts.actions = 5
// → Verify: counts.logs = 10
// → Verify: counts.extension_logs = 2
// → Verify: total_cleared = 27 (sum is correct)
```

---

### tracked_tab_id Edge Cases

**No tracking active:**
```javascript
// Disable tracking in extension or have no active tab
observe({what: "logs"})
// → Verify: tracked_tab_id field NOT present (or = 0)
// → Verify: No error, normal response

observe({what: "errors"})
observe({what: "actions"})
observe({what: "websocket_events"})
// → Verify: All omit tracked_tab_id when not tracking
```

**Tab switching:**
```javascript
// Switch tracked tab from tab A (id=10) to tab B (id=20)
observe({what: "logs"})
// → Verify: tracked_tab_id updates to new tab ID

// Switch back to tab A
observe({what: "logs"})
// → Verify: tracked_tab_id reverts to original tab ID
```

**tracked_tab_id consistency across handlers:**
```javascript
// With tracking active on tab_id=42
observe({what: "logs"})
// → Save tracked_tab_id value

observe({what: "errors"})
observe({what: "actions"})
observe({what: "websocket_events"})
observe({what: "websocket_status"})
observe({what: "network_waterfall"})

// → Verify: ALL handlers return same tracked_tab_id value
// → Verify: All return 42 (consistent tracking state)
```

**Type verification:**
```javascript
observe({what: "logs"})
// → Verify: tracked_tab_id is an integer, not a string
// → Verify: tracked_tab_id > 0 when tracking is active
```

---

### Version Checking Edge Cases

**Server startup version check:**
```bash
# Start server with network access
./dist/gasoline --port 7890

# Check startup logs
# → Verify: Version check happens on startup (or shortly after)
# → Verify: Log message indicates check occurred
# → Verify: No error if GitHub API is reachable
```

**Network failure handling:**
```bash
# Start server with network disconnected or GitHub blocked
# → Verify: Server still starts successfully
# → Verify: No crash or hang from version check failure
# → Verify: Error logged but not fatal
```

**Health endpoint version fields:**
```bash
curl http://localhost:7890/health | jq .

# → Verify: "version" field contains current version (e.g., "5.3.0")
# → Verify: "available_version" field exists (may be null or version string)
# → Verify: If newer version exists, available_version != null
# → Verify: If current is latest, available_version = null or same as version
```

**Cache behavior verification:**
```bash
# Check version at T=0
curl http://localhost:7890/health | jq .available_version

# Wait 10 seconds
# Check version at T=10s
curl http://localhost:7890/health | jq .available_version
# → Verify: Returns cached result (no GitHub API call)

# Wait 6+ hours (or restart server to clear cache)
# Check version again
# → Verify: New GitHub API call occurs (cache expired)
```

**Rate limiting protection:**
```bash
# Make 10 rapid health checks
for i in {1..10}; do curl -s http://localhost:7890/health | jq .available_version; done

# → Verify: No GitHub rate limit errors
# → Verify: All return cached result (not 10 API calls)
```

**Extension badge behavior:**
```
# With newer version available:
# → Verify: Extension icon shows badge (e.g., "⬆" or dot)
# → Verify: Hover text indicates new version available
# → Verify: Badge persists across page reloads

# With current version (no update available):
# → Verify: Extension icon has no badge
# → Verify: Clean icon appearance
```

---

## Performance & Scalability Tests ⭐⭐⭐

**Large data pagination performance:**
```javascript
// Fill buffer to maximum capacity (1000+ entries)
observe({what: "logs"})
// → Verify: Response time < 200ms
// → Verify: Memory usage reasonable (check /health)

observe({what: "logs", limit: 100})
// → Verify: Response time < 100ms (limit should be fast)

observe({what: "logs", limit: 1})
// → Verify: Response time < 50ms (minimal data)

// Pagination through large dataset
observe({what: "logs", limit: 50})
// → Save cursor, measure time T1

observe({what: "logs", after_cursor: "<cursor>", limit: 50})
// → Measure time T2
// → Verify: T2 ≈ T1 (consistent pagination performance)
```

**Buffer clearing performance:**
```javascript
// Fill all buffers to capacity
// → Network: 1000+ entries
// → WebSocket: 1000+ events
// → Actions: 1000+ actions
// → Logs: 1000+ entries

configure({action: "clear", buffer: "all"})
// → Verify: Response time < 200ms
// → Verify: All buffers actually cleared (check with observe)

// Individual buffer clears
configure({action: "clear", buffer: "network"})
// → Verify: < 100ms

configure({action: "clear", buffer: "websocket"})
// → Verify: < 100ms
```

**Memory leak detection:**
```javascript
// Baseline memory
configure({action: "health"})
// → Save memory.alloc_mb value

// Fill and clear buffers 10 times
for (let i = 0; i < 10; i++) {
  // Fill buffers with data
  // → Generate 500 logs, 500 network requests, etc.

  configure({action: "clear", buffer: "all"})
}

// Check memory after 10 cycles
configure({action: "health"})
// → Verify: memory.alloc_mb is close to baseline (no significant growth)
// → Verify: Not more than 2x baseline memory (acceptable GC overhead)
```

**Concurrent access stress test:**
```javascript
// Simulate multiple MCP clients accessing simultaneously
// → Client 1: Paginating logs
// → Client 2: Clearing buffers
// → Client 3: Adding new data

// → Verify: No race conditions
// → Verify: No data corruption
// → Verify: No crashes or deadlocks
// → Verify: Each client gets consistent responses
```

**Cursor stability under concurrent writes:**
```javascript
// Get initial cursor
observe({what: "logs", limit: 10})
// → Save cursor C1

// Add 100 new log entries (concurrent writes)

// Use old cursor
observe({what: "logs", after_cursor: "<C1>", limit: 10})
// → Verify: Returns correct older entries
// → Verify: New entries don't affect cursor validity (sequence-based)
```

**Network bodies memory pressure:**
```javascript
// Add 100 large network bodies (1MB each)
// → Verify: Memory tracking in /health shows growth
// → Verify: nbMemoryTotal increases appropriately

configure({action: "clear", buffer: "network"})

configure({action: "health"})
// → Verify: nbMemoryTotal back to 0
// → Verify: Memory freed (not just marked for GC)
```

**WebSocket event buffer pressure:**
```javascript
// Generate 5000+ WebSocket events rapidly
// → Verify: Buffer evicts oldest when capacity reached
// → Verify: No memory leak
// → Verify: wsMemoryTotal tracked correctly

configure({action: "clear", buffer: "websocket"})
// → Verify: wsMemoryTotal resets to 0
```

---

## Regression Tests - Previous Issues ⭐⭐⭐

**Log message/source overwrite bug (v5.2.5 fix):**
```javascript
// Add log with payload containing message and source fields
// → Payload: {message: "wrong", source: "bad.js", level: "info"}
// → Enriched: {message: "correct", source: "correct.js"}

observe({what: "logs"})
// → Verify: message field shows enriched value "correct" (NOT "wrong")
// → Verify: source field shows enriched value "correct.js" (NOT "bad.js")
// → Verify: Payload spread does NOT overwrite enriched fields
```

**Errors handler JSON migration (v5.3):**
```javascript
observe({what: "errors"})
// → Verify: Returns JSON format (NOT markdown table)
// → Verify: Has "errors" array field
// → Verify: Has cursor, count, total metadata
// → Verify: Each error has level, message, source, timestamp, sequence
```

**Backward compatibility - old clients:**
```javascript
// Old client behavior (no new v5.3 parameters)
observe({what: "logs"})
// → Verify: Works without cursor parameters (default last N entries)
// → Verify: Returns cursor metadata for future use

observe({what: "logs", limit: 50})
// → Verify: Works (limit-only, no cursor)

configure({action: "clear"})
// → Verify: Works (defaults to buffer: "logs")
// → Verify: Only logs cleared (backward compatible default)
```

**Extension content/inject script bundling (v5.2.0):**
```
# → Verify: Extension loads successfully in Chrome
# → Verify: No console errors about missing scripts
# → Verify: content.js and inject.js are bundled (not remote loaded)
# → Verify: Chrome Web Store policy compliance (no remote code)
```

**Accessibility audit failures (v5.2.0 fix):**
```javascript
observe({what: "accessibility"})
// → Verify: Works without errors
// → Verify: Does not crash on missing DOM elements
// → Verify: Parameter validation works correctly
```

---

## Multi-Client Isolation Tests ⭐⭐

**Concurrent clients don't interfere:**
```javascript
// Client A: Clear logs
configure({action: "clear", buffer: "logs"})

// Client B: Observe logs (simultaneously)
observe({what: "logs"})
// → Verify: Client B sees consistent state
// → Verify: No partial/corrupted data
```

**Session-specific data isolation:**
```javascript
// Client A: Store session data
configure({action: "store", store_action: "save", key: "test", data: {value: "A"}})

// Client B: Store session data with same key
configure({action: "store", store_action: "save", key: "test", data: {value: "B"}})

// Client A: Load data
configure({action: "store", store_action: "load", key: "test"})
// → Verify: Gets {value: "A"} (not B's data)

// Client B: Load data
configure({action: "store", store_action: "load", key: "test"})
// → Verify: Gets {value: "B"} (not A's data)
```

**Buffer clearing affects all clients:**
```javascript
// Client A: Observe logs
observe({what: "logs"})
// → Note: N entries

// Client B: Clear logs
configure({action: "clear", buffer: "logs"})

// Client A: Observe logs again
observe({what: "logs"})
// → Verify: Empty (buffer shared across clients)
```

---

## Cross-Browser Testing (Extension)

**Chrome:**
```
- [ ] Extension loads without errors
- [ ] tracked_tab_id updates correctly
- [ ] Version badge appears when update available
- [ ] All observe handlers return tracked_tab_id
- [ ] Buffer clearing works from extension UI
```

**Edge (Chromium):**
```
- [ ] Extension compatible (Chromium-based)
- [ ] Same behavior as Chrome
- [ ] No browser-specific issues
```

**Firefox (if supported):**
```
- [ ] Extension loads (or fails gracefully if MV3 not supported)
```

---

## Smoke Tests - Quick Sanity Checks

**5-Minute Smoke Test (run before every release):**
```bash
# 1. Start server
./dist/gasoline --port 7890

# 2. Check health
curl http://localhost:7890/health | jq .
# → Verify: Returns valid JSON with version, uptime, buffers

# 3. Basic pagination
# (via MCP client)
observe({what: "logs"})
# → Verify: Returns data with cursor metadata

# 4. Basic clearing
configure({action: "clear", buffer: "all"})
# → Verify: Returns cleared counts

# 5. Verify buffers empty
observe({what: "logs"})
observe({what: "network_waterfall"})
observe({what: "actions"})
# → Verify: All return empty arrays

# 6. Extension check
# → Open browser, verify extension icon shows no errors
# → Generate some logs, verify they appear in MCP
```

---

## Critical Path Test - End-to-End

**Full workflow test:**
```javascript
// 1. Start fresh
configure({action: "clear", buffer: "all"})

// 2. Generate test data
// → Visit a website with the extension
// → Generate console logs, network requests, user actions

// 3. Verify data capture
observe({what: "logs"})
// → Verify: Has entries, tracked_tab_id present

observe({what: "network_waterfall"})
// → Verify: Has entries, tracked_tab_id present

observe({what: "actions"})
// → Verify: Has entries, tracked_tab_id present

// 4. Test pagination
observe({what: "logs", limit: 5})
// → Save cursor

observe({what: "logs", after_cursor: "<cursor>", limit: 5})
// → Verify: Gets older entries

// 5. Test selective clearing
configure({action: "clear", buffer: "logs"})

observe({what: "logs"})
// → Verify: Empty

observe({what: "network_waterfall"})
// → Verify: Still has data (not cleared)

// 6. Test full clearing
configure({action: "clear", buffer: "all"})

// Verify all empty
observe({what: "logs"})
observe({what: "network_waterfall"})
observe({what: "actions"})
// → All empty

// 7. Verify health after operations
configure({action: "health"})
// → Verify: No memory leaks, reasonable resource usage
```

---

## Sign-Off

After completing all tests above:

### Critical Features (MUST PASS)
- [ ] Cursor-based pagination works for logs, errors, actions, websocket_events
- [ ] Pagination handles edge cases (empty buffers, invalid cursors, buffer overflow)
- [ ] Cursor expiration with restart_on_eviction works correctly
- [ ] Buffer-specific clearing works for network, websocket, actions, logs, all
- [ ] Buffer clearing actually frees memory (verified via /health)
- [ ] tracked_tab_id appears in all JSON observe handlers when tracking active
- [ ] Version checking works (startup check, /health endpoint, caching)
- [ ] Extension badge shows when update available

### Regression Prevention (MUST PASS)
- [ ] No log message/source overwrite bug (v5.2.5 regression)
- [ ] Errors handler returns JSON format (not markdown)
- [ ] Backward compatibility maintained (old clients still work)
- [ ] Extension loads without remote code violations
- [ ] No accessibility audit crashes

### Performance (MUST PASS)
- [ ] Pagination with 1000+ entries responds in < 200ms
- [ ] Buffer clearing responds in < 200ms
- [ ] No memory leaks after 10 fill/clear cycles
- [ ] Concurrent access does not cause race conditions or crashes

### Edge Cases (MUST PASS)
- [ ] Empty buffer operations work without errors
- [ ] Invalid cursors return proper errors (not crashes)
- [ ] Large limits do not cause memory allocation issues
- [ ] Cursor expiration handled gracefully
- [ ] Malformed requests return errors (not crashes)

### Security & Privacy (MUST PASS)
- [ ] tracked_tab_id does not leak sensitive data
- [ ] Version checking only contacts GitHub (no telemetry)
- [ ] Session data properly isolated between clients

---

**Tester:** _______________
**Date:** _______________
**Outcome:** PASS / FAIL

**Test Coverage:**
- Total tests run: _______
- Tests passed: _______
- Tests failed: _______
- Tests skipped: _______

**Blocking Issues Found:**
```
[List any critical issues that prevent release]
```

**Non-Blocking Issues:**
```
[List minor issues to address in future releases]
```

**Performance Observations:**
```
[Note any performance characteristics, memory usage, response times]
```

**Additional Notes:**
```
[Any other observations, concerns, or recommendations]
```

---

## Quick Reference - Test Prioritization

**If time is limited, run tests in this priority order:**

### Priority 1 - MUST TEST (30 min)
1. Smoke test (5 min)
2. Critical path end-to-end (10 min)
3. All critical feature tests from sections 1-4 (15 min)

### Priority 2 - SHOULD TEST (60 min)
4. All edge cases from pagination section
5. All edge cases from buffer clearing section
6. Performance tests (memory leaks, large data)
7. Regression tests (previous bug fixes)

### Priority 3 - NICE TO TEST (60 min)
8. tracked_tab_id edge cases
9. Version checking edge cases
10. Error handling & validation
11. Security & privacy tests
12. Multi-client isolation

---

## Known Limitations & Acceptable Behavior

**Expected/Acceptable:**
- Version checking requires network access (fails gracefully if offline)
- Buffer clearing is destructive (no undo)
- Cursor expiration requires restart_on_eviction flag or returns error
- tracked_tab_id only present when extension tracking is active
- Buffers are process-global (all MCP sessions share data)

**Not Tested (Out of Scope for v5.3):**
- Extremely large payloads (>100MB per entry)
- Network bodies > 10MB per request
- Browsers other than Chrome/Edge
- Mobile browser support
- WebSocket binary data > 10MB

---

## Troubleshooting Common Test Failures

**"Cursor expired" errors:**
- Expected when buffer overflows and old entries are evicted
- Use `restart_on_eviction: true` to auto-restart from oldest available

**tracked_tab_id missing:**
- Check that extension tracking is enabled
- Verify a tab is actively being tracked
- Check extension popup shows tracking status

**Buffer clearing returns 0 counts:**
- Expected if buffer is already empty
- Not an error - verify with observe() call

**Version checking shows no updates:**
- Expected if current version is latest
- Check GitHub releases manually to verify
- Cache may be stale (wait 6 hours or restart server)

**Performance tests fail:**
- Check system resources (CPU, memory)
- Verify no other processes interfering
- May need to adjust thresholds for slower systems

---

## Next Steps After UAT

If all tests PASS:
1. Update version to v5.3.0 in `cmd/dev-console/main.go` and `extension/manifest.json`
2. Update CHANGELOG.md with v5.3.0 release notes
3. Run final smoke test on clean build
4. Merge `next` → `main`
5. Tag v5.3.0
6. Create GitHub release with release notes
7. Publish to NPM, PyPI
8. Update documentation site
9. Announce release

If any tests FAIL:
1. Document failing test(s) in issue tracker with full details
2. Mark as blocker for release
3. Fix issue(s) with proper tests
4. Re-run full UAT from scratch (not just failed tests)
5. Do NOT release until 100% of critical tests pass
6. Consider if failures indicate need for additional test coverage

---

## Post-Release Verification

After release, verify in production:

- [ ] NPM package downloads successfully
- [ ] PyPI package installs without errors
- [ ] Extension updates in Chrome Web Store (if applicable)
- [ ] Version checking detects new release correctly
- [ ] Documentation site updated
- [ ] GitHub release notes accurate
- [ ] No immediate bug reports from early adopters (monitor for 24 hours)

---

## Appendix: Test Data Generation

**Generating large datasets for testing:**

```javascript
// Generate 1000 logs rapidly
for (let i = 0; i < 1000; i++) {
  console.log(`Test log ${i}: ${Date.now()}`);
}

// Generate network traffic
for (let i = 0; i < 100; i++) {
  fetch(`https://jsonplaceholder.typicode.com/posts/${i}`);
}

// Generate user actions
// → Click around the page, type in inputs, scroll, navigate

// Simulate buffer overflow
// → Generate 2000+ entries to force eviction in 1000-capacity buffer
```

**Checking memory usage:**
```bash
# Via health endpoint
curl http://localhost:7890/health | jq '.memory'

# Expected fields:
# - alloc_mb: Current memory allocation
# - sys_mb: System memory
# - num_gc: Garbage collection cycles
# - nbMemoryTotal: Network bodies memory
# - wsMemoryTotal: WebSocket memory
```

---

## Document Revision History

- **2026-01-30 (v1.0)** - Initial comprehensive UAT checklist for v5.3.0
  - Added 180+ test cases covering all features and edge cases
  - Emphasis on regression prevention and performance testing
  - Structured for time-boxed testing with priority levels
