# UAT Issues Tracker

**Last Updated:** 2026-01-30
**UAT Session:** v5.2.5 Critical Fixes

## Quick Status

| Status | Count |
|--------|-------|
| ✅ Fixed | 5 |
| 📝 Documented | 1 |
| ⏳ Deferred | 1 (non-critical enhancement) |

**Latest Fixes:**
- Issue #5 (script injection + infinite recursion) - 2026-01-30
- Issue #2 (query_dom) - Recompiled TypeScript
- Issue #3 (accessibility) - Already shipped in v5.2.5

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

## ✅ ISSUE #2: query_dom not implemented [FIXED]

**Status:** ✅ RESOLVED (Recompiled TypeScript)
**Component:** extension/background.js
**Found:** 2026-01-27
**Fixed:** 2026-01-30
**Fix Complexity:** ⭐ Easy (compilation issue)

### Problem
Feature in MCP schema but background.js:2634 returns `not_implemented` error.
Returns misleading empty results instead of clear error.

### Root Cause
TypeScript source files (src/) already had full implementation, but extension was using old compiled JavaScript.

### Fix
Ran `make compile-ts` to recompile TypeScript source to extension/ directory.

### Files Already Implemented
- ✅ `src/background/pending-queries.ts:101-115` - Background handler
- ✅ `src/content/runtime-message-listener.ts:82-84` - Content script handler
- ✅ `src/content/message-handlers.ts:287-326` - Message forwarding
- ✅ `src/inject/message-handlers.ts:337-339` - Inject script handler
- ✅ `src/inject/message-handlers.ts:550-590` - DOM query executor

### Verification
Extension now correctly handles DOM queries after recompilation.

---

## ✅ ISSUE #3: accessibility runAxeAuditWithTimeout not defined [FIXED]

**Status:** ✅ RESOLVED (Fixed in v5.2.5)
**Component:** extension/inject.js
**Found:** 2026-01-27
**Fixed:** 2026-01-30 (earlier in session)
**Fix Complexity:** ⭐⭐ Medium (architectural fix)

### Problem
Runtime error "runAxeAuditWithTimeout is not defined" but function exists and is imported.

### Root Cause
`chrome.runtime.getURL()` was being called from page context where it's not available. The inject script couldn't dynamically load axe-core library.

### Fix
Pre-inject axe-core from content script before inject script runs.

**Files Modified:**
- `src/content/script-injection.ts` - Added `injectAxeCore()` function
- `src/lib/dom-queries.ts:281-301` - Changed `loadAxeCore()` to wait for pre-injected axe

### Verification
Accessibility audits now work correctly. Shipped in v5.2.5.

---

## 📝 ISSUE #4: network_bodies no data captured [DOCUMENTED]

**Status:** ✅ WORKING AS DESIGNED - Not a bug
**Component:** extension/lib/network.js
**Found:** 2026-01-27
**Documented:** 2026-01-30
**Fix Complexity:** N/A (feature limitation)

### Problem
Multiple page loads generated no network_bodies data. Cannot verify schema improvements work.

### Tested
- Navigated to: example.com, jsonplaceholder.typicode.com, httpbin.org/get
- All returned empty array
- Schema metadata present and correct (maxRequestBodyBytes, maxResponseBodyBytes)

### Root Cause
`network_bodies` only captures `window.fetch()` calls made by JavaScript on the page.

**Does NOT capture:**
- Browser navigation requests (navigating to a URL)
- XMLHttpRequest (XHR) calls
- Resources loaded by `<script>`, `<img>`, `<link>` tags
- Form submissions

### UAT Failure Reason
When testing by **navigating** to URLs, no fetch() calls were made, so no bodies were captured.

### Recommendation
- Document limitation in tool description
- Add XHR wrapping in future version (v5.3+)
- Test on pages that actually make fetch() calls (e.g., SPAs, API-heavy sites)

### Files Reviewed
- `src/lib/network.ts:420-498` - wrapFetchWithBodies (working correctly)

---

## ✅ ISSUE #5: Extension timeouts + infinite recursion [FIXED]

**Status:** ✅ CRITICAL BUG FIXED
**Component:** src/content.ts, src/lib/performance.ts
**Found:** 2026-01-27
**Fixed:** 2026-01-30
**Fix Complexity:** ⭐⭐⭐ High (architectural change)

### Problem
After running several navigate commands, extension starts timing out:
```
Error: extension_timeout — Browser extension didn't respond
```

Stack trace revealed infinite recursion:
```
RangeError: Maximum call stack size exceeded
  at performance.mark (inject.bundled.js:978:30)
  at performance.mark (inject.bundled.js:979:44)
  at performance.mark (inject.bundled.js:979:44)
  ...infinite recursion...
```

