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

## <i class="fas fa-sync-alt"></i> MCP Disconnected — Recovery

When the AI client disconnects (closes its session), Gasoline logs the disconnect and exits after a 2-second grace period. This is by design — it frees the port so the next AI session can spawn a fresh process.

**What you'll see on stderr:**
```
[gasoline] MCP disconnected, shutting down in 2s (port 7890 will be freed)
[gasoline] Shutdown complete
```

**What's logged to JSONL:**
```jsonl
{"type":"lifecycle","event":"mcp_disconnect","port":7890,"timestamp":"..."}
{"type":"lifecycle","event":"shutdown","reason":"mcp_disconnect_grace","timestamp":"..."}
```

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

[Open an issue](https://github.com/brennhill/gasoline/issues/new) with:

1. **Environment**: OS, Chrome version, Gasoline version (`window.__gasoline.version`)
2. **Steps to reproduce**
3. **Expected vs actual behavior**
4. **Extension popup screenshot**
5. **Debug log export** (JSON file from popup)
6. **Browser console errors** (right-click extension → Inspect → Console)
7. **Server output** (any errors from `npx gasoline-mcp`)
