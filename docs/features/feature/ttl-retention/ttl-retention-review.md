---
status: shipped
scope: feature/ttl-retention/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# TTL Retention Review (Migrated)

> **[MIGRATION NOTICE]**
> Migrated from `/docs/specs/ttl-retention-review.md` on 2026-01-26.
> Related docs: [product-spec.md](product-spec.md), [tech-spec.md](tech-spec.md), [ADRS.md](ADRS.md).

---

# Technical Review: TTL-Based Retention (Feature 16)

**Reviewer:** Principal Engineer Review
**Spec:** `docs/ai-first/tech-spec-ttl-retention.md`
**Date:** 2026-01-26
**Verdict:** Conditionally approved -- address critical issues before implementation

---

## Executive Summary

The spec proposes a sound read-time TTL filtering mechanism that layers cleanly onto the existing ring buffer architecture. The core design (read-time filtering, per-buffer TTL, time-based not LRU) is well-reasoned and composable with the existing memory enforcement system. However, the spec has a structural mismatch between the proposed `TTLConfig` data model and the existing codebase (which uses a flat `Capture.TTL` field, not the `ttlConfig` struct the spec proposes), introduces a concurrency hazard with the split Server/Capture TTL ownership, and leaves several hot-path performance costs unaddressed.

---

## 1. Critical Issues (Must Fix Before Implementation)

### C1. Split Ownership of TTL Between Server and Capture Creates Incoherent State

**Sections:** Data Model, Migration, Storage Location

The spec proposes `ttlConfig TTLConfig` inside `Capture`, but the existing codebase has `Server.TTL` for console logs and `Capture.TTL` for everything else. The spec's Migration section shows `Capture.SetTTL()` updating `ttlConfig.Global`, but `Server.SetTTL()` is a completely separate field on a different struct.

The per-buffer TTL for `console` logs cannot be resolved from `Capture.ttlConfig` because console entries live in `Server.entries` protected by `Server.mu` -- a different mutex, different struct, different ownership.

**Current code** (`ttl.go:46-63`):
```go
func (s *Server) getEntriesWithTTL() []LogEntry {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if s.TTL == 0 { ... }
    cutoff := time.Now().Add(-s.TTL)
    ...
}
```

The spec says `TTLConfig.Console` controls console log TTL, but `Server` has no reference to `Capture.ttlConfig`. Either:

**Fix A (recommended):** Move TTLConfig to a shared, independently-locked struct that both Server and Capture reference. Use `atomic.Value` or a dedicated `sync.RWMutex` so TTL reads on the hot path avoid contention with the main Capture mutex.

**Fix B:** Duplicate TTLConfig into Server, with a single `SetTTLConfig()` method that propagates to both. Less clean but simpler.

### C2. Read-Time Filtering Scales Linearly with Buffer Size on Every Read

**Sections:** Implementation Details (Read-Time Filtering), Performance

Every call to `GetWebSocketEvents`, `GetNetworkBodies`, `GetEnhancedActions`, and `getEntriesWithTTL` iterates the entire buffer to filter by TTL. With max buffer sizes (500 WS events, 100 network bodies, 50 actions, 1000 console entries), this is acceptable today. But the spec proposes no mechanism to short-circuit the scan.

Since buffers are time-ordered (ring buffer writes are monotonic in time), entries older than TTL are contiguous at the start. Instead of checking every entry, binary search for the TTL cutoff index and slice from there:

```go
// O(log n) instead of O(n)
cutoffIdx := sort.Search(len(v.wsAddedAt), func(i int) bool {
    return !v.wsAddedAt[i].Before(cutoff)
})
// All entries from cutoffIdx onward are within TTL
```

This matters because reads happen under `RLock`, and linear scans hold the lock longer than necessary. Under high-frequency MCP tool calls (AI agents poll aggressively), this becomes a contention bottleneck.

**Caveat:** Ring buffer wraparound means `wsAddedAt` is not sorted when the buffer has wrapped. The binary search optimization only works for the non-wrapped case, or requires reading entries in logical order (oldest-first from `head`). The implementation must account for this.

