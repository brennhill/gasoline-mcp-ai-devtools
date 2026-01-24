---
title: "Gasoline + Cursor"
description: "Configure Gasoline as an MCP server for Cursor IDE. Give Cursor's AI real-time access to browser console logs, network errors, and exceptions."
keywords: "Cursor MCP server, Cursor browser debugging, Cursor AI browser logs, Cursor MCP extension"
permalink: /mcp-integration/cursor/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Fuel Cursor's AI with live browser data."
toc: true
toc_sticky: true
---

## <i class="fas fa-file-code"></i> Configuration

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

## <i class="fas fa-fire-alt"></i> Usage

After restarting Cursor, the AI can:

- <i class="fas fa-exclamation-triangle"></i> See console errors and warnings
- <i class="fas fa-wifi"></i> Inspect failed network requests with response bodies
- <i class="fas fa-code"></i> Query the live DOM with CSS selectors
- <i class="fas fa-plug"></i> Check WebSocket connection states
- <i class="fas fa-universal-access"></i> Run accessibility audits

Ask: _"What browser errors are happening?"_ — Cursor queries Gasoline automatically.

## <i class="fas fa-wrench"></i> Troubleshooting

1. **Restart Cursor** after adding config
2. **Check MCP status** in settings panel
3. **Verify extension** shows "Connected" in popup
