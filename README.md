<div align="center">

<img src="docs/assets/images/chrome_store/readme-banner.png" alt="Gasoline MCP - Browser Observability for AI Coding Agents" width="100%" />

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-5.6.0-green.svg)](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev/)
[![Chrome](https://img.shields.io/badge/Chrome-Manifest%20V3-4285F4.svg?logo=googlechrome&logoColor=white)](https://developer.chrome.com/docs/extensions/mv3/)
[![macOS](https://img.shields.io/badge/macOS-supported-000000.svg?logo=apple&logoColor=white)](https://github.com/brennhill/gasoline-mcp-ai-devtools)
[![Linux](https://img.shields.io/badge/Linux-supported-FCC624.svg?logo=linux&logoColor=black)](https://github.com/brennhill/gasoline-mcp-ai-devtools)
[![Windows](https://img.shields.io/badge/Windows-supported-0078D6.svg?logo=windows&logoColor=white)](https://github.com/brennhill/gasoline-mcp-ai-devtools)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/brennhill/gasoline-mcp-ai-devtools/pulls)
[![X Follow](https://img.shields.io/badge/follow-%40gasolinedev-000000.svg?logo=x&logoColor=white)](https://x.com/gasolinedev)

**Browser observability for AI coding agents - autonomously debug and fix issues in real time.** Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant. Enterprise ready.

[Documentation](https://cookwithgasoline.com) •
[Quick Start](https://cookwithgasoline.com/getting-started/) •
[Features](https://cookwithgasoline.com/features/) •
[MCP Setup](https://cookwithgasoline.com/mcp-integration/)

</div>

---


## Quick Start

**Step 1: Load the extension**

```bash
# Clone the repo
git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
cd gasoline

# Load the extension in Chrome:
#   - Open chrome://extensions
#   - Enable Developer mode (top right)
#   - Click "Load unpacked" and select the `extension/` folder
```

**Step 2: Configure MCP in your AI tool**

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
# Should return: {"status":"ok","version":"5.3.0",...}
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

## How AI Should Debug: Three Paradigms

Gasoline isn't a test tool. It's a **co-pilot for your entire stack**. Here's how AI debugging evolves:

| | **Traditional QA** | **Gasoline v6** | **Gasoline v7** |
|---|---|---|---|
| **Philosophy** | Test-First | Explore-First | Understand-First |
| **AI's Job** | Execute pre-written tests (human-like workflow) | Read spec, explore UI, find bugs (AI-native workflow) | Understand full system, trace root causes (true full-stack AI reasoning) |
| | | | |
| **What AI Sees** | | | |
| *Example: "Checkout failed"* | ❌ Test failed (binary) | ✅ Browser trace: UI actions + network + DOM + console | ✅ Full causality: Browser → API → Backend logs → Database → Which service changed 3 days ago |
| *Example: "Service A changed"* | Run all tests, hope nothing broke | Test Service A in isolation | ✅ Dependency graph: A impacts B, C, D; validate each contract; test critical paths |
| *Example: "Prod error"* | Check logs manually | Replay with local mods | ✅ Correlate prod request → backend logs → test coverage → git history |
| | | | |
| **AI's Autonomy** | | | |
| *What can it fix?* | Test code (not real bugs) | ✅ Bugs in single app | ✅ Multi-service bugs, contracts, broken workflows |
| *Loop prevention?* | 0 (human writes tests) | ✅ Bounded (doom loop detection) | ✅ Bounded + semantic understanding |
| *Confidence level* | Low (tests ≠ reality) | High for single-app | Very high (full-stack validation + contracts) |
| | | | |
| **Multi-Service Reality** | | | |
| *"Does Service B still work?"* | Run full test suite (30 min) | Only tests A, misses B | ✅ Impact analysis (30 sec), validate contracts, test critical paths |
| *"Race condition in prod?"* | Can't reproduce | Local timing variations | ✅ Replay exact scenario with prod state + correlation IDs |
| | | | |
| **Time to Know It's Safe** | 10–30 min | 30 sec – 2 min | 30 sec – 2 min (full-stack) |
| **Confidence Signal** | Tests pass? (false confidence) | Behavior matches spec + no loops | ✅ Causality validated + contracts honored + critical paths pass |

**v6 (Current):** AI-native testing for single apps. Read spec, explore UI, find and fix bugs autonomously.

**v7 (Roadmap):** Full-stack AI debugging. Add backend correlation, dependency graphs, API contracts, and edge case registry.

[See roadmap →](docs/roadmap.md)

## Why You Cook With Gasoline MCP

**No debug port required.** Other tools need Chrome launched with `--remote-debugging-port`, which disables security sandboxing and breaks your normal browser workflow. Gasoline MCP uses a standard extension — your browser stays secure and unmodified.

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

See [latest benchmarks](docs/benchmarks/latest-benchmark.md) for current performance data.

Last benchmarked: 2026-01-28 on darwin/arm64 (v5.2.5)

## Known Issues

See [docs/core/known-issues.md](docs/core/known-issues.md) for current known issues and the v5.3 roadmap.

## Development

```bash
make test                              # Go server tests
node --test tests/extension/*.test.js  # Extension tests
make dev                               # Build for current platform
```

**[Release process & quality gates →](docs/core/release.md)** · **[Changelog →](CHANGELOG.md)**

## License

**AGPL-3.0** — Free and open source for all use cases.

---

<div align="center">

<img src="docs/assets/images/sparky-wave.png" alt="Sparky the Salamander" width="120" />

**[cookwithgasoline.com](https://cookwithgasoline.com)**

*Pouring fuel on the AI development fire*

If you find Gasoline MCP useful, please consider giving it a star!

[![Star on GitHub](https://img.shields.io/github/stars/brennhill/gasoline.svg?style=social)](https://github.com/brennhill/gasoline-mcp-ai-devtools)

</div>
