---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Pre-Release Hardening Decisions

**Date:** 2026-02-11
**Status:** Approved
**Context:** Comprehensive codebase evaluation before feature-complete release

## Summary

Evaluation of architecture, polling, extension UI, and UX identified 18 issues.
16 approved for implementation, 2 deferred.

## Approved Changes

### P0 — Critical

| # | Issue | Fix |
|---|-------|-----|
| P0-1 | Extension disconnect detection — no heartbeat, pending queries hang indefinitely | Track `last_sync_seen` per client; expire pending queries after 10s silence; surface in `observe(what="pilot")` |
| P0-2 | "No tab being tracked" error missing fix instructions | Reword to include "Open the Gasoline extension popup and click 'Track This Tab'" |

### P1 — High

| # | Issue | Fix |
|---|-------|-----|
| P1-1 | Fixed 1s polling interval regardless of activity | Adaptive: 200ms when pending commands exist, 1s otherwise. No idle backoff beyond 1s |
| P1-2 | Version mismatch only visible in extension popup, not in MCP responses | Include warning in MCP tool responses until mismatch is resolved |

### P2 — Medium

| # | Issue | Fix |
|---|-------|-----|
| P2-1 | Wrapper exits without JSON-RPC error when binary not found | Send proper MCP-standard JSON-RPC error (isError=true) to stdout before exit |
| P2-2 | Log file grows unbounded | Log rotation — discard old entries when file exceeds size limit |
| P2-3 | Platform-specific commands in error messages (lsof doesn't exist on Windows) | Detect `runtime.GOOS` and show platform-appropriate commands |

### P3 — Low

| # | Issue | Fix |
|---|-------|-----|
| P3-1 | "Track This Tab" not obvious as prerequisite | Make it the hero action with explanatory text when no tab is tracked |
| P3-3 | No Help/Docs links in extension popup | Add links |

### UI — Extension Polish

| # | Issue | Fix |
|---|-------|-----|
| UI-1 | No light mode support | Add `prefers-color-scheme: light` CSS |
| UI-2 | Health indicator jargon ("circuit breaker: half-open") | Reword to plain language |
| UI-3 | Element highlight too harsh (4px solid red) | Soften to blue glow effect |
| UI-4 | Subtitle has no dismiss mechanism | Add close button on hover or Escape key |
| UI-5 | Missing accessibility — no label association, no focus indicators | Add `for` attributes and `:focus-visible` styles |
| UI-6 | "Defer Heavy Interceptors" label too technical | Rename to "Optimize Page Load Speed" |
| UI-7 | Log drop count not exposed | Add `dropped_count` to `/health` endpoint |

## Deferred

| # | Issue | Reason |
|---|-------|--------|
| P1-3 | Per-operation timeouts for async commands | Need more usage data to set appropriate values |
| P3-2 | Multi-profile data separation | Edge case; tab identifiers already exist |

## Implementation Notes

- All changes branch from `UNSTABLE`, PR to `UNSTABLE`
- Run `make compile-ts` after any `src/` changes
- Run `make test` to verify no regressions
