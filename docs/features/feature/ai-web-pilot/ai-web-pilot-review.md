# AI Web Pilot Spec Review

## Executive Summary

The spec defines a controlled escape hatch from Gasoline's capture-only model, adding highlight, state management, and JS execution tools gated behind a human opt-in toggle. The core safety model is sound -- toggle-per-session, explicit error codes, localhost-only -- but the spec has gaps in security hardening for `execute_javascript`, missing data contracts for error propagation, and an incomplete implementation surface (the shipped code includes `browser_action` which the spec never defines). The implementation is further along than the spec; the spec needs to catch up or risk becoming dead documentation.

---

## Critical Issues (Must Fix Before Shipping)

### C1. `execute_javascript` has no script size limit

**Section:** Features > Execute JavaScript

The spec says "JS code string" with no maximum length. The implementation in `inject.js:612` wraps the input in `new Function(fnBody)` -- the Go server passes the raw script through `json.RawMessage` with no size validation. An AI agent could send megabytes of JavaScript.

**Impact:** Memory pressure on both the Go server (pending query buffer) and the browser page context. Could also be used as a vector for denial-of-service against the developer's browser session.

**Fix:** Add a `maxScriptSize` constant (e.g., 64KB) validated in `handlePilotExecuteJS` before creating the pending query. Document the limit in the tool schema description.

---

### C2. Debug `fmt.Printf` statements in production code

**File:** `cmd/dev-console/pilot.go:104,115`

Two `fmt.Printf("[DEBUG] ...")` calls are present in `handlePilotHighlight`. These write to stdout, which is the MCP JSON-RPC transport channel. Any output to stdout that is not valid JSON-RPC will corrupt the MCP protocol stream and crash the client connection.

**Impact:** Protocol-breaking bug. Every successful highlight call injects non-JSON text into the MCP stdout stream.

**Fix:** Remove both `fmt.Printf` calls immediately. If debug logging is needed, use `fmt.Fprintf(os.Stderr, ...)` which is the established pattern elsewhere in the codebase (see `queries.go:208-214`).

---

### C3. Spec does not define `browser_action` tool

**Section:** Features (missing)

The implementation in `pilot.go:291-385` and `tools.go:997-1019` ships a `browser_action` tool with `open`, `refresh`, `navigate`, `back`, `forward` actions. The architecture doc in `.claude/docs/architecture.md:77` references it. But the spec document has zero mention of this tool -- no safety analysis, no schema, no test scenarios.

**Impact:** An undocumented tool with medium-risk browser control capability (navigation can cause data loss in unsaved forms, refresh can reset application state). The navigate action accepts arbitrary URLs with no validation -- an AI agent could navigate to `javascript:`, `data:`, or `file:` URLs.

**Fix:** Either (a) add a full `browser_action` section to the spec with URL scheme validation (whitelist `http:` and `https:` only), or (b) remove it from the implementation until specified. The navigate action should reject non-HTTP(S) URLs at the Go server level before forwarding to the extension.

---

### C4. `restoreState` clears all storage before restoring -- no rollback on partial failure

**Section:** Features > Browser State Snapshots

`inject.js:1019-1060` calls `localStorage.clear()` and `sessionStorage.clear()` before restoring. If the restore loop throws (e.g., storage quota exceeded, or a key triggers a storage event listener that throws), the developer loses their current state with no way to recover.

**Impact:** Data loss during state restoration. Contradicts the spec's claim that state changes are "reversible" (architecture.md:77).

**Fix:** Capture current state into a temp variable before clearing. On any error during restore, roll back to the captured state. Alternatively, do a diff-based restore (set/delete only changed keys) to minimize blast radius.

---

### C5. Cookie restore is incomplete and unsafe

**Section:** Features > Browser State Snapshots

`inject.js:1035-1046` attempts to clear cookies by setting `expires` to epoch, but only handles the root path. Cookies set with `path=/api` or `domain=.example.com` will not be cleared. The restore phase uses `document.cookie = c.trim()` which sets cookies without `path` or `domain` attributes, creating duplicates rather than replacing originals.

**Impact:** State snapshots will silently produce incorrect cookie state on restore. HttpOnly cookies cannot be read or written via `document.cookie` at all -- a fundamental limitation that the spec should acknowledge.

