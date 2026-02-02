# Track This Tab - Technical Specification

**Version:** 1.0
**Date:** 2026-01-27
**Status:** DRAFT - Awaiting Review
**Priority:** CRITICAL (Security/Privacy Issue)

---

## Technical Summary

This specification details the implementation of single-tab tracking isolation. Currently, the extension captures telemetry from ALL tabs regardless of the `trackedTabId` setting. This fix implements proper tab-scoped filtering at the content script layer.

**Implementation Complexity:** Medium (2-3 hours)
**Risk Level:** Low (isolated change, no API surface changes)
**Files Changed:** 5-7 files

---

## Architecture Overview

### Current Data Flow (INCORRECT)

```
┌─────────────┐
│   Tab 1     │  → inject.js → content.js → background.js → Server
│ (Tracked)   │     ✓ Captures everything
└─────────────┘

┌─────────────┐
│   Tab 2     │  → inject.js → content.js → background.js → Server
│ (Random)    │     ✓ ALSO captures (BUG!)
└─────────────┘

┌─────────────┐
│   Tab 3     │  → inject.js → content.js → background.js → Server
│ (Banking)   │     ✓ ALSO captures (SECURITY ISSUE!)
└─────────────┘
```

**Problem:** `trackedTabId` only used for query routing, NOT capture filtering.

### New Data Flow (CORRECT)

```
┌─────────────┐
│   Tab 1     │  inject.js → content.js (FILTER) → background.js → Server
│ (Tracked)   │                   ✓ PASS              ✓ Captures
└─────────────┘

┌─────────────┐
│   Tab 2     │  inject.js → content.js (FILTER) → [BLOCKED]
│ (Random)    │                   ✗ DROP
└─────────────┘

┌─────────────┐
│   Tab 3     │  inject.js → content.js (FILTER) → [BLOCKED]
│ (Banking)   │                   ✗ DROP
└─────────────┘
```

**Solution:** Filter in content.js before forwarding to background.js.

---

## Implementation Design

### Option 1: Filter in content.js (RECOMMENDED)

**Why Here:**
- Content script runs per-tab (has tab context)
- Earliest point in pipeline with tab awareness
- Minimal performance impact (in-memory check)
- No changes to inject.js or background.js message handlers

**Code Location:** `extension/content.js`

**Implementation:**
```javascript
// Add at top of file
let isTrackedTab = false
let currentTabId = null

// Initialize on load
async function updateTrackingStatus() {
  try {
    const storage = await chrome.storage.local.get(['trackedTabId'])
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
    currentTabId = tabs[0]?.id
    isTrackedTab = (currentTabId === storage.trackedTabId)

    console.log('[Gasoline] Tab tracking status:', {
      currentTabId,
      trackedTabId: storage.trackedTabId,
      isTracked: isTrackedTab
    })
  } catch (err) {
    console.error('[Gasoline] Failed to check tracking status:', err)
    isTrackedTab = false
  }
}

// Call on script load
updateTrackingStatus()

// Listen for tracking changes
chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    updateTrackingStatus()
  }
})

// Also update when tab becomes active (user switches back to this tab)
chrome.tabs.onActivated.addListener((activeInfo) => {
  updateTrackingStatus()
})

// MODIFY message handler
window.addEventListener('message', (event) => {
  if (event.source !== window) return

  // Filter: Only forward messages if this is the tracked tab
  if (!isTrackedTab) {
    // Optional: Log that we're dropping messages
    if (event.data?.type?.startsWith('GASOLINE_')) {
      console.debug('[Gasoline] Dropping message from untracked tab:', event.data.type)
    }
    return  // ← NEW: Drop messages from untracked tabs
  }

  const msgType = MESSAGE_MAP[event.data?.type]
  if (msgType) {
    safeSendMessage({ type: msgType, payload: event.data.payload })
  }
})
```

