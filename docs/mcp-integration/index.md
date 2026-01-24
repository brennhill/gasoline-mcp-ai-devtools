---
title: "Fuel Any Agent"
description: "Connect Gasoline to any MCP-compatible coding agent. Configuration guides for Claude Code, Cursor, Windsurf, Claude Desktop, Zed, and VS Code with Continue."
keywords: "MCP server configuration, Model Context Protocol, autonomous coding agent, agentic debugging, browser debugging MCP"
permalink: /mcp-integration/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "One config. Your AI tool fires up Gasoline automatically."
toc: true
toc_sticky: true
---

Gasoline implements the [Model Context Protocol](https://modelcontextprotocol.io/) — a standard for connecting AI assistants to external tools. Any MCP-compatible tool can tap into your browser state.

## <i class="fas fa-plug"></i> Supported Tools

| Tool | Config Location | Guide |
|------|----------------|-------|
| <i class="fas fa-terminal"></i> Claude Code | `.mcp.json` (project root) | [Setup →](/mcp-integration/claude-code/) |
| <i class="fas fa-i-cursor"></i> Cursor | `~/.cursor/mcp.json` | [Setup →](/mcp-integration/cursor/) |
| <i class="fas fa-wind"></i> Windsurf | `~/.codeium/windsurf/mcp_config.json` | [Setup →](/mcp-integration/windsurf/) |
| <i class="fas fa-desktop"></i> Claude Desktop | OS-specific config file | [Setup →](/mcp-integration/claude-desktop/) |
| <i class="fas fa-bolt"></i> Zed | `~/.config/zed/settings.json` | [Setup →](/mcp-integration/zed/) |
| <i class="fas fa-code"></i> VS Code + Continue | `~/.continue/config.json` | [Below](#-vs-code-with-continue) |

## <i class="fas fa-fire"></i> How MCP Mode Works

With `--mcp`, Gasoline runs as a dual-mode server:

- <i class="fas fa-server"></i> **HTTP server** — background process for the browser extension
- <i class="fas fa-exchange-alt"></i> **MCP protocol** — stdio channel for your AI tool
- <i class="fas fa-magic"></i> **Auto-managed** — your AI tool starts and stops the server

## <i class="fas fa-tools"></i> Available MCP Tools

Once the pipeline is connected:

| Tool | Description |
|------|-------------|
| `get_browser_errors` | <i class="fas fa-exclamation-triangle"></i> Console errors, network failures, exceptions |
| `get_browser_logs` | <i class="fas fa-list"></i> All logs (errors, warnings, info) |
| `clear_browser_logs` | <i class="fas fa-eraser"></i> Clear the log file |
| `get_websocket_events` | <i class="fas fa-plug"></i> WebSocket events. Filter by URL, ID, direction |
| `get_websocket_status` | <i class="fas fa-signal"></i> Connection states, rates, schemas |
| `get_network_bodies` | <i class="fas fa-exchange-alt"></i> Request/response bodies. Filter by URL, method, status |
| `query_dom` | <i class="fas fa-code"></i> Live DOM query with CSS selectors |
| `get_page_info` | <i class="fas fa-info-circle"></i> Page URL, title, viewport |
| `run_accessibility_audit` | <i class="fas fa-universal-access"></i> Accessibility violations |

## <i class="fas fa-cog"></i> Custom Port

If port 7890 is occupied:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp", "--port", "7891"]
    }
  }
}
```

Update the extension's Server URL in Options to match.

## <i class="fas fa-code"></i> VS Code with Continue

Add to `~/.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "npx",
          "args": ["gasoline-mcp", "--mcp"]
        }
      }
    ]
  }
}
```

## <i class="fas fa-check-circle"></i> Verify the Connection

1. Restart your AI tool
2. Gasoline server ignites automatically
3. Extension popup shows "Connected"
4. Ask your AI: _"What browser errors do you see?"_
