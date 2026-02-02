# UAT: Track This Tab Feature

**Date:** 2026-01-28
**Feature:** Single-tab tracking isolation
**Build:** Next commit (Track This Tab implementation)
**Tester:** _____________

---

## Pre-UAT Checklist

**Quality Gates:**
- [x] Unit tests pass: 40/40 content-tab-filtering tests
- [x] Go tests pass: `make test`
- [x] Go vet clean: `go vet ./cmd/dev-console/`
- [x] Critical bug fixed: GET_TAB_ID message passing implemented

**Environment:**
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome browser with extension installed
- [ ] Extension rebuilt: `make dev` (if needed)
- [ ] Extension loaded in chrome://extensions
- [ ] Extension shows "Connected" in popup

---

## Test 1: Single Tab Isolation ⭐ CRITICAL

**Purpose:** Verify untracked tabs do NOT send data (security fix)

**Steps:**
1. Open 3 tabs:
   - Tab A: `https://example.com`
   - Tab B: `https://httpbin.org/get`
   - Tab C: `https://jsonplaceholder.typicode.com/posts`

2. Track ONLY Tab A (example.com):
   - Click extension icon
   - Click "Track This Tab"
   - Verify button turns red, says "Stop Tracking"

3. Generate activity in ALL tabs:
   - Tab A: Refresh page (F5)
   - Tab B: Refresh page (F5)
   - Tab C: Refresh page (F5)
   - Wait 5 seconds for data capture

4. Query network data:
   ```
   observe({what: "network_waterfall", limit: 20})
   ```

**Expected Results:**
- ✅ ONLY example.com URLs in results
- ✅ NO httpbin.org URLs
- ✅ NO jsonplaceholder.typicode.com URLs
- ✅ All entries have `pageURL` containing "example.com"

**Pass Criteria:**
```javascript
// All network entries must be from Tab A (example.com)
result.entries.every(entry => entry.pageURL.includes('example.com'))
```

**Actual Results:**
```
Count: _____
Tab A data: YES / NO
Tab B data: YES / NO  ← MUST BE NO
Tab C data: YES / NO  ← MUST BE NO
```

**Status:** ⬜ PASS / ⬜ FAIL

**Notes:**
```


```

---

## Test 2: Tab ID Attached to Messages ⭐ CRITICAL

**Purpose:** Verify all messages include `tabId` field

**Steps:**
1. Track Tab A (example.com)
2. Refresh Tab A to generate network activity
3. Query network bodies:
   ```
   observe({what: "network_bodies", limit: 5})
   ```

**Expected Results:**
- ✅ Each request-response pair has `tabId` field
- ✅ `tabId` is a number (not null/undefined)
- ✅ All `tabId` values are the same (same tab)

**Actual Results:**
```
Sample entry:
{
  "url": "_______________",
  "tabId": _______________,  ← CHECK THIS
  ...
}

All tabId values same: YES / NO
tabId is numeric: YES / NO
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 3: Tracking Switch Mid-Session

**Purpose:** Verify switching tracking moves data capture to new tab

**Steps:**
1. Track Tab A (example.com)
2. Generate activity in Tab A (refresh)
3. Query data:
   ```
   observe({what: "network_waterfall", limit: 10})
   ```
   - Note: Should see example.com data

4. Switch tracking to Tab B (httpbin.org):
   - Navigate to Tab B
   - Click extension icon
   - Click "Stop Tracking" (clears Tab A)
   - Navigate back to Tab B
   - Click "Track This Tab"

5. Generate activity in BOTH tabs:
   - Tab A: Refresh
   - Tab B: Refresh
   - Wait 5 seconds

6. Query data again:
   ```
   observe({what: "network_waterfall", limit: 10})
   ```

**Expected Results:**
- ✅ Results include httpbin.org (Tab B)
- ✅ Results may include old example.com data (buffered)
- ✅ New data is ONLY from httpbin.org
- ⚠️ Tab A refresh should NOT add new data

**Actual Results:**
```
After switch:
- httpbin.org data: YES / NO
- New example.com data: YES / NO  ← Should be NO
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 4: Tracked Tab Closed

