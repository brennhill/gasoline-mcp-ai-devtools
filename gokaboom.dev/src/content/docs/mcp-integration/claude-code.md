---
title: KaBOOM + Claude Code
description: "Configure KaBOOM as an MCP server for Claude Code. Give Claude real-time access to browser console logs, network errors, and DOM state."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['mcp', 'integration', 'claude', 'code']
---

KaBOOM is an open-source MCP server that gives Claude Code real-time access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Project-Level Config (Recommended)

Create `.mcp.json` in your project root:

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

KaBOOM only fires up when you're in this project.

## Global Config

Available in all projects — add to `~/.claude/settings.json`:

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

Once configured, Claude Code auto-ignites the server. Ask:

- _"What browser errors do you see?"_
- _"Check the network responses for /api/users"_
- _"Run an accessibility audit on this page"_
- _"What's the DOM structure of the nav?"_
- _"Any WebSocket connection issues?"_

Claude uses the right MCP tool and returns actionable debugging info.

## Troubleshooting

1. **Restart Claude Code** after adding config
2. **Check the KaBOOM extension popup** — it should show "Connected"
3. **Verify tools**: Ask _"What MCP tools do you have?"_
4. **Check logs**: Look for MCP connection errors in output
