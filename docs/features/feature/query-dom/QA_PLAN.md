# QA Plan: Query DOM

> QA plan for the Query DOM feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Gasoline runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Password field values exposed via `serializeDOMElement` | Query `input[type=password]` and verify the response does NOT include the `.value` property content. `textContent` is empty for inputs, but the `value` HTML attribute could appear in `attributes`. | critical |
| DL-2 | Hidden auth tokens in DOM attributes | Query elements like `meta[name=csrf-token]` or `input[type=hidden]` and check if token values appear in the `attributes` map. Since all data stays on localhost, this is acceptable but should be documented. | medium |
| DL-3 | Autocomplete data in form inputs | Query `input[autocomplete=cc-number]` and verify the serialized element does not echo back the user-entered credit card number in the response. | high |
| DL-4 | Data attributes containing PII | Query `[data-email]` or `[data-user-id]` and check that data attributes are included. These are structural attributes and exposure is acceptable on localhost, but should be understood. | medium |
| DL-5 | Session cookies or localStorage accessible via query | Verify that `executeDOMQuery` does NOT serialize `document.cookie`, `localStorage`, or `sessionStorage` contents. Only DOM element properties are returned. | critical |
| DL-6 | Response payload transmitted over network | Verify the query response flows only over localhost (127.0.0.1:7890). No external network calls are made. | critical |
| DL-7 | Bounding box data revealing screen layout | Bounding boxes expose pixel positions. On localhost this is fine, but verify no screen resolution or monitor info leaks beyond element coordinates. | low |

### Negative Tests (must NOT leak)
- [ ] Password input `.value` does NOT appear in query response for `input[type=password]`
- [ ] `document.cookie` is NOT accessible through any DOM query
- [ ] `localStorage` / `sessionStorage` contents are NOT serialized
- [ ] No query response data is sent to any external server (only localhost)
- [ ] The response does not include `innerHTML` or full HTML source of queried elements

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Zero matches vs error distinction | `totalMatchCount: 0` with `matches: []` is clearly different from `error: "invalid_selector"`. An LLM should not confuse "no elements found" with "query failed". | [ ] |
| CL-2 | Truncation indicators | When results exceed 50 elements, `totalMatchCount` vs `returnedMatchCount` clearly shows truncation occurred. Verify LLM can understand "2 of 500 returned". | [ ] |
| CL-3 | Visibility semantics | `visible: true` vs `visible: false` -- verify the LLM understands this means CSS visibility, not whether the element is in the viewport. | [ ] |
| CL-4 | Empty attribute values | `disabled: ""` (empty string for boolean attributes) -- verify this is not misinterpreted as "disabled is empty/false". | [ ] |
| CL-5 | Bounding box interpretation | `bboxPixels` with `x`, `y`, `width`, `height` -- verify values are in pixels, viewport-relative, and not confused with percentage-based layouts. | [ ] |
| CL-6 | Text truncation flag | `textTruncated: true` signals the `text` field was cut short. Verify LLM understands the full text exists but was not returned. | [ ] |
| CL-7 | Timeout vs no extension | `extension_timeout` error -- verify the `recovery` and `hint` fields are clear enough for an LLM to suggest actionable next steps. | [ ] |
| CL-8 | Summary field parsing | `summary: "DOM query \"button.submit\": 2 match(es)"` -- verify the summary is parseable and contains the selector and count. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM confuses `visible: false` (CSS hidden) with "element does not exist" -- test by querying a hidden element and verifying the response clearly includes it with `visible: false`
- [ ] LLM misreads `disabled: ""` as "not disabled" because the value is an empty string -- test with a disabled button and verify the attribute is present
- [ ] LLM does not realize results are capped at 50 when `totalMatchCount` is much larger -- test with `selector: "*"` on a page with 1000+ elements
- [ ] LLM confuses `bboxPixels` coordinates with CSS `top`/`left` values -- verify response documentation/field names make the coordinate system clear

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Query a specific element | 1 step: `configure({action: "query_dom", selector: "..."})` | No -- already minimal |
| Query with tab targeting | 1 step: add `tab_id` parameter | No -- single call |
| Handle no-match result | 1 step: read `totalMatchCount: 0` and `hint` field | No -- hint provides guidance |
| Handle timeout | 1 step: read error, retry after checking extension | No -- recovery hint provided |