**Purpose:** Verify tracking disabled when tracked tab closes

**Steps:**
1. Track Tab A (example.com)
2. Verify button shows "Stop Tracking" (red)
3. Close Tab A (Cmd+W or click X)
4. Open extension popup
5. Observe button state

**Expected Results:**
- ✅ Button shows "Track This Tab" (gray, not red)
- ✅ Tooltip: "Start capturing telemetry from this browser tab only"
- ✅ No tracked URL displayed
- ✅ Shows: "⚠️ No tab tracked - data capture disabled"

**Actual Results:**
```
Button text: "_______________"
Button color: RED / GRAY
Tracked URL shown: YES / NO
Warning shown: YES / NO
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 5: Chrome Internal Pages Blocked

**Purpose:** Verify cannot track chrome:// pages

**Steps:**
1. Navigate to `chrome://extensions`
2. Open extension popup
3. Observe "Track This Tab" button state

**Expected Results:**
- ✅ Button is disabled (grayed out)
- ✅ Button text: "Cannot Track Internal Pages"
- ✅ Tooltip explains: "Chrome blocks extensions on internal pages like chrome:// and about:"
- ✅ Cannot click button

**Actual Results:**
```
Button enabled: YES / NO  ← Should be NO
Button text: "_______________"
Tooltip: "_______________"
```

