---
title: "Gasoline + Claude Desktop Setup"
description: "Configure Gasoline as an MCP server for Claude Desktop. Give Claude real-time access to your browser's console logs, network errors, and DOM state."
keywords: "Claude Desktop MCP server, Claude Desktop browser errors, Claude Desktop MCP config, Claude Desktop debugging"
permalink: /mcp-integration/claude-desktop/
toc: true
toc_sticky: true
---

Claude Desktop is Anthropic's desktop application for conversing with Claude. Gasoline connects via MCP to give Claude visibility into your browser.

## Configuration

Edit the Claude Desktop config file:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

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

## Usage

After restarting Claude Desktop, Claude can:

- Read browser console errors and warnings
- Inspect failed API calls with response bodies
- Query the live DOM
- Monitor WebSocket connections
- Run accessibility audits

Ask Claude: _"What errors are showing in my browser?"_

## Troubleshooting

1. **Restart Claude Desktop** after editing the config
2. **Check file location** — path is OS-specific (see above)
3. **Verify JSON syntax** — invalid JSON will silently fail
4. **Check extension popup** — should show "Connected"