**Fix:** Document that cookie snapshots are best-effort and exclude HttpOnly cookies. For the restore, set `path=/; domain=<current>` explicitly. Consider using `chrome.cookies` API in the background script for complete cookie management (requires adding the `cookies` permission to manifest.json).

---

## Recommendations (Should Consider)

### R1. Timeout inconsistency between spec and implementation

The spec (section: Execute JavaScript) says the timeout is 5000ms. The implementation in `handlePilotExecuteJS` uses `h.capture.queryTimeout` for the pending query wait, which is the server-wide query timeout (10 seconds by default in the codebase). The `timeout_ms` parameter is forwarded to the extension but the server-side wait uses a different timeout.

**Recommendation:** The server-side wait should be `timeout_ms + buffer` (e.g., `timeout_ms + 2000ms`) to allow for the extension's own timeout to fire first. Currently if the extension timeout fires at 5s but the server timeout is 10s, the server waits unnecessarily. If the server timeout is shorter than the script timeout, the server returns "timeout" before the extension has a chance to return the execution result.

### R2. `manage_state` timeout is hardcoded to 10 seconds

**File:** `pilot.go:190-193`

The `handlePilotManageState` and `handleBrowserAction` handlers both hardcode `10*time.Second` for the pending query and wait timeout, while `handlePilotHighlight` and `handlePilotExecuteJS` use `h.capture.queryTimeout`. This is inconsistent.

**Recommendation:** Use a single configurable pilot timeout for all pilot tools. If different tools need different timeouts, make it explicit in the param structs or use a constant.

### R3. `new Function()` will fail on CSP-restricted pages

The implementation correctly detects CSP errors (`inject.js:641-647`) and returns a structured `csp_blocked` error. However, the spec does not mention this limitation at all.

**Recommendation:** Add a "Limitations" subsection to the `execute_javascript` spec section noting that pages with `script-src` CSP directives that don't include `'unsafe-eval'` will reject execution. This is important for AI agents to understand so they don't retry indefinitely.

### R4. Highlight element positioning uses `position: fixed` -- breaks on scroll

The spec says `position: fixed` (section: Highlight Element, bullet 3), and the implementation matches. The scroll handler at `inject.js:942-959` re-positions on scroll, which is correct. However, the scroll handler only listens on `window` -- elements inside scrollable containers (e.g., `overflow: auto` divs) will show the highlight in the wrong position after inner scroll.

**Recommendation:** Use `position: absolute` relative to the document body and calculate page-relative coordinates using `rect.top + window.scrollY`, or use a `ResizeObserver`/`IntersectionObserver` to track the element's position continuously during the highlight duration.

### R5. `manage_state` spec lists `save`/`load`/`list`/`delete` but implementation adds `capture`

The spec (section: Features > Browser State Snapshots) defines actions: `save`, `load`, `list`, `delete`. The implementation in `tools.go:962` and `pilot.go:139` adds `capture` as a valid action. The tool description says "Capture, save, load, list, or delete".

**Recommendation:** Update the spec to document all five actions and clarify the difference between `capture` (returns current state in-memory) and `save` (persists to extension storage).

### R6. No rate limiting on pilot tools

An AI agent in a loop could call `execute_javascript` or `highlight_element` hundreds of times per second. Unlike screenshot capture (which has a 5s cooldown and 10/session max in `background.js:67-68`), pilot tools have no rate limiting.

**Recommendation:** Add a configurable rate limit (e.g., 10 calls/second per tool) at the Go server level. This is especially important for `execute_javascript` which has real page-context side effects.

### R7. Error contract is inconsistent across pilot tools

- `handlePilotHighlight` parses the result into a typed struct, checks for `error == "ai_web_pilot_disabled"`, then returns the raw result on success.
- `handlePilotManageState` parses into `map[string]any`, checks for `error` key, and returns indented JSON on success.
- `handlePilotExecuteJS` returns the raw result with no parsing at all -- if the extension returns `{ error: "ai_web_pilot_disabled" }`, it gets passed through as a success response.

