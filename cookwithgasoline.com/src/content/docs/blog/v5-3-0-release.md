---
title: "Gasoline v0.5.30 Released"
description: "WebSocket inspection and improved performance"
date: 2026-01-13T19:34:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.5']
---

## What's New in v0.5.30

Gasoline v0.5.30 adds comprehensive WebSocket inspection and significant performance improvements.

### Features

- **WebSocket Capture** — Full message tracking for WebSocket connections
- **Real-time Message Streaming** — Monitor WebSocket traffic as it happens
- **Binary Message Support** — Handle binary WebSocket frames alongside text
- **Connection State Tracking** — Visualize connection open/close lifecycle

### Performance

- 40% reduction in memory overhead
- Optimized log buffer management
- Faster message serialization

### Fixes

- Fixed observer timeout on high message volume
- Improved handling of concurrent connections
- Better cleanup of abandoned resources

## Upgrade

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

## Full Changelog

[v0.5.30 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.3.0)
