# Track This Tab - Edge Case Analysis

**Version:** 1.0
**Date:** 2026-01-27
**Status:** DRAFT - Awaiting Review

---

## Purpose

This document catalogs ALL edge cases for the "Track This Tab" feature, with detailed analysis of:
- What happens
- Expected behavior
- Implementation approach
- Test scenarios
- Risk level

---

## Edge Case Matrix

| # | Case | Frequency | Risk | Status | Priority |
|---|------|-----------|------|--------|----------|
| 1 | Tracked tab closed | High | Medium | Handled | P0 |
| 2 | Tracking switched mid-session | Medium | Low | Handled | P0 |
| 3 | Multiple windows | Low | Low | Handled | P1 |
| 4 | Extension reload | High | Low | Handled | P0 |
| 5 | Storage cleared | Low | Low | Handled | P1 |
| 6 | Tab crashes | Medium | Medium | Need impl | P1 |
| 7 | No tabs open | Rare | Low | Need impl | P2 |
| 8 | Chrome internal pages | Low | Low | Need impl | P1 |
| 9 | Rapid toggle | Low | Low | Need impl | P2 |
| 10 | Tab navigation during LLM op | Medium | Medium | Handled | P1 |
| 11 | Content script not loaded | Medium | High | Need impl | P0 |
| 12 | Network offline | Low | Low | Handled | P2 |
| 13 | Browser restart | High | Low | Handled | P0 |
| 14 | Tab duplicated | Low | Low | Need impl | P2 |
| 15 | Tab moved between windows | Rare | Low | Need impl | P2 |
| 16 | Incognito mode | Low | Medium | Need spec | P2 |
| 17 | Race: tracking + navigation | Medium | Medium | Need impl | P1 |
| 18 | Race: storage change events | Low | Low | Handled | P2 |
| 19 | LLM opens new tab | Medium | Medium | Need spec | P1 |
| 20 | Tab suspended/discarded | Low | Medium | Need impl | P2 |

---

## P0 Edge Cases (Must Handle)

### 1. Tracked Tab Closed

**Scenario:**
```
1. User tracks Tab A (ID: 123)
2. User closes Tab A (e.g., clicks X or Cmd+W)
3. LLM tries: observe({what: "errors"})
```

**Current Behavior (Background.js:2533-2557):**
```javascript
try {
  const trackedTab = await chrome.tabs.get(storage.trackedTabId)
  tabs = [trackedTab]
} catch (err) {
  // Tab doesn't exist - clear tracking and fall back to active tab
  await chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
  tabs = await new Promise((resolve) => {
    chrome.tabs.query({ active: true, currentWindow: true }, resolve)
  })
}
```

**Expected Behavior:**
✅ **Already handled correctly**
- Detect tab no longer exists
- Clear `trackedTabId` from storage
- Fall back to active tab (or return error if no tabs)
- LLM gets data from active tab (may not be what they want)

**Improved Behavior (Future):**
- Return explicit error: "tracked_tab_closed"
- Suggest switching to active tab
- Don't auto-switch (explicit user control)

**Test:**
```javascript
// 1. Track tab A
await trackTab(tabA.id)

// 2. Close tab A
await closeTab(tabA.id)

// 3. Try to observe
const result = await observe({ what: 'errors' })

// Expected: Error or active tab data (with warning)
assert(result.error === 'tracked_tab_closed' || result.url !== tabA.url)
```

**Risk:** Medium - LLM might get data from wrong tab
**Frequency:** High - Users close tabs frequently
**Status:** ✅ Handled (can improve UX)

---

### 2. Tracking Switched Mid-Session

**Scenario:**
```
1. LLM tracking Tab A, gathering network data
2. User opens popup, clicks "Stop Tracking"
3. User navigates to Tab B, clicks "Track This Tab"
4. LLM calls observe({what: "network_bodies"}) (still thinks tracking Tab A)
```

**Expected Behavior:**
- Immediate effect: All operations target Tab B
- Old Tab A data should be cleared or marked as stale
- LLM transparently gets data from new tab

**Implementation:**
- `chrome.storage.onChanged` fires in all content scripts
- Tab A content script sets `isTrackedTab = false` → stops forwarding
- Tab B content script sets `isTrackedTab = true` → starts forwarding
- Background script query logic already uses current `trackedTabId`

**Potential Issue: Buffered Data**
```
Timeline:
T0: Tab A tracked, buffer has 10 network bodies from Tab A
T1: User switches to Tab B
T2: LLM queries network_bodies
T3: Gets mix of Tab A (old) + Tab B (new) data ← PROBLEM
```

