<div align="center">

<img src="chrome_store_files/readme-banner.png" alt="Gasoline - Fuel for the AI Fire" width="100%" />

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-3.5.0-green.svg)](https://github.com/brennhill/gasoline/releases)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev/)
[![Chrome](https://img.shields.io/badge/Chrome-Manifest%20V3-4285F4.svg?logo=googlechrome&logoColor=white)](https://developer.chrome.com/docs/extensions/mv3/)
[![macOS](https://img.shields.io/badge/macOS-supported-000000.svg?logo=apple&logoColor=white)](https://github.com/brennhill/gasoline)
[![Linux](https://img.shields.io/badge/Linux-supported-FCC624.svg?logo=linux&logoColor=black)](https://github.com/brennhill/gasoline)
[![Windows](https://img.shields.io/badge/Windows-supported-0078D6.svg?logo=windows&logoColor=white)](https://github.com/brennhill/gasoline)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/brennhill/gasoline/pulls)

**Stop copy-pasting browser errors.** Gasoline captures console logs, network errors, exceptions, WebSocket events, and more from your browser and makes them available to your AI coding assistant via MCP.

[Documentation](https://cookwithgasoline.com) •
[Quick Start](https://cookwithgasoline.com/getting-started/) •
[Features](https://cookwithgasoline.com/features/) •
[MCP Setup](https://cookwithgasoline.com/mcp-integration/)

</div>

---

## Quick Start

```bash
# 1. Start the server
npx gasoline-mcp

# 2. Install the Chrome extension from the Chrome Web Store

# 3. Add to your AI tool (.mcp.json in project root)
```

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

Works with **Claude Code**, **Cursor**, **Windsurf**, **Claude Desktop**, **Zed**, and any MCP-compatible tool.

**[Full setup guide →](https://cookwithgasoline.com/getting-started/)**

## What It Does

- **Console logs** — `console.log()`, `.warn()`, `.error()` with full arguments
- **Network errors** — Failed API calls (4xx, 5xx) with response bodies
- **Exceptions** — Uncaught errors with full stack traces
- **WebSocket events** — Connection lifecycle and message payloads
- **Network bodies** — Request/response payloads for API debugging
- **Live DOM queries** — Query the page with CSS selectors via MCP
- **Accessibility audits** — Run a11y checks from your AI assistant
- **Developer API** — `window.__gasoline.annotate()` for custom context

**[Full feature list →](https://cookwithgasoline.com/features/)**

## Privacy

100% local. No cloud, no analytics, no telemetry. Logs never leave your machine.

**[Privacy details →](https://cookwithgasoline.com/privacy/)**

## Development

```bash
make test                              # Go server tests
node --test extension-tests/*.test.js  # Extension tests
make dev                               # Build for current platform
```

## License

**AGPL-3.0** — Free for personal and internal company use. [Commercial licensing available](https://cookwithgasoline.com/privacy/) for proprietary integration.

---

<div align="center">

**[cookwithgasoline.com](https://cookwithgasoline.com)**

If you find Gasoline useful, please consider giving it a star!

[![Star on GitHub](https://img.shields.io/github/stars/brennhill/gasoline.svg?style=social)](https://github.com/brennhill/gasoline)

</div>