### C3. Memory Pressure Multiplier Can Violate Minimum TTL Invariant

**Section:** Memory Pressure Interaction

The spec defines `minTTL = 1 minute` (line 7 of ttl.go, Requirement 7 of the spec). But the memory pressure code applies multipliers that can reduce TTL below 1 minute:

```go
// If baseTTL = 2m, and pressure = critical:
// effectiveTTL = 2m * 0.25 = 30s -- below minTTL!
return time.Duration(float64(baseTTL) * criticalTTLMultiplier)
```

This silently violates the documented invariant. The user configured 2m explicitly; the system is silently reducing it to 30s.

**Fix:** Clamp the pressure-adjusted TTL to `minTTL`:
```go
adjusted := time.Duration(float64(baseTTL) * multiplier)
if adjusted < minTTL {
    adjusted = minTTL
}
return adjusted
```

### C4. `TTL=0` Semantics Are Ambiguous with Pressure-Aware Mode

**Sections:** Memory Pressure Interaction, Requirements

The spec says "TTL=0 means unlimited" and "Unlimited stays unlimited even under pressure" (line 365). But this means a user with `TTL=0` (unlimited) gets zero eviction benefit from TTL when under memory pressure. The only defense is capacity-based ring buffer eviction.

This is technically correct but should be documented as a deliberate design choice. If the server is under memory pressure with `TTL=0`, the TTL feature provides no relief at all. The spec should explicitly state: "Memory pressure interaction requires a non-zero TTL to have any effect. Systems running with `TTL=0` rely solely on ring buffer capacity limits and the existing memory enforcement goroutine."

---

## 2. Recommendations (Should Consider)

### R1. Avoid `time.Now()` in Hot Read Path

**Section:** Read-Time Filtering

Every read calls `time.Now()` to compute the cutoff. Under lock. While `time.Now()` is fast (~25ns), it involves a syscall on some platforms and defeats monotonic clock caching. Since MCP reads are serialized per-client, consider caching `time.Now()` at the start of each MCP tool handler dispatch and passing it through:

```go
func (v *Capture) GetWebSocketEvents(filter WebSocketEventFilter, now time.Time) []WebSocketEvent {
    cutoff := now.Add(-effectiveTTL)
    ...
}
```

This avoids multiple `time.Now()` calls when a single tool invocation reads multiple buffers (e.g., `get_changes_since` reads all four buffers).

### R2. TTLStats Computation Is O(n) Per Buffer Per Call

**Section:** Data Model (TTLStats)

Computing `EntriesInTTL` and `FilteredByTTL` requires scanning every entry. If health is called frequently (some agents poll health in a loop), this adds up. Consider:

- Lazy computation: Only compute stats when explicitly requested via `ttl_action: "get"`, not on every `/v4/health` call.
- The spec already says "Stats calculation frequency: On-demand" (Limits table), which is good. Ensure the health endpoint TTL section (line 223-253) does NOT include per-buffer stats, only the config/effective fields.

### R3. Maximum TTL of 24 Hours Is Not Enforced in Spec Code

**Section:** Limits

The limits table says "Maximum TTL: 24 hours" but `ParseTTL()` (ttl.go:14-29) only enforces the minimum. Add a maximum check:

```go
const maxTTL = 24 * time.Hour

if d > maxTTL {
    return 0, fmt.Errorf("TTL %v exceeds maximum (%v)", d, maxTTL)
}
```

Without this, a user can set `--ttl 720h` (30 days), which the spec claims is prevented.

### R4. TTL Duration JSON Serialization Is Inconsistent

**Sections:** Data Model, JSON Schemas

The Go struct uses `time.Duration` (nanoseconds internally), but JSON uses strings like `"15m"`. `time.Duration` does not marshal to `"15m"` -- it marshals to a raw nanosecond integer. The spec needs custom JSON marshal/unmarshal for `TTLConfig`:

```go
func (c TTLConfig) MarshalJSON() ([]byte, error) {
    return json.Marshal(struct {
        Global    string `json:"global"`
        Console   string `json:"console"`
        // ...
    }{
        Global:  formatDuration(c.Global),
        Console: formatDuration(c.Console),
    })
}
```

