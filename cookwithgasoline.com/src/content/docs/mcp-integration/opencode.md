---
title: Gasoline + OpenCode
description: "Configure Gasoline as an MCP server for OpenCode. Give OpenCode access to browser console logs, network errors, and DOM state."
---

Gasoline is an open-source MCP server that gives OpenCode access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

```bash
gasoline-mcp --install opencode
```

## Manual Configuration

Add to `~/.config/opencode/opencode.json`:

```json
{
  "mcp": {
    "gasoline": {
      "type": "local",
      "command": ["gasoline-mcp"],
      "enabled": true
    }
  }
}
```

> OpenCode uses the `mcp` key (not `mcpServers`) and expects `command` as an array.

## Usage

After configuring, OpenCode can access Gasoline's full MCP toolset.

## Troubleshooting

1. **Restart OpenCode** after editing config
2. **Check the config key** — must be `mcp`, not `mcpServers`
3. **Verify extension popup** shows "Connected"
