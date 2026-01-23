---
title: "Performance SLOs"
description: "Gasoline's performance guarantees. Less than 0.1ms per console intercept, 20MB memory cap, and zero main thread blocking."
keywords: "browser extension performance, zero overhead debugging, extension performance SLO, browser extension memory usage"
permalink: /performance-slos/
toc: true
toc_sticky: true
---

Gasoline is designed to have minimal impact on page performance. These Service Level Objectives (SLOs) are enforced by benchmark tests.

## Latency Targets

| Operation | Target | Description |
|-----------|--------|-------------|
| Console interception | < 0.1ms | Overhead per console.log/warn/error call |
| Error serialization | < 1ms | Serializing typical error payloads |
| Error signature computation | < 0.1ms | Computing dedup signature per error |
| Log entry formatting | < 0.1ms | Formatting entry before sending |
| Error group processing | < 0.2ms | Deduplication and grouping logic |
| User action recording | < 0.1ms | Recording a click/input event |
| Network waterfall (50 req) | < 5ms | Collecting timing data for 50 requests |
| Full error path | < 5ms | Total time from error to queued for server |

## Memory Safeguards

| Safeguard | Limit | Purpose |
|-----------|-------|---------|
| String truncation | 10KB | Cap large log arguments |
| Screenshots | 5s rate, 10/session | Prevent capture flood |
| Network waterfall | 50 entries | Limit data collection |
| Performance marks | 50 entries | Limit data collection |
| User action buffer | 20 items | Rolling buffer, oldest dropped |
| Error dedup window | 5 seconds | Suppress duplicate errors |
| Error groups tracked | 100 max | Bound memory usage |
| Debug log buffer | 200 entries | Circular buffer |
| WebSocket buffer | 4MB | Per-buffer memory cap |
| Network bodies buffer | 8MB | Per-buffer memory cap |
| Global hard limit | 50MB | Total extension memory ceiling |

## Design Principles

- **Never block the main thread** — all capture is synchronous but < 0.1ms
- **Deferred setup** — WebSocket and network intercepts wait until after `load` event
- **Adaptive sampling** — high-frequency streams are sampled, not buffered entirely
- **FIFO eviction** — when buffers fill, oldest entries are dropped
- **Circuit breaker** — if the server is down, the extension backs off exponentially

## Running Benchmarks

```bash
node --test extension-tests/performance.test.js
```

This runs the full benchmark suite and fails if any SLO is exceeded.
