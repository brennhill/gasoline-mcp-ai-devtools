# Performance Budget Monitor - Technical Specification

## Overview

`check_performance` is a new MCP tool that gives AI coding assistants a performance snapshot of the current page, including load timing, network weight, and main-thread blocking. When baseline data exists, it highlights regressions so the AI can identify what changed and fix it proactively.

**Key insight:** AI-assisted development can introduce performance regressions that go unnoticed because developers interact with the app through the AI, not the browser. This tool surfaces those regressions at the point where they can be fixed — during the coding session.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                         Browser (inject.js)                        │
│                                                                    │
│  Navigation Timing ────┐  (page load phases via performance API)   │
│  PerformanceObserver ──┤  (long tasks, LCP, CLS, FCP)              │
│  Resource Timing ──────┤  (already captured via waterfall)          │
│  Pending Requests ─────┘  (already tracked)                        │
│                                                                    │
│  On navigation: collect snapshot, postMessage to content script     │
└───────────────────┬──────────────────────────────────────────────┘
                    │ postMessage
┌───────────────────▼──────────────────────────────────────────────┐
│              Content Script → Background → Server                  │
│              POST /performance-snapshot                             │
└───────────────────┬──────────────────────────────────────────────┘
                    │ HTTP
┌───────────────────▼──────────────────────────────────────────────┐
│                    Gasoline Server (Go)                             │
│                                                                    │
│  POST /performance-snapshot  - Receives page load data             │
│  GET  /performance-snapshot  - Returns latest + baseline diff      │
│                                                                    │
│  MCP tool: check_performance                                       │
│    - Returns current snapshot                                      │
│    - Compares against stored baseline                              │
│    - Highlights regressions                                        │
└──────────────────────────────────────────────────────────────────┘
```

---

## Data Model

### PerformanceSnapshot

Collected once per page navigation, after the `load` event fires.

```typescript
interface PerformanceSnapshot {
  // Identification
  url: string              // Page URL (path only, no query/hash for grouping)
  timestamp: string        // ISO 8601

  // Navigation Timing (milliseconds, relative to navigationStart)
  timing: {
    domContentLoaded: number  // DOMContentLoaded event
    load: number              // load event
    firstContentfulPaint: number | null  // FCP (via PerformanceObserver)
    largestContentfulPaint: number | null // LCP (via PerformanceObserver)
    timeToFirstByte: number   // responseStart - requestStart
    domInteractive: number    // DOM ready for interaction
  }

  // Network Summary (from Resource Timing entries)
  network: {
    requestCount: number       // Total requests during page load
    transferSize: number       // Total bytes transferred (compressed)
    decodedSize: number        // Total bytes decoded (uncompressed)
    byType: {
      script: { count: number, size: number }
      style: { count: number, size: number }
      image: { count: number, size: number }
      font: { count: number, size: number }
      fetch: { count: number, size: number }
      other: { count: number, size: number }
    }
    slowestRequests: Array<{   // Top 3 by duration
      url: string              // Truncated to 80 chars
      duration: number
      size: number
    }>
  }

  // Main Thread Health
  longTasks: {
    count: number              // Tasks > 50ms
    totalBlockingTime: number  // Sum of (task.duration - 50ms) for each long task
    longest: number            // Duration of longest task (ms)
  }

  // Layout Stability
  cumulativeLayoutShift: number | null  // CLS score (via PerformanceObserver)
}
```

### PerformanceBaseline

Stored on the server, one per URL path. Updated as a rolling average.

```typescript
interface PerformanceBaseline {
  url: string
  sampleCount: number      // How many snapshots contributed
  lastUpdated: string      // ISO 8601

  // Averaged metrics
  timing: {
    domContentLoaded: number
    load: number
    firstContentfulPaint: number | null
    largestContentfulPaint: number | null
    timeToFirstByte: number
  }

  network: {
    requestCount: number
    transferSize: number
  }

