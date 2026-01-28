# QA Plan: Local Web Scraping & Automation (LLM-Controlled)

> QA plan for the Local Web Scraping feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

**Note:** No TECH_SPEC.md is available for this feature. This QA plan is based solely on the PRODUCT_SPEC.md. This is explicitly identified as the **highest-risk feature** in Gasoline due to multi-step data extraction from authenticated sessions.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Gasoline runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses -- especially since scraped data from authenticated sessions enters the LLM's context window.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | LLM exfiltrates sensitive data (banking, email, PII) from authenticated sessions | Verify all scraped data stays on localhost (Gasoline server). Document that LLM context window is a trust boundary -- data MAY be sent to LLM provider (cloud). User must accept this risk via AI Web Pilot toggle. | critical |
| DL-2 | Password field values extracted via schema mapping | Verify `input[type="password"]` values are ALWAYS redacted to `"[REDACTED]"` regardless of schema configuration. Test with explicit selector targeting password fields. | critical |
| DL-3 | Sensitive DOM elements extracted despite redaction markers | Verify elements matching `[data-sensitive]`, `[data-private]`, or `.sensitive` are redacted. Test all three markers. | critical |
| DL-4 | Schema extraction accesses cookies, localStorage, or sessionStorage | Verify the schema mapping engine operates on visible DOM content ONLY. No access to `document.cookie`, `localStorage`, `sessionStorage`, or JavaScript global variables. | critical |
| DL-5 | Workflow recordings contain sensitive URLs and selectors | Verify recorded workflows stored in-memory only. No disk persistence in v1. Workflows cleared on server restart. | high |
| DL-6 | Workflow replay against wrong domain leaks data cross-site | Verify domain lock on replay: replaying workflow against different domain requires explicit `allow_domain_change: true` flag. Without flag, replay is denied with warning. | high |
| DL-7 | Scraped data exceeds 1MB and is truncated without warning | Verify truncation to 1MB includes `metadata.truncated: true` flag so LLM knows data is incomplete. | medium |
| DL-8 | Concurrent scraping allows data from different tabs to mix | Verify only one scrape workflow can run per tab. Second request returns `workflow_in_progress` error. | high |
| DL-9 | AI Web Pilot toggle bypass allows scraping without user consent | Verify scrape action ALWAYS requires AI Web Pilot toggle ON. No server-side override. | critical |
| DL-10 | Error recovery step retries leak partial data from failed steps | Verify partial data from retried steps is discarded; only successful step results are included in final output. | medium |
| DL-11 | Pagination scrapes pages the user never intended to visit | Verify `max_pages` cap prevents runaway pagination. Default delay between pages (500ms) prevents rapid automated access. | high |
| DL-12 | Workflow step types (navigate, click) can execute unintended actions on authenticated sessions | Verify scrape requires AI Web Pilot toggle (same as execute_js). All actions execute in user's existing browser context -- no escalation beyond what the user can do manually. | high |

