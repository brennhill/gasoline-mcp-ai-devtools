# Gasoline UAT (User Acceptance Testing) Plan

Complete user acceptance testing specification for Gasoline v5+. All tests must pass before release.

---

## Table of Contents

1. [Pre-UAT Quality Gates](#pre-uat-quality-gates)
2. [Demo Setup](#demo-setup)
3. [OBSERVE Tool (20 modes)](#observe-tool-20-modes)
4. [GENERATE Tool (7 formats)](#generate-tool-7-formats)
5. [CONFIGURE Tool (11 actions)](#configure-tool-11-actions)
6. [INTERACT Tool (11 actions)](#interact-tool-11-actions)
7. [P4: AI Web Pilot Feature Tests](#p4-ai-web-pilot-feature-tests)
8. [P5: Nice-to-Have Features](#p5-nice-to-have-features)
9. [Edge Cases: Input Validation](#edge-cases-input-validation)
10. [Edge Cases: Extension Connection](#edge-cases-extension-connection)
11. [Edge Cases: Timeouts & Slow Responses](#edge-cases-timeouts--slow-responses)
12. [Edge Cases: Buffer Limits & Data Volume](#edge-cases-buffer-limits--data-volume)
13. [Edge Cases: Rate Limiting & Circuit Breaker](#edge-cases-rate-limiting--circuit-breaker)
14. [Edge Cases: Execute JS](#edge-cases-execute-js)
15. [Edge Cases: State Capture/Restore](#edge-cases-state-capturestore)
16. [Edge Cases: Query DOM](#edge-cases-query-dom)
17. [Edge Cases: Security & Privacy](#edge-cases-security--privacy)
18. [Edge Cases: Multi-Client Mode](#edge-cases-multi-client-mode)
19. [Edge Cases: User Mistakes](#edge-cases-user-mistakes)
20. [Demo Bug Detection Matrix](#demo-bug-detection-matrix)
21. [Sign-Off](#sign-off)

---

## Pre-UAT Quality Gates

**STOP if any of these fail. Do NOT proceed to UAT.**

### 1. Run All Tests

```bash
go vet ./cmd/dev-console/
make test                             # Go tests
node --test tests/extension/*.test.js # Extension tests
```

### 2. File Headers & Comment Hygiene

Verify every source file has a descriptive header comment:

```bash
# Go files
for f in cmd/dev-console/*.go; do
  [[ "$f" == *_test.go ]] && continue
  head -1 "$f" | grep -q "^//" || echo "MISSING HEADER: $f"
done

# JS files
for f in extension/*.js tests/extension/*.js; do
  grep -q "@fileoverview" "$f" || echo "MISSING HEADER: $f"
done
```

Check for stale version/phase labels and dead endpoint references:

```bash
# No stale version labels in source comments
grep -rn "// .*(v4\|v5\|Phase [0-9])" cmd/dev-console/*.go | grep -v _test.go

# No stale endpoint references
grep -rn "/v4/" cmd/dev-console/ --include="*.go"
```

### 3. Code Quality & Coverage

- [ ] `go vet ./cmd/dev-console/` passes
- [ ] `make test` passes (all Go tests)
- [ ] `node --test tests/extension/*.test.js` passes
- [ ] Test coverage exceeds 90% for public functions
- [ ] All behavior, constraints, and edge cases have tests

### 4. Pre-Release Checklist

- [ ] No version label violations in code comments
- [ ] All file headers present and correct
- [ ] No uncommitted changes or all staged for commit
- [ ] Branch is clean and ready for tag

---

## Demo Setup

> **NOTE:** Tests require the ShopNow demo running at localhost:3000.
> See [gasoline-demos repo](https://github.com/brennhill/gasoline-mcp-ai-devtools-demos).

### Prerequisites

- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] ShopNow demo running: `cd ~/dev/gasoline-demos && npm start`
- [ ] Chrome extension installed and connected
- [ ] Extension shows "Connected" status in popup
- [ ] AI Web Pilot toggle visible in extension popup
- [ ] Open localhost:3000 in Chrome with extension active

### Demo Contains 13 Intentional Bugs

Across network, parsing, WebSocket, security, and UI layers. See [Demo Bug Detection Matrix](#demo-bug-detection-matrix) for verification.

---

## OBSERVE Tool (20 modes)

Each mode reads telemetry captured by the extension. Run tests in order (early tests provide data for later ones).

### 1.1 errors

**What**: Console errors and unhandled exceptions.

**Demo signal**: Products page calls `/api/products` but server serves `/api/v2/products` -- expect 404 errors.

- [ ] `observe {what: "errors"}` returns markdown table with errors
- [ ] Errors include level, message, source, timestamp
- [ ] Product fetch failure appears in output
- [ ] `observe {what: "errors", limit: 1}` returns exactly 1 entry

### 1.2 logs

**What**: All console output (log/warn/info/debug/error).

**Demo signal**: Server logs payment card data to console; various client-side logs.

- [ ] `observe {what: "logs"}` returns markdown table
- [ ] Output includes multiple log levels
- [ ] `observe {what: "logs", limit: 5}` caps results

### 1.3 network

**What**: HTTP requests with URL, method, status, timing.

**Demo signal**: 404 on `/api/products`, 401/429 on `/api/checkout`, 500 on order endpoint.

- [ ] `observe {what: "network"}` returns markdown table
- [ ] Shows URL, Method, Status, Duration columns
- [ ] `observe {what: "network", url: "products"}` filters to product requests
- [ ] `observe {what: "network", status_min: 400}` shows only errors
- [ ] `observe {what: "network", status_min: 400, status_max: 499}` shows only 4xx
- [ ] `observe {what: "network", method: "POST"}` filters by method
- [ ] `observe {what: "network", limit: 3}` caps results

### 1.4 websocket_events

**What**: WebSocket message frames with direction and content.

**Demo signal**: Chat connects to `/ws/chat` (wrong -- server expects `/api/ws/chat`). If fixed, typing indicators every 5s.

- [ ] `observe {what: "websocket_events"}` returns data (or empty message if no WS)
- [ ] `observe {what: "websocket_events", direction: "incoming"}` filters inbound
- [ ] `observe {what: "websocket_events", direction: "outgoing"}` filters outbound
- [ ] `observe {what: "websocket_events", url: "chat"}` filters by URL
- [ ] `observe {what: "websocket_events", limit: 5}` caps results
- [ ] `observe {what: "websocket_events", connection_id: "..."}` filters by connection

### 1.5 websocket_status

**What**: WebSocket connection states and message counts.

- [ ] `observe {what: "websocket_status"}` returns JSON with connections array
- [ ] Shows connection id, url, state, messageCount
- [ ] `observe {what: "websocket_status", url: "chat"}` filters by URL

### 1.6 actions

**What**: Recorded user interactions (clicks, navigation, form input).

- [ ] Navigate and click around the demo site first
- [ ] `observe {what: "actions"}` returns markdown table
- [ ] Shows Type, URL, Selector, Value, Time columns
- [ ] `observe {what: "actions", last_n: 3}` returns last 3 actions
- [ ] `observe {what: "actions", url: "localhost:3000"}` filters by URL

### 1.7 vitals

**What**: Web Vitals -- LCP, CLS, INP, FCP, TTFB.

- [ ] `observe {what: "vitals"}` returns JSON with snapshots array
- [ ] Contains numeric metrics for standard Web Vital keys

### 1.8 page

**What**: Current URL, title, readyState, full HTML.

- [ ] `observe {what: "page"}` returns JSON
- [ ] Contains url, title, readyState fields
- [ ] HTML content is present and matches ShopNow page

### 1.9 tabs

**What**: All open browser tabs.

- [ ] `observe {what: "tabs"}` returns markdown table
- [ ] Shows ID, URL, Title, Active columns
- [ ] At least one tab shows localhost:3000

### 1.10 pilot

**What**: AI Web Pilot toggle status.

- [ ] `observe {what: "pilot"}` returns JSON
- [ ] Shows enabled: true (since we toggled it on)
- [ ] Shows extensionConnected: true
- [ ] Shows source field (extension_poll or similar)

### 1.11 performance

**What**: Formatted performance metrics report.

- [ ] `observe {what: "performance"}` returns text report
- [ ] Contains FCP, LCP, CLS, INP sections

### 1.12 api

**What**: Inferred API schema from observed network traffic.

- [ ] Trigger some network requests first (load page, attempt checkout)
- [ ] `observe {what: "api"}` returns markdown table with Method, Path, Observations
- [ ] `observe {what: "api", format: "openapi_stub"}` returns OpenAPI-style JSON
- [ ] `observe {what: "api", url: "products"}` filters endpoints
- [ ] `observe {what: "api", min_observations: 2}` filters by frequency

### 1.13 accessibility

**What**: WCAG accessibility violations.

- [ ] `observe {what: "accessibility"}` returns markdown table
- [ ] Shows Impact, ID, Description, Nodes columns
- [ ] `observe {what: "accessibility", scope: ".product-grid"}` scopes to selector
- [ ] `observe {what: "accessibility", force_refresh: true}` bypasses cache

### 1.14 changes

**What**: What changed since a named checkpoint.

- [ ] `configure {action: "diff_sessions", session_action: "capture", name: "baseline"}` first
- [ ] Trigger new errors (click around)
- [ ] `observe {what: "changes", checkpoint: "baseline"}` returns diff JSON
- [ ] Shows new errors, new network requests, new actions
- [ ] `observe {what: "changes", severity: "errors_only"}` filters to errors

### 1.15 timeline

**What**: Chronological event timeline.

- [ ] `observe {what: "timeline"}` returns JSON with events array
- [ ] Events have type, timestamp, and data fields
- [ ] `observe {what: "timeline", last_n: 5}` returns last 5 events
- [ ] `observe {what: "timeline", include: ["errors", "network"]}` filters event types

### 1.16 error_clusters

**What**: Deduplicated error groups by signature.

- [ ] Trigger repeated errors (reload page multiple times)
- [ ] `observe {what: "error_clusters"}` returns JSON
- [ ] Shows clusters with signature, count, representative, rootCause

### 1.17 history

**What**: Patterns and anomalies in error history.

- [ ] `observe {what: "history"}` returns JSON
- [ ] Contains patterns and anomalies arrays

### 1.18 security_audit

**What**: Security scan for credentials, PII, headers, cookies.

**Demo signal**: Server exposes _debug with Stripe key, logs card numbers, missing secure cookie flags.

- [ ] `observe {what: "security_audit"}` returns JSON audit results
- [ ] Detects exposed credentials or debug info
- [ ] `observe {what: "security_audit", checks: ["credentials", "pii"]}` runs subset
- [ ] `observe {what: "security_audit", severity_min: "high"}` filters severity

### 1.19 third_party_audit

**What**: Third-party domain analysis.

- [ ] `observe {what: "third_party_audit"}` returns JSON
- [ ] Shows domains, request counts, risk levels
- [ ] `observe {what: "third_party_audit", first_party_origins: ["http://localhost:3000"]}` sets first-party

### 1.20 security_diff

**What**: Security state snapshots and comparison.

- [ ] `observe {what: "security_diff", action: "snapshot", name: "before"}` captures state
- [ ] Trigger changes (load new page)
- [ ] `observe {what: "security_diff", action: "snapshot", name: "after"}` captures again
- [ ] `observe {what: "security_diff", action: "compare", compare_from: "before", compare_to: "after"}` shows diff
- [ ] `observe {what: "security_diff", action: "list"}` shows all snapshots

---

## GENERATE Tool (7 formats)

Each format produces a different output type from captured telemetry.

### 2.1 reproduction

**What**: Playwright script replaying recorded actions.

- [ ] Perform some actions on demo site first
- [ ] `generate {format: "reproduction"}` returns JavaScript code
- [ ] Script includes navigation and user interactions
- [ ] `generate {format: "reproduction", last_n: 5}` limits to last 5 actions

### 2.2 test

**What**: Playwright test with assertions.

- [ ] `generate {format: "test", test_name: "shopnow_smoke"}` returns test code
- [ ] Code includes test assertions
- [ ] `generate {format: "test", assert_no_errors: true}` adds error assertion
- [ ] `generate {format: "test", assert_network: true}` adds network assertions

### 2.3 pr_summary

**What**: GitHub PR description from session data.

- [ ] `generate {format: "pr_summary"}` returns JSON
- [ ] Contains title, body, labels, testEvidence fields

### 2.4 sarif

**What**: SARIF 2.1 security report.

- [ ] `generate {format: "sarif"}` returns SARIF JSON
- [ ] Valid SARIF structure with runs, rules, results
- [ ] `generate {format: "sarif", include_passes: true}` includes passing checks

### 2.5 har

**What**: HAR archive of captured network traffic.

- [ ] `generate {format: "har"}` returns HAR JSON
- [ ] Valid HAR 1.2 structure with log.entries
- [ ] `generate {format: "har", url: "products"}` filters entries
- [ ] `generate {format: "har", status_min: 400}` filters by status

### 2.6 csp

**What**: Content-Security-Policy header generation.

- [ ] `generate {format: "csp"}` returns JSON with policy, directives, metaTag
- [ ] `generate {format: "csp", mode: "strict"}` generates strict policy
- [ ] `generate {format: "csp", mode: "report_only"}` generates report-only

### 2.7 sri

**What**: Subresource Integrity hashes.

- [ ] `generate {format: "sri"}` returns JSON with resources array
- [ ] Each resource has url, integrity, tag fields

---

## CONFIGURE Tool (11 actions)

Server configuration and state management.

### 3.1 health

**What**: Server health and metrics.

- [ ] `configure {action: "health"}` returns JSON
- [ ] Contains server, memory, buffers, rate_limiting, audit, pilot sections
- [ ] Extension shows connected

### 3.2 query_dom

**What**: CSS selector query on live page.

**Demo signal**: Product grid, cart counter, chat toggle are all queryable.

- [ ] `configure {action: "query_dom", selector: ".product-grid"}` returns matches
- [ ] `configure {action: "query_dom", selector: "#cart-count"}` returns cart count element
- [ ] `configure {action: "query_dom", selector: ".nonexistent"}` returns empty/no-match

### 3.3 clear

**What**: Clear all browser console logs.

- [ ] Verify logs exist: `observe {what: "logs"}`
- [ ] `configure {action: "clear"}` succeeds
- [ ] `observe {what: "logs"}` now returns empty/fewer entries

### 3.4 noise_rule

**What**: Filter out recurring noise patterns.

- [ ] `configure {action: "noise_rule", noise_action: "list"}` shows current rules
- [ ] `configure {action: "noise_rule", noise_action: "auto_detect"}` suggests rules
- [ ] `configure {action: "noise_rule", noise_action: "add", rules: [{"pattern": "favicon", "reason": "not relevant"}]}` adds rule
- [ ] `observe {what: "logs"}` no longer includes favicon entries
- [ ] `configure {action: "noise_rule", noise_action: "remove", rule_id: "..."}` removes rule
- [ ] `configure {action: "noise_rule", noise_action: "reset"}` clears all rules

### 3.5 dismiss

**What**: Quick noise dismissal.

- [ ] `configure {action: "dismiss", pattern: "favicon", category: "network"}` dismisses
- [ ] Pattern no longer appears in subsequent observe calls

### 3.6 store

**What**: Persistent key-value storage.

- [ ] `configure {action: "store", store_action: "save", namespace: "test", key: "foo", data: {"bar": 1}}` saves
- [ ] `configure {action: "store", store_action: "load", namespace: "test"}` retrieves
- [ ] `configure {action: "store", store_action: "list"}` lists namespaces
- [ ] `configure {action: "store", store_action: "stats"}` shows usage
- [ ] `configure {action: "store", store_action: "delete", namespace: "test", key: "foo"}` removes

### 3.7 load

**What**: Load session context from disk.

- [ ] `configure {action: "load"}` returns session context

### 3.8 diff_sessions

**What**: Capture and compare full session snapshots.

- [ ] `configure {action: "diff_sessions", session_action: "capture", name: "snap1"}` captures
- [ ] Trigger changes on the page
- [ ] `configure {action: "diff_sessions", session_action: "capture", name: "snap2"}` captures again
- [ ] `configure {action: "diff_sessions", session_action: "compare", compare_a: "snap1", compare_b: "snap2"}` shows diff
- [ ] `configure {action: "diff_sessions", session_action: "list"}` lists snapshots
- [ ] `configure {action: "diff_sessions", session_action: "delete", name: "snap1"}` deletes

### 3.9 validate_api

**What**: Check API response contracts.

- [ ] Trigger several API calls on the demo first
- [ ] `configure {action: "validate_api", api_action: "analyze"}` finds violations
- [ ] `configure {action: "validate_api", api_action: "report"}` generates full report
- [ ] `configure {action: "validate_api", api_action: "clear"}` resets state

### 3.10 audit_log

**What**: View MCP tool call audit trail.

- [ ] Make several MCP tool calls first
- [ ] `configure {action: "audit_log"}` returns recent calls
- [ ] `configure {action: "audit_log", tool_name: "observe"}` filters by tool
- [ ] `configure {action: "audit_log", limit: 3}` caps results

### 3.11 streaming

**What**: Real-time push notifications via MCP.

- [ ] `configure {action: "streaming", streaming_action: "status"}` shows current state
- [ ] `configure {action: "streaming", streaming_action: "enable", events: ["errors", "network_errors"]}` starts
- [ ] Trigger an error on the demo page
- [ ] Verify notification received (or check status shows active subscription)
- [ ] `configure {action: "streaming", streaming_action: "disable"}` stops

---

## INTERACT Tool (11 actions)

> **NOTE:** All browser actions (except state management) require AI Web Pilot enabled.

### 4.1 navigate

- [ ] `interact {action: "navigate", url: "http://localhost:3000"}` goes to demo
- [ ] `observe {what: "page"}` confirms URL changed

### 4.2 refresh

- [ ] `interact {action: "refresh"}` reloads page
- [ ] `observe {what: "network"}` shows fresh requests

### 4.3 back / forward

- [ ] Navigate to a second page first
- [ ] `interact {action: "back"}` goes back
- [ ] `interact {action: "forward"}` goes forward

### 4.4 new_tab

- [ ] `interact {action: "new_tab", url: "http://localhost:3000"}` opens tab
- [ ] `observe {what: "tabs"}` shows new tab

### 4.5 highlight

**What**: Visually highlight a DOM element.

- [ ] `interact {action: "highlight", selector: "#cart-link"}` highlights cart
- [ ] Response includes result confirmation
- [ ] `interact {action: "highlight", selector: ".product-grid", duration_ms: 3000}` custom duration
- [ ] `interact {action: "highlight", selector: ".nonexistent"}` returns appropriate error

### 4.6 execute_js

**What**: Run JavaScript in page context.

- [ ] `interact {action: "execute_js", script: "document.title"}` returns "ShopNow"
- [ ] `interact {action: "execute_js", script: "document.querySelectorAll('.product-card').length"}` returns count
- [ ] `interact {action: "execute_js", script: "window.location.href"}` returns URL
- [ ] `interact {action: "execute_js", script: "throw new Error('test')", timeout_ms: 1000}` handles error

### 4.7 save_state

**What**: Save page state snapshot (localStorage, sessionStorage, cookies).

- [ ] `interact {action: "save_state", snapshot_name: "clean"}` saves state
- [ ] `interact {action: "list_states"}` shows "clean" snapshot

### 4.8 load_state

**What**: Restore page state snapshot.

- [ ] Make changes (add items to cart, etc.)
- [ ] `interact {action: "load_state", snapshot_name: "clean"}` restores state
- [ ] `interact {action: "load_state", snapshot_name: "clean", include_url: true}` restores with URL

### 4.9 list_states

**What**: List all saved state snapshots.

- [ ] `interact {action: "list_states"}` shows all snapshots with metadata

### 4.10 delete_state

**What**: Remove a state snapshot.

- [ ] `interact {action: "delete_state", snapshot_name: "clean"}` removes snapshot
- [ ] `interact {action: "list_states"}` no longer shows "clean"

### 4.11 (reserved)

---

## P4: AI Web Pilot Feature Tests

**Critical Path: Safety Toggle must work correctly before any other pilot features.**

### Safety Toggle (Critical)

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 1 | Toggle defaults to OFF | Fresh install, open popup | "AI Web Pilot" checkbox unchecked |
| 2 | Toggle persists | Enable toggle, close/reopen popup | Toggle remains checked |
| 3 | Toggle blocks highlight | Toggle OFF, call `highlight_element` | Error: "ai_web_pilot_disabled" |
| 4 | Toggle blocks state | Toggle OFF, call `manage_state` | Error: "ai_web_pilot_disabled" |
| 5 | Toggle blocks execute | Toggle OFF, call `execute_javascript` | Error: "ai_web_pilot_disabled" |
| 6 | Toggle enables all | Toggle ON, call any pilot tool | Tool executes successfully |
| 7 | AI cannot enable toggle | No MCP tool can modify toggle state | N/A (verify no such tool exists) |
| 8 | Toggle syncs across tabs | Enable in one tab, check another | Both show enabled |

### highlight_element

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 9 | Basic highlight | Call with valid selector | Red overlay appears on element |
| 10 | Highlight positioning | Scroll page, observe highlight | Overlay follows element position |
| 11 | Auto-remove | Wait for duration | Overlay disappears after timeout |
| 12 | Replace highlight | Highlight A, then B | A removed, B shown |
| 13 | Invalid selector | Call with non-existent selector | Error: "element_not_found" |
| 14 | Return bounds | Call with valid selector | Response includes x, y, width, height |
| 15 | Custom duration | Call with duration_ms: 1000 | Overlay disappears after 1s |

### manage_state (execute_javascript included in 4.7-4.10)

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 16 | Save captures all | Set localStorage, sessionStorage, cookie, save | Snapshot includes all three |
| 17 | Load restores all | Load snapshot | All three storage types restored |
| 18 | Load clears first | Have existing data, load snapshot | Old data replaced, not merged |
| 19 | List shows metadata | Save 2 snapshots, call list | Returns both with url, timestamp, size |
| 20 | Delete removes | Delete snapshot, call list | Deleted snapshot not in list |
| 21 | Round-trip integrity | Save → clear all storage → load | Original values restored exactly |
| 22 | include_url navigation | Save on /page-a, load with include_url:true | Navigates to /page-a |
| 23 | include_url skip | Load with include_url:false | Stays on current page |
| 24 | Large state | Save 1MB of localStorage | Saves and loads without error |

### execute_javascript

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 25 | Simple expression | `1 + 1` | `{ success: true, result: 2 }` |
| 26 | Access window | `window.location.hostname` | Returns current hostname |
| 27 | Access DOM | `document.title` | Returns page title |
| 28 | Object return | `({ a: 1, b: [2, 3] })` | Properly serialized object |
| 29 | Function execution | `(() => 42)()` | `{ result: 42 }` |
| 30 | Error handling | `throw new Error('test')` | `{ success: false, error: 'execution_error', stack: '...' }` |
| 31 | Syntax error | `{{{` | Error response with message |
| 32 | Promise resolution | `Promise.resolve(42)` | `{ result: 42 }` |
| 33 | Promise rejection | `Promise.reject(new Error('fail'))` | Error response |
| 34 | Timeout | Script with infinite loop | Timeout error after 5s |
| 35 | Custom timeout | `timeout_ms: 1000` with 2s delay | Timeout after 1s |
| 36 | Circular reference | Object with circular ref | Serializes with [Circular] marker |
| 37 | DOM node return | `document.body` | Descriptive string, not crash |
| 38 | Redux store | `window.__REDUX_STORE__?.getState()` | Returns store state or undefined |
| 39 | Next.js data | `window.__NEXT_DATA__` | Returns Next.js payload if present |

### Integration Workflow

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 40 | Highlight → Execute → Verify | Highlight → Execute to read value → Verify | AI can see what it's about to interact with |
| 41 | Save state → Execute → Restore | Save state → Execute to modify DOM → Restore | State rollback works after AI changes |
| 42 | Disable mid-session | Disable toggle mid-session → Call tool | Immediately returns disabled error |
| 43 | Navigate → Check change | Navigate → Wait → Check page title changed | browser_action + execute_javascript work together |

---

## P5: Nice-to-Have Features

### Binary Format Detection

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 44 | MessagePack detection | Capture MessagePack body | `binary_format: "messagepack"` in response |
| 45 | Protobuf detection | Capture protobuf body | `binary_format: "protobuf"` |
| 46 | Unknown binary | Capture random binary | No binary_format field or null |
| 47 | Text not detected | Capture JSON/text | No binary_format field |
| 48 | WebSocket binary | Binary WS message | Format detected in WS event |

### Network Body E2E

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 49 | Large body truncation | Fetch 1MB response | Body truncated at limit, no crash |
| 50 | Binary preservation | Fetch binary data | Binary intact, not corrupted |
| 51 | Auth header stripped | Request with Authorization | Header not in captured data |
| 52 | POST body captured | POST with JSON body | Request body in capture |
| 53 | Error body captured | 500 response with body | Error body captured |

### Reproduction Enhancements

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 54 | Screenshot insertion | Generate with include_screenshots:true | Script has screenshot calls |
| 55 | Fixture generation | Generate with generate_fixtures:true | fixtures/ file created |
| 56 | Visual assertions | Generate with visual_assertions:true | toHaveScreenshot calls in script |
| 57 | Options default off | Generate with no options | No screenshots, fixtures, or visual assertions |

---

## Edge Cases: Input Validation

### Missing Required Parameters

- [ ] `observe {}` (no what) returns structured error with valid values hint
- [ ] `interact {}` (no action) returns structured error
- [ ] `generate {}` (no format) returns structured error
- [ ] `configure {}` (no action) returns structured error

### Invalid Mode/Action Values

- [ ] `observe {what: "invalid"}` returns error listing valid values
- [ ] `observe {what: "polling"}` returns unknown-mode error
- [ ] `interact {action: "invalid"}` returns error
- [ ] `generate {format: "invalid"}` returns error
- [ ] `configure {action: "invalid"}` returns error

### Malformed JSON Arguments

- [ ] Tool call with invalid JSON in arguments returns error
- [ ] Wrong parameter types (string where number expected) handled gracefully

### Empty Data Returns Descriptive Messages

- [ ] `observe {what: "websocket_events"}` with no WS returns descriptive text, not empty table
- [ ] `observe {what: "actions"}` before any user interaction returns descriptive text
- [ ] `observe {what: "error_clusters"}` with no errors returns descriptive text
- [ ] `observe {what: "changes", checkpoint: "nonexistent"}` returns meaningful error

---

## Edge Cases: Extension Connection

### Extension Never Connected

- [ ] Stop the extension entirely
- [ ] `interact {action: "execute_js", script: "1"}` returns `extension_timeout` error
- [ ] Error message explains how to verify extension is installed and enabled
- [ ] `observe {what: "pilot"}` shows `extensionConnected: false`
- [ ] `configure {action: "health"}` shows extension disconnected

### Extension Connected But Stale (>3s since last poll)

- [ ] Simulate by pausing extension or heavy CPU load
- [ ] `configure {action: "health"}` reports stale connection
- [ ] Interact commands still attempted with warning
- [ ] Commands may succeed if extension resumes quickly, or timeout after 10s

### Extension Reloaded Mid-Session

- [ ] Reload the extension via chrome://extensions
- [ ] Server detects new session ID in poll header
- [ ] `observe {what: "pilot"}` still shows connected after re-poll
- [ ] Previous pending queries lost (timeout after 10s)
- [ ] New queries work normally after extension re-establishes connection

### AI Web Pilot Toggled OFF

- [ ] Disable AI Web Pilot via extension popup (leave extension running)
- [ ] `interact {action: "execute_js", script: "1"}` returns `pilot_disabled` error
- [ ] Error message explains how to enable it
- [ ] `observe {what: "pilot"}` shows `enabled: false`
- [ ] Non-pilot commands still work: `observe {what: "errors"}`, etc.

### AI Web Pilot Cache Initialization Race

- [ ] After extension reload, immediately send an interact command
- [ ] The pilot-enabled cache may not have initialized yet
- [ ] Server uses most-optimistic view: checks both poll header and settings POST
- [ ] Verify command is attempted (not rejected due to stale cache)

### Tab Targeting Edge Cases

- [ ] `interact {action: "execute_js", script: "1", tab_id: 99999}` with nonexistent tab returns error
- [ ] `interact {action: "execute_js", script: "1", tab_id: 0}` targets active tab (default)
- [ ] Open chrome://settings, try to interact with it -- returns error
- [ ] Close target tab mid-query -- returns error

---

## Edge Cases: Timeouts & Slow Responses

### Extension Query Timeout (10s default)

- [ ] `interact {action: "execute_js", script: "new Promise(r => setTimeout(r, 15000))"}` times out after 10s
- [ ] Returns `extension_timeout` error, not a hang
- [ ] Server-side pending query cleaned up within 60s

### Execute JS with Custom Timeout

- [ ] `interact {action: "execute_js", script: "new Promise(r => setTimeout(r, 3000))", timeout_ms: 1000}` times out after 1s
- [ ] `interact {action: "execute_js", script: "new Promise(r => setTimeout(r, 100))", timeout_ms: 5000}` succeeds

### Accessibility Audit on Large DOM

- [ ] Navigate to a complex page with many elements
- [ ] `observe {what: "accessibility"}` completes (axe-core has 30s timeout)
- [ ] Second call within 30s returns cached result
- [ ] `observe {what: "accessibility", force_refresh: true}` forces fresh audit

### DOM Query on Large Page

- [ ] `configure {action: "query_dom", selector: "*"}` returns max 50 elements (capped)
- [ ] Response includes matchCount showing total matches vs. returned count

---

## Edge Cases: Buffer Limits & Data Volume

### Log Entry Limit (1000 default)

- [ ] Generate >1000 log entries (rapid page reloads or console spam)
- [ ] `observe {what: "logs"}` returns entries -- oldest rotated out
- [ ] No crash or memory spike from high volume

### Pending Query Limit (5 max)

- [ ] Fire 6+ interact commands rapidly without waiting for responses
- [ ] First 5 queued; 6th causes oldest to be silently dropped
- [ ] No error returned for dropped query (it times out)

### Network Body Truncation (8KB request / 16KB response)

- [ ] Trigger a large POST request (>8KB body) via the demo
- [ ] `observe {what: "network", url: "..."}` shows the request
- [ ] Body is truncated and marked accordingly

### WebSocket Event Buffer (500 max)

- [ ] Generate high-volume WebSocket traffic
- [ ] Oldest events evicted when buffer full
- [ ] `observe {what: "websocket_events"}` returns most recent events

### Single Log Entry Size (1MB max)

- [ ] `interact {action: "execute_js", script: "console.log('x'.repeat(2000000))"}` logs a 2MB string
- [ ] Entry exceeding 1MB rejected and not stored
- [ ] Server does not crash

---

## Edge Cases: Rate Limiting & Circuit Breaker

### MCP Tool Call Rate Limit (100/minute)

- [ ] Make >100 rapid tool calls
- [ ] After 100, calls return error with rate limit message
- [ ] After waiting, calls succeed again

### Event Rate Circuit Breaker (1000 events/sec for 5s)

- [ ] Generate extreme event volume (tight loop logging via execute_js)
- [ ] Circuit breaker opens after 5 consecutive seconds over threshold
- [ ] Extension receives HTTP 429 with retry_after_ms
- [ ] `configure {action: "health"}` shows circuit open state
- [ ] After 10s below threshold AND memory <30MB, circuit closes

### Memory Pressure Escalation

- [ ] Generate enough data to push memory over soft limit (20MB)
- [ ] Server evicts oldest 25% from buffers (observe data shrinks)
- [ ] If memory hits hard limit (50MB), circuit opens and 50% evicted
- [ ] If memory hits critical (100MB), all buffers cleared, minimal mode engaged

---

## Edge Cases: Execute JS

### Non-Serializable Return Values

- [ ] `interact {action: "execute_js", script: "document.body"}` returns DOM node as string representation
- [ ] `interact {action: "execute_js", script: "() => {}"}` returns function representation
- [ ] `interact {action: "execute_js", script: "let o = {}; o.self = o; o"}` handles circular reference

### Semicolon Heuristic (Known Limitation)

- [ ] `interact {action: "execute_js", script: "1 + 1"}` returns 2 (single expression, auto-return)
- [ ] `interact {action: "execute_js", script: "const x = 1; return x"}` returns 1
- [ ] `interact {action: "execute_js", script: "'a;b'"}` -- semicolon in string (known false positive)

### CSP Blocking

- [ ] If page has strict CSP disabling eval, `execute_js` returns `csp_blocked` error
- [ ] Error message is clear about the cause

### Error Handling in Scripts

- [ ] `interact {action: "execute_js", script: "throw new Error('boom')"}` returns error object
- [ ] `interact {action: "execute_js", script: "undefined.property"}` returns TypeError
- [ ] `interact {action: "execute_js", script: "fetch('http://nonexistent')"}` -- network error handled

### Async Script Behavior

- [ ] `interact {action: "execute_js", script: "Promise.resolve(42)"}` returns 42 (promise auto-awaited)
- [ ] `interact {action: "execute_js", script: "new Promise(r => setTimeout(() => r('done'), 100))"}` returns "done"

---

## Edge Cases: State Capture/Store

### What State Captures

- [ ] Save state, then check: localStorage is captured
- [ ] Save state, then check: sessionStorage is captured
- [ ] Save state, then check: cookies are captured

### What State Does NOT Capture

- [ ] In-memory JavaScript variables are lost on restore
- [ ] IndexedDB data is not captured
- [ ] Scroll position is not restored
- [ ] Form field values are not restored
- [ ] WebSocket connections are not reconnected
- [ ] Dynamically loaded content (fetched data) is not restored

### Restore Clears All Storage

- [ ] Load state clears ALL localStorage (not just Gasoline keys)
- [ ] Other app data stored in localStorage is lost
- [ ] Verify this is documented/expected behavior

### Restore with URL Navigation

- [ ] `interact {action: "load_state", snapshot_name: "x", include_url: true}` navigates to saved URL
- [ ] Page reload happens; restored storage survives the navigation

### Nonexistent Snapshot

- [ ] `interact {action: "load_state", snapshot_name: "doesnotexist"}` returns meaningful error
- [ ] `interact {action: "delete_state", snapshot_name: "doesnotexist"}` returns meaningful error

---

## Edge Cases: Query DOM

### Invalid CSS Selector

- [ ] `configure {action: "query_dom", selector: "[[[invalid"}` returns error (not crash)

### Element Cap (50 elements max)

- [ ] `configure {action: "query_dom", selector: "*"}` returns at most 50 elements
- [ ] Response includes total matchCount showing how many actually exist

### Hidden Elements

- [ ] `configure {action: "query_dom", selector: "[style*='display:none']"}` returns elements
- [ ] getBoundingClientRect returns zeroes for hidden elements (expected, not a bug)

---

## Edge Cases: Security & Privacy

### Auth Header Redaction

- [ ] Trigger a request with Authorization header from the demo
- [ ] `observe {what: "network"}` shows the request
- [ ] Authorization header value is redacted (shows [REDACTED])

### Redaction Engine on Tool Responses

- [ ] If captured data contains passwords or API keys, they are redacted before MCP response
- [ ] Verify redaction on `observe {what: "logs"}` if sensitive data was logged

### DNS Rebinding Protection

- [ ] HTTP requests to the server with `Host: attacker.com` header are rejected
- [ ] Only localhost variants (localhost, 127.0.0.1, [::1]) accepted

### POST Body Size Limit (5MB)

- [ ] Extension POSTing >5MB body receives 413 error
- [ ] Server does not crash or OOM

---

## Edge Cases: Multi-Client Mode

### Client Registration

- [ ] Start gasoline with `--connect` from a second terminal
- [ ] `configure {action: "health"}` shows client registered
- [ ] Second client can make tool calls independently

### Client Isolation

- [ ] Client A creates a checkpoint
- [ ] Client B cannot see Client A's checkpoint
- [ ] Pending query results are isolated per client

### Client Eviction (10 max)

- [ ] Register 10+ clients
- [ ] 11th client causes oldest idle client to be LRU-evicted

---

## Edge Cases: User Mistakes

### Wrong Server URL in Extension

- [ ] Configure extension to point to wrong port (e.g., 9999)
- [ ] Extension fails to connect; health shows disconnected
- [ ] Error messages guide user to check server URL

### Server Not Running

- [ ] Stop gasoline server
- [ ] MCP calls fail with connection error
- [ ] Extension polls fail silently (circuit breaker opens after 5 failures)
- [ ] Restart server: extension reconnects within 2-3 seconds

### Demo Site Not Running

- [ ] Stop the demo server (localhost:3000)
- [ ] `observe {what: "network"}` shows failed requests
- [ ] `observe {what: "errors"}` captures fetch failures
- [ ] Gasoline itself continues working

### Extension Not Installed

- [ ] Remove extension from Chrome
- [ ] `observe {what: "pilot"}` shows never-connected state
- [ ] All observe modes that read from log buffer still work
- [ ] Interact commands fail with clear "extension not connected" error

### Wrong Browser (No Extension)

- [ ] Open demo in Firefox (extension is Chrome-only)
- [ ] Gasoline server runs but receives no data
- [ ] `observe {what: "errors"}` returns "no data" rather than crashing

### Multiple Gasoline Instances on Same Port

- [ ] Start a second gasoline with `--port 7890`
- [ ] Second instance fails to bind port -- clear error message
- [ ] First instance continues working

### Extension Active on Non-Target Page

- [ ] Navigate to youtube.com or other site
- [ ] Extension captures that site's telemetry (expected behavior)
- [ ] `observe {what: "page"}` shows the non-target URL
- [ ] User must navigate back to demo or use tab targeting

---

## Demo Bug Detection Matrix

Each demo bug should be detectable through Gasoline. Verify each:

| # | Bug | Detection Method | Expected Signal |
|---|-----|-----------------|-----------------|
| 1 | Wrong products endpoint (`/api/products` vs `/api/v2/products`) | `observe {what: "network", status_min: 400}` | 404 on /api/products |
| 2 | Cart retry on 201 success | `observe {what: "network", url: "cart"}` | Multiple POST /api/cart for one add |
| 3 | Nested data field mismatch (`data.cartId` vs `data.data.cartId`) | `observe {what: "errors"}` | Undefined property errors |
| 4 | Checkout 401 infinite loop | `observe {what: "network", url: "checkout"}` | Rapidly repeating /api/checkout requests |
| 5 | Inverted form validation | `interact {action: "execute_js", script: "..."}` + `observe {what: "errors"}` | Forms submit without data |
| 6 | Payment status field mismatch (`status` vs `success`) | `observe {what: "network", url: "payment"}` + `observe {what: "errors"}` | Payment looks failed despite 200 |
| 7 | Wrong order endpoint (`/api/order/` vs `/api/v2/orders/`) | `observe {what: "network", status_min: 400}` | 404 on /api/order/ |
| 8 | Order ID field mismatch (camelCase vs snake_case) | `observe {what: "errors"}` | Undefined orderId |
| 9 | Wrong WebSocket path (`/ws/chat` vs `/api/ws/chat`) | `observe {what: "websocket_status"}` | Failed connection or no connection |
| 10 | WebSocket message field (`text` vs `txt`) | `observe {what: "websocket_events"}` | Messages parse as undefined |
| 11 | Typing indicator timeout never clears | `observe {what: "errors"}` or `interact {action: "execute_js"}` | Typing indicator stuck visible |
| 12 | Payment card data logged to console | `observe {what: "security_audit", checks: ["pii"]}` | PII detected in logs |
| 13 | Debug info in payment response (_debug with Stripe key) | `observe {what: "security_audit", checks: ["credentials"]}` | Exposed credentials |

---

## Execution Notes

- Run tests in section order (observe first gives data for later sections)
- After each interact action, follow up with an observe to verify the effect
- Some tests require triggering demo bugs first (click buttons, submit forms)
- Mark each checkbox as you go; note any unexpected behavior
- Edge case sections can be run in any order after sections 1-4

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Pre-UAT Quality Gates | | | |
| OBSERVE Tool (20 modes) | | | |
| GENERATE Tool (7 formats) | | | |
| CONFIGURE Tool (11 actions) | | | |
| INTERACT Tool (11 actions) | | | |
| AI Web Pilot Features | | | |
| P5 Nice-to-Have Features | | | |
| Edge Cases: Input Validation | | | |
| Edge Cases: Extension Connection | | | |
| Edge Cases: Timeouts | | | |
| Edge Cases: Buffer Limits | | | |
| Edge Cases: Rate Limiting | | | |
| Edge Cases: Execute JS | | | |
| Edge Cases: State Capture | | | |
| Edge Cases: Query DOM | | | |
| Edge Cases: Security & Privacy | | | |
| Edge Cases: Multi-Client | | | |
| Edge Cases: User Mistakes | | | |
| Demo Bug Detection | | | |

---

## Release Decision

**Overall Result:** [ ] PASS / [ ] FAIL

**Comments:**

**Approved By:** ______________________ **Date:** __________

**Version Released:** v_._._

