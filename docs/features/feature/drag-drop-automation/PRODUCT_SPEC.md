---
feature: drag-drop-automation
status: proposed
version: null
tool: interact
mode: drag
authors: []
created: 2026-01-28
updated: 2026-01-28
---

# Drag & Drop Automation

> Programmatic drag-and-drop interactions via the `interact` tool, supporting both CSS selector-based and coordinate-based targeting to automate Kanban boards, sortable lists, file uploads, and other drag-enabled UI patterns.

## Problem

Modern web applications heavily rely on drag-and-drop interactions: Kanban boards (Trello, Jira, Linear), sortable lists, file upload zones, layout builders, and calendar scheduling. AI coding agents currently cannot test or automate these interactions through Gasoline. The only workaround is `execute_js` with manually crafted drag event sequences, which is fragile, verbose, and requires deep knowledge of the target application's drag implementation (HTML5 Drag API vs mouse events vs framework-specific internals).

This gap means AI agents cannot:
- Verify that a Kanban card moves between columns after a code change
- Test sortable list reordering logic
- Automate drag-based UI workflows during development
- Validate drop zone acceptance logic

## Solution

Add a `drag` action to the `interact` tool that synthesizes the correct sequence of drag-and-drop events on the target page. The implementation detects whether the target elements use the HTML5 Drag and Drop API or legacy mouse-event-based dragging, and dispatches the appropriate event sequence. Both CSS selector-based and coordinate-based targeting are supported to handle all common drag patterns.

The implementation follows Gasoline's existing async command architecture: the Go server creates a pending query with a correlation ID, the extension picks it up via polling, inject.js executes the drag sequence in page context, and the result is posted back asynchronously.

## User Stories

- As an AI coding agent, I want to drag an element from one container to another so that I can verify drag-and-drop UI behavior after code changes.
- As an AI coding agent, I want to reorder items in a sortable list so that I can test list sorting logic end-to-end.
- As a developer using Gasoline, I want to automate drag-and-drop interactions without writing raw JavaScript event dispatch code so that my AI assistant can test drag-heavy UIs like Kanban boards.
- As an AI coding agent, I want to drag using coordinates so that I can interact with canvas-based or custom drag implementations that do not expose stable CSS selectors.

## MCP Interface

**Tool:** `interact`
**Action:** `drag`

### Request (selector-based)

```json
{
  "tool": "interact",
  "arguments": {
    "action": "drag",
    "from": "#task-123",
    "to": "#column-done",
    "tab_id": 0
  }
}
```

### Request (coordinate-based)

```json
{
  "tool": "interact",
  "arguments": {
    "action": "drag",
    "from_x": 150,
    "from_y": 300,
    "to_x": 600,
    "to_y": 300,
    "tab_id": 0
  }
}
```

### Request (selector with offset)

