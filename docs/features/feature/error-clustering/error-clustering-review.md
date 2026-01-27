# Error Clustering Review

_Migrated from /specs/error-clustering-review.md_

# Error Clustering Spec Review

## Executive Summary

The spec is well-scoped and already implemented. The 2-of-3 signal matching
model is sound, the session-scoped lifecycle is correct for this product,
and the implementation aligns with the spec. However, the implementation
has several concurrency hazards, an unbounded memory growth path in the
unclustered list, and missing spec coverage for the `Cleanup` goroutine
lifecycle. These need to be addressed before the feature is considered
production-ready.

---

## Critical Issues (Must Fix)

### C1. Unbounded unclustered error list

**Spec section:** Edge Cases -- "Very high error rate"

The spec caps instances per cluster (20) and active clusters (50), but
the `unclustered` slice in `ClusterManager` has no cap. If an application
emits 10,000 unique errors per minute (each with a distinct message and
distinct stack), every one goes into `cm.unclustered` with no eviction.
This is the primary memory exhaustion vector.

The implementation at `clustering.go:232` simply appends without bounds:

```go
cm.unclustered = append(cm.unclustered, err)
```

**Fix:** Cap `unclustered` at a reasonable limit (e.g., 200). Evict oldest
when exceeded. Document this cap in the spec's Performance Constraints
section alongside the existing cluster caps.

### C2. Cleanup is never invoked

**Spec section:** Cluster Lifecycle -- "removed when no new instances arrive for 5 minutes"

`ClusterManager.Cleanup()` exists but nothing calls it. There is no
background goroutine, no ticker, and no call from `AddError`. Clusters
never expire during a running session.

**Fix:** Either:
- (a) Start a ticker goroutine in `NewClusterManager` (preferred; add a
  `Stop()` method for clean shutdown and testability), or
- (b) Call `cm.Cleanup()` at the top of `AddError()` to lazily expire
  stale clusters on each new error. This is simpler but means clusters
  linger until the next error arrives.

Option (a) is cleaner. The goroutine should run every 30-60 seconds.

### C3. O(n^2) scan on every new error against unclustered list

**Spec section:** Performance Constraints -- "Cluster matching per new error: under 1ms"

`AddError` iterates all existing clusters (up to 50, fast), then iterates
all unclustered errors (unbounded per C1). For each unclustered error, it
calls `countSignals`, which re-parses the stack trace of the existing
unclustered error every time (`parseStack(existing.Stack)` at
`clustering.go:276`).

With 500 unclustered errors, each with a 20-line stack trace, this is
500 * 20 regex operations per new error. That will exceed the 1ms SLO.

**Fix:**
- Cache parsed frames on the `ErrorInstance` or in a parallel lookup
  structure. Store `normalizedMsg` and `appFrames` at insertion time.
- Enforce the cap from C1 to bound the scan.

### C4. Race condition: `onEntries` callback accesses `ClusterManager` while holding no server lock but assuming temporal ordering

**File:** `tools.go:206-246`, `main.go:532-538`

The `onEntries` callback is invoked _after_ `s.mu.Unlock()` in
`addEntries` (line 533). This is intentionally outside the server lock to
avoid holding it during cluster processing. However, two concurrent
`addEntries` calls can invoke `onEntries` concurrently, and
`ClusterManager.AddError` takes its own lock. This is safe from a data-race
perspective, but it means error ordering is not guaranteed -- two batches
can interleave, causing temporal proximity calculations to be
non-deterministic.

This is acceptable for the current design, but the spec should acknowledge
that temporal proximity is approximate under concurrent ingestion. Not a
functional bug, but worth documenting.

---

## Recommendations (Should Consider)

### R1. Missing spec coverage: `{path}` and `{string}` normalization placeholders

**Spec section:** Message Normalization

The spec lists six placeholder types: `{uuid}`, `{id}`, `{url}`, `{path}`,
`{timestamp}`, `{string}`. The implementation only handles four: `{uuid}`,
`{url}`, `{timestamp}`, `{id}`. File paths and long quoted strings are
not normalized.

Either add the missing regex patterns or remove `{path}` and `{string}`
from the spec. Adding `{path}` (e.g., `/Users/foo/bar/baz.js`) is low-risk.
Adding `{string}` (quoted strings > 20 chars) risks over-normalizing and
collapsing distinct errors into the same template. I would drop `{string}`
from the spec.

### R2. Stack frame comparison uses file:line but spec says "2+ stack frames in the same call path"

**Spec section:** Cluster Formation -- Signal 1

The spec says "sharing 2+ stack frames." The implementation at
`matchesCluster` (line 249) only requires `shared >= 1`. The
`countSignals` method for unclustered matching also uses `>= 1`. The
spec-implementation mismatch should be reconciled.

