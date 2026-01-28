---
feature: form-filling
status: proposed
version: null
tool: interact
mode: fill_form
authors: []
created: 2026-01-28
updated: 2026-01-28
---

# Form Filling Automation

> High-level API for AI agents to populate form fields programmatically, with auto-detection of input types, framework-aware event dispatch, and per-field success/failure reporting.

## Problem

AI coding agents frequently need to fill forms during development workflows: testing registration flows, reproducing bugs that require specific form state, or verifying that form validation works correctly after code changes. Today, the only option is `interact({action: "execute_js", script: "..."})`, which requires the AI to:

1. **Write raw DOM manipulation code** for every input type (text inputs, selects, checkboxes, radios, date pickers, textareas).
2. **Know framework-specific event dispatch** -- React ignores native `input` events and requires synthetic event simulation via the React fiber/internal instance. Vue's `v-model` listens on `input` events but needs them dispatched on the correct target. Angular's `ngModel` needs `input` + `change` events plus zone-aware digest cycles.
3. **Handle validation triggers** -- HTML5 constraint validation, custom validators, form-level vs field-level validation all require different event sequences.
4. **Build error handling per field** -- if one field fails (e.g., selector not found, read-only input), the agent has no structured way to know which fields succeeded and which failed.

This is the most common multi-step `execute_js` use case. Every form-filling session requires 10-30 lines of bespoke JavaScript that the AI must generate, and framework reactivity bugs are the #1 source of "I filled the form but the app didn't register it" failures.

A high-level `fill_form` action eliminates this entire class of problems with a declarative API: specify selector + field values, and the extension handles type detection, event dispatch, and per-field reporting.

## Solution

Add `fill_form` as a new action under the existing `interact` tool. The AI provides a form selector (or omits it to target all visible fields) and a `fields` object mapping field identifiers to desired values. The extension:

1. Locates the form container element (or uses `document` as root if no selector).
2. For each field in the `fields` object, resolves the target input element by name, id, `data-testid`, label text, or CSS selector.
3. Auto-detects the input type (`text`, `select`, `checkbox`, `radio`, `textarea`, `date`, `number`, `email`, `tel`, `url`, `color`, `range`, `file`).
4. Sets the value using the appropriate DOM API for each type.
5. Dispatches framework-aware events in the correct order to ensure React, Vue, Angular, and vanilla JS all register the change.
6. Reports per-field results: which fields were filled successfully, which failed, and why.

The entire operation runs in the page context via inject.js (same execution model as `execute_js`), ensuring access to the live DOM and framework internals.

## User Stories

- As an AI coding agent, I want to fill a login form with test credentials so that I can verify authentication works after code changes, without writing raw DOM manipulation JavaScript.
- As an AI coding agent, I want to fill a complex registration form (text fields, dropdowns, checkboxes, date pickers) in a single tool call so that I can efficiently reproduce a bug that requires specific form state.
- As an AI coding agent, I want per-field success/failure reporting so that when a field cannot be filled (e.g., it is disabled, read-only, or the selector is wrong), I know exactly which field failed and why, without having to inspect the page.
- As a developer using Gasoline, I want form filling to work correctly with React/Vue/Angular so that filled values are recognized by the framework's state management and not silently ignored.

## MCP Interface

**Tool:** `interact`
**Action:** `fill_form`

### Request

