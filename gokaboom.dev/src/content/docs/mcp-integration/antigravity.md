---
title: KaBOOM + Antigravity
description: "Configure KaBOOM as an MCP server for Google Antigravity. Give Antigravity's AI agent access to browser console logs, network errors, and DOM state."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['mcp', 'integration', 'antigravity']
---

KaBOOM is an open-source MCP server that gives Antigravity's AI agent access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

```bash
kaboom-agentic-browser --install antigravity
```

## Manual Configuration

Add to `~/.gemini/antigravity/mcp_config.json`:

```json
{
  "mcpServers": {
    "kaboom": {
      "command": "kaboom-agentic-browser",
      "args": []
    }
  }
}
```

If using npx:

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

> Antigravity does not support `${workspaceFolder}` — use absolute paths only.

## Usage

After configuring, open an Agent session in Antigravity. The AI agent can access KaBOOM's full MCP toolset.

## Troubleshooting

1. **Restart Antigravity** after editing config
2. **Use absolute paths** — `${workspaceFolder}` is not supported
3. **Verify the KaBOOM extension popup** shows "Connected"
