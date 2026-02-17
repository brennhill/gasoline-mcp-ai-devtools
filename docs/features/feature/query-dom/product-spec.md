---
feature: query-dom
status: proposed
version: null
tool: configure
mode: query_dom
authors: []
created: 2026-01-28
updated: 2026-02-16
doc_type: product-spec
feature_id: feature-query-dom
last_reviewed: 2026-02-16
---

# Query DOM

> Execute CSS selector queries against the live page DOM and return structured element data (tag, text, attributes, visibility, bounding box) so AI agents can inspect page structure without screenshots or vision models.

## Problem

AI coding agents frequently need to verify page structure: "Is the login form present?", "How many list items rendered?", "What text does the error banner show?". Today, the `query_dom` action is advertised in the MCP schema under the `configure` tool, but the extension returns `not_implemented`. The server-side handler (`toolQueryDOM` in `queries.go`) is fully built -- it creates a pending query, waits for a result, and reshapes the response for LLM consumption. The extension-side execution function (`executeDOMQuery` in `dom-queries.js`) is also fully built -- it runs `querySelectorAll`, serializes elements with attributes, bounding boxes, computed styles, and children. The only missing piece is the message forwarding chain: `background.js` needs to forward `dom` type queries to `content.js`, which needs to forward them to `inject.js`, which needs to call `executeDOMQuery` and return the result.

This is a wiring problem, not a design problem. All three endpoints (server handler, extension executor) exist; the pipe between them is disconnected.

## Solution

Complete the message forwarding chain for DOM queries by following the exact same pattern already used by the `a11y` query flow:

1. **background.js**: Replace the `not_implemented` stub with a `chrome.tabs.sendMessage` call that forwards the query to content.js (identical to how `a11y` queries are forwarded).
2. **content.js**: Add a `DOM_QUERY` message handler that forwards query params to inject.js via `window.postMessage` and relays the result back (identical pattern to `A11Y_QUERY`).
3. **inject.js**: Add a `GASOLINE_DOM_QUERY` message handler that calls the existing `executeDOMQuery()` function and posts the result back (identical pattern to `GASOLINE_A11Y_QUERY`).

No new server code is required. No new extension library code is required. The existing `toolQueryDOM` (Go) and `executeDOMQuery` (JS) implementations are used as-is.

## User Stories

- As an AI coding agent, I want to query the DOM by CSS selector so that I can verify page structure after making code changes without using a vision model.
- As an AI coding agent, I want to get element attributes, text content, and visibility status so that I can assert correctness of rendered UI.
- As a developer debugging with Gasoline, I want to inspect specific DOM elements by selector so that I can understand what the page looks like without switching to browser DevTools.

## MCP Interface

**Tool:** `configure`
**Action:** `query_dom`

### Request

```json
{
  "tool": "configure",
  "arguments": {
    "action": "query_dom",
    "selector": "button.submit",
    "tab_id": 12345
  }
}
```

#### Parameters:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | Must be `"query_dom"` |
| `selector` | string | yes | CSS selector to query (passed to `querySelectorAll`) |
| `tab_id` | number | no | Target tab ID. If omitted, uses the currently tracked tab |
| `pierce_shadow` | boolean \| "auto" | no | Shadow traversal mode: `true`, `false`, or `"auto"` |

### `pierce_shadow` Semantics

- `true`: traverse open and captured closed shadow roots.
- `false`: light DOM only.
- `"auto"`: background resolves to boolean using active debug intent heuristic.

Active debug intent requires:

1. AI Web Pilot enabled.
2. Target tab is the tracked debug tab.
3. Target tab origin matches tracked tab origin.

### Response (matches found)

```json
{
  "summary": "DOM query \"button.submit\": 2 match(es)",
  "data": {
    "url": "http://localhost:3000/checkout",
    "pageTitle": "Checkout - MyApp",
    "selector": "button.submit",
    "totalMatchCount": 2,
    "returnedMatchCount": 2,
    "maxElementsReturned": 50,
    "maxDepthQueried": 5,
    "maxTextLength": 500,
    "matches": [
      {
        "tag": "button",
        "text": "Place Order",
        "textTruncated": false,
        "visible": true,
        "attributes": {
          "class": "submit btn-primary",
          "type": "submit",
          "data-testid": "place-order-btn"
        },
        "bboxPixels": {
          "x": 320,
          "y": 540,
          "width": 200,
          "height": 44
        }
      },
      {
        "tag": "button",
        "text": "Save Draft",
        "textTruncated": false,
        "visible": true,
        "attributes": {
          "class": "submit btn-secondary",
          "disabled": ""
        },
        "bboxPixels": {
          "x": 320,
          "y": 600,
          "width": 200,
          "height": 44
        }
      }
    ]
  }
}
```