**Repeat for:**
- [ ] `chrome://settings`
- [ ] `about:blank`
- [ ] `chrome-extension://...` (extension's own pages)

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 6: "No Tracking" Mode - Server Status

**Purpose:** Verify server receives tracking status updates

**Steps:**
1. Ensure NO tab is tracked (click "Stop Tracking" if needed)
2. Wait 30 seconds (status ping interval)
3. Check server logs for status ping

**Expected Results:**
- ✅ Server receives status ping
- ✅ `tracking_enabled: false`
- ✅ `tracked_tab_id: null`
- ✅ Message: "no tab tracking enabled"

**Actual Results:**
```
Server received ping: YES / NO
tracking_enabled: _____
tracked_tab_id: _____
message: "_____"
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 7: "No Tracking" Mode - LLM Error

**Purpose:** Verify LLM gets clear error when tracking disabled

**Steps:**
1. Ensure NO tab is tracked
2. Try to observe:
   ```
   observe({what: "errors"})
   ```

**Expected Results:**
- ✅ Response includes warning/error
- ✅ Message explains tracking disabled
- ✅ Suggests enabling tracking
- ⚠️ OR returns empty data with warning prepended

**Actual Results:**
```
Error received: YES / NO
Error type: "_______________"
Message: "_______________"
Suggestion included: YES / NO
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 8: Browser Restart Clears Tracking

**Purpose:** Verify tracking cleared on browser restart

**Steps:**
1. Track a tab (example.com)
2. Verify button shows "Stop Tracking" (red)
3. Close browser completely (Cmd+Q / Quit)
4. Reopen browser
5. Open extension popup

**Expected Results:**
- ✅ Button shows "Track This Tab" (gray, not red)
- ✅ No tab tracked
- ✅ Warning: "⚠️ No tab tracked - data capture disabled"

**Actual Results:**
```
After restart:
Button text: "_______________"
Button color: RED / GRAY  ← Should be GRAY
Tracking state: CLEARED / PERSISTED  ← Should be CLEARED
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 9: Multiple Windows

**Purpose:** Verify tracking works across windows

**Steps:**
1. Open Window 1 with Tab A (example.com)
2. Open Window 2 with Tab B (httpbin.org)
3. In Window 1, track Tab A
4. In Window 2, open extension popup
5. Observe button state

**Expected Results:**
- ✅ Window 2 popup shows "Stop Tracking" (tracking is global)
- ✅ Tracked URL shows example.com (from Window 1)
- ✅ Indicates tracking is in different window (nice to have)

**Actual Results:**
```
Window 2 popup shows:
Button: "_______________"
Tracked URL: "_______________"
Window indication: YES / NO
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 10: LLM Navigation Within Tracked Tab

**Purpose:** Verify LLM can navigate to multiple sites in tracked tab

**Steps:**
1. Track Tab A (example.com)
2. Navigate within same tab:
   ```
   interact({action: "navigate", url: "https://httpbin.org/get"})
   ```
3. Wait for navigation to complete
4. Query page info:
   ```
   observe({what: "page"})
   ```

**Expected Results:**
- ✅ URL is now httpbin.org
- ✅ Tab is still tracked (same tab ID)
- ✅ Data capture continues
- ✅ Extension popup still shows "Stop Tracking"

**Actual Results:**
```
Navigation succeeded: YES / NO
New URL: "_______________"
Still tracked: YES / NO
Data captured from httpbin: YES / NO
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 11: Tab ID Consistency

**Purpose:** Verify tabId remains consistent across operations

**Steps:**
1. Track Tab A
2. Generate various types of activity:
   - Console log: `console.log("test")`
   - Network request: Refresh page
   - Click action: Click any link

3. Query different data types:
   ```
   observe({what: "logs", limit: 5})
   observe({what: "network_waterfall", limit: 5})
   observe({what: "actions", limit: 5})
   ```

**Expected Results:**
- ✅ All logs have same `tabId`
- ✅ All network entries have same `tabId`
- ✅ All actions have same `tabId`
- ✅ `tabId` matches the actual Chrome tab ID

**Actual Results:**
```
Logs tabId: _____
Network tabId: _____
Actions tabId: _____
All same: YES / NO
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Test 12: Extension Reload Persistence

**Purpose:** Verify tracking persists across extension reload

**Steps:**
1. Track Tab A (example.com)
2. Reload extension:
   - Go to chrome://extensions
   - Click "Reload" button for Gasoline
3. Wait 5 seconds for re-initialization
4. Open extension popup
5. Observe button state

**Expected Results:**
- ✅ Button shows "Stop Tracking" (red)
- ✅ Tracked URL still shows example.com
- ✅ Tracking state persisted
- ✅ Data capture resumes after reload

**Actual Results:**
```
After reload:
Button: "_______________"
Tracked URL: "_______________"
Tracking persisted: YES / NO
Data capture working: YES / NO
```

**Status:** ⬜ PASS / ⬜ FAIL

---

## Summary

| Test | Description | Status | Notes |
|------|-------------|--------|-------|
| 1 | Single tab isolation | ⬜ | CRITICAL - Security fix |
| 2 | Tab ID attached | ⬜ | CRITICAL - Data attribution |
| 3 | Tracking switch | ⬜ | Important |
| 4 | Tracked tab closed | ⬜ | Important |
| 5 | Chrome:// blocked | ⬜ | Important |
| 6 | No tracking - server | ⬜ | Important |
| 7 | No tracking - LLM error | ⬜ | Important |
| 8 | Browser restart | ⬜ | Important |
| 9 | Multiple windows | ⬜ | Nice to have |
| 10 | LLM navigation | ⬜ | Nice to have |
| 11 | Tab ID consistency | ⬜ | Nice to have |
| 12 | Extension reload | ⬜ | Nice to have |

**Pass Rate:** _____ / 12 tests

**Critical Tests (Must Pass):**
- Test 1: Single tab isolation
- Test 2: Tab ID attached

**Release Criteria:**
- ✅ All critical tests pass
- ✅ 10/12 total tests pass (83%)
- ✅ No critical or high-severity bugs found

---

## Issues Found

| # | Test | Severity | Description | Fix Required |
|---|------|----------|-------------|--------------|
| | | | | |

---

## Sign-Off

**Tester:** _____________
**Date:** _____________
**Result:** ⬜ PASS / ⬜ FAIL
**Ready for Commit:** ⬜ YES / ⬜ NO

**Notes:**
```


```
