---
title: Fire It Up
description: Install and configure Gasoline in under 2 minutes. Start streaming browser logs to your autonomous coding agent with a single command.
---

Gasoline is an open-source browser extension + MCP server that streams real-time browser telemetry (console logs, network errors, exceptions, WebSocket events) to AI coding assistants like Claude Code, Cursor, Windsurf, and Zed. One command to install. Zero dependencies.

## 1. Install the Extension

The extension captures logs from your browser and sends them to the local Gasoline server.

```bash
# 1. Clone the repo for the extension
git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
cd gasoline

# 2. Load the extension:
```

1. Open `chrome://extensions`
2. Enable **Developer mode** (top right toggle)
3. Click **Load unpacked**
4. Select the `extension/` folder from the cloned repository

You'll see the Gasoline icon in your toolbar. It will show "Not Connected" until you complete step 2.

## 2. Connect Your AI Tool

Add this config to your AI tool and it will start Gasoline automatically:

**Claude Code** — `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["-y", "gasoline-mcp"]
    }
  }
}
```

**Claude Desktop** — `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["-y", "gasoline-mcp"]
    }
  }
}
```

Or auto-install to any supported tool:

```bash
gasoline-mcp --install          # all detected clients
gasoline-mcp --install gemini   # just Gemini CLI
gasoline-mcp --install cursor   # just Cursor
```

See [MCP Integration](/mcp-integration/) for Cursor, Windsurf, Zed, Gemini CLI, OpenCode, Antigravity, and more.

**Restart your AI tool.** The server will start automatically when your AI connects.

## 3. Verify It Works

Open your web app in Chrome. Trigger a test error:

```javascript
console.error("Gasoline test — is the fire lit?")
```

Ask your AI: _"What browser errors do you see?"_

The extension icon should now show **Connected** (green indicator).

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

## Alternative: Run from Source

For development or if you prefer building from source:

```bash
# Clone and run
git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
cd gasoline
go run ./cmd/dev-console

# MCP config for source install
{
  "mcpServers": {
    "gasoline": {
      "command": "go",
      "args": ["run", "./cmd/dev-console"]
    }
  }
}
```

Requires [Go 1.21+](https://go.dev/).

## Next Steps

- [MCP Integration](/mcp-integration/) — setup for your specific tool
- [All capabilities](/features/) — everything Gasoline captures
