# Gasoline Performance Benchmarks

**Generated:** 2026-01-24T15:57:00Z
**Machine:** Darwin arm64, Apple M1 Pro
**Go:** go1.25.6 | **Node:** v24.1.0
**Version:** 4.8.0
**Commit:** e8d5d19

## Binary Sizes

| Platform | Size |
|----------|------|
| darwin-arm64 | 6.2 MB |
| darwin-x64 | 6.7 MB |
| linux-arm64 | 6.1 MB |
| linux-x64 | 6.5 MB |
| win32-x64 | 6.7 MB |

All binaries well under the 15 MB hard limit.

## Go Benchmarks

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| AddEntries | 6,893,869 | 236,509 | 6,215 |
| AddEntriesBatch | 18,374,462 | 502,618 | 15,310 |
| LogRotation | 877,690 | 22,499 | 603 |
| MCPGetBrowserErrors | 124,398 | 46,811 | 728 |
| MCPGetBrowserLogs | 407,870 | 142,238 | 2,534 |
| PostLogsHTTP | 10,659,062 | 256,577 | 8,478 |

MCP tool responses (GetBrowserErrors: ~0.12ms, GetBrowserLogs: ~0.41ms) are well within the 200ms SLO target.

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

All 16 performance tests passing.

## Test Coverage

| Metric | Value |
|--------|-------|
| Go coverage | 95.0% |
| Go test count | 312+ |
| JS test count | 691 |
| Total | 1003+ |

## SLO Compliance

| SLO | Target | Measured | Status |
|-----|--------|----------|--------|
| Binary size | < 15 MB | 6.7 MB (max) | PASS |
| Go coverage | >= 90% | 95.0% | PASS |
| MCP tool response | < 200ms | 0.41ms (max) | PASS |
| Serialization (simple) | < 0.1ms | < 0.1ms | PASS |
| Error path total | < 5ms | < 5ms | PASS |
| fetch() overhead | < 0.5ms | unmeasured (needs E2E) | UNMEASURED |
| WS handler | < 0.1ms/msg | unmeasured (needs E2E) | UNMEASURED |
| Page load impact | < 20ms | unmeasured (needs Lighthouse) | UNMEASURED |
| Server memory | < 30MB | unmeasured (needs load test) | UNMEASURED |
