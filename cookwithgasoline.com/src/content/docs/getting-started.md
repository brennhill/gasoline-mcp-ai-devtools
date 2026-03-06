---
title: Fire It Up
description: Install and configure Gasoline in under 2 minutes. Start streaming browser logs to your autonomous coding agent with a single command.
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['getting', 'started']
---

Gasoline is an open-source browser extension + MCP server that streams real-time browser telemetry (console logs, network errors, exceptions, WebSocket events) to AI coding assistants like Claude Code, Cursor, Windsurf, and Zed. One command to install. Zero dependencies.

## 1. Install Everything

One command downloads the binary, stages the extension, and auto-configures all detected AI tools:

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.ps1 | iex
```

This automatically:
- Downloads the latest stable binary to `~/.gasoline/bin/`
- Verifies SHA-256 checksum
- Extracts the Chrome extension to `~/GasolineAgenticDevtoolExtension/`
- Auto-configures all detected MCP clients (Claude Code, Cursor, Windsurf, Zed, Gemini CLI, OpenCode, Antigravity, Claude Desktop, VS Code)

## 2. Load the Chrome Extension

This is the one step that requires human interaction — Chrome doesn't allow programmatic extension installation.

1. Open `chrome://extensions`
2. Enable **Developer mode** (top right toggle)
3. Click **Load unpacked**
4. Select the folder: **`~/GasolineAgenticDevtoolExtension`**

You'll see the Gasoline icon in your toolbar. It will show "Not Connected" until you complete step 3.

:::tip[Skip the UI clicks]
If you're willing to restart Chrome, you can pre-load the extension via CLI flag:
```bash
# macOS
open -a "Google Chrome" --args --load-extension="$HOME/GasolineAgenticDevtoolExtension"
```
This only applies to that Chrome session. For persistent installation, use the Load Unpacked flow above.
:::

## 3. Verify It Works

**Restart your AI tool** (quit and reopen Claude Code, Cursor, etc.) to activate the MCP server.

Open your web app in Chrome. Trigger a test error:

```javascript
console.error("Gasoline test — is the fire lit?")
```

Ask your AI: _"What browser errors do you see?"_

The extension icon should now show **Connected** (green indicator).

You can also verify with the built-in doctor command:
```bash
~/.gasoline/bin/gasoline-agentic-devtools --doctor
```

## What tools does Gasoline give my AI?

Your AI now has 5 tools covering the full debugging lifecycle:

| Tool | What it does |
|------|-------------|
| `observe` | Browser state — errors, logs, network, WebSocket, actions, Web Vitals, page info, recordings |
| `analyze` | Active analysis — DOM queries, accessibility audits, security audits, performance, link health, visual annotations |
| `generate` | Artifacts — Playwright tests, reproduction scripts, PR summaries, SARIF, HAR, CSP, SRI, test healing |
| `configure` | Session — noise filtering, persistent storage, recording, streaming, health |
| `interact` | Browser control — navigate, click, type, execute JS, upload, draw mode, state management |

Each tool has sub-modes. For example, `observe` with `what: "errors"` returns console errors, while `what: "websocket_status"` returns active WebSocket connections.

See [MCP Integration](/mcp-integration/) for full tool documentation.

## Alternative Install Methods

**npm** (if you prefer Node.js):
```bash
npm install -g gasoline-agentic-browser && gasoline-agentic-devtools --install
```

**From source** (for development):
```bash
git clone https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp.git
cd gasoline-agentic-browser-devtools-mcp
go run ./cmd/dev-console
```
Requires [Go 1.24+](https://go.dev/).

## Next Steps

- [MCP Integration](/mcp-integration/) — setup for your specific tool
- [All capabilities](/features/) — everything Gasoline captures
