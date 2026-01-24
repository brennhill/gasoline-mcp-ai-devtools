# gasoline-mcp

**Browser observability for AI coding agents.** Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant—letting it autonomously debug and fix issues in real time.

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
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

**Option B: Global** — add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

After adding, restart Claude Code. You should see "gasoline" listed when you run `/mcp`.

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
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
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
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
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
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
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
          "args": ["gasoline-mcp", "--mcp"]
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
        "args": ["gasoline-mcp", "--mcp"]
      }
    }
  }
}
```

</details>

### Step 2: Install the Browser Extension

The extension captures logs from your browser and sends them to the local Gasoline server.

1. Download or clone the [Gasoline repository](https://github.com/brennhill/gasoline)
2. Open `chrome://extensions` in Chrome
3. Enable **Developer mode** (top right toggle)
4. Click **Load unpacked**
5. Select the `extension/` folder from the repository

Once installed, you'll see the Gasoline icon in your browser toolbar. Click it to check connection status.

### That's It!

Your AI assistant now has access to three tools:

| Tool                 | What it does                                                    |
| -------------------- | --------------------------------------------------------------- |
| `get_browser_errors` | Fetches recent console errors, network failures, and exceptions |
| `get_browser_logs`   | Fetches all logs (errors + warnings + info)                     |
| `clear_browser_logs` | Clears the log file                                             |

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
  --mcp                  Run in MCP mode for AI tool integration
  --version              Show version
  --help                 Show help
```

## Troubleshooting

**"gasoline" not showing up in my AI tool?**

- Make sure you restarted the AI tool after adding the config
- Check the config file path is correct for your tool
- Try running `npx gasoline-mcp --version` to verify the package works

**Extension shows "Disconnected"?**

- The MCP server starts automatically when your AI tool launches — make sure it's running
- For manual mode, run `npx gasoline-mcp` in a terminal first
- Check if port 7890 is in use: try `--port 7891` and update extension Options to match

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

- [Full documentation](https://github.com/brennhill/gasoline)
- [Report an issue](https://github.com/brennhill/gasoline/issues)