**Recommendation:** Extract a shared `parsePilotResult` function that all four handlers call. It should: (1) unmarshal the result, (2) check for `error` field, (3) handle `ai_web_pilot_disabled` consistently, (4) return the parsed result. The spec's "Extension Implementation" section (lines 119-131) defines message types but not the error contract. Add an "Error Contract" section.

### R8. `manage_state` spec claims snapshots stored in `chrome.storage.local` but actual implementation stores in page context

The spec says "Snapshots stored in extension's `chrome.storage.local` under `gasoline_snapshots` namespace." The actual `captureState()` and `restoreState()` functions in `inject.js:990-1060` operate directly on `localStorage`/`sessionStorage`/`document.cookie` -- they capture the page's own storage, not the extension's. The extension-side persistence of named snapshots happens in the background script.

**Recommendation:** Clarify the spec: the snapshot *data* is the page's storage state; the snapshot *persistence* (named save/load/list/delete) is in `chrome.storage.local`. These are two different concerns with different size limits (`chrome.storage.local` has a 10MB limit by default).

### R9. Spec's "MCP Tool Registration" section is confused

Section: MCP Tool Registration (lines 147-158) shows pilot tools as sub-actions of the `configure` composite tool. The actual implementation registers them as standalone tools (see `tools.go:929-1019`). The spec should be updated to reflect the standalone registration approach.

### R10. Audit logging gap for pilot tools

The codebase has an `AuditTrail` system (`tools.go:199`) that logs MCP tool invocations. Pilot tools should be explicitly audited since they have side effects (JS execution, state modification, navigation). The spec does not mention auditing.

**Recommendation:** Ensure `handlePilotExecuteJS` logs the script hash (not content, to avoid leaking sensitive code) and result status to the audit trail. Same for `browser_action` navigate calls.

---

## Implementation Roadmap

Based on severity and dependency ordering:

### Phase 0: Fix Protocol-Breaking Bug (immediate)

1. **Remove debug printf statements** from `pilot.go:104,115` -- these corrupt the MCP JSON-RPC stream. This is a live bug.

### Phase 1: Security Hardening (before any public release)

2. **Add script size limit** to `execute_javascript` (64KB max).
3. **Add URL scheme validation** to `browser_action` navigate/open -- reject non-HTTP(S) URLs.
4. **Add rate limiting** for pilot tools at the server level.
5. **Fix `execute_javascript` error contract** -- parse result for `ai_web_pilot_disabled` before returning.

### Phase 2: Data Integrity (before state management is promoted)

6. **Add rollback safety** to `restoreState` -- capture current state before clearing.
7. **Document cookie limitations** -- HttpOnly cookies are invisible, path/domain handling is incomplete.
8. **Add `chrome.storage.local` size guard** for snapshot persistence -- warn when approaching 10MB limit.
9. **Update spec** to document all five `manage_state` actions including `capture`.

### Phase 3: Consistency and Maintainability

10. **Extract shared `parsePilotResult`** function used by all four pilot handlers.
11. **Unify timeout handling** -- derive server-side wait from client-specified timeout.
12. **Fix timeout inconsistency** -- `manage_state` and `browser_action` should not hardcode 10s.
13. **Add `browser_action` section to spec** with full schema, safety analysis, and test scenarios.
14. **Update "MCP Tool Registration" section** -- remove stale `configure` sub-action examples.

### Phase 4: Robustness

15. **Fix highlight positioning** for elements inside scrollable containers.
16. **Add CSP limitation documentation** to the `execute_javascript` spec section.
17. **Wire pilot tools into audit trail** with script hash logging.
18. **Add integration test** that simulates extension-connected pilot workflow end-to-end.

---

## Notes

- The safety model (human opt-in toggle, explicit error codes, timeout != disabled distinction) is well-designed. The architecture doc's toggle protocol checklist (`.claude/docs/architecture.md:124-131`) is a good engineering artifact.
- The pending query mechanism (`queries.go`) is clean and the client isolation via `currentClientID` is the right pattern for multi-client MCP sessions.
- The spec correctly identifies that this breaks the "capture, don't interpret" philosophy (`product-philosophy.md:15`). The justification -- human verification and faster reproduction -- is reasonable, but the feature surface should be kept minimal to avoid scope creep.