### Default Behavior Verification
- [ ] Feature works with zero configuration when `tab_id` is omitted (uses tracked tab)
- [ ] Default `maxElementsReturned` is 50 (no need to specify)
- [ ] Default `maxDepthQueried` is 5 (no need to specify)
- [ ] Default `maxTextLength` is 500 (no need to specify)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | `toolQueryDOM` creates pending query with correct type | `{action: "query_dom", selector: "div"}` | Pending query with type `"dom"` and selector param | must |
| UT-2 | `toolQueryDOM` returns timeout error after 10s | No extension response | `{error: "extension_timeout"}` with recovery hint | must |
| UT-3 | `toolQueryDOM` formats successful response | Extension returns 3 matches | `{summary: "DOM query...: 3 match(es)", data: {...}}` | must |
| UT-4 | `toolQueryDOM` handles zero matches | Extension returns empty matches | `{data: {totalMatchCount: 0, hint: "No elements matched..."}}` | must |
| UT-5 | `executeDOMQuery` runs querySelectorAll with given selector | `selector: "button.submit"` | Array of serialized elements | must |
| UT-6 | `executeDOMQuery` caps results at DOM_QUERY_MAX_ELEMENTS | Selector matching 200 elements | Returns 50 elements, `totalMatchCount: 200` | must |
| UT-7 | `executeDOMQuery` catches invalid selector exception | `selector: "[invalid"` | `{error: "invalid_selector", message: "..."}` | must |
| UT-8 | `executeDOMQuery` serializes attributes correctly | Element with `class`, `id`, `data-testid` | All attributes present in `attributes` map | should |
| UT-9 | `executeDOMQuery` computes bounding box | Visible element with known position | `bboxPixels` with correct `x`, `y`, `width`, `height` | should |
| UT-10 | `executeDOMQuery` reports visibility correctly | Hidden element (`display: none`) | `visible: false` | should |
| UT-11 | `executeDOMQuery` truncates long text | Element with 1000+ char textContent | `text` truncated, `textTruncated: true` | should |
| UT-12 | `executeDOMQuery` handles empty selector | `selector: ""` | Structured error (same as invalid selector) | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full message forwarding chain | background.js -> content.js -> inject.js -> executeDOMQuery | Query sent via MCP returns structured DOM data | must |
| IT-2 | Tab targeting via tab_id | Go server -> background.js with specific tab_id | Query executes on correct tab, not the active tab | must |
| IT-3 | Content script not injected | Go server -> background.js -> chrome.tabs.sendMessage fails | Structured error returned, not silent failure | must |
| IT-4 | Concurrent DOM queries | Two queries with different selectors in flight | Both return correct results routed by request ID | must |
| IT-5 | Extension disconnects mid-query | Server pending query, extension goes offline | Timeout error returned after 10s | should |
| IT-6 | Tab closes between query dispatch and execution | Query dispatched, tab closed | Structured error from background.js catch | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | End-to-end latency for simple query | Time from MCP call to response | < 2s typical | must |
| PT-2 | `executeDOMQuery` execution time | JS execution time in inject.js | < 50ms for < 100 elements | must |
| PT-3 | Message forwarding overhead | Time per hop (background -> content -> inject) | < 5ms per hop | should |
| PT-4 | Response payload size | Bytes of JSON response | < 50KB typical, < 200KB max | should |
| PT-5 | Memory impact of pending query map | Memory usage in content.js | < 100KB per entry | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Selector matching thousands of elements | `selector: "*"` on complex page | Capped at 50, `totalMatchCount` reflects true count | must |
| EC-2 | Invalid CSS selector syntax | `selector: "[invalid"` | `{error: "invalid_selector"}` with exception message | must |
| EC-3 | Empty string selector | `selector: ""` | `{error: "invalid_selector"}` | must |
| EC-4 | Content script not on chrome:// page | Query targeting chrome://extensions | Structured error about unsupported page type | must |
| EC-5 | Page navigating during query | Navigation starts after query dispatched | 30s timeout fires, returns timeout error | should |
| EC-6 | Multiple matches with mixed visibility | Some visible, some hidden | All returned with correct `visible` flag | should |
| EC-7 | Element with extremely long attribute values | `data-*` attribute with 10KB value | Attribute included (no attribute truncation currently) | could |
| EC-8 | SVG elements matched by selector | `selector: "svg"` | SVG elements serialized with SVG-specific attributes | could |
| EC-9 | Shadow DOM elements | Selector targeting shadow DOM | Not matched (shadow DOM not traversed) | could |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page loaded with known DOM structure (e.g., a page with buttons, forms, headings, hidden elements)
- [ ] Tab is being tracked by the extension

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "button"}}` | Page has visible buttons | Response with `totalMatchCount` > 0, each match has `tag: "button"`, `text`, `visible`, `attributes`, `bboxPixels` | [ ] |
| UAT-2 | `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "h1"}}` | Page has an h1 heading | Response with heading element, `text` matching visible heading text | [ ] |
| UAT-3 | `{"tool": "configure", "arguments": {"action": "query_dom", "selector": ".nonexistent-class"}}` | No such class on page | `totalMatchCount: 0`, `matches: []`, `hint` field present | [ ] |
| UAT-4 | `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "[[[invalid"}}` | N/A | `error: "invalid_selector"` with descriptive message | [ ] |
| UAT-5 | `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "input[type=password]"}}` | Password field on page with value entered | Response includes the element but does NOT include the typed password value | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "*"}}` | Page has 500+ elements | `totalMatchCount: 500+`, `returnedMatchCount: 50`, confirms truncation | [ ] |
| UAT-7 | `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "div[style*='display: none']"}}` | Hidden div on page | Match returned with `visible: false` | [ ] |
| UAT-8 | `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "button", "tab_id": 99999}}` | No tab with ID 99999 | Error response indicating tab not found or content script not available | [ ] |
| UAT-9 | Close the tracked tab, then: `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "div"}}` | Tab is closed | Error response (timeout or tab not found) | [ ] |
| UAT-10 | Disconnect extension, then: `{"tool": "configure", "arguments": {"action": "query_dom", "selector": "div"}}` | Extension popup closed / disabled | `extension_timeout` error with recovery hint | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Password field value not in response | Enter "secret123" in a password field, query `input[type=password]` | The string "secret123" does NOT appear anywhere in the response | [ ] |
| DL-UAT-2 | Response only on localhost | Monitor network traffic (DevTools Network tab on extension) during query | All traffic goes to 127.0.0.1:7890, no external requests | [ ] |
| DL-UAT-3 | Hidden input tokens visible but on localhost only | Query `input[type=hidden]` on a page with CSRF token | Token value appears in `attributes` but response stays on localhost | [ ] |
| DL-UAT-4 | No cookie data in response | Query any element | `document.cookie` content does not appear | [ ] |

### Regression Checks
- [ ] Existing `configure` tool actions (`capture`, `health`) still work after query_dom is enabled
- [ ] A11y query flow (`observe({what: "accessibility"})`) still functions (shared message bridge)
- [ ] Extension performance is not degraded (page load time unchanged)
- [ ] Multiple rapid queries do not cause memory leaks in content.js pending request map

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
