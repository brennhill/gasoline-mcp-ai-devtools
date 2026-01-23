---
title: "Gasoline — Fuel for the AI Fire"
description: "Capture browser console logs, network errors, WebSocket events, and DOM state. Feed them to Claude Code, Cursor, Windsurf, or any MCP-compatible AI assistant."
keywords: "browser debugging, MCP server, AI coding assistant, console log capture, browser extension, Claude Code, Cursor, Windsurf"
layout: splash
permalink: /
header:
  overlay_color: "#1a1f2e"
  actions:
    - label: "Fire It Up"
      url: /getting-started/
    - label: "GitHub"
      url: "https://github.com/brennhill/gasoline"
excerpt: "Stop copy-pasting browser errors into your AI. Gasoline captures everything — console logs, network failures, exceptions, WebSocket events — and feeds it directly to your AI coding assistant."
---

## Now You're Cooking

One command. Your AI assistant can see your browser.

```bash
npx gasoline-mcp
```

Gasoline is a **browser extension + local MCP server** that fuels AI coding assistants with real-time browser data. Console errors, failed API calls, uncaught exceptions, WebSocket traffic, live DOM state — your AI sees it all without you lifting a finger.

## The Pipeline

```
[ Browser ] → [ Extension ] → [ Local Server ] → [ AI ]
```

1. The extension passively captures your browser activity
2. Data flows to a local server on `localhost:7890`
3. Your AI tool reads it via [MCP](https://modelcontextprotocol.io/)
4. **Nothing leaves your machine** — 100% local, zero telemetry

## Fuel Any MCP-Compatible Tool

| Tool | Setup Guide |
|------|-------------|
| [Claude Code](/mcp-integration/claude-code/) | `.mcp.json` in project root |
| [Cursor](/mcp-integration/cursor/) | `~/.cursor/mcp.json` |
| [Windsurf](/mcp-integration/windsurf/) | `~/.codeium/windsurf/mcp_config.json` |
| [Claude Desktop](/mcp-integration/claude-desktop/) | OS-specific config |
| [Zed](/mcp-integration/zed/) | `~/.config/zed/settings.json` |
| VS Code + Continue | `~/.continue/config.json` |

## What Gets Captured

- **Console Logs** — `console.log()`, `.warn()`, `.error()` with full arguments
- **Network Errors** — Failed API calls (4xx/5xx) with response bodies
- **Exceptions** — Uncaught errors with full stack traces
- **[WebSocket Events](/websocket-monitoring/)** — Connection lifecycle and message payloads
- **[Network Bodies](/network-bodies/)** — Request/response payloads for API debugging
- **[Live DOM](/dom-queries/)** — Query the page with CSS selectors via MCP
- **[Accessibility](/accessibility-audit/)** — Run axe-core audits from your AI
- **[Context API](/developer-api/)** — Annotate errors with `window.__gasoline`

## Zero Bloat

| | |
|---|---|
| **Single binary** | Go. No Node.js, no Python, no runtime deps. |
| **< 0.1ms overhead** | Per console intercept. Your browsing stays fast. |
| **20MB memory cap** | The extension never bloats your browser. |
| **Localhost only** | Data never leaves your machine. |

[Fire It Up →](/getting-started/){: .btn .btn--primary .btn--large}
