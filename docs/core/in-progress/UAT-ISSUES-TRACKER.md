# UAT Issues Tracker

**Last Updated:** 2026-01-27
**UAT Session:** Schema Improvements (f1b1f4f)

## Quick Status

| Status | Count |
|--------|-------|
| 🔴 Critical | 3 |
| 🟡 High | 1 |
| ✅ Fixed | 1 |
| ✅ Pass | 1 |

**Latest Fix:** Issue #1 (validate_api parameter) - Commit 4868b98

---

## ✅ ISSUE #1: validate_api parameter name conflict [FIXED]

**Status:** ✅ RESOLVED (Commit 4868b98)
**Component:** cmd/dev-console/tools.go
**Found:** 2026-01-27
**Fixed:** 2026-01-27
**Fix Complexity:** ⭐ Easy (5 lines, 7 minutes)

### Problem
Schema says use `api_action` but implementation expects `action`, creating impossible parameter conflict.

### Fix
```go
// Line 1955 - Change from:
Action string `json:"action"`

// To:
Action string `json:"api_action"`
```

### Files to Change
- `cmd/dev-console/tools.go:1955`

### Test
```bash
go test ./cmd/dev-console/... -run TestValidateAPI
```

---

## 🔴 ISSUE #2: query_dom not implemented

**Status:** Critical - Feature returns fake results
**Component:** extension/background.js
**Found:** 2026-01-27
**Fix Complexity:** ⭐⭐⭐ Medium (needs full implementation)

### Problem
Feature in MCP schema but background.js:2634 returns `not_implemented` error.
Returns misleading empty results instead of clear error.

### Current Code
```javascript
// extension/background.js:2634-2640
// TODO: dom queries need proper implementation
if (query.type === 'dom') {
  await postQueryResult(serverUrl, query.id, 'dom', {
    success: false,
    error: 'not_implemented',
  })
  return
}
```

### Fix Option A: Implement (Recommended)
```javascript
if (query.type === 'dom') {
  try {
    const result = await chrome.tabs.sendMessage(tabId, {
      type: 'DOM_QUERY',
      params: query.params,
    })
    await postQueryResult(serverUrl, query.id, 'dom', result)
  } catch (err) {
    await postQueryResult(serverUrl, query.id, 'dom', {
      error: 'dom_query_failed',
      message: err.message || 'Failed to execute DOM query',
    })
  }
  return
}
```

Then add handler in content.js to forward to inject.js (similar to A11Y_QUERY).

### Fix Option B: Remove from schema until ready
Remove `query_dom` from MCP tool enum in tools.go:920.

### Files to Change
- `extension/background.js:2634-2640`
- `extension/content.js` (add DOM_QUERY handler)
- `extension/inject.js` (already has executeDOMQuery function)

---

## 🔴 ISSUE #3: accessibility runAxeAuditWithTimeout not defined

**Status:** Critical - Feature fails at runtime
**Component:** extension/inject.js
**Found:** 2026-01-27
**Fix Complexity:** ⭐ Easy (likely deployment issue)

### Problem
Runtime error "runAxeAuditWithTimeout is not defined" but function exists and is imported.

### Code Status
- ✅ Function defined: extension/lib/dom-queries.js:192
- ✅ Function imported: extension/inject.js:113
- ✅ Function called: extension/inject.js:927
- ❌ Runtime: "not defined"

### Most Likely Cause
Extension not fully reloaded after code changes. Chrome caches aggressively.

### Fix Steps
1. In Chrome, go to chrome://extensions
2. Find Gasoline extension
3. Click "Remove" (not "Reload")
4. Close ALL browser tabs
5. Re-add extension from dist/
6. Test again

### Alternative: Add Defensive Check
```javascript
if (typeof runAxeAuditWithTimeout === 'undefined') {
  window.postMessage({
    type: 'GASOLINE_A11Y_QUERY_RESPONSE',
    requestId,
    result: { error: 'Accessibility audit not available - please reload extension' },
  }, window.location.origin)
  return
}
```

---

## 🟡 ISSUE #4: network_bodies no data captured

**Status:** High - Cannot verify schema improvements
**Component:** extension/lib/network.js or capture configuration
**Found:** 2026-01-27
**Fix Complexity:** ⭐⭐⭐⭐ Hard (investigation needed)

### Problem
Multiple page loads generated no network_bodies data. Cannot verify schema improvements work.

### Tested
- Navigated to: example.com, jsonplaceholder.typicode.com, httpbin.org/get
- All returned empty array
- Schema metadata present and correct (maxRequestBodyBytes, maxResponseBodyBytes)