Without this, the API response will contain `{ "global": 900000000000 }` instead of `{ "global": "15m" }`. This is a data contract violation.

### R5. `isExpiredByTTL` Uses `time.Since()` Which Calls `time.Now()` Again

**Section:** Implementation Details

`isExpiredByTTL` (ttl.go:68-73) calls `time.Since(addedAt)` which internally calls `time.Now()`. But `getEntriesWithTTL` already computed `cutoff` from `time.Now()`. The WS/network/action read paths use `isExpiredByTTL` per-entry, calling `time.Now()` N times instead of once.

The console log path (`getEntriesWithTTL`) correctly computes `cutoff` once. The Capture read paths should do the same -- compute cutoff once, then compare `addedAt.Before(cutoff)`. The spec's example code (lines 318-320) correctly shows this pattern, but the existing `isExpiredByTTL` helper used by the current implementation does not.

### R6. Presets Are Underspecified

**Section:** Configuration Options (Presets)

The preset feature has no validation logic. What happens if `ttl_action: "set"` includes both a `preset` and explicit `ttl_config` values? Which wins? The spec should define precedence:

- **Option A:** Explicit values override preset values (merge semantics)
- **Option B:** Preset and explicit values are mutually exclusive (error if both provided)

Recommend Option B for simplicity: reject the request if both `preset` and `ttl_config` are provided.

### R7. toolGetBrowserErrors Does Not Apply TTL Filtering

**File:** `tools.go:1224-1244`

The `toolGetBrowserErrors` handler reads `h.MCPHandler.server.entries` directly without TTL filtering. It should call `server.getEntriesWithTTL()` (or a per-buffer variant) instead of accessing `server.entries` raw. Similarly, `toolGetBrowserLogs` (line 1246) reads `server.entries` directly.

This means the observe/errors and observe/logs MCP tools will return stale entries that should have been filtered by TTL. This is a correctness bug in the integration.

### R8. Ring Buffer Generic Type Does Not Support TTL

**File:** `ring_buffer.go`

The spec proposes TTL for Capture-owned slices (`wsEvents`, `networkBodies`, `enhancedActions`), but the codebase also has a generic `RingBuffer[T]` type. If the project is migrating buffers to use `RingBuffer[T]`, TTL filtering needs to be a first-class method on the generic type:

```go
func (rb *RingBuffer[T]) ReadAllWithTTL(ttl time.Duration) []T
```

Otherwise, the TTL integration will be inconsistent: some buffers use the generic ring buffer with no TTL support, others use the Capture-embedded slices with manual TTL filtering. Clarify which pattern is canonical going forward.

---

## 3. Additional Observations

### Data Contract Risks

- The `configure` tool's `action` enum expansion from the current 6 values to 7 (adding `"ttl"`) is additive and non-breaking.
- The `/v4/health` endpoint adding a `ttl` key is additive and non-breaking for clients that ignore unknown fields.
- The `TTLStats` type uses `time.Time` for `OldestEntry`/`NewestEntry`, which marshals as RFC 3339. This is consistent with existing timestamp usage. Good.

### Testing Gaps in Spec

The testing strategy (lines 470-535) is thorough but missing:

1. **Concurrent TTL update during read** -- the spec mentions this as an edge case (item 11) but provides no test skeleton. This needs a `race_test.go` entry using `t.Parallel()` and multiple goroutines.
2. **TTL interaction with `get_changes_since`** -- checkpoint-based reads should also respect TTL.
3. **TTL interaction with `diff_sessions` / `verify_fix`** -- snapshot-based tools may capture data that is later TTL-expired. What happens when comparing snapshots across a TTL boundary?

### Security

- No new attack surface. TTL values are bounded (min 1m, max 24h, if R3 is implemented).
- No data exposure risk: TTL only reduces what is returned, never expands it.
- Input validation: `ParseTTL` wraps `time.ParseDuration`, which handles edge cases. The JSON schema regex pattern (`^([0-9]+[hms])+$|^$`) is more restrictive than Go's `ParseDuration` (which supports `us`, `ns`, `ms`). This mismatch could confuse clients. Align the regex with what the server actually accepts, or document the discrepancy.

