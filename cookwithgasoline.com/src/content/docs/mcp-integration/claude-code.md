---
title: Gasoline + Claude Code
description: "Configure Gasoline as an MCP server for Claude Code. Give Claude real-time access to browser console logs, network errors, and DOM state."
---

Gasoline is an open-source MCP server that gives Claude Code real-time access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Project-Level Config (Recommended)

Create `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp"]
    }
  }
}
```

Gasoline only fires up when you're in this project.

## Global Config

Available in all projects — add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp"]
    }
  }
}
```

## Usage

Once configured, Claude Code auto-ignites the server. Ask:

- _"What browser errors do you see?"_
- _"Check the network responses for /api/users"_
- _"Run an accessibility audit on this page"_
- _"What's the DOM structure of the nav?"_
- _"Any WebSocket connection issues?"_

Claude uses the right MCP tool and returns actionable debugging info.

## Troubleshooting

1. **Restart Claude Code** after adding config
2. **Check extension popup** — should show "Connected"
3. **Verify tools**: Ask _"What MCP tools do you have?"_
4. **Check logs**: Look for MCP connection errors in output
