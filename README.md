<div align="center">

<img src="chrome_store_files/readme-banner.png" alt="Gasoline - Browser Observability for AI Coding Agents" width="100%" />

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-5.0.0-green.svg)](https://github.com/brennhill/gasoline/releases)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev/)
[![Chrome](https://img.shields.io/badge/Chrome-Manifest%20V3-4285F4.svg?logo=googlechrome&logoColor=white)](https://developer.chrome.com/docs/extensions/mv3/)
[![macOS](https://img.shields.io/badge/macOS-supported-000000.svg?logo=apple&logoColor=white)](https://github.com/brennhill/gasoline)
[![Linux](https://img.shields.io/badge/Linux-supported-FCC624.svg?logo=linux&logoColor=black)](https://github.com/brennhill/gasoline)
[![Windows](https://img.shields.io/badge/Windows-supported-0078D6.svg?logo=windows&logoColor=white)](https://github.com/brennhill/gasoline)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/brennhill/gasoline/pulls)
[![X Follow](https://img.shields.io/badge/follow-%40gasolinedev-000000.svg?logo=x&logoColor=white)](https://x.com/gasolinedev)

**Browser observability for AI coding agents - autonomously debug and fix issues in real time.** Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant. Enterprise ready.

[Documentation](https://cookwithgasoline.com) •
[Quick Start](https://cookwithgasoline.com/getting-started/) •
[Features](https://cookwithgasoline.com/features/) •
[MCP Setup](https://cookwithgasoline.com/mcp-integration/)

</div>

---


## Quick Start

**Until the Chrome Web Store listing is approved, install manually:**

```bash
# 1. Clone the repo
git clone https://github.com/brennhill/gasoline.git
cd gasoline

# 2. Start the server
go run ./cmd/dev-console

# 3. Load the extension manually:
#    - Open chrome://extensions
#    - Enable Developer mode
#    - Click "Load unpacked" and select the `extension/` folder in this repo
```

**Once the Chrome extension is approved:**

```bash
npx gasoline-mcp
# Then install from the Chrome Web Store
```

**MCP config example:**

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

Works with **Claude Code**, **Cursor**, **Windsurf**, **Claude Desktop**, **Zed**, and any MCP-compatible tool.

**[Full setup guide →](https://cookwithgasoline.com/getting-started/)**

## Comparison

| | Gasoline | Chrome DevTools MCP | BrowserTools MCP | Cursor Browser |
|---|:---:|:---:|:---:|:---:|
| **Console logs** | ✅ | ✅ | ✅ | ✅ |
| **Network errors** | ✅ | ✅ | ✅ | ❌ |
| **Network bodies** | ✅ | ❌ | ❌ | ❌ |
| **WebSocket events** | ✅ | ❌ | ❌ | ❌ |
| **User action recording** | ✅ | ❌ | ❌ | ❌ |
| **DOM queries** | ✅ | ✅ | ✅ | ✅ |
| **Screenshots** | ✅ | ✅ | ✅ | ✅ |
| | | | | |
| **[Web Vitals](https://cookwithgasoline.com/web-vitals/)** | ✅ LCP, CLS, INP, FCP | ❌ | ❌ | ❌ |
| **[Regression detection](https://cookwithgasoline.com/regression-detection/)** | ✅ Automatic | ❌ | ❌ | ❌ |
| **[API schema inference](https://cookwithgasoline.com/api-schema/)** | ✅ OpenAPI from traffic | ❌ | ❌ | ❌ |
| **[Accessibility audits](https://cookwithgasoline.com/accessibility-audit/)** | ✅ WCAG + SARIF | ❌ | ❌ | ❌ |
| **[Session checkpoints](https://cookwithgasoline.com/session-checkpoints/)** | ✅ Named + auto | ❌ | ❌ | ❌ |
| **[Noise filtering](https://cookwithgasoline.com/noise-filtering/)** | ✅ Auto-detect | ❌ | ❌ | ❌ |
| | | | | |
| **[Test generation](https://cookwithgasoline.com/generate-test/)** | ✅ Playwright | ❌ | ❌ | ❌ |
| **[Reproduction scripts](https://cookwithgasoline.com/reproduction-scripts/)** | ✅ From actions | ❌ | ❌ | ❌ |
| **[PR summaries](https://cookwithgasoline.com/pr-summaries/)** | ✅ Perf impact | ❌ | ❌ | ❌ |
| **[HAR export](https://cookwithgasoline.com/har-export/)** | ✅ | ❌ | ❌ | ❌ |
| | | | | |
| **Zero dependencies** | ✅ Single Go binary | ❌ Node.js + Chrome flags | ❌ Node.js + Puppeteer | ❌ Electron |
| **Vendor neutral** | ✅ Any MCP tool | ⚠️ Any MCP tool | ⚠️ Any MCP tool | ❌ Cursor only |
| **No debug port** | ✅ | ❌ `--remote-debugging-port` | ❌ `--remote-debugging-port` | N/A |
| **Privacy** | ✅ Localhost only | ✅ Local | ⚠️ Optional cloud | ❌ Cursor servers |
| **Performance overhead** | < 0.1ms | ~5ms | ~5ms | Unknown |

## Why You Cook With Gasoline

**No debug port required.** Other tools need Chrome launched with `--remote-debugging-port`, which disables security sandboxing and breaks your normal browser workflow. Gasoline uses a standard extension — your browser stays secure and unmodified.

**Single binary, zero runtime.** No Node.js, no Python, no Puppeteer, no package.json. One Go binary that runs anywhere. No supply chain risk. No `node_modules`.

**Captures what others can't.** WebSocket messages, full request/response bodies, user action recording, Web Vitals, automatic regression detection, API schema inference, and Playwright test generation from real browser sessions — features no other MCP browser tool offers.

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
- **Live DOM queries** — Query the page with CSS selectors via MCP
- **Accessibility audits** — WCAG checks with SARIF export
- **API schema inference** — Auto-discover OpenAPI from captured traffic
- **Session checkpoints** — Save state, diff changes, detect regressions
- **Test generation** — Playwright tests and reproduction scripts from actions
- **Noise filtering** — Auto-detect and dismiss irrelevant errors
- **Developer API** — `window.__gasoline.annotate()` for custom context

**[Full feature list →](https://cookwithgasoline.com/features/)**

## Privacy

100% local. No cloud, no analytics, no telemetry. Logs never leave your machine.

**[Privacy details →](https://cookwithgasoline.com/privacy/)**

## Performance

See [latest benchmarks](benchmarks/latest-benchmark.md) for current performance data.

Last benchmarked: 2026-01-24 on darwin/arm64 (v5.0.0)

## Development

```bash
make test                              # Go server tests
node --test extension-tests/*.test.js  # Extension tests
make dev                               # Build for current platform
```

**[Release process & quality gates →](RELEASE.md)**

## License

**AGPL-3.0** — Free for personal and internal company use. [Commercial licensing available](https://cookwithgasoline.com/privacy/) for proprietary integration.

---

<div align="center">

**[cookwithgasoline.com](https://cookwithgasoline.com)**

*Pouring fuel on the AI development fire*

If you find Gasoline useful, please consider giving it a star!

[![Star on GitHub](https://img.shields.io/github/stars/brennhill/gasoline.svg?style=social)](https://github.com/brennhill/gasoline)

</div>
