# Performance Capture: Web Vitals (FCP/LCP/CLS)

## Status: Implementation Ready

---

## Overview

The extension already has `PerformanceObserver` infrastructure for marks and measures. Core Web Vitals (FCP, LCP, CLS) use the same browser API but are not currently captured or sent to the server.

This spec adds Web Vitals capture to the extension and a corresponding MCP tool to retrieve them.

**Philosophy check:** Pure capture of browser-provided metrics. No interpretation (no "good"/"bad" labels), no thresholds, no scoring. Just "here are the numbers the browser reported."

---

## What Are Web Vitals?

| Metric | Full Name | What It Measures | API |
|--------|-----------|-----------------|-----|
| FCP | First Contentful Paint | Time to first text/image render | `PerformanceObserver` type `paint` |
| LCP | Largest Contentful Paint | Time to largest visible element | `PerformanceObserver` type `largest-contentful-paint` |
| CLS | Cumulative Layout Shift | Visual stability (unexpected shifts) | `PerformanceObserver` type `layout-shift` |
| FID | First Input Delay | Responsiveness to first interaction | `PerformanceObserver` type `first-input` |
| INP | Interaction to Next Paint | Responsiveness across all interactions | `PerformanceObserver` type `event` |
| TTFB | Time to First Byte | Server response time | `PerformanceNavigationTiming` |

**Scope for this spec:** FCP, LCP, CLS, TTFB. FID and INP are deferred (more complex, less commonly needed during development).

---

## Extension Changes

### New: Web Vitals Observer

Add to `inject.js` — a dedicated observer for Web Vitals that runs alongside the existing performance marks observer:

```javascript
// Web Vitals state
let webVitals = {
  fcp: null,   // { value: ms, timestamp: ISO }
  lcp: null,   // { value: ms, timestamp: ISO, element: tagName }
  cls: null,   // { value: score, timestamp: ISO, shifts: count }
  ttfb: null,  // { value: ms, timestamp: ISO }
}
let webVitalsObservers = []

export function startWebVitalsCapture() {
  // FCP
  try {
    const fcpObserver = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (entry.name === 'first-contentful-paint') {
          webVitals.fcp = {
            value: Math.round(entry.startTime),
            timestamp: new Date().toISOString()
          }
        }
      }
    })
    fcpObserver.observe({ type: 'paint', buffered: true })
    webVitalsObservers.push(fcpObserver)
  } catch (e) { /* unsupported */ }

  // LCP
  try {
    const lcpObserver = new PerformanceObserver((list) => {
      const entries = list.getEntries()
      const last = entries[entries.length - 1]
      if (last) {
        webVitals.lcp = {
          value: Math.round(last.startTime),
          timestamp: new Date().toISOString(),
          element: last.element?.tagName || null,
          size: last.size || null
        }
      }
    })
    lcpObserver.observe({ type: 'largest-contentful-paint', buffered: true })
    webVitalsObservers.push(lcpObserver)
  } catch (e) { /* unsupported */ }

  // CLS
  try {
    let clsValue = 0
    let clsShifts = 0
    const clsObserver = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (!entry.hadRecentInput) {
          clsValue += entry.value
          clsShifts++
        }
      }
      webVitals.cls = {
        value: parseFloat(clsValue.toFixed(4)),
        timestamp: new Date().toISOString(),
        shifts: clsShifts
      }
    })
    clsObserver.observe({ type: 'layout-shift', buffered: true })
    webVitalsObservers.push(clsObserver)
  } catch (e) { /* unsupported */ }

  // TTFB (from Navigation Timing)
  try {
    const navEntries = performance.getEntriesByType('navigation')
    if (navEntries.length > 0) {
      const nav = navEntries[0]
      webVitals.ttfb = {
        value: Math.round(nav.responseStart - nav.requestStart),
        timestamp: new Date().toISOString()
      }
    }
  } catch (e) { /* unsupported */ }
}

export function stopWebVitalsCapture() {
  webVitalsObservers.forEach(obs => obs.disconnect())
  webVitalsObservers = []
}

export function getWebVitals() {
  return { ...webVitals }
}

export function resetWebVitals() {
  webVitals = { fcp: null, lcp: null, cls: null, ttfb: null }
}
```

### Sending to Server

Web Vitals are included in the performance snapshot message that the background script already sends. Modify the snapshot to include vitals:

```javascript
// In background.js, when sending performance snapshot:
const snapshot = {
  type: 'web_vitals',
  vitals: webVitals,
  url: currentURL,
  timestamp: new Date().toISOString()
}
```

