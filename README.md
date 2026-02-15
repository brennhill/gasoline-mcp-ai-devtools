<div align="center">

<img src="docs/assets/images/chrome_store/readme-banner.png" alt="Gasoline MCP - Browser Observability for AI Coding Agents" width="100%" />

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.7.2-green.svg)](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev/)
[![Chrome](https://img.shields.io/badge/Chrome-Manifest%20V3-4285F4.svg?logo=googlechrome&logoColor=white)](https://developer.chrome.com/docs/extensions/mv3/)
[![macOS](https://img.shields.io/badge/macOS-supported-000000.svg?logo=apple&logoColor=white)](https://github.com/brennhill/gasoline-mcp-ai-devtools)
[![Linux](https://img.shields.io/badge/Linux-supported-FCC624.svg?logo=linux&logoColor=black)](https://github.com/brennhill/gasoline-mcp-ai-devtools)
[![Windows](https://img.shields.io/badge/Windows-supported-0078D6.svg?logo=windows&logoColor=white)](https://github.com/brennhill/gasoline-mcp-ai-devtools)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/62158fcb044348c3bc51942787a9a535)](https://app.codacy.com/gh/brennhill/gasoline-mcp-ai-devtools/dashboard?utm_source=gh&utm_medium=referral&utm_content=&utm_campaign=Badge_grade)
[![Snyk Status](https://snyk.io/test/github/brennhill/gasoline-mcp-ai-devtools/badge.svg)](https://snyk.io/test/github/brennhill/gasoline-mcp-ai-devtools)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/brennhill/gasoline-mcp-ai-devtools/pulls)
[![X Follow](https://img.shields.io/badge/follow-%40gasolinedev-000000.svg?logo=x&logoColor=white)](https://x.com/gasolinedev)
[![Made with love for AI developers](https://img.shields.io/badge/made%20with%20❤%20for-AI%20developers-FF6B6B.svg)](https://cookwithgasoline.com)

**Browser observability for AI coding agents - autonomously debug and fix issues in real time.** Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant. Enterprise ready.

[Documentation](https://cookwithgasoline.com) •
[Quick Start](https://cookwithgasoline.com/getting-started/) •
[Features](https://cookwithgasoline.com/features/) •
[MCP Setup](https://cookwithgasoline.com/mcp-integration/)

</div>

---

<div align="center">

## 📦 Latest Release

Current version: **v0.7.2** — Link health analyzer, browser automation, recording, and performance analysis for AI agents.

```bash
npx gasoline-mcp@0.7.2
```

</div>

---

## Quick Start

**You need TWO things:**

- **Browser extension** (captures browser telemetry)
- **MCP server** (forwards data to your AI tool)

---

### Step 1: Install the Browser Extension

1. Download the latest release from [GitHub Releases](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases) and unzip it, or clone the repo:
   ```bash
   git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
   ```
2. Open `chrome://extensions`
3. Enable **Developer mode** (top right)
4. Click **Load unpacked**
5. Select the `extension/` folder

### Step 2: Start the MCP Server

Choose one option below based on your setup:

#### Option A: NPM (recommended)

```bash
npx gasoline-mcp@0.7.2
```

#### Option B: PyPI

```bash
pip install gasoline-mcp
gasoline-mcp
```

#### Option C: Local development

```bash
cd gasoline
go run ./cmd/dev-console
```

### Step 3: Configure MCP in your AI tool

Choose one option below based on your setup:

*Option A: NPM (recommended)*
```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp"]
    }
  }
}
```

*Option B: PyPI*
```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "gasoline-mcp"
    }
  }
}
```

*Option C: Local development (from repo root)*
```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "go",
      "args": ["run", "./cmd/dev-console"]
    }
  }
}
```

**Verify setup:**
```bash
curl http://localhost:7890/health
# Should return: {"status":"ok","version":"0.7.2",...}
```

**How it works:**
- Gasoline MCP runs as a stdio-based MCP server (bridge mode)
- The bridge automatically spawns a persistent daemon on port 7890 if needed
- Extension connects to the daemon to send browser telemetry
- MCP client communicates via stdio
- Both share the same browser telemetry state

Works with **Claude Code**, **Cursor**, **Windsurf**, **Claude Desktop**, **Zed**, and any MCP-compatible tool.

**[Full setup guide →](https://cookwithgasoline.com/getting-started/)**

**CLI options:**

| Flag | Description |
|------|-------------|
| `--port <n>` | Port to listen on (default: 7890) |
| `--server` | HTTP-only mode (no MCP) |
| `--persist` | Keep running after MCP disconnect |
| `--api-key <key>` | Require API key for HTTP requests |
| `--connect` | Connect to existing server (multi-client) |
| `--check` | Verify setup before running |
| `--help` | Show all options |

## Why You Cook With Gasoline MCP

**No debug port required.** Other tools need Chrome launched with `--remote-debugging-port`, which disables security sandboxing and breaks your normal browser workflow. Gasoline MCP uses a standard extension — your browser stays secure and unmodified.

**Single binary, zero runtime.** No Node.js, no Python, no Puppeteer, no package.json. One Go binary that runs anywhere. No supply chain risk. No `node_modules`.

**Captures what others can't.** WebSocket messages, full request/response bodies, user action recording, Web Vitals, automatic regression detection, visual annotations, and Playwright test generation from real browser sessions — features no other MCP browser tool offers.

**Works with every MCP tool.** Claude Code, Cursor, Windsurf, Zed, Claude Desktop, VS Code + Continue. Switch AI tools without changing your debugging setup.

**Enterprise-safe by design.** Binds to `127.0.0.1` only. Auth headers are stripped automatically. No telemetry, no accounts, no cloud. Audit the source — it's AGPL-3.0.

## What It Does

- **Console logs** — `console.log()`, `.warn()`, `.error()` with full arguments
- **Network errors** — Failed API calls (4xx, 5xx) with response bodies
- **Exceptions** — Uncaught errors with full stack traces
- **WebSocket events** — Connection lifecycle and message payloads
- **Network bodies** — Request/response payloads for API debugging
- **User actions** — Click, type, navigate, scroll recording with smart selectors
- **Web Vitals** — LCP, CLS, INP, FCP with regression detection
- **DOM inspection** — Query the page with CSS selectors via MCP
- **Accessibility audits** — WCAG checks with SARIF export
- **Security audits** — Credentials, PII, headers, cookies, third-party analysis
- **Browser automation** — Click, type, select, upload, navigate with semantic selectors
- **Visual annotations** — Draw mode for user feedback with computed style extraction
- **Test generation** — Playwright tests from context, self-healing selectors, failure classification
- **Reproduction scripts** — Playwright scripts from recorded user actions
- **Noise filtering** — Auto-detect and dismiss irrelevant errors
- **Developer API** — `window.__gasoline.annotate()` for custom context

**[Full feature list →](https://cookwithgasoline.com/features/)**

## Privacy

100% local. No cloud, no analytics, no telemetry. Logs never leave your machine.

**[Privacy details →](https://cookwithgasoline.com/privacy/)**

## Performance

See [latest benchmarks](docs/benchmarks/latest-benchmark.md) for current performance data.

Last benchmarked: 2026-02-09 on darwin/arm64 (v0.7.2)

## Known Issues

See [docs/core/known-issues.md](docs/core/known-issues.md) for current known issues.

## Development

```bash
make test                              # Go server tests
node --test tests/extension/*.test.js  # Extension tests
make dev                               # Build for current platform
```

**[Release process & quality gates →](docs/core/release.md)** · **[Changelog →](CHANGELOG.md)**

## License

**AGPL-3.0** — Free and open source for all use cases.

Artwork, logos, and the Sparky mascot are **Copyright (c) Brenn Hill** and are not covered by the AGPL. See [LICENSE-ARTWORK](LICENSE-ARTWORK) for details.

---

<div align="center">

<img src="docs/assets/images/sparky-wave.png" alt="Sparky the Salamander" width="120" />

**[cookwithgasoline.com](https://cookwithgasoline.com)**

*Pouring fuel on the AI development fire*

If you find Gasoline MCP useful, please consider giving it a star!

[![Star on GitHub](https://img.shields.io/github/stars/brennhill/gasoline-mcp-ai-devtools.svg?style=social)](https://github.com/brennhill/gasoline-mcp-ai-devtools)

</div>
