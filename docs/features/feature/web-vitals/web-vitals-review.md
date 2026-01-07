# Review: Web Vitals Capture (tech-spec-web-vitals.md)

## Executive Summary

This spec promotes FCP, LCP, CLS, and INP to first-class metrics with Google's threshold classifications (good/needs-improvement/poor). The implementation is already partially complete (`performance.go` has assessment functions and `InteractionToNextPaint` in the type definitions). INP is the only genuinely new metric; the others are already captured. The spec is well-aligned with Google's measurement methodology, but INP implementation has subtle correctness issues around interaction grouping and the extension-side observer pattern requires careful resource management.

## Critical Issues (Must Fix Before Implementation)

### 1. INP 98th Percentile Calculation Is Wrong for >50 Interactions

**Section:** "INP Implementation" -- step 4

The spec says: "Report the worst one (if <=50 interactions) or the 98th percentile (if >50)."

Google's actual INP methodology (documented in web.dev/articles/inp) is:
- If total interactions <= 50: report the **highest** interaction duration
- If total interactions > 50: **ignore the single highest** interaction and report the second-highest

This is NOT a 98th percentile. It is "second worst" -- equivalent to a P98 only when there are exactly 50 interactions. At 200 interactions, P98 would be the 4th-worst; Google's method still uses the 2nd-worst.

The distinction matters: a user who triggers 200 interactions has one very slow outlier (1500ms click on a complex modal) and a second-worst of 300ms. Google's method reports 300ms (good context for the page). A true P98 on 200 samples would report the 4th-worst, which might be 200ms -- making the page look better than it actually is for most users.

**Fix:** Match Google's methodology exactly: for >50 interactions, report the second-worst (highest minus one outlier), not a percentile calculation. The spec's "sorted list" approach works, just pick `sortedDurations[len-2]` instead of computing P98.

### 2. Only Storing Top 50 Interactions by Duration Loses Accuracy

**Section:** "Edge Cases" -- "Very many interactions (>200)"

The spec says: "Only the top 50 by duration are stored."

INP needs to know the *total interaction count* to decide which calculation to use (worst vs. second-worst). If you only store the top 50, you know the top 50 durations, but you have lost the count of sub-threshold interactions. The spec stores `interaction_count` separately, so the threshold check works, but the selection logic is incomplete.

More critically, if a developer triggers 500 interactions, 450 of which are under 100ms and 50 are between 100-300ms, the stored set is the 50 slowest. The "second worst" from this set is the actual second-worst overall -- so the result is correct. But if you later want percentile-based analysis (e.g., P75 for INP trends), you cannot compute it from 50 samples out of 500.

**Fix:** Store the full interaction count AND only the top 50 by duration (already specified). Document that sub-P98 percentiles are unavailable when interaction count exceeds storage. Consider increasing storage to 200 entries (each entry is ~100 bytes; 200 entries = 20KB, well within budget) to improve percentile accuracy.

### 3. LCP Finalization Timing Creates a Race Condition

**Section:** "Extension Changes" -- inject.js

The spec says: "Register a listener for `visibilitychange` and first user input to stop updating LCP."

**Problem:** LCP is reported by a `PerformanceObserver` that fires asynchronously. The observer callback and the `visibilitychange`/input event handlers are both async tasks scheduled by the browser. There is no guaranteed ordering. Consider:

1. User clicks at T=1000ms
2. LCP observer fires at T=1001ms with a new largest element (rendered just before the click)
3. The input handler fires at T=1002ms and "finalizes" LCP

The LCP value is the entry from step 2, which is correct. But if the ordering were reversed (input handler at T=1000ms, LCP entry at T=1001ms), the handler finalizes before the last valid LCP entry arrives. The missed entry causes under-reporting.

Google's `web-vitals` library handles this by using `PerformanceObserver.takeRecords()` in the finalization handler to synchronously flush pending entries before finalizing.

**Fix:** In the finalization handler (on `visibilitychange` or first input), call `observer.takeRecords()` and process any pending entries before marking LCP as final. This is a one-line addition but is critical for correctness.

### 4. INP Attribution Data May Not Be Available in All Browsers

**Section:** "INP Attribution"

The spec lists attribution data: `worst_target` (CSS selector), `worst_type`, `worst_processing_ms`, `worst_delay_ms`, `worst_presentation_ms`.

The `PerformanceEventTiming` entry provides `duration`, `processingStart`, `processingEnd`, `startTime`, and `interactionId`. The processing/delay/presentation breakdown can be computed from these. However, `target` (the DOM element) is only available on entries with `duration >= 104ms` in Chrome (entries below this threshold have `null` target for performance reasons).

If the worst interaction is 150ms, `target` is available. If it is 80ms but "good" threshold is 200ms, attribution is not needed. But for "needs-improvement" range (200-500ms), `target` should always be available since `duration >= 104ms`. This is fine for the primary use case.

**Problem:** The CSS selector generation from a DOM element (`event.target`) requires traversing the DOM at callback time. If the element has been removed from the DOM between the interaction and the observer callback (e.g., a modal that closes on click), the selector will be unavailable or wrong.

**Fix:** Generate the CSS selector at interaction time (in the event handler), not in the PerformanceObserver callback. Store a map of `interactionId -> selector` populated by an event listener on `click`, `keydown`, and `pointerdown`. When the observer fires, look up the selector by `interactionId`.

