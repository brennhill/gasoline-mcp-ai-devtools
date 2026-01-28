# QA Plan: Form Filling Automation

> QA plan for the Form Filling Automation feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Gasoline runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Password values echoed in response | The response must NOT include the value that was set for any field. Verify password field results show `status: "filled"` without the password string. | critical |
| DL-2 | Sensitive field values in response | The response must NOT echo back any field values (SSN, credit card, email). Only field keys, status, and input_type should appear. | critical |
| DL-3 | Field values logged to server JSONL | Verify the Go server does NOT log the `fields` object values to the JSONL log file. Only the field keys and action should be logged. | critical |
| DL-4 | Field values cached in extension memory | Verify the extension does NOT store or cache field values after the fill operation completes. Values are used and immediately discarded. | high |
| DL-5 | Field values in extension debug logs | Verify `debugLog` in background.js/content.js/inject.js does NOT output field values. Only field keys and statuses may be logged. | high |
| DL-6 | XSS via field values | Verify field values are set via `.value` property (safe) and never via `innerHTML`, `document.write`, or `eval`. No HTML parsing of values. | critical |
| DL-7 | Form selector in response | `form_selector: "#registration-form"` is structural data, not sensitive. Acceptable on localhost. | low |
| DL-8 | `resolved_selector` in response | Per-field `resolved_selector` like `[name="username"]` is structural. Verify it does not contain PII. | low |
| DL-9 | Error messages leaking field values | If a field fails, the error message (e.g., `"option_not_found"`) should not echo the value that was attempted. | high |
| DL-10 | Data transmission path | Verify all form data flows only over localhost (127.0.0.1:7890). No external network calls. | critical |
| DL-11 | AI Web Pilot toggle bypass | Verify `fill_form` is gated behind `isAiWebPilotEnabled()`. Toggle off = no form filling. | critical |
| DL-12 | Form submission triggered accidentally | Verify `fill_form` does NOT submit the form. Only fields are populated. No `form.submit()` or submit button click. | high |

### Negative Tests (must NOT leak)
- [ ] Password values do NOT appear anywhere in the MCP response
- [ ] No field values appear in the response (only keys, statuses, and types)
- [ ] No field values appear in the server JSONL log
- [ ] No field values appear in extension debug logs (`debugLog` output)
- [ ] Values are set via `.value` property, NEVER via `innerHTML` or `eval`
- [ ] Form is NOT submitted by `fill_form` (no `form.submit()`, no submit button click)
- [ ] AI Web Pilot toggle OFF prevents all form filling
- [ ] No form data is transmitted to external servers

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Per-field status distinction | LLM can distinguish `filled`, `failed`, and `skipped` statuses and understand each means something different. | [ ] |
| CL-2 | `failed` vs `skipped` semantics | `failed` = could not fill (element not found, wrong type). `skipped` = element found but cannot be changed (disabled, read-only). LLM must treat these differently. | [ ] |
| CL-3 | `resolved_by` field | LLM understands which resolution strategy found the element (name, id, data-testid, aria-label, label text, CSS selector). Useful for debugging. | [ ] |
| CL-4 | `field_not_found` error detail | Error message lists all 6 resolution strategies that were tried. LLM should understand these are the available matching methods. | [ ] |
| CL-5 | Partial success semantics | `success: false` when `filled: 3, failed: 1` -- LLM must understand some fields were filled even though overall status is false. | [ ] |
| CL-6 | Form not found vs field not found | `error: "form_not_found"` (no fields attempted) vs individual `status: "failed"` per field (form found, specific fields missing). | [ ] |
| CL-7 | `hint` field guidance | Hints suggest using `observe` or `query_dom` to inspect. LLM should follow these suggestions. | [ ] |
| CL-8 | `selected_option` for select fields | Response shows which option text was selected. LLM can verify the right option was chosen. | [ ] |
| CL-9 | Checkbox `previous_value` / `new_value` | Response shows state transition. LLM can verify the checkbox changed as expected. | [ ] |
| CL-10 | No echoed values for sensitive fields | LLM should NOT expect to see the actual value in the response. The absence of value in the response is intentional security behavior, not a bug. | [ ] |
| CL-11 | Async command pattern | LLM understands `fill_form` follows the async command pattern with `correlation_id` and polling. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM expects the response to echo back the values it set (e.g., `"value": "testuser"`) and thinks something failed when values are not present -- verify response documentation makes clear values are intentionally omitted
- [ ] LLM assumes `success: false` means NO fields were filled, when actually partial success is possible -- test with mixed success/failure scenario
- [ ] LLM confuses `skipped` (disabled/read-only) with `failed` (not found) -- test with disabled field and verify error reason is clear
- [ ] LLM does not realize form was NOT submitted and expects page to navigate after fill -- verify response does NOT suggest submission occurred
- [ ] LLM tries to fill file inputs and does not understand why it fails -- verify `file` input error message is clear about browser security restriction

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (single call for all fields)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Fill a login form | 1 step: `interact({action: "fill_form", fields: {username: "...", password: "..."}})` + poll | No -- already minimal |
| Fill form with selector | 1 step: add `selector` parameter | No |
| Fill complex form (10+ fields) | 1 step: all fields in single `fields` object | No -- batch operation by design |
| Handle partial failure | 1 step: read per-field results | No -- results are structured |
| Fill form then submit | 2 steps: (1) `fill_form`, (2) `execute_js("form.submit()")` | Could be 1 step with `submit: true` param (OI-1, deferred) |
| Retry failed fields | 2 steps: (1) read which fields failed, (2) call `fill_form` again with corrected identifiers | Could auto-retry with alternate resolution, but explicit retry is clearer |