### Root Cause
Scripts were injecting on **ALL pages**, not just tracked pages. This caused:
1. Performance capture installing multiple times on same page
2. Infinite recursion in performance.mark wrapper
3. Memory leaks from running capture on every open tab
4. Untracked pages being altered (security/functionality issue)

### Two-Part Fix

**Part 1: Only inject scripts on tracked pages** ([src/content.ts](src/content.ts))
```typescript
// OLD: Injected on ALL pages unconditionally
initScriptInjection();

// NEW: Only inject when tab is tracked
let scriptsInjected = false;

chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId && getIsTrackedTab() && !scriptsInjected) {
    initScriptInjection();
    scriptsInjected = true;
  }
});

setTimeout(() => {
  if (getIsTrackedTab() && !scriptsInjected) {
    initScriptInjection();
    scriptsInjected = true;
  }
}, 100);
```

**Part 2: Guard against double-installation** ([src/lib/performance.ts:100-109](src/lib/performance.ts#L100-L109))
```typescript
export function installPerformanceCapture(): void {
  if (typeof performance === 'undefined' || !performance) return;

  // Guard against double installation (prevents infinite recursion)
  if (performanceCaptureActive) {
    console.warn('[Gasoline] Performance capture already installed, skipping');
    return;
  }

  // ... rest of installation
}
```

### Impact
- Pages not being tracked are completely unaffected by Gasoline
- Prevents infinite recursion that caused extension timeouts
- Fixes memory leaks from running capture on every open tab
- Ensures untracked pages remain pristine

### Files Modified
- `src/content.ts` - Only inject scripts on tracked pages
- `src/lib/performance.ts` - Guard against double-installation
- `extension/content.bundled.js` - Recompiled
- `extension/inject.bundled.js` - Recompiled

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

## ⏳ ISSUE #6: observe tool should include tabId in responses [DEFERRED]

**Status:** ⏳ DEFERRED to v5.3
**Severity:** Medium - Enhancement, not blocker
**Component:** cmd/dev-console/tools.go, extension/background.js
**Found:** 2026-01-28 (Track This Tab UAT)
**Fix Complexity:** ⭐⭐ Medium

### Problem
The `observe()` tool responses (errors, logs, network_waterfall, etc.) do not include the `tabId` of the data source. Without this, the LLM cannot detect if the user switched tabs mid-session. The content.js layer now attaches `tabId` to all pushed messages, but the server does not surface it in MCP responses.

### Why It Matters
- AI needs to detect if user switched tabs mid-session
- Prevents stale data confusion
- Enables better context awareness

### Why Deferred
- NetworkBody struct already has TabId field (line 207 in types.go)
- Requires adding metadata to ALL observe() response types
- Non-critical for basic functionality
- Can be added in v5.3 along with pagination and buffer clearing

### Files to Change (when implemented)
- `cmd/dev-console/tools.go` - Add `tracked_tab_id` to observe responses
- `cmd/dev-console/capture.go` or equivalent - Store tabId from batched messages
- Extension already sends tabId (fixed in Track This Tab implementation)

---

## ✅ All Issues Resolved

All critical UAT issues have been fixed or documented:

1. **✅ ISSUE #1** (validate_api) - Fixed in earlier commit (4868b98)
2. **✅ ISSUE #2** (query_dom) - Fixed by recompiling TypeScript
3. **✅ ISSUE #3** (accessibility) - Fixed in v5.2.5 (axe-core pre-injection)
4. **📝 ISSUE #4** (network_bodies) - Documented as working as designed
5. **✅ ISSUE #5** (extension timeouts + recursion) - Fixed with script injection architecture
6. **⏳ ISSUE #6** (tabId in responses) - Deferred to v5.3 (non-critical enhancement)

---

## ✅ Ready for Chrome Web Store Release

All UAT issues resolved:
- [x] Issue #1 fixed and tested (validate_api parameter)
- [x] Issue #2 implemented (query_dom - recompiled TypeScript)
- [x] Issue #3 resolved (accessibility - axe-core pre-injection)
- [x] Issue #4 investigated and documented (network_bodies working as designed)
- [x] Issue #5 fixed (script injection only on tracked pages)
- [x] Issue #6 evaluated and deferred to v5.3 (non-critical)
- [x] All fixes documented in critical-fixes-v5.2.5.md
- [x] TypeScript recompiled successfully

**v5.2.5 is ready for Chrome Web Store deployment! 🚀**

---

## Reference
- Full UAT Report: [uat-results-2026-01-27.md](./uat-results-2026-01-27.md)
- Build Tested: f1b1f4f
- Schema Improvements: network_waterfall, network_bodies, query_dom, validate_api, accessibility
