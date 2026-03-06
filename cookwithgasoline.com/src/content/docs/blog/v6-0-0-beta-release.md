---
title: "Gasoline v0.6.00 Released"
description: "AI agents can now check all links on your page, automate your browser with complete visibility, and identify performance bottlenecks in real time."
date: 2026-02-09T23:34:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.6', 'beta']
---

## What's New in v0.6.00

Gasoline v0.6.00 introduces the Link Health Analyzer, plus browser automation, recording, and performance analysis for AI agents. Check all links on your page, record full sessions with video, capture performance metrics, and let AI agents test, debug, and fix your app automatically. Complete visibility. You stay in control.

### Features

- **Link Health Analyzer** — Automatically check all links on your page for issues (broken, redirects, auth-required). 20 concurrent checks, categorized results, and async tracking with correlation IDs.

- **Full Recording System** — Record browser tabs with video and audio. Videos stream to local disk. No cloud, no transcoding—raw WebM format.

- **Permission Prompts** — When recording starts, you get a clear prompt to approve it. No silent recordings. You're always in control.

### Security

- **CWE-942 Fixed** — Replaced wildcard `postMessage` origins with `window.location.origin` across content scripts, test helpers, and background workers. Prevents message hijacking on cross-origin pages.

- **Secure Cookie Attributes** — Cookie deletion and restoration now include `Secure` and `SameSite` attributes, preventing session fixation and CSRF vulnerabilities.

- **Path Traversal Protection** — Hardened file operations in extension persistence layer to prevent directory traversal attacks.

- **Input Validation** — Comprehensive validation of extension log queue capacity (2000-entry cap) and screenshot rate limiter bounds to prevent unbounded memory growth.

### Performance

- **Smart HTTP Timeouts** — 5s default timeout for localhost operations, extended to 30s+ only when accessibility features are requested. Reduces false positives while respecting slow connections.

- **Atomic File Writes** — Log rotation uses temp + rename pattern, preventing partial writes and data loss on disk full.

- **Efficient Deduplication** — SeenMessages pruning optimized for large event volumes.

### Testing

- **99.4% Pass Rate** — 154 out of 155 smoke tests pass (one known edge case with watermark on rapid navigation).
- **Comprehensive UAT Suite** — 140 tests covering recording, permissions, security, performance, and WebSocket capture.
- **Full TypeScript Strict Mode** — No implicit any, zero Codacy security issues.

### Breaking Changes

- **Extension v0.5.x → v0.6.00** — Auto-update via Chrome. Manual re-add may be required if permissions are denied.
- **MCP Server** — Same 4-tool interface; no API changes.

## Upgrade

### Browser Extension

**[📥 Download gasoline-extension-v0.6.00.crx](/downloads/gasoline-extension-v6.0.0.crx)** (480 KB)

Extension ID: `behrmkvjipzkr7hu6mwmbt5vpdgcdyvk`

Drag-drop the signed CRX into `chrome://extensions/`:

1. Download the file above
2. Open `chrome://extensions/`
3. Enable Developer mode (top right)
4. Drag and drop the `.crx` file into the page
5. Click "Add extension" when prompted

[Full installation guide →](/downloads/)

### MCP Server

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
~/.gasoline/bin/gasoline-agentic-browser --help
```

## Known Limitations

- **Recording audio on muted tabs** — Tab audio capture requires tab to have sound playing. Silent tabs record video only.
- **Watermark on rapid navigation** — Watermark may not re-appear if user navigates during recording. Next navigation resets correctly.
- **Chrome 120+ only** — Manifest v0.3 (MV3) requires Chrome 120 or later. No Safari/Firefox support in v6.

## What's Next

- **File Upload API** — Automated file form handling for bulk uploads to no-API platforms.
- **Replay System** — Event playback with timeline scrubbing.
- **Deployment Integration** — Capture git-linked deploy events for post-incident correlation.

## Full Changelog

[v0.5.80...v0.6.00](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/compare/v5.8.0...v6.0.0)
