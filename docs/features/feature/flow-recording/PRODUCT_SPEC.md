---
feature: Flow Recording & Playback
status: proposed
tool: interact, observe, configure
mode: recording, playback, test-generation
version: v6.0
---

# Product Spec: Flow Recording & Playback (Regression Testing)

## Problem Statement

**When bugs are discovered in production, developers spend hours reproducing them, analyzing root causes, and verifying fixes — manually.**

Regression testing today is **slow, manual, and error-prone**:

1. **Reproducibility Gap:** "Works on my machine" — reproducing user-reported bugs requires exact environment replication
2. **Root Cause Blindness:** Logs are opaque; developers manually trace through to understand what broke
3. **Fix Verification:** After coding a fix, QA manually re-tests the entire flow (15-30 min per bug)
4. **Regression Risk:** No automated way to re-test all historical flows; some regressions re-appear multiple times
5. **Incident Response:** On-call engineers waste time on reproduction; they need answers in minutes, not hours

**Result:** Critical bugs take 2-4 hours to fix + verify; non-critical regressions accumulate; team loses confidence in releases.

---

## Solution

**Gasoline Flow Recording & Playback** is the **AI-powered regression testing tool for developers**.

**The workflow:**
1. **QA records** a user's reported flow once (e.g., "checkout fails on coupon code entry")
2. **Developer fixes** the bug in code
3. **LLM invokes Gasoline** to replay the flow and analyze what changed
4. **Gasoline suggests** the root cause + code fixes (with file/line numbers and git context)
5. **Developer verifies** the fix in <5 minutes (not 30 minutes of manual testing)

**Why Gasoline:**
- **Purpose-built for regression testing**, not general test automation
- **Root cause analysis + fix suggestions** (unique) — tells you WHY it broke, not just that it broke
- **AI-driven** — uses Claude to understand logs, suggest fixes, and rank confidence
- **Local-first** — runs entirely on your machine (no cloud, no shared state)
- **Zero dependencies** — lean, fast, audit-friendly

---

## User Workflows

### Workflow 1: Record a Flow (QA Engineer)

```
1. QA clicks "Start Recording" in extension (or calls configure() via MCP)
2. Browser navigates to target URL
3. QA performs flow: login → add to cart → checkout → submit
4. Extension records:
   - URL changes (timestamps)
   - Clicks (x/y, selectors)
   - Typing (field selector, text, x/y)
   - Screenshots at key points
5. QA clicks "Stop Recording"
→ Recording saved as "shopping-checkout-{timestamp}"
→ Recording is now queryable via MCP
```

### Workflow 2: Replay & Verify Fix (LLM + CI)

```
1. Bug found: "Checkout button missing error message"
2. Dev fixes bug in code
3. LLM says: "replay shopping-checkout recording"
4. Gasoline:
   - Navigates to original URL
   - Replays all clicks/typing in sequence
   - Captures new logs alongside original
5. LLM compares original logs vs new logs:
   - Error message missing in original ✗
   - Error message present in new ✓
   - Regression fixed ✓
6. Test passes, CI proceeds
```

### Workflow 3: Generate Variations (LLM Synthesized)

```
1. LLM reads recorded action sequence:
   [navigate → click product → type qty:1 → checkout]
2. LLM generates variations:
   - qty: 1 → 5
   - product: item-1 → item-3
   - Variation: Add coupon code
3. Each variation replayed, logs compared
4. All variations pass → broader regression coverage
```

### Workflow 4: Root Cause Analysis & Auto-Fix (LLM + Git)

```
1. Playback detects regression (new logs differ from original)
2. LLM invokes /gasoline-fix skill:
   "Analyze logs and suggest fixes for checkout-flow regression"
3. Gasoline:
   - Diffs original vs new logs
   - Identifies error types (404, timeout, DOM change, etc.)
   - Suggests root cause ("Missing field 'cvv' in form")
   - If git available, finds commits that changed related code
   - Suggests code fixes (e.g., "Add cvv field validation")
4. LLM reviews suggestions and applies fixes to codebase
5. Re-runs playback to verify fix works
6. Skill reports: "✓ Fix verified, regression resolved"
```

---