### Possible Causes
1. Body capture disabled by default
2. URL filtering too aggressive
3. Content-Type filtering excluding all responses
4. Extension not intercepting fetch/XHR
5. CORS/CSP blocking body access

### Investigation Steps
```javascript
// Check if body capture is enabled
observe({what: "extension_logs"}) // Look for network intercept logs

// Try enabling explicitly (if there's a config option)
configure({action: "...", network_bodies: true})

// Check what's in waterfall vs bodies
observe({what: "network_waterfall", limit: 10})  // Should have data
observe({what: "network_bodies", limit: 10})     // Empty
```

### Files to Investigate
- `extension/lib/network.js` (wrapFetchWithBodies, shouldCaptureUrl)
- `extension/background.js` (network body posting)
- `cmd/dev-console/network.go` (body filtering)

---

## 🔴 ISSUE #5: Extension timeouts after several operations

**Status:** Critical - Blocks continued testing
**Component:** extension/background.js or browser state
**Found:** 2026-01-27
**Fix Complexity:** ⭐⭐ Medium (resource leak?)

### Problem
After running several navigate commands, extension starts timing out:
```
Error: extension_timeout — Browser extension didn't respond
```

### Observed Pattern
1. First 5-6 navigations work fine
2. Then timeouts start occurring
3. Pilot still shows connected
4. Page info still works
5. But interact commands fail

### Possible Causes
1. Message queue backup
2. Memory leak in extension
3. Too many pending promises
4. Chrome tab/context issues

### Investigation
- Check Chrome task manager for extension memory
- Look for unclosed WebSocket connections
- Check for unresolved promises in background.js
- Review async command handling

---

## ✅ PASS: network_waterfall schema

**Status:** All improvements working
**Verified:** 2026-01-27

All schema improvements present and correct:
- ✅ Unit suffixes (durationMs, transferSizeBytes, etc.)
- ✅ compressionRatio computed field
- ✅ capturedAt renamed from timestamp
- ✅ Metadata timestamps (oldestTimestamp, newestTimestamp)
- ✅ Helpful limitations array

---

## 🟡 ISSUE #6: observe tool should include tabId in responses

**Status:** Open - Feature gap
**Component:** cmd/dev-console/tools.go, extension/background.js
**Found:** 2026-01-28 (Track This Tab UAT)
**Fix Complexity:** ⭐⭐ Medium

### Problem
The `observe()` tool responses (errors, logs, network_waterfall, etc.) do not include the `tabId` of the data source. Without this, the LLM cannot detect if the user switched tabs mid-session. The content.js layer now attaches `tabId` to all pushed messages, but the server does not surface it in MCP responses.

### Why It Matters
- LLM needs to know which tab data came from
- If user switches tracking to a different tab, LLM should see the change
- Enables LLM to detect stale data from a previous tab

### Fix
1. Server should store `tabId` from pushed messages (logs, network_bodies, etc.)
2. `observe()` responses should include `tracked_tab_id` metadata field
3. Compare with current tracking status to detect tab switches

### Files to Change
- `cmd/dev-console/tools.go` - Add `tracked_tab_id` to observe responses
- `cmd/dev-console/capture.go` or equivalent - Store tabId from batched messages
- Extension already sends tabId (fixed in Track This Tab implementation)

---

## Priority Order for Fixes

1. **🔴 ISSUE #1** (validate_api) - 5 minutes - Just change struct tag
2. **🔴 ISSUE #3** (accessibility) - 10 minutes - Remove/reload extension
3. **🔴 ISSUE #2** (query_dom) - 2 hours - Implement message forwarding
4. **🟡 ISSUE #4** (network_bodies) - 4 hours - Investigation + fix
5. **🔴 ISSUE #5** (extension timeouts) - Unknown - Requires debugging

---

## Next UAT Run Criteria

Before next UAT:
- [ ] Issue #1 fixed and tested
- [ ] Issue #2 either implemented or removed from schema
- [ ] Issue #3 resolved (extension reloaded properly)
- [ ] Issue #4 investigated and root cause identified
- [ ] All fixes have unit tests
- [ ] All fixes documented in commit messages

---

## Reference
- Full UAT Report: [UAT-RESULTS-2026-01-27.md](./UAT-RESULTS-2026-01-27.md)
- Build Tested: f1b1f4f
- Schema Improvements: network_waterfall, network_bodies, query_dom, validate_api, accessibility