  longTasks: {
    count: number
    totalBlockingTime: number
  }
}
```

---

## inject.js Changes

### New: `capturePerformanceSnapshot()`

Runs after the `load` event + 2000ms delay (to allow LCP and late resources to settle).

```javascript
function capturePerformanceSnapshot() {
  const nav = performance.getEntriesByType('navigation')[0]
  if (!nav) return

  const snapshot = {
    url: location.pathname,
    timestamp: new Date().toISOString(),
    timing: {
      domContentLoaded: nav.domContentLoadedEventEnd,
      load: nav.loadEventEnd,
      firstContentfulPaint: getFCP(),
      largestContentfulPaint: getLCP(),
      timeToFirstByte: nav.responseStart - nav.requestStart,
      domInteractive: nav.domInteractive,
    },
    network: aggregateResourceTiming(),
    longTasks: getLongTaskMetrics(),
    cumulativeLayoutShift: getCLS(),
  }

  window.postMessage({
    type: 'DEV_CONSOLE_PERFORMANCE_SNAPSHOT',
    payload: snapshot,
  }, '*')
}
```

### New: Long Task Observer

Installed on page load. Collects all long tasks (> 50ms) until snapshot is taken.

```javascript
let longTasks = []

const longTaskObserver = new PerformanceObserver((list) => {
  for (const entry of list.getEntries()) {
    longTasks.push({
      duration: entry.duration,
      startTime: entry.startTime,
    })
  }
})
longTaskObserver.observe({ type: 'longtask', buffered: true })
```

### New: Web Vitals Observers

FCP, LCP, and CLS are captured via PerformanceObserver (already partially exists for performance marks):

```javascript
let fcpValue = null
let lcpValue = null
let clsValue = 0

new PerformanceObserver((list) => {
  for (const entry of list.getEntries()) {
    if (entry.name === 'first-contentful-paint') fcpValue = entry.startTime
  }
}).observe({ type: 'paint', buffered: true })

new PerformanceObserver((list) => {
  for (const entry of list.getEntries()) {
    lcpValue = entry.startTime  // Last one wins
  }
}).observe({ type: 'largest-contentful-paint', buffered: true })

new PerformanceObserver((list) => {
  for (const entry of list.getEntries()) {
    if (!entry.hadRecentInput) {
      clsValue += entry.value
    }
  }
}).observe({ type: 'layout-shift', buffered: true })
```

### `aggregateResourceTiming()`

Summarizes all resource entries loaded during the page navigation:

```javascript
function aggregateResourceTiming() {
  const resources = performance.getEntriesByType('resource')
  const byType = { script: {count:0,size:0}, style: {count:0,size:0}, ... }

  let totalTransfer = 0, totalDecoded = 0
  const slowest = []

  for (const r of resources) {
    totalTransfer += r.transferSize || 0
    totalDecoded += r.decodedBodySize || 0
    const type = mapInitiatorType(r.initiatorType)
    byType[type].count++
    byType[type].size += r.transferSize || 0
    slowest.push({ url: r.name.slice(0, 80), duration: r.duration, size: r.transferSize })
  }

  slowest.sort((a, b) => b.duration - a.duration)

  return {
    requestCount: resources.length,
    transferSize: totalTransfer,
    decodedSize: totalDecoded,
    byType,
    slowestRequests: slowest.slice(0, 3),
  }
}
```

### Feature Toggle

Performance snapshot capture is **enabled by default** (unlike performance marks/waterfall which are opt-in). It has negligible overhead: one snapshot per navigation, collected passively.

The `DEV_CONSOLE_SETTING` message can disable it:
```javascript
{ type: 'DEV_CONSOLE_SETTING', key: 'performanceSnapshotEnabled', value: false }
```

---

## Server Changes (v4.go)

### New Types

```go
type PerformanceSnapshot struct {
  URL       string                `json:"url"`
  Timestamp string                `json:"timestamp"`
  Timing    PerformanceTiming     `json:"timing"`
  Network   NetworkSummary        `json:"network"`
  LongTasks LongTaskMetrics       `json:"longTasks"`
  CLS       *float64              `json:"cumulativeLayoutShift"`
}

type PerformanceTiming struct {
  DOMContentLoaded       float64  `json:"domContentLoaded"`
  Load                   float64  `json:"load"`
  FirstContentfulPaint   *float64 `json:"firstContentfulPaint"`
  LargestContentfulPaint *float64 `json:"largestContentfulPaint"`
  TimeToFirstByte        float64  `json:"timeToFirstByte"`
  DOMInteractive         float64  `json:"domInteractive"`
}

type NetworkSummary struct {
  RequestCount    int                       `json:"requestCount"`
  TransferSize    int64                     `json:"transferSize"`
  DecodedSize     int64                     `json:"decodedSize"`
  ByType          map[string]TypeSummary    `json:"byType"`
  SlowestRequests []SlowRequest             `json:"slowestRequests"`
}

