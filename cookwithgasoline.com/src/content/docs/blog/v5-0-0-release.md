---
title: "Gasoline v0.5.00 Released"
description: "Initial public release with core observability features"
date: 2026-01-04T00:03:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.5']
---

## What's New in v0.5.00

Gasoline v0.5.00 marks the initial public release of the Gasoline MCP protocol and extension. This release includes core browser observability features for AI coding assistants.

### Features

- **Browser Telemetry Capture** — Real-time streaming of console logs, network requests, and exceptions
- **MCP Protocol Integration** — Compatible with Claude Code, Cursor, Copilot, and other MCP clients
- **4-Tool Interface** — observe, generate, configure, interact tools for full-stack automation
- **Zero Dependencies** — Lightweight Go binary with no external service requirements

### Capabilities

- Real-time log and error capture
- Network request/response inspection
- Page interaction and automation
- CSS-based element highlighting
- Form filling and submission automation

## Get Started

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
gasoline-mcp --help
```

Install the browser extension from the Chrome Web Store or load it manually via Developer Mode.

## Full Changelog

[v0.5.00 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.0.0)
