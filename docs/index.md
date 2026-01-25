---
title: "Gasoline — Browser Observability for AI Coding Agents"
description: "Autonomously debug and fix issues in real time. Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant. Enterprise ready."
keywords: "autonomous coding agent, browser debugging, MCP server, AI coding assistant, agentic debugging, vendor neutral MCP, local AI debugging, Claude Code, Cursor, Windsurf"
layout: splash
permalink: /
header:
  overlay_color: "#1a1f2e"
  actions:
    - label: "Fire It Up"
      url: /getting-started/
    - label: "GitHub"
      url: "https://github.com/brennhill/gasoline"
excerpt: "<div style='display: flex; align-items: center; gap: 2rem;'><div>Browser observability for AI coding agents - autonomously debug and fix issues in real time. Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant. Enterprise ready.<br><br><span style='color: #fb923c; font-size: 0.85em; font-style: italic;'>Pouring fuel on the AI development fire</span></div><img src='/assets/images/sparky-grill-vector.svg' alt='Sparky the mascot grilling' style='width: 200px; height: auto; flex-shrink: 0;'></div>"
---

## Now You're Cooking

One command. Your AI agent can see your browser.

```bash
npx gasoline-mcp
```


Gasoline is a **browser extension + local MCP server** that streams real-time browser data to autonomous coding agents. Console errors, failed API calls, uncaught exceptions, WebSocket traffic, live DOM state — your AI sees it all and fixes issues without you lifting a finger.

## Smart Teams Cook With Gasoline

**No debug port required.** Other tools need Chrome launched with `--remote-debugging-port`, which disables security sandboxing and breaks your normal browser workflow. Gasoline uses a standard extension — your browser stays secure and unmodified.

**Single binary, zero runtime.** No Node.js, no Python, no Puppeteer, no package.json. One Go binary that runs anywhere. No supply chain risk. No `node_modules`.

**Captures what others can't.** WebSocket messages, full request/response bodies, user action recording, Web Vitals with regression detection, API schema inference, and Playwright test generation from real browser sessions — features no other MCP browser tool offers.

**Works with every MCP tool.** Claude Code, Cursor, Windsurf, Zed, Claude Desktop, VS Code + Continue. Switch AI tools without changing your debugging setup.

**Enterprise-safe by design.** Binds to `127.0.0.1` only. Auth headers are stripped automatically. No telemetry, no accounts, no cloud. Audit the source — it's AGPL-3.0.

## How Gasoline Compares

| | Gasoline | Chrome DevTools MCP | BrowserTools MCP | Cursor Browser |
|---|:---:|:---:|:---:|:---:|
| **Console logs** | ✅ | ✅ | ✅ | ✅ |
| **Network errors** | ✅ | ✅ | ✅ | ❌ |
| **Network bodies** | ✅ | ❌ | ❌ | ❌ |
| **WebSocket events** | ✅ | ❌ | ❌ | ❌ |
| **User action recording** | ✅ | ❌ | ❌ | ❌ |
| **DOM queries** | ✅ | ✅ | ✅ | ✅ |
| **Screenshots** | ✅ | ✅ | ✅ | ✅ |
| | | | | |
| **[Web Vitals](/web-vitals/)** | ✅ LCP, CLS, INP, FCP | ❌ | ❌ | ❌ |
| **[Regression detection](/regression-detection/)** | ✅ Automatic | ❌ | ❌ | ❌ |
| **[API schema inference](/api-schema/)** | ✅ OpenAPI from traffic | ❌ | ❌ | ❌ |
| **[Accessibility audits](/accessibility-audit/)** | ✅ WCAG + SARIF | ❌ | ❌ | ❌ |
| **[Session checkpoints](/session-checkpoints/)** | ✅ Named + auto | ❌ | ❌ | ❌ |
| **[Noise filtering](/noise-filtering/)** | ✅ Auto-detect | ❌ | ❌ | ❌ |
| | | | | |
| **[Test generation](/generate-test/)** | ✅ Playwright | ❌ | ❌ | ❌ |
| **[Reproduction scripts](/reproduction-scripts/)** | ✅ From actions | ❌ | ❌ | ❌ |
| **[PR summaries](/pr-summaries/)** | ✅ Perf impact | ❌ | ❌ | ❌ |
| **[HAR export](/har-export/)** | ✅ | ❌ | ❌ | ❌ |
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

- **[Claude Code](/mcp-integration/claude-code/)** — `.mcp.json` in project root
- **[Cursor](/mcp-integration/cursor/)** — `~/.cursor/mcp.json`
- **[Windsurf](/mcp-integration/windsurf/)** — `~/.codeium/windsurf/mcp_config.json`
- **[Claude Desktop](/mcp-integration/claude-desktop/)** — OS-specific config
- **[Zed](/mcp-integration/zed/)** — `~/.config/zed/settings.json`
- **VS Code + Continue** — `~/.continue/config.json`

Not tied to Anthropic. Not tied to Cursor. Not tied to anyone. If your agent speaks MCP, Gasoline fuels it.

## The Pipeline

```
[ Browser ] → [ Extension ] → [ localhost:7890 ] → [ Any MCP AI ]
                                    ↕
                              Stays on your machine
```

1. The extension passively captures your browser activity
2. Data flows to a local server — never the internet
3. Your AI tool reads it via MCP (stdio, not network)
4. **Nothing leaves your machine** — private by architecture

## What Gets Captured

- **Console Logs** — `console.log()`, `.warn()`, `.error()` with full arguments
- **Network Errors** — Failed API calls (4xx/5xx) with response bodies
- **Exceptions** — Uncaught errors with full stack traces
- **[WebSocket Events](/websocket-monitoring/)** — Connection lifecycle and message payloads
- **[Network Bodies](/network-bodies/)** — Request/response payloads for API debugging
- **User Actions** — Click, type, navigate, scroll recording with smart selectors
- **Web Vitals** — LCP, CLS, INP, FCP with automatic regression detection
- **[Live DOM](/dom-queries/)** — Query the page with CSS selectors via MCP
- **[Accessibility](/accessibility-audit/)** — WCAG audits with SARIF export
- **API Schema Inference** — Auto-discover OpenAPI from captured traffic
- **Session Checkpoints** — Save state, diff changes, detect regressions over time
- **[Test Generation](/generate-test/)** — Playwright tests and reproduction scripts from actions
- **Noise Filtering** — Auto-detect and dismiss irrelevant errors
- **[Context API](/developer-api/)** — Annotate errors with `window.__gasoline`

## Zero Bloat, Zero Risk

- **Single binary** — Go. No Node.js, no Python, no runtime deps.
- **< 0.1ms overhead** — Per console intercept. Your browsing stays fast.
- **20MB memory cap** — The extension never bloats your browser.
- **Zero network calls** — The binary never connects to the internet.
- **No dependencies** — No supply chain risk. One binary, auditable.

---

<i class="fas fa-shield-alt"></i> [Security & Privacy](/security/) · <i class="fas fa-building"></i> [Enterprise Ready](/security/#enterprise-ready) · <i class="fas fa-bolt"></i> [Performance SLOs](/performance-slos/) · <i class="fas fa-map"></i> [Roadmap](/roadmap/) · <i class="fas fa-code-branch"></i> [Developer API](/developer-api/) · <i class="fas fa-balance-scale"></i> [Alternatives Compared](/alternatives/) · <i class="fab fa-github"></i> [GitHub](https://github.com/brennhill/gasoline)

[Fire It Up →](/getting-started/){: .btn .btn--primary .btn--large}

<em style="color: #fb923c;">Pouring fuel on the AI development fire</em>