The background script sends this via `POST /web-vitals` to the server.

### When to Capture

- Start observers on page load (after `load` event + 100ms, consistent with v4 deferral)
- LCP finalizes on first user interaction (click/keypress/scroll) — the observer handles this automatically
- CLS accumulates throughout the session
- FCP and TTFB are one-shot (captured once per navigation)

### Message Flow

```
inject.js (PerformanceObserver) → webVitals state
                                        ↓
background.js polls every 5s    → POST /web-vitals
                                        ↓
server (V4Server)               → stores WebVitalsSnapshot
                                        ↓
MCP tool (get_web_vitals)       → returns current values
```

### Performance Budget

- Observer callbacks: < 0.05ms each (no DOM access, just number storage)
- Memory: ~200 bytes for vitals state
- Network: One POST every 5 seconds (only if values changed)
- No main thread blocking

---

## Server Implementation

### New HTTP Endpoint

```
POST /web-vitals
```

**Request body:**

```json
{
  "vitals": {
    "fcp": {"value": 1200, "timestamp": "2026-01-23T10:30:01Z"},
    "lcp": {"value": 2400, "timestamp": "2026-01-23T10:30:02Z", "element": "IMG", "size": 150000},
    "cls": {"value": 0.05, "timestamp": "2026-01-23T10:30:05Z", "shifts": 2},
    "ttfb": {"value": 180, "timestamp": "2026-01-23T10:30:00Z"}
  },
  "url": "https://example.com/dashboard",
  "timestamp": "2026-01-23T10:30:05Z"
}
```

### Types

```go
type WebVitalsSnapshot struct {
    FCP       *WebVitalEntry `json:"fcp,omitempty"`
    LCP       *LCPEntry      `json:"lcp,omitempty"`
    CLS       *CLSEntry      `json:"cls,omitempty"`
    TTFB      *WebVitalEntry `json:"ttfb,omitempty"`
    URL       string         `json:"url"`
    Timestamp string         `json:"timestamp"`
}

type WebVitalEntry struct {
    Value     int    `json:"value"`     // milliseconds
    Timestamp string `json:"timestamp"`
}

type LCPEntry struct {
    Value     int    `json:"value"`     // milliseconds
    Timestamp string `json:"timestamp"`
    Element   string `json:"element,omitempty"` // tagName
    Size      int    `json:"size,omitempty"`    // pixels
}

type CLSEntry struct {
    Value     float64 `json:"value"`     // CLS score (0-1+)
    Timestamp string  `json:"timestamp"`
    Shifts    int     `json:"shifts"`    // number of layout shifts
}
```

### Storage

On `V4Server`:

```go
type V4Server struct {
    // ... existing fields ...

    // Web Vitals - stores most recent snapshot per URL
    webVitals    map[string]*WebVitalsSnapshot // key: URL
    vitalsOrder  []string                       // insertion order for eviction
}
```

- Max 50 URL entries (one per page navigated)
- LRU eviction when cap hit
- Each new POST updates the entry for that URL (latest values win)

### Handler

```go
func (v *V4Server) HandleWebVitals(w http.ResponseWriter, r *http.Request) {
    // POST: store snapshot
    // Memory/rate check first (like other handlers)
}
```

---

## MCP Tool Definition

### `get_web_vitals`

**Description:** Get Core Web Vitals (FCP, LCP, CLS, TTFB) for pages visited in this session. Useful for checking performance after code changes.

**Input Schema:**

```json
{
  "type": "object",
  "properties": {
    "url": {
      "type": "string",
      "description": "Filter to a specific page URL"
    }
  }
}
```

**Response:**

```json
{
  "pages": [
    {
      "url": "https://example.com/dashboard",
      "timestamp": "2026-01-23T10:30:05Z",
      "vitals": {
        "fcp": {"value": 1200, "timestamp": "2026-01-23T10:30:01Z"},
        "lcp": {"value": 2400, "timestamp": "2026-01-23T10:30:02Z", "element": "IMG", "size": 150000},
        "cls": {"value": 0.05, "timestamp": "2026-01-23T10:30:05Z", "shifts": 2},
        "ttfb": {"value": 180, "timestamp": "2026-01-23T10:30:00Z"}
      }
    }
  ],
  "pages_count": 1
}
```

If a vital hasn't been captured yet (e.g., user hasn't interacted so LCP isn't finalized), the field is `null`.

---

## Test Cases

### Extension Tests (`extension-tests/web-vitals.test.js`)

