# QA Plan: Dialog Handling

> QA plan for the Dialog Handling feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Gasoline runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Dialog message text containing PII | `confirm("Delete account john@example.com?")` -- the dialog message is captured as-is. Verify this stays on localhost only. Messages may contain emails, usernames, or account details. | high |
| DL-2 | Prompt response text with credentials | AI provides `text` parameter for `prompt()` -- e.g., a password or API key. Verify the AI-provided text is not logged to the server's JSONL log or extension debug logs. | critical |
| DL-3 | Dialog events in log buffer | Dialog events are stored as log entries via `/logs`. Verify these entries do not persist beyond the ring buffer and are not written to disk. | high |
| DL-4 | Auto-response configuration containing secrets | `setup` mode stores auto-response config in extension memory. Verify this config is only in-memory, not persisted to `chrome.storage.local` or disk. | high |
| DL-5 | beforeunload handler text | `beforeunload` event text (e.g., "You have unsaved changes to form: Credit Card Application") may reveal sensitive context. Captured in telemetry. | medium |
| DL-6 | URL in dialog event metadata | Dialog events include the `url` where the dialog appeared. URLs may contain query parameters with tokens. | medium |
| DL-7 | Dialog queue leaking to other tabs | Auto-response configs are per-tab. Verify dialog events from tab A are not visible in tab B's context. | high |
| DL-8 | Data transmission path | Verify all dialog data flows only over localhost (127.0.0.1:7890). No external network calls. | critical |

### Negative Tests (must NOT leak)
- [ ] Prompt `text` parameter value does NOT appear in server JSONL logs
- [ ] Auto-response configuration is NOT persisted to `chrome.storage.local` (in-memory only)
- [ ] Dialog events from one tab do NOT appear when querying another tab's context
- [ ] Dialog message text is NOT transmitted to any external server
- [ ] `document.cookie` and `localStorage` are NOT accessed by dialog handling code
- [ ] Clearing auto-responses (`clear: true`) actually removes all stored config from memory

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Reactive vs proactive distinction | LLM understands `setup: true` is for future dialogs, while omitting `setup` responds to current/pending dialogs. Verify response messages clarify which mode was used. | [ ] |
| CL-2 | Dialog already dismissed | When using reactive mode, the dialog has already returned a default value (synchronous). LLM must understand the reactive response is informational only -- it cannot change the return value. | [ ] |
| CL-3 | `no_pending_dialog` semantics | LLM should not interpret this as "no dialogs have ever appeared". It means "no dialog is currently waiting for a response". Verify the message guides toward `setup` mode. | [ ] |
| CL-4 | `once: true` behavior | LLM must understand this means "auto-respond to the NEXT dialog only, then revert". Not "respond once per type". | [ ] |
| CL-5 | Default behaviors | LLM should know: alert=auto-accept, confirm/prompt=auto-deny (default). Verify this is stated in responses. | [ ] |
| CL-6 | `dialog_type: "all"` semantics | Applies configuration to ALL dialog types, not a specific "all" dialog type. Verify response clarity. | [ ] |
| CL-7 | Status query response | `pending_dialogs` and `auto_responses` arrays clearly describe current state. LLM should parse these to decide next action. | [ ] |
| CL-8 | `beforeunload` limitations | LLM must understand that `beforeunload` handling removes listeners (preventing the dialog) rather than responding to an existing dialog. | [ ] |
| CL-9 | Synchronous nature of alert/confirm/prompt | LLM must understand these are synchronous -- the override decides immediately. There is no "wait for AI to respond" capability for reactive handling. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM thinks reactive `handle_dialog` can change the return value of a dialog that already fired -- verify response clearly states the dialog was already auto-responded
- [ ] LLM confuses `setup: true` (pre-configure) with actual dialog response -- test by calling setup when no dialog exists and verifying "configured" response
- [ ] LLM does not call `setup` BEFORE the action that triggers a dialog -- verify documentation guides proactive usage pattern
- [ ] LLM assumes `clear: true` only clears one dialog type when `dialog_type: "all"` clears everything
- [ ] LLM does not understand that `once: true` auto-response is consumed after first dialog, subsequent dialogs use defaults

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Pre-configure auto-accept for confirm | 1 step: `handle_dialog({setup: true, dialog_type: "confirm", accept: true})` | No -- already minimal |
| Trigger action with dialog | 2 steps: (1) setup auto-response, (2) execute triggering action | No -- setup must precede trigger due to synchronous dialog nature |
| Check dialog status | 1 step: `handle_dialog({status: true})` | No -- already minimal |
| Clear all auto-responses | 1 step: `handle_dialog({setup: true, dialog_type: "all", clear: true})` | No -- single call |
| Handle unexpected dialog | 2 steps: (1) observe dialog event in logs, (2) call `handle_dialog({accept: true})` (informational only) | Could be simplified if dialogs auto-accept by default, but this is a safety tradeoff |
| Full delete confirmation flow | 3 steps: (1) setup confirm auto-accept, (2) click delete button, (3) verify result | This is the minimum for safe automation |

