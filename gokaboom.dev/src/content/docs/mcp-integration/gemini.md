---
title: KaBOOM + Gemini CLI
description: "Configure KaBOOM as an MCP server for Gemini CLI. Give Gemini access to browser console logs, network errors, and DOM state."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['mcp', 'integration', 'gemini']
---

KaBOOM is an open-source MCP server that gives Gemini CLI access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

```bash
kaboom-agentic-browser --install gemini
```

## Manual Configuration

Add to `~/.gemini/settings.json`:

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

## Usage

After configuring, Gemini CLI can access KaBOOM's full MCP toolset.

## Troubleshooting

1. **Restart Gemini CLI** after editing settings
2. **Verify the KaBOOM extension popup** shows "Connected"
3. **Test**: Ask Gemini _"What browser errors do you see?"_
