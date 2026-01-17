# Gasoline UAT (User Acceptance Testing) Plan - v2.0

**UPDATED**: 2026-01-30 - Reflects actual v5.2.0 implementation after tool reorganization.

Complete user acceptance testing specification for Gasoline. All tests must pass before release.

---

## Table of Contents

1. [Pre-UAT Quality Gates](#pre-uat-quality-gates)
2. [Demo Setup](#demo-setup)
3. [OBSERVE Tool (24 modes)](#observe-tool-24-modes)
4. [GENERATE Tool (7 formats)](#generate-tool-7-formats)
5. [CONFIGURE Tool (13 actions)](#configure-tool-13-actions)
6. [INTERACT Tool (11 actions)](#interact-tool-11-actions)
7. [Edge Cases & Integration](#edge-cases--integration)
8. [Sign-Off](#sign-off)

---

## Pre-UAT Quality Gates

**STOP if any of these fail. Do NOT proceed to UAT.**

### 1. Extension Status
- [ ] Extension service worker loads without errors
- [ ] Extension shows "Connected" in popup
- [ ] `configure {action: "health"}` shows `extension_connected: true`
- [ ] `observe {what: "pilot"}` shows `enabled: true`

### 2. Server Status
- [ ] Gasoline server running on port 7890
- [ ] `configure {action: "health"}` returns valid health data
- [ ] Server version matches expected (5.2.0+)

### 3. Code Quality
- [ ] `make compile-ts` passes (if TypeScript changed)
- [ ] `go vet ./cmd/dev-console/` passes
- [ ] `make test` passes (all Go tests)
- [ ] No uncommitted breaking changes

---

## Demo Setup

### Prerequisites

- [ ] Gasoline server running: `./gasoline --port 7890`
- [ ] ShopNow demo running: `cd ~/dev/gasoline-demos && npm start` (localhost:3000)
- [ ] Chrome extension installed and connected
- [ ] Extension AI Web Pilot toggle **enabled**
- [ ] Navigate to localhost:3000 in Chrome

### Demo Contains 13 Intentional Bugs

Bugs across network, parsing, WebSocket, security, and UI layers for testing detection capabilities.

---

## OBSERVE Tool (24 modes)

Tests read telemetry captured by the extension. Run in order where dependencies exist.

### 1. errors

**What**: Console errors and unhandled exceptions.

**Demo signal**: Products page calls wrong endpoint → 404 errors.

- [ ] `observe {what: "errors"}` returns data or "No browser errors found"
- [ ] If errors exist, shows level, message, source, timestamp
- [ ] Errors include stack traces where available

### 2. logs

**What**: All console output (log/warn/info/debug/error).

**Demo signal**: Server logs payment card data; client-side logs.

- [ ] `observe {what: "logs"}` returns markdown table
- [ ] Shows Level, Message, Source, Time, Tab columns
- [ ] Multiple log levels represented

**Known Issue**: May show warning about incomplete fields if extension version mismatch.

### 3. network_waterfall

**What**: HTTP requests with URL, method, status, timing.

**Demo signal**: 404 on `/api/products`, 401/429 on `/api/checkout`.

- [ ] `observe {what: "network_waterfall"}` returns markdown table
- [ ] Shows URL, Method, Status, Duration columns
- [ ] `observe {what: "network_waterfall", url: "products"}` filters requests
- [ ] `observe {what: "network_waterfall", status_min: 400}` shows only errors
- [ ] `observe {what: "network_waterfall", status_min: 400, status_max: 499}` shows 4xx only
- [ ] `observe {what: "network_waterfall", method: "POST"}` filters by method

### 4. network_bodies

**What**: Request/response bodies for network calls.

- [ ] `observe {what: "network_bodies"}` returns captured bodies
- [ ] Shows request and response body content
- [ ] `observe {what: "network_bodies", url: "products"}` filters
- [ ] `observe {what: "network_bodies", status_min: 400}` filters errors
- [ ] Bodies truncated at size limits (8KB request / 16KB response)

### 5. websocket_events

**What**: WebSocket message frames with direction and content.

**Demo signal**: Chat connects to wrong path (`/ws/chat` vs `/api/ws/chat`).

- [ ] `observe {what: "websocket_events"}` returns events or empty message
- [ ] `observe {what: "websocket_events", direction: "incoming"}` filters inbound
- [ ] `observe {what: "websocket_events", direction: "outgoing"}` filters outbound
- [ ] `observe {what: "websocket_events", url: "chat"}` filters by URL
- [ ] `observe {what: "websocket_events", connection_id: "..."}` filters by connection

### 6. websocket_status

**What**: WebSocket connection states and message counts.

- [ ] `observe {what: "websocket_status"}` returns JSON with connections
- [ ] Shows connection id, url, state, messageCount
- [ ] `observe {what: "websocket_status", url: "chat"}` filters

### 7. actions

**What**: Recorded user interactions (clicks, navigation, form input).

- [ ] Navigate and interact with demo site first
- [ ] `observe {what: "actions"}` returns markdown table
- [ ] Shows Type, URL, Selector, Value, Time columns
- [ ] `observe {what: "actions", url: "localhost:3000"}` filters by URL

### 8. vitals

**What**: Web Vitals -- LCP, CLS, INP, FCP, TTFB.

- [ ] `observe {what: "vitals"}` returns JSON with snapshots
- [ ] Contains numeric metrics for Web Vital keys

### 9. page

**What**: Current URL, title, readyState, viewport.

- [ ] `observe {what: "page"}` returns JSON
- [ ] Contains url, title, status, viewport fields
- [ ] URL matches current page (localhost:3000)

### 10. tabs

**What**: All open browser tabs.

- [ ] `observe {what: "tabs"}` returns markdown table
- [ ] Shows ID, URL, Title, Active columns
- [ ] At least one tab shows localhost:3000

### 11. pilot

**What**: AI Web Pilot toggle status.

- [ ] `observe {what: "pilot"}` returns JSON
- [ ] Shows enabled: true (toggle is on)
- [ ] Shows extensionConnected: true
- [ ] Shows source field (extension_poll)

### 12. performance

**What**: Formatted performance metrics report.

- [ ] `observe {what: "performance"}` returns text report
- [ ] Contains FCP, LCP, CLS, INP sections

### 13. api

**What**: Inferred API schema from network traffic.

- [ ] Trigger network requests first (load page, navigate)
- [ ] `observe {what: "api"}` returns markdown table
- [ ] Shows Method, Path, Observations columns
- [ ] `observe {what: "api", format: "openapi_stub"}` returns OpenAPI JSON
- [ ] `observe {what: "api", url: "products"}` filters endpoints
- [ ] `observe {what: "api", min_observations: 2}` filters by frequency

### 14. accessibility

**What**: WCAG accessibility violations.

- [ ] `observe {what: "accessibility"}` returns markdown table
- [ ] Shows Impact, ID, Description, Nodes columns
- [ ] `observe {what: "accessibility", scope: ".product-grid"}` scopes to selector
- [ ] `observe {what: "accessibility", force_refresh: true}` bypasses cache

### 15. changes

**What**: What changed since a named checkpoint.

- [ ] `configure {action: "diff_sessions", session_action: "capture", name: "baseline"}` first
- [ ] Trigger new errors (reload page, click around)
- [ ] `observe {what: "changes", checkpoint: "baseline"}` returns diff JSON
- [ ] Shows new errors, network requests, actions
- [ ] `observe {what: "changes", severity: "errors_only"}` filters

### 16. timeline

**What**: Chronological event timeline.

- [ ] `observe {what: "timeline"}` returns JSON with events array
- [ ] Events have type, timestamp, data fields
- [ ] `observe {what: "timeline", include: ["errors", "network"]}` filters types

### 17. error_clusters

**What**: Deduplicated error groups by signature.

- [ ] Trigger repeated errors (reload page multiple times)
- [ ] `observe {what: "error_clusters"}` returns JSON
- [ ] Shows clusters with signature, count, representative

### 18. history

**What**: Patterns and anomalies in error history.

- [ ] `observe {what: "history"}` returns JSON
- [ ] Contains patterns and anomalies arrays

### 19. security_audit

**What**: Security scan for credentials, PII, headers, cookies.

**Demo signal**: _debug field with Stripe key, card numbers logged.

- [ ] `observe {what: "security_audit"}` returns JSON audit results
- [ ] Detects exposed credentials or debug info
- [ ] `observe {what: "security_audit", checks: ["credentials", "pii"]}` runs subset
- [ ] `observe {what: "security_audit", severity_min: "high"}` filters severity

### 20. third_party_audit

**What**: Third-party domain analysis.

- [ ] `observe {what: "third_party_audit"}` returns JSON
- [ ] Shows domains, request counts, risk levels
- [ ] `observe {what: "third_party_audit", first_party_origins: ["http://localhost:3000"]}` sets first-party

### 21. security_diff

**What**: Security state snapshots and comparison.

- [ ] `observe {what: "security_diff", action: "snapshot", name: "before"}` captures
- [ ] Trigger changes (load new page)
- [ ] `observe {what: "security_diff", action: "snapshot", name: "after"}` captures again
- [ ] `observe {what: "security_diff", action: "compare", compare_from: "before", compare_to: "after"}` shows diff
- [ ] `observe {what: "security_diff", action: "list"}` shows all snapshots

### 22. command_result

**What**: Retrieve async command results by correlation_id.

- [ ] Execute an async command with correlation_id
- [ ] `observe {what: "command_result", correlation_id: "..."}` retrieves result
- [ ] Shows status (pending/complete/timeout) and result/error

### 23. pending_commands

**What**: List all pending async commands.

- [ ] Execute multiple async commands
- [ ] `observe {what: "pending_commands"}` shows in-flight commands
- [ ] Shows correlation_id, command type, elapsed time

### 24. failed_commands

**What**: List recently failed commands.

- [ ] Execute commands that fail (invalid selector, etc.)
- [ ] `observe {what: "failed_commands"}` shows failed commands
- [ ] Shows correlation_id, error, timestamp

---

## GENERATE Tool (7 formats)

Produces outputs from captured telemetry.

### 1. reproduction

**What**: Playwright script replaying recorded actions.

- [ ] Perform actions on demo site first
- [ ] `generate {format: "reproduction"}` returns JavaScript code
- [ ] Script includes navigation and interactions
- [ ] `generate {format: "reproduction", last_n: 5}` limits to last 5 actions

### 2. test

**What**: Playwright test with assertions.

- [ ] `generate {format: "test", test_name: "shopnow_smoke"}` returns test code
- [ ] Code includes test assertions
- [ ] `generate {format: "test", assert_no_errors: true}` adds error assertion
- [ ] `generate {format: "test", assert_network: true}` adds network assertions

### 3. pr_summary

**What**: GitHub PR description from session data.

- [ ] `generate {format: "pr_summary"}` returns JSON
- [ ] Contains title, body, labels, testEvidence fields

### 4. sarif

**What**: SARIF 2.1 security report.

- [ ] `generate {format: "sarif"}` returns SARIF JSON
- [ ] Valid structure with runs, rules, results
- [ ] `generate {format: "sarif", include_passes: true}` includes passing checks

### 5. har

**What**: HAR archive of network traffic.

- [ ] `generate {format: "har"}` returns HAR JSON
- [ ] Valid HAR 1.2 structure with log.entries
- [ ] `generate {format: "har", url: "products"}` filters entries
- [ ] `generate {format: "har", status_min: 400}` filters by status

### 6. csp

**What**: Content-Security-Policy header generation.

- [ ] `generate {format: "csp"}` returns JSON with policy, directives, metaTag
- [ ] `generate {format: "csp", mode: "strict"}` generates strict policy
- [ ] `generate {format: "csp", mode: "report_only"}` generates report-only

### 7. sri

**What**: Subresource Integrity hashes.

- [ ] `generate {format: "sri"}` returns JSON with resources array
- [ ] Each resource has url, integrity, tag fields

---

## CONFIGURE Tool (13 actions)

Server configuration and state management.

### 1. health

**What**: Server health and metrics.

- [ ] `configure {action: "health"}` returns JSON
- [ ] Contains server, memory, buffers, rate_limiting, audit, pilot sections
- [ ] Extension shows connected
- [ ] Memory usage reasonable (< 20MB)

### 2. query_dom

**What**: CSS selector query on live page.

**Demo signal**: Product grid, cart counter queryable.

- [ ] `configure {action: "query_dom", selector: ".product-grid"}` returns matches
- [ ] `configure {action: "query_dom", selector: "#cart-count"}` returns element
- [ ] `configure {action: "query_dom", selector: ".nonexistent"}` returns no match
- [ ] Results capped at 50 elements max

### 3. clear

**What**: Clear all browser console logs.

- [ ] Verify logs exist: `observe {what: "logs"}`
- [ ] `configure {action: "clear"}` succeeds
- [ ] `observe {what: "logs"}` returns fewer/no entries

### 4. capture

**What**: Configure capture settings (enable/disable data collection).

- [ ] `configure {action: "capture"}` shows current settings
- [ ] Settings include what data types are being captured

### 5. record_event

**What**: Record custom temporal event.

- [ ] `configure {action: "record_event", ...}` records custom event
- [ ] Event appears in timeline

### 6. noise_rule

**What**: Filter recurring noise patterns.

- [ ] `configure {action: "noise_rule", noise_action: "list"}` shows rules
- [ ] `configure {action: "noise_rule", noise_action: "auto_detect"}` suggests rules
- [ ] `configure {action: "noise_rule", noise_action: "reset"}` clears all

### 7. dismiss

**What**: Quick noise dismissal.

- [ ] `configure {action: "dismiss", pattern: "favicon", category: "network"}` dismisses
- [ ] Pattern no longer in subsequent observe calls

### 8. store

**What**: Persistent key-value storage.

- [ ] `configure {action: "store", store_action: "save", namespace: "test", key: "foo", data: {"bar": 1}}` saves
- [ ] `configure {action: "store", store_action: "load", namespace: "test"}` retrieves
- [ ] `configure {action: "store", store_action: "list"}` lists namespaces
- [ ] `configure {action: "store", store_action: "stats"}` shows usage
- [ ] `configure {action: "store", store_action: "delete", namespace: "test", key: "foo"}` removes

### 9. load

**What**: Load session context from disk.

- [ ] `configure {action: "load"}` returns session context

### 10. diff_sessions

**What**: Capture and compare full session snapshots.

- [ ] `configure {action: "diff_sessions", session_action: "capture", name: "snap1"}` captures
- [ ] Trigger changes
- [ ] `configure {action: "diff_sessions", session_action: "capture", name: "snap2"}` captures
- [ ] `configure {action: "diff_sessions", session_action: "compare", compare_a: "snap1", compare_b: "snap2"}` shows diff
- [ ] `configure {action: "diff_sessions", session_action: "list"}` lists snapshots
- [ ] `configure {action: "diff_sessions", session_action: "delete", name: "snap1"}` deletes

### 11. validate_api

**What**: Check API response contracts.

- [ ] Trigger API calls first
- [ ] `configure {action: "validate_api", operation: "analyze"}` finds violations
- [ ] `configure {action: "validate_api", operation: "report"}` generates report
- [ ] `configure {action: "validate_api", operation: "clear"}` resets state

### 12. audit_log

**What**: View MCP tool call audit trail.

- [ ] Make several tool calls first
- [ ] `configure {action: "audit_log"}` returns recent calls
- [ ] `configure {action: "audit_log", tool_name: "observe"}` filters by tool
- [ ] `configure {action: "audit_log", limit: 3}` caps results

### 13. streaming

**What**: Real-time push notifications via MCP.

- [ ] `configure {action: "streaming", streaming_action: "status"}` shows state
- [ ] `configure {action: "streaming", streaming_action: "enable", events: ["errors"]}` starts
- [ ] Trigger error on page
- [ ] Verify notification (check status shows subscription)
- [ ] `configure {action: "streaming", streaming_action: "disable"}` stops

---

## INTERACT Tool (11 actions)

> **NOTE:** Browser actions require AI Web Pilot enabled.

### 1. navigate

- [ ] `interact {action: "navigate", url: "http://localhost:3000"}` navigates
- [ ] `observe {what: "page"}` confirms URL changed

### 2. refresh

- [ ] `interact {action: "refresh"}` reloads page
- [ ] `observe {what: "network_waterfall"}` shows fresh requests

### 3. back / forward

- [ ] Navigate to second page first
- [ ] `interact {action: "back"}` goes back
- [ ] `interact {action: "forward"}` goes forward

### 4. new_tab

- [ ] `interact {action: "new_tab", url: "http://localhost:3000"}` opens tab
- [ ] `observe {what: "tabs"}` shows new tab

### 5. highlight

**What**: Visually highlight DOM element.

- [ ] `interact {action: "highlight", selector: "#cart-link"}` highlights
- [ ] Response includes confirmation
- [ ] `interact {action: "highlight", selector: ".product-grid", duration_ms: 3000}` custom duration
- [ ] `interact {action: "highlight", selector: ".nonexistent"}` returns error

### 6. execute_js

**What**: Run JavaScript in page context.

- [ ] `interact {action: "execute_js", script: "document.title"}` returns title
- [ ] `interact {action: "execute_js", script: "document.querySelectorAll('.product-card').length"}` returns count
- [ ] `interact {action: "execute_js", script: "window.location.href"}` returns URL
- [ ] `interact {action: "execute_js", script: "throw new Error('test')"}` handles error
- [ ] `interact {action: "execute_js", script: "Promise.resolve(42)"}` resolves promise

### 7. save_state

**What**: Save page state (localStorage, sessionStorage, cookies).

- [ ] `interact {action: "save_state", snapshot_name: "clean"}` saves
- [ ] `interact {action: "list_states"}` shows "clean"

### 8. load_state

**What**: Restore page state snapshot.

- [ ] Make changes (add to cart, etc.)
- [ ] `interact {action: "load_state", snapshot_name: "clean"}` restores
- [ ] `interact {action: "load_state", snapshot_name: "clean", include_url: true}` with URL

### 9. list_states

**What**: List saved state snapshots.

- [ ] `interact {action: "list_states"}` shows snapshots with metadata

### 10. delete_state

**What**: Remove state snapshot.

- [ ] `interact {action: "delete_state", snapshot_name: "clean"}` removes
- [ ] `interact {action: "list_states"}` no longer shows "clean"

### 11. (reserved for future)

---

## Edge Cases & Integration

### Input Validation

- [ ] Missing required params returns structured error
- [ ] Invalid mode/action returns error with valid values
- [ ] Malformed JSON handled gracefully
- [ ] Empty data returns descriptive messages

### Extension Connection

- [ ] Extension disconnected shows proper error
- [ ] Extension stale (>3s) shows warning
- [ ] Extension reload mid-session recovers
- [ ] AI Web Pilot toggle OFF returns pilot_disabled error

### Timeouts & Performance

- [ ] Extension query timeout (10s) works
- [ ] Execute JS custom timeout works
- [ ] Accessibility audit completes (30s timeout)
- [ ] DOM query caps at 50 elements

### Buffer Limits

- [ ] Log entry limit (1000) rotates oldest
- [ ] Pending query limit (5) drops oldest
- [ ] Network body truncation (8KB/16KB) works
- [ ] WebSocket buffer (500) evicts oldest

### Security & Privacy

- [ ] Auth headers redacted
- [ ] Sensitive data in logs redacted
- [ ] DNS rebinding protection works
- [ ] POST body size limit (5MB) enforced

---

## Sign-Off

| Area | Status | Notes |
|------|--------|-------|
| Pre-UAT Quality Gates | ☐ | |
| OBSERVE Tool (24 modes) | ☐ | |
| GENERATE Tool (7 formats) | ☐ | |
| CONFIGURE Tool (13 actions) | ☐ | |
| INTERACT Tool (11 actions) | ☐ | |
| Edge Cases | ☐ | |

**Overall Result:** [ ] PASS / [ ] FAIL

**Tester:** ______________________ **Date:** __________

**Version:** v5.2.0+