### Default Behavior Verification
- [ ] Feature works with just `fields` parameter (no `selector` = search entire document)
- [ ] Default `timeout_ms` is 10000 (10 seconds)
- [ ] Default `tab_id` is 0 (currently tracked tab)
- [ ] Field resolution uses 6-step automatic strategy (no manual specification of resolution method)
- [ ] React value tracker override runs unconditionally (works on non-React pages as no-op)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Server validates `fields` is non-empty object | `{action: "fill_form", fields: {}}` | Validation error: `no_fields` | must |
| UT-2 | Server validates `fields` parameter exists | `{action: "fill_form"}` | Validation error | must |
| UT-3 | Server creates pending query with correct type | `{action: "fill_form", fields: {username: "test"}}` | Pending query type `"fill_form"` | must |
| UT-4 | Server checks AI Web Pilot toggle | Toggle disabled | `ai_web_pilot_disabled` error | must |
| UT-5 | Field resolution: by `name` attribute | `fields: {username: "test"}`, form has `<input name="username">` | Resolved by `name`, status `filled` | must |
| UT-6 | Field resolution: by `id` | `fields: {email: "test"}`, form has `<input id="email">` | Resolved by `id` | must |
| UT-7 | Field resolution: by `data-testid` | `fields: {login: "test"}`, form has `<input data-testid="login">` | Resolved by `data-testid` | must |
| UT-8 | Field resolution: by `aria-label` | `fields: {"Email address": "test"}`, form has `<input aria-label="Email address">` | Resolved by `aria-label` | must |
| UT-9 | Field resolution: by label text | `fields: {"Email": "test"}`, form has `<label>Email<input></label>` | Resolved by label text | must |
| UT-10 | Field resolution: by CSS selector | `fields: {"input[type=email]": "test"}` | Resolved by CSS selector | must |
| UT-11 | Field resolution: not found | `fields: {nonexistent: "test"}` | `status: "failed"`, `error: "field_not_found"` with strategies tried | must |
| UT-12 | Text input filling | `<input type="text">` with value "hello" | Value set, `input` and `change` events dispatched | must |
| UT-13 | Email input filling | `<input type="email">` with value "a@b.com" | Value set correctly | must |
| UT-14 | Password input filling | `<input type="password">` with value "secret" | Value set, response does NOT echo "secret" | must |
| UT-15 | Textarea filling | `<textarea>` with value "long text" | Value set, events dispatched | must |
| UT-16 | Select single option by value | `<select>` with `value: "US"` | Option with `value="US"` selected | must |
| UT-17 | Select single option by visible text | `<select>` with `value: "United States"` | Option with text "United States" selected (case-insensitive) | should |
| UT-18 | Select with no matching option | `value: "XX"`, no option matches | `status: "failed"`, `error: "option_not_found"`, available options listed | must |
| UT-19 | Multi-select | `<select multiple>` with `value: ["US", "UK"]` | Both options selected | should |
| UT-20 | Checkbox set to true | `<input type="checkbox">` with value `true` | `checked = true`, change event dispatched | must |
| UT-21 | Checkbox set to false | `<input type="checkbox" checked>` with value `false` | `checked = false`, change event dispatched | must |
| UT-22 | Checkbox with truthy string | `value: "yes"` | Coerced to `true`, checkbox checked | should |
| UT-23 | Checkbox with invalid string | `value: "maybe"` | `status: "failed"`, `error: "invalid_checkbox_value"` | should |
| UT-24 | Radio button selection | Radio group with `value: "developer"` | Correct radio selected | must |
| UT-25 | Radio with no matching value | `value: "astronaut"` | `status: "failed"`, `error: "radio_option_not_found"`, available values listed | must |
| UT-26 | Date input | `<input type="date">` with `value: "1990-05-15"` | Value set in ISO format | must |
| UT-27 | Number input | `<input type="number" min="0" max="100">` with `value: "50"` | Value set, clamped if out of range | should |
| UT-28 | Color input | `<input type="color">` with `value: "#ff0000"` | Value set | should |
| UT-29 | Hidden input | `<input type="hidden">` with `value: "token"` | Value set, no events dispatched | should |
| UT-30 | File input | `<input type="file">` | `status: "failed"`, error about browser security | must |
| UT-31 | Disabled field | `<input disabled>` | `status: "skipped"`, `reason: "Element is disabled"` | must |
| UT-32 | Read-only field | `<input readonly>` | `status: "skipped"`, `reason: "Element is read-only"` | must |
| UT-33 | React value tracker override | React app with controlled input | Value registered by React's state management | must |
| UT-34 | Event sequence: focus -> input -> change -> blur | Any text input | All four events dispatched in order | must |
| UT-35 | Multiple elements match identifier | Two inputs with same name | First visible match used, `note` in result | should |
| UT-36 | Response does not echo values | Fill password and email | No field values in response JSON | must |
| UT-37 | Form not found | `selector: "#nonexistent-form"` | `error: "form_not_found"` with hint | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full form fill round trip | Go server -> background.js -> content.js -> inject.js -> form filling | All fields filled, per-field results returned | must |
| IT-2 | Form selector forwarded | Server passes `selector: "#reg-form"` | Filling scoped to form container | must |
| IT-3 | Async command pattern (correlation_id) | Server queues, extension polls, result posted | Correct result returned on poll | must |
| IT-4 | AI Web Pilot toggle gating | Toggle OFF | `ai_web_pilot_disabled` immediately | must |
| IT-5 | Extension timeout | Extension disconnected | `extension_timeout` error | must |
| IT-6 | Partial success reporting | Mix of valid and invalid field identifiers | Per-field results with mixed statuses | must |
| IT-7 | React app form fill | React SPA with controlled inputs | React state updated correctly, form validation triggers | should |
| IT-8 | Vue app form fill | Vue app with v-model inputs | Vue reactivity picks up changes | should |
| IT-9 | Concurrent fill_form calls | Two forms filled simultaneously | Each gets unique correlation_id, results independent | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | End-to-end latency for 5-field form | Time from MCP call to response | < 3s | must |
| PT-2 | Per-field processing time | Time per field in inject.js | < 20ms | must |
| PT-3 | Total fill execution for 20 fields | inject.js execution time | < 500ms | must |
| PT-4 | Memory impact | Memory during fill operation | < 50KB | should |
| PT-5 | Main thread blocking | Continuous blocking time | < 50ms | must |
| PT-6 | Response payload size for 20 fields | Bytes of JSON response | < 10KB | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Empty fields object | `fields: {}` | `error: "no_fields"` | must |
| EC-2 | Form not found | `selector: "#missing"` | `error: "form_not_found"` with hint | must |
| EC-3 | All fields not found | Three fields, all invalid identifiers | `filled: 0, failed: 3` | must |
| EC-4 | All fields disabled | Three disabled inputs | `filled: 0, skipped: 3` | must |
| EC-5 | Page navigates during fill | Navigation triggered mid-operation | Timeout error, partial results lost | should |
| EC-6 | Form in iframe | Form inside cross-origin iframe | `form_not_found` (cannot access iframe content) | must |
| EC-7 | Select with very many options (1000+) | Large dropdown | Correct option selected, error message lists up to 10 options if not found | should |
| EC-8 | Input with custom Web Component | `<my-input>` with shadow DOM | Fallback: attempt light DOM first, then shadow root | could |
| EC-9 | Contenteditable element | WYSIWYG editor | Not supported, `field_not_found` or `skipped` | should |
| EC-10 | Multiple forms on page, no selector | Two forms with same field names | First form's fields matched | should |
| EC-11 | Field identifier is valid CSS selector | `fields: {"input[type=email]": "test@test.com"}` | Resolved as CSS selector (step 6) | should |
| EC-12 | Unicode field values | `fields: {name: "\u4e16\u754c"}` (CJK characters) | Value set correctly | should |
| EC-13 | Very long field value | `fields: {bio: "a".repeat(100000)}` | Value set, no truncation by Gasoline | could |
| EC-14 | Concurrent fill_form and execute_js | Both in flight simultaneously | Each completes independently | could |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome extension installed and connected
- [ ] AI Web Pilot toggle enabled
- [ ] A test web page loaded with a registration form containing:
  - Text inputs (username, email)
  - Password input
  - Select dropdown (country)
  - Checkbox (agree to terms)
  - Radio buttons (role)
  - Date input (birth date)
  - A disabled input
  - A read-only input
  - A file input