### Maintainability

- Adding TTL to 4 read paths (WS, network, actions, console) means 4 places to maintain the filtering logic. A shared helper or generic approach (see R8) would reduce this to 1.
- The spec creates a new file `cmd/dev-console/ttl.go` which already exists with simpler logic. The migration path (expanding the existing file) is correct.
- Complexity budget: This feature adds approximately 300 lines of Go (types, parsing, filtering, config handler, health integration, tests). Reasonable for the capability added.

---

## 4. Implementation Roadmap

Ordered steps, each producing a testable increment:

### Phase 1: Foundation (Tests First)

1. **Add `TTLConfig` struct and `EffectiveTTL()` method** to `types.go`
   - Write table-driven tests for TTL resolution (buffer-specific > global > unlimited)
   - Write tests for max TTL enforcement (R3)

2. **Implement custom JSON marshaling** for `TTLConfig` (R4)
   - Test round-trip: Go struct -> JSON string -> Go struct

3. **Add `ParseTTL` max validation** (R3)
   - Update existing test table in `ttl_test.go`

### Phase 2: Core Integration

4. **Resolve Server/Capture TTL ownership** (C1)
   - Create shared TTL config accessible by both Server and Capture
   - Update `Server.getEntriesWithTTL()` to use per-buffer console TTL
   - Update `Capture.GetWebSocketEvents()`, `GetNetworkBodies()`, `GetEnhancedActions()` to use per-buffer TTL

5. **Fix `isExpiredByTTL` to use precomputed cutoff** (R5)
   - Single `time.Now()` per read operation, not per entry

6. **Fix `toolGetBrowserErrors` and `toolGetBrowserLogs`** to use TTL-filtered reads (R7)

### Phase 3: API Surface

7. **Add `ttl` action to `toolConfigure`** dispatch
   - Implement `get`, `set`, `reset` sub-actions
   - Add preset support with mutual exclusivity validation (R6)

8. **Add CLI flags** (`--ttl`, `--ttl-console`, etc.)
   - Wire through `main.go` flag parsing

9. **Add TTL section to health endpoint**
   - Config + effective TTL only (not per-buffer stats, per R2)

### Phase 4: Memory Pressure (Optional, Behind Flag)

10. **Implement pressure-aware TTL multiplier** with `minTTL` clamping (C3)
    - Add `--ttl-pressure-aware` flag
    - Test that adjusted TTL never drops below `minTTL`

### Phase 5: Hardening

11. **Add race condition tests** (concurrent TTL update + read)
12. **Add integration tests** for MCP tool flow
13. **Performance benchmark**: measure read latency with and without TTL filtering at max buffer capacity
14. **Evaluate binary search optimization** (C2) -- benchmark first, implement only if contention is measured

---

## Appendix: Files Requiring Changes

| File | Change Type | Notes |
|------|-------------|-------|
| `cmd/dev-console/types.go` | Add `TTLConfig`, `TTLStats` structs | New types |
| `cmd/dev-console/ttl.go` | Expand with per-buffer logic, max TTL validation | Existing file |
| `cmd/dev-console/ttl_test.go` | Expand with per-buffer tests, migration tests | Existing file |
| `cmd/dev-console/websocket.go` | Update `GetWebSocketEvents` to use per-buffer TTL | L97 |
| `cmd/dev-console/network.go` | Update `GetNetworkBodies` to use per-buffer TTL | L123 |
| `cmd/dev-console/actions.go` | Update `GetEnhancedActions` to use per-buffer TTL | L61 |
| `cmd/dev-console/tools.go` | Add `"ttl"` case in `toolConfigure` switch, fix `toolGetBrowserErrors`/`toolGetBrowserLogs` | L1200, L1224, L1246 |
| `cmd/dev-console/health.go` | Add TTL section to `MCPHealthResponse` | L293 |
| `cmd/dev-console/main.go` | Add CLI flag parsing for TTL flags | Flag section |
| `cmd/dev-console/main_test.go` | Golden file updates for tool schema | If schema changes |