### 5. No Mechanism to Reset INP Between Page Loads in SPA

**Section:** "Edge Cases" -- "SPA navigations"

The spec says: "Soft navigations (pushState) don't reset vitals. FCP/LCP are only valid for the initial hard navigation. CLS and INP accumulate across the page lifetime."

This means a long-lived SPA session (developer working for 2 hours) accumulates hundreds of interactions. The INP reflects the worst interaction across the entire session, not the worst interaction on the current route. This is correct per Google's methodology (INP is a page-lifecycle metric), but it is unhelpful for development.

If the developer fixed a slow handler on `/dashboard` 30 minutes ago, INP still reflects the old slow interaction because it accumulated before the fix. The developer cannot see whether the fix worked without a full page reload.

**Fix:** Add a `reset_vitals` MCP tool (or parameter on `get_web_vitals`) that clears the accumulated INP data and starts fresh. This deviates from Google's methodology but is essential for development-time feedback loops. Document the deviation clearly: "This resets INP for development purposes. Google's INP metric is per-page-lifecycle and cannot be reset in production."

Alternatively, if the SPA Route Measurement spec is co-implemented, track per-route INP (interactions that occurred while a route was active). This provides route-level interaction performance without resetting the page-level INP.

## Recommendations (Should Consider)

### 1. `get_web_vitals` Should Return History Without a Separate Parameter

The spec has `include_history` as an optional boolean parameter. In practice, the AI will almost always want history (for trend detection). Make history the default response (last 10 entries) and add `latest_only: true` for the single-snapshot use case. This reduces tool call count for the common case.

### 2. The `summary` Field Should Be Data, Not Prose

**Section:** "MCP Tool: `get_web_vitals`"

The response includes:
```
"summary": "All Core Web Vitals pass. INP is close to threshold (180ms / 200ms good limit)."
```

This is interpretation, which the product philosophy discourages. The AI writes better summaries than we can. Replace with a structured `alerts` array:

```json
{
  "alerts": [
    { "metric": "inp", "alert": "near_threshold", "value_ms": 180, "threshold_ms": 200, "headroom_pct": 10 }
  ]
}
```

The AI can synthesize this into any summary it needs.

### 3. Assessment Functions Should Handle Edge Values Consistently

**Section:** Implementation: `performance.go` lines 192-236

The thresholds use `<` for "good" and `>=` for "poor":
```go
func assessFCP(value float64) string {
    if value < 1800 { return "good" }
    if value >= 3000 { return "poor" }
    return "needs-improvement"
}
```

This means `value == 1800` is "needs-improvement" (correct per Google's documentation: "good" is *less than* 1.8s). The implementation is correct, but add a comment referencing Google's thresholds document (web.dev/articles/vitals) to prevent future "optimization" of the boundary conditions.

### 4. INP Should Track Event Handler Source When Possible

For INP attribution, knowing which event handler is slow is more actionable than knowing which element was clicked. If the `PerformanceEventTiming` entry includes script attribution (currently behind the `PeformanceLongTaskTiming` attribution API, experimental), include the script URL and function name. This turns "button.submit-form was slow" into "handleSubmit() in checkout.js:142 was slow."

This is a stretch goal since the attribution API is experimental, but it is worth monitoring.

### 5. The 80/20 Weighted Average for Baselines Loses Early Data Too Fast

**Section:** Not directly in this spec, but in `performance.go` lines 130-143

The existing baseline computation uses 80% existing + 20% new after 5 samples. For web vitals specifically, this means a single outlier FCP (e.g., 5000ms due to cold cache) contributes 20% to the baseline, pulling it significantly higher. The next normal load (800ms) only pulls it back 20%. After 5 loads post-outlier, the baseline still reflects 33% of the outlier value.

**Fix:** For web vitals metrics specifically, use median rather than weighted average. Vitals are noisy (background tabs, cold cache, GC pauses), and median is more robust to outliers. Store the last 10 values per metric and compute the running median.

## Implementation Roadmap

1. **INP observer in inject.js** (1 day): Implement `PerformanceObserver` for `event` type with `durationThreshold: 16`. Group by `interactionId`. Store top 200 by duration. Track CSS selector via interaction event listeners. Use Google's "second-worst" algorithm, not P98.

2. **LCP finalization fix** (0.5 days): Add `observer.takeRecords()` call in `visibilitychange` and first-input handlers before finalizing LCP value. Mark as "estimated" if page hidden before any interaction.

3. **Server-side vitals assessment** (0.5 days): Already implemented in `performance.go`. Add INP regression threshold (>50ms increase). Replace `summary` string with structured `alerts` array. Add reference comments for Google threshold values.

4. **Vitals history storage** (0.5 days): Store last 10 vitals entries per URL path in `PerformanceStore`. Return by default in `get_web_vitals` response.

5. **INP reset mechanism** (0.5 days): Add `reset_vitals` tool or parameter. Document deviation from Google methodology.

6. **Per-route INP** (deferred): Coordinate with SPA Route Measurement spec. Track interactions per active route.

Total: ~3 days of implementation work. Most of the infrastructure exists; the new work is INP capture and correctness fixes.
