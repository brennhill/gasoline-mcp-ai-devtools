---
title: KaBOOM + OpenCode
description: "Configure KaBOOM as an MCP server for OpenCode. Give OpenCode access to browser console logs, network errors, and DOM state."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['mcp', 'integration', 'opencode']
---

KaBOOM is an open-source MCP server that gives OpenCode access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

```bash
kaboom-agentic-browser --install opencode
```

## Manual Configuration

Add to `~/.config/opencode/opencode.json`:

```json
{
  "mcp": {
    "kaboom": {
      "type": "local",
      "command": ["kaboom-agentic-browser"],
      "enabled": true
    }
  }
}
```

> OpenCode uses the `mcp` key (not `mcpServers`) and expects `command` as an array.

## Usage

After configuring, OpenCode can access KaBOOM's full MCP toolset.

## Troubleshooting

1. **Restart OpenCode** after editing config
2. **Check the config key** — must be `mcp`, not `mcpServers`
3. **Verify the KaBOOM extension popup** shows "Connected"
