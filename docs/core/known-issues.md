---
status: active
scope: issues/blockers
ai-priority: high
tags: [known-issues, v0.7]
last-verified: 2026-02-18
canonical: true
---

# Known Issues

## Code Review Findings (2026-02-18)

Comprehensive review of `next` branch covering end-to-end data flows, spec compliance, performance, documentation, and LLM tool usability.

---

### Data Integrity & Correctness

| # | Issue | Severity | Location | Details |
|---|-------|----------|----------|---------|
| CR-1 | Double-completion race in sync.go | High | `internal/capture/sync.go:150-157` | When extension sends a `SyncCommandResult` with both `ID` and `CorrelationID`, `SetQueryResultWithClient` calls `CompleteCommand` (status="complete"), then `CompleteCommandWithStatus` is called again with the extension-reported status. If extension reports `status="error"`, a goroutine blocked in `WaitForCommand` can wake on the first signal, read "complete", and return success to the LLM before the error overwrites it. TOCTOU race. |
| CR-2 | cleanExpiredQueries doesn't call ExpireCommand | High | `internal/capture/queries.go:127-137` | Periodic sweep removes expired queries from `pendingQueries` but does NOT call `ExpireCommand()` on their correlation IDs. Callers blocked in `WaitForCommand` hang until their own 15s timeout instead of being unblocked immediately. Already noted in MEMORY.md but not fixed. |
| CR-3 | elementIndexStore shared across clients | Medium | `cmd/dev-console/tools_core.go:221-226` | `list_interactive` writes to a single shared index map. Two concurrent LLM agents calling `list_interactive` overwrite each other's index, causing subsequent `click` calls to target wrong elements with no error reported. Comment acknowledges this. |
| CR-4 | GetCommandResult ignores clientID | Medium | `internal/capture/queries.go:667-688` | Command lookup by correlation ID does not verify it belongs to the requesting client. One client could poll and consume another client's result. |
| CR-5 | handleBrowserActionRefresh drops user args | Medium | `cmd/dev-console/tools_interact.go:364-370` | Refresh action constructs a hardcoded `{"action":"refresh"}` param, dropping any custom timeout, analyze, or subtitle flags from the original args. |
| CR-6 | refreshWaterfallIfStale blocks 5s while disconnected | Medium | `cmd/dev-console/tools_observe_analysis.go:56-79` | Issues HTTP request to extension with 5s timeout. When disconnected, every `observe(what:"network_waterfall")` call blocks for 5s before returning stale data. |
| CR-7 | Bridge never sets X-Gasoline-Client header | Low | `cmd/dev-console/bridge.go:783-789` | All bridge-forwarded requests have empty `ClientID`, making multi-client isolation features non-functional in bridge mode. |
| CR-8 | link_health drops tab_id | Low | `cmd/dev-console/tools_analyze.go:506-517` | `PendingQuery.TabID` always set to 0 regardless of caller-specified `tab_id`. Extension always uses active tab. |
| CR-9 | Timestamp parse error silently discarded | Low | `cmd/dev-console/tools_observe.go:263-266` | `time.Parse(time.RFC3339, ts)` error discarded with `_`. Non-RFC3339 timestamps cause `data_age: "no_data"` metadata even when data exists. |

### MCP Spec Compliance

| # | Issue | Severity | Location | Details |
|---|-------|----------|----------|---------|
| CR-10 | sendStartupError uses "startup" as JSON-RPC ID | Medium | `cmd/dev-console/main_connection.go` | JSON-RPC 2.0 requires `null` for the ID when the request ID could not be determined. Using `"startup"` is non-compliant. |
| CR-11 | Notification processing signals responseSent prematurely | Low | `cmd/dev-console/bridge.go` | When handler receives a notification (no `id`), `signalResponseSent()` fires immediately. Could allow `bridgeShutdown` to proceed before all tool responses complete. |
| CR-12 | HTTP 204 silent drop lacks debug logging | Low | `cmd/dev-console/bridge.go` | Daemon 204 responses (for notifications) dropped with no debug log entry, making notification handling hard to trace. |
| CR-13 | Connect mode missing stdout mutex | Low | `cmd/dev-console/connect_mode.go` | Stdout writes in `connect` mode don't use `mcpStdoutMu`. Not used in production MCP flows. |

### Performance