### Negative Tests (must NOT leak)
- [ ] Password field values must ALWAYS return `"[REDACTED]"` -- never the actual password
- [ ] `document.cookie` must NOT be accessible via schema extraction
- [ ] `localStorage` / `sessionStorage` must NOT be accessible via schema extraction
- [ ] Elements with `[data-sensitive]`, `[data-private]`, `.sensitive` must be redacted
- [ ] Workflows must NOT persist to disk (in-memory only, cleared on restart)
- [ ] Domain lock must prevent cross-domain workflow replay without explicit flag
- [ ] Scraping must NOT be possible without AI Web Pilot toggle ON
- [ ] Scraped data must NOT be sent to any external endpoint by Gasoline (LLM provider is user's responsibility)

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Operation types are clearly distinct | `extract`, `workflow`, `replay` are three distinct sub-operations with different schemas. Verify error messages guide to correct operation type. | [ ] |
| CL-2 | Status values are unambiguous | Response `status` is one of: `"complete"`, `"queued"`, `"in_progress"`, `"error"`, `"partial"`. | [ ] |
| CL-3 | Correlation ID enables async polling | `correlation_id` in response is usable with `observe({what: "command_result"})` for progress tracking. | [ ] |
| CL-4 | Schema field extraction is deterministic | Same selector + same page always produces same data. Null for missing elements, not undefined or empty string. | [ ] |
| CL-5 | Error messages are actionable | Errors include `error` type (e.g., `navigation_redirect`, `tab_closed`, `workflow_in_progress`) and enough context for LLM to decide on recovery. | [ ] |
| CL-6 | Metadata includes scraping statistics | `metadata` contains `pages_scraped`, `rows_extracted`, `duration_ms`, `url`, `errors` -- all needed for LLM to assess data quality. | [ ] |
| CL-7 | Workflow progress is observable | During execution, polling returns `steps_completed` / `steps_total` for progress indication. | [ ] |
| CL-8 | Pagination completion is clearly indicated | When pagination stops (no more pages or max reached), metadata shows final page count. | [ ] |
| CL-9 | [REDACTED] placeholder is consistent | All redacted values use exactly `"[REDACTED]"` string. LLM can detect and report redactions. | [ ] |
| CL-10 | Type coercions are documented | Schema supports `text`, `number`, `attr:X`, `html`, `boolean`. Invalid coercion returns null with warning. | [ ] |
| CL-11 | Workflow recording confirmation | When `record: true`, response includes `recorded: true` and `workflow_id` for later replay. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM may assume it can scrape any page without AI Web Pilot toggle. Verify the error message clearly states the requirement.
- [ ] LLM may not understand the trust boundary: Gasoline is localhost but the LLM itself may be cloud-hosted. Verify warnings when scraping authenticated data.
- [ ] LLM may try to extract `document.cookie` via schema `attr:` syntax. Verify this is blocked at the extraction engine level.
- [ ] LLM may chain 50+ workflow steps. Verify a reasonable step limit (50 suggested) with clear error.
- [ ] LLM may not realize `null` in extracted data means element not found (not an error). Verify null vs error distinction is clear.
- [ ] LLM may assume `replay` works identically across different page states. Verify replay with changed page structure returns errors or partial data gracefully.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium to High (multi-step orchestration with security gates)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Single-page data extraction | 1 step: `interact({action: "scrape", operation: "extract", ...})` | No -- already minimal |
| Multi-step workflow | 1 step (workflow defined in single call) | No -- single call encapsulates multiple browser steps |
| Replay recorded workflow | 1 step: `interact({action: "scrape", operation: "replay", workflow_id: "..."})` | No -- minimal |
| First-time setup | User must enable AI Web Pilot toggle + 1 MCP call | Cannot simplify -- security requirement |
| Extract with pagination | 1 step with `paginate` option | No -- pagination config is inline |
| Poll for async workflow progress | Multiple observe calls with correlation_id | Could auto-wait, but async pattern is consistent with other features |

### Default Behavior Verification
- [ ] Scrape action requires AI Web Pilot toggle to be ON (denied otherwise)
- [ ] Extract with no `options` uses sensible defaults (no pagination, 5s timeout)
- [ ] Workflow error_recovery defaults to a reasonable strategy if not specified
- [ ] Schema coercion defaults to `text` if no pipe expression is used
- [ ] Rate limiting between steps defaults to a reasonable delay (500ms)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse extract operation | `{action: "scrape", operation: "extract", selector: "tr", schema: {...}}` | Routes to extract handler | must |
| UT-2 | Parse workflow operation | `{action: "scrape", operation: "workflow", steps: [...]}` | Routes to workflow handler | must |
| UT-3 | Parse replay operation | `{action: "scrape", operation: "replay", workflow_id: "..."}` | Routes to replay handler | must |
| UT-4 | Reject scrape without AI Web Pilot | AI Web Pilot toggle OFF | Error: pilot required for scraping | must |
| UT-5 | Schema field: text coercion | `"td:nth-child(1) \| text"` | Extract text content | must |
| UT-6 | Schema field: number coercion | `"td.points \| text \| number"` | Extract text, parse as number | must |
| UT-7 | Schema field: attr coercion | `"img \| attr:src"` | Extract src attribute value | must |
| UT-8 | Schema field: html coercion | `"div.content \| html"` | Extract innerHTML | must |
| UT-9 | Schema field: boolean coercion | `"input[type=checkbox] \| attr:checked \| boolean"` | Extract as boolean | should |
| UT-10 | Schema field: invalid coercion | `"td \| unknown"` | Null with warning in metadata | must |
| UT-11 | Password field redaction | Schema targeting `input[type="password"]` | Value: `"[REDACTED]"` | must |
| UT-12 | Sensitive element redaction ([data-sensitive]) | Schema targeting `[data-sensitive]` elements | Value: `"[REDACTED]"` | must |
| UT-13 | Sensitive element redaction ([data-private]) | Schema targeting `[data-private]` elements | Value: `"[REDACTED]"` | must |
| UT-14 | Sensitive element redaction (.sensitive) | Schema targeting `.sensitive` elements | Value: `"[REDACTED]"` | must |
| UT-15 | Selector matches zero elements | `selector: ".nonexistent"` | Empty array, rows_extracted: 0, not an error | must |
| UT-16 | Pagination: max_pages cap | `max_pages: 5` with 10 available pages | Only 5 pages scraped | must |
| UT-17 | Pagination: no more pages | Next selector element not found | Pagination stops; data so far returned | must |
| UT-18 | Workflow step: navigate | `{type: "navigate", url: "..."}` | Dispatches navigation command | must |
| UT-19 | Workflow step: wait | `{type: "wait", selector: "...", timeout_ms: 5000}` | Wait for selector with timeout | must |
| UT-20 | Workflow step: click | `{type: "click", selector: "..."}` | Click element | must |
| UT-21 | Workflow step: extract | `{type: "extract", selector: "...", schema: {...}}` | Extract structured data | must |
| UT-22 | Error recovery: retry on timeout | Step timeout + retry strategy | Step retried up to max_retries | must |
| UT-23 | Error recovery: abort with partial | Step failure + abort strategy | Partial results from completed steps returned | must |
| UT-24 | Workflow recording stores definition | `record: true` | Workflow stored in memory with workflow_id | should |
| UT-25 | Workflow replay retrieves definition | Valid workflow_id | Stored workflow replayed | should |
| UT-26 | Workflow replay with override_params | Override step URL | Modified step executed | should |
| UT-27 | Domain lock on replay | Replay against different domain, no flag | Error with domain mismatch warning | should |
| UT-28 | Domain lock with explicit override | `allow_domain_change: true` | Replay proceeds | should |
| UT-29 | Data truncation at 1MB | Extraction exceeds 1MB | Truncated with `metadata.truncated: true` | must |
| UT-30 | Concurrent workflow rejection | Two workflows on same tab | Second gets `workflow_in_progress` error | must |
| UT-31 | Workflow storage LRU eviction | 21st workflow stored (max 20) | Oldest workflow evicted | should |
| UT-32 | Invalid schema field selector | `schema: {field: "[[[bad"}` | Field returns null, warning in metadata | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end single extract | Go server + Extension inject.js + async pipeline | Structured data returned from page table | must |
| IT-2 | End-to-end multi-step workflow | Go server workflow engine + Extension step execution | All steps complete, final extract data returned | must |
| IT-3 | Workflow with error recovery (timeout) | Go server + Extension (simulated slow load) | Step retried, eventually succeeds or returns partial | must |
| IT-4 | AI Web Pilot toggle enforcement | Go server + Extension (toggle OFF) | Scrape denied with actionable error | must |
| IT-5 | Pagination across multiple pages | Go server + Extension (multi-page content) | Data aggregated from all pages; metadata.pages_scraped accurate | should |
| IT-6 | Async polling for workflow progress | Go server + MCP client polling | Progress updates (steps_completed/steps_total) available during execution | should |
| IT-7 | Workflow record and replay | Go server workflow storage + replay handler | Recorded workflow replayed successfully | should |
| IT-8 | Extension disconnect mid-workflow | Go server timeout + partial result handling | Partial data returned; error status with explanation | must |
| IT-9 | Tab closed mid-workflow | Extension tab tracking + workflow engine | Workflow aborts with `tab_closed` error and partial data | must |
| IT-10 | Password field redaction in real page | Extension + inject.js + login form page | Password values always `[REDACTED]` | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Single extract: 500 elements | End-to-end time | < 2s | must |
| PT-2 | Schema mapping per element | Per-element time | < 1ms | must |
| PT-3 | Workflow engine step dispatch | Per-step server overhead | < 50ms | must |
| PT-4 | Step dispatch latency | Extension poll pickup | < 2s | must |
| PT-5 | Full 5-step workflow | Total time | < 30s typical | must |
| PT-6 | Workflow storage memory (20 workflows) | Memory usage | < 200KB | must |
| PT-7 | Workflow results memory (10 results) | Memory usage | < 2MB | must |
| PT-8 | Main thread blocking during extraction | Blocking time | 0ms (all async) | must |
| PT-9 | Pagination with 5 pages, 100 elements each | Total time | < 15s (including delays) | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Selector matches zero elements | `.nonexistent` selector | Empty array, metadata.rows_extracted: 0, not an error | must |
| EC-2 | Page requires login mid-workflow | Navigation redirects to login page | Step fails with `navigation_redirect`; workflow pauses | must |
| EC-3 | Pagination reaches empty page | Next page has no results | Pagination stops; data from previous pages returned | must |
| EC-4 | Extension disconnects mid-workflow | Connection lost | Current step times out; workflow errors with partial data | must |
| EC-5 | Popup/modal blocks workflow step | Expected content obscured by modal | wait_for times out; error recovery applies | should |
| EC-6 | Tab closed during workflow | User closes tab | `tab_closed` error; partial data returned | must |
| EC-7 | Two concurrent workflows on same tab | Two scrape requests | Second gets `workflow_in_progress` error with active ID | must |
| EC-8 | Invalid schema field CSS selector | `schema: {name: "[[[invalid"}` | Field returns null; warning in metadata | must |
| EC-9 | Data exceeds 1MB | Very large page extraction | Truncated to 1MB; `metadata.truncated: true` | must |
| EC-10 | Workflow with 50+ steps | Maximum step count | Error if exceeds limit (50 suggested) | should |
| EC-11 | Replay against different domain | Different domain, no allow flag | Error with domain mismatch | should |
| EC-12 | Replay against different domain with flag | Different domain, `allow_domain_change: true` | Replay proceeds | should |
| EC-13 | Workflow ID not found on replay | Invalid workflow_id | Error: workflow not found | must |
| EC-14 | Server restart clears workflows | Restart server after recording | Recorded workflow unavailable | must |
| EC-15 | Extract from Shadow DOM element | Open shadow root | Depends on A5 assumption; may not discover elements | could |
| EC-16 | Navigate step with slow page load | 10s+ page load | wait_for timeout triggers; error recovery kicks in | should |
| EC-17 | Extract returns only [REDACTED] values | All elements match redaction criteria | All fields `[REDACTED]`; LLM knows extraction was blocked | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome extension installed and connected
- [ ] AI Web Pilot toggle enabled in extension
- [ ] A web page with a data table loaded (e.g., user list, product catalog)
- [ ] Optionally: a page requiring authentication (to test authenticated scraping)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "interact", "arguments": {"action": "scrape", "operation": "extract", "selector": "table tbody tr", "schema": {"col1": "td:nth-child(1) \| text", "col2": "td:nth-child(2) \| text"}}}` | No visible browser changes (extraction is read-only) | Response with `status: "complete"` and structured data array matching table content | [ ] |
| UAT-2 | Verify extracted data matches actual table content | Compare response data to visible table rows | Data is accurate; column values match | [ ] |
| UAT-3 | `{"tool": "interact", "arguments": {"action": "scrape", "operation": "extract", "selector": "input[type='password']", "schema": {"password": ". \| attr:value"}}}` | Password field on page | Password value is `"[REDACTED]"` in response | [ ] |
| UAT-4 | `{"tool": "interact", "arguments": {"action": "scrape", "operation": "extract", "selector": ".nonexistent", "schema": {"text": ". \| text"}}}` | No matching elements | Response with `data: []` and `metadata.rows_extracted: 0` | [ ] |
| UAT-5 | `{"tool": "interact", "arguments": {"action": "scrape", "operation": "workflow", "steps": [{"type": "navigate", "url": "https://example.com"}, {"type": "wait", "selector": "body", "timeout_ms": 5000}, {"type": "extract", "selector": "h1", "schema": {"title": ". \| text"}}]}}` | Browser navigates to example.com | Response with workflow completion, extracted h1 title | [ ] |
| UAT-6 | Verify async polling works | Poll with `observe({what: "command_result", correlation_id: "..."})` | Progress updates showing steps_completed / steps_total | [ ] |
| UAT-7 | `{"tool": "interact", "arguments": {"action": "scrape", "operation": "extract", "selector": "table tbody tr", "schema": {"name": "td:nth-child(1) \| text"}, "options": {"paginate": {"next_selector": "a.next-page", "max_pages": 3, "delay_ms": 1000}}}}` | Browser auto-navigates through pages (clicks "Next") | Data aggregated from up to 3 pages; metadata.pages_scraped shows actual count | [ ] |
| UAT-8 | Disable AI Web Pilot toggle, then: `{"tool": "interact", "arguments": {"action": "scrape", "operation": "extract", "selector": "body", "schema": {"text": ". \| text"}}}` | No browser action | Error: AI Web Pilot required for scraping | [ ] |
| UAT-9 | Record a workflow: `{"tool": "interact", "arguments": {"action": "scrape", "operation": "workflow", "record": true, "name": "test_workflow", "steps": [...]}}` (with Pilot re-enabled) | Workflow executes | Response includes `workflow_id` and `recorded: true` | [ ] |
| UAT-10 | Replay: `{"tool": "interact", "arguments": {"action": "scrape", "operation": "replay", "workflow_id": "..."}}` | Workflow re-executes | Response with replayed data; metadata.replayed_from shows original ID | [ ] |
| UAT-11 | Restart server, try replay | Server restarted | Error: workflow not found (cleared on restart) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Password fields always redacted | Extract from a login form with filled password | Value is `"[REDACTED]"` | [ ] |
| DL-UAT-2 | Sensitive elements redacted | Add `data-sensitive` attribute to an element, then extract | Value is `"[REDACTED]"` | [ ] |
| DL-UAT-3 | No cookie access via schema | Attempt schema `". \| attr:data-cookie"` or similar | No access to document.cookie (only DOM attribute values) | [ ] |
| DL-UAT-4 | AI Web Pilot toggle enforced | Toggle OFF + attempt scrape | Denied with clear error | [ ] |
| DL-UAT-5 | Domain lock on replay | Record workflow on site A, replay against site B | Denied with domain mismatch error | [ ] |
| DL-UAT-6 | Workflows cleared on restart | Restart server after recording workflow | Workflow not found | [ ] |
| DL-UAT-7 | No external data transmission | Monitor all network during scrape | No outbound requests by Gasoline; all data localhost | [ ] |
| DL-UAT-8 | Data truncation warns LLM | Extract very large dataset (>1MB) | Response includes `metadata.truncated: true` | [ ] |

### Regression Checks
- [ ] Existing `interact({action: "execute_js"})` still works independently
- [ ] Existing `interact({action: "navigate"})` still works independently
- [ ] AI Web Pilot toggle still controls existing execute_js and navigate actions
- [ ] Other MCP tools (observe, generate, configure) unaffected
- [ ] Extension performance not degraded when scraping is not in use
- [ ] Telemetry capture (errors, network) works normally during scrape operations

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
