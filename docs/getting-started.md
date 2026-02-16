---
title: "Fire It Up"
description: "Install and configure Gasoline in under 2 minutes. Start streaming browser logs to your autonomous coding agent with a single command."
keywords: "install gasoline, gasoline mcp setup, npx gasoline-mcp, browser extension install, MCP server setup"
permalink: /getting-started/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "One command. Two minutes. Your AI sees your browser."
toc: true
toc_sticky: true
---

## <i class="fas fa-fire"></i> 1. Ignite the Server

<img src="/assets/images/sparky/features/sparky-fight-fire-web.webp" alt="Sparky firing up the server" style="float: right; width: 140px; margin: 0 0 20px 20px; border-radius: 6px;" />

**Option A: NPM (recommended)**

```bash
npx gasoline-mcp
```

**Option B: PyPI (Python)**

```bash
pip install gasoline-mcp
gasoline-mcp
```

**Option C: From source**

```bash
git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
cd gasoline
go run ./cmd/dev-console
```

You'll see: `[gasoline] v0.7.2 — HTTP on port 7890`

npm and PyPI are distribution channels — they deliver the native Go binary for your platform. The MCP server runs natively with no Node.js or Python runtime dependency.

Leave this burning. No global install needed.

**Extension install** (until Chrome Web Store approval):

1. Open `chrome://extensions`
2. Enable **Developer mode**
3. Click **Load unpacked** and select the `extension/` folder in this repo

## <i class="fas fa-puzzle-piece"></i> 2. Install the Extension

<img src="/assets/images/sparky/features/sparky-wave-web.webp" alt="Sparky waving from the toolbar" style="float: left; width: 140px; margin: 0 20px 20px 0; border-radius: 6px;" />

Grab it from the [Chrome Web Store](https://chromewebstore.google.com) (search "Gasoline"). Click the icon in your toolbar — it should show **Connected**.

<details>
<summary><i class="fas fa-wrench"></i> Load Unpacked (Development)</summary>

1. Clone the [repository](https://github.com/brennhill/gasoline-mcp-ai-devtools)
2. Open `chrome://extensions` → enable **Developer mode**
3. Click **Load unpacked** → select the `extension/` folder

</details>

<img src="/assets/images/sparky/features/sparky-presents-browser-web.webp" alt="Sparky presenting the connected browser" style="float: right; width: 140px; margin: 0 0 20px 20px; border-radius: 6px;" />

## <i class="fas fa-plug"></i> 3. Connect Your AI Tool

Drop this config and your AI tool fires up Gasoline automatically:

**Claude Code** — `.mcp.json` in your project root:

*Option A: NPM (recommended)*
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

*Option B: PyPI (Python)*
```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp"
    }
  }
}
```

*Option C: From source (development)*
```json
{
  "mcpServers": {
    "gasoline": {
      "command": "go",
      "args": ["run", "./cmd/dev-console"]
    }
  }
}
```

<i class="fas fa-fire-alt"></i> See [MCP Integration](/mcp-integration/) for Cursor, Windsurf, Claude Desktop, Zed, and more.

**Restart your AI tool.** From now on, the server ignites automatically.

## <i class="fas fa-check-circle"></i> Verify the Flame

Open your web app. Trigger an error:

```javascript
console.error("Gasoline test — is the fire lit?")
```

Ask your AI: _"What browser errors do you see?"_

## <i class="fas fa-tools"></i> Available Tools

Your AI now has 5 tools covering the full debugging lifecycle:

| Tool | What it does |
|------|-------------|
| `observe` | Captured state — errors, logs, network, WebSocket, actions, Web Vitals, page, tabs, timeline, error bundles, screenshot |
| `analyze` | Active analysis — DOM inspection, performance, accessibility, security audits, error clusters, link health, annotations |
| `generate` | Artifacts — reproduction scripts, CSP, SARIF, test generation (from context, heal, classify), visual tests, annotation reports |
| `configure` | Session — persistent memory, noise filtering, health, streaming, recording, log diff |
| `interact` | Browser automation — navigate, click, type, select, execute JS, highlight, upload, draw mode, save/load state |

Each tool has sub-modes. For example, `observe` with `what: "errors"` returns console errors, while `what: "websocket_status"` returns active WebSocket connections.

See [MCP Integration](/mcp-integration/) for full tool documentation.

## <i class="fas fa-file-alt"></i> No MCP? No Problem.

Run standalone — Gasoline writes to `~/gasoline-logs.jsonl`. Point your AI at the file.

```bash
npx gasoline-mcp
```

## Next Steps

- <i class="fas fa-sliders-h"></i> [Configure the server](/configuration/) — port, log rotation, file path
- <i class="fas fa-plug"></i> [MCP Integration](/mcp-integration/) — setup for your specific tool
- <i class="fas fa-fire-alt"></i> [All capabilities](/features/) — everything Gasoline captures
