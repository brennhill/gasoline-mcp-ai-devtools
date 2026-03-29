---
title: KaBOOM + Zed
description: "Configure KaBOOM as a context server for Zed editor. Give Zed's AI assistant access to browser console logs, network errors, and DOM state."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['mcp', 'integration', 'zed']
---

KaBOOM is an open-source MCP server that gives Zed's AI assistant access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

```bash
kaboom-agentic-browser --install zed
```

## Manual Configuration

Add to `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "kaboom": {
      "source": "custom",
      "command": "kaboom-agentic-browser",
      "args": []
    }
  }
}
```

If using npx:

```json
{
  "context_servers": {
    "kaboom": {
      "source": "custom",
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser"]
    }
  }
}
```

> Zed uses `context_servers` instead of `mcpServers`, and entries require `"source": "custom"` for manually configured servers.

## Usage

After restarting Zed, the AI assistant taps into KaBOOM's full MCP toolset.

## Troubleshooting

1. **Restart Zed** after editing settings
2. **Check the config key** — must be `context_servers`
3. **Verify the KaBOOM extension popup** shows "Connected"
