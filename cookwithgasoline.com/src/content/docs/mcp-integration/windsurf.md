---
title: Gasoline + Windsurf
description: "Configure Gasoline as an MCP server for Windsurf (Codeium). Give Windsurf's AI access to browser console logs, network errors, and page state."
---

Gasoline is an open-source MCP server that gives Windsurf's AI access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Configuration

Add to `~/.codeium/windsurf/mcp_config.json`:

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

## Usage

After restarting Windsurf, the AI can access all browser state:

- Console errors and warnings
- Failed network requests with response bodies
- WebSocket events and connection states
- Live DOM queries
- Accessibility audit results

## Troubleshooting

1. **Restart Windsurf** after adding config
2. **Check extension popup** — should show "Connected"
3. **Verify config path** — must be `~/.codeium/windsurf/mcp_config.json`