### Default Behavior Verification
- [ ] Without any setup, `alert()` auto-accepts (returns undefined, dialog is suppressed)
- [ ] Without any setup, `confirm()` auto-denies (returns false)
- [ ] Without any setup, `prompt()` auto-denies (returns null)
- [ ] Dialog events are reported to server log even without explicit setup
- [ ] `status` query works with no prior setup (returns empty arrays)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | `window.alert` override intercepts and reports | Page calls `alert("Hello")` | Alert suppressed, event posted to content.js with message "Hello" | must |
| UT-2 | `window.confirm` override returns false by default | Page calls `confirm("Sure?")` | Returns `false`, event posted | must |
| UT-3 | `window.prompt` override returns null by default | Page calls `prompt("Name?")` | Returns `null`, event posted | must |
| UT-4 | Auto-response for confirm returns true | Setup `accept: true` for confirm, then `confirm("Sure?")` | Returns `true` | must |
| UT-5 | Auto-response for prompt returns text | Setup `accept: true, text: "John"` for prompt, then `prompt("Name?")` | Returns `"John"` | must |
| UT-6 | `once: true` auto-response consumed after first use | Setup once for confirm, call `confirm()` twice | First returns configured value, second returns default (false) | must |
| UT-7 | `once: false` persists across multiple dialogs | Setup persistent for confirm, call `confirm()` three times | All return configured value | must |
| UT-8 | `dialog_type: "all"` applies to all types | Setup for "all" with accept=true | `alert()`, `confirm()`, `prompt()` all auto-accept | must |
| UT-9 | `clear: true` removes all auto-responses | Setup confirm auto-accept, then clear all | Next `confirm()` returns default (false) | must |
| UT-10 | Dialog event includes message and type | Page calls `confirm("Delete?")` | Event has `type: "confirm"`, `message: "Delete?"` | must |
| UT-11 | Dialog event includes URL | Dialog fires on example.com | Event has `url: "https://example.com/..."` | should |
| UT-12 | Dialog event includes timestamp | Any dialog fires | Event has ISO 8601 timestamp | should |
| UT-13 | Rate limiting for rapid dialogs | `alert()` called 20 times in 1 second | Max 10 events reported per second | should |
| UT-14 | `beforeunload` listener detection | Page registers `beforeunload` handler | Gasoline detects and reports the listener | should |
| UT-15 | `beforeunload` suppression | Setup `accept: true` for `beforeunload` | Page's `beforeunload` listeners removed | should |
| UT-16 | `onbeforeunload` property monitored | Page sets `window.onbeforeunload = fn` | Property assignment detected | should |
| UT-17 | Prompt with accept=false returns null | Setup `accept: false` for prompt | Returns `null` regardless of `text` param | must |
| UT-18 | Prompt with accept=true but no text | Setup `accept: true` with no `text` | Returns empty string `""` | should |
| UT-19 | Override detection minimization | Check `alert.toString()` | Returns native-looking string (via Object.defineProperty) | could |
| UT-20 | Server-side `handle_dialog` dispatch | `{action: "handle_dialog", setup: true, dialog_type: "confirm"}` | Pending query of type "dialog_config" created | must |
| UT-21 | Server-side reactive handler | `{action: "handle_dialog", accept: true}` | Checks dialog queue, returns appropriate response | must |
| UT-22 | Server-side status query | `{action: "handle_dialog", status: true}` | Returns `pending_dialogs` and `auto_responses` arrays | must |
| UT-23 | Server dialog queue max size | 6+ dialogs reported without AI handling | Queue capped at 5 entries | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full proactive setup flow | Go server -> background.js -> content.js -> inject.js | Auto-response configured, next dialog uses it | must |
| IT-2 | Dialog event telemetry flow | inject.js -> content.js -> background.js -> POST /logs | Dialog event appears in log buffer | must |
| IT-3 | Reactive handle_dialog flow | Server checks queue -> returns pending dialog info | AI receives dialog details | must |
| IT-4 | Status query end-to-end | Server queries extension state | Accurate pending_dialogs and auto_responses returned | must |
| IT-5 | Tab isolation | Setup on tab A, dialog on tab B | Tab B uses defaults, not tab A's config | should |
| IT-6 | Config survives page navigation | Setup, then navigate same tab | Config should be re-sent by background.js on new page load | should |
| IT-7 | Extension disconnect during setup | Server sends setup, extension disconnects | Timeout error returned | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Dialog interception latency | Time from `confirm()` call to override return | < 0.1ms | must |
| PT-2 | Auto-response lookup time | Time to check auto-response config | < 0.05ms | must |
| PT-3 | Dialog event reporting time | Time from interception to postMessage | < 1ms | must |
| PT-4 | Memory overhead of override code | inject.js size increase | < 5KB | should |
| PT-5 | Memory overhead of dialog queue | Server-side queue memory | < 10KB (max 5 entries) | should |
| PT-6 | Rapid dialog handling | 20 `alert()` calls in 1 second | All suppressed without blocking, events rate-limited | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No pending dialog when reactive called | `handle_dialog({accept: true})` with no dialog | `no_pending_dialog` response with setup hint | must |
| EC-2 | Multiple dialogs in rapid succession | `confirm()` x 5 in tight loop | Each intercepted individually. `once: true` consumed on first. | must |
| EC-3 | Dialog fires before setup completes (race) | Dialog fires 10ms after setup request sent | Dialog uses default behavior; setup takes effect for subsequent dialogs | must |
| EC-4 | Tab closed while auto-response configured | Close tab with active config | Config silently discarded, no error | should |
| EC-5 | Extension disconnected when dialog fires | Extension crashed/reloaded | Native dialogs appear (graceful degradation) | should |
| EC-6 | `alert()` in tight loop (100 calls) | Page code runs 100 alerts | All suppressed. Rate-limited to 10 events/sec. No UI blocking. | should |
| EC-7 | `prompt()` with no text and accept=true | Setup confirm for prompt with no text param | Returns `""` (empty string) | should |
| EC-8 | `beforeunload` event vs `onbeforeunload` property | Both registration methods used | Both detected and suppressible | should |
| EC-9 | Page code detects override via `toString()` | `alert.toString()` called by page | Returns native-looking string | could |
| EC-10 | Concurrent handle_dialog calls | Two AI agents call handle_dialog simultaneously | Both receive consistent state without race conditions | could |
| EC-11 | Dialog message with special characters | `confirm('Delete "item" & reload?')` | Message captured correctly including quotes and ampersand | should |
| EC-12 | Setup for one type, dialog of different type | Setup for `confirm`, `alert()` fires | Alert uses alert default (accept), not confirm config | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page with buttons that trigger `alert()`, `confirm()`, `prompt()`, and a link that triggers `beforeunload`
- [ ] Tab is being tracked by the extension
- [ ] AI Web Pilot toggle is enabled

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "interact", "arguments": {"action": "handle_dialog", "status": true}}` | No dialogs have appeared yet | `{"status": "ok", "pending_dialogs": [], "auto_responses": []}` | [ ] |
| UAT-2 | `{"tool": "interact", "arguments": {"action": "handle_dialog", "setup": true, "dialog_type": "confirm", "accept": true, "once": true}}` | N/A (no visible change) | `{"status": "configured", "dialog_type": "confirm", "accept": true, "once": true}` | [ ] |
| UAT-3 | Human clicks button that triggers `confirm("Are you sure?")` | NO native dialog appears (intercepted) | Dialog was auto-accepted. Page proceeds as if user clicked OK. | [ ] |
| UAT-4 | `{"tool": "interact", "arguments": {"action": "handle_dialog", "status": true}}` | N/A | Auto-response consumed (`once: true`). `auto_responses` is now empty. | [ ] |
| UAT-5 | Human clicks the confirm button again | Native `confirm()` does NOT appear (override returns `false` default) | Page receives `false` (default deny). No native dialog shown. | [ ] |
| UAT-6 | `{"tool": "interact", "arguments": {"action": "handle_dialog", "setup": true, "dialog_type": "prompt", "accept": true, "text": "TestUser", "once": true}}` | N/A | Prompt auto-response configured | [ ] |
| UAT-7 | Human clicks button that triggers `prompt("Enter name:")` | NO native dialog appears | Page receives `"TestUser"` as the prompt response | [ ] |
| UAT-8 | `{"tool": "interact", "arguments": {"action": "handle_dialog", "accept": true}}` | No dialog currently pending | `{"status": "no_pending_dialog", "message": "No dialog is currently pending..."}` | [ ] |
| UAT-9 | `{"tool": "interact", "arguments": {"action": "handle_dialog", "setup": true, "dialog_type": "all", "accept": true, "once": false}}` | N/A | Persistent auto-accept configured for ALL dialog types | [ ] |
| UAT-10 | Human clicks alert, confirm, prompt buttons in sequence | NO native dialogs appear for any | All three auto-accepted. Alert suppressed, confirm returns true, prompt returns "" | [ ] |
| UAT-11 | `{"tool": "interact", "arguments": {"action": "handle_dialog", "setup": true, "dialog_type": "all", "clear": true}}` | N/A | All auto-responses cleared | [ ] |
| UAT-12 | Human clicks confirm button | NO native dialog (override still present, returns default false) | Page receives `false` (default deny behavior) | [ ] |
| UAT-13 | Verify dialog events in logs: `{"tool": "observe", "arguments": {"what": "logs"}}` | N/A | Log entries with dialog type events present, showing messages and timestamps | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Dialog messages stay on localhost | Monitor network during dialog interception | All traffic goes to 127.0.0.1:7890 only | [ ] |
| DL-UAT-2 | Prompt text not in server logs | Check server's JSONL log file for the prompt text "TestUser" | "TestUser" does NOT appear in JSONL logs (only in in-memory dialog queue) | [ ] |
| DL-UAT-3 | Auto-response config not persisted | Reload extension, check `chrome.storage.local` | No dialog auto-response config in storage | [ ] |
| DL-UAT-4 | Tab isolation | Setup on tracked tab, trigger dialog on different tab | Different tab uses default behavior, not tracked tab's config | [ ] |

### Regression Checks
- [ ] Existing `interact` tool actions (`execute_js`, `highlight`) still work after dialog handling is enabled
- [ ] Extension does not interfere with pages that legitimately use `alert()`/`confirm()`/`prompt()` when Gasoline is installed but the feature is not actively configured
- [ ] Page load performance is not degraded by dialog override installation
- [ ] AI Web Pilot toggle correctly gates dialog handling (disabled toggle = no interception)

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
