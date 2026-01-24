---
title: "Fire It Up"
description: "Install and configure Gasoline in under 2 minutes. Start streaming browser logs to your autonomous coding agent with a single command."
keywords: "install gasoline, gasoline mcp setup, npx gasoline-mcp, browser extension install, MCP server setup"
permalink: /getting-started/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "One command. Two minutes. Your AI sees your browser."
toc: true
toc_sticky: true
---

## <i class="fas fa-fire"></i> 1. Ignite the Server

```bash
npx gasoline-mcp
```

You'll see: `Gasoline server listening on http://localhost:7890`

Leave this burning. No global install — `npx` handles everything.

## <i class="fas fa-puzzle-piece"></i> 2. Install the Extension

Grab it from the [Chrome Web Store](https://chromewebstore.google.com) (search "Gasoline"). Click the icon in your toolbar — it should show **Connected**.

<details>
<summary><i class="fas fa-wrench"></i> Load Unpacked (Development)</summary>

1. Clone the [repository](https://github.com/brennhill/gasoline)
2. Open `chrome://extensions` → enable **Developer mode**
3. Click **Load unpacked** → select the `extension/` folder

</details>

## <i class="fas fa-plug"></i> 3. Connect Your AI Tool

Drop this config and your AI tool fires up Gasoline automatically:

**Claude Code** — `.mcp.json` in your project root:

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

<i class="fas fa-fire-alt"></i> See [MCP Integration](/mcp-integration/) for Cursor, Windsurf, Claude Desktop, Zed, and more.

**Restart your AI tool.** From now on, the server ignites automatically.

## <i class="fas fa-check-circle"></i> Verify the Flame

Open your web app. Trigger an error:

```javascript
console.error("Gasoline test — is the fire lit?")
```

Ask your AI: _"What browser errors do you see?"_

## <i class="fas fa-tools"></i> Available Tools

Your AI now has these at its disposal:

| Tool | What it does |
|------|-------------|
| `get_browser_errors` | <i class="fas fa-exclamation-triangle"></i> Console errors, network failures, exceptions |
| `get_browser_logs` | <i class="fas fa-list"></i> All logs (errors + warnings + info) |
| `clear_browser_logs` | <i class="fas fa-eraser"></i> Clear the log file |
| `get_websocket_events` | <i class="fas fa-plug"></i> WebSocket messages and lifecycle |
| `get_websocket_status` | <i class="fas fa-signal"></i> Connection states and rates |
| `get_network_bodies` | <i class="fas fa-exchange-alt"></i> Request/response payloads |
| `query_dom` | <i class="fas fa-code"></i> Live DOM query with CSS selectors |
| `get_page_info` | <i class="fas fa-info-circle"></i> Page URL, title, viewport |
| `run_accessibility_audit` | <i class="fas fa-universal-access"></i> Accessibility violations |

## <i class="fas fa-file-alt"></i> No MCP? No Problem.

Run standalone — Gasoline writes to `~/gasoline-logs.jsonl`. Point your AI at the file.

```bash
npx gasoline-mcp
```

## Next Steps

- <i class="fas fa-sliders-h"></i> [Configure the server](/configuration/) — port, log rotation, file path
- <i class="fas fa-plug"></i> [MCP Integration](/mcp-integration/) — setup for your specific tool
- <i class="fas fa-fire-alt"></i> [All capabilities](/features/) — everything Gasoline captures