**Benefits:**
- Simple: ~30 lines of code
- Fast: Single boolean check per message
- Reliable: Uses Chrome's tab APIs
- Maintainable: All filtering logic in one place

**Drawbacks:**
- Requires tab query APIs (minimal overhead)
- Must handle async initialization

### Option 2: Filter in background.js (NOT RECOMMENDED)

**Why Not:**
- Messages already batched and in flight
- Must track sender.tab.id in every message type
- More complex (modify 10+ message handlers)
- Higher memory usage (storing tab IDs per message)

### Option 3: Filter in inject.js (NOT RECOMMENDED)

**Why Not:**
- inject.js runs in page context (no access to chrome.tabs API)
- Would need to pass tab ID from content.js to inject.js
- More complex message passing
- inject.js doesn't know its own tab ID natively

---

## Detailed Implementation

### File 1: extension/content.js

**Changes Required:**
1. Add tab tracking state variables
2. Add `updateTrackingStatus()` function
3. Call on script load
4. Listen to storage changes
5. Listen to tab activation
6. Modify message handler with filter

**Code Sections:**

```javascript
// ===== SECTION 1: State Variables (add at top) =====
let isTrackedTab = false
let currentTabId = null

// ===== SECTION 2: Tracking Status Function =====
async function updateTrackingStatus() {
  try {
    const storage = await chrome.storage.local.get(['trackedTabId'])
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
    currentTabId = tabs[0]?.id
    isTrackedTab = (currentTabId === storage.trackedTabId)

    // Log for debugging
    if (process.env.NODE_ENV === 'development') {
      console.log('[Gasoline Content] Tracking status updated:', {
        currentTabId,
        trackedTabId: storage.trackedTabId,
        isTracked: isTrackedTab,
        url: window.location.href,
      })
    }
  } catch (err) {
    console.error('[Gasoline Content] Failed to update tracking status:', err)
    isTrackedTab = false
  }
}

// ===== SECTION 3: Initialize on Load =====
updateTrackingStatus()

// ===== SECTION 4: Storage Listener =====
chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    console.log('[Gasoline Content] trackedTabId changed:', {
      from: changes.trackedTabId.oldValue,
      to: changes.trackedTabId.newValue,
    })
    updateTrackingStatus()
  }
})

// ===== SECTION 5: Tab Activation Listener =====
chrome.tabs.onActivated.addListener((activeInfo) => {
  // When user switches to this tab, re-check tracking status
  updateTrackingStatus()
})

// ===== SECTION 6: Modified Message Handler =====
window.addEventListener('message', (event) => {
  if (event.source !== window) return

  // NEW: Tab isolation filter
  if (!isTrackedTab) {
    // Drop all GASOLINE_ messages from untracked tabs
    if (event.data?.type?.startsWith('GASOLINE_')) {
      // Only log in development to avoid console spam
      if (process.env.NODE_ENV === 'development') {
        console.debug('[Gasoline Content] Dropped message from untracked tab:', {
          type: event.data.type,
          currentTabId,
        })
      }
    }
    return  // ← Drop message
  }

  // Existing message forwarding logic
  const msgType = MESSAGE_MAP[event.data?.type]
  if (msgType) {
    safeSendMessage({ type: msgType, payload: event.data.payload })
  }

  // ... rest of existing handlers (A11Y_QUERY_RESPONSE, etc.)
})
```

**Testing:**
```javascript
// Manual test in browser console (content script context):
// 1. Open 3 tabs
// 2. Track tab 1
// 3. In tab 2 console, check:
console.log('Is tracked?', isTrackedTab)  // Should be false
console.log('Current tab ID:', currentTabId)
console.log('Tracked tab ID:', await chrome.storage.local.get('trackedTabId'))

// 4. Try sending a test message:
window.postMessage({ type: 'GASOLINE_LOG', payload: { level: 'info', message: 'test' } }, '*')
// Should be dropped (not forwarded to background)

// 5. Switch to tab 1, repeat test
// Should be forwarded to background
```

