# Incident Report: LinkedIn Automation Failure (2026-02-21)

## Summary
On **February 21, 2026**, an end-to-end automation task to publish a LinkedIn post via Gasoline failed.  
The attempt exposed multiple reliability defects across CLI/browser action dispatch, command result payloads, and reconnect timing.

## Context
- Goal: research a current AI release, generate a LinkedIn post, and publish it through Gasoline automation.
- Environment: `gasoline-mcp 0.7.7`, macOS (`darwin/arm64`), extension connected intermittently.

## Coverage Matrix
| Flow | Status | Notes |
|---|---|---|
| Preflight health | Partial | Health often reported `pass`, but pilot/extension readiness varied during restart windows. |
| `interact navigate` canary | Failed | `unknown_action` |
| `interact new_tab` canary | Failed | `unknown_action` |
| `interact list_interactive` canary | Failed | Success response, but payload was `value:null` |
| Reconnect transition | Failed | Immediate calls after daemon restart returned `pilot_disabled` before extension reattached |

## Repro Evidence
Commands run on **2026-02-21**:

```bash
gasoline-mcp interact navigate --url https://www.linkedin.com/feed/
gasoline-mcp interact new-tab --url https://www.linkedin.com/feed/
gasoline-mcp interact list-interactive
```

Observed output excerpts:
- `FAILED ... error: unknown_action ... message: "Unknown action: undefined"` for both `navigate` and `new_tab`.
- `interact list_interactive` returned `[OK]` but response body had `"result":{"value":null,...}` rather than interactive element data.
- Immediately after daemon restart, health showed `extension_connected=false`, `pilot_enabled=false`, and `interact` calls failed with `pilot_disabled`.

## Failures

### F1: Browser actions fail with `unknown_action`
- Affected actions: `navigate`, `new_tab`.
- Severity: High.
- Reproducible: Yes.
- Impact: Core browser control actions are unusable from CLI in normal flow.
- Likely root cause:
  - `cmd/dev-console/tools_interact.go:271` (`handleBrowserActionNavigate`) forwards raw args as `browser_action` params.
  - `cmd/dev-console/tools_interact.go:395` (`handleBrowserActionNewTab`) does the same.
  - Extension side expects `action`, while CLI path provides `what`; result is `Unknown action: undefined`.

### F2: `list_interactive` returns null payload
- Severity: High.
- Reproducible: Yes.
- Impact: Selector/index planning is blocked; deterministic automation degrades to guesswork.
- Observed: command completes successfully but returns `result.value:null` instead of element list.

### F3: Reconnect race after daemon restart
- Severity: Medium.
- Reproducible: Yes.
- Impact: First commands after restart fail with `pilot_disabled`, forcing manual retries.
- Observed: `configure health` initially showed disconnected pilot, then transitioned to connected state seconds later.
- Required behavior: pilot should be presumed enabled by default unless there is explicit proof that the user actively disabled it.

## Mitigations
- Add strict payload normalization for browser actions so both `what` and `action` are accepted and always translated to extension-compatible `action`.
- Enforce contract tests for non-null result payloads on `list_interactive`.
- Treat pilot as **assumed enabled** during startup uncertainty; only emit hard `pilot_disabled` when disablement is explicit and authoritative.
- Add bounded auto-retry/wait in CLI for reconnect windows, and defer disablement errors to extension execution when truly disabled.
- Improve health/readiness semantics to distinguish:
  - `server up`
  - `extension connected`
  - `pilot ready`
  - `tracked tab ready`

## Release Risk
- Risk level: **High** for automation workflows that depend on navigation/tab control and element discovery.
- Recommendation: block release for automation reliability claims until F1/F2 are fixed and reconnect handling is hardened.

## Proposed GitHub Issues
1. `#210` `[Regression] interact navigate/new_tab fail with unknown_action`
2. `#211` `[Regression] interact list_interactive returns success with null payload`
3. `#212` `[Reliability] pilot_disabled should default to enabled unless explicitly user-disabled`
