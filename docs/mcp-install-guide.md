# MCP Installation Guide

Gasoline MCP supports 9 AI coding tools. Use the auto-installer or configure manually.

## Auto-Install

```bash
# Install to all detected clients
gasoline-mcp --install

# Install to a specific tool
gasoline-mcp --install gemini
gasoline-mcp --install opencode
gasoline-mcp --install cursor

# Preview without writing
gasoline-mcp --install --dry-run

# Check what's configured
gasoline-mcp --doctor
```

Valid tool names: `claude`, `claude-desktop`, `cursor`, `windsurf`, `vscode`, `gemini`, `opencode`, `antigravity`, `zed`

## Per-Tool Reference

### Claude Code

| | |
|---|---|
| **Install method** | CLI (`claude mcp add-json`) |
| **Auto-install** | `gasoline-mcp --install claude` |

Claude Code is configured via its own CLI, not a config file. The installer runs `claude mcp add-json --scope user gasoline` automatically.

### Claude Desktop

| | |
|---|---|
| **Config path (macOS)** | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| **Config path (Windows)** | `%APPDATA%/Claude/claude_desktop_config.json` |
| **Auto-install** | `gasoline-mcp --install claude-desktop` |

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp",
      "args": []
    }
  }
}
```

### Cursor

| | |
|---|---|
| **Config path** | `~/.cursor/mcp.json` |
| **Auto-install** | `gasoline-mcp --install cursor` |

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp",
      "args": []
    }
  }
}
```

### Windsurf

| | |
|---|---|
| **Config path** | `~/.codeium/windsurf/mcp_config.json` |
| **Auto-install** | `gasoline-mcp --install windsurf` |

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp",
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
| **Auto-install** | `gasoline-mcp --install vscode` |

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp",
      "args": []
    }
  }
}
```

### Gemini CLI

| | |
|---|---|
| **Config path** | `~/.gemini/settings.json` |
| **Auto-install** | `gasoline-mcp --install gemini` |

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp",
      "args": []
    }
  }
}
```

### OpenCode

| | |
|---|---|
| **Config path** | `~/.config/opencode/opencode.json` |
| **Auto-install** | `gasoline-mcp --install opencode` |

OpenCode uses a different config format (`mcp` key with array-style commands):

```json
{
  "mcp": {
    "gasoline": {
      "type": "local",
      "command": ["gasoline-mcp"],
      "enabled": true
    }
  }
}
```

### Antigravity

| | |
|---|---|
| **Config path** | `~/.gemini/antigravity/mcp_config.json` |
| **Auto-install** | `gasoline-mcp --install antigravity` |

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp",
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
| **Auto-install** | `gasoline-mcp --install zed` |

Zed uses the `context_servers` key:

```json
{
  "context_servers": {
    "gasoline": {
      "source": "custom",
      "command": "gasoline-mcp",
      "args": []
    }
  }
}
```

## Verification

After installing, verify the setup:

```bash
# Check all client configurations
gasoline-mcp --doctor

# Test the server is reachable
curl http://localhost:7890/health
```

## Uninstall

```bash
# Remove from all clients
gasoline-mcp --uninstall

# Preview removal
gasoline-mcp --uninstall --dry-run
```
