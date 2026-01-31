# Track This Tab - Product Specification

**Version:** 1.0
**Date:** 2026-01-27
**Status:** DRAFT - Awaiting Review
**Priority:** CRITICAL (Security/Privacy Issue)

---

## Executive Summary

Gasoline currently captures telemetry from ALL browser tabs regardless of user intent, creating a critical privacy and security issue. This specification defines the correct behavior: the extension should only capture data from a single "attached" tab, and only when the user has explicitly enabled tracking.

**Key Changes:**
- Rename button: "Track This Page" → "Track This Tab"
- Implement tab-scoped data capture filtering
- Add "No Tracking" mode with clear LLM communication
- Provide explicit user control over what gets captured

---

## Problem Statement

### Current Behavior (INCORRECT)
- User clicks "Track This Page" button
- Extension stores `trackedTabId` in chrome.storage.local
- **BUT**: Extension captures data from ALL tabs (untracked tabs included)
- User believes only tracked tab is being monitored
- **Security Risk**: Banking, email, private tabs all send data to extension

### User Impact
```
User Scenario:
1. Opens banking site in Tab 1
2. Opens their app in Tab 2
3. Clicks "Track This Page" in Tab 2
4. Believes only Tab 2 is tracked

Reality:
- Tab 1 (banking) STILL captures:
  - Login credentials in network bodies
  - Account numbers in API responses
  - Session tokens in headers
  - All API calls
- All data goes to background script → server
```

This is a **critical privacy violation**.

---

## Solution: Single Tab Attachment Model

### Core Principle
> The extension is "attached" to exactly ONE tab at a time. Data capture occurs ONLY from the attached tab, ONLY when tracking is enabled.

### User Model
1. **By default**: No tab is attached → No data captured
2. **User action**: Click "Track This Tab" → Attach to current tab → Capture begins
3. **LLM navigation**: LLM navigates within attached tab → Continues capturing
4. **Multi-tab isolation**: Other tabs (even if open) → Never captured
5. **Stop tracking**: Click "Stop Tracking" → Detach from tab → Capture stops

---

## Feature Specification

### 1. Button Behavior

#### Initial State (No Tab Tracked)
```
Button Text: "Track This Tab"
Button Color: Dark gray (#252525)
Tooltip: "Start capturing telemetry from this browser tab only"
Popup Message: "⚠️ No tab tracked - data capture disabled"
```

#### Active State (Tab Tracked)
```
Button Text: "Stop Tracking"
Button Color: Red (#f85149)
Display: "📍 Tracking: https://example.com"
Tooltip: "Stop capturing telemetry from this tab"
```

#### User Actions
- **Click when not tracking**: Attach to current tab, enable capture
- **Click when tracking**: Detach from tab, disable capture
- **Switch tabs**: Button updates to show if current tab is the tracked one

### 2. Data Capture Rules

#### What Gets Captured (Only from Attached Tab)
When tracking is enabled for Tab X:
- ✅ Console logs (errors, warnings, info)
- ✅ Network requests (URLs, methods, status codes)
- ✅ Network bodies (request/response data)
- ✅ WebSocket messages
- ✅ User actions (clicks, inputs, navigation)
- ✅ Performance metrics (LCP, CLS, INP, etc.)
- ✅ DOM queries (when LLM requests them)
- ✅ Page errors and exceptions

#### What Does NOT Get Captured
- ❌ Data from Tab Y (any other tab)
- ❌ Data when tracking disabled (button not clicked)
- ❌ Data before tracking enabled
- ❌ Data after tracking disabled

### 3. LLM Integration

#### "No Tracking" Mode

**Server Behavior:**
- Extension pings server every 30s with status message:
  ```json
  {
    "type": "status",
    "tracking_enabled": false,
    "message": "no tab tracking enabled",
    "extension_connected": true,
    "timestamp": "2026-01-27T19:00:00Z"
  }
  ```

**LLM Experience:**
When LLM calls any `observe()` or `interact()` tool:

```json
{
  "error": "tracking_disabled",
  "message": "Extension is connected but tab tracking is disabled",
  "suggestion": "Ask user to click 'Track This Tab' button in the Gasoline extension popup",
  "extension_status": "connected",
  "last_ping": "2026-01-27T19:00:00Z"
}
```

**LLM Communication Pattern:**
```
LLM: Let me observe the page errors
[calls observe({what: "errors"})]

Response: {
  "error": "tracking_disabled",
  "message": "Extension connected but tracking disabled. Ask user to enable."
}

LLM to User: "I can see the Gasoline extension is connected, but tab tracking
is currently disabled. To help debug this issue, please click the 'Track This
Tab' button in the Gasoline extension popup (top-right of your browser)."
```

