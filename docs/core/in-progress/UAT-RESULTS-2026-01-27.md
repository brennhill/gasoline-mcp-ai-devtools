# UAT Results - Schema Improvements
**Date:** 2026-01-27
**Tester:** Claude Sonnet 4.5
**Build:** f1b1f4f (feat: implement network schema improvements for optimal LLM consumption)

## Executive Summary

Conducted full UAT testing of network schema improvements and related features. Found **4 critical issues** that block functionality:

- ✅ **PASS:** network_waterfall schema - All improvements working correctly
- ⚠️ **PARTIAL:** network_bodies schema - Schema correct, but no data captured
- ❌ **FAIL:** query_dom - Returns 0 results for all selectors including "*"
- ❌ **FAIL:** validate_api - Parameter name conflict prevents any usage
- ❌ **FAIL:** accessibility - Function not defined error

---

## Test 1: network_waterfall Schema ✅ PASS

**Status:** All schema improvements verified and working correctly.

### What Was Tested
- Navigated to example.com to generate network traffic
- Called `observe({what:"network_waterfall", limit:3})`
- Verified all new fields and metadata

### Results
✅ **All improvements present and correct:**

```json
{
  "count": 3,
  "entries": [
    {
      "url": "https://gemini.google.com/_/BardChatUi/data/...",
      "initiatorType": "xmlhttprequest",
      "durationMs": 249.69999992847443,        // ✅ Unit suffix added
      "startTimeMs": 1835184.5,                 // ✅ Unit suffix added
      "fetchStartMs": 0,                        // ✅ Unit suffix added
      "responseEndMs": 0,                       // ✅ Unit suffix added
      "transferSizeBytes": 454,                 // ✅ Unit suffix added
      "decodedBodySizeBytes": 167,              // ✅ Unit suffix added
      "encodedBodySizeBytes": 154,              // ✅ Unit suffix added
      "compressionRatio": 0.9221556886227545,   // ✅ NEW computed field
      "cached": false,
      "pageURL": "https://gemini.google.com/app/...",
      "capturedAt": "2026-01-27T18:51:18+01:00" // ✅ Renamed from timestamp
    }
  ],
  "timespan": "0.0s",
  "oldestTimestamp": "2026-01-27T18:51:18+01:00", // ✅ NEW metadata
  "newestTimestamp": "2026-01-27T18:51:18+01:00", // ✅ NEW metadata
  "limitations": [
    "No HTTP status codes (use network_bodies for 404s/500s/401s)",
    "No request methods (GET/POST/etc.)",
    "No request/response headers or bodies"
  ]
}
```

### Verification
- ✅ All field names have unit suffixes (Ms, Bytes)
- ✅ compressionRatio computed correctly (0.922 = 154/167)
- ✅ timestamp renamed to capturedAt
- ✅ Metadata fields present (oldestTimestamp, newestTimestamp)
- ✅ limitations array provides helpful context

---

## Test 2: network_bodies Schema ⚠️ PARTIAL

**Status:** Schema improvements implemented correctly, but no data captured during testing.

### What Was Tested
- Navigated to multiple URLs: example.com, jsonplaceholder.typicode.com, httpbin.org
- Called `observe({what:"network_bodies", limit:5})`
- Waited for requests to complete

### Results

**Schema is correct when empty:**
```json
{
  "count": 0,
  "networkRequestResponsePairs": [],          // ✅ Renamed from "requests"
  "maxRequestBodyBytes": 8192,                // ✅ NEW metadata
  "maxResponseBodyBytes": 16384               // ✅ NEW metadata
}
```

### Issues Found

**ISSUE #1: No network bodies captured**
- **Severity:** High
- **Description:** Multiple page loads and API calls generated no network_bodies data
- **Expected:** Should capture request/response bodies for JSON endpoints
- **Actual:** Empty array in all cases
- **Root Cause:** Unknown - possibly body capture is disabled by default or requires specific configuration
- **Impact:** Cannot verify the full schema improvements (durationMs, capturedAt, size fields, binaryFormatInterpretation)

### What Could Not Be Verified
Due to no data being captured, the following improvements could not be tested:
- ❓ durationMs field (renamed from duration)
- ❓ capturedAt field (renamed from timestamp)
- ❓ requestBodySizeBytes, responseBodySizeBytes
- ❓ requestBodyTruncated, responseBodyTruncated flags
- ❓ binaryFormatInterpretation computed field
- ❓ Summary text change ("network request-response pair(s)")

### Recommendation
- Investigate why network bodies aren't being captured
- Check if body capture needs to be explicitly enabled
- Verify extension is intercepting fetch/XHR properly

---

## Test 3: query_dom Schema ❌ FAIL

**Status:** Critical failure - returns 0 results for all queries including universal selector.