---

### File 2: extension/popup.js

**Changes Required:**
1. Change button text: "Track This Page" → "Track This Tab"
2. Update tooltips and descriptions
3. Add confirmation dialog for tracking switch (optional)
4. Update URL display text

**Code Changes:**

```javascript
// Line ~270: Change button text
export async function initTrackPageButton() {
  const btn = document.getElementById('track-page-btn')
  const urlDisplay = document.getElementById('tracked-url-display')

  chrome.storage.local.get(['trackedTabId', 'trackedTabUrl'], async (result) => {
    if (result.trackedTabId) {
      btn.textContent = 'Stop Tracking'
      btn.style.background = '#f85149'
      btn.title = 'Stop capturing telemetry from this tab'  // ← Updated

      if (urlDisplay && result.trackedTabUrl) {
        urlDisplay.textContent = `📍 Tracking: ${result.trackedTabUrl}`
        urlDisplay.style.display = 'block'
      }
    } else {
      btn.textContent = 'Track This Tab'  // ← Changed from "Page"
      btn.style.background = '#252525'
      btn.title = 'Start capturing telemetry from this browser tab only'  // ← Updated

      if (urlDisplay) {
        urlDisplay.textContent = '⚠️ No tab tracked - data capture disabled'
        urlDisplay.style.display = 'block'
        urlDisplay.style.color = '#f85149'
      }
    }

    btn.addEventListener('click', handleTrackPageClick)
  })
}

// Optional: Add confirmation when switching tabs
export async function handleTrackPageClick() {
  chrome.storage.local.get(['trackedTabId'], async (result) => {
    if (result.trackedTabId) {
      // Stopping tracking - no confirmation needed
      await chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
      console.log('[Gasoline] Tracking disabled')
    } else {
      // Starting tracking
      chrome.tabs.query({ active: true, currentWindow: true }, async (tabs) => {
        if (tabs[0]) {
          const tab = tabs[0]

          // Block chrome:// and other internal URLs
          if (isInternalUrl(tab.url)) {
            alert('Cannot track internal Chrome pages. Please navigate to a regular webpage.')
            return
          }

          await chrome.storage.local.set({
            trackedTabId: tab.id,
            trackedTabUrl: tab.url,
          })
          console.log('[Gasoline] Now tracking tab:', tab.id, tab.url)
        }
      })
    }

    // Refresh popup UI
    initTrackPageButton()
  })
}

// Helper: Check if URL is internal/blocked
function isInternalUrl(url) {
  if (!url) return true
  return (
    url.startsWith('chrome://') ||
    url.startsWith('chrome-extension://') ||
    url.startsWith('about:') ||
    url.startsWith('edge://') ||
    url.startsWith('brave://')
  )
}
```

---

### File 3: extension/popup.html

**Changes Required:**
1. Update button text
2. Add URL display element (if not exists)
3. Update tooltip/help text

**Code Changes:**

```html
<!-- Update button -->
<button id="track-page-btn" class="btn btn-primary">
  Track This Tab  <!-- Changed from "Track This Page" -->
</button>

<!-- Add URL display if not exists -->
<div id="tracked-url-display" style="
  margin-top: 10px;
  font-size: 12px;
  color: #8b949e;
  word-break: break-all;
  display: none;
">
  ⚠️ No tab tracked - data capture disabled
</div>

<!-- Update help text -->
<div class="help-text">
  Click "Track This Tab" to enable telemetry capture from this browser tab only.
  Other tabs will not be monitored.
</div>
```

---

### File 4: cmd/dev-console/status.go (New Handler)

**Purpose:** Handle "no tracking" status pings from extension

**Implementation:**