#### Active Tracking Mode

**Server Behavior:**
- Extension pings server every 30s:
  ```json
  {
    "type": "status",
    "tracking_enabled": true,
    "tracked_tab_id": 123,
    "tracked_tab_url": "https://example.com",
    "extension_connected": true,
    "timestamp": "2026-01-27T19:00:00Z"
  }
  ```

**LLM Experience:**
All `observe()` and `interact()` calls work normally, operating on the tracked tab.

### 4. LLM Navigation Within Tab

**Use Case:**
LLM is helping debug an issue that requires visiting multiple pages:

```
User: "Debug why login redirects to wrong page"

LLM: "I'll track the login flow across multiple pages"
1. observe({what: "page"})  → https://app.example.com/
2. interact({action: "navigate", url: "https://app.example.com/login"})
3. observe({what: "network_bodies"})
4. interact({action: "execute_js", script: "document.querySelector('form').submit()"})
5. observe({what: "page"})  → https://app.example.com/dashboard (after redirect)
```

**Behavior:**
- All navigation happens in THE SAME attached tab
- All sites visited within that tab are captured
- LLM has full control of attached tab
- Other tabs remain completely isolated

---

## Edge Cases & Handling

### Edge Case 1: Tracked Tab Closed

**Scenario:**
```
1. User tracks Tab A (id: 123)
2. User closes Tab A
3. LLM tries to observe()
```

**Handling:**
- Background script detects tab no longer exists
- Clears `trackedTabId` from storage
- Returns to "No Tracking" mode
- Next LLM operation gets "tracking_disabled" error

**Alternative (Future):**
Offer to auto-switch to active tab:
```json
{
  "error": "tracked_tab_closed",
  "message": "The tracked tab was closed",
  "suggestion": "Would you like to track the current active tab instead?",
  "current_active_tab": {
    "id": 456,
    "url": "https://other-site.com"
  }
}
```

### Edge Case 2: Tracking Switched Mid-Session

**Scenario:**
```
1. LLM tracking Tab A, gathering data
2. User manually switches tracking to Tab B
3. LLM continues operating
```

**Handling:**
- Immediate effect: All subsequent `observe()` calls target Tab B
- Background script clears old tab's buffer data
- LLM receives data from NEW tracked tab
- No mixing of data from multiple tabs

**UX:**
- Show warning in popup: "⚠️ Switching tabs will clear buffered data"
- Require confirmation before switch

### Edge Case 3: Multiple Windows

**Scenario:**
```
1. User has 3 Chrome windows open
2. Tracks tab in Window 1
3. Opens extension popup in Window 2
```

**Handling:**
- Popup shows: "📍 Tracking: https://example.com (Window 1, Tab 3)"
- Button says "Stop Tracking" (since tracking is active globally)
- If user clicks "Track This Tab" in Window 2:
  - Clears previous tracking
  - Switches to tab in Window 2
  - Shows confirmation: "Switched tracking to this tab"

### Edge Case 4: Extension Reload/Update

**Scenario:**
```
1. User tracking Tab A
2. Extension reloads (dev mode) or auto-updates
3. LLM tries to operate
```

**Handling:**
- `trackedTabId` persists in chrome.storage.local (survives reload)
- Extension re-initializes with same tracked tab
- If tab still exists: Resume tracking seamlessly
- If tab closed during reload: Fall back to "No Tracking" mode

### Edge Case 5: Storage Cleared

**Scenario:**
```
1. User manually clears extension storage
2. Or browser clears storage due to space
```

**Handling:**
- Extension detects no `trackedTabId`
- Enters "No Tracking" mode
- LLM gets "tracking_disabled" on next operation
- No crash, graceful degradation

### Edge Case 6: Tab Crashes or Becomes Unresponsive

**Scenario:**
```
1. User tracking Tab A
2. Tab A crashes or becomes unresponsive
3. LLM tries interact({action: "navigate", ...})
```

**Handling:**
- Chrome APIs return error when messaging crashed tab
- Background script catches error
- Returns to LLM:
  ```json
  {
    "error": "tab_unresponsive",
    "message": "The tracked tab is not responding",
    "suggestion": "The tab may have crashed. Try refreshing it or track a different tab.",
    "tracked_tab_id": 123
  }
  ```
- **Do not** auto-clear tracking (user may refresh tab)

### Edge Case 7: User Has No Tabs Open