### What Was Tested
- Navigated to example.com (known working page)
- Tested selector: "h1" (example.com has h1 element)
- Tested selector: "body" (every page has body)
- Tested selector: "*" (universal selector matches everything)

### Results

**All queries return 0 matches:**
```json
{
  "hint": "No elements matched selector \"*\". Verify the selector is correct...",
  "matches": null,
  "maxDepthQueried": 5,           // ✅ Metadata present
  "maxElementsReturned": 50,      // ✅ Metadata present
  "maxTextLength": 500,           // ✅ Metadata present
  "pageTitle": "",                // ❌ EMPTY - should be "Example Domain"
  "returnedMatchCount": 0,        // ❌ Should be > 0
  "selector": "*",
  "totalMatchCount": 0,           // ❌ Should be > 0
  "url": ""                       // ❌ EMPTY - should be "https://example.com/"
}
```

### Issues Found

**ISSUE #2: query_dom NOT IMPLEMENTED despite being in schema**
- **Severity:** Critical - Feature advertised but not implemented
- **Description:** Even universal selector "*" returns 0 matches on known-good page
- **Expected:** Should find at least html, head, body, h1, p, etc.
- **Actual:** Returns 0 matches with empty url and pageTitle fields
- **Root Cause:** FOUND - background.js:2634-2640 has TODO comment and returns "not_implemented"
- **Impact:** Feature is completely non-functional, returns fake empty results instead of error

**Code Evidence:**
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

**ISSUE #3: query_dom returns misleading empty results instead of error**
- **Severity:** High - UX issue
- **Description:** Returns schema-compliant empty response instead of error message
- **Expected:** Should return clear error: "DOM query feature not yet implemented"
- **Actual:** Returns empty matches with hint saying "No elements matched selector"
- **Root Cause:** Server-side toolQueryDOM wraps the "not_implemented" error in valid schema
- **Impact:** Users think their query failed rather than understanding feature isn't ready

### What Could Not Be Verified
Due to no results returned:
- ❓ textTruncated field
- ❓ bboxPixels field (renamed from boundingBox)
- ❓ Match data structure
- ❓ Hint message for empty results (did show but only due to failure)

### Recommendation
- **URGENT:** Either implement query_dom or remove from schema
- If not ready for release, remove from MCP tool enum so it can't be called
- If keeping in schema, add clear "not_implemented" error instead of fake empty results
- Implementation should forward to content script like a11y queries do:
  ```javascript
  if (query.type === 'dom') {
    const result = await chrome.tabs.sendMessage(tabId, {
      type: 'DOM_QUERY',
      params: query.params,
    })
    await postQueryResult(serverUrl, query.id, 'dom', result)
  }
  ```
- Then in content.js/inject.js, call executeDOMQuery which already exists in dom-queries.js

---

## Test 4: validate_api Schema ❌ FAIL

**Status:** Critical failure - parameter name conflict prevents usage.

### What Was Tested
- Attempted to call validate_api with api_action="analyze"
- Checked MCP schema definition vs implementation

### Results

**Error when calling:**
```json
{
  "error": "invalid_param",
  "message": "action parameter must be 'analyze', 'report', or 'clear'",
  "retry": "Use a valid value for 'action'",
  "param": "action",
  "hint": "analyze, report, or clear"
}
```

### Issues Found

**ISSUE #4: Parameter name conflict in validate_api**
- **Severity:** Critical
- **Description:** MCP schema and implementation disagree on parameter name
- **Expected:** Schema says use `api_action` parameter
- **Actual:** Implementation expects `action` parameter, but that conflicts with configure's `action` parameter
- **Root Cause:** Schema/implementation mismatch
  - **Schema (tools.go:1014):** `"api_action": map[string]interface{}`
  - **Implementation (tools.go:1955):** `Action string json:"action"`
- **Impact:** Feature is completely unusable - cannot pass required parameter

### Code Evidence

**MCP Schema definition:**
```go
// cmd/dev-console/tools.go:1014
"api_action": map[string]interface{}{
    "type":        "string",
    "description": "API validation sub-action: analyze, report, clear (applies to validate_api)",
    "enum":        []string{"analyze", "report", "clear"},
},
```

**Implementation:**
```go
// cmd/dev-console/tools.go:1953-1955
func (h *ToolHandler) toolValidateAPI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
    var params struct {
        Action string `json:"action"`  // ❌ Should be `json:"api_action"`
```

### The Problem
When calling configure:
```json
{
  "action": "validate_api",  // Tells configure which handler to call
  "api_action": "analyze"    // What schema says to use
}
```

But toolValidateAPI expects:
```json
{
  "action": "analyze"  // ❌ Conflicts with outer action parameter
}
```

JSON doesn't allow duplicate keys, so there's no way to call this function.

### Fix Required
Change line 1955 in tools.go from:
```go
Action string `json:"action"`
```
to:
```go
Action string `json:"api_action"`
```