| # | Issue | Severity | Location | Details |
|---|-------|----------|----------|---------|
| CR-14 | Full-size allocations on every buffer eviction | High | `internal/capture/websocket.go`, `network_bodies.go` | `evictWSByCount()` and `evictNBByCount()` allocate new slices on every eviction cycle. At high throughput, creates steady GC pressure. Existing `RingBuffer[T]` type could replace this. |
| CR-15 | Array.filter() + allocation on every WS message | High | `src/lib/websocket.ts:251` | `_messageTimestamps.filter()` runs on every WebSocket message, allocating a new array each time. Hottest path in extension. |
| CR-16 | Double JSON.parse per WS message in extension | High | `src/lib/websocket.ts:253-295` | Incoming JSON messages parsed twice: once for schema detection, once for variant tracking. Second parse is pure waste. |
| CR-17 | time.After leak in WaitForPendingQueries | Medium | `internal/capture/queries.go:117` | `time.After` creates timer that leaks until fired. Should use `time.NewTimer` with `defer t.Stop()`. |
| CR-18 | No context.Context propagation | Medium | All Go handlers | Tool handlers don't accept `context.Context`. Client disconnect during blocking wait (up to 30s) cannot abort the operation. |
| CR-19 | Goroutine per WaitForResult call | Medium | `internal/capture/queries.go:399-411` | Each sync tool call spawns a goroutine ticking every 10ms to broadcast condition variable. Creates unbounded goroutine growth under concurrent load. |
| CR-20 | Parallel array synchronization fragile | Medium | `internal/capture/websocket.go`, `network_bodies.go` | `wsEvents`+`wsAddedAt` parallel slices can drift out of sync. `repairWSParallelArrays()` existence confirms historical sync failures. Should migrate to `RingBuffer[T]`. |

### LLM Tool Usability

| # | Issue | Severity | Location | Details |
|---|-------|----------|----------|---------|
| CR-21 | Guide says analyze is async, schema says sync-by-default | High | `cmd/dev-console/playbooks.go` guide text vs `tools_schema.go` | Guide and playbooks show async polling pattern ("analyze dispatches... poll with observe"). Schema description says "Synchronous Mode (Default)". LLMs following guide will unnecessarily poll. |
| CR-22 | scope param has 3 different semantics | Medium | `tools_schema.go` observe/analyze/generate | `observe.scope` = "current_page"/"all" (data filter). `analyze.scope` = CSS selector. `generate.scope` = CSS selector. Same name, different types. |
| CR-23 | describe_capabilities action has no oneOf branch | Medium | `tools_schema.go` configure | Exists in `action` enum but has no corresponding schema branch or documentation. LLM trap. |
| CR-24 | severity_min has different enum scales | Medium | `tools_schema.go` analyze vs configure | analyze: `critical\|high\|medium\|low\|info`. configure/streaming: `info\|warning\|error`. Same param name, different values. |
| CR-25 | sync and wait params overlap | Medium | `tools_schema.go` analyze, interact | Both exist, both control async behavior, interaction unclear. `interact` says "wait is alias for sync". `analyze` gives `wait` special annotation meaning (5-min block). |
| CR-26 | record_start vs recording_start naming | Medium | `tools_schema.go` interact vs configure | `interact`: `record_start/record_stop` (video). `configure`: `recording_start/recording_stop` (session). Dangerously similar names for different systems. |
| CR-27 | paste, get_readable, get_markdown undocumented | Medium | `tools_schema.go` interact | Actions exist in enum but appear in no guide, quickstart, or description. |
| CR-28 | noise_rule has no inner schema for rules objects | Medium | `tools_schema.go` configure | `rules` array accepts objects but provides no JSON Schema for required fields. LLM must guess. |
| CR-29 | Quickstart missing examples for 3 of 5 tools | Medium | `cmd/dev-console/playbooks.go` | `gasoline://quickstart` has zero examples for `generate`, `configure`, or `interact`. |
| CR-30 | Guide missing many shipped actions/modes | Low | `cmd/dev-console/playbooks.go` | Guide omits `page_summary`, `audit_log`, `diff_sessions`, `telemetry`, `test_boundary_*`, `navigate_and_wait_for`, `fill_form_and_submit`, `run_a11y_and_export_sarif` from tables. |

### Documentation Gaps