**Scenario:**
```
1. User closes all tabs (only extension popup open)
2. User clicks "Track This Tab"
```

**Handling:**
- Button disabled/grayed out if no tabs available
- Tooltip: "No tabs available to track"
- Or: Prompt user to open a tab first

### Edge Case 8: Tracking About/Chrome Pages

**Scenario:**
```
1. User tries to track chrome://extensions or about:blank
```

**Handling:**
- Show error: "Cannot track internal Chrome pages"
- Tooltip on button: "Disabled for internal pages"
- Suggest: "Navigate to a regular webpage to enable tracking"

**Blocked URLs:**
- `chrome://`
- `chrome-extension://`
- `about:`
- `edge://`
- `brave://`
- Local file:// (optional: allow for testing)

### Edge Case 9: Rapid Tracking Toggle

**Scenario:**
```
1. User rapidly clicks Track/Stop/Track/Stop
2. Messages in flight, race conditions
```

**Handling:**
- Debounce button clicks (500ms)
- Show loading state: "Enabling tracking..."
- Serialize tracking state changes
- Last click wins

### Edge Case 10: Tab Navigation During LLM Operation

**Scenario:**
```
1. LLM sends navigate command
2. User manually navigates in same tab before command completes
3. Race condition
```

**Handling:**
- Track navigation ID per command
- If user navigation interrupts:
  - Abort pending LLM navigation
  - Return error: "Navigation interrupted by user"
- LLM can detect and retry if needed

---

## Privacy & Security Guarantees

### What Users Can Trust

1. **Single Tab Isolation**
   - Only ONE tab is tracked at any time
   - Other tabs NEVER send data to extension
   - No background capture from untracked tabs

2. **Explicit Control**
   - User MUST click "Track This Tab" to enable
   - Tracking state clearly visible in popup
   - One-click disable at any time

3. **No Surprise Capture**
   - Extension starts in "No Tracking" mode
   - No capture without user action
   - Clear visual indicator when tracking active

4. **Sensitive Content Protection**
   - Banking sites: Safe if not the tracked tab
   - Email: Safe if not the tracked tab
   - Multiple accounts: Only tracked tab exposed

### User Communication

**Extension Description:**
> Captures browser telemetry (logs, network, DOM) from a single tracked tab for AI coding assistants. You control which tab is tracked. Other tabs are never monitored.

**First-Time Setup:**
```
Welcome to Gasoline!

This extension helps AI assistants debug your web applications by
capturing telemetry from ONE browser tab at a time.

🔒 Privacy: Only the tab you explicitly track sends data.
✋ Control: Click "Track This Tab" to enable, "Stop Tracking" to disable.
🌐 Other tabs: Never monitored, completely isolated.

Ready to start? Open your app and click "Track This Tab"!
```

---

## Success Criteria

### Must Have (MVP)
- ✅ Single tab tracking (only attached tab captures)
- ✅ Button renamed to "Track This Tab"
- ✅ "No Tracking" mode with clear LLM messaging
- ✅ Server status ping includes tracking state
- ✅ Handle tracked tab closed gracefully
- ✅ Block tracking of chrome:// pages

### Should Have (V1.1)
- ✅ Tracking switch confirmation dialog
- ✅ Visual indicator on tracked tab (badge icon)
- ✅ Tracked tab closes → suggest auto-switch to active tab
- ✅ Better error messages for all edge cases

### Nice to Have (Future)
- Multi-tab allowlist mode (advanced users)
- "Track This Domain" mode (all tabs on example.com)
- Tracking history/logs for auditing
- Per-domain privacy rules

---

## User Workflows

### Workflow 1: Debug Single Page App

```
1. User opens their app: https://app.example.com
2. User opens Gasoline extension popup
3. User clicks "Track This Tab"
   → Button turns red, shows "📍 Tracking: https://app.example.com"
4. User asks Claude: "Debug the login error"
5. Claude calls observe({what: "errors"})
   → Gets errors from app.example.com only
6. Claude calls observe({what: "network_bodies"})
   → Gets API calls from app.example.com only
7. Claude navigates within tab to test different pages
   → All navigation in same tracked tab, continuously captured
8. When done, user clicks "Stop Tracking"
   → Capture stops, button returns to gray
```

### Workflow 2: Multi-Site User Flow

```
1. User wants to debug checkout flow spanning multiple domains:
   - https://shop.example.com (main site)
   - https://payment.stripe.com (payment)
   - https://shop.example.com/confirmation (return)

2. User tracks shop.example.com tab
3. Claude navigates through checkout
4. When redirected to payment.stripe.com:
   → SAME tab, still tracked, captures Stripe page
5. When redirected back to shop.example.com/confirmation:
   → SAME tab, still tracked, captures confirmation
6. Claude analyzes full flow from single tab
```

