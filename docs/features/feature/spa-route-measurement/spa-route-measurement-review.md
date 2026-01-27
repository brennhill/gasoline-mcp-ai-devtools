# Review: SPA Route Measurement (tech-spec-spa-route-measurement.md)

## Executive Summary

This spec adds per-route performance measurement for single-page applications by intercepting `history.pushState`, `replaceState`, and `popstate`, then measuring time-to-interactive via a quiescence heuristic (network + render + main thread idle). The feature fills a genuine gap -- Gasoline currently sees only initial page loads -- but the quiescence detection algorithm has several conditions that will produce unreliable TTI values in real-world apps, and the route normalization logic is brittle.

## Critical Issues (Must Fix Before Implementation)

### 1. Quiescence Detection Will Produce Unreliable TTI for Real-World SPAs

**Section:** "Time-to-Interactive Measurement"

The algorithm waits for three simultaneous conditions:
- No new fetch requests for 500ms
- No layout shifts or DOM mutations for 200ms
- No long tasks (>50ms) for 200ms

**Problem 1: Polling and background requests.** Many SPAs have polling intervals (60s health checks, 30s notification polls, real-time dashboards with 5s refresh). These fetch requests will reset the 500ms network quiescence timer. The spec acknowledges WebSocket as an issue ("A more lenient mode ignores ongoing WebSocket activity") but not `fetch`-based polling. A dashboard that polls `/api/notifications` every 5 seconds will never reach network quiescence.

**Problem 2: Animations.** CSS animations and `requestAnimationFrame` loops cause continuous DOM mutations and layout computations. A page with a loading spinner (DOM mutation every 16ms) followed by a fade-in transition will reset the 200ms render quiescence timer repeatedly. The TTI will measure "time until animation completes" rather than "time until route is usable."

**Problem 3: Intersection of conditions is too strict.** The three conditions must be met *simultaneously*. In practice, a route might finish network requests at T+800ms, finish rendering at T+1200ms, but then a lazy-loaded component fires a new fetch at T+1300ms (triggered by an IntersectionObserver). The network timer resets. Meanwhile, the route has been visually complete and interactive since T+1200ms.

**Fix:** Replace the simultaneous-quiescence model with a staged model:
1. Network quiescence: No new fetch requests for 500ms, EXCLUDING requests matching known polling patterns (same URL repeating at regular intervals).
2. Render quiescence: No *structural* DOM mutations for 200ms (ignore text-only changes, attribute-only changes that match animation patterns like `transform`, `opacity`).
3. Main thread quiescence: No long tasks for 200ms.

TTI = max(network_quiesce_time, render_quiesce_time, main_thread_quiesce_time) rather than "first moment all three are simultaneously quiet." This is how the Chrome team's soft navigation heuristics work internally.

Additionally, add a "visually complete" heuristic: LCP element rendered + no further layout shifts. This provides a more useful metric than strict quiescence for routes where background activity never fully stops.

### 2. Route Normalization Heuristics Are Too Aggressive

**Section:** "Route Normalization"

The spec says:
- Purely numeric segments replaced with `:id`
- UUIDs replaced with `:uuid`
- Segments that "vary between observations" replaced with `:param`

**Problem:** The third rule -- "segments that vary between observations of the same route prefix" -- is algorithmically ambiguous and dangerous. Consider:

- `/api/v2/items/456/comments` normalizes to `/api/v2/items/:id/comments` (correct)
- `/dashboard/settings` and `/dashboard/billing` -- these are two distinct routes that share a prefix. If observed sequentially, the algorithm might normalize to `/dashboard/:param`, collapsing two completely different pages into one metric.

The spec provides no algorithm for distinguishing "dynamic segment" from "different route." This requires either framework integration (reading the router config) or a statistically significant sample (10+ observations of the same prefix with varying segments).

**Fix:** Start conservative: only normalize segments that are purely numeric (>= 1 digit, no letters) or match UUID format (`[0-9a-f]{8}-...`). Do NOT auto-detect varying segments in v1. Add a `route_patterns` configuration option (e.g., in `.gasoline.json`) where developers can explicitly declare patterns:

```json
{
  "route_patterns": [
    "/user/:id",
    "/post/:slug",
    "/api/v2/items/:id/comments"
  ]
}
```

This is more reliable than heuristics and aligns with the product philosophy ("Capture, don't interpret").

### 3. MutationObserver at 100ms Intervals May Miss Fast Transitions

**Section:** "Performance Constraints"

The spec says MutationObserver is sampled at 100ms intervals. This is a misunderstanding of MutationObserver. MutationObservers fire synchronously after DOM mutations, not on a polling interval. If the intent is to *batch* mutations every 100ms (i.e., count mutations accumulated in each 100ms window), this needs to be stated clearly.