```json
{
  "tool": "interact",
  "arguments": {
    "action": "drag",
    "from": "#task-123",
    "to": "#column-done",
    "from_offset": { "x": 10, "y": 10 },
    "to_offset": { "x": 50, "y": 50 },
    "tab_id": 0
  }
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | Must be `"drag"` |
| `from` | string | conditional | CSS selector for the drag source element. Required unless `from_x`/`from_y` are provided. |
| `to` | string | conditional | CSS selector for the drop target element. Required unless `to_x`/`to_y` are provided. |
| `from_x` | number | conditional | X coordinate (viewport-relative) for drag start. Required unless `from` is provided. |
| `from_y` | number | conditional | Y coordinate (viewport-relative) for drag start. Required unless `from` is provided. |
| `to_x` | number | conditional | X coordinate (viewport-relative) for drop target. Required unless `to` is provided. |
| `to_y` | number | conditional | Y coordinate (viewport-relative) for drop target. Required unless `to` is provided. |
| `from_offset` | object | no | `{x, y}` offset from the center of the `from` element. Defaults to `{x: 0, y: 0}` (center). Only applies when `from` selector is used. |
| `to_offset` | object | no | `{x, y}` offset from the center of the `to` element. Defaults to `{x: 0, y: 0}` (center). Only applies when `to` selector is used. |
| `steps` | number | no | Number of intermediate `mousemove`/`dragover` events to synthesize along the drag path. Default: `5`. Higher values improve compatibility with frameworks that track movement granularity. |
| `tab_id` | number | no | Target tab ID (from `observe({what: 'tabs'})`). Omit or `0` for active tab. |

**Validation rules:**
- Either `from` (selector) or both `from_x` and `from_y` (coordinates) must be provided. Not both.
- Either `to` (selector) or both `to_x` and `to_y` (coordinates) must be provided. Not both.
- Mixed mode is allowed: selector for source, coordinates for target, or vice versa.
- `steps` must be >= 1 if provided.

### Response (success)

```json
{
  "status": "queued",
  "correlation_id": "cmd_abc123",
  "message": "Drag command queued. Extension will execute in 1-2s. Poll for result using: observe({what: 'command_result', correlation_id: 'cmd_abc123'})"
}
```

### Async result (polled via `observe({what: 'command_result'})`)

```json
{
  "success": true,
  "from": { "selector": "#task-123", "x": 150, "y": 300, "tag": "DIV", "text": "Fix login bug" },
  "to": { "selector": "#column-done", "x": 600, "y": 300, "tag": "DIV", "text": "Done" },
  "method": "html5_drag_api",
  "events_dispatched": ["mousedown", "dragstart", "dragover", "dragover", "dragover", "drop", "dragend"],
  "duration_ms": 85
}
```

### Async result (failure)

```json
{
  "success": false,
  "error": "element_not_found",
  "message": "Could not find element matching selector '#task-123'",
  "selector": "#task-123"
}
```

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Selector-based drag: resolve `from` and `to` CSS selectors, compute center coordinates, dispatch drag event sequence | must |
| R2 | Coordinate-based drag: accept explicit `from_x`/`from_y` and `to_x`/`to_y` viewport coordinates | must |
| R3 | HTML5 Drag API detection: if the source element has `draggable="true"` or a `dragstart` listener, use `DragEvent` sequence (`mousedown` -> `dragstart` -> `dragover` (N steps) -> `drop` -> `dragend`) | must |
| R4 | Mouse event fallback: if the source element does not use HTML5 Drag API, use `MouseEvent` sequence (`mousedown` -> `mousemove` (N steps) -> `mouseup`) with `pointerId` for pointer event compatibility | must |
| R5 | Synthesize intermediate movement events (`dragover` or `mousemove`) along a linear path between source and target, controlled by `steps` parameter | must |
| R6 | Return structured result including resolved coordinates, element metadata, detection method used, and events dispatched | must |
| R7 | Follow async command pattern: server returns immediately with `correlation_id`, client polls `observe({what: 'command_result'})` | must |
| R8 | Require AI Web Pilot toggle enabled (same security gate as `execute_js` and `highlight`) | must |
| R9 | Support `from_offset` and `to_offset` for precise positioning within elements | should |
| R10 | Include `DataTransfer` object with `effectAllowed` and `dropEffect` properties in synthesized `DragEvent`s | should |
| R11 | Dispatch `PointerEvent`s (`pointerdown`, `pointermove`, `pointerup`) alongside `MouseEvent`s for frameworks that listen on pointer events | should |
| R12 | Configurable `steps` parameter (default 5) to control movement granularity | should |
| R13 | Add timing delays between events (configurable, default ~10ms per step) to satisfy frameworks that reject instantaneous drag sequences | could |
| R14 | Mixed mode: allow selector for source and coordinates for target (or vice versa) | could |

## Non-Goals

- This feature does NOT handle file drag-and-drop from the OS file system into the browser (e.g., dragging a file from Finder/Explorer into a drop zone). That requires OS-level automation outside the browser sandbox.
- This feature does NOT handle cross-tab or cross-window drag operations.
- This feature does NOT handle drag-and-drop within `<canvas>` elements that use custom rendering (no DOM elements to target). Coordinate-based mode can target canvas positions, but cannot introspect canvas-internal state.
- Out of scope: scroll-during-drag (auto-scrolling a container when dragging near its edge). This may be a future enhancement.
- Out of scope: multi-touch or gesture-based drag on mobile/tablet viewports.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Server response time (queue + return) | < 50ms |
| Extension execution time (full drag sequence) | < 500ms |
| Total round-trip (queue -> poll -> result) | < 3s |
| Memory impact of drag handler in inject.js | < 20KB |
| Main thread blocking per event dispatch | < 0.1ms |

## Security Considerations

- **Same security model as `execute_js`:** Drag automation dispatches synthetic DOM events in page context. This is equivalent in power to `execute_js` running `element.dispatchEvent(new DragEvent(...))`. No new attack surface is introduced.
- **AI Web Pilot toggle required:** The drag action is gated behind the same `isAiWebPilotEnabled()` check as all other interact actions. If the toggle is off, the command is rejected with `ai_web_pilot_disabled`.
- **No data exfiltration:** The drag operation reads element positions and dispatches events. It does not capture page content, form data, or cookies beyond what is already exposed by the existing `highlight` and `execute_js` actions.
- **Localhost only:** All communication stays on `127.0.0.1:7890`. No drag data leaves the machine.
- **DataTransfer sanitization:** Synthesized `DataTransfer` objects are empty by default (no `setData` calls). The drag operation does not inject data into the page's clipboard or drag payload unless the page's own `dragstart` handler populates it.

## Edge Cases

- **Source element not found:** If the `from` selector matches no elements, return `{success: false, error: "element_not_found", selector: "..."}`. Same for `to`.
- **Source element not visible:** If the element exists but is hidden (`display: none`, `visibility: hidden`, zero dimensions, or off-screen), return `{success: false, error: "element_not_visible"}` with element details.
- **Multiple matches:** If a selector matches multiple elements, use the first match. Include `matches_count` in the response so the caller knows ambiguity exists.
- **Element removed mid-drag:** If the source element is removed from the DOM between `dragstart` and `drop` (e.g., React re-render), dispatch `dragend` on `document` as cleanup and return `{success: false, error: "element_detached"}`.
- **Drop rejected by target:** Some applications call `event.preventDefault()` on `dragover` to accept drops, or deliberately do not, to reject them. The drag automation dispatches events faithfully; whether the drop "succeeds" from the application's perspective depends on the app's event handlers. The response reports events dispatched, not application-level success.
- **Coordinates out of viewport:** If `from_x`/`from_y` or `to_x`/`to_y` fall outside the visible viewport, return `{success: false, error: "coordinates_out_of_viewport", viewport: {width, height}}`.
- **Extension disconnected:** If the extension is not connected or the tab is closed, the async command times out and the polling returns `{status: "timeout"}` per standard async command behavior.
- **AI Web Pilot disabled:** Return `{error: "ai_web_pilot_disabled"}` immediately, same as `execute_js`.
- **iframe targets:** Elements inside iframes are not reachable by top-level `document.querySelector`. Return `{success: false, error: "element_not_found"}`. Cross-origin iframes are not supported. Same-origin iframe support may be added later.

## Implementation Notes

### Event Sequence: HTML5 Drag API

When the source element has `draggable="true"` or a registered `dragstart` listener:

```
1. mousedown  (on source element)
2. dragstart  (on source element, with DataTransfer)
3. dragover   (on intermediate points, N steps along path)
4. dragover   (on target element)
5. drop       (on target element, with DataTransfer)
6. dragend    (on source element)
```

### Event Sequence: Mouse Events (legacy/framework)

When the source element does not use HTML5 Drag API:

```
1. pointerdown + mousedown  (on source element)
2. pointermove + mousemove  (on intermediate points, N steps along path)
3. pointermove + mousemove  (on target element)
4. pointerup   + mouseup    (on target element)
```

### Detection Heuristic

To determine which event sequence to use:

1. Check if the source element has `draggable` attribute set to `"true"`.
2. Check if `getEventListeners` is available (Chrome DevTools protocol) to detect `dragstart` listeners. In inject.js context, this is not available, so fall back to attribute check.
3. If neither condition is met, default to mouse event sequence.
4. Expose the detection result in the response (`method: "html5_drag_api"` or `method: "mouse_events"`) so the caller can verify.

### Framework Compatibility Notes

| Framework | Drag Implementation | Gasoline Approach |
|-----------|-------------------|-------------------|
| Native HTML5 | `draggable="true"` + DragEvent listeners | HTML5 event sequence (R3) |
| react-dnd (HTML5 backend) | HTML5 Drag API wrapper | HTML5 event sequence (R3) |
| react-dnd (touch backend) | Mouse/touch event listeners | Mouse event sequence (R4) |
| @dnd-kit | Pointer event listeners | Mouse + Pointer event sequence (R4, R11) |
| SortableJS | Mouse event listeners, optional HTML5 fallback | Mouse event sequence (R4); detects `draggable="true"` for HTML5 path |
| react-beautiful-dnd | Mouse/pointer events with timing requirements | Mouse event sequence with steps (R4, R12); may need R13 delays |
| Vanilla mouse drag | mousedown/mousemove/mouseup | Mouse event sequence (R4) |

### Async Command Flow

Follows the same pattern as `execute_js` (v6.0.0):

```
AI agent                    Go server                Extension (background.js)      inject.js
   |                            |                            |                          |
   |-- interact({action:"drag"}) -->|                        |                          |
   |                            |-- create pending query --->|                          |
   |<-- {status:"queued",       |   (type: "drag")           |                          |
   |     correlation_id}        |                            |                          |
   |                            |                 poll /pending-queries                  |
   |                            |<---------------------------|                          |
   |                            |--- return query ---------->|                          |
   |                            |                            |-- GASOLINE_DRAG -------->|
   |                            |                            |   (via content.js)       |
   |                            |                            |                    execute drag
   |                            |                            |<-- GASOLINE_DRAG_RESULT -|
   |                            |<-- POST /execute-result ---|                          |
   |                            |                            |                          |
   |-- observe({what:           |                            |                          |
   |   "command_result"}) ----->|                            |                          |
   |<-- {success: true, ...} ---|                            |                          |
