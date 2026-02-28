---
title: "Gasoline v5.6.0 Released"
description: "Server reliability, persistence guarantees, and architecture tests"
date: 2026-02-06
authors: [brennhill]
tags: [release]
---

## What's New in v5.6.0

Gasoline v5.6.0 focuses on server-side reliability with persistence guarantees and comprehensive architecture invariant tests.

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
npm install -g gasoline-mcp@5.6.0
```

## Full Changelog

[v5.6.0 Release](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases/tag/v5.6.0)