**Solution:**
```javascript
// In background.js - clear buffers on tracking change
chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    console.log('[Gasoline] Tracking changed, clearing buffers')

    // Clear all batchers
    networkBodyBatcher.clear()
    logBatcher.clear()
    wsBatcher.clear()
    enhancedActionBatcher.clear()
    performanceSnapshotBatcher.clear()

    // Notify server tracking changed
    sendStatusPing()
  }
})
```

**Test:**
```javascript
// 1. Track tab A
await trackTab(tabA.id)
await generateActivity(tabA.id)

// 2. Verify data from Tab A
let bodies = await observe({ what: 'network_bodies' })
assert(bodies.count > 0)

// 3. Switch to Tab B
await trackTab(tabB.id)

// 4. Verify data cleared (no Tab A data)
bodies = await observe({ what: 'network_bodies' })
assert(bodies.count === 0, 'Buffers should be cleared on tracking switch')

// 5. Generate activity in Tab B
await generateActivity(tabB.id)

// 6. Verify only Tab B data
bodies = await observe({ what: 'network_bodies' })
assert(bodies.networkRequestResponsePairs.every(pair => pair.pageURL.includes(tabB.url)))
```

**Risk:** Low - Clear communication, easy to fix
**Frequency:** Medium - Users may switch tracking occasionally
**Status:** ⚠️ Needs buffer clearing implementation

---

### 4. Extension Reload

**Scenario:**
```
1. User tracking Tab A in dev mode
2. User edits code, reloads extension (chrome://extensions → Reload)
3. Content scripts re-inject
4. Background script restarts
5. LLM tries to operate
```

**Expected Behavior:**
- `trackedTabId` persists in chrome.storage.local (survives reload)
- Content scripts re-run initialization
- Call `updateTrackingStatus()` on load
- Resume tracking Tab A seamlessly

**Implementation:**
```javascript
// In content.js - run on every script load
(async function init() {
  await updateTrackingStatus()

  console.log('[Gasoline Content] Initialized:', {
    tabId: currentTabId,
    isTracked: isTrackedTab,
  })
})()
```

**Edge Case Within Edge Case: Tab No Longer Exists**
```
1. User tracked Tab A (ID: 123)
2. User closed Tab A
3. User reloaded extension
4. Extension reads trackedTabId: 123 (stale)
5. Tries to operate on non-existent tab
```