- [ ] Tab is being tracked by the extension

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"username": "testuser", "email": "test@example.com"}}}` | Username and email fields populated | Response: `filled: 2, failed: 0`. Fields show values in browser. | [ ] |
| UAT-2 | Poll for result: `{"tool": "observe", "arguments": {"what": "command_result", "correlation_id": "<id>"}}` | Fields still show values | Per-field results with `status: "filled"`, `resolved_by`, `input_type` | [ ] |
| UAT-3 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"password": "SecurePass123!"}}}` | Password field shows dots (filled) | Response: `filled: 1`. Password value NOT in response. | [ ] |
| UAT-4 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"country": "US"}}}` | Country dropdown shows "United States" | Response: `filled: 1`, `selected_option: "United States"` | [ ] |
| UAT-5 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"agree_terms": true}}}` | Checkbox is checked | Response: `filled: 1`, `previous_value: false`, `new_value: true` | [ ] |
| UAT-6 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"role": "developer"}}}` | "Developer" radio selected | Response: `filled: 1` | [ ] |
| UAT-7 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"birth_date": "1990-05-15"}}}` | Date picker shows 1990-05-15 | Response: `filled: 1` | [ ] |
| UAT-8 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"nonexistent_field": "value"}}}` | No change | Response: `failed: 1`, `error: "field_not_found"` with strategies tried | [ ] |
| UAT-9 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"disabled_input": "value"}}}` | Disabled field unchanged | Response: `skipped: 1`, `reason: "Element is disabled"` | [ ] |
| UAT-10 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"readonly_input": "value"}}}` | Read-only field unchanged | Response: `skipped: 1`, `reason: "Element is read-only"` | [ ] |
| UAT-11 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"file_upload": "test.pdf"}}}` | File input unchanged | Response: `failed: 1`, error about file input security | [ ] |
| UAT-12 | `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#nonexistent-form", "fields": {"username": "test"}}}` | No change | Response: `error: "form_not_found"` with hint | [ ] |
| UAT-13 | Fill all fields in one call: `{"tool": "interact", "arguments": {"action": "fill_form", "selector": "#registration-form", "fields": {"username": "fulltest", "email": "full@test.com", "password": "Pass123!", "country": "US", "agree_terms": true, "birth_date": "1990-01-01", "role": "developer"}}}` | All fillable fields populated | Response: `filled: 7, failed: 0, skipped: 0, success: true` | [ ] |
| UAT-14 | Verify form was NOT submitted | Page has not navigated, no POST request | Form fields are filled but no submission occurred | [ ] |
| UAT-15 | Disable AI Web Pilot, attempt fill: `{"tool": "interact", "arguments": {"action": "fill_form", "fields": {"username": "test"}}}` | No change | `ai_web_pilot_disabled` error | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Password not in response | Fill password field, inspect MCP response | "SecurePass123!" does NOT appear anywhere in response | [ ] |
| DL-UAT-2 | No field values in response | Fill email and username, inspect response | Neither "testuser" nor "test@example.com" appear in response | [ ] |
| DL-UAT-3 | No field values in server logs | Check server JSONL log after fill | No field values logged, only field keys and action | [ ] |
| DL-UAT-4 | All traffic on localhost | Monitor network during fill | Only 127.0.0.1:7890 traffic | [ ] |
| DL-UAT-5 | No form submission | Monitor network for POST request after fill | No form submission request sent | [ ] |
| DL-UAT-6 | Values set via .value, not innerHTML | Inspect extension source / inject.js | All value setting uses `.value` property assignment | [ ] |

### Regression Checks
- [ ] Existing `interact` tool actions (`execute_js`, `highlight`, `drag`, `handle_dialog`) still work
- [ ] Extension message forwarding handles `fill_form` type without affecting other types
- [ ] Page form functionality works normally when Gasoline is installed but no fill commands issued
- [ ] React, Vue, and Angular forms can be filled without framework-specific issues

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
