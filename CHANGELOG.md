# Changelog

All notable changes to Dev Console will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [5.1.0] - 2026-01-28

### Security

- **Single-tab tracking isolation** — Extension now only captures telemetry from the explicitly tracked tab. Previously, data from ALL browser tabs was captured regardless of tracking state. This was a critical privacy vulnerability.

### Added

- **"Track This Tab" button** — Replaces "Track This Page". Attaches to a single browser tab for telemetry capture. One-click enable/disable.
- **"No Tracking" mode** — When no tab is tracked, LLM receives clear warning with actionable instructions.
- **Status ping endpoint** (`/api/extension-status`) — Extension pings server every 30s with tracking state.
- **Chrome internal page blocking** — Cannot track `chrome://`, `about:`, or other internal pages.
- **Browser restart handling** — Tracking state cleared automatically on browser restart.
- **Tab ID attachment** — All forwarded messages include `tabId` for data attribution.
- **40 new tab-filtering tests** — Comprehensive test coverage for isolation, filtering, and edge cases.

### Changed

- **validate_api parameter** — Renamed from conflicting `action`/`api_action` to `operation` for LLM discoverability.
- **Network schema improvements** — Unit suffixes (`durationMs`, `transferSizeBytes`), `compressionRatio`, `capturedAt` timestamps, helpful `limitations` array.

### Infrastructure
- **PyPI distribution** — `pip install gasoline-mcp` now available alongside NPM
- **Repository renamed** to `gasoline-mcp-ai-devtools` for SEO discoverability
- **Documentation overhaul** — Consolidated tests into `tests/`, assets into `docs/assets/`, ADRs into `docs/adrs/`, added templates, feature index, and master doc navigation

### Known Issues

See [KNOWN-ISSUES.md](KNOWN-ISSUES.md) for issues targeted for v5.2.

## [5.0.0] - 2026-01-26

### Added
- **4-tool MCP architecture** — `observe`, `generate`, `configure`, `interact` (replaced discrete tools)
- **AI Web Pilot** — Browser automation via MCP (navigate, click, type, execute JS, save/load state)
- **Async command execution** — Background commands between MCP server and browser extension
- **Enterprise audit trail** — Tamper-evident logging with SARIF export
- **API schema inference** — Auto-discover OpenAPI schemas from captured network traffic
- **Web Vitals monitoring** — LCP, CLS, INP, FCP with automatic regression detection
- **Error clustering** — Automatic grouping with noise filtering
- **Session checkpoints** — Named save points with diff support
- **HAR export** — Standard HTTP Archive from captured network data
- **CSP and SRI generation** — Security policy generation from audit data
- **Temporal graph** — Cross-session history tracking
- **Rate limiting and memory enforcement** — Resource management with configurable TTLs
- **Binary format detection** — Smart handling of non-text network bodies
- **NPM distribution** — `npx gasoline-mcp` with platform-specific binaries

### Changed
- Migrated from multiple discrete tools to 4 composite tools
- Restructured MCP tool descriptions for LLM optimization
- Go server rewrite (zero dependencies, single binary)

## [1.0.0] - 2024-01-22

### Added

- Initial release
- **Server**
  - Zero-dependency Node.js server
  - JSONL log file format
  - Log rotation (configurable max entries)
  - CORS support for browser extension
  - Health check endpoint
  - Clear logs endpoint
- **Browser Extension**
  - Console capture (log, warn, error, info, debug)
  - Network error capture (4xx, 5xx responses)
  - Exception capture (onerror, unhandled rejections)
  - Configurable capture levels
  - Domain filtering
  - Connection status badge
- **Landing Page**
  - Quick start instructions
  - Feature overview
  - Privacy information
