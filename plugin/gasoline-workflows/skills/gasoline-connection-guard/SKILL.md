---
name: gasoline-connection-guard
description: Use when a tool call fails with "extension not connected", the daemon is unreachable, or browser telemetry stops arriving.
auto-trigger:
  - on-error-pattern: "extension not connected"
  - on-error-pattern: "no tracked tab"
  - on-error-pattern: "connection refused"
  - on-error-pattern: "daemon not running"
  - on-error-pattern: "ECONNREFUSED"
  - on-error-pattern: "no active connection"
allowed-tools:
  - mcp__gasoline__configure
---

# Gasoline Connection Guard

This skill activates automatically when a Gasoline MCP tool call fails with a connection-related error. It diagnoses the problem and guides the user through recovery.

## When This Activates

Any Gasoline tool call (`observe`, `analyze`, `generate`, `interact`, `configure`) returns an error matching:
- "extension not connected"
- "no tracked tab"
- "connection refused"
- "daemon not running"
- "ECONNREFUSED"
- "no active connection"

## Diagnostic Workflow

### Step 1: Identify the Error Class

Classify the error into one of these categories:

| Error Pattern | Diagnosis | Likely Cause |
|--------------|-----------|--------------|
| "daemon not running" / "ECONNREFUSED" | **Daemon Down** | Gasoline MCP server process is not running |
| "extension not connected" / "no active connection" | **Extension Disconnected** | Chrome extension lost its WebSocket connection to the daemon |
| "no tracked tab" | **No Tracked Tab** | Extension is connected but no tab is being monitored |

### Step 2: Run Health Check

Attempt `configure` with `what: "health"`.

**If health check succeeds:** The daemon is running. The issue is likely transient or tab-specific. Report the health status and suggest the user retry their command.

**If health check fails with connection refused:** The daemon is not running. Proceed to Step 3.

**If health check returns but shows extension disconnected:** The daemon is running but the extension is not connected. Proceed to Step 4.

### Step 3: Daemon Recovery

Tell the user:

```
The Gasoline daemon is not running.

To fix this:
1. Check if the process is alive: `ps aux | grep gasoline`
2. Restart the daemon — it should auto-start when Claude Code calls a Gasoline tool
3. If it doesn't auto-start, run `gasoline-mcp` manually to see any startup errors

Common causes:
- Port conflict (another process on the daemon port)
- Crashed due to an unhandled error (check recent logs)
- Not installed or not in PATH
```

After the user reports the daemon is back, run the health check again to verify.

### Step 4: Extension Reconnection

Tell the user:

```
The Gasoline daemon is running but the Chrome extension is not connected.

To fix this:
1. Open Chrome and check the Gasoline extension icon — it should show a green indicator when connected
2. Click the extension icon and verify the connection status
3. If disconnected, try:
   - Refresh the page you're working on
   - Click "Reconnect" in the extension popup (if available)
   - Disable and re-enable the extension in chrome://extensions
   - Close and reopen Chrome as a last resort

If the extension shows "connected" but tools still fail, the tracked tab may have been closed.
```

After the user reports the extension is reconnected, run the health check again to verify.

### Step 5: No Tracked Tab Recovery

Tell the user:

```
The extension is connected but no tab is being tracked.

To fix this:
1. Open the page you want to work with in Chrome
2. Click the Gasoline extension icon on that tab
3. Enable tracking for the tab (toggle or click "Track this tab")

The extension needs an active tracked tab to capture browser telemetry.
```

After the user reports a tab is tracked, run the health check again to verify.

### Step 6: Verification

After any recovery step, run `configure` with `what: "health"` one final time.

**If healthy:** Tell the user the connection is restored and they can retry their original command.

**If still failing:** Report the persistent failure and suggest checking the Gasoline GitHub issues or logs for more information.

## Rules

- Always attempt the health check before giving recovery instructions — the issue may have resolved itself.
- Be concise — the user is interrupted mid-workflow and wants to get back to their task.
- After successful recovery, remind the user what command they were trying to run so they can retry it.
- Do NOT attempt to start the daemon or modify extension settings yourself — guide the user through manual recovery.
