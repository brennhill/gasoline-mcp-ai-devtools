---
title: "Gasoline v5.7.4 Released"
description: "Stability and MCP reliability improvements"
date: 2026-02-07T22:56:00Z
authors: [brennhill]
tags: [release]
---

## What's New in v5.7.4

Gasoline v5.7.4 improves stability and MCP protocol reliability based on production feedback.

### Improvements

- Better handling of slow client connections
- Improved timeout recovery and reconnection logic
- Enhanced message serialization performance
- More robust error reporting to clients

### Fixes

- Fixed observer timeout on pages with extremely high event volume
- Resolved occasional message ordering issues
- Improved cleanup of abandoned connections
- Better resilience to malformed MCP requests

### Performance

- Reduced latency for high-frequency events
- Optimized buffer management for large responses

## Upgrade

```bash
npm install -g gasoline-mcp@5.7.4
```

## Full Changelog

[v5.7.4 Release](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases/tag/v5.7.4)
