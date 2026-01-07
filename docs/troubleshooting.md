---
title: "Troubleshooting"
description: "Fix common Gasoline issues: extension not connecting, logs not appearing, MCP mode problems, port conflicts, and debug mode usage."
keywords: "gasoline troubleshooting, extension not connecting, MCP not working, gasoline debug mode, port conflict fix"
permalink: /troubleshooting/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Flame out? Get it burning again."
toc: true
toc_sticky: true
---

## <i class="fas fa-unlink"></i> Extension Not Connecting to Server

1. **Check server is running**: Look for `[gasoline] vX.Y.Z — HTTP on port 7890` on stderr
2. **Check extension badge**: Red `!` means disconnected, green means connected
3. **Check port conflict**: Another process may be using port 7890. Try `--port 7891`
4. **Update extension URL**: If using a different port, go to Options and change the Server URL
5. **Check browser console**: Right-click extension icon → Inspect → Console tab

## <i class="fas fa-eye-slash"></i> Logs Not Appearing

1. **Check capture level**: Popup may be set to "Errors Only" — try "All Logs"
2. **Check domain filter**: Options page may have filters excluding your domain
3. **Check log file**: `cat ~/gasoline-logs.jsonl | tail -5` to see recent entries
4. **Reload the page**: Extension injects on page load

## <i class="fas fa-robot"></i> MCP Mode Not Working

1. **Check config path**: Must be in your AI tool's config directory
2. **Restart your AI tool**: MCP servers are loaded on startup
3. **Check for JSON errors**: Invalid JSON in config will silently fail
4. **Verify with AI**: Ask _"What MCP tools do you have?"_ — Gasoline tools should appear

## <i class="fas fa-layer-group"></i> MCP Config Conflicts (User vs Project)

**Symptom**: You added `.mcp.json` to your project but Gasoline uses wrong settings (different port, old path, etc.)

**Cause**: Claude Code has two config levels:
- **User-level**: `~/.claude.json` → `mcpServers` section (applies globally)
- **Project-level**: `.mcp.json` in project root (applies to that project)

**User-level config takes precedence.** If both define `"gasoline"`, the user-level wins silently.

**To diagnose**:
```bash
# Check for user-level gasoline config
grep -A5 '"gasoline"' ~/.claude.json
```

**To fix**:
1. Edit `~/.claude.json` and remove the `gasoline` entry from `mcpServers`
2. Or update the user-level config to match your desired settings
3. Restart Claude Code to pick up changes

**Best practice**: Use project-level `.mcp.json` for Gasoline. This keeps the config with the project and avoids conflicts when working on multiple projects.

## <i class="fas fa-times-circle"></i> MCP Server Shows "Failed"

**Symptom**: `/mcp` shows gasoline as "failed" even though the config looks correct.

**Common cause**: A stale gasoline process is holding the port from a previous session.

**To diagnose**:
```bash
# Check if something is using the port
lsof -i :7890
```

**To fix**:
```bash
# Kill the stale process
lsof -ti :7890 | xargs kill
```

Then retry `/mcp` — Claude Code will spawn a fresh instance.

## <i class="fas fa-sync-alt"></i> MCP Disconnected — Recovery

When the AI client disconnects (closes its session), Gasoline logs the disconnect and exits after a brief grace period. This is by design — it frees the port so the next AI session can spawn a fresh process.

**What you'll see on stderr:**
```
[gasoline] MCP disconnected, shutting down in 100ms (port 7890 will be freed)
[gasoline] Shutdown complete
```

**Want to keep the server running?** Use `--persist`:
```bash
npx gasoline-mcp --persist
```
This keeps the HTTP server running after MCP disconnect so the extension stays connected between AI sessions. Press Ctrl+C to stop.

**To recover:** Simply start a new AI session. Your AI tool will spawn a fresh Gasoline process automatically. The extension reconnects to the new instance on its next poll.

**If port is still in use:** A previous Gasoline process may not have exited cleanly. Kill it:
```bash
lsof -ti :7890 | xargs kill
```

## <i class="fas fa-plug"></i> Changing the Server Port

If port 7890 is in use:

```bash
npx gasoline-mcp --port 7891
```

Then update the extension:

1. Click the Gasoline extension icon
2. Click "Options"
3. Change **Server URL** to `http://localhost:7891`
4. Click "Save Options"

## <i class="fas fa-check-circle"></i> Verifying Your Setup

Run the built-in setup check:
```bash
npx gasoline-mcp --check
```

This verifies port availability, log file directory, and prints next steps. You can also test from the extension: go to **Options** and click the **Test** button next to the Server URL.

## <i class="fas fa-exclamation-triangle"></i> Version Mismatch Warning

**Symptom**: Extension popup shows a yellow "Version mismatch" banner.

**Cause**: The extension and server have different major versions (e.g., extension v4.x, server v5.x).

**To fix**:
1. Update the server: `npx gasoline-mcp@latest`
2. Update the extension: pull latest from GitHub and reload in chrome://extensions
3. Both should show the same major version

Minor/patch version differences are fine and won't trigger the warning.

## <i class="fas fa-bug"></i> Using Debug Mode

The extension has built-in debug logging for troubleshooting:

1. **Enable**: Open popup → scroll to "Debugging" → toggle "Debug Mode" on
2. **View logs**: With debug mode on, activity appears in the browser console
3. **Export**: Click "Export Debug Log" to download a JSON file

### Debug Log Categories

| Category | What it captures |
|----------|-----------------|
| `connection` | Server connection/disconnection events |
| `capture` | Log capture, filtering, processing |
| `error` | Extension internal errors |
| `lifecycle` | Extension startup/shutdown |
| `settings` | Configuration changes |

The debug buffer holds 200 entries (circular — oldest are dropped). Logs are stored even when debug mode is off, so you can export after an issue occurs.

## <i class="fas fa-flag"></i> Reporting Issues

[Open an issue](https://github.com/brennhill/gasoline-mcp-ai-devtools/issues/new) with:

1. **Environment**: OS, Chrome version, Gasoline version (`window.__gasoline.version`)
2. **Steps to reproduce**
3. **Expected vs actual behavior**
4. **Extension popup screenshot**
5. **Debug log export** (JSON file from popup)
6. **Browser console errors** (right-click extension → Inspect → Console)
7. **Server output** (any errors from `npx gasoline-mcp`)