type LongTaskMetrics struct {
  Count             int     `json:"count"`
  TotalBlockingTime float64 `json:"totalBlockingTime"`
  Longest           float64 `json:"longest"`
}

type PerformanceBaseline struct {
  URL         string            `json:"url"`
  SampleCount int               `json:"sampleCount"`
  LastUpdated string            `json:"lastUpdated"`
  Timing      PerformanceTiming `json:"timing"`
  Network     BaselineNetwork   `json:"network"`
  LongTasks   LongTaskMetrics   `json:"longTasks"`
}
```

### Storage

```go
type V4Server struct {
  // ... existing fields ...
  perfSnapshots map[string]PerformanceSnapshot  // keyed by URL path, last per page
  perfBaselines map[string]PerformanceBaseline  // keyed by URL path, rolling average
}
```

- **Snapshots:** Latest snapshot per URL path (max 20 URLs). Overwritten on each navigation.
- **Baselines:** Rolling weighted average. After 5 samples, new data is weighted 20% vs 80% existing. Max 20 baselines.

### HTTP Endpoints

#### `POST /performance-snapshot`

Receives a snapshot from the extension. Updates both the latest snapshot and the baseline for that URL.

```
Request body: PerformanceSnapshot JSON
Response: 200 { "received": true, "baseline_updated": true }
```

**Baseline update logic:**
```go
func (v *V4Server) updateBaseline(snapshot PerformanceSnapshot) {
  existing, found := v.perfBaselines[snapshot.URL]
  if !found || existing.SampleCount < 5 {
    // Simple average for first 5 samples
    existing = mergeSimpleAverage(existing, snapshot)
  } else {
    // Weighted: 80% existing + 20% new
    existing = mergeWeighted(existing, snapshot, 0.2)
  }
  existing.SampleCount++
  existing.LastUpdated = snapshot.Timestamp
  v.perfBaselines[snapshot.URL] = existing
}
```

#### `GET /performance-snapshot`

Returns the latest snapshot for the current page (or all pages).

Query params:
- `url` (optional) — filter by URL path

---

## MCP Tool: `check_performance`

### Tool Definition

```json
{
  "name": "check_performance",
  "description": "Get current page performance metrics including load timing, network weight, and main-thread blocking. Compares against baseline to highlight regressions.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "url": {
        "type": "string",
        "description": "Filter by URL path (optional, defaults to most recent page)"
      }
    }
  }
}
```

### Response Format

The tool returns a human-readable performance report with regression indicators:

```
Page: /dashboard
Captured: 2s ago

── Load Timing ──────────────────────────────────────
  Time to First Byte:    120ms
  First Contentful Paint: 450ms
  Largest Contentful Paint: 1,200ms (⚠️ was 380ms, +216%)
  DOM Content Loaded:    800ms (⚠️ was 320ms, +150%)
  Load Event:            3,200ms (⚠️ was 1,100ms, +191%)

── Network ──────────────────────────────────────────
  Requests: 47 (⚠️ was 12, +292%)
  Transfer: 2.4MB (⚠️ was 340KB, +606%)
  Breakdown:
    Scripts: 31 files, 1.8MB
    Styles:  4 files, 120KB
    Images:  8 files, 420KB
    Fetch:   3 requests, 45KB
  Slowest:
    /api/dashboard/analytics — 1,800ms, 890KB
    /static/vendor.js — 650ms, 1.2MB
    /api/users?include=all — 420ms, 340KB

── Main Thread ──────────────────────────────────────
  Long Tasks: 4 (⚠️ was 0)
  Total Blocking Time: 380ms (⚠️ was 0ms)
  Longest Task: 210ms

── Layout Stability ─────────────────────────────────
  CLS: 0.12 (⚠️ was 0.02)

── Regressions ──────────────────────────────────────
  ⚠️ Load time increased 191% (1.1s → 3.2s)
  ⚠️ Request count increased 292% (12 → 47)
  ⚠️ 4 new long tasks blocking main thread
  Baseline: 8 samples over this session
