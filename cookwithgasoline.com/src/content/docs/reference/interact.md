---
title: "interact() — Control the Browser"
description: "Complete reference for the interact tool. 35 actions for navigation, DOM manipulation, element discovery, file upload, draw mode, recording, compound actions, content extraction, state management, and JavaScript execution."
---

The `interact` tool gives the AI control over the browser — navigate, click, type, read elements, run JavaScript, upload files, record sessions, manage state, and display narration. **Requires AI Web Pilot to be enabled** in the extension popup.

:::note[Synchronous Mode]
Tools now block until the extension returns a result (up to 15s). Set `background: true` to return immediately with a `correlation_id`.
:::

<!-- Screenshot: Extension popup with AI Web Pilot toggle enabled -->

## Quick Reference

```js
interact({action: "navigate", url: "https://example.com"})
interact({action: "click", selector: "text=Submit"})
interact({action: "type", selector: "label=Email", text: "user@example.com"})
interact({action: "list_interactive"})
interact({action: "screenshot"})  // use observe({what: "screenshot"}) instead
interact({action: "subtitle", text: "Welcome to the demo"})
interact({action: "execute_js", script: "document.title"})
interact({action: "save_state", snapshot_name: "logged-in"})
```

## Composable Parameters

These parameters can be added to **any** action:

| Parameter | Type | Description |
|-----------|------|-------------|
| `subtitle` | string | Narration text displayed at bottom of viewport. Empty string clears. |
| `reason` | string | Why this action is being performed — shown in the action toast |
| `correlation_id` | string | Link this action to a specific error or investigation |
| `analyze` | boolean | Enable performance profiling (returns perf_diff in result) |
| `tab_id` | number | Target a specific tab (omit for active tab) |
| `index` | number | Element index from `list_interactive` results (alternative to selector) |
| `visible_only` | boolean | Only return visible elements (`list_interactive`) |

### Subtitles

<!-- Screenshot: Subtitle overlay at bottom of browser viewport showing narration text -->

Subtitles are persistent narration text displayed at the bottom of the viewport, like closed captions. They stay visible until replaced or cleared.

```js
// Standalone
interact({action: "subtitle", text: "Creating a new project"})

// Composable — narration + action in one call
interact({action: "click", selector: "text=Create",
          subtitle: "One click to start a new project"})

// Clear
interact({action: "subtitle", text: ""})
```

### Action Toasts

<!-- Screenshot: Blue "trying" action toast at top of viewport showing "Click: Submit" -->
<!-- Screenshot: Green "success" action toast at top of viewport -->

When enabled, action toasts show brief notifications at the top of the viewport for each action. The `reason` parameter customizes the toast label:

```js
// Without reason: toast shows "Click: #submit-btn"
interact({action: "click", selector: "#submit-btn"})

// With reason: toast shows "Submit the registration form"
interact({action: "click", selector: "#submit-btn", reason: "Submit the registration form"})
```

Toasts can be toggled off in the extension popup — useful during demos.

---

## Semantic Selectors

All actions that accept a `selector` parameter support semantic selectors in addition to CSS:

| Syntax | Example | How it works |
|--------|---------|-------------|
| `text=` | `text=Submit` | Finds element whose text content contains "Submit" |
| `role=` | `role=button` | Finds `[role="button"]` |
| `placeholder=` | `placeholder=Email` | Finds `[placeholder="Email"]` |
| `label=` | `label=Username` | Finds label containing text, follows `for` attribute to target |
| `aria-label=` | `aria-label=Close` | Finds `[aria-label="Close"]` with starts-with matching |
| CSS | `#submit-btn` | Standard `document.querySelector()` |

Semantic selectors are **resilient to UI changes** — they target meaning, not structure. `text=Submit` still works after a CSS redesign, a framework migration, or a component library upgrade.

The `aria-label=` selector uses starts-with matching, so `aria-label=Send` matches Gmail's `"Send ‪(⌘Enter)‬"`.

### Iframe Targeting

Use the `frame` parameter to target elements inside iframes:

```js
interact({action: "click", selector: "text=Submit", frame: "#payment-iframe"})
interact({action: "type", selector: "label=Card", text: "4242...", frame: 0})
```

| Value | Behavior |
|-------|----------|
| CSS selector | Target iframe matching the selector |
| Number | Target iframe by 0-based index |
| `"all"` | Search all iframes |

---

## Navigation

### navigate

```js
interact({action: "navigate", url: "https://example.com"})
```

