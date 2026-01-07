---
title: "Gasoline + Claude Code"
description: "Configure Gasoline as an MCP server for Claude Code. Give Claude real-time access to browser console logs, network errors, and DOM state."
keywords: "Claude Code MCP server, Claude Code browser errors, Claude Code debugging, Claude Code browser extension"
permalink: /mcp-integration/claude-code/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Fuel Claude Code with live browser data."
toc: true
toc_sticky: true
---

## <i class="fas fa-file-code"></i> Project-Level Config (Recommended)

Create `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
    }
  }
}
```

Gasoline only fires up when you're in this project. The single process handles both the HTTP server (for extension) and MCP stdio (for Claude Code).

## <i class="fas fa-globe"></i> Global Config

Available in all projects — add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
    }
  }
}
```

## <i class="fas fa-fire-alt"></i> Usage

Once configured, Claude Code auto-ignites the server. Ask:

- _"What browser errors do you see?"_
- _"Check the network responses for /api/users"_
- _"Run an accessibility audit on this page"_
- _"What's the DOM structure of the nav?"_
- _"Any WebSocket connection issues?"_

Claude uses the right MCP tool and returns actionable debugging info.

## <i class="fas fa-wrench"></i> Troubleshooting

1. **Restart Claude Code** after adding config
2. **Check extension popup** — should show "Connected"
3. **Verify tools**: Ask _"What MCP tools do you have?"_
4. **Check logs**: Look for MCP connection errors in output

**Port conflict ("bind: address already in use")?**

Kill any manually-started Gasoline instances:

```bash
pkill -f gasoline
```

Then reload the MCP connection. Do NOT manually start Gasoline when using MCP mode — let Claude Code spawn and manage the process.