### Response (no matches)

```json
{
  "summary": "DOM query \".nonexistent\": 0 match(es)",
  "data": {
    "url": "http://localhost:3000/checkout",
    "pageTitle": "Checkout - MyApp",
    "selector": ".nonexistent",
    "totalMatchCount": 0,
    "returnedMatchCount": 0,
    "maxElementsReturned": 50,
    "maxDepthQueried": 5,
    "maxTextLength": 500,
    "matches": [],
    "hint": "No elements matched selector \".nonexistent\". Verify the selector is correct and the page has loaded. Try a broader selector like 'div' or '*' to explore the DOM."
  }
}
```

### Response (extension timeout)

```json
{
  "error": "extension_timeout",
  "message": "Timeout waiting for DOM query result",
  "recovery": "Browser extension didn't respond -- wait a moment and retry",
  "hint": "Check that the browser extension is connected and a page is focused"
}
```

## Tool Placement Decision

### Decision: `query_dom` stays under `configure`.

Rationale: While `observe` ("show me something") might seem like a natural fit, `query_dom` is not a passive read of buffered telemetry -- it is an on-demand command that creates a pending query, sends it to the extension, and waits for execution. This active command pattern is consistent with other `configure` actions that interact with the extension (e.g., `capture`, `health`). The `observe` tool reads from server-side ring buffers; `query_dom` triggers real-time extension-side execution. Changing the tool assignment would break existing integrations that already reference `configure` + `query_dom` in the schema.

Additionally, the `query_dom` action is already registered in the `configure` tool's enum in `tools.go` and documented in the MCP tool distribution table. Moving it would be a breaking change with no functional benefit.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | background.js forwards `dom` type pending queries to the content script via `chrome.tabs.sendMessage` with type `DOM_QUERY`, following the same pattern as `a11y` queries | must |
| R2 | content.js handles `DOM_QUERY` messages by forwarding params to inject.js via `window.postMessage` with type `GASOLINE_DOM_QUERY`, and relays the response back to background.js | must |
| R3 | inject.js handles `GASOLINE_DOM_QUERY` messages by calling the existing `executeDOMQuery()` function and posting the result back via `GASOLINE_DOM_QUERY_RESPONSE` | must |
| R4 | The forwarding chain must support the `tab_id` parameter for multi-tab targeting (background.js already resolves tab_id before forwarding) | must |
| R5 | A 30-second timeout in content.js prevents stale pending requests from leaking memory (consistent with a11y, highlight, and execute_js timeouts) | must |
| R6 | If content script is not loaded on the target tab, background.js catches the error and posts a structured error result (not a silent failure) | must |
| R7 | Error responses from inject.js (e.g., invalid selector causing `querySelectorAll` to throw) are caught and returned as structured error objects, not uncaught exceptions | should |
| R8 | The forwarding chain passes all existing `executeDOMQuery` parameters through: `selector`, `include_styles`, `properties`, `include_children`, `max_depth` | should |
| R9 | Extension internal logging (debugLog) records DOM query forwarding events for diagnostics | could |

## Non-Goals

- **This feature does NOT add new query types (XPath, text content search, aria queries).** The scope is strictly completing the CSS selector forwarding chain. Additional query types are a separate feature.
- **This feature does NOT modify the server-side handler (`toolQueryDOM`).** The Go code in `queries.go` is already complete and tested. No server changes are needed.
- **This feature does NOT modify the extension-side query executor (`executeDOMQuery`).** The JS code in `dom-queries.js` is already complete. No changes to serialization logic, element caps, or style handling.
- **This feature does NOT implement DOM fingerprinting.** The `dom-fingerprinting` feature (separate spec) uses a different query type (`dom_fingerprint`) with its own extraction logic. `query_dom` is a raw CSS selector query; fingerprinting is a structured semantic extraction.
- **This feature does NOT add computed style querying by default.** The `include_styles` parameter already exists in `executeDOMQuery` but is not exposed in the MCP schema. Exposing it is a future enhancement.
- **Out of scope: child element serialization parameters.** The `include_children` and `max_depth` parameters exist in `executeDOMQuery` but are not currently exposed through the MCP schema. The server hardcodes defaults.

## Edge Cases

