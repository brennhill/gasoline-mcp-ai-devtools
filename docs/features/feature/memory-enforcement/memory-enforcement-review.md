---
status: shipped
scope: feature/memory-enforcement/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# Memory Enforcement Review

_Migrated from /specs/memory-enforcement-review.md_

# Review: Memory Enforcement Spec

**Reviewer**: Principal Engineer Review
**Spec**: `docs/ai-first/tech-spec-memory-enforcement.md`
**Date**: 2026-01-26

---

## Executive Summary

The spec addresses a real operational problem -- unbounded memory growth in long-running sessions -- and the existing implementation in `memory.go` is clean and well-structured. The three-tier threshold model is sound. However, there are critical issues with the eviction strategy that can cause memory to never actually decrease due to Go's slice retention semantics, a missing `memoryHardLimit` constant that is defined elsewhere, and a mismatch between the spec's eviction priority description and the implementation's behavior. The extension-side enforcement is underspecified and shares no coordination protocol with the server.

---

## Critical Issues (Must Fix Before Implementation)

### 1. Slice Reslicing Does Not Free Memory

**Section**: "Performance Constraints" -- "No allocations during eviction (reslicing existing slices)"

The spec treats reslicing as an advantage: "eviction of 25% of 500 entries: under 0.5ms (slice reslicing, not copying)." The current implementation in `memory.go:183` does `v.networkBodies = v.networkBodies[n:]`. This reslices but does NOT free the underlying array. The garbage collector cannot reclaim the evicted elements because the slice header still points into the original backing array. In a sustained-load scenario, eviction appears to work (the slice reports fewer elements) but actual process memory never decreases.

This is confirmed by the existing implementation: `evictCritical()` at line 211 does `v.wsEvents = v.wsEvents[:0]` which also retains the backing array.

**Fix**: After eviction, copy the surviving elements into a new slice:

```go
surviving := make([]NetworkBody, len(v.networkBodies)-n)
copy(surviving, v.networkBodies[n:])
v.networkBodies = surviving
```

This trades O(n) allocation for actual memory reclamation. Without this fix, the entire memory enforcement system is a no-op at the process level. The "under 0.5ms" performance claim would need to be revised upward, but the copy of 375 small structs is still well under 1ms.

### 2. Spec and Implementation Disagree on Eviction at Hard Limit

**Section**: "On Ingest" vs. actual code

The spec says at hard limit: "evict oldest 50%, set memory-exceeded flag (rejects future network body POSTs until memory drops)." The implementation in `evictHard()` (line 171) calls `evictBuffers(2, memoryHardLimit)` which does the 50% eviction, but does NOT set any memory-exceeded flag. The `isMemoryExceeded()` function (line 237) checks `calcTotalMemory() > memoryHardLimit` dynamically -- it does not use a sticky flag.

This means the "rejects future network body POSTs until memory drops" behavior from the spec is only partially implemented. If eviction successfully drops memory below the hard limit, the next ingest will be accepted immediately. The spec implies a sticky rejection that requires explicit clearing, but the code does not implement this.

**Decision needed**: Is the dynamic check (current code) or the sticky flag (spec) the correct behavior? The dynamic check is simpler and arguably better -- if eviction worked, there is no reason to reject new data. Recommend updating the spec to match the code and removing the sticky flag concept.

### 3. `memoryHardLimit` Is Defined in `types.go`, Not `memory.go`

**Section**: "File Locations"

The spec says existing functions in `queries.go` should be moved to `memory.go`. The implementation has already done this (the functions are in `memory.go`). However, `memoryHardLimit` is still defined in `types.go:345` as a constant. This creates a split where `memorySoftLimit` and `memoryCriticalLimit` are in `memory.go:17-18` but `memoryHardLimit` is in `types.go:345`. All three thresholds should be co-located.

**Fix**: Move `memoryHardLimit` from `types.go` to `memory.go` alongside the other memory constants.

### 4. Periodic Check Can Race with Ingest Eviction

**Section**: "Periodic Check"

The periodic goroutine (`checkMemoryAndEvict` at line 275) acquires the write lock and calls `enforceMemory()`. If an ingest operation is also running `enforceMemory()` under the same lock, they cannot race (mutex protects). However, the eviction cooldown (1 second) creates a subtle issue: if the periodic check runs during the cooldown period after an ingest-triggered eviction, it silently does nothing. If memory is still above the soft limit after a soft eviction (because 25% was not enough), the system waits up to 10 seconds (periodic interval) + 1 second (cooldown) before retrying -- an 11-second window where the server is above the soft limit.

**Fix**: After eviction in `evictBuffers`, re-check memory. If still above the threshold, evict again immediately (without cooldown). The cooldown should prevent _trigger frequency_, not _eviction completeness_. Alternatively, the eviction loop should escalate: if soft eviction does not bring memory below soft limit, immediately try hard eviction.

