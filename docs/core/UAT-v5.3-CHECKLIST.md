# v5.3 UAT Checklist

**Release:** v5.3.0
**Focus:** Pagination + Buffer Clearing + Version Checking
**Date:** 2026-01-30

---

## Pre-Deployment Checklist

- [ ] All Pre-UAT Quality Gates passed (see main UAT plan)
- [ ] `git status` shows clean working directory
- [ ] Current branch: `next`
- [ ] All commits pushed to remote

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
// → Should work (returns last N entries with cursor metadata)

observe({what: "logs", limit: 100})
// → Should work (returns last 100 entries)
```

**Old clear syntax:**
```javascript
configure({action: "clear"})
// → Should work (clears logs by default)
```

**Old error handler clients:**
```javascript
observe({what: "errors"})
// → Should return JSON format (breaking change from markdown, but acceptable)
```

---

## Edge Cases

**Empty buffers:**
```javascript
configure({action: "clear", buffer: "all"})
configure({action: "clear", buffer: "network"})
// → Should return counts: 0, not error
```

**Pagination with no data:**
```javascript
configure({action: "clear", buffer: "logs"})
observe({what: "logs"})
// → Should return empty array with count: 0, total: 0
```

**Large limits:**
```javascript
observe({what: "logs", limit: 999999})
// → Should cap at buffer capacity (not crash)
```

**Invalid cursor format:**
```javascript
observe({what: "logs", after_cursor: "invalid"})
// → Should return error
```

---

## Performance Checks

**Pagination performance:**
```javascript
// Generate 1000+ log entries
observe({what: "logs", limit: 100})
// → Should respond in < 100ms
```

**Buffer clearing performance:**
```javascript
// Fill buffers with 1000+ entries
configure({action: "clear", buffer: "all"})
// → Should respond in < 100ms
```

---

## Sign-Off

After completing all tests above:

- [ ] All critical v5.3 features work as expected
- [ ] No regressions in existing functionality
- [ ] Backward compatibility maintained
- [ ] Performance acceptable
- [ ] Ready for v5.3 release

**Tester:** _______________
**Date:** _______________
**Outcome:** PASS / FAIL (circle one)

**Notes:**
```
[Space for observations, issues, or concerns]
```

---

## Next Steps After UAT

If all tests PASS:
1. Update version to v5.3.0 in `cmd/dev-console/main.go` and `extension/manifest.json`
2. Update CHANGELOG.md with v5.3.0 release notes
3. Merge `next` → `main`
4. Tag v5.3.0
5. Create GitHub release
6. Announce release

If any tests FAIL:
1. Document failing test(s) in issue tracker
2. Fix issue(s)
3. Re-run UAT from scratch
4. Do NOT release until all tests pass