### Recommendation
- **URGENT:** Fix parameter name mismatch
- Update struct tag to match schema
- Add integration test to catch schema/implementation mismatches

---

## Test 5: accessibility Feature ❌ FAIL

**Status:** Critical failure - function not defined.

### What Was Tested
- Navigated to example.com
- Called `observe({what:"accessibility"})`

### Results

**Error returned:**
```json
{
  "error": "runAxeAuditWithTimeout is not defined"
}
```

### Issues Found

**ISSUE #5: runAxeAuditWithTimeout function not defined**
- **Severity:** Critical
- **Description:** Accessibility audit fails with "function not defined" error
- **Expected:** Should run axe-core audit and return violations
- **Actual:** Error message indicating missing function
- **Root Cause:** Function runAxeAuditWithTimeout not defined in inject.js or not in scope
- **Impact:** Feature is completely non-functional

### Code Context
Based on previous session summary, parallel agents implemented:
- background.js: Forwards A11Y_QUERY to content script ✅
- content.js: Forwards to inject.js via postMessage ✅
- inject.js: Should call runAxeAuditWithTimeout ❌

**Code Verification:**
- ✅ Function IS defined in extension/lib/dom-queries.js:192
- ✅ Function IS imported in extension/inject.js:113
- ✅ Function IS called correctly in inject.js:927
- ❌ Runtime error says "not defined"

### Root Cause Analysis

The code is correct in the repository. The error "runAxeAuditWithTimeout is not defined" suggests:

1. **Most Likely:** Extension wasn't fully reloaded after code changes
   - Chrome caches service workers and injected scripts aggressively
   - Clicking "reload" in chrome://extensions isn't always sufficient
   - May need to: Remove extension → Close all tabs → Re-add extension

2. **Module Loading Issue:** ES module imports not resolving in page context
   - inject.js uses ES6 imports which need proper MIME types
   - Browser may be blocking module loads due to CORS or CSP

3. **Race Condition:** Query arrives before module finishes loading
   - Unlikely but possible if accessibility query happens very early

### Recommendation
- **Try First:** Hard reload extension (remove + re-add, not just reload button)
- Check browser console for ES module loading errors
- Verify manifest.json serves inject.js with correct type="module"
- Add defensive check: `if (typeof runAxeAuditWithTimeout === 'undefined')`
- Consider pre-loading axe-core on extension startup

---

## Summary of Issues

### Critical Issues (Block Functionality)

| # | Component | Issue | Severity | Status |
|---|-----------|-------|----------|--------|
| 1 | network_bodies | No data captured | High | Open |
| 2 | query_dom | Returns 0 results for all queries | Critical | Open |
| 3 | query_dom | Empty url/pageTitle fields | Critical | Open |
| 4 | validate_api | Parameter name conflict (api_action vs action) | Critical | Open |
| 5 | accessibility | runAxeAuditWithTimeout is not defined | Critical | Open |

### What's Working

✅ **network_waterfall:** All schema improvements working perfectly
- Unit suffixes on all fields
- compressionRatio computed field
- Metadata timestamps
- Helpful limitations array

✅ **MCP Connection:** Server connected and responding
✅ **Extension Connection:** Pilot enabled and polling successfully
✅ **Navigate Actions:** Successfully navigating to pages
✅ **Page Observation:** Getting page info correctly

---

## Recommendations

### Immediate Actions Required

1. **Fix validate_api parameter conflict** (1 line change)
   - Change struct tag from `json:"action"` to `json:"api_action"` in tools.go:1955

2. **Debug query_dom message passing**
   - Add logging to track message flow from server → background → content → inject
   - Verify query type is handled in each layer

3. **Fix accessibility function**
   - Check if runAxeAuditWithTimeout exists in inject.js
   - Ensure axe-core is loaded before calling
   - Add proper error handling

4. **Investigate network_bodies capture**
   - Determine why no bodies are being captured
   - Check if feature needs explicit enablement
   - Verify extension intercepts are working

### Before Next Release

- Add integration tests for schema/implementation matching
- Add end-to-end tests for message passing (query_dom, accessibility)
- Add tests that verify actual data capture (network_bodies)
- Document any required configuration for body capture

---

## Test Environment

- **Server Build:** f1b1f4f
- **Server Status:** Connected, responding normally
- **Extension Status:** Connected, pilot enabled
- **Browser:** Active tab focused
- **Test Duration:** ~15 minutes
- **Tests Executed:** 5/5 feature areas
- **Tests Passed:** 1/5 (network_waterfall only)
- **Tests Failed:** 4/5 (network_bodies partial, others critical failures)

---

## Next Steps

1. Fix the 5 identified issues
2. Re-run UAT with fixes applied
3. Add automated tests to prevent regression
4. Document any configuration requirements
5. Update UAT checklist based on findings
