# QA Plan: Drag & Drop Automation

> QA plan for the Drag & Drop Automation feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Gasoline runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Dragged element text content in response | The response includes `from.text` and `to.text` (e.g., `"Fix login bug"`, `"Done"`). These may contain task titles with PII. Verify all data stays on localhost. | medium |
| DL-2 | DataTransfer object payload | Synthesized `DataTransfer` objects are empty by default. Verify no page data is injected into the drag payload by Gasoline. Page's own `dragstart` handler may populate it, but Gasoline should not. | high |
| DL-3 | Element selectors revealing DOM structure | Response includes `from.selector` and `to.selector`. Selectors are structural, not sensitive. Verify no PII in selectors. | low |
| DL-4 | Coordinate data revealing screen layout | Response includes `x`/`y` coordinates for both source and target. This reveals element positions but not sensitive content. | low |
| DL-5 | Tag and element metadata | Response includes `tag: "DIV"`. Non-sensitive structural data. | low |
| DL-6 | Error messages containing sensitive selectors | Failure responses include the selector that was tried. If the selector itself contains PII (unlikely but possible with `data-email` attributes), it would be echoed. | low |
| DL-7 | Data transmission path | Verify all drag data flows only over localhost (127.0.0.1:7890). No external network calls. | critical |
| DL-8 | AI Web Pilot toggle bypass | Verify drag action is gated behind `isAiWebPilotEnabled()`. If toggled off, no drag operations should execute. | critical |

### Negative Tests (must NOT leak)
- [ ] Gasoline does NOT inject any data into the `DataTransfer` object (it is empty on creation)
- [ ] No drag event data is sent to external servers
- [ ] Element text content in response stays on localhost
- [ ] AI Web Pilot toggle OFF prevents all drag operations (returns `ai_web_pilot_disabled`)
- [ ] No `document.cookie`, `localStorage`, or `sessionStorage` accessed during drag

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Async response model | LLM understands the initial response is `"status": "queued"` with a `correlation_id`, and the actual result requires polling via `observe({what: "command_result"})`. | [ ] |
| CL-2 | Selector vs coordinate mode | LLM understands it can use CSS selectors (`from`/`to`) OR coordinates (`from_x`/`from_y`, `to_x`/`to_y`) but should not mix both for the same endpoint (source or target). | [ ] |
| CL-3 | HTML5 vs mouse event method | `method: "html5_drag_api"` vs `method: "mouse_events"` -- LLM should understand which detection was used and what it means for compatibility. | [ ] |
| CL-4 | `events_dispatched` array | List of event names dispatched. LLM should understand this describes what happened, not what should happen. Useful for debugging. | [ ] |
| CL-5 | Element not found vs not visible | `error: "element_not_found"` vs `error: "element_not_visible"` -- LLM should distinguish between "selector matched nothing" and "element exists but cannot be interacted with". | [ ] |
| CL-6 | Drop success is application-dependent | The drag automation dispatches events faithfully. Whether the application's drop handlers accept the drop is outside Gasoline's control. LLM should not assume a successful response means the UI state changed. | [ ] |
| CL-7 | `matches_count` for ambiguous selectors | If multiple elements match, first is used. LLM should be aware of ambiguity. | [ ] |
| CL-8 | Coordinates out of viewport | `error: "coordinates_out_of_viewport"` with viewport dimensions helps LLM correct coordinates. | [ ] |
| CL-9 | `element_detached` mid-drag | Source removed during drag. LLM should understand this means a React re-render or DOM mutation interrupted the operation. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM assumes `{"success": true}` means the application accepted the drop -- verify response documentation makes clear this is event-dispatch success, not application-level success
- [ ] LLM tries to use both selector and coordinates for the same endpoint -- verify validation error is clear
- [ ] LLM does not poll `observe({what: "command_result"})` after receiving `"status": "queued"` -- verify the response message includes explicit polling instructions
- [ ] LLM does not understand `steps` parameter affects compatibility -- verify default is sensible and increasing steps is suggested for framework issues
- [ ] LLM confuses `from_offset` (relative to element center) with absolute coordinates

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Drag element A to element B (selector) | 2 steps: (1) `interact({action: "drag", from: "...", to: "..."})`, (2) poll for result | Could be 1 step with synchronous mode (but async is correct for reliability) |
| Drag by coordinates | 2 steps: same as above with `from_x/y`, `to_x/y` | No -- same async pattern |
| Drag with offset | 2 steps: add `from_offset`/`to_offset` params | No -- parameters are additive |
| Verify drag succeeded | 3 steps: (1) drag, (2) poll result, (3) `query_dom` or `observe` to verify UI state | Could be simplified with a "drag and verify" combo, but separation is cleaner |
| Debug failed drag | 2 steps: (1) read error response, (2) try alternate selector/coordinates | Error messages include guidance |

