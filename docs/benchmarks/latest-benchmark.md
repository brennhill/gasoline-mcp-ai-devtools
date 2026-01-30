# Gasoline Performance Benchmarks

**Generated:** 2026-01-28T00:51:00Z
**Machine:** Darwin arm64, Apple M1 Pro, 32 GB RAM
**Go:** go1.25.6 | **Node:** v24.1.0
**Version:** 5.2.5
**Commit:** 3d1dc3d

## Binary Sizes

| Platform | Size | Previous | Delta | Change |
|----------|------|----------|-------|--------|
| darwin-arm64 | 7.4 MB | 7.3 MB | +0.1 MB | +1.4% |
| darwin-x64 | 7.9 MB | 7.8 MB | +0.1 MB | +1.3% |
| linux-arm64 | 7.2 MB | 7.1 MB | +0.1 MB | +1.4% |
| linux-x64 | 7.8 MB | 7.7 MB | +0.1 MB | +1.3% |
| win32-x64 | 8.0 MB | 7.9 MB | +0.1 MB | +1.3% |

Minor size increase from new `/api/extension-status` endpoint and tracking state fields. All remain well under the 15 MB hard limit.

## Go Benchmarks

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| DetectBinaryFormat/messagepack | 34.58 | 48 | 1 |
| DetectBinaryFormat/protobuf | 68.76 | 64 | 2 |
| DetectBinaryFormat/cbor | 28.79 | 48 | 1 |
| DetectBinaryFormat/bson | 38.34 | 48 | 1 |
| DetectBinaryFormat/text | 16.50 | 0 | 0 |
| AddEntries | 4,881,288 | 358,830 | 7,209 |
| AddEntriesBatch | 693,622 | 32,764 | 604 |
| LogRotation | 541,701 | 30,557 | 608 |
| MCPGetBrowserErrors | 1,346 | 355 | 9 |
| MCPGetBrowserLogs | 1,488 | 376 | 9 |
| PostLogsHTTP | 58,191 | 10,419 | 97 |
| RedactSmallInput | 703,504 | 113,996 | 31 |

MCP tool responses (GetBrowserErrors: ~0.001ms, GetBrowserLogs: ~0.001ms) are well within the 200ms SLO target.

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
| Go coverage | 83.1% |
| Go test count | 2,319 |
| JS test count | 808+ |
| Total | 3,127+ |

## SLO Compliance

| SLO | Target | Measured | Status |
|-----|--------|----------|--------|
| Binary size | < 15 MB | 8.0 MB (max) | PASS |
| Go coverage | >= 90% | 83.1% | WARN |
| MCP tool response | < 200ms | 0.001ms (max) | PASS |
| Serialization (simple) | < 0.1ms | < 0.1ms | PASS |
| Error path total | < 5ms | < 5ms | PASS |
| fetch() overhead | < 0.5ms | unmeasured (needs E2E) | UNMEASURED |
| WS handler | < 0.1ms/msg | unmeasured (needs E2E) | UNMEASURED |
| Page load impact | < 20ms | unmeasured (needs Lighthouse) | UNMEASURED |
| Server memory | < 30MB | unmeasured (needs load test) | UNMEASURED |