| Test | Setup | Expected |
|------|-------|----------|
| FCP captured | Mock PerformanceObserver, emit `first-contentful-paint` entry | `webVitals.fcp.value` is set |
| LCP updates | Emit multiple LCP entries | Only last entry stored |
| CLS accumulates | Emit 3 layout-shift entries (no recent input) | CLS value is sum |
| CLS ignores input-driven | Emit shift with `hadRecentInput: true` | Not added to CLS |
| TTFB from navigation | Mock `performance.getEntriesByType('navigation')` | `webVitals.ttfb.value` is responseStart - requestStart |
| Observer error | PerformanceObserver constructor throws | No crash, vital stays null |
| Reset | Call `resetWebVitals()` | All values null |
| getWebVitals | Set some values | Returns copy, not reference |

### Server Tests (`cmd/dev-console/v4_test.go`)

| Test | Setup | Expected |
|------|-------|----------|
| Store snapshot | POST /web-vitals with full data | Stored correctly |
| Update snapshot | POST twice for same URL | Latest values kept |
| Multiple URLs | POST for 3 different URLs | 3 entries stored |
| URL cap | POST 51 unique URLs | Only 50 stored, oldest evicted |
| Partial vitals | POST with only FCP set | Other fields null, no error |
| Invalid JSON | POST malformed body | 400 response |
| Rate limited | Exceed rate limit, POST | 429 response |

### MCP Tool Tests

| Test | Setup | Expected |
|------|-------|----------|
| No vitals | Empty store | `{"pages": [], "pages_count": 0}` |
| One page | Store one snapshot | Returns it with all vitals |
| URL filter | Store 3 pages, filter by URL | Only matching page returned |
| Null vitals | Store page with only FCP | LCP/CLS/TTFB are null in response |

---

## SDK Replacement Angle

### What This Replaces

| Traditional Tool | What It Does | Gasoline Equivalent |
|-----------------|--------------|---------------------|
| `web-vitals` npm package | Captures FCP/LCP/CLS in production | Same metrics, zero package install, dev-only |
| Google Lighthouse | Lab-based performance audit | Real session metrics (not synthetic) |
| Chrome UX Report (CrUX) | Field data from real users | Dev session data (instant feedback) |
| Sentry Performance | Web Vitals in production with alerting | Same capture, local-only, AI-readable |
| New Relic Browser | Real User Monitoring (RUM) with dashboards | Same metrics, no dashboard — AI reads directly |
| SpeedCurve | Synthetic + RUM performance monitoring | Dev-time capture, no separate service |

### Key Differentiators

1. **Zero package install.** No `web-vitals` library, no Sentry SDK, no New Relic agent in your bundle.
2. **Real session, not synthetic.** Unlike Lighthouse, these are metrics from your actual dev session with real data and network conditions.
3. **AI-first output.** Raw numbers, not dashboards. The AI decides if 2400ms LCP is a problem based on context.
4. **Instant feedback loop.** Change code → reload → ask AI "how's performance?" → get numbers. No deploy-and-wait.
5. **No production overhead.** These observers run only during development. Zero impact on shipped code.
6. **Per-page granularity.** Navigate 5 pages during debugging → get vitals for each. SDKs aggregate across all users.

### Ecosystem Value

Web Vitals data feeds into:
- **Performance diffing** (future): "LCP was 1200ms, now 2400ms after your CSS change"
- **HAR export**: Resource waterfall + vitals = complete performance picture
- **CI integration** (future): Export vitals as JSON, compare against budget in CI

---

## Extension Changes Summary

| File | Change |
|------|--------|
| `extension/inject.js` | Add `startWebVitalsCapture()`, `stopWebVitalsCapture()`, `getWebVitals()`, `resetWebVitals()` |
| `extension/background.js` | Add 5-second polling for vitals changes, POST to `/web-vitals` |
| `extension/content.js` | Bridge `getWebVitals` message from background to inject |

---

## Server Changes Summary

| File | Change |
|------|--------|
| `cmd/dev-console/v4.go` | Add `WebVitalsSnapshot` types, storage, `HandleWebVitals()` |
| `cmd/dev-console/main.go` | Register `POST /web-vitals` handler |
| `cmd/dev-console/v4.go` | Add `get_web_vitals` to `v4ToolsList()` and `handleV4ToolCall()` |

---

## Implementation Notes

- Extension: ~80 lines JS (observers + state management)
- Server: ~100 lines Go (types + handler + MCP tool)
- Tests: ~80 lines JS + ~100 lines Go
- No new dependencies (PerformanceObserver is a browser API, not a library)
- CLS is the only metric that accumulates — others are one-shot per navigation
- LCP may update multiple times before user interaction finalizes it; always keep the latest
