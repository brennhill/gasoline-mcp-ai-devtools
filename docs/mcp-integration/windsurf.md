---
title: "Gasoline + Windsurf Setup"
description: "Configure Gasoline as an MCP server for Windsurf (Codeium). Give Windsurf's AI access to browser console logs, network errors, and page state."
keywords: "Windsurf MCP server, Windsurf browser debugging, Codeium MCP, Windsurf AI browser logs"
permalink: /mcp-integration/windsurf/
toc: true
toc_sticky: true
---

Windsurf (by Codeium) is an AI-powered IDE. Gasoline connects via MCP to give its AI real-time browser visibility.

## Configuration

Add to `~/.codeium/windsurf/mcp_config.json`:

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

## Usage

After restarting Windsurf, the AI can access all browser state:

- Console errors and warnings
- Failed network requests with response bodies
- WebSocket events and connection states
- Live DOM queries
- Accessibility audit results

## Troubleshooting

1. **Restart Windsurf** after adding the config
2. **Check the extension popup** — should show "Connected"
3. **Verify the config path** — must be exactly `~/.codeium/windsurf/mcp_config.json`