---

## Recommendations (Should Consider)

### 5. The Three Thresholds Have a Large Gap

**Section**: "Three Thresholds"

The gap between soft (20MB) and hard (50MB) is 30MB. The gap between hard (50MB) and critical (100MB) is 50MB. A sustained burst that pushes memory from 20MB to 100MB between two periodic checks (10 seconds apart) would skip soft and hard eviction entirely and go straight to critical (full buffer clear). This is unlikely but possible with large network body payloads.

**Recommendation**: Add a check after each ingest (which the code already does via `enforceMemory()`), but ensure the check evaluates all three thresholds sequentially, not just the first match. The current code at line 138-163 uses early returns (`return` after each threshold check), which is correct -- it escalates. But the spec's "On Ingest" section describes a sequential check (step 2, 3, 4) that implies re-checking after each level, which the code does not do.

### 6. Extension-Side Enforcement is Underspecified

**Section**: "Extension-Side Memory"

The extension enforcement is described in two sentences. Key missing details:
- How does the extension estimate memory? `entry count * average entry size` -- what is the average entry size for each buffer type?
- What happens when buffer capacities are reduced by 50% and there are already more entries than the new capacity?
- Is the 30-second check interval configurable?
- How does "disable network body capture entirely" interact with the server-side network body ingest endpoint? Does the extension stop sending, or does it send empty bodies?

**Recommendation**: Either expand this section into a proper sub-spec or defer it to a separate extension-only spec. The current description is not implementable.

### 7. Memory Calculation Ignores Connection State and Query Buffers

**Section**: "Memory Calculation"

The memory calculation sums only three buffers: WS events, network bodies, and enhanced actions. But the `Capture` struct also holds:
- `connections` map (unbounded, up to `maxActiveConns` = 20, each with string fields)
- `closedConns` (up to 10 entries)
- `pendingQueries` (up to 5 entries)
- `queryResults` map (unbounded until consumed)
- Performance snapshots and baselines
- A11y cache entries

These are individually small but collectively non-trivial. The spec acknowledges this ("catches cases where memory grows from internal bookkeeping") but the calculation does not measure them.

**Recommendation**: At minimum, add the a11y cache and performance store to the memory calculation. These can hold large JSON blobs (a11y audit results are often 50-200KB each, and the cache holds up to 10).

### 8. Test Scenario 18 is Inconsistent

**Section**: "Test Scenarios"

Test 18 says "AddWebSocketEvents at hard limit -> events rejected (not added)." But the spec's "On Ingest" section says eviction happens _before_ processing the new data (step 1-4, then step 5: "Add the new data"). If eviction at the hard limit removes 50% of entries, there should be room for the new data. The rejection should only happen if eviction fails to bring memory below the hard limit AND the memory-exceeded flag is set. The current code in `enforceMemory()` does not reject -- it evicts and then the caller proceeds to add data normally.

**Fix**: Either update the test expectation to match the implementation (evict then accept), or implement actual rejection after failed eviction.

### 9. No Metric for Eviction Effectiveness

The spec tracks `totalEvictions` and `evictedEntries` but does not track `memoryBeforeEviction` and `memoryAfterEviction`. Without these, the AI agent cannot determine whether eviction is actually working or just churning. Add per-eviction delta metrics to the `MemoryStatus` response.

---

## Implementation Roadmap

1. **Fix slice retention**: Replace all reslice evictions with copy-to-new-slice. This is the highest-priority fix -- without it, memory enforcement is cosmetic only. Update `evictBuffers` and `evictCritical` in `memory.go`.

2. **Co-locate constants**: Move `memoryHardLimit` from `types.go` to `memory.go` alongside `memorySoftLimit` and `memoryCriticalLimit`.

3. **Add escalation after eviction**: After `evictSoft`, re-check memory. If still above soft limit, call `evictHard`. After `evictHard`, re-check. If still above hard limit, call `evictCritical`. Remove early returns in `enforceMemory` and replace with cascading checks.

4. **Reconcile spec and code on hard-limit behavior**: Choose between sticky flag (spec) and dynamic check (code). Recommend dynamic check. Update the spec to match.

5. **Write missing tests**: Add test for slice memory reclamation (measure actual Go process memory before/after eviction using `runtime.MemStats`). Add test for escalation path. Add test for the cooldown interaction with periodic checks.

6. **Expand extension-side spec**: Define concrete memory estimation formulas, reduction behavior when capacity shrinks below current count, and the interaction with server-side ingest rejection.

7. **Add a11y and perf store to memory calculation**: Extend `calcTotalMemory` to include `a11y.cache` entries and `perf.snapshots`/`perf.baselines`.

8. **Add eviction effectiveness metrics**: Track pre/post memory for each eviction cycle. Expose in `GetMemoryStatus()`.