If mutations are only checked every 100ms, a route that renders in 50ms will have its entire render cycle between two sample points, making the DOM mutation count unreliable (could be 0 or 1 depending on timing alignment).

**Fix:** Use MutationObserver in its normal callback mode (fires after every batch of mutations). Debounce the "last mutation" timestamp -- each callback updates it. The 200ms quiescence check then compares `now - lastMutationTimestamp > 200`. This is both more accurate and lower overhead than polling, since the observer fires only when mutations occur.

### 4. Query-Param-Only Changes Are Not Always Non-Navigations

**Section:** "Edge Cases"

The spec says: "replaceState calls that only change query params (`?page=2`) are ignored as route transitions unless the path also changes."

This is wrong for many applications. Search pages (`/search?q=react`), filter pages (`/products?category=shoes&sort=price`), and paginated lists (`/users?page=3`) treat query param changes as meaningful navigations that trigger data fetching and re-rendering. Ignoring these means Gasoline misses the most common pagination performance pattern.

**Fix:** Track query-param-only changes as route transitions but normalize the metric key to the path only (without query params). This way, `/search?q=react` and `/search?q=vue` both contribute to the `/search` route baseline, and the data fetching triggered by query changes is captured.

### 5. No Handling of Next.js App Router / Server Components

**Section:** General omission

The spec mentions "Next.js prefetch" briefly but does not address the Next.js App Router (the default since Next.js 13.4), which uses React Server Components. In this model:
- Route transitions fetch RSC payloads (not JSON APIs)
- The browser receives a React flight stream, not a full HTML document
- `pushState` fires, but the rendering happens via React's concurrent pipeline

The quiescence heuristic needs to handle RSC flight stream responses (content-type `text/x-component`) as network activity associated with the navigation.

**Fix:** Add RSC flight streams (`text/x-component`, `text/plain` with `__next` prefix) to the list of navigation-associated network requests. Consider detecting Next.js apps specifically (presence of `__next` data in page) and adjusting quiescence parameters.

## Recommendations (Should Consider)

### 1. Use the Soft Navigation API When Available

**Section:** "Opportunity & Business Value" mentions this but the spec does not integrate it.

Chrome's Soft Navigation API (available behind a flag, expected to ship in 2025-2026) provides browser-native SPA navigation detection and metrics. When available (`PerformanceObserver.supportedEntryTypes.includes('soft-navigation')`), Gasoline should use it instead of the pushState interception. This gives browser-native TTI without the quiescence heuristic. The custom implementation becomes the fallback.

### 2. Store Transition Pairs, Not Just Route Metrics

The data model stores per-route metrics but the source route is only kept as `last_transition_from`. For performance analysis, the *transition pair* matters: `/dashboard` -> `/settings` may be fast, but `/dashboard` -> `/analytics` may be slow (different data requirements). Store metrics keyed by `source|destination` pairs and aggregate to per-destination when needed.

### 3. Coordinate with Web Vitals Spec on INP During Route Transitions

The web vitals spec captures INP for the entire page lifetime. When an SPA route transition occurs, interactions during the transition should be attributed to the transition, not the overall page. This requires coordination between the two specs -- the SPA measurement should mark a window during which INP events are "transition-associated."

### 4. Performance Budget for MutationObserver Callback

The spec budgets "under 0.1ms per sample" for MutationObserver. In practice, the MutationObserver callback receives a `MutationRecord[]` that can contain hundreds of records for a single React render. Counting nodes from these records is O(n) in the number of mutations. For a complex component tree, this can exceed 1ms.

Cap the node-counting at 100 records per callback. Beyond that, increment a "large mutation batch" counter instead of counting individual nodes.

## Implementation Roadmap

1. **pushState/popstate/hashchange interception** (0.5 days): Wrap APIs in inject.js. Record transition start. Handle concurrent navigations (cancel previous).

2. **Quiescence detection** (2 days): Implement staged quiescence model with polling exclusion. Use MutationObserver callback (not polling). Add 10s timeout. Handle WebSocket exclusion.

3. **Route normalization** (1 day): Implement numeric-only and UUID normalization. Add `route_patterns` config support. Skip heuristic "varying segment" detection in v1.

4. **Per-route data collection** (1 day): Capture fetch requests, CLS, long tasks, DOM mutation count during transition window. Aggregate into RouteTransition entries.

5. **Server-side storage and MCP tool** (1 day): Implement route metrics storage (50 routes x 10 history). Add `get_route_metrics` tool. Add baseline computation after 3+ observations.

6. **Soft Navigation API integration** (0.5 days): Feature-detect and use when available. Fall back to custom implementation.

Total: ~6 days of implementation work.