### Workflow 3: No Tracking (User Education)

```
1. User installs extension, doesn't enable tracking
2. User asks Claude: "What errors are on the page?"
3. Claude calls observe({what: "errors"})
   → Gets error: "tracking_disabled"
4. Claude responds:
   "I can see the Gasoline extension is connected, but you haven't
    enabled tab tracking yet. To help debug, please click the
    'Track This Tab' button in the extension popup."
5. User clicks button
6. Claude retries observe()
   → Now gets error data
```

---

## Out of Scope (Explicitly NOT Included)

### Multi-Tab Simultaneous Tracking
**Why:** Too complex for MVP, increases privacy risk
**Future:** May add as advanced mode with warnings

### Automatic Tab Switching
**Why:** Surprising behavior, user should control
**Future:** May add "suggest switch" prompts

### Cross-Tab Correlation
**Why:** Privacy concern, defeats single-tab model
**Future:** User could manually track tabs in sequence

### Background Tabs Capture
**Why:** Core privacy violation we're fixing
**Future:** Never - this is the bug we're solving

---

## Testing Requirements

### Unit Tests
- Tab ID filtering logic
- Storage management (set/clear tracking)
- Message filtering in content.js

### Integration Tests
- Multi-tab scenario: Track Tab A, verify Tab B not captured
- Tab closed scenario: Clear tracking, return "No Tracking" mode
- Navigation scenario: LLM navigates within tracked tab

### Manual UAT
See [UAT-TEST-PLAN.md](./UAT-TEST-PLAN.md) for detailed checklist:
- Test 1: Single tab isolation (3 tabs, track 1, verify others silent)
- Test 2: Tracking switch (track A → track B, verify data switches)
- Test 3: No tracking mode (verify LLM gets clear error)
- Test 4: Tracked tab closes (verify graceful fallback)
- Test 5: LLM navigation within tab (multi-site flow)

---

## Documentation Updates Required

### User-Facing
- Extension store description (emphasize privacy)
- README.md (tracking model explanation)
- First-time setup guide

### Developer-Facing
- Architecture docs (message filtering)
- MCP tool docs (tracking state in responses)
- Testing guide (multi-tab scenarios)

---

## Open Questions

1. **Auto-switch on tab close?**
   - Option A: Return to "No Tracking" mode (safer, explicit)
   - Option B: Suggest switching to active tab (convenient, but surprising?)
   - **Recommendation:** Option A for MVP, add Option B later with prompt

2. **Visual indicator on tracked tab?**
   - Extension badge icon showing "📍" when tab is tracked?
   - **Recommendation:** Yes, add in V1.1

3. **Tracking persists across browser restart?**
   - Should `trackedTabId` survive browser close?
   - **Recommendation:** Clear on browser restart (tab IDs invalidated anyway)

4. **Allow tracking file:// URLs?**
   - Useful for local development
   - But requires special permission
   - **Recommendation:** Support if `file://` permission granted

5. **Warn when switching tabs?**
   - "This will clear buffered data and switch tracking. Continue?"
   - **Recommendation:** Yes, show confirmation dialog

---

## Rollout Plan

### Phase 1: Critical Fix (This Release)
- Implement single-tab filtering
- Rename button
- Add "No Tracking" mode
- Fix critical security issue

### Phase 2: UX Improvements (Next Release)
- Tab switch confirmation
- Better error messages
- Visual indicator on tracked tab
- First-time setup guide

### Phase 3: Advanced Features (Future)
- Multi-tab allowlist mode
- Tracking history
- Per-domain rules
- Performance optimizations

---

## Success Metrics

### Privacy (Primary Goal)
- Zero data captured from untracked tabs
- User controls explicitly what is tracked
- No surprise background capture

### Usability
- LLM can debug multi-site flows within single tab
- Clear communication when tracking disabled
- Minimal user friction (one-click enable/disable)

### Technical
- No performance degradation
- Message filtering < 0.1ms overhead
- Graceful handling of all edge cases

---

## Approval

**Status:** DRAFT - Awaiting Review

**Reviewers:**
- [ ] Product Owner (User)
- [ ] Technical Lead
- [ ] Security Review

**Changes Requested:**

**Approved By:**

**Approval Date:**

---

## Change Log

| Date | Version | Changes |
|------|---------|---------|
| 2026-01-27 | 1.0 | Initial draft specification |
