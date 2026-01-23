---
title: "Gasoline + Cursor Setup"
description: "Configure Gasoline as an MCP server for Cursor IDE. Give Cursor's AI real-time access to browser console logs, network errors, and exceptions."
keywords: "Cursor MCP server, Cursor browser debugging, Cursor AI browser logs, Cursor MCP extension"
permalink: /mcp-integration/cursor/
toc: true
toc_sticky: true
---

Cursor is an AI-powered code editor. Gasoline connects to Cursor via MCP, letting its AI see your browser errors in real time.

## Configuration

Add to `~/.cursor/mcp.json`:

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

Or use Cursor's UI: **Settings → MCP Servers → Add Server** and paste:

```json
{
  "gasoline": {
    "command": "npx",
    "args": ["gasoline-mcp", "--mcp"]
  }
}
```

## Usage

After restarting Cursor, the AI can:

- See console errors and warnings from your app
- Inspect failed network requests with response bodies
- Query the live DOM with CSS selectors
- Check WebSocket connection states
- Run accessibility audits

Ask Cursor's AI: _"What browser errors are happening?"_ and it will query Gasoline automatically.

## Troubleshooting

1. **Restart Cursor** after adding the config
2. **Check MCP status** in Cursor's settings panel
3. **Verify the extension** shows "Connected" in the popup