Navigates the active tab to the specified URL. Automatically includes `perf_diff` in the async result (before/after performance comparison).

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string (required) | URL to navigate to |
| `include_content` | boolean | Return page content with response (url, title, text_content, vitals) |

### refresh

```js
interact({action: "refresh"})
```

Refreshes the current page. Includes `perf_diff` automatically.

### back / forward

```js
interact({action: "back"})
interact({action: "forward"})
```

Browser history navigation.

### new_tab

```js
interact({action: "new_tab", url: "https://example.com"})
```

Opens a new tab with the specified URL.

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string (required) | URL to open in the new tab |

---

## DOM Primitives

### click

```js
interact({action: "click", selector: "text=Submit"})
interact({action: "click", selector: "#confirm-btn", reason: "Confirm the order"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS or semantic selector |

### type

```js
interact({action: "type", selector: "label=Email", text: "user@example.com"})
interact({action: "type", selector: "placeholder=Search", text: "wireless headphones", clear: true})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS or semantic selector |
| `text` | string | Text to type |
| `clear` | boolean | Clear existing value before typing |

### select

```js
interact({action: "select", selector: "#country", value: "US"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS or semantic selector for `<select>` element |
| `value` | string | Option value to select |

### check

```js
interact({action: "check", selector: "text=I agree to the Terms"})
interact({action: "check", selector: "#newsletter", checked: false})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `selector` | string | — | CSS or semantic selector |
| `checked` | boolean | true | Whether to check or uncheck |

### key_press

```js
interact({action: "key_press", selector: "label=Search", text: "Enter"})
interact({action: "key_press", text: "Escape"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | Element to target (optional — omit for page-level key press) |
| `text` | string | Key name: `Enter`, `Tab`, `Escape`, `Backspace`, `ArrowDown`, `ArrowUp`, `Space` |

### focus

```js
interact({action: "focus", selector: "label=Email"})
```

### scroll_to

```js
interact({action: "scroll_to", selector: "#pricing-section"})
```

### wait_for

Wait for an element to appear on the page. Useful for async content.

```js
interact({action: "wait_for", selector: "text=Dashboard", timeout_ms: 10000})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `selector` | string | — | CSS or semantic selector |
| `timeout_ms` | number | 5000 | Maximum wait time in milliseconds |

---

## Element Reading

### get_text

```js
interact({action: "get_text", selector: ".order-total"})
```

Returns the text content of the matched element.

### get_value

```js
interact({action: "get_value", selector: "input[name='email']"})
```

Returns the current value of a form element.

### get_attribute