```json
{
  "tool": "interact",
  "arguments": {
    "action": "fill_form",
    "selector": "#registration-form",
    "fields": {
      "username": "testuser",
      "email": "test@example.com",
      "password": "SecurePass123!",
      "country": "US",
      "agree_terms": true,
      "birth_date": "1990-05-15",
      "role": "developer"
    },
    "tab_id": 0
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | Must be `"fill_form"` |
| `selector` | string | no | CSS selector for the form container. If omitted, searches the entire document for matching fields. |
| `fields` | object | yes | Key-value map of field identifiers to desired values. Keys are resolved by name attribute, id, `data-testid`, `aria-label`, or CSS selector (tried in that order). Values are strings, booleans (for checkboxes), or arrays (for multi-selects). |
| `tab_id` | number | no | Target tab ID. 0 or omitted = currently tracked tab. |
| `timeout_ms` | number | no | Total timeout for the fill operation in ms. Default: 10000. Max: 30000. |

### Field Identifier Resolution Order

For each key in `fields`, the extension resolves the target element by trying these strategies in order, stopping at the first match:

1. **`name` attribute**: `form.querySelector('[name="<key>"]')`
2. **`id` attribute**: `form.querySelector('#<key>')`
3. **`data-testid` attribute**: `form.querySelector('[data-testid="<key>"]')`
4. **`aria-label` attribute**: `form.querySelector('[aria-label="<key>"]')`
5. **Label text**: Find a `<label>` whose `textContent` matches `<key>` (case-insensitive), then resolve its `for` attribute or contained input.
6. **CSS selector**: `form.querySelector('<key>')` -- allows direct CSS selectors as keys (e.g., `"input[type=email]": "test@example.com"`).

### Value Types by Input Type

| Input Type | Expected Value | Behavior |
|------------|---------------|----------|
| `text`, `email`, `tel`, `url`, `search`, `password` | string | Clears existing value, sets new value |
| `number`, `range` | string or number | Converts to string, sets value, clamps to min/max |
| `textarea` | string | Clears existing value, sets new value |
| `select` (single) | string | Selects option by `value`, then by visible text |
| `select` (multiple) | string[] | Selects all matching options |
| `checkbox` | boolean | Sets checked state (true = checked, false = unchecked) |
| `radio` | string | Selects the radio button with matching `value` within the radio group |
| `date`, `datetime-local`, `time`, `month`, `week` | string | Sets value in the appropriate format (ISO 8601) |
| `color` | string | Sets value (hex format, e.g., `"#ff0000"`) |
| `file` | *(not supported)* | Returns error -- file inputs cannot be set programmatically for security reasons |
| `hidden` | string | Sets value directly (no events needed) |

### Response (success -- all fields filled)

```json
{
  "summary": "Form fill result: 5/5 fields filled successfully",
  "data": {
    "success": true,
    "form_selector": "#registration-form",
    "total_fields": 5,
    "filled": 5,
    "failed": 0,
    "skipped": 0,
    "results": [
      {
        "field": "username",
        "status": "filled",
        "input_type": "text",
        "resolved_by": "name",
        "resolved_selector": "[name=\"username\"]"
      },
      {
        "field": "email",
        "status": "filled",
        "input_type": "email",
        "resolved_by": "name",
        "resolved_selector": "[name=\"email\"]"
      },
      {
        "field": "country",
        "status": "filled",
        "input_type": "select-one",
        "resolved_by": "name",
        "resolved_selector": "[name=\"country\"]",
        "selected_option": "United States"
      },
      {
        "field": "agree_terms",
        "status": "filled",
        "input_type": "checkbox",
        "resolved_by": "id",
        "resolved_selector": "#agree_terms",
        "previous_value": false,
        "new_value": true
      },
      {
        "field": "birth_date",
        "status": "filled",
        "input_type": "date",
        "resolved_by": "name",
        "resolved_selector": "[name=\"birth_date\"]"
      }
    ]
  }
}
```

### Response (partial success)

```json
{
  "summary": "Form fill result: 3/5 fields filled, 1 failed, 1 skipped",
  "data": {
    "success": false,
    "form_selector": "#registration-form",
    "total_fields": 5,
    "filled": 3,
    "failed": 1,
    "skipped": 1,
    "results": [
      {
        "field": "username",
        "status": "filled",
        "input_type": "text",
        "resolved_by": "name",
        "resolved_selector": "[name=\"username\"]"
      },
      {
        "field": "nonexistent_field",
        "status": "failed",
        "error": "field_not_found",
        "message": "No element found matching identifier 'nonexistent_field'. Tried: [name], #id, [data-testid], [aria-label], label text, CSS selector."
      },
      {
        "field": "disabled_input",
        "status": "skipped",
        "input_type": "text",
        "resolved_by": "name",
        "resolved_selector": "[name=\"disabled_input\"]",
        "reason": "Element is disabled"
      },
      {
        "field": "email",
        "status": "filled",
        "input_type": "email",
        "resolved_by": "name",
        "resolved_selector": "[name=\"email\"]"
      },
      {
        "field": "country",
        "status": "filled",
        "input_type": "select-one",
        "resolved_by": "name",
        "resolved_selector": "[name=\"country\"]",
        "selected_option": "United States"
      }
    ],
    "hint": "1 field could not be found. Verify the field identifier matches a name, id, data-testid, aria-label, label text, or valid CSS selector on the page."
  }
}
```

### Response (form not found)

```json
{
  "summary": "Form fill failed: form container not found",
  "data": {
    "success": false,
    "error": "form_not_found",
    "form_selector": "#nonexistent-form",
    "message": "No element found matching selector '#nonexistent-form'. Verify the selector is correct and the form is visible on the page.",
    "hint": "Use observe({what: 'page'}) or configure({action: 'query_dom', selector: 'form'}) to inspect available forms."
  }
}
```

### Response (extension timeout)

```json
{
  "error": "extension_timeout",
  "message": "Timeout waiting for form fill result",
  "recovery": "Browser extension didn't respond -- wait a moment and retry",
  "hint": "Check that the browser extension is connected and a page is focused"
}
```

## Framework Reactivity Strategy

The core challenge of form filling is not setting the DOM value -- it is making the framework's state management layer recognize the change. Each framework uses a different event model:

### Event Dispatch Sequence

For each field, after setting the value, the extension dispatches events in this order:

```
1. focus          (FocusEvent)     -- activates the field
2. input          (InputEvent)     -- triggers React/Vue/vanilla handlers
3. change         (Event)          -- triggers Angular/vanilla handlers, select/checkbox handlers
4. blur           (FocusEvent)     -- triggers validation, onBlur handlers
```

### React

React uses synthetic events built on top of native DOM events. React 16+ with Fiber architecture stores internal state via `__reactFiber$` or `__reactInternalInstance$` properties on DOM nodes. Simply setting `.value` and dispatching a native `input` event is insufficient because React's internal value tracker compares the event's value against its cached value and ignores the event if they match.

**Strategy:** Before dispatching the `input` event, override React's internal value tracker by finding the value property descriptor on the element's prototype (`HTMLInputElement.prototype` or `HTMLTextAreaElement.prototype`), calling the native setter via `Object.getOwnPropertyDescriptor(...).set.call(element, newValue)`, and then dispatching the native `input` event. React's event system picks up the native event and the value tracker sees a real change.

### Vue

Vue 2 uses `v-model` which is syntactic sugar for `:value` + `@input` (for text inputs) or `:checked` + `@change` (for checkboxes/radios). Vue 3 uses the same pattern but with the Composition API.

**Strategy:** Set the DOM value, then dispatch native `input` event. Vue's event delegation picks up native events correctly. For Vue 2 with `v-model.lazy`, also dispatch `change`. No special prototype manipulation needed.

### Angular

Angular uses `ngModel` with `ControlValueAccessor` implementations that listen on native `input` and `change` events. Angular's zone.js patches these events, so dispatching them from within the page context (inject.js) means they are already zone-aware.

**Strategy:** Set the DOM value, dispatch `input` event (for text inputs) or `change` event (for selects, checkboxes). Angular's `DefaultValueAccessor` calls `onChange()` from the native `input` event listener. No special handling needed beyond the standard event sequence.

### Vanilla JS / Web Components

Standard `addEventListener('input')` and `addEventListener('change')` handlers.

**Strategy:** The standard event dispatch sequence covers all vanilla JS patterns. For Web Components with Shadow DOM, the field resolution step must use `element.shadowRoot.querySelector()` if the initial query fails on the light DOM.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Add `fill_form` to the `interact` tool's `action` enum in `tools.go` | must |
| R2 | Server-side handler validates `fields` parameter is present and is a non-empty object | must |
| R3 | Extension resolves field identifiers using the 6-step resolution order (name, id, data-testid, aria-label, label text, CSS selector) | must |
| R4 | Extension auto-detects input type and applies the correct value-setting strategy for text, select, checkbox, radio, textarea, date, number, email, and hidden inputs | must |
| R5 | Extension dispatches framework-aware events (focus, input, change, blur) after setting each field value | must |
| R6 | React value tracker override: use native setter from prototype's property descriptor before dispatching input event | must |
| R7 | Per-field result reporting: each field reports status (filled/failed/skipped), input_type, resolved_by, and error details if applicable | must |
| R8 | Form container not found returns a structured error with the selector that was tried and a hint to use observe or query_dom to inspect available forms | must |
| R9 | Disabled and read-only fields are reported as `skipped` (not `failed`) with a clear reason | must |
| R10 | File inputs return a structured error explaining they cannot be set programmatically | must |
| R11 | Select inputs attempt to match by option `value` first, then by option visible text (case-insensitive) | should |
| R12 | Multi-select inputs accept an array of values and select all matching options | should |
| R13 | Radio button filling selects the radio with matching `value` within the same `name` group | should |
| R14 | The `selector` parameter is optional; when omitted, the extension searches the entire document | should |
| R15 | Shadow DOM traversal: if a field is not found in the light DOM, attempt to search within open shadow roots of elements within the form container | could |
| R16 | The response includes `previous_value` and `new_value` for checkbox and radio fields to confirm the state change | could |
| R17 | The async command pattern (correlation_id) is used, consistent with `execute_js` | must |

## Non-Goals

- **This feature does NOT submit the form.** `fill_form` populates fields but does not click submit buttons or trigger form submission. The AI can use `execute_js` to submit after filling, or use a future `click` action. Combining fill + submit creates ambiguity about which step failed.
- **This feature does NOT handle file upload inputs.** Browsers prohibit setting file input values programmatically for security reasons. File upload requires a different approach (e.g., drag-and-drop simulation or native file dialog automation) that is out of scope.
- **This feature does NOT handle CAPTCHA or bot-detection fields.** Fields protected by reCAPTCHA, hCaptcha, or similar systems cannot be filled programmatically. These are reported as `skipped` with reason `"captcha_detected"` if identifiable, or simply fail to resolve.
- **This feature does NOT persist filled values across page navigations.** Each `fill_form` call operates on the current DOM state. If the page navigates (e.g., multi-step wizard), the AI must call `fill_form` again for each step.
- **This feature does NOT validate the form.** It fills fields and dispatches events that may trigger the form's own validation, but it does not assert that validation passed. The AI can use `query_dom` or `execute_js` to check validation state after filling.
- **Out of scope: contenteditable and rich text editors.** WYSIWYG editors (TinyMCE, CKEditor, ProseMirror, Draft.js) use non-standard input mechanisms. Supporting them requires editor-specific integration that belongs in a separate feature.

## Performance SLOs

| Metric | Target | Rationale |
|--------|--------|-----------|
| End-to-end latency (MCP call to response) | < 3s for forms with up to 20 fields | Poll latency (~1-2s) + field processing (~10ms per field) + event dispatch (~5ms per field). |
| Per-field processing time | < 20ms per field | Element resolution (~5ms), value setting (~1ms), event dispatch (~10ms), result recording (~1ms). |
| Total form fill execution (inject.js) | < 500ms for 20 fields | Sequential field processing at ~20ms each = ~400ms. Parallelization not needed at this scale. |
| Memory impact (extension) | < 50KB per fill_form invocation | Field results array + event objects. Garbage collected after response. |
| Main thread blocking | < 50ms continuous | Each field is a micro-task. Between fields, yield to the event loop if cumulative time exceeds 50ms. |
| Response payload size | < 10KB for 20 fields | ~500 bytes per field result x 20 fields = ~10KB. |

## Security Considerations

- **AI Web Pilot toggle required.** `fill_form` is an interactive browser control action. It requires the AI Web Pilot toggle to be enabled, same as `execute_js`, `highlight`, and all other `interact` actions. The extension checks the toggle before execution and returns `ai_web_pilot_disabled` if off.
- **Localhost only.** All data flows over localhost. Filled values (including passwords and sensitive form data) never leave the machine.
- **No credential storage.** The extension does not cache, log, or persist the values passed in `fields`. Values are used to set DOM properties and then discarded. They do not appear in the extension's internal debug logs or the server's JSONL log.
- **Password fields.** Password values are set via the standard `.value` property. The response does NOT echo back password values. Password fields report `status: "filled"` without revealing the value in the result.
- **No arbitrary code execution.** Unlike `execute_js`, `fill_form` does not accept or execute arbitrary JavaScript. The extension uses a fixed, auditable code path for each input type. The only dynamic inputs are the CSS selectors (for field resolution) and the values (set via DOM properties, not `eval` or `innerHTML`).
- **XSS prevention.** Field values are set via `.value` (for inputs/textareas) and `.selected` (for options), which are safe DOM property assignments. Values are never inserted via `innerHTML`, `document.write`, or other HTML-parsing APIs.
- **Field value redaction in response.** The response includes the field key and status but does NOT include the value that was set. This prevents sensitive data (passwords, SSNs, credit card numbers) from appearing in MCP tool responses that may be logged by the AI client.

## Edge Cases

- **Form not found.** If the `selector` parameter is provided but no element matches, return a structured error with `error: "form_not_found"`. Expected behavior: immediate error response, no fields attempted.
- **Empty fields object.** If `fields` is an empty object `{}`, return a structured error with `error: "no_fields"`. Expected behavior: `"message": "The fields object is empty. Provide at least one field to fill."`.
- **Field not found.** If a field identifier does not match any element through the 6-step resolution, report `status: "failed"` with `error: "field_not_found"` for that field. Other fields are still attempted.
- **Disabled field.** If the resolved element has the `disabled` attribute, report `status: "skipped"` with `reason: "Element is disabled"`. The value is not set.
- **Read-only field.** If the resolved element has the `readonly` attribute, report `status: "skipped"` with `reason: "Element is read-only"`. The value is not set.
- **Hidden input.** Hidden inputs (`type="hidden"`) can be filled. They are set via `.value` with no event dispatch needed (hidden inputs are not interactive). Status: `"filled"`.
- **Select with no matching option.** If the value does not match any option's `value` or text, report `status: "failed"` with `error: "option_not_found"` and include available options in the error message (up to 10).
- **Checkbox with non-boolean value.** If a boolean is expected but a string is provided, coerce truthy strings ("true", "yes", "1", "on") to `true` and falsy strings ("false", "no", "0", "off", "") to `false`. Any other string reports `status: "failed"` with `error: "invalid_checkbox_value"`.
- **Radio with no matching value.** If no radio button in the group has a matching `value`, report `status: "failed"` with `error: "radio_option_not_found"` and list available values.
- **Multiple elements match a field identifier.** If the resolution finds multiple elements (e.g., two inputs with the same name), use the first visible one. If all are hidden, use the first one. Log a note in the result: `"note": "Multiple elements matched; used first visible match."`.
- **Page navigates during fill.** If the page navigates while fields are being filled, the content script context is destroyed. The server timeout (10s) fires and returns a timeout error. Partially-filled results are lost.
- **Extension not connected.** Handled by the standard `checkPilotReady` mechanism. Returns `extension_timeout` or `ai_web_pilot_disabled` as appropriate.
- **Concurrent fill_form calls.** Each call gets a unique correlation_id. Results are routed independently. No interference between concurrent calls.
- **Iframe forms.** Forms inside iframes are not accessible from the top-level inject.js context due to same-origin policy. If the form selector matches nothing because the form is in an iframe, the response is `form_not_found`. Cross-origin iframe forms are explicitly unsupported.

## Dependencies

- **Depends on:**
  - `interact` tool infrastructure -- action enum, dispatcher, `checkPilotReady`, pilot toggle verification (`tools.go`, `pilot.go`)
  - Async command architecture -- correlation_id generation, `CreatePendingQueryWithClient`, `SetCommandResult`, `observe({what: "command_result"})` polling (`pilot.go`, `main.go`)
  - Extension message forwarding chain -- pending query polling, content script bridge, inject.js execution context (`background.js`, `content.js`, `inject.js`)
  - Tab tracking -- `trackedTabId` resolution for `tab_id` parameter (`background.js`)

- **Depended on by:**
  - Future E2E testing integration -- form filling is a prerequisite for automated test flows
  - Future `click` action -- fill_form + click_submit is a natural composition

## Assumptions

- A1: The browser extension is connected and polling `/pending-queries`.
- A2: The AI Web Pilot toggle is enabled in the extension popup.
- A3: The target tab has a content script injected (standard web page, not chrome:// or extension page).
- A4: The page DOM is loaded and the form is rendered (not behind lazy loading or a pending API call).
- A5: Form fields are standard HTML input elements, select elements, or textarea elements. Non-standard custom inputs (Web Components, rich text editors) are not guaranteed to work.
- A6: For React apps, the React internal fiber structure uses `__reactFiber$` or `__reactInternalInstance$` prefixed properties. React 15 and earlier used different internal structures and are not targeted.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should `fill_form` support a `submit` boolean parameter to optionally trigger form submission after filling? | open | Keeping fill and submit separate is cleaner, but a `submit: true` convenience parameter would reduce tool calls. Risk: ambiguity about which step failed. Recommendation: defer -- agents can compose `fill_form` + `execute_js("document.querySelector('form').submit()")`. |
| OI-2 | Should field resolution support XPath or ARIA role selectors in addition to the 6-step resolution? | open | XPath would help for deeply nested forms without unique attributes. ARIA roles (`role="textbox"`) are becoming more common. Risk: increased complexity in the resolution code. Recommendation: start with the 6-step resolution and add more strategies based on user feedback. |
| OI-3 | Should the event dispatch sequence be configurable per field? | open | Some custom components require non-standard event sequences (e.g., `keydown` + `keypress` + `keyup` before `input`). Making this configurable adds complexity. Recommendation: defer -- the standard sequence covers 95%+ of cases. Use `execute_js` for edge cases. |
| OI-4 | Should `fill_form` detect which framework is in use and adjust its strategy automatically? | open | Auto-detecting React vs Vue vs Angular by checking for global variables (`__REACT_DEVTOOLS_GLOBAL_HOOK__`, `__VUE__`, `ng.probe`) could improve reliability. Risk: framework detection is brittle and version-dependent. Recommendation: use the React value tracker override unconditionally (it is a no-op on non-React pages) and rely on standard events for Vue/Angular. |
| OI-5 | How should contenteditable and rich text editors be handled in the future? | open | WYSIWYG editors (TinyMCE, CKEditor, ProseMirror, Draft.js) cannot be filled via `.value`. They require `execCommand('insertText')` or editor-specific APIs. This is explicitly out of scope for v1 but should be tracked as a follow-up feature. |