```go
// Add to status.go or create new file
type ExtensionStatus struct {
    Type               string    `json:"type"`
    TrackingEnabled    bool      `json:"tracking_enabled"`
    TrackedTabID       int       `json:"tracked_tab_id,omitempty"`
    TrackedTabURL      string    `json:"tracked_tab_url,omitempty"`
    Message            string    `json:"message,omitempty"`
    ExtensionConnected bool      `json:"extension_connected"`
    Timestamp          time.Time `json:"timestamp"`
}

// POST /api/extension-status
func (h *Handler) handleExtensionStatus(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var status ExtensionStatus
    if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Store latest status in memory
    h.mu.Lock()
    h.lastExtensionStatus = status
    h.lastStatusUpdate = time.Now()
    h.mu.Unlock()

    // Log status for debugging
    if !status.TrackingEnabled {
        log.Printf("[Extension Status] No tab tracking enabled")
    } else {
        log.Printf("[Extension Status] Tracking tab %d: %s", status.TrackedTabID, status.TrackedTabURL)
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "received": true,
        "timestamp": time.Now(),
    })
}

// Add getter for status
func (h *Handler) GetExtensionStatus() (ExtensionStatus, time.Time, bool) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    // Consider status stale after 60 seconds
    if time.Since(h.lastStatusUpdate) > 60*time.Second {
        return ExtensionStatus{}, h.lastStatusUpdate, false
    }

    return h.lastExtensionStatus, h.lastStatusUpdate, true
}
```

---

### File 5: cmd/dev-console/tools.go (Modify All Tools)

**Purpose:** Check tracking status before executing tools, return helpful errors

**Implementation:**

```go
// Add helper function
func (h *ToolHandler) checkTrackingStatus(req JSONRPCRequest) *JSONRPCResponse {
    status, lastUpdate, valid := h.handler.GetExtensionStatus()

    if !valid {
        // No recent status ping
        return &JSONRPCResponse{
            JSONRPC: "2.0",
            ID:      req.ID,
            Result: mcpStructuredError(
                ErrExtensionTimeout,
                "Extension status unknown - no recent ping from browser extension",
                "Check that the Gasoline extension is installed and enabled",
                withHint("Extension may be disconnected or disabled"),
            ),
        }
    }

    if !status.TrackingEnabled {
        // Extension connected but tracking disabled
        return &JSONRPCResponse{
            JSONRPC: "2.0",
            ID:      req.ID,
            Result: mcpStructuredError(
                "tracking_disabled",
                "Extension is connected but tab tracking is disabled",
                "Ask user to click 'Track This Tab' button in the Gasoline extension popup",
                withParam("tracking_status"),
                withHint("Extension connected at "+lastUpdate.Format(time.RFC3339)),
            ),
        }
    }

    // Tracking enabled - OK to proceed
    return nil
}

// Modify each tool handler to check status first
func (h *ToolHandler) toolObserve(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
    // Check tracking status
    if errResp := h.checkTrackingStatus(req); errResp != nil {
        return *errResp
    }

    // ... existing observe logic ...
}

func (h *ToolHandler) toolInteract(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
    // Check tracking status
    if errResp := h.checkTrackingStatus(req); errResp != nil {
        return *errResp
    }

    // ... existing interact logic ...
}

// Note: configure() and generate() may not need this check
// (they're meta-operations that don't require active tab)
```

---

### File 6: extension/background.js (Add Status Ping)

**Purpose:** Send periodic status updates to server

**Implementation:**

```javascript
// Add at top of file
const STATUS_PING_INTERVAL = 30000  // 30 seconds

// Add status ping function
async function sendStatusPing() {
  try {
    const storage = await chrome.storage.local.get(['trackedTabId', 'trackedTabUrl'])

    const statusMessage = {
      type: 'status',
      tracking_enabled: !!storage.trackedTabId,
      tracked_tab_id: storage.trackedTabId || null,
      tracked_tab_url: storage.trackedTabUrl || null,
      message: storage.trackedTabId ? 'tracking enabled' : 'no tab tracking enabled',
      extension_connected: true,
      timestamp: new Date().toISOString(),
    }

    // Send to server
    const response = await fetch(`${serverUrl}/api/extension-status`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(statusMessage),
    })

    if (!response.ok) {
      console.error('[Gasoline] Failed to send status ping:', response.status)
    }
  } catch (err) {
    console.error('[Gasoline] Status ping error:', err)
  }
}

// Start status ping loop on extension load
setInterval(sendStatusPing, STATUS_PING_INTERVAL)
sendStatusPing()  // Send immediately on load

// Also send status ping when tracking changes
chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    sendStatusPing()  // Immediate ping on tracking change
  }
})
```