Since the implementation requires 2 signals total (and stack similarity
is just one signal), requiring 2+ shared frames within the stack signal
is arguably too strict. I recommend updating the spec to say "1+ shared
app-code frames" to match the implementation, since false positives are
controlled by the 2-of-3 gate.

### R3. `ClusterSummary` omits `instances` array from spec response

**Spec section:** MCP Interface -- Response

The spec response includes an `instances` array per cluster. The
`ClusterSummary` struct in the implementation does not include it. The
`GetAnalysisResponse` method returns only summary fields.

This is a data contract gap. Either add instances to the response or
remove them from the spec. Given token budget concerns, I recommend adding
instances behind an optional `include_instances: true` parameter, or
limiting to the first 5.

### R4. "Representative error" selection is always the first error, not "the most informative"

**Spec section:** Cluster Structure -- "the first or most informative instance"

The implementation always uses the first error as the representative
(`createCluster` at line 329 sets `Representative: first`). It never
re-evaluates as more informative instances arrive (e.g., one with a
longer stack trace). This is fine for now -- simplicity over perfection.
But the spec should drop "or most informative" since it implies a
selection heuristic that doesn't exist.

### R5. Alert integration is one-deep

`ClusterManager.pendingAlert` is a single pointer. If two clusters hit
3 instances in the same ingestion batch, only the last alert survives.
Consider using a slice (`pendingAlerts []Alert`) drained by `DrainAlert`.

### R6. `Cleanup` does not expire unclustered errors

`Cleanup` only removes expired clusters. The `unclustered` slice grows
indefinitely even with cleanup running. Unclustered errors from 30
minutes ago are not useful for matching. Add an age-based eviction for
unclustered errors (same 5-minute window).

### R7. Test gap: no test for `Cleanup` of unclustered errors

The test `TestClusterExpiresAfterInactivity` covers cluster expiry. No
test covers unclustered error accumulation or eviction. Add a test that
verifies unclustered errors don't grow without bound and are cleaned up.

### R8. Test gap: no test for analyze(target: "errors") MCP integration

Tests cover `GetAnalysisResponse()` directly but not the full MCP
round-trip through `toolAnalyzeErrors`. The existing test pattern
elsewhere in the codebase uses `httptest.NewRecorder` for this.

### R9. `inferRootCause` assumes stacks are deepest-first

**Spec section:** Root Cause Inference

The comment at line 401 says "first in the list, since stacks go from
deepest to shallowest." This is backwards for V8 stack traces, where the
top frame is the _throw site_ (deepest) and the bottom frame is the
entry point (shallowest). The code iterates top-down and returns the
first non-framework frame, which _is_ the throw site. The comment is
misleading but the behavior is correct.

Fix the comment, not the code.

### R10. Product philosophy alignment

Per `product-philosophy.md`, Gasoline should "capture, don't interpret"
and avoid making the AI "confidently wrong." The root cause inference
(`inferRootCause`) is interpretive -- it guesses which frame caused the
error. This is borderline.

The current implementation is acceptable because it uses a simple
heuristic (deepest common frame) that a developer can verify by
inspection. But the `summary` field in `GetAnalysisResponse` says
"Primary root cause: ..." which presents a hypothesis as a conclusion.
Consider rephrasing to "Most common throw site" or "Common frame" to
avoid overconfidence.

---

## Implementation Roadmap

Ordered by priority and dependency:

1. **Cap unclustered list** (C1). Add `maxUnclustered = 200` constant.
   Evict oldest on overflow in `AddError`. Add test. -- 30 min.

2. **Wire up Cleanup goroutine** (C2). Add a `time.Ticker` in
   `NewClusterManager`, a `Stop()` method, and call `Stop()` on server
   shutdown. Add unclustered expiry to `Cleanup` (R6). -- 1 hr.

3. **Cache parsed frames** (C3). Add `parsedFrames []StackFrame` and
   `normalizedMsg string` fields to `ErrorInstance` (or a wrapper).
   Populate on ingestion. Remove re-parsing in `countSignals`. -- 1 hr.

4. **Fix alert to support multiple pending** (R5). Change
   `pendingAlert *Alert` to `pendingAlerts []Alert`. Update
   `DrainAlert` to drain all. -- 30 min.

5. **Reconcile spec/implementation gaps** (R1, R2, R3, R4, R9). Update
   the spec document: drop `{string}`, clarify 1+ frame threshold, note
   instances are summary-only, drop "most informative", fix root cause
   wording. -- 30 min.

6. **Add missing tests** (R7, R8). Unclustered cap test, unclustered
   expiry test, MCP round-trip test. -- 1 hr.

7. **Soften interpretive language** (R10). Change "Primary root cause"
   to "Most common throw site" in summary text. -- 10 min.

Total estimated effort: ~5 hours.
