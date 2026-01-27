# Server Rate Limiting & Memory Enforcement

## Status: Implementation Ready

---

## Overview

The server currently has rate limiting checks (`isRateLimited()`) and memory checks (`isMemoryExceeded()`) but enforcement is incomplete:

1. **Rate limiting**: Returns 429 on WebSocket events but not on other ingest endpoints
2. **Memory enforcement**: Checks exist but never trigger automatic buffer clearing
3. **No recovery**: Once memory is exceeded, the server stays in a degraded state until restart

This spec completes the reliability story: reject excess traffic, clear buffers when full, and recover gracefully.

---

## Current State (What Exists)

```go
// v4.go - Already implemented:
rateLimitThreshold  = 1000              // events/sec threshold
memoryHardLimit     = 50 * 1024 * 1024  // 50MB

func (v *V4Server) isRateLimited() bool     // checks eventCount > threshold
func (v *V4Server) isMemoryExceeded() bool   // checks calcTotalMemory() > hardLimit
func (v *V4Server) RecordEventReceived()     // increments eventCount per second
```

**What's missing:**
- `RecordEventReceived()` is never called from HTTP handlers
- `isRateLimited()` is only checked in `HandleWebSocketEvents`
- `isMemoryExceeded()` returns 503 but never clears buffers
- No logging or MCP notification when limits are hit

---

## Specification

### 1. Rate Limiting (429)

**All ingest endpoints** must check rate limits before accepting data:

| Endpoint | Currently Rate Limited | Should Be |
|----------|----------------------|-----------|
| `POST /websocket-events` | Yes | Yes |
| `POST /network-bodies` | No | Yes |
| `POST /enhanced-actions` | No | Yes |
| `POST /logs` | No | Yes |
| `POST /screenshots` | No | No (binary, already slow) |

**Behavior:**
1. On every POST to an ingest endpoint, call `RecordEventReceived()`
2. If `isRateLimited()` returns true, respond with `429 Too Many Requests`
3. Response body: `{"error": "rate_limited", "retry_after_ms": 1000}`
4. The 1-second window resets automatically (existing behavior)

**Extension handling:** The extension already has a circuit breaker with exponential backoff. A 429 response will trigger the backoff naturally.

### 2. Memory Enforcement (Auto-Clear)

When `isMemoryExceeded()` is true, the server should **proactively clear** the oldest 50% of each buffer rather than just refusing new data:

```go
func (v *V4Server) enforceMemoryLimit() bool {
    if !v.isMemoryExceeded() {
        return false
    }
    // Clear oldest 50% of each buffer
    if len(v.wsEvents) > 0 {
        v.wsEvents = v.wsEvents[len(v.wsEvents)/2:]
    }
    if len(v.networkBodies) > 0 {
        v.networkBodies = v.networkBodies[len(v.networkBodies)/2:]
    }
    if len(v.enhancedActions) > 0 {
        v.enhancedActions = v.enhancedActions[len(v.enhancedActions)/2:]
    }
    return true
}
```

**When to enforce:**
- On every `AddWebSocketEvents()` call (after adding)
- On every `AddNetworkBodies()` call (after adding)
- On every `AddEnhancedActions()` call (after adding)

**Behavior:**
1. After adding new data to any buffer, check `isMemoryExceeded()`
2. If exceeded, clear oldest 50% of all buffers
3. Log a warning: `"memory limit exceeded, cleared buffers (was: %dMB, now: %dMB)"`
4. Set a flag `lastMemoryClear time.Time` for diagnostics
5. Continue accepting new data (the server has recovered)

### 3. Health Endpoint Enhancement

The existing `/health` endpoint should report rate limiting and memory status:

