# gasoline-mcp

**Browser observability for AI coding agents - autonomously debug and fix issues in real time.** Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant. Enterprise ready.

## Setup (2 Steps)

### Step 1: Add the MCP Server to Your AI Tool

MCP (Model Context Protocol) lets your AI assistant talk to external tools. You just need to add a config snippet — no global install required. `npx` downloads and runs the binary automatically.

Pick your tool and add the config:

<details>
<summary><strong>Claude Code (CLI)</strong></summary>

**Option A: Per-project** (recommended for teams) — create `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
    }
  }
}
```

**Option B: Global** — add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
    }
  }
}
```

After adding, restart Claude Code. You should see "gasoline" listed when you run `/mcp`.

**Architecture note:** Your AI tool spawns a SINGLE Gasoline process that handles both:
- HTTP server on port 7890 (for browser extension telemetry)
- stdio MCP protocol (for AI tool commands)

Both interfaces share the same browser state. Do NOT manually start Gasoline — let the MCP system manage it.

</details>

<details>
<summary><strong>Claude Desktop</strong></summary>

Edit your config file:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
    }
  }
}
```

Restart Claude Desktop after saving.

</details>

<details>
<summary><strong>Cursor</strong></summary>

Go to Settings → MCP Servers → Add Server, or add to `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
    }
  }
}
```

Restart Cursor after saving.

</details>

<details>
<summary><strong>Windsurf</strong></summary>

Add to `~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
    }
  }
}
```

Restart Windsurf after saving.

</details>

<details>
<summary><strong>VS Code with Continue</strong></summary>

Add to `~/.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "npx",
          "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
        }
      }
    ]
  }
}
```

</details>

<details>
<summary><strong>Zed</strong></summary>

Add to `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "gasoline": {
      "command": {
        "path": "npx",
        "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
      }
    }
  }
}
```

</details>

### Step 2: Install the Browser Extension

The extension captures logs from your browser and sends them to the local Gasoline server.

1. Download or clone the [Gasoline repository](https://github.com/brennhill/gasoline-mcp-ai-devtools)
2. Open `chrome://extensions` in Chrome
3. Enable **Developer mode** (top right toggle)
4. Click **Load unpacked**
5. Select the `extension/` folder from the repository

Once installed, you'll see the Gasoline icon in your browser toolbar. Click it to check connection status.

### That's It!

Your AI assistant now has access to 5 tools:

| Tool | What it does |
| ---- | ------------ |
| `observe` | Captured browser state (errors, logs, network, websocket, actions, vitals, page, tabs, timeline, error_bundles, screenshot) |
| `analyze` | Active analysis (dom, performance, accessibility, security_audit, third_party_audit, error_clusters, link_health, annotations) |
| `generate` | Artifacts (reproduction, csp, sarif, test_from_context, test_heal, test_classify, visual_test, annotation_report) |
| `configure` | Session management (store, load, noise_rule, clear, health, streaming, recording_start, recording_stop, log_diff) |
| `interact` | Browser automation (navigate, click, type, select, execute_js, highlight, scroll_to, key_press, upload, draw_mode_start) |

Try it: open your web app, trigger an error, then ask your AI assistant _"What browser errors do you see?"_

## How It Works

```
Browser → Extension → Local Server (localhost:7890) → Log File → AI reads via MCP
```

1. The extension captures console logs, network errors, and exceptions from your browser
2. Logs are sent to the Gasoline server running on your machine (localhost only)
3. Your AI assistant reads the logs through the MCP protocol
4. Everything stays local — no data ever leaves your machine

## Manual Server Mode (No MCP)

If your AI tool doesn't support MCP, you can run the server standalone:

```bash
npx gasoline-mcp
```

This starts an HTTP server on `http://localhost:7890` and writes logs to `~/gasoline-logs.jsonl`. Your AI can read this file directly.

## Options

```
gasoline-mcp [options]

  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: ~/gasoline-logs.jsonl)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --mcp                  No-op (MCP mode is the default)
  --version              Show version
  --help                 Show help
```

### External Skills Catalog (Optional)

`gasoline-mcp --install` can install managed skills from a separate GitHub repo (for example `brennhill/gasoline-skills`) instead of only the bundled package copy.

Examples:

```bash
# Simple owner/repo source
gasoline-mcp --install --skills-repo brennhill/gasoline-skills

# GitHub tree URL with subpath (auto-detects ref and path)
gasoline-mcp --install --skills-repo https://github.com/brennhill/gasoline-skills/tree/main/skills

# Explicit manifest/path controls
gasoline-mcp --install \
  --skills-repo brennhill/gasoline-skills \
  --skills-ref main \
  --skills-manifest skills/skills.json \
  --skills-path skills
```

Environment variable equivalents:

- `GASOLINE_SKILLS_REPO`
- `GASOLINE_SKILLS_REF`
- `GASOLINE_SKILLS_MANIFEST_PATH`
- `GASOLINE_SKILLS_PATH`
- `GASOLINE_SKILLS_DIR` (local directory override)
- `GASOLINE_SKILLS_NO_FALLBACK=1` (fail instead of falling back to bundled skills)

## Troubleshooting

**"gasoline" not showing up in my AI tool?**

- Make sure you restarted the AI tool after adding the config
- Check the config file path is correct for your tool
- Try running `npx gasoline-mcp --version` to verify the package works

**"bind: address already in use" error?**

If MCP fails to start with a port conflict, you likely have a manually-started Gasoline instance running:

```bash
# Find and kill the conflicting process
ps aux | grep gasoline | grep -v grep
kill <PID>

# Or on macOS/Linux:
pkill -f gasoline
```

Then reload your MCP connection. The MCP system will spawn a fresh instance. Remember: do NOT manually start Gasoline when using MCP mode.

**Extension shows "Disconnected"?**

- The MCP server starts automatically when your AI tool launches — check if it's running: `ps aux | grep gasoline`
- Verify the extension's Server URL matches your config (default: `http://localhost:7890`)
- Try restarting your AI tool to re-initialize the MCP connection

**No logs appearing?**

- Click the extension icon and check the capture level (try "All Logs")
- Make sure the page was loaded/refreshed after the extension was installed
- Check `~/gasoline-logs.jsonl` to see if entries are being written

## Supported Platforms

- macOS (Apple Silicon & Intel)
- Linux (x64 & ARM64)
- Windows (x64)

## Privacy

100% local. Logs never leave your machine. No cloud, no analytics, no telemetry. The server only binds to `127.0.0.1`.

## Links

- [Full documentation](https://github.com/brennhill/gasoline-mcp-ai-devtools)
- [Report an issue](https://github.com/brennhill/gasoline-mcp-ai-devtools/issues)