**Solution:**
Already handled by try/catch in background.js (see Edge Case #1)

**Test:**
```javascript
// 1. Track tab A
await trackTab(tabA.id)

// 2. Generate activity
await generateActivity(tabA.id)

// 3. Reload extension
await reloadExtension()

// 4. Wait for re-initialization
await sleep(1000)

// 5. Generate more activity
await generateActivity(tabA.id)

// 6. Verify still tracking Tab A
const bodies = await observe({ what: 'network_bodies' })
assert(bodies.count > 0, 'Should still capture after reload')
```

**Risk:** Low - Chrome APIs handle gracefully
**Frequency:** High (in dev), Low (in production)
**Status:** ✅ Handled by storage persistence

---

### 11. Content Script Not Loaded

**Scenario:**
```
1. User opens new tab to fast-loading page
2. User immediately clicks "Track This Tab" (before content script loads)
3. Page loads, no inject.js or content.js yet
4. LLM tries to observe
```

**Expected Behavior:**
- Extension should detect content script not ready
- Return clear error to LLM
- Suggest waiting or refreshing tab

**Implementation:**

```javascript
// In background.js - check content script is loaded
async function ensureContentScriptLoaded(tabId) {
  try {
    // Try to ping content script
    const response = await chrome.tabs.sendMessage(tabId, {
      type: 'PING',
    })

    return response?.success === true
  } catch (err) {
    // Content script not loaded
    return false
  }
}

// In query handler
if (storage.trackedTabId) {
  const loaded = await ensureContentScriptLoaded(storage.trackedTabId)

  if (!loaded) {
    return {
      error: 'content_script_not_loaded',
      message: 'The tracked tab is not ready yet',
      suggestion: 'Wait a moment for the page to load, or refresh the tab',
      tracked_tab_id: storage.trackedTabId,
    }
  }
}
```

```javascript
// In content.js - respond to ping
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'PING') {
    sendResponse({ success: true, tabId: currentTabId })
    return true
  }

  // ... other handlers
})
```

**Test:**
```javascript
// 1. Open new tab (don't wait for load)
const tabId = await openTabNoWait('https://example.com')

// 2. Immediately track
await trackTab(tabId)

// 3. Immediately try to observe (content script not loaded yet)
const result = await observe({ what: 'page' })

// Expected: Error or wait for ready
assert(
  result.error === 'content_script_not_loaded' || result.readyState === 'loading',
  'Should handle content script not loaded'
)

// 4. Wait for page load
await waitForPageLoad(tabId)

// 5. Try again
const result2 = await observe({ what: 'page' })
assert(result2.url, 'Should work after page loaded')
```

**Risk:** High - Common scenario, confusing error
**Frequency:** Medium - Fast clicks, slow pages
**Status:** ⚠️ Needs implementation

---

### 13. Browser Restart

**Scenario:**
```
1. User tracking Tab A
2. User closes browser completely (Cmd+Q)
3. User reopens browser (may or may not restore tabs)
4. LLM tries to operate
```

**Expected Behavior:**

**Case A: Browser restores tabs**
- Tab IDs change on restore (Chrome assigns new IDs)
- `trackedTabId` in storage is now invalid (points to old ID)
- Should detect and enter "No Tracking" mode

**Case B: Browser doesn't restore tabs**
- No tabs from previous session
- `trackedTabId` points to non-existent tab
- Should detect and enter "No Tracking" mode

**Implementation:**
Already handled by Edge Case #1 (tracked tab closed)

```javascript
// On browser restart, Chrome APIs return error for invalid tab ID
// Background script catches error, clears tracking, falls back
```

**Improvement: Clear on Browser Startup**

```javascript
// In background.js - run on extension startup
chrome.runtime.onStartup.addListener(async () => {
  console.log('[Gasoline] Browser restarted')

  // Tab IDs are invalid after restart - clear tracking
  await chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])

  console.log('[Gasoline] Cleared tracking state (browser restart)')
})
```

**Test:**
```javascript
// Manual test only (can't automate browser restart)
// 1. Track a tab
// 2. Note the tab ID
// 3. Close browser
// 4. Reopen browser
// 5. Check storage - trackedTabId should be cleared
// 6. Try to observe - should get "tracking_disabled" error
```

**Risk:** Low - Graceful degradation
**Frequency:** High - Users restart browsers often
**Status:** ✅ Handled (can improve with onStartup clear)

---

## P1 Edge Cases (Should Handle)

### 3. Multiple Windows

**Scenario:**
```
1. User has 3 Chrome windows open
2. Window 1: Tracking Tab A
3. User opens popup in Window 2
4. Clicks "Track This Tab" in Window 2
```

**Expected Behavior:**
- `trackedTabId` is global (single value in storage)
- Tracking switches from Window 1 Tab A → Window 2 Tab B
- Window 1's popup updates to show "Track This Tab" (not tracking anymore)
- Window 2's popup shows "Stop Tracking"

**Implementation:**
Already handled - `trackedTabId` is global across all windows

**UI Consideration:**
Show which window the tracked tab is in:

```javascript
// In popup.js
chrome.tabs.get(result.trackedTabId, (tab) => {
  chrome.windows.get(tab.windowId, (win) => {
    urlDisplay.textContent = `📍 Tracking: ${tab.url} (Window ${win.id})`
  })
})
```

**Test:**
```javascript
// 1. Open 2 windows
const win1 = await openWindow()
const win2 = await openWindow()

// 2. Open tab in each
const tabA = await openTabInWindow(win1.id, 'https://example.com')
const tabB = await openTabInWindow(win2.id, 'https://test.com')

// 3. Track tab in window 1
await trackTab(tabA.id)

// 4. Verify tracking in window 1
assert(await isTracking(tabA.id))

// 5. Track different tab in window 2
await trackTab(tabB.id)

// 6. Verify tracking switched
assert(await isTracking(tabB.id))
assert(!(await isTracking(tabA.id)))
```

**Risk:** Low - Clear behavior
**Frequency:** Low - Most users have 1 window
**Status:** ✅ Handled by global storage

---

### 6. Tab Crashes or Becomes Unresponsive

**Scenario:**
```
1. User tracking Tab A
2. Tab A crashes due to memory leak or page error
3. Tab shows "Aw, Snap!" or "Page Unresponsive"
4. LLM tries: interact({action: "navigate", ...})
```

**Expected Behavior:**
- Chrome can't send messages to crashed tab
- Extension detects error
- Return clear error to LLM
- DO NOT clear tracking (user might refresh)

**Implementation:**

```javascript
// In background.js - wrap tab messaging
async function sendMessageToTab(tabId, message, timeout = 5000) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error('tab_timeout'))
    }, timeout)

    chrome.tabs.sendMessage(tabId, message, (response) => {
      clearTimeout(timer)

      if (chrome.runtime.lastError) {
        const err = chrome.runtime.lastError.message

        if (err.includes('Receiving end does not exist')) {
          resolve({
            error: 'tab_unresponsive',
            message: 'The tracked tab is not responding',
            suggestion: 'The tab may have crashed. Try refreshing it or track a different tab.',
            tracked_tab_id: tabId,
          })
        } else {
          reject(new Error(err))
        }
      } else {
        resolve(response)
      }
    })
  })
}

// Use in interact handler
const response = await sendMessageToTab(tabId, {
  type: 'NAVIGATE',
  url: params.url,
})

if (response.error === 'tab_unresponsive') {
  return JSONRPCResponse{
    JSONRPC: "2.0",
    ID: req.ID,
    Result: response,  // Pass error to LLM
  }
}
```

**Test:**
```javascript
// Manual test (hard to automate tab crash)
// 1. Track a tab
// 2. In DevTools console: while(true) {} (hang tab)
// 3. Try to navigate via LLM
// Expected: "tab_unresponsive" error, NOT crash
```

**Risk:** Medium - Could confuse LLM
**Frequency:** Medium - Pages crash occasionally
**Status:** ⚠️ Needs implementation

---

### 8. Chrome Internal Pages

**Scenario:**
```
1. User is on chrome://extensions
2. User clicks "Track This Tab"
```

**Expected Behavior:**
- Button should be disabled/grayed out
- Tooltip: "Cannot track internal Chrome pages"
- If user somehow bypasses: Return error

**Reason:**
- Chrome blocks content scripts from chrome:// URLs
- No inject.js, no content.js loaded
- Can't capture anything anyway

**Implementation:**

```javascript
// In popup.js
function isInternalUrl(url) {
  if (!url) return true

  const internalProtocols = [
    'chrome://',
    'chrome-extension://',
    'about:',
    'edge://',
    'brave://',
    'devtools://',
  ]

  return internalProtocols.some(protocol => url.startsWith(protocol))
}

export async function initTrackPageButton() {
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    const tab = tabs[0]

    if (isInternalUrl(tab.url)) {
      btn.disabled = true
      btn.textContent = 'Cannot Track Internal Pages'
      btn.title = 'Chrome blocks extensions on internal pages like chrome:// and about:'
      btn.style.opacity = '0.5'
      return
    }

    // ... normal init logic
  })
}
```

**Test:**
```javascript
// 1. Navigate to chrome://extensions
await navigate('chrome://extensions')

// 2. Open popup
const popup = await openPopup()

// 3. Verify button disabled
assert(popup.trackButton.disabled === true)
assert(popup.trackButton.textContent.includes('Cannot Track'))

// 4. Try to track anyway (bypass)
try {
  await trackTab(currentTabId)
  assert.fail('Should not allow tracking internal pages')
} catch (err) {
  assert(err.message.includes('internal'))
}
```

**Risk:** Low - Clear UX
**Frequency:** Low - Users rarely on chrome:// pages
**Status:** ⚠️ Needs implementation

---

### 10. Tab Navigation During LLM Operation

**Scenario:**
```
1. LLM sends: interact({action: "navigate", url: "https://example.com"})
2. Command is async (takes 500ms to execute)
3. User manually navigates to different URL in same tab
4. LLM's navigate completes, but wrong page loaded
```

**Expected Behavior:**
- Detect user navigation interrupted command
- Abort pending navigation
- Return error: "navigation_interrupted"
- LLM can retry if needed

**Implementation:**

```javascript
// In inject.js - track navigation IDs
let currentNavigationId = 0

window.addEventListener('message', (event) => {
  if (event.data.type === 'GASOLINE_NAVIGATE') {
    const navId = ++currentNavigationId

    // Start navigation
    setTimeout(() => {
      // Check if navigation ID still current
      if (currentNavigationId !== navId) {
        // User navigated in the meantime
        window.postMessage({
          type: 'GASOLINE_NAVIGATE_RESPONSE',
          result: {
            error: 'navigation_interrupted',
            message: 'User navigated before command completed',
          },
        }, window.location.origin)
        return
      }

      // Proceed with navigation
      window.location.href = event.data.url
    }, 0)
  }
})

// Also listen for beforeunload
window.addEventListener('beforeunload', () => {
  currentNavigationId++  // Invalidate pending navigation
})
```

**Test:**
```javascript
// Timing-sensitive test
// 1. Track tab
// 2. Send navigate command (don't await)
const navPromise = interact({ action: 'navigate', url: 'https://example.com' })

// 3. Immediately user-navigate (simulate user action)
await userNavigate(tabId, 'https://other-site.com')

// 4. Wait for command result
const result = await navPromise

// Expected: Error or no-op (not navigate to example.com)
assert(result.error === 'navigation_interrupted' || result.url !== 'https://example.com')
```

**Risk:** Medium - Could cause confusion
**Frequency:** Medium - Users multitask
**Status:** ⚠️ Needs implementation

---

### 17. Race Condition: Tracking Toggle + Navigation

**Scenario:**
```
Timeline:
T0: User clicks "Track This Tab" (Tab A)
T1: Storage write starts (async)
T2: LLM sends navigate command
T3: Storage write completes
T4: Navigate executes

Question: At T2, is tracking enabled yet?
```

**Expected Behavior:**
- Storage writes should complete before operations execute
- LLM should wait for "tracking enabled" confirmation

**Implementation:**

```javascript
// In popup.js - return promise on tracking change
export async function handleTrackPageClick() {
  if (result.trackedTabId) {
    // Stop tracking
    await chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
    await sendStatusPing()  // Notify server immediately
  } else {
    // Start tracking
    await chrome.storage.local.set({
      trackedTabId: tab.id,
      trackedTabUrl: tab.url,
    })
    await sendStatusPing()  // Notify server immediately
  }

  // Wait for content scripts to update
  await new Promise(resolve => setTimeout(resolve, 100))
}
```

```javascript
// In background.js - debounce status ping
let statusPingScheduled = false

function scheduleStatusPing() {
  if (statusPingScheduled) return

  statusPingScheduled = true

  setTimeout(() => {
    sendStatusPing()
    statusPingScheduled = false
  }, 100)
}

chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    scheduleStatusPing()
  }
})
```

**Test:**
```javascript
// Race condition test
// 1. Click track (don't wait)
const trackPromise = trackTab(tabA.id)

// 2. Immediately try to observe
const observePromise = observe({ what: 'page' })

// 3. Wait for both
const [trackResult, observeResult] = await Promise.all([trackPromise, observePromise])

// Expected: Observe should see tracking enabled (or get error if not ready)
assert(
  observeResult.url || observeResult.error === 'tracking_not_ready',
  'Should handle race condition gracefully'
)
```

**Risk:** Medium - Could cause flaky behavior
**Frequency:** Low - Very fast users
**Status:** ⚠️ Needs debounce + wait logic

---

### 19. LLM Opens New Tab

**Scenario:**
```
1. User tracking Tab A
2. LLM sends: interact({action: "execute_js", script: "window.open('https://example.com')"})
3. New Tab B opens
4. LLM calls observe()
```

**Expected Behavior:**

**Option A: Stay on Tab A (Recommended)**
- New Tab B opens but is NOT tracked
- observe() still returns data from Tab A
- LLM must explicitly switch tracking to Tab B if desired

**Option B: Auto-switch to new tab**
- New Tab B automatically becomes tracked
- observe() returns data from Tab B
- Tab A no longer tracked

**Recommendation: Option A**
- More predictable (user controls tracking)
- Less surprising behavior
- LLM can check new tab and decide to track it

**Implementation (Option A):**
No special handling needed - default behavior

**LLM Workflow:**
```javascript
// 1. LLM opens new tab
await interact({
  action: 'execute_js',
  script: "window.open('https://example.com'); return 'opened'"
})

// 2. New tab opens (not tracked)

// 3. LLM can query all tabs
const tabs = await observe({ what: 'tabs' })

// 4. LLM finds new tab
const newTab = tabs.find(t => t.url.includes('example.com'))

// 5. LLM asks user to track new tab
// "I've opened example.com in a new tab. To inspect it, please track that tab."

// Or: Auto-switch (if we implement that)
await configure({ action: 'track_tab', tab_id: newTab.id })
```

**Test:**
```javascript
// 1. Track Tab A
await trackTab(tabA.id)

// 2. Open new tab via script
await interact({
  action: 'execute_js',
  script: "window.open('https://example.com')"
})

// 3. Wait for new tab
await sleep(1000)

// 4. Observe - should still target Tab A
const page = await observe({ what: 'page' })
assert(page.url === tabA.url, 'Should stay on original tracked tab')

// 5. New tab should NOT send data
const bodies = await observe({ what: 'network_bodies' })
assert(bodies.networkRequestResponsePairs.every(pair => pair.pageURL === tabA.url))
```

**Risk:** Medium - Could confuse LLM
**Frequency:** Medium - LLMs often open links
**Status:** ⚠️ Needs specification (Option A is default, no code change)

---

## P2 Edge Cases (Nice to Have)

### 5. Storage Cleared

**Scenario:**
```
1. User tracking Tab A
2. User manually clears extension storage (DevTools → Application → Storage)
3. Or browser clears storage due to quota
```

**Expected Behavior:**
- Extension detects no `trackedTabId`
- Enters "No Tracking" mode
- Popup shows "Track This Tab"
- No crash

**Implementation:**
Already handled - default behavior when storage empty

**Test:**
```javascript
// 1. Track tab
await trackTab(tabA.id)

// 2. Clear storage
await chrome.storage.local.clear()

// 3. Trigger content script update
await chrome.storage.onChanged.dispatch({})

// 4. Verify no tracking
const status = await getTrackingStatus()
assert(status.tracking_enabled === false)

// 5. Try to observe
const result = await observe({ what: 'page' })
assert(result.error === 'tracking_disabled')
```

**Risk:** Low - Rare, graceful degradation
**Frequency:** Low - Power users only
**Status:** ✅ Handled by default behavior

---

### 7. No Tabs Open

**Scenario:**
```
1. User closes all tabs
2. Only extension popup is open
3. User tries to click "Track This Tab"
```

**Expected Behavior:**
- Button should be disabled
- Tooltip: "No tabs available to track"
- Or: Show message "Open a tab first"

**Implementation:**

```javascript
// In popup.js
chrome.tabs.query({}, (tabs) => {
  // Filter out popup tab
  const normalTabs = tabs.filter(t => !t.url.startsWith('chrome-extension://'))

  if (normalTabs.length === 0) {
    btn.disabled = true
    btn.textContent = 'No Tabs Available'
    btn.title = 'Open a webpage to enable tracking'
    btn.style.opacity = '0.5'
    return
  }

  // ... normal init
})
```

**Test:**
```javascript
// 1. Close all tabs except popup
await closeAllTabs()

// 2. Open popup
const popup = await openPopup()

// 3. Verify button disabled
assert(popup.trackButton.disabled === true)

// 4. Open a tab
const tab = await openTab('https://example.com')

// 5. Refresh popup
await popup.refresh()

// 6. Verify button enabled
assert(popup.trackButton.disabled === false)
```

**Risk:** Low - Edge case
**Frequency:** Rare - Users always have tabs
**Status:** ⚠️ Nice to have

---

### 9. Rapid Toggle

**Scenario:**
```
1. User clicks "Track This Tab"
2. User immediately clicks "Stop Tracking" (within 100ms)
3. User clicks "Track This Tab" again
4. Race conditions in storage/state
```

**Expected Behavior:**
- Last click wins
- No flickering or inconsistent state
- Debounce to prevent rapid changes

**Implementation:**

```javascript
// In popup.js - add debounce
let toggleInProgress = false
let toggleTimeout = null

export async function handleTrackPageClick() {
  // Clear any pending toggle
  if (toggleTimeout) {
    clearTimeout(toggleTimeout)
  }

  // Debounce - wait 200ms for more clicks
  toggleTimeout = setTimeout(async () => {
    if (toggleInProgress) {
      console.log('[Gasoline] Toggle already in progress')
      return
    }

    toggleInProgress = true
    btn.disabled = true
    btn.textContent = '...'

    try {
      // ... actual toggle logic ...
    } finally {
      toggleInProgress = false
      btn.disabled = false
      toggleTimeout = null
      await initTrackPageButton()  // Refresh UI
    }
  }, 200)
}
```

**Test:**
```javascript
// Automated test
// 1. Click track 5 times rapidly
for (let i = 0; i < 5; i++) {
  clickTrackButton()
  await sleep(50)  // 50ms between clicks
}

// 2. Wait for debounce
await sleep(300)

// 3. Check final state (should be consistent)
const status = await getTrackingStatus()
assert(typeof status.tracking_enabled === 'boolean', 'Should have consistent state')
```

**Risk:** Low - Annoying but not breaking
**Frequency:** Low - Users don't click that fast
**Status:** ⚠️ Nice to have (debounce)

---

### 14. Tab Duplicated

**Scenario:**
```
1. User tracking Tab A (ID: 123)
2. User right-clicks tab → "Duplicate"
3. New Tab B created (ID: 456, same URL as Tab A)
4. Which tab is tracked?
```

**Expected Behavior:**
- Tab A (original) remains tracked
- Tab B (duplicate) is NOT tracked
- User can manually switch tracking to Tab B if desired

**Implementation:**
No special handling needed - `trackedTabId` remains 123

**Edge Case: User closes Tab A, keeps Tab B**
- Tab A (tracked) closed → tracking cleared
- Tab B (duplicate, untracked) remains open
- Falls back to "No Tracking" mode
- User can track Tab B

**Test:**
```javascript
// 1. Track Tab A
await trackTab(tabA.id)

// 2. Duplicate tab
const tabB = await duplicateTab(tabA.id)

// 3. Verify Tab A still tracked, Tab B not
assert(await isTracked(tabA.id))
assert(!(await isTracked(tabB.id)))

// 4. Generate activity in both
await generateActivity(tabA.id)
await generateActivity(tabB.id)

// 5. Observe - should only see Tab A data
const bodies = await observe({ what: 'network_bodies' })
assert(bodies.count > 0)
// Can't easily verify it's from Tab A vs B (same URL), but filter logic ensures it
```

**Risk:** Low - Expected behavior
**Frequency:** Low - Duplicate tab feature not commonly used
**Status:** ✅ Handled by tab ID tracking

---

### 15. Tab Moved Between Windows

**Scenario:**
```
1. User tracking Tab A in Window 1
2. User drags tab to Window 2
3. Tab ID stays the same (Chrome preserves ID)
4. Is it still tracked?
```

**Expected Behavior:**
- Yes - `trackedTabId` points to same tab ID
- Tab location (window) doesn't matter
- Continue tracking seamlessly

**Implementation:**
No special handling needed - tab ID is window-independent

**Test:**
```javascript
// 1. Track Tab A in Window 1
const win1 = await openWindow()
const tabA = await openTabInWindow(win1.id, 'https://example.com')
await trackTab(tabA.id)

// 2. Move tab to Window 2
const win2 = await openWindow()
await moveTabToWindow(tabA.id, win2.id)

// 3. Verify still tracked
assert(await isTracked(tabA.id))

// 4. Generate activity
await generateActivity(tabA.id)

// 5. Observe - should still work
const bodies = await observe({ what: 'network_bodies' })
assert(bodies.count > 0)
```

**Risk:** Low - Chrome handles seamlessly
**Frequency:** Rare - Power users only
**Status:** ✅ Handled by Chrome

---

### 16. Incognito Mode

**Scenario:**
```
1. User opens incognito window
2. Opens tab in incognito
3. Extension runs in incognito (if allowed)
4. User tries to track incognito tab
```

**Expected Behavior (Needs Spec):**

**Option A: Block tracking in incognito**
- Don't allow tracking incognito tabs
- Privacy guarantee
- Show: "Cannot track incognito tabs"

**Option B: Allow with warning**
- Allow tracking but show warning
- "Incognito tabs are tracked same as regular tabs"
- User explicit consent

**Recommendation: Option A (block by default)**

**Implementation:**

```javascript
// In popup.js
chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
  const tab = tabs[0]

  if (tab.incognito) {
    btn.disabled = true
    btn.textContent = 'Cannot Track Incognito Tabs'
    btn.title = 'Gasoline does not track incognito tabs for privacy'
    btn.style.opacity = '0.5'
    return
  }

  // ... normal init
})
```

**Test:**
```javascript
// 1. Open incognito window
const incognitoWin = await openIncognitoWindow()

// 2. Open tab in incognito
const incognitoTab = await openTabInWindow(incognitoWin.id, 'https://example.com')

// 3. Try to track
try {
  await trackTab(incognitoTab.id)
  assert.fail('Should not allow tracking incognito tabs')
} catch (err) {
  assert(err.message.includes('incognito'))
}
```

**Risk:** Medium - Privacy concern
**Frequency:** Low - Most users don't use incognito with extensions
**Status:** ⚠️ Needs spec decision (recommend block)

---

### 20. Tab Suspended/Discarded

**Scenario:**
```
1. User tracking Tab A
2. Chrome suspends Tab A due to memory pressure (tab not used for hours)
3. Tab appears normal but is actually frozen
4. LLM tries to interact
```

**Expected Behavior:**
- Chrome unfreezes tab when extension tries to message it
- Some delay while tab reactivates
- Operation succeeds after unfreeze

**Implementation:**
Mostly handled by Chrome, but add timeout:

```javascript
// In background.js
async function sendMessageToTab(tabId, message, timeout = 10000) {  // 10s timeout for suspended tabs
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error('Tab took too long to respond (may be suspended)'))
    }, timeout)

    chrome.tabs.sendMessage(tabId, message, (response) => {
      clearTimeout(timer)
      // ... handle response
    })
  })
}
```

**Test:**
```javascript
// Manual test (hard to automate suspension)
// 1. Track a tab
// 2. Wait several hours (Chrome suspends)
// 3. Try to observe
// Expected: Slight delay, then works (or timeout error)
```

**Risk:** Low - Chrome handles automatically
**Frequency:** Low - Long idle times
**Status:** ⚠️ Add timeout for safety

---

## Race Conditions Summary

| Race | Scenario | Mitigation | Status |
|------|----------|------------|--------|
| Storage write + read | Toggle tracking, immediate observe | Debounce, wait 100ms | Needed |
| Tracking change + message in flight | Switch tabs, message from old tab | Check tab ID before send | Handled |
| Multiple storage.onChanged events | Rapid toggles | Debounce listener | Needed |
| Tab close + query in flight | Close tab, query arrives | Try/catch tab.get | Handled |
| Navigate + user navigate | LLM navigates, user also navigates | Navigation ID, detect interrupt | Needed |
| Extension reload + operation | Reload during operation | Operations timeout/retry | Handled |

---

## Testing Priority

### P0 - Must Test Before Release
1. ✅ Single tab isolation (3 tabs, track 1)
2. ✅ Tracked tab closed
3. ✅ Tracking switched mid-session
4. ✅ Extension reload
5. ✅ Content script not loaded
6. ✅ Browser restart

### P1 - Should Test Before Release
7. Multiple windows
8. Tab crashes/unresponsive
9. Chrome internal pages blocked
10. Tab navigation during LLM op
11. Race: tracking toggle + navigate

### P2 - Nice to Test
12. Storage cleared
13. No tabs open
14. Rapid toggle debounce
15. Tab duplicated
16. Tab moved between windows
17. Incognito mode (spec needed)
18. Tab suspended

---

## Implementation Checklist

### Core Filtering (P0)
- [ ] Add `isTrackedTab` state to content.js
- [ ] Add `updateTrackingStatus()` function
- [ ] Add storage.onChanged listener
- [ ] Add tab activation listener
- [ ] Modify message handler with filter
- [ ] Clear buffers on tracking switch

### UI Updates (P0)
- [ ] Change "Track This Page" → "Track This Tab"
- [ ] Block chrome:// URLs
- [ ] Show "No tracking" message
- [ ] Disable button on internal pages

### Server Communication (P0)
- [ ] Add status ping endpoint
- [ ] Send ping every 30s
- [ ] Add tracking state to ping payload
- [ ] Check tracking status in tools
- [ ] Return "tracking_disabled" error

### Error Handling (P1)
- [ ] Content script not loaded detection
- [ ] Tab crash detection & error
- [ ] Navigation interrupt detection
- [ ] Timeout for suspended tabs

### Edge Cases (P2)
- [ ] Debounce rapid toggle
- [ ] Clear tracking on browser startup
- [ ] Disable button when no tabs
- [ ] Incognito mode decision

---

## Risk Assessment

| Risk Category | Level | Mitigation |
|---------------|-------|------------|
| Data leakage (untracked tabs) | CRITICAL | P0 fix - tab filtering |
| Race conditions | MEDIUM | Debounce, timeouts |
| User confusion (tracking disabled) | MEDIUM | Clear error messages |
| Extension crashes | LOW | Try/catch, graceful errors |
| Performance degradation | LOW | Single boolean check |
| Backward compatibility | LOW | No API changes |

---

## Sign-off

**Edge Cases Reviewed:**
- [ ] Product Owner
- [ ] Technical Lead
- [ ] QA Engineer

**Test Coverage:**
- [ ] Unit tests written
- [ ] Integration tests written
- [ ] UAT checklist updated

**Ready for Implementation:** ❌ (Awaiting spec approval)

---

## Change Log

| Date | Version | Changes |
|------|---------|---------|
| 2026-01-27 | 1.0 | Initial edge case analysis |
