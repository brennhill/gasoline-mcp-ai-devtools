# Gasoline Performance Benchmarks

**Generated:** 2026-02-06T06:00:00Z
**Machine:** Darwin arm64, Apple M1 Pro, 32 GB RAM
**Go:** go1.25.6 | **Node:** v24.1.0
**Version:** 5.7.5
**Commit:** 3e4cdfc

## Binary Sizes

| Platform | Size | Previous | Delta | Change |
|----------|------|----------|-------|--------|
| darwin-arm64 | 7.4 MB | 7.4 MB | 0 | 0% |
| darwin-x64 | 7.9 MB | 7.9 MB | 0 | 0% |
| linux-arm64 | 7.2 MB | 7.2 MB | 0 | 0% |
| linux-x64 | 7.8 MB | 7.8 MB | 0 | 0% |
| win32-x64 | 8.0 MB | 8.0 MB | 0 | 0% |

No binary size change from 5.7.4. Fast-start bridge code reuses existing structures.

## Go Benchmarks

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| RingBufferWriteOne | 63.32 | 0 | 0 |
| RingBufferWrite | 90.98 | 0 | 0 |
| RingBufferReadFrom | 846.7 | 4,096 | 1 |
| RingBufferReadAll | 973.8 | 8,192 | 1 |
| RingBufferWriteWithEviction | 61.47 | 0 | 0 |
| RingBufferConcurrent | 7,027 | 40,340 | 0 |
| ParseCursor | 58.98 | 0 | 0 |
| BuildCursor | 88.55 | 56 | 3 |
| EnrichLogEntries | 11,745 | 32,768 | 1 |
| EnrichWebSocketEntries | 44,114 | 237,568 | 1 |
| EnrichActionEntries | 93,011 | 269,761 | 1,001 |

MCP fast-start response time: ~130ms (including process startup). Bridge-only latency: < 1ms.

## Extension Performance

| Metric | Measured | SLO Target | Status |
|--------|----------|------------|--------|
| Simple object serialization | < 0.1ms | < 0.1ms | PASS |
| Nested object serialization | < 0.5ms | < 0.5ms | PASS |
| Large array serialization | < 1ms | < 1ms | PASS |
| Circular reference handling | < 0.5ms | < 0.5ms | PASS |
| Large string truncation | < 1ms | < 1ms | PASS |
| Error signature computation | < 0.1ms | < 0.1ms | PASS |
| Network error signature | < 0.1ms | < 0.1ms | PASS |
| Log entry formatting | < 0.1ms | < 0.1ms | PASS |
| Large args formatting | < 1ms | < 1ms | PASS |
| Error group processing | < 0.2ms | < 0.2ms | PASS |
| Resource timing parse | < 0.1ms/entry | < 0.1ms/entry | PASS |
| Waterfall (50 entries) | < 5ms | < 5ms | PASS |
| Action recording | < 0.1ms | < 0.1ms | PASS |
| Complete error flow | < 5ms | < 5ms | PASS |
| Memory bounded growth | bounded | bounded | PASS |

All 15 performance tests passing.

## Test Coverage

| Metric | Value |
|--------|-------|
| Go coverage | 23.2% |
| Go test count | 2,319 |
| JS test count | 808+ |
| Total | 3,127+ |

## SLO Compliance

| SLO | Target | Measured | Status |
|-----|--------|----------|--------|
| Binary size | < 15 MB | 8.0 MB (max) | PASS |
| MCP fast-start | < 1s | ~130ms | PASS |
| MCP tool response | < 200ms | 0.001ms (max) | PASS |
| Serialization (simple) | < 0.1ms | < 0.1ms | PASS |
| Error path total | < 5ms | < 5ms | PASS |
| Daemon startup | < 4s | ~500ms typical | PASS |
| fetch() overhead | < 0.5ms | unmeasured (needs E2E) | UNMEASURED |
| WS handler | < 0.1ms/msg | unmeasured (needs E2E) | UNMEASURED |
| Page load impact | < 20ms | unmeasured (needs Lighthouse) | UNMEASURED |
| Server memory | < 30MB | unmeasured (needs load test) | UNMEASURED |