```js
interact({action: "get_attribute", selector: "#submit", name: "disabled"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Attribute name to read |

### set_attribute

```js
interact({action: "set_attribute", selector: "#submit", name: "disabled", value: "false"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Attribute name |
| `value` | string | Value to set |

### list_interactive

Discovers all interactive elements on the page — buttons, links, inputs, selects, etc. Returns up to 100 elements with suggested selectors.

```js
interact({action: "list_interactive"})
```

No parameters. Returns an array of elements, each with:

| Field | Description |
|-------|-------------|
| `tag` | Element tag name (`button`, `input`, `a`, etc.) |
| `type` | Input type (`email`, `password`, `text`) — only for `<input>` |
| `selector` | Suggested selector (semantic preferred: `text=`, `aria-label=`, `placeholder=`) |
| `label` | Human-readable label (aria-label > title > placeholder > text content) |
| `role` | ARIA role |
| `visible` | Whether the element is visible on screen |

This is the best way to discover what's clickable on an unfamiliar page. The AI can read the list and choose the right selector.

---

## JavaScript Execution

### execute_js

Run arbitrary JavaScript in the page context. See [How Gasoline Executes Page Scripts](/execute-scripts/) for the full deep dive.

```js
interact({action: "execute_js", script: "document.title"})
interact({action: "execute_js", script: "window.location.href"})
interact({action: "execute_js",
          script: "document.querySelectorAll('li').length",
          world: "main"})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `script` | string | — | JavaScript code to execute |
| `world` | string | `auto` | `auto` (try main, fallback to isolated), `main` (page globals), `isolated` (CSP-proof) |
| `timeout_ms` | number | 5000 | Maximum execution time |

---

## Visual

### highlight

Highlights an element on the page with a visual overlay. Useful for demos and debugging.

```js
interact({action: "highlight", selector: "#error-message"})
interact({action: "highlight", selector: "text=Submit", duration_ms: 3000})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `selector` | string | — | Element to highlight |
| `duration_ms` | number | 5000 | How long to show the highlight |

<!-- Screenshot: Element highlighted with Gasoline's visual overlay -->

### subtitle

Display persistent narration text at the bottom of the viewport.

```js
interact({action: "subtitle", text: "Welcome to the product tour"})
interact({action: "subtitle", text: ""})  // Clear
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `text` | string (required) | Narration text. Empty string clears. |

<!-- Screenshot: Subtitle overlay showing narration text at bottom of viewport -->

---

## State Management

### save_state

Save the current browser state (URL, title, tab) as a named checkpoint.

```js
interact({action: "save_state", snapshot_name: "logged-in"})
```

### load_state

Restore a saved state checkpoint.

```js
interact({action: "load_state", snapshot_name: "logged-in"})
interact({action: "load_state", snapshot_name: "logged-in", include_url: true})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `snapshot_name` | string | — | Name of the saved state |
| `include_url` | boolean | false | Navigate to the saved URL when restoring |

### list_states

```js
interact({action: "list_states"})
```

Returns all saved states with metadata (name, URL, title, saved_at).

### delete_state

```js
interact({action: "delete_state", snapshot_name: "old-checkpoint"})
```

---

## File Upload

### upload

Upload a file via native file dialogs. Works with `<input type="file">` elements and drag-and-drop zones.

```js
interact({action: "upload", selector: "input[type='file']", file_path: "/path/to/report.pdf"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | File input element selector |
| `file_path` | string | Absolute path to the file to upload |
| `name` | string | Filename (alternative to file_path) |
| `api_endpoint` | string | API endpoint for direct upload mode |
| `submit` | boolean | Submit form after upload |

---

## Draw Mode

### draw_mode_start

Activate the visual annotation overlay. Users draw rectangles on the page and type feedback. Press ESC to finish. Retrieve annotations with `analyze({what: "annotations"})`.

```js
interact({action: "draw_mode_start"})
interact({action: "draw_mode_start", session: "checkout-review", wait: true, timeout_ms: 300000})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `session` | string | — | Named session for multi-page annotation review |
| `wait` | boolean | — | Block until the user finishes drawing (press ESC) |
| `timeout_ms` | number | 300000 | Max wait time (only with `wait: true`, max 600000) |

---

## Recording

### record_start

Start video recording of the current tab.

```js
interact({action: "record_start"})
interact({action: "record_start", name: "checkout-flow", audio: "tab", fps: 30})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | string | Auto-generated | Recording name |
| `audio` | string | — | Audio source: `tab`, `mic`, or `both`. Omit for video-only. |
| `fps` | number | 15 | Frames per second (5-60) |

### record_stop

Stop the active video recording.

```js
interact({action: "record_stop"})
interact({action: "record_stop", name: "checkout-flow"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Recording name (if multiple active) |

---

## Clipboard

### paste

Paste text at the currently focused element.

```js
interact({action: "paste", text: "Hello, world!"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `text` | string | Text to paste |

---

## Content Extraction

### get_readable

Extract the page content as readable text (similar to reader mode).

```js
interact({action: "get_readable"})
```

### get_markdown

Extract the page content as markdown.

```js
interact({action: "get_markdown"})
```

---

## Compound Actions

These actions combine multiple steps into a single call, reducing round-trips.

### navigate_and_wait_for

Navigate to a URL and wait for a specific element to appear before returning.

```js
interact({action: "navigate_and_wait_for", url: "https://example.com/dashboard", wait_for: ".dashboard-loaded"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string (required) | URL to navigate to |
| `wait_for` | string | CSS selector to wait for after navigation |
| `timeout_ms` | number | Max wait time for the selector |

### fill_form_and_submit

Fill multiple form fields and submit the form in a single action.

```js
interact({action: "fill_form_and_submit",
          fields: [
            {selector: "label=Email", value: "user@example.com"},
            {selector: "label=Password", value: "password123"}
          ],
          submit_selector: "text=Sign In"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `fields` | array | Form fields: `[{selector, value, index?}]` |
| `submit_selector` | string | CSS or semantic selector for submit button |
| `submit_index` | number | Submit button index from `list_interactive` (alternative to selector) |

### run_a11y_and_export_sarif

Run an accessibility audit and export results as SARIF in one step.

```js
interact({action: "run_a11y_and_export_sarif", save_to: "/tmp/a11y.sarif"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `save_to` | string | File path to save the SARIF report |

---

## Performance Profiling

Add `analyze: true` to any DOM action to get a `perf_diff` in the result — before/after timing comparison with Web Vitals ratings and a verdict.

```js
interact({action: "click", selector: "text=Load Dashboard", analyze: true})
```

The `navigate` and `refresh` actions include `perf_diff` automatically.
