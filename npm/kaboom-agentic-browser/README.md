# kaboom-agentic-browser

**Kaboom Agentic Browser - rapid e2e web development.** Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant. Enterprise ready.

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
    "kaboom-browser-devtools": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser", "--port", "7890", "--persist"]
    }
  }
}
```

**Option B: Global** — add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "kaboom-browser-devtools": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser", "--port", "7890", "--persist"]
    }
  }
}
```

After adding, restart Claude Code. You should see "kaboom-browser-devtools" listed when you run `/mcp`.

**Architecture note:** Your AI tool spawns a SINGLE Kaboom process that handles both:
- HTTP server on port 7890 (for browser extension telemetry)
- stdio MCP protocol (for AI tool commands)

Both interfaces share the same browser state. Do NOT manually start Kaboom — let the MCP system manage it.

</details>

<details>
<summary><strong>Claude Desktop</strong></summary>

Edit your config file:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "kaboom-browser-devtools": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser", "--port", "7890", "--persist"]
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
    "kaboom-browser-devtools": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser", "--port", "7890", "--persist"]
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
    "kaboom-browser-devtools": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser", "--port", "7890", "--persist"]
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
          "args": ["-y", "kaboom-agentic-browser", "--port", "7890", "--persist"]
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
    "kaboom-browser-devtools": {
      "command": {
        "path": "npx",
        "args": ["-y", "kaboom-agentic-browser", "--port", "7890", "--persist"]
      }
    }
  }
}
```

</details>

### Step 2: Install the Browser Extension

The extension captures logs from your browser and sends them to the local Kaboom server.

1. Download or clone the [Kaboom repository](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP)
2. Open `chrome://extensions` in Chrome
3. Enable **Developer mode** (top right toggle)
4. Click **Load unpacked**
5. Select the `extension/` folder from the repository

Once installed, you'll see the Kaboom icon in your browser toolbar. Click it to check connection status.

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
2. Logs are sent to the Kaboom server running on your machine (localhost only)
3. Your AI assistant reads the logs through the MCP protocol
4. Everything stays local — no data ever leaves your machine

## Manual Server Mode (No MCP)

If your AI tool doesn't support MCP, you can run the server standalone:

```bash
npx kaboom-agentic-browser
```

This starts an HTTP server on `http://localhost:7890` and writes logs to `~/.kaboom/logs/kaboom.jsonl`. Your AI can read this file directly.

## Options

```
kaboom-agentic-browser [options]

  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: ~/.kaboom/logs/kaboom.jsonl)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --mcp                  No-op (MCP mode is the default)
  --version              Show version
  --help                 Show help
```

### External Skills Catalog (Optional)

Kaboom ships with bundled skills (including `debug-triage`, `performance`, `regression-test`, `api-validation`, `ux-audit`, and `site-audit`).

`site-audit` provides full menu mapping plus page-by-page and feature-by-feature product analysis with usability findings and reproducibility notes.

- `kaboom-agentic-browser --install` installs bundled skills into detected agent skill directories.
- npm package install also runs the bundled skill installer via postinstall.
- Set `KABOOM_SKIP_SKILL_INSTALL=1` to disable automatic skill installation.

`kaboom-agentic-browser --install` can install managed skills from a separate GitHub repo (for example `brennhill/kaboom-skills`) instead of only the bundled package copy.

Examples:

```bash
# Simple owner/repo source
kaboom-agentic-browser --install --skills-repo brennhill/kaboom-skills

# GitHub tree URL with subpath (auto-detects ref and path)
kaboom-agentic-browser --install --skills-repo https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP-skills/tree/stable/skills

# Explicit manifest/path controls
kaboom-agentic-browser --install \
  --skills-repo brennhill/kaboom-skills \
  --skills-ref main \
  --skills-manifest skills/skills.json \
  --skills-path skills
```

Environment variable equivalents:

- `KABOOM_SKILLS_REPO`
- `KABOOM_SKILLS_REF`
- `KABOOM_SKILLS_MANIFEST_PATH`
- `KABOOM_SKILLS_PATH`
- `KABOOM_SKILLS_DIR` (local directory override)
- `KABOOM_SKILLS_NO_FALLBACK=1` (fail instead of falling back to bundled skills)

## Troubleshooting

**"kaboom-browser-devtools" not showing up in my AI tool?**

- Make sure you restarted the AI tool after adding the config
- Check the config file path is correct for your tool
- Try running `npx kaboom-agentic-browser --version` to verify the package works

**"bind: address already in use" error?**

If MCP fails to start with a port conflict, you likely have a manually-started Kaboom instance running:

```bash
# Find and kill the conflicting process
ps aux | grep kaboom-agentic-browser | grep -v grep
kill <PID>

# Or on macOS/Linux:
pkill -f kaboom-agentic-browser
```

Then reload your MCP connection. The MCP system will spawn a fresh instance. Remember: do NOT manually start Kaboom when using MCP mode.

**Extension shows "Disconnected"?**

- The MCP server starts automatically when your AI tool launches — check if it's running: `ps aux | grep kaboom-agentic-browser`
- Verify the extension's Server URL matches your config (default: `http://localhost:7890`)
- Try restarting your AI tool to re-initialize the MCP connection

**No logs appearing?**

- Click the extension icon and check the capture level (try "All Logs")
- Make sure the page was loaded/refreshed after the extension was installed
- Check `~/.kaboom/logs/kaboom.jsonl` to see if entries are being written

## Supported Platforms

- macOS (Apple Silicon & Intel)
- Linux (x64 & ARM64)
- Windows (x64)

## Privacy

100% local. Logs never leave your machine. No cloud, no analytics, no telemetry. The server only binds to `127.0.0.1`.

## Links

- [Full documentation](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP)
- [Report an issue](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues)
