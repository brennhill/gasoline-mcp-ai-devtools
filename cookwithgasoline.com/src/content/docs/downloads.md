---
title: Downloads
description: Download Gasoline extension and tools for your platform
---

<!-- markdownlint-disable MD033 -->
<img src="/images/sparky-working-laptop-web.webp" alt="Sparky coding at laptop" style="max-width:50%;height:auto;" />
<!-- markdownlint-enable MD033 -->

## Gasoline 0.7.2

Download Gasoline for your platform. Currently **0.7.2** is available.

### 🔥 Chrome Extension

**Gasoline Extension v0.7.2** - Browser observability for AI agents

#### Installation (Load Unpacked)

1. **Download** the latest release from [GitHub Releases](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases) and unzip, or clone the repo:
   ```bash
   git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
   ```
2. **Open Chrome** and navigate to `chrome://extensions/`
3. **Enable** "Developer mode" (toggle in top right)
4. **Click** "Load unpacked"
5. **Select** the `extension/` folder

The extension will be pinned to your toolbar and ready to use.

#### What's New in 0.7.x

✨ **5th Tool: analyze**
- Active analysis tool with 15 modes — DOM queries, accessibility, security audits, link health, visual annotations, API validation, and more
- Clean separation: observe reads passive buffers, analyze triggers active work

🔗 **Link Health & Validation**
- Browser-based link checker with CORS detection
- Server-side URL validation with SSRF-safe transport
- Concurrent worker pools for fast scanning

🎨 **Draw Mode & Visual Annotations**
- User draws rectangles and types feedback directly on the page
- Multi-page annotation sessions that accumulate across navigation
- Full computed style extraction for annotated elements
- Visual test generation from annotation sessions

🧪 **Test Healing & Classification**
- Self-healing Playwright selectors — automatically repair broken tests
- Context-aware test generation from errors, interactions, or regressions
- Test failure classification for batch triage

📹 **Recording & Playback**
- Full tab video recording with audio capture
- Recording playback and metadata tracking
- Log diff: compare error states between recordings

🔀 **Multi-Client Support**
- Multiple AI tools can connect to the same daemon
- Client registry with automatic ID derivation from CWD

### 💻 MCP Server (Command Line Tools)

**Gasoline MCP Server v0.7.2** - Enable AI agents to see and control your browser

Install the MCP server on your system:

#### npm (Recommended)

```bash
npm install -g gasoline-mcp@0.7.2
gasoline-mcp --help
```

#### pip

```bash
pip install gasoline-mcp
gasoline-mcp --help
```

#### From Source

```bash
git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
cd gasoline-mcp-ai-devtools
make build
./dist/gasoline-darwin-arm64  # or your platform binary
```

**Available Binaries:**

- `gasoline-darwin-arm64` (macOS Apple Silicon)
- `gasoline-darwin-x64` (macOS Intel)
- `gasoline-linux-arm64` (Linux ARM64)
- `gasoline-linux-x64` (Linux x86-64)
- `gasoline-win32-x64.exe` (Windows x86-64)

## System Requirements

- **Browser:** Chrome/Chromium 120+ (for MV3 support)
- **Runtime:** Native Go binary (no Node.js required for standalone binary installs)
- **Node.js:** 18+ (optional, only if you install via npm)
- **Platform:** macOS, Linux, Windows
- **MCP Client:** Claude Code, Cursor, Windsurf, Claude Desktop, Zed, Gemini CLI, OpenCode, Antigravity, or any other MCP-compliant system/agent

## Verification

To verify the extension installed correctly:

1. Open any webpage
2. Click the **Gasoline icon** (<img src="/images/logo.png" alt="Gasoline" style="display: inline; width: 20px; height: 20px; vertical-align: middle; margin: 0; padding: 0;" />) in your toolbar
3. You should see the popup with recording and tracking options
4. Check the extension's popup shows "Connected" status

## Troubleshooting

**Extension not appearing?**
- Try refreshing the page (Cmd+R / Ctrl+R)
- Ensure Chrome is version 120 or higher
- Check that Developer mode is enabled

**Recording not working?**
- Click the Gasoline icon to grant recording permission
- Ensure the tab you want to record is active
- Check your Chrome permissions for microphone access (if recording audio)

**Issues with MCP integration?**
- See [Claude Code Integration Guide](/mcp-integration/claude-code)
- See [Cursor Integration Guide](/mcp-integration/cursor)

## Support

- 📖 [Documentation](/getting-started)
- 🐛 [Report Issues](https://github.com/brennhill/gasoline-mcp-ai-devtools/issues)
- 💬 [GitHub Discussions](https://github.com/brennhill/gasoline-mcp-ai-devtools/discussions)
- 🔒 [Security Policy](/security)

## Release Notes

### v0.7.2 (February 2026)

**Major Features:**
- 5th MCP tool: `analyze` — 15 active analysis modes
- Link health checker with CORS detection and SSRF-safe server-side validation
- Draw mode visual annotations with multi-page sessions
- Test healing: self-repairing Playwright selectors
- Context-aware test generation and failure classification
- Recording system with video, audio, playback, and log diff
- Multi-client support with client registry
- Page summary analysis mode

**Security:**
- CWE-942 postMessage validation
- Security audit and third-party audit moved to dedicated analyze tool
- Input validation improvements

**Quality:**
- Zero-dependency Go binary — no Node.js or Python runtime
- Comprehensive smoke tests
- Full TypeScript strict mode compliance

See [GitHub Releases](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases) for complete version history.
