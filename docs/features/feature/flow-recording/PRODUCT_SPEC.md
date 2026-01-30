---
feature: Flow Recording & Playback
status: proposed
tool: interact, observe, configure
mode: recording, playback, test-generation
version: v6.0
---

# Product Spec: Flow Recording & Playback (Regression Testing)

## Problem Statement

Manual QA testing is slow, fragile, and hard to maintain:

1. **Recording Gap:** QA engineers manually write test scripts. If UI changes, scripts break.
2. **Regression Testing:** When bugs are fixed, QA manually re-tests. Can't guarantee coverage or prevent regressions.
3. **Reproducibility:** "Works on my machine" — environment differences cause flaky tests.
4. **Variation Testing:** Testing multiple scenarios (different carts, different users) requires copying scripts.
5. **CI/CD Integration:** No automated way to test user flows in CI pipelines.

**Result:** Regressions slip through, manual testing is 30% of release cycle time.

---

## Solution

**Flow Recording & Playback** lets LLMs record user flows once, then:
- **Replay** to verify regressions are fixed
- **Generate variations** automatically (different inputs/flows)
- **Compare logs** (original vs replay) to detect issues

Transforms manual QA into **automated, AI-driven regression testing**.

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

**No Truncation:**
- [ ] Full text typed (passwords, long values, etc.)
- [ ] Sensitive data: **configurable setting** (warn if enabled and secrets detected)
- [ ] Default: **DO NOT hide** (warn user to disable setting if testing login flows)

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
- [ ] Format: JPEG (85% compression, PNG fallback if alpha needed)
- [ ] Max file size: Configurable (default 500KB per screenshot)
- [ ] Storage: Disk (local file system, not in-memory)
- [ ] Naming: `{date}-{recording_name}-{action_number}-{issue_type}.jpg`
  - Example: `20260130-shopping-checkout-003-click.jpg`
  - Issue types: `click`, `type`, `navigate`, `page-load`, `moved-selector`, `error`

**When Captured:**
- [ ] On page load (after navigation)
- [ ] After significant delay (> 500ms post-action)
- [ ] On error/timeout
- [ ] Every N actions (configurable, default 5)

### R4: Element Matching Strategy

**For Playback, Match Elements Using (Priority Order):**
1. **data-testid** attribute (most reliable for dynamic content)
2. **CSS selector path** (e.g., `.product-card:nth-child(3) .add-to-cart`)
3. **aria-label** or semantic attributes
4. **x/y coordinates** (fallback if selectors fail)

**If Element Moved:**
- [ ] Log warning: "Element moved: old=[x:500, y:200], new=[x:505, y:210]"
- [ ] Screenshot with issue type: `moved-selector`
- [ ] Continue playback with new coordinates
- [ ] Report to LLM for debugging

---

### R5: Playback & Sequence Execution

**Execution Mode:**
- [ ] **Sequence mode** (default): Execute actions in order, ignoring timing
  - Navigate → click → type → navigate → click (fast-forward)
  - Useful for regression testing (speed matters)
- [ ] **Timed mode** (optional): Respect original delays between actions
  - Useful for performance/timing validation
  - No multiplier (1x speed) to start

**On Page Load During Playback:**
- [ ] Wait for page to load (network idle or timeout: 5 sec)
- [ ] If selector not found, log error and take screenshot (`moved-selector` issue)
- [ ] If element found but location different, note in logs and screenshot
- [ ] Continue playback (non-blocking)

**Error Handling:**
- [ ] Selector not found → Log + screenshot, continue
- [ ] Click outside viewport → Scroll to element, then click
- [ ] Type in non-input → Log error, continue
- [ ] Navigation timeout → Log, continue
- [ ] Network timeout → Log, continue

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

### R9: Recording Limits & Cleanup

**Constraints:**
- [ ] **Max duration:** 30 minutes per recording
- [ ] **Max actions:** 500 per recording (typical workflow)
- [ ] **Max storage:** 1GB total (configurable)
- [ ] **Concurrent:** Only 1 active recording at a time
- [ ] **Auto-cleanup:**
  - Keep last 7 days (configurable)
  - Archive older (compress, move to long-term storage)
  - Warn when approaching 1GB limit

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

## Claude Skill Definition: `/gasoline-fix`

**Purpose:** LLM-callable skill to analyze flow recording regressions and suggest fixes.

**Invocation:**
```
User: "Analyze the regression and suggest fixes"
Claude: Invokes /gasoline-fix with parameters
→ Returns: root cause, confidence, suggested fixes, related commits
```

**Parameters:**
- `recording_id` (string, required): ID of original recording
- `original_test_boundary` (string, required): Test boundary ID from original recording
- `replay_test_boundary` (string, required): Test boundary ID from replay/regression
- `git_repo_path` (string, optional): Path to git repo for commit analysis

**Response:**
```json
{
  "root_cause": "POST /api/order endpoint returns 404 (renamed to /api/orders)",
  "confidence": "HIGH",
  "error_types": ["network_error"],
  "affected_action": 5,
  "suggested_fixes": [
    {
      "file": "src/api/checkout.ts",
      "line": 45,
      "change": "endpoint: '/api/orders'",
      "rationale": "Endpoint was renamed in recent refactor"
    }
  ],
  "affected_files": [
    "src/api/checkout.ts",
    "src/handlers/order.ts"
  ],
  "related_commits": [
    {
      "hash": "abc123",
      "message": "Refactor API endpoints",
      "author": "alice@company.com",
      "date": "2026-01-28"
    }
  ],
  "error_log_excerpt": "POST /api/order 404 Not Found",
  "screenshot_paths": [
    "/recordings/checkout-flow/20260130-...-error.jpg"
  ]
}
```

**Safety Constraints:**
- No auto-apply fixes; LLM must review and approve
- Git operations read-only (no commits, pushes, or destructive operations)
- Requires explicit opt-in from LLM to use git (optional parameter)
- All suggestions ranked by confidence; speculative suggestions marked clearly
- Audit trail logged: who ran skill, when, parameters, results

**Usage Example:**
```python
# LLM invokes skill when regression detected
skill_result = invoke_skill("gasoline-fix", {
    "recording_id": "shopping-checkout-20260130T...",
    "original_test_boundary": "shopping-checkout-original",
    "replay_test_boundary": "shopping-checkout-replay",
    "git_repo_path": "/home/dev/my-app"
})

# LLM reviews result
print(f"Root Cause: {skill_result['root_cause']}")
print(f"Confidence: {skill_result['confidence']}")

# LLM applies suggested fix and re-runs playback
apply_suggested_fix(skill_result['suggested_fixes'][0])
replay_recording(...)
```

---

## Next Steps

1. **Tech Spec:** Architecture, data structures, API contracts, skill implementation
2. **QA Plan:** Unit tests (recording, playback, selector matching, root cause analysis), UAT steps
3. **Implementation Phase 1:** Basic recording/playback + skill scaffolding
4. **Implementation Phase 2:** Root cause analysis + git integration, test generation, large-scale storage