```

### Regression Detection

A metric is flagged as a regression when:

| Metric | Threshold |
|--------|-----------|
| Load / FCP / LCP | > 50% increase AND > 200ms absolute increase |
| Request count | > 50% increase AND > 5 absolute increase |
| Transfer size | > 100% increase AND > 100KB absolute increase |
| Long task count | Any increase from 0, or > 100% increase |
| Total blocking time | > 100ms absolute increase |
| CLS | > 0.05 absolute increase |

Both relative AND absolute thresholds must be met to avoid false positives on small pages.

### When No Baseline Exists

If no baseline is stored (first navigation to that path), the tool returns the snapshot without comparison:

```
Page: /dashboard (no baseline yet — will compare on next load)

── Load Timing ──────────────────────────────────────
  Time to First Byte:    120ms
  First Contentful Paint: 450ms
  ...
```

---

## Performance Budgets (Extension SLOs)

The performance capture itself must not impact the page:

| Operation | Budget | Implementation |
|-----------|--------|----------------|
| PerformanceObserver callbacks | < 0.1ms each | Only append to array |
| Snapshot aggregation | < 5ms | Runs 2s after load, off main path |
| Resource timing iteration | < 2ms | Already available via browser API |
| Message to content script | < 0.1ms | postMessage, async |
| Snapshot JSON size | < 4KB | Truncate URLs, limit slowest to 3 |

### Memory

- Long task array: max 50 entries per page (older evicted)
- Snapshots on server: max 20 URLs (LRU eviction)
- Baselines on server: max 20 URLs (LRU eviction)
- Total memory budget: < 200KB on server

---

## Message Flow

```
[Page load event + 2s]
         │
inject.js: capturePerformanceSnapshot()
         │
         ▼ postMessage
content.js: forward to background
         │
         ▼ chrome.runtime.sendMessage
background.js: POST /performance-snapshot
         │
         ▼ HTTP
server: store snapshot, update baseline
         │
         ▼ (later, when AI invokes tool)
MCP: check_performance → format + compare → return report
```

---

## Edge Cases

1. **SPA navigation:** Only captures initial page load. SPA route changes don't trigger Navigation Timing entries. Future enhancement: detect `history.pushState` and recollect Resource Timing.

2. **Service Worker caching:** Resources served from SW cache have `transferSize: 0`. These are still counted in request count but not in transfer size.

3. **Cross-origin resources:** Resource Timing may lack size data due to CORS (`Timing-Allow-Origin` header). These show `size: 0` but are still counted.

4. **Very fast pages:** If `loadEventEnd` is < 100ms, the snapshot may capture before LCP is determined. The 2s delay after load mitigates this.

5. **Tabs with no navigation:** If the user opens a blank tab or `about:` page, no snapshot is generated.

6. **Multiple navigations:** Each navigation to the same URL overwrites the previous snapshot but contributes to the baseline average.

---

## Testing Strategy

### Unit Tests (extension-tests/performance-snapshot.test.js)

- `capturePerformanceSnapshot()` collects correct fields from Performance API mocks
- `aggregateResourceTiming()` groups by initiator type correctly
- Long task observer accumulates entries
- FCP/LCP/CLS observers capture values
- Feature toggle enables/disables capture
- Message format is correct

### Unit Tests (Go: v4_test.go)

- `POST /performance-snapshot` stores snapshot
- `POST /performance-snapshot` updates baseline (simple average < 5 samples)
- `POST /performance-snapshot` updates baseline (weighted average >= 5 samples)
- Baseline comparison detects regressions per threshold table
- MCP `check_performance` returns formatted report
- MCP `check_performance` with no baseline returns snapshot only
- LRU eviction at 20 URLs
- Concurrent access safety

### E2E Tests (e2e-tests/performance-budget.spec.js)

- Page load generates a performance snapshot on the server
- Second navigation to same URL creates a baseline
- Regression detection works when page loads more resources
- `check_performance` MCP tool returns expected format

---

## Implementation Order (TDD)

1. **Go types + test stubs** — Define types, write failing tests for storage/comparison
2. **Go endpoint + baseline logic** — Implement POST handler, baseline averaging, regression detection
3. **Go MCP tool** — Implement `check_performance` tool with formatted output
4. **inject.js observers** — Add PerformanceObserver for longtask, LCP, CLS, FCP
5. **inject.js snapshot** — Implement `capturePerformanceSnapshot()` + aggregation
6. **Extension plumbing** — Wire content.js + background.js for new message type
7. **E2E tests** — Full pipeline test
