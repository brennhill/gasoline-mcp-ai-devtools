---
feature: dialog-handling
status: proposed
version: null
tool: interact
mode: handle_dialog
authors: []
created: 2026-01-28
updated: 2026-01-28
---

# Dialog Handling

> Intercept and respond to browser dialogs (alert, confirm, prompt, beforeunload) so AI agents can automate workflows without being blocked by modal interrupts.

## Problem

Browser dialogs (`alert()`, `confirm()`, `prompt()`, `beforeunload`) are modal and blocking. When an AI coding agent automates browser interactions via Gasoline's `interact` tool, an unexpected dialog halts all execution. The agent cannot click, navigate, type, or run JavaScript until the dialog is dismissed. There is no way for the agent to even detect that a dialog appeared -- it only sees a timeout.

This creates two distinct pain points:

1. **Unexpected dialogs** -- The agent is automating a flow and hits a dialog it did not anticipate (e.g., a site shows `confirm("Are you sure you want to leave?")` on navigation). The agent's command times out with no explanation.
2. **Expected dialogs** -- The agent knows a dialog will appear (e.g., it is testing a delete button that shows `confirm()`) and needs to pre-configure the response before triggering the action.

Both cases require Gasoline to intercept dialogs before they block and either auto-respond or queue them for AI decision.

## Solution

Add a `handle_dialog` action to the `interact` tool that provides two complementary mechanisms:

1. **Reactive handling** -- `interact({action: "handle_dialog"})` responds to a dialog that is currently pending (queued by the extension after interception).
2. **Proactive configuration** -- `interact({action: "handle_dialog", setup: true, ...})` pre-configures auto-responses for dialogs that will appear in the future, before the triggering action is taken.

### How It Works

The extension overrides `window.alert()`, `window.confirm()`, and `window.prompt()` in inject.js (page context) to intercept calls before they show the native modal. For `beforeunload`, the extension listens via `window.addEventListener('beforeunload')`. Intercepted dialogs are captured into a small queue and reported to the server. The AI agent can then respond or pre-configure responses.

### Why Two Mechanisms

The reactive path handles the "unexpected dialog" case: the dialog is already queued, the agent discovers it (via timeout or `observe`), and dismisses it. The proactive path handles the "expected dialog" case: the agent sets up auto-accept before clicking a delete button that triggers `confirm()`. Both paths use the same `handle_dialog` action with different parameters.

## User Stories

- As an AI coding agent, I want to pre-configure dialog responses before triggering actions so that `confirm()` and `prompt()` dialogs do not block my automation.
- As an AI coding agent, I want to detect and dismiss unexpected dialogs so that I can recover from blocked states without human intervention.
- As a developer using Gasoline, I want to see captured dialog events in the telemetry stream so that I can understand what dialogs appeared during a test run.
- As an AI coding agent, I want to handle `beforeunload` events so that page navigations are not blocked by "Are you sure you want to leave?" prompts.

## MCP Interface

**Tool:** `interact`
**Mode/Action:** `handle_dialog`

### Request: Respond to a Pending Dialog (Reactive)