---

## Edge Case Handling (Implementation Details)

### Edge Case 1: Tracked Tab Closed

**Detection:**
```javascript
// In background.js query handler (existing code)
try {
  const trackedTab = await chrome.tabs.get(storage.trackedTabId)
  tabs = [trackedTab]
} catch (err) {
  // Tab no longer exists - clear tracking
  console.log('[Gasoline] Tracked tab closed, clearing tracking')
  await chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])

  // Fall back to active tab or return error
  // (existing fallback logic already handles this)
}
```

**No new code needed** - already handled in background.js:2533-2557

### Edge Case 2: Tracking Switch Mid-Session

**Behavior:**
- User clicks "Stop Tracking" → storage cleared → all content scripts update
- User clicks "Track This Tab" in different tab → storage updated → all content scripts update
- Only newly tracked tab forwards messages

**Implementation:**
Already handled by `chrome.storage.onChanged` listener in content.js

### Edge Case 3: Multiple Windows

**Behavior:**
- `trackedTabId` is global across all windows
- Popup in any window shows correct tracking state
- If user tracks tab in Window 2, Window 1's tracked tab stops being tracked

**Implementation:**
No special handling needed - Chrome tab IDs are unique across windows

### Edge Case 4: Extension Reload

**Behavior:**
- `chrome.storage.local` persists across reloads
- Content scripts re-initialize with correct tracking state
- Background script re-sends status ping

**Implementation:**
Already handled by storage persistence + `updateTrackingStatus()` on load

### Edge Case 5: Tab Crashes

**Detection:**
```javascript
// In background.js when sending messages to tab
try {
  const response = await chrome.tabs.sendMessage(tabId, message)
  return response
} catch (err) {
  if (err.message.includes('Receiving end does not exist')) {
    return {
      error: 'tab_unresponsive',
      message: 'The tracked tab is not responding',
      suggestion: 'The tab may have crashed. Try refreshing it or track a different tab.',
      tracked_tab_id: tabId,
    }
  }
  throw err
}
```

**Add:** Error handling wrapper for tab messages

### Edge Case 6: Rapid Toggle

**Implementation:**
```javascript
// In popup.js - add debounce
let toggleInProgress = false

export async function handleTrackPageClick() {
  if (toggleInProgress) {
    console.log('[Gasoline] Toggle already in progress, ignoring click')
    return
  }

  toggleInProgress = true
  const btn = document.getElementById('track-page-btn')
  btn.disabled = true
  btn.textContent = '...'

  try {
    // ... existing toggle logic ...
  } finally {
    toggleInProgress = false
    btn.disabled = false
    initTrackPageButton()  // Refresh UI
  }
}
```

### Edge Case 7: Chrome Internal Pages

**Implementation:**
```javascript
// In popup.js (already shown above)
function isInternalUrl(url) {
  if (!url) return true
  return (
    url.startsWith('chrome://') ||
    url.startsWith('chrome-extension://') ||
    url.startsWith('about:') ||
    url.startsWith('edge://') ||
    url.startsWith('brave://')
  )
}

// In handleTrackPageClick:
if (isInternalUrl(tab.url)) {
  alert('Cannot track internal Chrome pages. Please navigate to a regular webpage.')
  return
}
```

---

## Performance Considerations

### Message Filter Overhead

**Current:** 0 checks
**New:** 1 boolean check per message

