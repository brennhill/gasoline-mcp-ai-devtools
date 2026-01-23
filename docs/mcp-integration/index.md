---
title: "MCP Integration"
description: "Connect Gasoline to any MCP-compatible AI coding assistant. Configuration guides for Claude Code, Cursor, Windsurf, Claude Desktop, Zed, and VS Code with Continue."
keywords: "MCP server configuration, Model Context Protocol, AI coding assistant integration, browser debugging MCP"
permalink: /mcp-integration/
toc: true
toc_sticky: true
---

Gasoline implements the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP), a standard for connecting AI assistants to external tools. This means any MCP-compatible tool can access your browser state.

## Supported Tools

| Tool | Config Location | Guide |
|------|----------------|-------|
| Claude Code | `.mcp.json` (project root) | [Setup →](/mcp-integration/claude-code/) |
| Cursor | `~/.cursor/mcp.json` | [Setup →](/mcp-integration/cursor/) |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | [Setup →](/mcp-integration/windsurf/) |
| Claude Desktop | OS-specific config file | [Setup →](/mcp-integration/claude-desktop/) |
| Zed | `~/.config/zed/settings.json` | [Setup →](/mcp-integration/zed/) |
| VS Code + Continue | `~/.continue/config.json` | [Below](#vs-code-with-continue) |

## How MCP Mode Works

When you add `--mcp` to the command, Gasoline runs as an MCP server:

- **HTTP server** runs in the background for the browser extension
- **MCP protocol** runs over stdio for your AI tool
- Your AI tool starts and manages the server process automatically

## Available MCP Tools

Once connected, your AI assistant has access to:

| Tool | Description |
|------|-------------|
| `get_browser_errors` | Recent browser errors (console errors, network failures, exceptions) |
| `get_browser_logs` | All browser logs (errors, warnings, info) |
| `clear_browser_logs` | Clear the log file |
| `get_websocket_events` | WebSocket events (messages, lifecycle). Filter by URL, connection ID, or direction |
| `get_websocket_status` | WebSocket connection states, message rates, and schemas |
| `get_network_bodies` | Network request/response bodies. Filter by URL, method, or status code |
| `query_dom` | Query the live DOM using a CSS selector |
| `get_page_info` | Current page URL, title, and viewport |
| `run_accessibility_audit` | Run an accessibility audit on the current page |

## Custom Port

If port 7890 is in use, specify a different port:

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

Remember to update the extension's Server URL in Options to match.

## VS Code with Continue

Add to Continue's config (`~/.continue/config.json`):

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

## Verifying the Connection

After configuring:

1. Restart your AI tool to load the new MCP server
2. The Gasoline server should start automatically
3. Check the extension popup — it should show "Connected"
4. Ask your AI: _"What browser errors do you see?"_
