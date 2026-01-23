# Gasoline Roadmap

## In Progress

### Performance Budget Monitor (`check_performance`)
- **Branch:** `feature/performance-budget-monitor`
- **Status:** Being handled by agent
- **Spec:** [docs/performance-budget-spec.md](docs/performance-budget-spec.md)
- **Summary:** MCP tool that gives AI coding assistants a performance snapshot of the current page (load timing, network weight, main-thread blocking). Highlights regressions when baseline data exists.

## Completed

### Circuit Breaker + Exponential Backoff
- Extension resilience when server is down
- States: closed → open (after N failures) → half-open (after timeout)
- Backoff: doubles each failure, caps at configurable max

### Memory Enforcement (Auto-Eviction)
- Per-buffer limits: 4MB WebSocket, 8MB Network Bodies
- Global hard limit: 50MB
- Server returns 503 when memory exceeded
- FIFO eviction of oldest entries

## Planned

### Network Body Capture E2E Tests
- End-to-end validation of request/response body capture
- Coverage for large bodies, streaming, and error cases

### Capture Profiles
- Configurable capture modes (minimal, standard, verbose)
- Per-site profile overrides

### Extension Health Metrics via MCP
- Expose extension internal state (buffer sizes, circuit breaker status)
- MCP tool for AI assistants to check extension health
