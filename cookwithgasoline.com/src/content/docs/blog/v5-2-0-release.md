---
title: "Gasoline v5.2.0 Released"
description: "Improved error handling and network inspection"
date: 2026-01-10T20:15:00Z
authors: [brennhill]
tags: [release]
---

## What's New in v5.2.0

Gasoline v5.2.0 improves error handling, network inspection, and adds better filtering options for high-volume environments.

### Features

- **Enhanced Error Clustering** — Group related errors for clearer debugging
- **Network Filtering** — Filter requests by status, content-type, and size
- **Better Error Context** — Stack traces and request/response details inline
- **Performance Improvements** — Reduced memory usage on long-running sessions

### Improvements

- Error deduplication across similar stack traces
- Network waterfall visualization improvements
- Better handling of WebSocket connections
- Improved timeout handling for slow networks

## Upgrade

```bash
npm install -g gasoline-mcp@5.2.0
```

## Full Changelog

[v5.2.0 Release](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases/tag/v5.2.0)
