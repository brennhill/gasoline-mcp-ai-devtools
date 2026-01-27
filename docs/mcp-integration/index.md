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

**Critical:** Your AI tool spawns a SINGLE Gasoline process that handles both:

- <i class="fas fa-server"></i> **HTTP server** (port 7890) — for browser extension telemetry capture
- <i class="fas fa-exchange-alt"></i> **MCP stdio** — for AI tool commands (observe, generate, configure, interact)

**Both interfaces share the same browser state.** Do NOT manually start Gasoline with `npx gasoline-mcp` or `go run` — let your AI tool's MCP system spawn and manage the process. If you have a manually-started Gasoline instance on port 7890, kill it first to avoid conflicts.

## <i class="fas fa-tools"></i> Available MCP Tools

Gasoline exposes **5 composite tools** — each with multiple sub-modes controlled by a single parameter. This reduces decision space by 79% compared to individual tool calls.

| Tool | What it does | Sub-modes |
|------|-------------|-----------|
| `observe` | <i class="fas fa-eye"></i> Real-time browser state | errors, logs, network, websocket_events, websocket_status, actions, vitals, page |
| `analyze` | <i class="fas fa-chart-line"></i> Data analysis and insights | performance, api, accessibility, changes, timeline |
| `generate` | <i class="fas fa-code"></i> Code generation from captured data | reproduction, test, pr_summary, sarif, har |
| `configure` | <i class="fas fa-cog"></i> Session and noise management | store, noise_rule, dismiss, clear |
| `query_dom` | <i class="fas fa-search"></i> Live DOM inspection | CSS selector queries |

### observe

Monitor browser state in real time. Use the `what` parameter to select:

| Mode | Returns |
|------|---------|
| `errors` | Console errors with deduplication and noise filtering |
| `logs` | All console output (configurable limit) |
| `network` | HTTP requests — filter by URL, method, status range |
| `websocket_events` | WebSocket messages — filter by connection ID, direction |
| `websocket_status` | Active WebSocket connections and stats |
| `actions` | User interactions (click, input, navigate, scroll, select) |
| `vitals` | Core Web Vitals — FCP, LCP, CLS, INP |
| `page` | Current page URL, title, viewport |

### analyze

Derive insights from captured data. Use the `target` parameter:

| Target | Returns |
|--------|---------|
| `performance` | Performance snapshot with regression detection against baselines |
| `api` | Auto-inferred API schema (OpenAPI stub or gasoline format) |
| `accessibility` | WCAG audit results with caching |
| `changes` | Compressed state diff since a named/auto checkpoint |
| `timeline` | Merged session timeline (actions + network + errors) |

### generate

Generate code artifacts from your session. Use the `format` parameter:

| Format | Output |
|--------|--------|
| `reproduction` | Playwright script reproducing user actions |
| `test` | Playwright test with network/error assertions |
| `pr_summary` | Markdown performance impact summary |
| `sarif` | SARIF accessibility report (standard format) |
| `har` | HTTP Archive export |

### configure

Manage session state and noise filtering. Use the `action` parameter:

| Action | Effect |
|--------|--------|
| `store` | Persistent key-value storage (save/load/list/delete/stats) |
| `noise_rule` | Manage noise filtering rules (add/remove/list/auto_detect) |
| `dismiss` | Quick one-off noise dismissal by regex pattern |
| `clear` | Clear all browser logs |

## <i class="fas fa-cog"></i> Custom Port

If port 7890 is occupied:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--port", "7891"]
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
          "args": ["gasoline-mcp"]
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

## <i class="fas fa-exclamation-triangle"></i> Troubleshooting

### "bind: address already in use" error

If MCP fails to start with a port conflict:

```bash
# Find and kill existing Gasoline process
ps aux | grep gasoline | grep -v grep
kill <PID>

# Or if on macOS/Linux:
pkill -f gasoline
```

Then reload your MCP connection. The MCP system will spawn a fresh instance.

### Extension shows "Disconnected"

- Check that your AI tool has started the MCP server (look for Gasoline in process list)
- Verify the extension's Server URL matches the port in your MCP config (default: `http://localhost:7890`)
- Try restarting your AI tool to re-initialize the MCP connection

### No browser data appearing in AI responses

1. Open the extension popup and verify "Connected" status
2. Check capture level is not set to "Errors Only" if you expect all logs
3. Refresh the browser page to ensure content script is injected
4. Ask your AI to run: `observe({what: "page"})` to verify MCP communication