## Core Requirements

### R1: Flow Recording

**What Gets Recorded:**
- [ ] **URL changes** (timestamp, new URL)
- [ ] **Clicks** (x/y coordinates, element selector, timestamp)
- [ ] **Typing** (field selector, text typed, x/y, timestamp)
- [ ] **Screenshots** (at key points: page load, after click, after type)
- [ ] **Page metadata** (viewport width/height, timestamp)

**Storage:**
- [ ] Recordings stored as **JSON action sequence** (for LLM consumption)
- [ ] Screenshots stored as **JPEG/PNG files on disk**
- [ ] Metadata: name, creation time, duration, action count
- [ ] Queryable via MCP: `observe({what: 'recordings'})`

**Naming:**
- [ ] User-provided name (e.g., "shopping-checkout")
- [ ] OR auto-generated: "{adjective}-{noun}-{adjective}-{ISO8601}" (e.g., "magic-badger-hammer-20260130T143022Z")
- [ ] Name + timestamp used in file paths/IDs

**Recording Policy:**
- [ ] Full text typed captured (necessary for regression testing with real data)
- [ ] ⚠️ **Security Warning:** Never record flows with real production credentials (passwords, API keys, credit cards)
- [ ] Recommended: Use test accounts and fake data for all recordings
- [ ] Recordings stored locally only; not transmitted to cloud

### R2: Recording UI

**Extension Popup:**
- [ ] Button: "Start Recording" (opens dialog for name + URL)
- [ ] Dialog: name field (optional, auto-generates if empty), URL field (optional)
- [ ] Button: "Stop Recording" (visible after start)
- [ ] List: recent recordings with timestamp, action count

**MCP Actions:**
- [ ] `configure({action: 'recording_start', name?: 'shopping-cart', url?: 'https://...'})`
- [ ] `configure({action: 'recording_stop', recording_id: 'shopping-cart-20260130T...'})` (auto-generates ID if not provided)
- [ ] Browser auto-navigates to URL if provided on `recording_start`

### R3: Screenshot Management

**Format & Compression:**
- [ ] Format: JPEG (85% compression)
- [ ] Max file size: 500KB per screenshot
- [ ] Storage: Disk (local file system)
- [ ] Naming: `{date}-{recording_name}-{action_index}-{issue_type}.jpg`
  - Example: `20260130-shopping-checkout-003-page-load.jpg`
  - Issue types: `page-load` (after navigation), `moved-selector` (element not found), `error` (assertion/timeout/network)

**When Captured:**
- [ ] On page load (after navigation)
- [ ] When element selector fails or moves
- [ ] On error/timeout
- [ ] Sampling: every N actions (configurable, default 5)

### R4: Element Matching & Self-Healing (Robust Selector Recovery)

**For Playback, Match Elements Using (Priority Order):**
1. **data-testid** attribute (most reliable for dynamic content)
2. **x/y coordinates + context** (if selector fails, search nearby elements)
3. **Visual recovery** (if above fails, use OCR on screenshots to find element by visible text)

**Self-Healing on Selector Failure:**
- [ ] Primary: Try exact data-testid match
- [ ] Secondary: Try recorded CSS selector
- [ ] Tertiary: Check if element moved (nearby search based on old x/y)
- [ ] Quaternary: Use OCR on screenshot to find by visible text
- [ ] Final: Use last-known x/y coordinates with warning

**If Element is Fragile (Moved Multiple Times):**
- [ ] Log warning: "Fragile selector: element moved 3 times across test runs"
- [ ] Screenshot with issue type: `moved-selector`
- [ ] Recommend using `data-testid` instead of selectors
- [ ] Suggest code change: "Add data-testid=product-card-1 to improve test stability"
- [ ] Report to LLM for debugging

**Competitive Advantage:**
- Gasoline has access to real browser context (logs, network, visual state)
- Can detect selector fragility and suggest fixes proactively
- Unlike cloud-based tools, we see the actual user environment

---

### R5: Playback & Sequence Execution

**Execution:**
- [ ] **Sequence mode**: Execute actions in order, ignoring timing (fast-forward)
  - Navigate → click → type → navigate → click
  - Useful for regression testing (speed is priority)
  - Target: 10+ actions/second

