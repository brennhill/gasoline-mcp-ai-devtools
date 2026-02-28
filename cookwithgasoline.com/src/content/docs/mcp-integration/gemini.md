---
title: Gasoline + Gemini CLI
description: "Configure Gasoline as an MCP server for Gemini CLI. Give Gemini access to browser console logs, network errors, and DOM state."
---

Gasoline is an open-source MCP server that gives Gemini CLI access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

```bash
gasoline-mcp --install gemini
```

## Manual Configuration

Add to `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp",
      "args": []
    }
  }
}
```

If using npx:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["-y", "gasoline-mcp"]
    }
  }
}
```

## Usage

After configuring, Gemini CLI can access Gasoline's full MCP toolset.

## Troubleshooting

1. **Restart Gemini CLI** after editing settings
2. **Verify extension popup** shows "Connected"
3. **Test**: Ask Gemini _"What browser errors do you see?"_
