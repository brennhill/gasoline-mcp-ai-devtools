# MCP Installation Guide

Gasoline MCP supports 9 AI coding tools. Use the one-liner installer or configure manually.

## Automatic Installation

The quickest way to install Gasoline and configure all your AI tools is via the one-liner script:

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

This script:
1.  **Downloads** the latest stable binary.
2.  **Installs** the browser extension files to `~/.gasoline/extension`.
3.  **Auto-configures** all detected MCP clients listed below to run the binary directly (no `npx`).
4.  **Displays** a polished, step-by-step install UI with progress and a final checklist card.

Important:
- The installer **cannot** click browser UI for you.
- You must manually open `chrome://extensions` (or `brave://extensions`), enable **Developer mode**, then click **Load unpacked** and select `~/.gasoline/extension`.
- After loading, pin the extension (recommended) and click **Track This Tab** in the popup.

## Per-Tool Reference

If you prefer to configure your tools manually, point them to the `gasoline` binary (usually located at `~/.gasoline/bin/gasoline`).

### Claude Code

| | |
|---|---|
| **Install method** | CLI (`claude mcp add-json`) |

Claude Code is configured via its own CLI. Run:
```bash
claude mcp add-json --scope user gasoline-browser-devtools <<< '{"command": "/Users/YOUR_USER/.gasoline/bin/gasoline", "args": []}'
```

### Claude Desktop

| | |
|---|---|
| **Config path (macOS)** | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| **Config path (Windows)** | `%APPDATA%/Claude/claude_desktop_config.json` |

```json
{
  "mcpServers": {
    "gasoline-browser-devtools": {
      "command": "/Users/YOUR_USER/.gasoline/bin/gasoline",
      "args": []
    }
  }
}
```

### Cursor

| | |
|---|---|
| **Config path** | `~/.cursor/mcp.json` |

```json
{
  "mcpServers": {
    "gasoline-browser-devtools": {
      "command": "/Users/YOUR_USER/.gasoline/bin/gasoline",
      "args": []
    }
  }
}
```

### Windsurf

| | |
|---|---|
| **Config path** | `~/.codeium/windsurf/mcp_config.json` |

```json
{
  "mcpServers": {
    "gasoline-browser-devtools": {
      "command": "/Users/YOUR_USER/.gasoline/bin/gasoline",
      "args": []
    }
  }
}
```

### VS Code

| | |
|---|---|
| **Config path (macOS)** | `~/Library/Application Support/Code/User/mcp.json` |
| **Config path (Windows)** | `%APPDATA%/Code/User/mcp.json` |
| **Config path (Linux)** | `~/.config/Code/User/mcp.json` |

```json
{
  "mcpServers": {
    "gasoline-browser-devtools": {
      "command": "/Users/YOUR_USER/.gasoline/bin/gasoline",
      "args": []
    }
  }
}
```

### Gemini CLI

| | |
|---|---|
| **Config path** | `~/.gemini/settings.json` |

```json
{
  "mcpServers": {
    "gasoline-browser-devtools": {
      "command": "/Users/YOUR_USER/.gasoline/bin/gasoline",
      "args": []
    }
  }
}
```

### OpenCode

| | |
|---|---|
| **Config path** | `~/.config/opencode/opencode.json` |

OpenCode uses a different config format (`mcp` key with array-style commands):

```json
{
  "mcp": {
    "gasoline-browser-devtools": {
      "type": "local",
      "command": ["/Users/YOUR_USER/.gasoline/bin/gasoline"],
      "enabled": true
    }
  }
}
```

### Antigravity

| | |
|---|---|
| **Config path** | `~/.gemini/antigravity/mcp_config.json` |

```json
{
  "mcpServers": {
    "gasoline-browser-devtools": {
      "command": "/Users/YOUR_USER/.gasoline/bin/gasoline",
      "args": []
    }
  }
}
```

Note: Antigravity does not support `${workspaceFolder}` — use absolute paths only.

### Zed

| | |
|---|---|
| **Config path** | `~/.config/zed/settings.json` |

Zed uses the `context_servers` key:

```json
{
  "context_servers": {
    "gasoline-browser-devtools": {
      "source": "custom",
      "command": "/Users/YOUR_USER/.gasoline/bin/gasoline",
      "args": []
    }
  }
}
```

## Verification

After installing, verify the setup:

```bash
# Test the server is reachable
curl http://localhost:7890/health
```
