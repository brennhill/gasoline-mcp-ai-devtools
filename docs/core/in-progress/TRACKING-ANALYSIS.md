# Tracking Mechanism Analysis & Proposed Fixes

**Date:** 2026-01-27
**Issue:** "Track This Page" button doesn't control data capture
**Severity:** High - Privacy & Security Issue

---

## Current Behavior (INCORRECT)

### What "Track This Page" Actually Does

When user clicks "Track This Page":
1. Stores `trackedTabId` and `trackedTabUrl` in chrome.storage.local
2. Button changes to "Stop Tracking" (red)
3. Shows URL being "tracked"

**User Expectation:** "Only capture data from this tab"
**Actual Reality:** "Only QUERY this tab, but capture from ALL tabs"

---

## The Problem: Data Captured From ALL Tabs

### Data Flow (Current Implementation)

```
┌─────────────┐
│   Tab 1     │  → inject.js → content.js → background.js → Server
│ (Tracked)   │     ✓ Captures everything
└─────────────┘

┌─────────────┐
│   Tab 2     │  → inject.js → content.js → background.js → Server
│ (Random)    │     ✓ ALSO captures everything (BUG!)
└─────────────┘

┌─────────────┐
│   Tab 3     │  → inject.js → content.js → background.js → Server
│ (Banking?)  │     ✓ ALSO captures everything (SECURITY ISSUE!)
└─────────────┘
```

**All tabs send data regardless of tracking status.**

### Evidence in Code

**1. Network Body Capture (lib/network.js:375-388)**
```javascript
if (win && networkBodyCaptureEnabled) {  // ← Only checks if capture enabled
  win.postMessage({
    type: 'GASOLINE_NETWORK_BODY',
    payload: { url, method, status, requestBody, responseBody, ... }
  }, '*')
}
```
❌ **NO tab filtering** - Captures from every page

**2. Content Script Forwarding (content.js:23-29)**
```javascript
const MESSAGE_MAP = {
  GASOLINE_LOG: 'log',
  GASOLINE_WS: 'ws_event',
  GASOLINE_NETWORK_BODY: 'network_body',  // ← Forwards all messages
  ...
}
```
❌ **NO tab filtering** - Forwards from every tab

**3. Background Script Storage (background.js:1820-1821)**
```javascript
else if (message.type === 'network_body') {
  networkBodyBatcher.add(message.payload)  // ← Stores from every tab
}
```
❌ **NO tab filtering** - Stores data from all tabs

**4. trackedTabId ONLY Used for Queries (background.js:2533-2553)**
```javascript
const storage = await chrome.storage.local.get(['trackedTabId'])
if (storage.trackedTabId) {
  // Use the tracked tab for QUERIES ONLY
  const trackedTab = await chrome.tabs.get(storage.trackedTabId)
  tabs = [trackedTab]
  tabId = storage.trackedTabId
}
```
✅ **Used for queries** - But NOT for filtering capture

---

## Security & Privacy Implications

### What Gets Captured From Untracked Tabs

All of these are captured from EVERY tab:
- ✅ Console logs (including errors with stack traces)
- ✅ Network requests (URLs, methods, status codes)
- ✅ Network bodies (request/response data)
- ✅ WebSocket messages
- ✅ User actions (clicks, inputs, navigation)
- ✅ Performance metrics
- ✅ Errors and exceptions

### Example Attack Scenario

```
User workflow:
1. Opens banking site in Tab 1
2. Opens their app in Tab 2
3. Clicks "Track This Page" in Tab 2
4. Believes only Tab 2 is being tracked

Reality:
- Tab 1 (banking) is STILL sending:
  - Login credentials in network bodies
  - Account numbers in API responses
  - Session tokens in request headers
  - All XHR/fetch calls to bank APIs

All this data goes to:
- Background script buffers
- Server via batch POST
- Stored in Go server memory
- Returned in observe() calls
```

**This is a critical security issue.**

---

## Why UAT Failed to Detect This

During UAT, I:
1. Navigated to example.com → Captured & cleared on next navigation
2. Navigated to jsonplaceholder → Captured & cleared on next navigation
3. Navigated to httpbin.org → Captured data
4. Checked network_bodies → Empty (thought it was a bug)

**What Actually Happened:**
- Each navigation was in the SAME tab
- When I navigated, the page reloaded, clearing inject.js state
- Network requests from OLD page were gone
- NEW page hadn't made any JSON API calls yet
- So buffer was legitimately empty

**I should have tested:**
```javascript
// Proper test:
1. Open Tab 1: example.com (don't track)
2. Open Tab 2: jsonplaceholder.com (don't track)
3. Open Tab 3: httpbin.org (track this one)
4. observe({what: "network_bodies"})
5. Expected: Only httpbin.org data
6. Actual: Data from ALL THREE tabs (if they made requests)
```

---

## Proposed Fix: Multi-Approach Solution

### Option 1: Single Tab Tracking (Recommended)

**Change button to: "Track This Tab" (not "Page")**

Modify capture to ONLY send data from tracked tab:

```javascript
// extension/content.js - Add at top
let isTrackedTab = false

// Check if this tab is the tracked one
chrome.storage.local.get(['trackedTabId'], (result) => {
  chrome.tabs.getCurrent((tab) => {
    isTrackedTab = (tab?.id === result.trackedTabId)
  })
})

// Listen for tracking changes
chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    chrome.tabs.getCurrent((tab) => {
      isTrackedTab = (tab?.id === changes.trackedTabId?.newValue)
    })
  }
})

// Modify message handler
window.addEventListener('message', (event) => {
  // Only forward if this is the tracked tab
  if (!isTrackedTab) return  // ← NEW FILTER

  if (event.source !== window) return
  const msgType = MESSAGE_MAP[event.data?.type]
  if (msgType) {
    safeSendMessage({ type: msgType, payload: event.data.payload })
  }
})
```

