---
title: "Gasoline v0.5.60 Released"
description: "Server reliability, persistence guarantees, and architecture tests"
date: 2026-02-06
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.5']
---

## What's New in v0.5.60

Gasoline v0.5.60 focuses on server-side reliability with persistence guarantees and comprehensive architecture invariant tests.

### Features

- **Persistent Message Queue** — Guarantees no messages lost during server restarts
- **Transaction-Safe State** — Atomic operations for observer state updates
- **Architecture Validation** — New test suite validating core invariants

### Improvements

- Improved graceful shutdown of long-running observations
- Better handling of concurrent client connections
- Enhanced observability into server health and performance
- Stricter validation of MCP protocol compliance

### Testing

- 50+ new architecture invariant tests
- Stress testing with high message volume
- Connection resilience testing

## Upgrade

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

## Full Changelog

[v0.5.60 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.6.0)