### Default Behavior Verification
- [ ] Feature works with just `from` and `to` selectors (minimal parameters)
- [ ] Default `steps: 5` provides reasonable framework compatibility
- [ ] Default `tab_id: 0` uses currently tracked tab
- [ ] Default `from_offset`/`to_offset` is center of element (no specification needed)
- [ ] HTML5 vs mouse event detection is automatic (no manual specification)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | `handleDrag` validates from/to parameters | `{action: "drag", from: "#a", to: "#b"}` | Pending query created with type `"drag"` | must |
| UT-2 | `handleDrag` validates coordinate parameters | `{from_x: 100, from_y: 200, to_x: 300, to_y: 400}` | Pending query created | must |
| UT-3 | `handleDrag` rejects missing source | `{action: "drag", to: "#b"}` (no from or from_x/from_y) | Validation error | must |
| UT-4 | `handleDrag` rejects missing target | `{action: "drag", from: "#a"}` (no to or to_x/to_y) | Validation error | must |
| UT-5 | `handleDrag` rejects both selector and coordinates for same endpoint | `{from: "#a", from_x: 100, from_y: 200, to: "#b"}` | Validation error | must |
| UT-6 | `handleDrag` allows mixed mode | `{from: "#a", to_x: 300, to_y: 400}` | Accepted (selector for source, coordinates for target) | should |
| UT-7 | `handleDrag` rejects steps < 1 | `{from: "#a", to: "#b", steps: 0}` | Validation error | should |
| UT-8 | `handleDrag` checks AI Web Pilot toggle | Toggle disabled | `{error: "ai_web_pilot_disabled"}` | must |
| UT-9 | Drag execution: HTML5 detected | Source has `draggable="true"` | `method: "html5_drag_api"`, DragEvent sequence dispatched | must |
| UT-10 | Drag execution: mouse event fallback | Source without `draggable` attribute | `method: "mouse_events"`, MouseEvent sequence dispatched | must |
| UT-11 | Drag execution: source element not found | Selector matches nothing | `{success: false, error: "element_not_found"}` | must |
| UT-12 | Drag execution: source element not visible | Element with `display: none` | `{success: false, error: "element_not_visible"}` | must |
| UT-13 | Drag execution: target element not found | To selector matches nothing | `{success: false, error: "element_not_found"}` | must |
| UT-14 | Drag execution: multiple matches | Selector matches 3 elements | First element used, `matches_count: 3` in response | should |
| UT-15 | Drag execution: correct event sequence (HTML5) | HTML5 drag | `mousedown -> dragstart -> dragover(xN) -> drop -> dragend` | must |
| UT-16 | Drag execution: correct event sequence (mouse) | Mouse drag | `pointerdown+mousedown -> pointermove+mousemove(xN) -> pointerup+mouseup` | must |
| UT-17 | Drag execution: intermediate steps | `steps: 3` | 3 intermediate movement events along linear path | must |
| UT-18 | Drag execution: offset from center | `from_offset: {x: 10, y: -5}` | Drag starts at element center + offset | should |
| UT-19 | Drag execution: DataTransfer is empty | HTML5 drag | `DataTransfer` has `effectAllowed` but no `setData` calls from Gasoline | should |
| UT-20 | Drag execution: coordinates out of viewport | `from_x: -100, from_y: 5000` | `{success: false, error: "coordinates_out_of_viewport"}` | must |
| UT-21 | Drag execution: element detached mid-drag | Source removed from DOM during event sequence | `{success: false, error: "element_detached"}`, cleanup `dragend` fired | should |
| UT-22 | Drag execution: PointerEvents dispatched alongside MouseEvents | Mouse event mode | `pointerdown`, `pointermove`, `pointerup` also dispatched | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full drag round trip (selector-based) | Go server -> background.js -> content.js -> inject.js -> drag execution | Drag executed, result posted back, AI polls and receives result | must |
| IT-2 | Full drag round trip (coordinate-based) | Same pipeline with coordinates | Same flow with coordinate resolution | must |
| IT-3 | Async command pattern | Server queues, extension polls, result posted | Correlation ID matches between queue and result | must |
| IT-4 | AI Web Pilot toggle gating | Toggle OFF, drag requested | Immediate `ai_web_pilot_disabled` error | must |
| IT-5 | Extension timeout | Extension disconnected, drag requested | Timeout after standard duration | must |
| IT-6 | Tab targeting | `tab_id` specifying non-tracked tab | Drag executes on correct tab | should |
| IT-7 | Message forwarding: GASOLINE_DRAG type | content.js bridge | Correct message type forwarded to inject.js | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Server response time (queue + return) | Time to return `queued` status | < 50ms | must |
| PT-2 | Full drag execution time | inject.js drag sequence | < 500ms | must |
| PT-3 | Total round-trip (queue -> poll -> result) | End-to-end latency | < 3s | must |
| PT-4 | Memory impact of drag handler | inject.js size increase | < 20KB | should |
| PT-5 | Main thread blocking per event dispatch | Per-event blocking time | < 0.1ms | must |
| PT-6 | Drag with 20 steps | Higher step count | < 700ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Source and target are same element | `from: "#el", to: "#el"` | Drag executes (degenerate case), events dispatched at same coordinates | should |
| EC-2 | Source is inside target | Nested elements | Drag from child to parent, correct coordinates computed | should |
| EC-3 | Target is inside source | Nested elements (reverse) | Drag from parent to child, events dispatched correctly | should |
| EC-4 | Element inside iframe | Selector in iframe context | `element_not_found` (top-level querySelector cannot reach iframe) | must |
| EC-5 | Cross-origin iframe target | Cross-origin content | `element_not_found` | must |
| EC-6 | Canvas-based drag target | Coordinate mode targeting canvas | Events dispatched at coordinates, Gasoline cannot introspect canvas state | should |
| EC-7 | `event.isTrusted` check by framework | Framework rejects synthetic events | Drag "succeeds" from Gasoline's perspective (events dispatched) but app may not respond. Document as known limitation. | should |
| EC-8 | Extremely long drag path | `steps: 100`, large distance | All intermediate events dispatched, performance within SLO | could |
| EC-9 | Zero-dimension element | `from` matches element with 0x0 size | `element_not_visible` error | should |
| EC-10 | Concurrent drag operations | Two drags requested simultaneously | Each gets unique correlation_id, results routed independently | could |
| EC-11 | React-beautiful-dnd timing requirement | Framework needs ~120ms delay after mousedown | May fail without timing delay. Document as known limitation (OI-4). | should |
| EC-12 | SortableJS detection | Element with `draggable="true"` set by SortableJS | HTML5 drag path used correctly | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome extension installed and connected
- [ ] AI Web Pilot toggle enabled in extension popup
- [ ] A test web page loaded with:
  - A Kanban board or sortable list with draggable items (e.g., `draggable="true"` cards)
  - A drop zone that accepts HTML5 drops
  - Items identifiable by CSS selectors (IDs or data-testid attributes)
