---
title: "Gasoline — Browser Debugging for AI Coding Assistants"
description: "Capture browser console logs, network errors, WebSocket events, and DOM state. Feed them to Claude Code, Cursor, Windsurf, or any MCP-compatible AI assistant."
keywords: "browser debugging, MCP server, AI coding assistant, console log capture, browser extension, Claude Code, Cursor, Windsurf"
layout: splash
permalink: /
header:
  overlay_image: /assets/images/hero-bg.png
  overlay_filter: 0.6
  actions:
    - label: "Get Started"
      url: /getting-started/
    - label: "View on GitHub"
      url: "https://github.com/brennhill/gasoline"
excerpt: "Stop copy-pasting browser errors. Gasoline captures console logs, network failures, exceptions, WebSocket events, and more — then makes them available to your AI coding assistant via MCP."
---

## What is Gasoline?

Gasoline is a **browser extension + local MCP server** that gives AI coding assistants real-time access to what's happening in your browser.

Your AI can see console errors, failed API calls, uncaught exceptions, WebSocket traffic, and even query the live DOM — without you copying anything.

```bash
npx gasoline-mcp
```

That's it. One command starts the server. Install the Chrome extension, connect your AI tool, and your assistant can now see your browser.

## How It Works

```
Browser → Extension → Local Server → AI Assistant (via MCP)
```

1. The extension passively observes your browser (console, network, errors)
2. Logs are sent to a local server on `localhost:7890`
3. Your AI tool reads them via the [Model Context Protocol](https://modelcontextprotocol.io/)
4. **Nothing leaves your machine** — 100% local

## Works With Every MCP-Compatible Tool

| Tool | Status |
|------|--------|
| [Claude Code](/mcp-integration/claude-code/) | Supported |
| [Cursor](/mcp-integration/cursor/) | Supported |
| [Windsurf](/mcp-integration/windsurf/) | Supported |
| [Claude Desktop](/mcp-integration/claude-desktop/) | Supported |
| [Zed](/mcp-integration/zed/) | Supported |
| VS Code + Continue | Supported |

## Key Capabilities

- **Console Capture** — `console.log()`, `.warn()`, `.error()` with full arguments
- **Network Errors** — Failed API calls (4xx/5xx) with method, URL, status, and response body
- **Exception Tracking** — Uncaught errors and promise rejections with stack traces
- **[WebSocket Monitoring](/websocket-monitoring/)** — Connection lifecycle and message payloads
- **[Network Body Capture](/network-bodies/)** — Request/response payloads for API debugging
- **[Live DOM Queries](/dom-queries/)** — Query the page with CSS selectors via MCP
- **[Accessibility Audit](/accessibility-audit/)** — Run axe-core audits from your AI tool
- **[Developer API](/developer-api/)** — Add custom context with `window.__gasoline.annotate()`

## Zero Dependencies, Zero Overhead

- **Single Go binary** — no Node.js runtime, no Python, no Java
- **< 0.1ms** per console intercept — your browsing stays fast
- **20MB memory cap** — never bloats your browser
- **Localhost only** — nothing leaves your machine

[Get Started in 2 Minutes →](/getting-started/){: .btn .btn--primary .btn--large}