```json
{
  "status": "ok",
  "uptime": "2h15m",
  "buffers": {
    "websocket_events": 342,
    "network_bodies": 87,
    "enhanced_actions": 15,
    "endpoints": 23
  },
  "memory": {
    "total_bytes": 2456000,
    "limit_bytes": 52428800,
    "last_cleared": null
  },
  "rate": {
    "current_events_per_sec": 45,
    "limit_events_per_sec": 1000,
    "limited_since": null
  }
}
```

### 4. MCP Diagnostics

Add internal state to the `get_page_info` response or a new section:

```json
{
  "server_health": {
    "rate_limited": false,
    "memory_percent": 4.7,
    "buffers_cleared_count": 0
  }
}
```

This lets AI agents know if they're missing data due to buffer clears.

---

## Constants

```go
const (
    rateLimitThreshold    = 1000                   // events/sec (existing)
    memoryHardLimit       = 50 * 1024 * 1024       // 50MB (existing)
    memoryClearPercent    = 50                      // clear 50% on exceed
)
```

---

## Test Cases

### Rate Limiting (`TestRateLimiting`)

| Test | Setup | Expected |
|------|-------|----------|
| Under threshold | Send 999 events in 1 second | All return 200 |
| At threshold | Send 1001 events in 1 second | 1001st returns 429 |
| Reset after 1 second | Exceed, wait 1.1s, send again | Returns 200 |
| All endpoints limited | Exceed on /websocket-events, POST to /network-bodies | Also returns 429 |
| Response body | Trigger 429 | Body contains `retry_after_ms: 1000` |
| GET not limited | Exceed rate, GET /websocket-events | Returns 200 (reads never limited) |

### Memory Enforcement (`TestMemoryEnforcement`)

| Test | Setup | Expected |
|------|-------|----------|
| Under limit | Add data totaling 10MB | No clearing occurs |
| At limit | Add data totaling 51MB | Buffers cleared to ~25MB |
| Recovery | Exceed, verify cleared, add more | Accepts new data normally |
| All buffers cleared | Exceed with data in all buffers | All buffers reduced by 50% |
| Flag set | Trigger clear | `lastMemoryClear` is set |
| Multiple clears | Add 51MB, clear, add 51MB again | Clears again, no crash |

### Health Endpoint (`TestHealthEndpoint`)

| Test | Expected |
|------|----------|
| Normal state | `status: ok`, buffer counts accurate, no limits active |
| Rate limited | `rate.limited_since` is non-null |
| After memory clear | `memory.last_cleared` is non-null |

### Integration (`TestRateLimitAndMemoryTogether`)

| Test | Setup | Expected |
|------|-------|----------|
| Rate limit + memory OK | 1001 events, 10MB total | 429, no buffer clear |
| Rate OK + memory exceeded | 100 events, 51MB total | 200, buffers cleared |
| Both exceeded | 1001 events, 51MB total | 429 first, then clear |

---

## SDK Replacement Angle

### What This Replaces

| Traditional Tool | What It Does | Gasoline Equivalent |
|-----------------|--------------|---------------------|
| Sentry Rate Limiting | SDK-side event throttling to stay under plan limits | Server-side 429 + extension circuit breaker |
| DataDog Agent Memory | Agent process memory management and data flushing | `enforceMemoryLimit()` auto-clear |
| LogRocket Session Limits | Max events per session before data is dropped | Buffer caps + LRU eviction |

### Key Differentiators

1. **Transparent.** When data is dropped, the AI knows (via health endpoint). SDKs silently sample.
2. **Local recovery.** Buffer clearing is instant and local — no "wait for cloud to catch up."
3. **No billing pressure.** SDKs rate-limit to stay within pricing tiers. Gasoline rate-limits for process health only.

---

## Extension Changes

**None.** The extension already has circuit breaker handling for non-200 responses. A 429 triggers exponential backoff automatically.

---

## Implementation Notes

- All changes are in `cmd/dev-console/v4.go` (enforcement logic) and `cmd/dev-console/main.go` (handler wiring)
- Approximately 50 lines of new code + 30 lines of test updates
- No new dependencies
