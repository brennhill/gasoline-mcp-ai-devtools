---
title: "Gasoline + Claude Code Setup"
description: "Configure Gasoline as an MCP server for Claude Code. Give Claude real-time access to browser console logs, network errors, and DOM state."
keywords: "Claude Code MCP server, Claude Code browser errors, Claude Code debugging, Claude Code browser extension"
permalink: /mcp-integration/claude-code/
toc: true
toc_sticky: true
---

Claude Code is Anthropic's CLI tool for AI-assisted coding. Gasoline connects to Claude Code via MCP, giving it real-time access to your browser state.

## Project-Level Config (Recommended)

Create `.mcp.json` in your project root:

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

This config is project-specific — Gasoline only runs when you're in this project.

## Global Config

To make Gasoline available in all projects, add to `~/.claude/settings.json`:

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

Once configured, Claude Code automatically starts the Gasoline server. You can ask:

- _"What browser errors do you see?"_
- _"Check the network responses for the /api/users endpoint"_
- _"Run an accessibility audit on the current page"_
- _"What's the current DOM structure of the nav element?"_
- _"Are there any WebSocket connection issues?"_

Claude Code will use the appropriate MCP tool to fetch the data and respond with actionable debugging information.

## Troubleshooting

1. **Restart Claude Code** after adding the config
2. **Check the extension popup** — should show "Connected"
3. **Verify with**: Ask Claude _"What MCP tools do you have available?"_
4. **Check logs**: Look for MCP connection errors in Claude Code's output