```

### Estimated Implementation Scope

| Component | Estimated Lines | Description |
|-----------|----------------|-------------|
| inject.js | ~120 | Drag execution logic, event synthesis, detection heuristic |
| content.js | ~20 | Message forwarding for `GASOLINE_DRAG` / `GASOLINE_DRAG_RESULT` |
| background.js | ~15 | Pending query dispatch for `type: "drag"` |
| pilot.go | ~80 | `handleDrag` handler, parameter validation, pending query creation |
| tools.go | ~10 | Add `"drag"` to action enum, add drag-specific parameters to schema |
| Tests | ~100 | Go unit tests + extension tests |
| **Total** | **~345** | |

## Dependencies

- Depends on: `interact` tool infrastructure (pilot.go, async command pattern, pending queries)
- Depends on: AI Web Pilot toggle and security gate (`isAiWebPilotEnabled()`)
- Depends on: Extension message passing pipeline (background.js -> content.js -> inject.js)
- Depended on by: None (new leaf feature)

## Assumptions

- A1: The browser extension is connected and actively polling `/pending-queries`.
- A2: AI Web Pilot toggle is enabled in the extension popup.
- A3: The target page is loaded and the DOM elements referenced by selectors exist at the time of execution.
- A4: The drag source and drop target are in the top-level document (not inside a cross-origin iframe).
- A5: Synthetic events dispatched via `dispatchEvent()` are treated equivalently to real user events by the target application. Note: some frameworks may check `event.isTrusted`, which will be `false` for synthetic events. This is a known browser limitation that also affects `execute_js`-based approaches.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | `event.isTrusted` limitation | open | Synthetic events have `isTrusted: false`. Some frameworks (e.g., certain react-beautiful-dnd versions) check this property and may reject synthetic drags. No workaround exists within the extension sandbox. Document as a known limitation. |
| OI-2 | Scroll-during-drag support | open | Auto-scrolling containers when dragging near edges is common UX but adds complexity. Defer to a future enhancement or `execute_js` workaround. |
| OI-3 | `DataTransfer` `files` and `items` population | open | For file upload drop zones, the `DataTransfer` object needs `files` or `items`. Constructing a synthetic `File` object is possible but increases scope. Evaluate whether this belongs in v1 or a follow-up. |
| OI-4 | Timing/delay tuning for framework compatibility | open | react-beautiful-dnd requires a ~120ms delay after mousedown before it recognizes a drag vs click. Default `steps` and timing may need per-framework tuning. Consider exposing a `delay_ms` parameter. |
