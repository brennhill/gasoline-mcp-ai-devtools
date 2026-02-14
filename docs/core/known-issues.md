---
status: active
scope: issues/blockers
ai-priority: high
tags: [known-issues, v5.8]
last-verified: 2026-02-06
canonical: true
---

# Known Issues

## v5.8.0 — Current Release

### Open Issues

| # | Issue | Severity | Details |
|---|-------|----------|---------|
| 1 | Extension timeout on first interact() | Medium | Content script may not be fully loaded when first `interact()` command is sent after navigation. **Workaround:** Retry after 2-3 seconds. |
| 2 | Tracking loss during cross-origin navigation | Medium | Extension can lose tab tracking state during AI-initiated cross-origin navigation via `interact({action: "navigate"})`. **Workaround:** Re-enable tracking via extension popup. |
| 3 | Pilot test zombies | Low | `tests/extension/pilot-*.test.js` have hardcoded `version: '5.2.0'` and no exit — become zombie processes that spam the daemon with sync requests. |

### Immediate Roadmap (stub handlers hidden from schema)

These handlers exist in code but are not yet functional. They are **hidden from the MCP schema** so clients cannot discover them. They will be exposed once implemented.

| Handler | Tool | What it needs |
|---------|------|---------------|
| `audit_log` | configure | Store and retrieve tool invocation history per session. Currently returns `{"entries":[]}`. |
| `diff_sessions` | configure | Capture, compare, list, and delete session snapshots. Currently echoes action back without doing work. |
| `playback_results` | observe | Store and return results from recording playback. Currently returns placeholder with empty results. |

### Flaky Tests (Pre-existing)

- `TestAsyncQueueReliability/Slow_polling` — times out at 30s intermittently
- `tests/extension/async-timeout.test.js` — 3 tests flaky

### Fixed in v5.8.0

- Early-patch WebSocket capture — pages creating WS connections before inject script loads now captured
- camelCase to snake_case field mapping for network waterfall entries
- Command results routing through /sync endpoint with proper client ID filtering
- Post-navigation tracking state broadcast for favicon updates
- Empty arrays return `[]` instead of `null` in JSON responses
- Bridge timeouts return proper `extension_timeout` error code

### Fixed in v5.7.x

- Extension health check timeout (5s threshold added)
- Hardcoded version in inject.bundled.js (now reads from VERSION file via esbuild define)
- Stale compiled JS vs TS source