- **Invalid CSS selector**: If the selector is syntactically invalid (e.g., `"[invalid"`), `document.querySelectorAll` throws a `DOMException`. inject.js must catch this and return a structured error with the exception message. Expected behavior: `{ error: "invalid_selector", message: "... is not a valid selector" }`.
- **Empty selector**: If no selector is provided, the server-side handler passes an empty string. `querySelectorAll("")` throws. Expected behavior: same as invalid selector -- a structured error.
- **Massive result set**: Selectors like `"*"` or `"div"` can match thousands of elements. Expected behavior: `executeDOMQuery` already caps at `DOM_QUERY_MAX_ELEMENTS` (50 elements). The response includes `totalMatchCount` vs `returnedMatchCount` so the agent knows results were truncated.
- **Extension not connected**: The server creates a pending query and waits. If no extension polls within the timeout (10s), the server returns a timeout error. Expected behavior: structured error with recovery hint.
- **Tab closed between query creation and execution**: background.js attempts `chrome.tabs.sendMessage` to a closed tab. Expected behavior: `chrome.tabs.sendMessage` throws, background.js catches and posts an error result.
- **Content script not injected**: On pages where content scripts cannot run (chrome:// URLs, extension pages, PDF viewer), `chrome.tabs.sendMessage` fails. Expected behavior: structured error indicating the page type is not supported.
- **Page navigating during query**: If the page navigates after the query is dispatched but before execution completes, the content script context is destroyed. Expected behavior: the 30-second timeout fires and returns a timeout error.
- **Concurrent DOM queries**: Multiple queries can be in flight simultaneously. Each has a unique `requestId` in the content script and a unique `query.id` in the server. Expected behavior: responses are correctly routed by their IDs with no cross-contamination.

## Security and Privacy

- **No new data exposure.** `executeDOMQuery` already exists and is callable from inject.js. This change only connects the existing execution path to the MCP interface.
- **Localhost only.** All data flows over localhost (browser extension to localhost:7890 server). No data leaves the machine.
- **Sensitive input protection.** The `serializeDOMElement` function in `dom-queries.js` serializes all attributes including `value`. For password fields and other sensitive inputs, the text content is the raw DOM `textContent` which does not include input values (input values are in `.value` property, not `.textContent`). However, if `include_children` is used and an input has a `value` attribute in HTML, that attribute will appear. This is acceptable because: (a) the user explicitly opted into DOM querying, (b) data stays on localhost, and (c) the existing `executeDOMQuery` already has this behavior.
- **No new attack surface.** This completes a message forwarding path using established patterns (`chrome.tabs.sendMessage`, `window.postMessage`). The same origin validation (`event.source === window`) and message type discrimination already protect the channel.

## Dependencies

- **Depends on:**
  - `extension/lib/dom-queries.js` -- `executeDOMQuery()` function (already implemented)
  - `cmd/dev-console/queries.go` -- `toolQueryDOM()` handler (already implemented)
  - Async command architecture -- pending query polling, result posting (already implemented)
  - Tab tracking -- `trackedTabId` resolution (already implemented)

- **Depended on by:**
  - `dom-fingerprinting` -- shares the pending query infrastructure and content/inject message bridge pattern. Implementing `query_dom` validates the forwarding chain that `dom_fingerprint` will also use.

## Performance SLOs

| Metric | Target | Rationale |
|--------|--------|-----------|
| End-to-end latency (MCP call to response) | < 2s typical, < 10s max | Server timeout is 10s. Extension polls every 1-2s. DOM query execution is < 50ms. The dominant cost is poll latency. |
| `executeDOMQuery` execution time | < 50ms for selectors matching < 100 elements | `querySelectorAll` is native and fast. Serialization is capped at 50 elements. |
| Message forwarding overhead | < 5ms per hop (background to content to inject and back) | `chrome.tabs.sendMessage` and `window.postMessage` are sub-millisecond. |
| Memory impact (content script) | < 100KB per pending request map entry | Each pending request stores only a resolve callback. The 30s timeout ensures cleanup. |
| Response payload size | < 50KB typical, < 200KB max | 50 elements x ~1KB per serialized element = ~50KB. The server does not add significant overhead. |

## Assumptions

- A1: The browser extension is connected and polling `/pending-queries`.
- A2: The target tab has a content script injected (standard web page, not chrome:// or extension page).
- A3: The page DOM is loaded (at minimum `DOMContentLoaded` has fired).
- A4: The `executeDOMQuery` function in `dom-queries.js` is correct and complete (it has been in the codebase since the dom-queries feature was built).

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should `include_styles`, `include_children`, and `max_depth` be exposed as MCP parameters? | open | These parameters exist in `executeDOMQuery` but are not in the current `configure` tool schema. Adding them increases the tool schema complexity. Recommendation: defer to a follow-up -- the current defaults (no styles, no children, depth 3) cover the primary use case. |
| OI-2 | Should the server detect and surface the `not_implemented` error explicitly? | open | Currently the server treats `{success: false, error: "not_implemented"}` as a valid (empty) query result. After implementation this becomes moot, but during the transition the server could check for `error` field in the extension response and return a clear error. |
