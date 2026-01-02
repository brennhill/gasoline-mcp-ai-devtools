---
title: "Getting Started with Gasoline"
description: "Install and configure Gasoline in under 2 minutes. Start capturing browser logs for your AI coding assistant with a single command."
keywords: "install gasoline, gasoline mcp setup, npx gasoline-mcp, browser extension install, MCP server setup"
permalink: /getting-started/
toc: true
toc_sticky: true
---

Gasoline has two parts: a **local server** (receives browser data) and a **browser extension** (captures it). Your AI assistant connects to the server via [MCP](https://modelcontextprotocol.io/).

## 1. Start the Server

```bash
npx gasoline-mcp
```

You should see: `Gasoline server listening on http://localhost:7890`

Leave this running in the background. No global install needed — `npx` handles it.

## 2. Install the Browser Extension

Install from the [Chrome Web Store](https://chromewebstore.google.com) (search "Gasoline"), then click the Gasoline icon in your toolbar — it should show **Connected**.

### Load Unpacked (Development)

1. Download or clone the [repository](https://github.com/brennhill/gasoline)
2. Open `chrome://extensions` in Chrome
3. Enable **Developer mode** (top right toggle)
4. Click **Load unpacked**
5. Select the `extension/` folder

## 3. Connect Your AI Tool

Add MCP config so your AI tool starts Gasoline automatically:

**Claude Code** — create `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

See the [MCP Integration](/mcp-integration/) section for Cursor, Windsurf, Claude Desktop, Zed, and more.

**Restart your AI tool after adding the config.** From now on, the server starts automatically.

## Verify It's Working

Open your web app, trigger an error (e.g., `console.error("test")`), and ask your AI: _"What browser errors do you see?"_

Your AI assistant now has access to these tools:

| Tool | What it does |
|------|-------------|
| `get_browser_errors` | Recent console errors, network failures, and exceptions |
| `get_browser_logs` | All logs (errors + warnings + info) |
| `clear_browser_logs` | Clears the log file |
| `get_websocket_events` | Captured WebSocket messages and lifecycle events |
| `get_websocket_status` | Active WebSocket connection states and rates |
| `get_network_bodies` | Captured request/response payloads |
| `query_dom` | Query the live DOM with a CSS selector |
| `get_page_info` | Current page URL, title, and viewport |
| `run_accessibility_audit` | Run an accessibility audit on the page |

## Alternative: Manual Server Mode (No MCP)

If your AI tool doesn't support MCP, run the server standalone and point your AI at the log file:

```bash
npx gasoline-mcp
```

The server writes logs to `~/gasoline-logs.jsonl`. Point your AI assistant at this file for manual debugging.

## Next Steps

- [Configure server options](/configuration/) (port, log rotation, log file path)
- [Set up MCP integration](/mcp-integration/) for your specific AI tool
- [Explore all captured data](/features/) (WebSocket, network bodies, DOM queries)
