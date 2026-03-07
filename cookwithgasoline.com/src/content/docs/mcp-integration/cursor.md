---
title: Gasoline + Cursor
description: "Configure Gasoline as an MCP server for Cursor IDE. Give Cursor's AI real-time access to browser console logs, network errors, and exceptions."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['mcp', 'integration', 'cursor']
---

Gasoline is an open-source MCP server that gives Cursor's AI real-time access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Configuration

Add to `~/.cursor/mcp.json`:

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

Or use Cursor's UI: **Settings → MCP Servers → Add Server**:

```json
{
  "gasoline": {
    "command": "npx",
    "args": ["gasoline-mcp"]
  }
}
```

## Usage

After restarting Cursor, the AI can:

- See console errors and warnings
- Inspect failed network requests with response bodies
- Query the live DOM with CSS selectors
- Check WebSocket connection states
- Run accessibility audits

Ask: _"What browser errors are happening?"_ — Cursor queries Gasoline automatically.

## Troubleshooting

1. **Restart Cursor** after adding config
2. **Check MCP status** in settings panel
3. **Verify extension** shows "Connected" in popup
