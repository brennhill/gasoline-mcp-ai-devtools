---
status: shipped
scope: feature/api-schema/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# API Schema Review

_Migrated from /specs/api-schema-review.md_

# API Schema Inference: Technical Review

**Spec:** `docs/ai-first/tech-spec-api-schema.md`
**Implementation:** `cmd/dev-console/api_schema.go`
**Reviewer:** Principal Engineer Review
**Date:** 2026-01-26

---

## 1. Executive Summary

The API schema inference feature is already implemented and well-tested (1872 lines of tests, 27 test scenarios covered). The core design -- passive observation with on-demand schema building -- is sound and aligns with the codebase's architecture. However, the spec contains three claims that diverge from the implementation (persistence, 2MB memory enforcement, goroutine lifecycle), and the feature sits in tension with the project's "capture, don't interpret" philosophy, which warrants explicit acknowledgment.

---

## 2. Critical Issues (Must Fix)

### 2.1. Spec Claims Persistence That Does Not Exist

**Spec (Test Scenario 27):** "Schema persists and loads across sessions"

**Implementation:** `SchemaStore` is a purely in-memory struct with no serialization, no file I/O, and no load-on-startup logic. `NewSchemaStore()` returns an empty store every time the server starts. There is no corresponding test for persistence -- the test file covers 26 of the 27 scenarios but omits #27 entirely.

**Impact:** An implementer following the spec would build a persistence layer. The current behavior is correct for a zero-dependency, memory-safe design. The spec needs to either:
- (a) Remove the persistence claim, or
- (b) Spec out the persistence mechanism (format, file location, max size, corruption recovery, TTL for stale schemas).

**Recommendation:** Remove it. Schema inference is session-scoped by nature -- the AI agent starts fresh each session. Persisting inferred schemas across sessions risks serving stale/wrong type information, which directly violates the product philosophy's "zero false confidence" principle (product-philosophy.md, principle 6). If persistence is wanted later, it should be a separate spec.

### 2.2. 2MB Memory Cap Is Specified But Not Enforced

**Spec (Edge Cases):** "Total memory cap: 2MB for all accumulators combined."

**Spec (Performance Constraints):** "Total accumulator memory: under 2MB"

**Implementation:** The `SchemaStore` has no memory tracking or enforcement whatsoever. The only controls are count-based caps: `maxSchemaEndpoints=200`, `maxLatencySamples=100`, `maxActualPaths=20`, `maxQueryParamValues=10`, `maxResponseShapes=10`. These provide indirect memory bounding, but no actual byte accounting.

**Worst case analysis:** 200 endpoints, each with 10 response shapes, each with unbounded field maps (no cap on number of fields per response body). A deeply nested JSON API with 50+ fields per response across 200 endpoints could exceed 2MB of accumulator state. The `fieldAccumulator.typeCounts` map grows without bound per field.

**Impact:** The spec makes a memory guarantee that the implementation cannot honor. Either enforce it or remove the claim.

**Recommendation:** The count-based caps are pragmatically sufficient for the expected use case (browser-observed APIs rarely have 200+ unique endpoints). Remove the "2MB" claim from the spec and replace with a note that memory is bounded by the count caps. If strict enforcement is needed, add a `calcSchemaMemory()` method analogous to `calcNBMemory()` in `memory.go` with an eviction strategy (LRU by `lastSeen`).

### 2.3. Goroutine Lifecycle: Fire-and-Forget With No Cancellation

**Spec (Integration Point):** "Each new network body triggers schema inference in the background (separate goroutine, separate lock)."

**Implementation (`network.go:66-69`):**
```go
go func() {
    for _, b := range bodiesCopy {
        v.schemaStore.Observe(b)
    }
}()
```

This spawns an unbounded goroutine per `AddNetworkBodies` call with no context, no cancellation, and no waitgroup. During server shutdown (`runMCPMode`, 100ms grace period), in-flight goroutines are abandoned. Under burst traffic (extension sends batches rapidly), goroutines queue up contending on `SchemaStore.mu`.

**Impact:** Goroutine leak on shutdown. Under sustained load (e.g., SPA with aggressive polling), goroutine count grows until the lock contention itself acts as implicit backpressure. Not a crash risk but violates the principle that the system should be predictable.

**Recommendation:** Either:
- (a) Make observation synchronous inside the Capture lock (the "under 1ms per body" spec target makes this viable -- the Capture lock is already held during `AddNetworkBodies`), or
- (b) Use a buffered channel + single worker goroutine pattern with graceful drain on shutdown.

Option (a) is simpler and aligns with zero-dependency ethos. The separate mutex is still valuable to avoid holding the Capture lock during schema computation, but the goroutine-per-batch is unnecessary overhead.

---

## 3. Recommendations (Should Consider)

# ...existing code...