**On Page Load During Playback:**
- [ ] Wait for page to load (network idle or timeout: 5 sec)
- [ ] If selector not found, attempt self-healing (R4)
- [ ] If self-healing fails, log error and take screenshot
- [ ] Continue playback (non-blocking)

**Error Handling (Non-Blocking):**
- [ ] Selector not found (after self-healing) → Log + screenshot, continue
- [ ] Click outside viewport → Scroll to element, then click
- [ ] Type in non-input → Log error, continue
- [ ] Navigation timeout → Log, continue
- [ ] Network error → Log, continue

**Graceful Degradation:**
- Playback completes even if some actions fail (important for regression analysis)
- All failures logged and visible to LLM for debugging

---

### R6: MCP Integration

**Recording Management:**
- [ ] `observe({what: 'recordings'})` → List all recordings
  - Returns: `[{id, name, created_at, duration, action_count, url}]`
- [ ] `observe({what: 'recording_actions', recording_id: 'shopping-cart'})` → Action sequence
  - Returns: `[{action, selector, text, x, y, timestamp, screenshot_path}]`

**Playback:**
- [ ] `interact({action: 'playback', recording: 'shopping-cart-{id}', test_id: 'replay-shopping-cart'})`
  - Records playback logs under test boundary `replay-shopping-cart`
  - Returns: `{status, actions_executed, errors, duration}`

**Test Boundary Integration:**
- [ ] Playback runs under `test_boundary_id` so logs can be correlated
- [ ] LLM can query: `observe({what: 'logs', test_boundary: 'replay-shopping-cart'})`
- [ ] Compare to original logs: `observe({what: 'logs', test_boundary: 'original-shopping-cart'})`

---

### R7: Log Streaming (WebSocket Migration)

**Current Issue:** Polling introduces timing inaccuracy. Recording needs millisecond precision.

**Solution:** Migrate extension from polling to **WebSocket** connection.

**Behavior:**
- [ ] Extension connects to server via WebSocket on startup
- [ ] Logs/events streamed in real-time with timestamps
- [ ] Server buffers with timestamps (last 10 seconds)
- [ ] If buffer overflows, drop oldest, log warning icon in popup
- [ ] LLM receives logs with millisecond accuracy for action correlation

**Backward Compatibility:**
- [ ] Polling still works (fallback if WebSocket unavailable)
- [ ] Health check: if WebSocket stale > 3 sec, warn user

---

### R8: Test Generation (LLM Synthesis)

**LLM Capability:**
- [ ] Read recorded action sequence (JSON format)
- [ ] Understand action parameters (selector, text, coordinates)
- [ ] Generate variations:
  - Different input values (cart quantity: 1 → 3, 5, 10)
  - Different selectors (product A → B → C)
  - Different flows (skip coupon vs apply coupon)
  - Different user states (logged in vs guest)
- [ ] Generate new recordings as JSON (not stored, executed on-the-fly)

**Format:**
```json
[
  {"action": "navigate", "url": "https://example.com/shop"},
  {"action": "click", "selector": "[data-testid=product-qty-selector]"},
  {"action": "type", "selector": "input#quantity", "text": "3", "x": 523, "y": 456},
  {"action": "click", "selector": "[data-testid=add-to-cart-btn]", "x": 600, "y": 300},
  {"action": "navigate", "url": "https://example.com/checkout"}
]
```

**Execution:**
- [ ] LLM-generated variations executed same as recorded flows
- [ ] Logs captured, compared to original
- [ ] Results reported to LLM

---

### R9: Recording Storage & Management

**Storage:**
- [ ] **Max storage:** 1GB total on disk (warn at 80%, error at 100%)
- [ ] **Concurrent:** Only 1 active recording at a time
- [ ] Storage location: `~/.gasoline/recordings/` (configurable)

**Guidance (Not Hard Limits):**
- Typical flow: 5-30 minutes, 20-100 actions
- If recording approaches 1GB, user should manage manually:
  - Delete old recordings
  - Or expand storage
- Gasoline doesn't auto-delete recordings (data loss risk)

### R10: Root Cause Analysis & Auto-Fix Skill

