---
title: "Gasoline + Zed Setup"
description: "Configure Gasoline as a context server for Zed editor. Give Zed's AI assistant access to browser console logs, network errors, and DOM state."
keywords: "Zed MCP server, Zed context server, Zed browser debugging, Zed AI browser logs"
permalink: /mcp-integration/zed/
toc: true
toc_sticky: true
---

Zed is a high-performance code editor with built-in AI features. Gasoline connects as a context server, giving Zed's assistant access to your browser state.

## Configuration

Add to `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "gasoline": {
      "command": {
        "path": "npx",
        "args": ["gasoline-mcp", "--mcp"]
      }
    }
  }
}
```

Note: Zed uses `context_servers` instead of `mcpServers`, and the command format differs slightly from other tools.

## Usage

After restarting Zed, the AI assistant can access browser state through Gasoline's MCP tools.

## Troubleshooting

1. **Restart Zed** after editing settings
2. **Check the config key** — must be `context_servers`, not `mcpServers`
3. **Verify the extension popup** shows "Connected"