```javascript
if (!isTrackedTab) return  // ← ~0.001ms
```

**Impact:** Negligible (< 0.01% CPU)

### Storage Access

**updateTrackingStatus()** calls:
- On content script load: 1 call per tab
- On storage change: 1 call per tracking toggle
- On tab activation: 1 call per tab switch

**Frequency:** ~10 calls per minute (user behavior dependent)
**Cost:** chrome.storage.local.get() ~1ms, chrome.tabs.query() ~1ms
**Impact:** Negligible

### Status Ping Network

**Frequency:** 1 ping every 30 seconds
**Payload:** ~200 bytes JSON
**Bandwidth:** 0.4 KB/min = 24 KB/hour
**Impact:** Negligible

---

## Testing Strategy

### Unit Tests

**File:** `tests/extension/content-tracking.test.js`

```javascript
import { describe, it, before, after } from 'node:test'
import assert from 'node:assert'

describe('Content Script Tab Filtering', () => {
  let mockStorage = {}
  let mockTabs = []
  let currentTabId = 1

  before(() => {
    // Mock chrome.storage API
    global.chrome = {
      storage: {
        local: {
          get: (keys) => Promise.resolve(mockStorage),
          set: (data) => { Object.assign(mockStorage, data); return Promise.resolve() },
        },
        onChanged: {
          addListener: () => {},
        },
      },
      tabs: {
        query: () => Promise.resolve([{ id: currentTabId }]),
        onActivated: {
          addListener: () => {},
        },
      },
    }
  })

  it('should block messages from untracked tab', async () => {
    mockStorage = { trackedTabId: 999 }  // Different tab
    currentTabId = 1

    const result = await shouldForwardMessage(currentTabId)
    assert.strictEqual(result, false, 'Should block untracked tab')
  })

  it('should allow messages from tracked tab', async () => {
    mockStorage = { trackedTabId: 1 }
    currentTabId = 1

    const result = await shouldForwardMessage(currentTabId)
    assert.strictEqual(result, true, 'Should allow tracked tab')
  })

  it('should block all tabs when tracking disabled', async () => {
    mockStorage = {}  // No trackedTabId
    currentTabId = 1

    const result = await shouldForwardMessage(currentTabId)
    assert.strictEqual(result, false, 'Should block when tracking disabled')
  })
})

// Helper function (extracted from content.js for testing)
async function shouldForwardMessage(tabId) {
  const storage = await chrome.storage.local.get(['trackedTabId'])
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
  const currentTabId = tabs[0]?.id
  return (currentTabId === storage.trackedTabId)
}
```

### Integration Tests

**File:** `tests/integration/tab-isolation.test.js`

```javascript
describe('Tab Isolation Integration', () => {
  it('should only capture from tracked tab', async () => {
    // 1. Open 3 tabs
    const tab1 = await openTab('https://example.com')
    const tab2 = await openTab('https://test.com')
    const tab3 = await openTab('https://demo.com')

    // 2. Track tab 1
    await trackTab(tab1.id)

    // 3. Generate activity in all tabs
    await generateActivity(tab1.id)  // Should capture
    await generateActivity(tab2.id)  // Should NOT capture
    await generateActivity(tab3.id)  // Should NOT capture

    // 4. Query network bodies
    const bodies = await observe({ what: 'network_bodies' })

    // 5. Verify only tab 1 data present
    assert.strictEqual(bodies.networkRequestResponsePairs.length > 0, true)
    assert.strictEqual(
      bodies.networkRequestResponsePairs.every(pair => pair.pageURL.includes('example.com')),
      true,
      'Should only have data from tracked tab'
    )
  })

  it('should switch tracking between tabs', async () => {
    const tab1 = await openTab('https://example.com')
    const tab2 = await openTab('https://test.com')

    // Track tab 1
    await trackTab(tab1.id)
    await generateActivity(tab1.id)

    // Switch to tab 2
    await trackTab(tab2.id)
    await generateActivity(tab2.id)

    // Query data - should only see tab 2
    const bodies = await observe({ what: 'network_bodies' })
    assert.strictEqual(
      bodies.networkRequestResponsePairs.every(pair => pair.pageURL.includes('test.com')),
      true,
      'Should only have data from newly tracked tab'
    )
  })
})
```