**Issue Detection:**
- [ ] Compare original recording logs vs replay logs
- [ ] Identify error types: network error (404, 500, timeout), DOM changes (missing elements), timeout/delay, assertion failures
- [ ] Extract error context: error message, stack trace, affected URL, affected action
- [ ] Take screenshot when error detected (if not already taken)

**Root Cause Analysis:**
- [ ] Analyze error pattern: "404 on /api/checkout" → "API endpoint missing or renamed"
- [ ] Suggest likely causes: code change, missing migration, environment mismatch
- [ ] Report confidence level (high/medium/low) based on error clarity

**Fix Suggestions:**
- [ ] Propose code changes (e.g., "Add 'cvv' field to checkout form validation")
- [ ] List files likely to need changes (form component, validation logic, database schema)
- [ ] Do NOT auto-apply fixes; LLM reviews and approves

**Git Integration (Optional):**
- [ ] If git repo available, find commits that touched affected files
- [ ] Show commit messages and authors: "User auth changes (PR #523)"
- [ ] Suggest which commits might have introduced the issue
- [ ] Report commits that fixed related issues (for comparison)

**Claude Skill API:**
- [ ] Skill name: `/gasoline-fix`
- [ ] Parameters: `recording_id`, `original_test_boundary`, `replay_test_boundary`
- [ ] Returns: `{root_cause, confidence, suggested_fixes, related_commits, affected_files}`
- [ ] Callable by LLM during CI/CD or interactive debugging

**Safety & Guardrails:**
- [ ] Never auto-apply fixes; always propose for review
- [ ] If git unavailable, suggest based on error logs only
- [ ] Log all suggestions for audit trail
- [ ] Flag speculative fixes (low confidence) vs obvious fixes (high confidence)

---

## Out of Scope