| # | Issue | Severity | Location | Details |
|---|-------|----------|----------|---------|
| CR-31 | .claude/refs/architecture.md does not exist | High | `CLAUDE.md` line 79 | CLAUDE.md directs LLMs to this path. It's broken. Actual architecture docs are scattered across `docs/core/` and `docs/features/mcp-persistent-server/`. |
| CR-32 | privacy.md understates extension permissions | High | `docs/privacy.md` | Claims `activeTab` + `storage` + localhost only. Actual manifest: `storage`, `alarms`, `tabs`, `scripting`, `tabCapture`, `offscreen`, `activeTab` + `<all_urls>` host permission. |
| CR-33 | Playback engine doesn't replay navigate/type/scroll | High | `internal/capture/playback.go` | `executeAction` for navigate/type/scroll has comments "In real implementation, would navigate browser. For tests, just verify the action exists." Returns "ok" without side effects. Feature docs claim replay works. |
| CR-34 | DL-1/DL-2/DL-3 noise filtering tests not implemented | High | `TEST_PLAN_SUMMARY.md` | Critical data leak tests marked NOT IMPLEMENTED and BLOCKING for production. Auth failures, app errors, and security events could be incorrectly auto-filtered. Not reflected in this file. |
| CR-35 | Recording lost if server crashes before StopRecording | High | `internal/capture/recording.go` | Recording state held in memory until `StopRecording`. No partial-save mechanism. Server crash = recording gone. |
| CR-36 | Service worker termination drops in-flight state | High | Extension service worker | Chrome kills service workers after 5-30 min inactivity. All pending queries, circuit breaker state, recording actions lost. Recovery via `chrome.storage.session` exists but implications for active recordings/commands not documented. |
| CR-37 | feature-navigation.md shipped features table is empty | Medium | `docs/features/feature-navigation.md` | Auto-generated "Shipped Features" table shows zero entries. Doc generation script is broken. |
| CR-38 | feature/mcp-tool-descriptions/ referenced but missing | Medium | `docs/features/feature-index.md` | Dangling link to non-existent folder. |
| CR-39 | error-recovery.md has stale pseudocode | Low | `docs/core/error-recovery.md:160` | Execute JS example uses `new Function(script)()`. Actual implementation uses `chrome.scripting.executeScript` with CSP fallback. |
| CR-40 | Password field values not redacted in recordings | High | `src/background/recording.ts` | Flow-recording tech spec shows `[redacted]` for password fields. No redaction logic found in actual recording code. Passwords captured as plaintext. |

### Code Standards Violations

| # | Issue | Severity | Location | Details |
|---|-------|----------|----------|---------|
| CR-41 | Files exceeding 800 LOC limit | Medium | Multiple | `pending-queries.ts` (1301), `dom-primitives.ts` (1096), `bridge.go` (1031), `tools_schema.go` (1019), `main.go` (956), `tools_observe_analysis.go` (884), `server_routes.go` (808) |
| CR-42 | 24+ Go files missing required file headers | Low | `internal/`, `cmd/dev-console/` | CLAUDE.md requires `// filename.go -- Purpose summary.` Many internal/ and cmd/ files lack this header. |

### Security

| # | Issue | Severity | Location | Details |
|---|-------|----------|----------|---------|
| CR-43 | X-Gasoline-Client header spoofable | Medium | `internal/server/` | Any local process can spoof `X-Gasoline-Client: gasoline-extension`. With no API key (default), any local process can read commands, inject results, or consume telemetry. |
| CR-44 | execute_js can read sensitive page data | Medium | Extension `execute_js` | Can read localStorage tokens, session secrets, form values. Documented in ai-web-pilot qa-plan but not in privacy.md. No runtime sandboxing. |

---

## Pre-existing Issues

### Open Issues

| # | Issue | Severity | Details |
|---|-------|----------|---------|
| 1 | Extension timeout on first interact() | Medium | Content script may not be fully loaded when first `interact()` command is sent after navigation. **Workaround:** Retry after 2-3 seconds. |
| 2 | Tracking loss during cross-origin navigation | Medium | Extension can lose tab tracking state during AI-initiated cross-origin navigation via `interact({action: "navigate"})`. **Workaround:** Re-enable tracking via extension popup. |
| 3 | Pilot test zombies | Low | `tests/extension/pilot-*.test.js` have hardcoded `version: '5.2.0'` and no exit -- become zombie processes that spam the daemon with sync requests. |

### Immediate Roadmap (stub handlers hidden from schema)

These handlers exist in code but are not yet functional. They are **hidden from the MCP schema** so clients cannot discover them. They will be exposed once implemented.

| Handler | Tool | What it needs |
|---------|------|---------------|
| `audit_log` | configure | Store and retrieve tool invocation history per session. Currently returns `{"entries":[]}`. |
| `diff_sessions` | configure | Capture, compare, list, and delete session snapshots. Currently echoes action back without doing work. |
| `playback_results` | observe | Store and return results from recording playback. Currently returns placeholder with empty results. |

### Flaky Tests (Pre-existing)

- `TestAsyncQueueReliability/Slow_polling` -- times out at 30s intermittently
- `tests/extension/async-timeout.test.js` -- 3 tests flaky

### Fixed in v5.8.0

- Early-patch WebSocket capture -- pages creating WS connections before inject script loads now captured
- camelCase to snake_case field mapping for network waterfall entries
- Command results routing through /sync endpoint with proper client ID filtering
- Post-navigation tracking state broadcast for favicon updates
- Empty arrays return `[]` instead of `null` in JSON responses
- Bridge timeouts return proper `extension_timeout` error code

### Fixed in v5.7.x

- Extension health check timeout (5s threshold added)
- Hardcoded version in inject.bundled.js (now reads from VERSION file via esbuild define)
- Stale compiled JS vs TS source
