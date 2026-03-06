---
title: Gasoline + Antigravity
description: "Configure Gasoline as an MCP server for Google Antigravity. Give Antigravity's AI agent access to browser console logs, network errors, and DOM state."
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['mcp', 'integration', 'antigravity']
---

Gasoline is an open-source MCP server that gives Antigravity's AI agent access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

```bash
gasoline-mcp --install antigravity
```

## Manual Configuration

Add to `~/.gemini/antigravity/mcp_config.json`:

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

> Antigravity does not support `${workspaceFolder}` — use absolute paths only.

## Usage

After configuring, open an Agent session in Antigravity. The AI agent can access Gasoline's full MCP toolset.

## Troubleshooting

1. **Restart Antigravity** after editing config
2. **Use absolute paths** — `${workspaceFolder}` is not supported
3. **Verify extension popup** shows "Connected"