- Mobile/native app testing (web only, Gasoline doesn't run in mobile)
- Video replay (screenshots + logs sufficient)
- Real-time collaboration (async sharing via recordings)
- Multi-user conflict resolution (one recording at a time)
- Auto-apply fixes without review (Gasoline proposes, LLM reviews and applies)

---

## Success Criteria

### Functional
- ✅ LLM can start/stop recordings via `configure()` and UI
- ✅ All user interactions (click, type, navigate) recorded with selectors + x/y
- ✅ Screenshots captured and stored on disk
- ✅ Recordings queryable via `observe()`
- ✅ Playback replays actions in correct sequence
- ✅ Element selector matching works (data-testid → CSS → x/y fallback)
- ✅ Moved elements logged with screenshots
- ✅ LLM can read action sequences and generate variations
- ✅ Generated flows execute and logs are captured
- ✅ LLM can invoke `/gasoline-fix` skill to analyze regressions
- ✅ Root cause analysis detects error types (404, timeout, missing element, etc.)
- ✅ Fix suggestions identify affected files and propose code changes
- ✅ Git integration finds related commits (if repo available)
- ✅ Suggestions ranked by confidence (high/medium/low)

### Non-Functional
- ✅ Recording overhead: < 5% CPU, < 20MB memory per active recording
- ✅ Playback speed: 10+ actions per second (sequence mode)
- ✅ Log accuracy: ±100ms timestamp deviation (WebSocket streaming)
- ✅ Storage: 1GB limit enforced, cleanup automatic
- ✅ Screenshot size: < 500KB per image
- ✅ Zero data loss (buffer with warnings before drop)

### Integration
- ✅ Works with test boundaries (logs tagged with test_id)
- ✅ Works with log comparison (original vs replay)
- ✅ Works in CI/CD (automated playback)
- ✅ MCP API consistent with observe/interact patterns

---

## Integration & Dependencies

### Internal
- **observe()**: Extended to support `what: 'recordings'`, `what: 'recording_actions'`
- **interact()**: Extended with `action: 'playback'`
- **configure()**: Extended with `action: 'recording_start'`, `action: 'recording_stop'`
- **Test Boundaries**: Playback runs under test boundary for log correlation
- **WebSocket**: Requires migration from polling for accurate timing
- **Claude Skill**: New `/gasoline-fix` skill for LLM to invoke root cause analysis
- **Git Integration**: Optional (read-only) for finding related commits

### External
- **Browser APIs:** `chrome.webRequest` (capture URLs), Mutation Observer (detect clicks/typing), Intersection Observer (screenshots)
- **File System:** Local disk storage for screenshots/metadata
- **Database:** Optional (for large-scale recording management, defer to Phase 2)

---

## Success Stories

### Story 1: Regression Testing
```
QA records 5-minute checkout flow.
Week later, dev fixes "missing error message on invalid card".
LLM: "Replay checkout recording"
→ Playback executes, logs captured
→ LLM compares original vs replay logs
→ Error message now present ✓
→ Regression test passes, no manual testing needed
```

### Story 2: Automated Variations
```
LLM reads checkout recording (add item → checkout).
Generates 3 variations:
  - Different product (shoes → shirt)
  - Different quantity (1 → 5)
  - Different coupon (none → SUMMER20)
All variations run, logs compared.
→ All pass → broader regression coverage
→ Results reported to CI/CD
```

### Story 3: Fragile Selector Debugging
```
Recording: "Click 'Add to Cart' button"
UI changed: button moved from (500, 200) to (505, 210)
Playback:
  - Tries [data-testid=add-to-cart] ✓ (found, uses new coords)
  - Logs: "Element moved: old=[500,200] new=[505,210]"
  - Screenshot: moved-selector issue
→ LLM notified, suggested update selector for robustness
```

### Story 4: Root Cause Analysis & Auto-Fix

```
Original: Checkout flow works (records clean logs).
Week later: Playback detects regression → POST /api/order returns 404.

LLM invokes skill: /gasoline-fix
→ Root cause analysis:
  - Error: "404 Not Found"
  - URL: "POST /api/order"
  - Likely cause: Endpoint renamed or removed
  - Confidence: HIGH

→ Fix suggestions:
  - Affected files: src/api/checkout.ts, src/handlers/order.ts
  - Proposed fix: "Endpoint renamed from /api/order to /api/orders"
  - Check: Recent commit (PR #234) changed API routes

→ Related commits:
  - Commit abc123: "Refactor API endpoints" (author: alice@company.com)
  - Commit def456: "Fix: Restore /api/order endpoint" (author: bob@company.com)

LLM reviews suggestions:
  - "Got it, the endpoint was renamed. I'll update the client call."
  - Applies fix: checkout.ts calls /api/orders instead
  - Re-runs playback
  → ✓ Regression resolved, logs match original
```

---

## Metrics & Observability

**What Gets Logged:**
- Recording start/stop (timestamp, name, action count)
- Playback execution (duration, actions executed, errors)
- Element matching (selector tried, success/failure, location shift)
- Screenshots (why, when, issue type)
- WebSocket events (connected, disconnected, buffer overflow, timestamp)

**LLM Observability:**
- `observe({what: 'recordings'})` to list
- `observe({what: 'recording_actions'})` to read actions
- `observe({what: 'logs', test_boundary: 'replay-X'})` to compare

---

## Claude Skill: `/gasoline-fix`

**When to Use:**
When flow recording playback detects a regression (logs differ between original and replay), invoke this skill to automatically analyze the root cause and suggest code fixes.

**How to Invoke:**
```
/gasoline-fix recording_id="shopping-checkout-20260130T..." \
              original_test_boundary="shopping-checkout-original" \
              replay_test_boundary="shopping-checkout-replay" \
              git_repo_path="/home/dev/my-app"
```

**Parameters:**
- `recording_id` (required): ID of the original recording (e.g., "shopping-checkout-20260130T143022Z")
- `original_test_boundary` (required): Test boundary ID from original recording (e.g., "shopping-checkout-original")
- `replay_test_boundary` (required): Test boundary ID from replay showing regression (e.g., "shopping-checkout-replay")
- `git_repo_path` (optional): Path to git repo for commit analysis. If provided, skill finds related commits. If omitted, skill analyzes error logs only.

**What the Skill Does:**

1. **Compares Logs:** Diffs logs from original test boundary vs replay test boundary
2. **Identifies Errors:** Detects error types:
   - Network errors (404, 500, timeout, connection refused)
   - DOM errors (element not found, selector changed)
   - Assertion failures (expected text missing)
   - Timing issues (load timeout)
3. **Suggests Root Causes:** Analyzes error patterns and proposes likely causes
4. **Finds Git Context (if git_repo_path provided):**
   - Identifies files that changed between commits
   - Shows commits that touched affected code
   - Highlights commits that might have introduced the issue
5. **Ranks Confidence:** Marks suggestions as HIGH/MEDIUM/LOW based on error clarity

**What You Get Back:**

The skill returns a natural language analysis with:
- **Root Cause:** Clear explanation of what's broken (e.g., "POST /api/order endpoint returns 404 because the endpoint was renamed to /api/orders")
- **Confidence Level:** How confident the analysis is (HIGH if error is explicit, MEDIUM if pattern-based, LOW if speculative)
- **Affected Files:** List of source files likely to need changes
- **Suggested Fixes:** Specific code changes to try (file, line number, what to change, why)
- **Related Commits:** If git available, shows commits that changed the affected code
- **Error Evidence:** Direct quotes from error logs and screenshot paths

**Example Response:**

```
ROOT CAUSE (HIGH confidence):
The /api/order endpoint was renamed to /api/orders in commit abc123
(PR #234 "Refactor API endpoints").

AFFECTED FILES:
- src/api/checkout.ts (line 45)
- src/handlers/order.ts (line 123)

SUGGESTED FIXES:
1. In src/api/checkout.ts line 45:
   Change: await fetch('/api/order', ...)
   To: await fetch('/api/orders', ...)
   Reason: Endpoint was renamed in refactor

RELATED COMMITS:
- Commit abc123 "Refactor API endpoints" (alice@company.com, Jan 28)
  → This commit likely introduced the issue
- Commit def456 "Fix: Restore /api/order endpoint" (bob@company.com, Jan 29)
  → This commit attempted to fix it but was reverted

ERROR LOG EVIDENCE:
POST /api/order 404 Not Found
at retry attempt 3/3

SCREENSHOT: /recordings/checkout-flow/20260130-...-error.jpg
```

**How to Use the Results:**

1. **Review:** Read the root cause analysis and suggested fixes
2. **Approve:** Decide if the suggestion makes sense (high confidence suggestions usually do)
3. **Apply:** Implement the suggested fix in your code
4. **Verify:** Re-run the recording playback to confirm the fix works
5. **Report:** If fix resolves the issue, you have automated root cause analysis + fix verification

**Safety Guardrails:**

- ✅ Skill never auto-applies fixes; you must review and approve
- ✅ Git operations are read-only (no commits, pushes, or destructive operations)
- ✅ All suggestions ranked by confidence; you know which are speculative vs obvious
- ✅ Full audit trail logged: who ran skill, when, parameters, results
- ✅ Works without git (analyzes error logs only if git_repo_path omitted)

**Common Workflows:**

### Fast Path (High Confidence)
```
Playback shows: "POST /api/order 404"
→ /gasoline-fix suggests: "Endpoint renamed to /api/orders"
→ You apply fix, re-run playback, ✓ passes
→ Done in 2 minutes
```

### Investigation Path (Low Confidence)
```
Playback shows: "Element not found: button.add-to-cart"
→ /gasoline-fix suggests: (LOW confidence) "DOM structure changed or CSS selector is fragile"
→ Related commits show UI refactor
→ You review code, update selector to use data-testid, re-run
→ ✓ passes
```

### No Git Available
```
Playback shows: "GET /user/profile timeout"
→ /gasoline-fix analyzes error logs (no git)
→ Suggests: "API endpoint may be slow or unavailable; check server logs"
→ You investigate server metrics, find database query N+1 problem
→ Fix applied, re-run, ✓ passes
```

---

## Next Steps

1. **Tech Spec:** Architecture, data structures, API contracts, skill implementation
2. **QA Plan:** Unit tests (recording, playback, selector matching, root cause analysis), UAT steps
3. **Implementation Phase 1:** Basic recording/playback + skill scaffolding
4. **Implementation Phase 2:** Root cause analysis + git integration, test generation, large-scale storage