### Manual UAT Checklist

See [UAT-TEST-PLAN.md](./UAT-TEST-PLAN.md) for full checklist. Key scenarios:

```
✅ Test 1: Single Tab Isolation
1. Open 3 tabs: A (bank), B (app), C (test)
2. Track only tab C
3. Generate activity in all 3 tabs
4. observe({what: "network_bodies"})
5. VERIFY: Only tab C data present

✅ Test 2: Tracking Switch
1. Open 2 tabs: A, B
2. Track tab A
3. Generate activity in A
4. Switch tracking to tab B
5. Generate activity in B
6. VERIFY: Only B's data present (A's cleared)

✅ Test 3: No Tracking Mode
1. Open tab, don't track
2. Call observe({what: "errors"})
3. VERIFY: Get "tracking_disabled" error

✅ Test 4: Tracked Tab Closes
1. Track tab A
2. Close tab A
3. Call observe()
4. VERIFY: Falls back gracefully, no crash

✅ Test 5: LLM Navigation
1. Track tab A
2. LLM navigates: site1 → site2 → site3 (same tab)
3. VERIFY: All sites captured in same tab
```

---

## Rollback Plan

### If Critical Bug Found

**Symptoms:**
- Extension crashes
- No tabs can be tracked
- Data loss or corruption

**Rollback Steps:**
1. Revert commit: `git revert <commit-hash>`
2. Rebuild extension: `make dev`
3. Reload extension in browser
4. Notify users via release notes

**Time to Rollback:** < 5 minutes

### Feature Flag (Alternative)

If gradual rollout preferred:

```javascript
// In content.js
const TAB_FILTERING_ENABLED = true  // Feature flag

window.addEventListener('message', (event) => {
  if (event.source !== window) return

  // Only apply filter if feature enabled
  if (TAB_FILTERING_ENABLED && !isTrackedTab) {
    return
  }

  // ... rest of handler
})
```

Set to `false` to disable filtering without full rollback.

---

## Migration & Compatibility

### Existing Users

**No migration needed:**
- Storage schema unchanged (still uses `trackedTabId`)
- API surface unchanged (no new MCP tools)
- Server protocol unchanged

**Breaking Change:**
- Previously: All tabs captured (unexpected behavior)
- Now: Only tracked tab captured (correct behavior)

**User Communication:**
```
🔒 Security Update: Tab Tracking Isolation

Gasoline now only captures telemetry from the tab you explicitly track.
Previously, data from all tabs was captured (security issue).

Action Required: None - tracking works as expected now.

If you relied on multi-tab capture, please let us know in GitHub issues.
```

### Server Compatibility

**New Endpoint:** `/api/extension-status`
**Required:** Yes (for "no tracking" mode)

**Backward Compatibility:**
- Old extension (no status ping) → Server still works, just doesn't know tracking state
- New extension + old server → Status ping fails, extension continues working (degraded mode)

**Recommended:** Deploy server first, then extension.

---

## Documentation Updates

### Files to Update

1. **README.md** - Add "Track This Tab" explanation
2. **docs/architecture.md** - Update data flow diagrams
3. **docs/privacy.md** - Add single-tab isolation guarantee
4. **docs/mcp-tools.md** - Document "tracking_disabled" error
5. **extension/popup.html** - Update help text
6. **CHANGELOG.md** - Add breaking change note

### Example README Section

