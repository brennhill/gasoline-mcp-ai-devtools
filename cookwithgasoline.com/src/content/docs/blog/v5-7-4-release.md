---
title: "Gasoline v0.5.74 Released"
description: "Stability and MCP reliability improvements"
date: 2026-02-07T22:56:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.5']
---

## What's New in v0.5.74

Gasoline v0.5.74 improves stability and MCP protocol reliability based on production feedback.

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
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

## Full Changelog

[v0.5.74 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.7.4)