```json
{
  "tool": "interact",
  "arguments": {
    "action": "handle_dialog",
    "accept": true,
    "text": "user input for prompt dialogs"
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | Must be `"handle_dialog"` |
| `accept` | boolean | no | `true` to accept/OK, `false` to cancel/dismiss. Default: `true` |
| `text` | string | no | Response text for `prompt()` dialogs. Ignored for alert/confirm/beforeunload |
| `tab_id` | integer | no | Target tab. Default: tracked tab |

### Request: Pre-configure Auto-Response (Proactive)

```json
{
  "tool": "interact",
  "arguments": {
    "action": "handle_dialog",
    "setup": true,
    "dialog_type": "confirm",
    "accept": true,
    "text": "optional prompt response",
    "once": true
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | Must be `"handle_dialog"` |
| `setup` | boolean | yes | `true` to configure future auto-response |
| `dialog_type` | string | yes (when setup) | `"alert"`, `"confirm"`, `"prompt"`, `"beforeunload"`, or `"all"` |
| `accept` | boolean | no | `true` to accept, `false` to dismiss. Default: `true` |
| `text` | string | no | Response text for `prompt()` dialogs |
| `once` | boolean | no | `true` = auto-respond to next dialog only, then revert. `false` = persist until cleared. Default: `true` |
| `tab_id` | integer | no | Target tab. Default: tracked tab |

### Request: Clear Auto-Response Configuration

```json
{
  "tool": "interact",
  "arguments": {
    "action": "handle_dialog",
    "setup": true,
    "dialog_type": "all",
    "clear": true
  }
}
```

### Request: Query Pending Dialogs (Check Status)

```json
{
  "tool": "interact",
  "arguments": {
    "action": "handle_dialog",
    "status": true
  }
}
```

### Response: Reactive (dialog dismissed)

```json
{
  "status": "dismissed",
  "dialog": {
    "type": "confirm",
    "message": "Are you sure you want to delete this item?",
    "accepted": true,
    "url": "https://example.com/dashboard",
    "timestamp": "2026-01-28T10:30:00Z"
  }
}
```

### Response: Proactive (setup configured)

```json
{
  "status": "configured",
  "dialog_type": "confirm",
  "accept": true,
  "once": true,
  "message": "Auto-response configured for next confirm dialog. Will accept and revert to default."
}
```

### Response: Status Query

```json
{
  "status": "ok",
  "pending_dialogs": [],
  "auto_responses": [
    {
      "dialog_type": "confirm",
      "accept": true,
      "once": true
    }
  ]
}
```

### Response: No Pending Dialog

```json
{
  "status": "no_pending_dialog",
  "message": "No dialog is currently pending. Use setup=true to pre-configure auto-responses for future dialogs.",
  "auto_responses": []
}
```

## Technical Design

### Dialog Interception (inject.js)

`alert()`, `confirm()`, and `prompt()` are synchronous, blocking native functions. They cannot be intercepted after they fire. The extension must override them in inject.js (page context) before any page code calls them.

```
// Interception flow:
1. inject.js overrides window.alert, window.confirm, window.prompt
2. Override checks for pre-configured auto-response
3. If auto-response exists: return immediately (accept/dismiss/text)
4. If no auto-response: queue dialog info, return default (accept=true for alert, false for confirm/prompt)
5. Post dialog event to content.js via window.postMessage
6. content.js forwards to background.js via chrome.runtime.sendMessage
7. background.js posts to server via /logs endpoint (tagged as dialog event)
```

**Critical constraint:** `alert()`, `confirm()`, `prompt()` are synchronous. The override cannot wait for the server to respond. It must decide immediately: either use a pre-configured auto-response, or use a sensible default and queue the dialog for retroactive awareness.

### beforeunload Handling (inject.js)

`beforeunload` works differently. It fires as an event, not a function call. The browser shows a native "Leave site?" dialog that cannot be suppressed by JavaScript (browser security). However:

- The extension can detect `beforeunload` listeners registered by page code
- The extension can remove those listeners to prevent the dialog entirely
- The extension can listen for the `beforeunload` event itself and report it

```
// beforeunload flow:
1. inject.js monitors addEventListener calls for 'beforeunload'
2. When setup auto-response is configured for 'beforeunload':
   - If accept=true: remove all page-registered beforeunload listeners (prevents dialog)
   - If accept=false: leave listeners in place (dialog will show natively)
3. Report beforeunload events to server for telemetry
```

### Server-Side (Go)

The server needs minimal changes:

1. **Dialog event storage** -- Dialog events arrive as log entries via `/logs` with a `dialog` type tag. They are stored in the existing ring buffer alongside other log entries.
2. **Pending dialog queue** -- A small queue (max 5) of pending dialog events that have not been responded to. Stored in `Capture` struct.
3. **`handle_dialog` dispatch** -- New case in `toolInteract` switch that routes to the handler.
4. **Async command pattern** -- Uses the existing pending query + correlation ID mechanism. The server creates a pending query of type `"dialog"`, the extension picks it up and responds.

### Extension-Side (background.js + inject.js)

1. **inject.js** -- Override `window.alert`, `window.confirm`, `window.prompt`. Monitor `beforeunload` listeners. Check auto-response config. Post dialog events via `window.postMessage`.
2. **content.js** -- Bridge dialog events from inject.js to background.js. Forward dialog response commands from background.js to inject.js.
3. **background.js** -- Handle `dialog` query type from server. Manage auto-response configuration per tab. Post dialog events to server.

### Data Flow

```
Proactive (setup):
  AI calls interact(handle_dialog, setup=true)
  → Server creates pending query (type: "dialog_config")
  → Extension polls, picks up config
  → Extension stores auto-response rule in inject.js via postMessage
  → Next matching dialog auto-responds immediately

Reactive (respond):
  Page calls confirm("Delete?")
  → inject.js override fires, no auto-response configured
  → Returns false (default deny for confirm), queues event
  → Event posted to server as dialog log entry
  → AI notices timeout / observes dialog event
  → AI calls interact(handle_dialog, accept=true)
  → Server queues response (but dialog already returned default)
  → Response is informational only (dialog already dismissed)
```

**Important:** Because `alert`/`confirm`/`prompt` are synchronous, the reactive path cannot change the return value of a dialog that already fired. Reactive handling is primarily for awareness and telemetry. For actual control over dialog return values, the proactive (setup) path must be used.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Override `window.alert()` in inject.js to intercept and report | must |
| R2 | Override `window.confirm()` in inject.js to intercept, auto-respond, and report | must |
| R3 | Override `window.prompt()` in inject.js to intercept, auto-respond with text, and report | must |
| R4 | Support proactive `setup` mode to pre-configure auto-responses before triggering action | must |
| R5 | Support `once` flag for single-use auto-responses that revert after firing | must |
| R6 | Report dialog events as log entries with type `dialog` to the server | must |
| R7 | Support `status` query to check pending dialogs and active auto-responses | must |
| R8 | Monitor `beforeunload` listeners and support suppression via `setup` | should |
| R9 | Support `clear` to remove all auto-response configurations | should |
| R10 | Default behavior (no setup): alert auto-accepts, confirm/prompt auto-deny | should |
| R11 | Include dialog message text and originating URL in telemetry | should |
| R12 | Support `dialog_type: "all"` to configure responses for all dialog types at once | could |
| R13 | Support per-tab auto-response isolation (different configs for different tabs) | could |

## Non-Goals

- This feature does NOT provide visual rendering of dialogs in a custom UI. Dialogs are intercepted and auto-handled programmatically.
- This feature does NOT handle `window.open()` popups. Popup/new-window handling is a separate concern.
- This feature does NOT handle file picker dialogs (`<input type="file">`). File upload dialogs are OS-native and outside extension scope.
- This feature does NOT handle HTTP authentication dialogs (401 challenges). Those are handled at the network level.
- Out of scope: Custom modal dialogs built with HTML/CSS (e.g., React modals). Those are regular DOM elements and can be interacted with via `execute_js`.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Dialog interception latency | < 0.1ms (synchronous override, no async) |
| Auto-response lookup | < 0.05ms (in-memory map check) |
| Dialog event reporting | < 1ms (async postMessage, non-blocking) |
| Memory impact (inject.js) | < 5KB for override code + auto-response config |
| Memory impact (server) | < 10KB for dialog queue (max 5 entries) |

## Security Considerations

- **Dialog message content** -- Dialog messages may contain sensitive information (e.g., "Delete user john@example.com?"). Messages are captured as-is in telemetry. Since all data stays on localhost, this follows existing privacy model.
- **Override detection** -- Page code could detect that `alert`/`confirm`/`prompt` have been overridden (e.g., by checking `alert.toString()`). This is a known tradeoff. Mitigation: use `Object.defineProperty` with appropriate descriptors to minimize detectability.
- **Auto-response persistence** -- Auto-response configs are stored per-tab in extension memory only. They do not survive extension reload or tab close. No persistence to disk.
- **beforeunload suppression** -- Removing `beforeunload` listeners is a security-sensitive action (it can cause data loss if unsaved changes exist). This should only be done via explicit AI request (setup mode), never automatically.
- **No new attack surface** -- Dialog handling only intercepts/responds to dialogs on the currently tracked tab. It does not grant new capabilities beyond what `execute_js` already provides (an agent could already override `window.confirm` via `execute_js`). This feature makes the pattern reliable, reusable, and observable.

## Edge Cases

- **No pending dialog when reactive handle_dialog is called** -- Expected behavior: Return `no_pending_dialog` status with guidance to use `setup` mode.
- **Multiple dialogs fire in rapid succession** -- Expected behavior: Each dialog is intercepted individually. If auto-response is configured with `once: true`, only the first dialog gets the auto-response; subsequent ones use defaults.
- **Dialog fires before setup completes (race condition)** -- Expected behavior: The dialog uses the default behavior (alert: accept, confirm/prompt: deny). The setup takes effect for subsequent dialogs. Mitigation: agent should call `setup` before the action that triggers the dialog.
- **Tab closed while auto-response is configured** -- Expected behavior: Auto-response config is discarded. No cleanup needed since it is in-memory per tab.
- **Extension disconnected when dialog fires** -- Expected behavior: Without the extension's inject.js overrides, native dialogs appear as normal. No interception occurs. This is the graceful degradation path.
- **Page code calls `alert()` in a tight loop** -- Expected behavior: Each call is intercepted and auto-responded. The override returns immediately so no blocking occurs. Dialog events are rate-limited to prevent log flooding (max 10 dialog events per second).
- **`beforeunload` event vs `onbeforeunload` property** -- Expected behavior: Both registration methods are monitored. The extension intercepts `addEventListener('beforeunload', ...)` and `window.onbeforeunload = ...` assignment.
- **`prompt()` called but no text configured** -- Expected behavior: Returns empty string `""` if auto-response is set with `accept: true` but no `text` provided. Returns `null` if auto-response is set with `accept: false`.

## Dependencies

- **Depends on:** inject.js page context (for synchronous override of native dialog functions), content.js bridge (for message passing), background.js (for query handling), async command infrastructure (for server communication)
- **Depended on by:** Any future automation workflow feature that needs to handle dialogs (e.g., form submission automation, navigation flows, testing workflows)

## Assumptions

- A1: The extension's inject.js is injected before any page JavaScript executes (using `document_start` in manifest).
- A2: The tracked tab has a content script loaded (required for message bridge).
- A3: AI Web Pilot toggle is enabled (required for all `interact` tool actions).
- A4: The synchronous nature of `alert`/`confirm`/`prompt` means the override must decide immediately -- it cannot await a server response.
- A5: `beforeunload` native dialog cannot be suppressed by JavaScript alone in modern browsers; only the listener can be removed to prevent it from registering.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Default behavior for unhandled confirm/prompt: deny (false/null) or accept (true)? | open | Deny is safer (prevents accidental destructive actions). Accept is more permissive for automation. Spec proposes: alert=accept, confirm/prompt=deny as defaults. |
| OI-2 | Should dialog events be reported via `observe({what: "dialogs"})` as a new observe mode, or as log entries under `observe({what: "logs"})`? | open | Log entries are simpler (no new mode). Dedicated mode gives better discoverability. Recommend: log entries with `source: "dialog"` filter, defer dedicated mode to later. |
| OI-3 | Rate limiting for dialog event reporting | open | A tight loop calling `alert()` could flood the log buffer. Proposed: max 10 dialog events/sec, aggregate beyond that. |
| OI-4 | Should `setup` configuration survive page navigation within the same tab? | open | inject.js is re-injected on navigation, so config would be lost unless background.js re-sends it. Recommend: background.js re-sends active config on page load. |