```markdown
## Privacy & Security

Gasoline uses a **single-tab tracking model**:

- 🔒 Only ONE tab is tracked at a time
- ✋ Other tabs are completely isolated (no data captured)
- 📍 Click "Track This Tab" to enable tracking for current tab
- 🛑 Click "Stop Tracking" to disable tracking

**Your banking, email, and other tabs are safe** - they're never monitored
unless you explicitly track them.
```

---

## Monitoring & Observability

### Metrics to Track

**Extension:**
- Number of blocked messages per tab
- Tracking toggles per session
- Average time between toggles
- Tabs closed while tracked (recovery events)

**Server:**
- Status ping frequency
- "tracking_disabled" error rate
- LLM operations attempted while tracking disabled

**Implementation:**

```javascript
// In content.js
let droppedMessageCount = 0

window.addEventListener('message', (event) => {
  if (!isTrackedTab && event.data?.type?.startsWith('GASOLINE_')) {
    droppedMessageCount++

    // Report every 100 messages
    if (droppedMessageCount % 100 === 0) {
      console.log(`[Gasoline Metrics] Dropped ${droppedMessageCount} messages from untracked tab`)
    }
  }
  // ... rest of handler
})
```

### Debug Mode

```javascript
// Enable verbose logging
const DEBUG_TAB_FILTERING = localStorage.getItem('gasoline_debug_filtering') === 'true'

if (DEBUG_TAB_FILTERING) {
  console.log('[Gasoline Debug] Message from untracked tab blocked:', {
    type: event.data.type,
    currentTabId,
    trackedTabId: storage.trackedTabId,
    isTracked: isTrackedTab,
  })
}
```

**Enable in console:**
```javascript
localStorage.setItem('gasoline_debug_filtering', 'true')
// Reload page
```

---

## Security Review

### Threat Model

**Threat 1: Data Leakage from Untracked Tabs**
- **Before:** All tabs send data regardless of tracking (CRITICAL)
- **After:** Only tracked tab sends data (FIXED)

**Threat 2: Malicious Page Spoofing Tracking State**
- **Attack:** Page tries to fake `isTrackedTab = true`
- **Mitigation:** Content script scope isolated from page context
- **Status:** Not vulnerable (content script runs in separate context)

**Threat 3: Race Condition on Tracking Toggle**
- **Attack:** Toggle tracking rapidly to bypass filter
- **Mitigation:** Debounce button, serialize state changes
- **Status:** Low risk, mitigated by debounce

**Threat 4: Tab ID Spoofing**
- **Attack:** Content script tries to fake tab ID
- **Mitigation:** Tab ID from Chrome API (trusted source)
- **Status:** Not vulnerable (Chrome APIs are trusted)

### Permissions Required

**No new permissions needed:**
- `tabs` - Already required for query operations
- `storage` - Already required for settings

---

## Open Questions / Future Work

1. **Visual indicator on tracked tab?**
   - Extension badge icon showing "📍" when tab is tracked
   - Implementation: `chrome.action.setBadgeText()`
   - Effort: 30 minutes

2. **Track multiple tabs?**
   - Allow tracking 2-3 tabs simultaneously
   - Effort: 4 hours
   - Risk: Increases complexity, privacy concerns

3. **Auto-pause tracking when tab hidden?**
   - Pause capture when tab in background
   - Resume when tab becomes visible
   - Effort: 2 hours
   - Benefit: Reduces noise from background tabs

4. **Tracking history/logs?**
   - Show which tabs were tracked when
   - Useful for debugging multi-session issues
   - Effort: 4 hours

5. **Per-domain tracking rules?**
   - "Always track example.com"
   - "Never track *.bank.com"
   - Effort: 8 hours

---

## Approval

**Status:** DRAFT - Awaiting Review

**Reviewers:**
- [ ] Technical Lead
- [ ] Security Review
- [ ] Product Owner

**Changes Requested:**

**Approved By:**

**Approval Date:**

---

## Change Log

| Date | Version | Changes |
|------|---------|---------|
| 2026-01-27 | 1.0 | Initial technical specification |
