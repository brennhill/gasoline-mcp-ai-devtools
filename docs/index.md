---
title: "Gasoline — Fuel for the AI Fire"
description: "Enterprise-ready, vendor-neutral browser debugging for AI. Capture console logs, network errors, WebSocket events, and DOM state — 100% local, zero data shared with any provider."
keywords: "browser debugging, MCP server, AI coding assistant, enterprise browser debugging, vendor neutral MCP, local AI debugging, no data sharing, Claude Code, Cursor, Windsurf"
layout: splash
permalink: /
header:
  overlay_color: "#1a1f2e"
  actions:
    - label: "Fire It Up"
      url: /getting-started/
    - label: "GitHub"
      url: "https://github.com/brennhill/gasoline"
excerpt: "Vendor-neutral. Enterprise-ready. Zero data shared with any provider. Gasoline captures browser logs, network failures, exceptions, and WebSocket events — and feeds them to any MCP-compatible AI assistant, entirely on your machine."
---

## Now You're Cooking

One command. Your AI assistant can see your browser.

```bash
npx gasoline-mcp
```

Gasoline is a **browser extension + local MCP server** that fuels AI coding assistants with real-time browser data. Console errors, failed API calls, uncaught exceptions, WebSocket traffic, live DOM state — your AI sees it all without you lifting a finger.

## Smart Teams Cook With Gasoline

**No debug port required.** Other tools need Chrome launched with `--remote-debugging-port`, which disables security sandboxing and breaks your normal browser workflow. Gasoline uses a standard extension — your browser stays secure and unmodified.

**Single binary, zero runtime.** No Node.js, no Python, no Puppeteer, no package.json. One Go binary that runs anywhere. No supply chain risk. No `node_modules`.

**Captures what others can't.** WebSocket messages, full request/response bodies, and Playwright test generation from real browser sessions — features no other MCP browser tool offers.

**Works with every MCP tool.** Claude Code, Cursor, Windsurf, Zed, Claude Desktop, VS Code + Continue. Switch AI tools without changing your debugging setup.

**Enterprise-safe by design.** Binds to `127.0.0.1` only. Auth headers are stripped automatically. No telemetry, no accounts, no cloud. Audit the source — it's AGPL-3.0.

## How Gasoline Compares

| | Gasoline | Chrome DevTools MCP | BrowserTools MCP | Cursor Browser |
|---|:---:|:---:|:---:|:---:|
| **Console logs** | ✅ | ✅ | ✅ | ✅ |
| **Network errors** | ✅ | ✅ | ✅ | ❌ |
| **Network bodies** | ✅ | ❌ | ❌ | ❌ |
| **WebSocket events** | ✅ | ❌ | ❌ | ❌ |
| **DOM queries** | ✅ | ✅ | ✅ | ✅ |
| **Accessibility audits** | ✅ | ❌ | ❌ | ❌ |
| **Test generation** | ✅ | ❌ | ❌ | ❌ |
| **Screenshots** | ✅ | ✅ | ✅ | ✅ |
| | | | | |
| **Zero dependencies** | ✅ Single Go binary | ❌ Node.js + Chrome flags | ❌ Node.js + Puppeteer | ❌ Electron |
| **Vendor neutral** | ✅ Any MCP tool | ⚠️ Any MCP tool | ⚠️ Any MCP tool | ❌ Cursor only |
| **No debug port** | ✅ | ❌ `--remote-debugging-port` | ❌ `--remote-debugging-port` | N/A |
| **Privacy** | ✅ Localhost only | ✅ Local | ⚠️ Optional cloud | ❌ Cursor servers |
| **Performance overhead** | < 0.1ms | ~5ms | ~5ms | Unknown |

[Full comparison →](/alternatives/)

## Enterprise Ready — Zero Data Leakage

**No browser data is ever shared with any AI provider.** Gasoline runs entirely on your machine:

- **Localhost only** — the server binds to `127.0.0.1`, unreachable from the network
- **No cloud, no accounts, no telemetry** — nothing phones home, ever
- **Auth headers stripped** — tokens and API keys are automatically redacted
- **Open source (AGPL-3.0)** — audit every line your security team cares about

Your browser logs stay on your hardware. The AI reads a local file via stdio. At no point does debugging data touch a third-party server — making Gasoline safe for regulated environments, proprietary codebases, and enterprise security policies.

[Full Security Details →](/security/)

## Ecosystem Neutral — No Vendor Lock-In

Gasoline implements the open **[Model Context Protocol](https://modelcontextprotocol.io/)** standard. Swap AI tools without changing your debugging setup:

| Tool | Setup Guide |
|------|-------------|
| [Claude Code](/mcp-integration/claude-code/) | `.mcp.json` in project root |
| [Cursor](/mcp-integration/cursor/) | `~/.cursor/mcp.json` |
| [Windsurf](/mcp-integration/windsurf/) | `~/.codeium/windsurf/mcp_config.json` |
| [Claude Desktop](/mcp-integration/claude-desktop/) | OS-specific config |
| [Zed](/mcp-integration/zed/) | `~/.config/zed/settings.json` |
| VS Code + Continue | `~/.continue/config.json` |

Not tied to Anthropic. Not tied to Cursor. Not tied to anyone. If your tool speaks MCP, Gasoline fuels it.

## The Pipeline

```
[ Browser ] → [ Extension ] → [ localhost:7890 ] → [ Any MCP AI ]
                                    ↕
                              Stays on your machine
```

1. The extension passively captures your browser activity
2. Data flows to a local server — never the internet
3. Your AI tool reads it via MCP (stdio, not network)
4. **Nothing leaves your machine** — compliant by design

## What Gets Captured

- **Console Logs** — `console.log()`, `.warn()`, `.error()` with full arguments
- **Network Errors** — Failed API calls (4xx/5xx) with response bodies
- **Exceptions** — Uncaught errors with full stack traces
- **[WebSocket Events](/websocket-monitoring/)** — Connection lifecycle and message payloads
- **[Network Bodies](/network-bodies/)** — Request/response payloads for API debugging
- **[Live DOM](/dom-queries/)** — Query the page with CSS selectors via MCP
- **[Accessibility](/accessibility-audit/)** — Run axe-core audits from your AI
- **[Test Generation](/generate-test/)** — Turn browser sessions into Playwright regression tests
- **[Context API](/developer-api/)** — Annotate errors with `window.__gasoline`

## Zero Bloat, Zero Risk

| | |
|---|---|
| **Single binary** | Go. No Node.js, no Python, no runtime deps. |
| **< 0.1ms overhead** | Per console intercept. Your browsing stays fast. |
| **20MB memory cap** | The extension never bloats your browser. |
| **Zero network calls** | The binary never connects to the internet. |
| **No dependencies** | No supply chain risk. One binary, auditable. |

[Fire It Up →](/getting-started/){: .btn .btn--primary .btn--large}