**Benefits:**
- Simple to implement (< 20 lines)
- Clear user model: "Track THIS tab only"
- Solves privacy issue completely
- Minimal performance impact

**Drawbacks:**
- Captures from only ONE tab at a time
- User must manually switch tracking if they want different tab

---

### Option 2: Multi-Tab Allowlist

Allow tracking MULTIPLE tabs with explicit opt-in:

```javascript
// Store array of tracked tabs
chrome.storage.local.set({
  trackedTabIds: [tab1, tab2, tab3]
})

// UI shows list of tracked tabs
// "Add This Tab" / "Remove This Tab" buttons
// "Clear All Tracking"
```

**Benefits:**
- More flexible - can track multiple tabs
- Good for comparing behavior across tabs
- User explicitly chooses what to track

**Drawbacks:**
- More complex UI
- Need tab management interface
- More storage/state to manage

---

### Option 3: Global Toggle with Smart Defaults

```javascript
// Three modes:
1. "Off" - No tracking
2. "This Tab Only" - Current tab only (default)
3. "All Tabs" - Everything (expert mode, show warning)
```

**Benefits:**
- Flexible for different use cases
- Clear user choice
- Can accommodate both use cases

**Drawbacks:**
- More complex UI
- Risk of users leaving "All Tabs" on
- Need warning/confirmation for mode 3

---

## Recommended Implementation

### Phase 1: Critical Fix (Option 1)

**Immediate implementation for security:**

1. **Change button text:** "Track This Page" → "Track This Tab"

2. **Add tab ID filtering in content.js:**
   ```javascript
   let isTrackedTab = false

   async function updateTrackingStatus() {
     const storage = await chrome.storage.local.get(['trackedTabId'])
     const currentTab = await chrome.tabs.getCurrent()
     isTrackedTab = (currentTab?.id === storage.trackedTabId)
   }

   // Update on load and storage changes
   updateTrackingStatus()
   chrome.storage.onChanged.addListener(updateTrackingStatus)

   // Filter messages
   window.addEventListener('message', (event) => {
     if (!isTrackedTab && event.data?.type?.startsWith('GASOLINE_')) return
     // ... rest of handler
   })
   ```

3. **Update popup.js:**
   - Change all "Track This Page" text to "Track This Tab"
   - Update tooltip: "Only capture data from this browser tab"

4. **Add warning when untracked:**
   - Show in popup: "⚠️ No tab tracked - data capture disabled"
   - Encourage user to track a tab

5. **Documentation:**
   - Update README with tracking behavior
   - Add privacy section explaining single-tab capture
   - Document in extension description

### Phase 2: Enhanced Features (Optional)

If users need multi-tab tracking:
- Add "Tracked Tabs" list in popup
- "Add/Remove Tab" buttons
- Maximum 5 tracked tabs (memory consideration)
- Show data volume per tab

---

## Testing Plan

### Test 1: Single Tab Isolation
```
1. Open 3 tabs: A (bank), B (app), C (test)
2. Track only tab C
3. Generate activity in all 3 tabs:
   - Tab A: Login to bank site
   - Tab B: Make API calls
   - Tab C: Test requests
4. Check observe({what: "network_bodies"})
5. ✅ Should see ONLY tab C data
6. ❌ Should NOT see tab A or B data
```

### Test 2: Tracking Switch
```
1. Open 2 tabs: A, B
2. Track tab A
3. Generate activity in A
4. observe() → should see A's data
5. Switch tracking to tab B
6. Generate activity in B
7. observe() → should see B's data, NOT A's anymore
```

### Test 3: No Tracking Mode
```
1. Open tab, don't track
2. Generate activity
3. observe() → should be empty or show warning
```

### Test 4: Tracked Tab Closes
```
1. Track tab A
2. Close tab A
3. Try observe()
4. Should fall back gracefully (maybe to active tab, or show error)
```

---

## Files to Modify

1. **extension/content.js** - Add tab filtering (main fix)
2. **extension/popup.js** - Change "Page" to "Tab" everywhere
3. **extension/popup.html** - Update button text
4. **docs/** - Update documentation

**Estimated effort:** 2-3 hours including testing

---

## Breaking Changes

**User Impact:**
- Previously: All tabs captured (security issue)
- Now: Only tracked tab captured (secure, but less data)

**Migration:**
- No migration needed
- Users who relied on multi-tab capture need to switch tracking
- Most users won't notice (they thought it was single-tab anyway)

**Communication:**
- Release notes: "SECURITY: Now only captures from explicitly tracked tab"
- Mark as breaking change in changelog
- Update all documentation

---

## Summary

### Current State
❌ "Track This Page" is misleading
❌ Captures from ALL tabs regardless
❌ Privacy/security issue
❌ User has no control over what's captured

### After Fix (Option 1)
✅ "Track This Tab" is accurate
✅ Captures from ONLY tracked tab
✅ Privacy/security issue resolved
✅ User has explicit control

### Priority
🔴 **CRITICAL** - This is a security/privacy issue that should be fixed before any public release.

---

## Related Issues

- UAT Issue #4: "network_bodies no data captured" - Incorrect diagnosis, need to retest with proper multi-tab methodology
- Need to update UAT documentation with proper testing procedures
- Need to add multi-tab test cases to test suite
