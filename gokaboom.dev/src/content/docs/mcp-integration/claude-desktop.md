---
title: KaBOOM + Claude Desktop
description: "Configure KaBOOM as an MCP server for Claude Desktop. Give Claude real-time access to your browser's console logs, network errors, and DOM state."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['mcp', 'integration', 'claude', 'desktop']
---

KaBOOM is an open-source MCP server that gives Claude Desktop real-time access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Configuration

Edit the Claude Desktop config file:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "kaboom": {
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser"]
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

1. **Restart Claude Desktop** after editing config
2. **Check file location** — path is OS-specific
3. **Verify JSON syntax** — invalid JSON fails silently
4. **Check the KaBOOM extension popup** — it should show "Connected"