- [ ] Tab is being tracked by the extension

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "interact", "arguments": {"action": "drag", "from": "#task-1", "to": "#column-done"}}` | Watch for visual drag movement | Response: `{"status": "queued", "correlation_id": "cmd_..."}` | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "command_result", "correlation_id": "<id from UAT-1>"}}` | Task card moved to Done column | `{"success": true, "method": "html5_drag_api", "events_dispatched": [...]}` | [ ] |
| UAT-3 | `{"tool": "interact", "arguments": {"action": "drag", "from_x": 150, "from_y": 300, "to_x": 600, "to_y": 300}}` | Element at source coordinates dragged to target | Queued response, then success on poll | [ ] |
| UAT-4 | `{"tool": "interact", "arguments": {"action": "drag", "from": "#nonexistent", "to": "#column-done"}}` | N/A | Poll returns `{"success": false, "error": "element_not_found", "selector": "#nonexistent"}` | [ ] |
| UAT-5 | `{"tool": "interact", "arguments": {"action": "drag", "from": "#hidden-item", "to": "#column-done"}}` | Hidden item exists in DOM | Poll returns `{"success": false, "error": "element_not_visible"}` | [ ] |
| UAT-6 | `{"tool": "interact", "arguments": {"action": "drag", "from": ".task-card", "to": "#column-done"}}` | Multiple cards with class `.task-card` | First visible card dragged, response includes `matches_count` | [ ] |
| UAT-7 | `{"tool": "interact", "arguments": {"action": "drag", "from": "#task-2", "to": "#column-done", "steps": 10}}` | Smoother drag animation (more steps) | Success with 10 intermediate events in `events_dispatched` | [ ] |
| UAT-8 | Disable AI Web Pilot toggle, then: `{"tool": "interact", "arguments": {"action": "drag", "from": "#task-3", "to": "#column-done"}}` | N/A | `{"error": "ai_web_pilot_disabled"}` | [ ] |
| UAT-9 | `{"tool": "interact", "arguments": {"action": "drag", "from_x": -50, "from_y": 300, "to_x": 600, "to_y": 300}}` | N/A | `{"success": false, "error": "coordinates_out_of_viewport"}` | [ ] |
| UAT-10 | Test with mouse-event-based sortable list (no `draggable` attribute) | Item reorders in list | `method: "mouse_events"`, events include `pointerdown`, `pointermove`, `pointerup` | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | All traffic on localhost | Monitor network during drag operations | Only 127.0.0.1:7890 traffic | [ ] |
| DL-UAT-2 | DataTransfer is empty | Inspect drag events in page console | No Gasoline-injected data in DataTransfer | [ ] |
| DL-UAT-3 | AI Web Pilot toggle enforced | Disable toggle, attempt drag | Drag rejected immediately | [ ] |
| DL-UAT-4 | Element text in response is informational only | Check response `from.text` and `to.text` | Contains element text but only transmitted on localhost | [ ] |

### Regression Checks
- [ ] Existing `interact` tool actions (`execute_js`, `highlight`, `handle_dialog`) still work
- [ ] Extension message forwarding handles `GASOLINE_DRAG` type without affecting other message types
- [ ] Page drag-and-drop functionality works normally when Gasoline is installed but no drag commands are issued
- [ ] Async command infrastructure (`correlation_id`, polling) works for both `drag` and `execute_js` concurrently

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
