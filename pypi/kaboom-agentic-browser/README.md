# Kaboom Agentic Browser MCP

Agentic Browser Devtools - rapid e2e web development.

Streams console logs, network errors, WebSocket events, and exceptions to Claude Code, Cursor, Windsurf, Claude Desktop, Zed, or any MCP-compatible assistant.

## Installation

```bash
pip install kaboom-agentic-browser
```

The correct platform-specific binary will be installed automatically.

## Usage

### With Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "gasoline-browser-devtools": {
      "type": "stdio",
      "command": "kaboom-agentic-browser"
    }
  }
}
```

### With Claude Code

Add to your `.mcp.json`:

```json
{
  "mcpServers": {
    "gasoline-browser-devtools": {
      "type": "stdio",
      "command": "kaboom-agentic-browser"
    }
  }
}
```

### Standalone

```bash
kaboom-agentic-browser
```

### Install MCP config + bundled skills

```bash
kaboom-agentic-browser --install
```

This installs MCP config for detected clients and managed bundled skills (`debug-triage`, `performance`, `regression-test`, `api-validation`, `ux-audit`, `site-audit`) into your agent skill directories.

`site-audit` includes full menu mapping plus page-by-page and feature-by-feature product analysis with usability findings and reproducibility notes.

## Chrome Extension

Install the Chrome extension to capture browser telemetry:

[Chrome Web Store Link](https://chrome.google.com/webstore) (coming soon)

Or load from source:
1. Download the extension from [GitHub Releases](https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp/releases)
2. Go to `chrome://extensions`
3. Enable "Developer mode"
4. Click "Load unpacked" and select the `extension/` folder

## Features

- **Console logs** - All levels (log, warn, error, info, debug)
- **Network requests** - Full request/response capture with bodies
- **WebSocket events** - Real-time bidirectional message capture
- **User actions** - Clicks, navigation, form submissions
- **Errors** - Unhandled exceptions with stack traces
- **Web Vitals** - LCP, FID, CLS, INP, FCP, TTFB
- **Accessibility audits** - WCAG compliance scanning
- **Security audits** - CSP generation, third-party analysis
- **AI Web Pilot** - Execute JavaScript, highlight elements, manage state

## Documentation

- [Getting Started](https://gokaboom.dev/getting-started/)
- [GitHub Repository](https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp)
- [Issue Tracker](https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp/issues)

## Privacy

100% local. No cloud, no analytics, no telemetry. Logs never leave your machine.

## License

AGPL-3.0 — Free for personal and internal company use. [Commercial licensing available](https://gokaboom.dev/privacy/) for proprietary integration.
