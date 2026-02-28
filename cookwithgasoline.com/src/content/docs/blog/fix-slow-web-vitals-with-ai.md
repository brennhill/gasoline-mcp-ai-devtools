---
title: "How to Fix Slow Web Vitals with AI Using Gasoline MCP"
date: 2026-02-07
authors: [brenn]
tags: [performance, web-vitals, ai-development, how-to]
---

Your Core Web Vitals are red. LCP is 4.2 seconds. CLS is 0.35. Google Search Console is sending angry emails. Lighthouse gives you a list of suggestions, but they're generic — "reduce unused JavaScript" doesn't tell you *which* JavaScript or *why* it's slow.

Here's how to use Gasoline MCP to give your AI assistant real-time performance data, so it can identify exactly what's wrong and fix it.

<!-- more -->

## The Problem with Traditional Performance Tools

Lighthouse runs a synthetic test on a throttled connection. It's useful for benchmarking but disconnects from your actual development experience:

- **It's a snapshot**, not real-time — you fix something, re-run Lighthouse, wait 30 seconds, check the score, repeat
- **Suggestions are generic** — "eliminate render-blocking resources" doesn't tell you which stylesheet is the problem
- **No before/after** — you can't easily compare metrics across changes
- **No correlation** — it doesn't connect slow performance to specific code changes or network requests

Gasoline solves all four problems.

## Step 1: See Your Current Vitals

```js
observe({what: "vitals"})
```

Your AI gets the real numbers immediately:

| Metric | Value | Rating |
|--------|-------|--------|
| FCP | 2.1s | needs_improvement |
| LCP | 4.2s | poor |
| CLS | 0.35 | poor |
| INP | 280ms | needs_improvement |

No waiting for Lighthouse. No throttled simulation. These are the real metrics from your real browser on your real page.

## Step 2: Get the Full Performance Snapshot

```js
observe({what: "performance"})
```

This returns everything — not just vitals, but the full diagnostic picture:

**Navigation timing**: TTFB, DomContentLoaded, Load event — shows where time is spent during page load.

**Network summary by type**: How many scripts, stylesheets, images, and fonts loaded. Total transfer size and decoded size per category. Your AI can immediately see "you're loading 2.1MB of JavaScript across 47 files."

**Slowest requests**: The top resources by duration. If a single API call takes 3 seconds, it shows up here.

**Long tasks**: JavaScript execution that blocks the main thread for more than 50ms. The count, total blocking time, and longest task. If INP is bad, this is where you find out why.

## Step 3: Diagnose Each Metric

### Fixing LCP (Largest Contentful Paint)

LCP measures when the main content becomes visible. Common causes of slow LCP:

**High TTFB**: If `time_to_first_byte` is over 800ms, the server is the bottleneck. The AI checks your server code, database queries, or caching configuration.

**Render-blocking resources**: The network waterfall shows which scripts and stylesheets load before content paints:

```js
observe({what: "network_waterfall"})
```

The AI looks for CSS and JavaScript files with early `start_time` and long `duration`. These are the render-blocking resources. The fix: defer non-critical scripts, inline critical CSS, use `media` attributes on non-essential stylesheets.

**Large hero images**: If the LCP element is an image, the performance snapshot shows its transfer size. A 2MB uncompressed PNG as the hero image? The AI suggests WebP, proper sizing, and `fetchpriority="high"`.

**Late-loading content**: If FCP is fast but LCP is slow, the main content loads late — maybe behind an API call or a client-side render. The timeline shows the gap:

```js
observe({what: "timeline", include: ["network"]})
```

### Fixing CLS (Cumulative Layout Shift)

CLS measures visual stability. Things that cause layout shifts:

**Images without dimensions**: An `<img>` without `width` and `height` causes the browser to reflow when the image loads. The AI can audit your images:

```js
configure({action: "query_dom", selector: "img"})
```

**Dynamic content insertion**: Ads, banners, or lazy-loaded content that pushes existing content down. The timeline shows when shifts happen relative to network requests.

**Font loading**: Web fonts that cause text to resize. The AI checks for `font-display: swap` or `font-display: optional` in your CSS.

**CSS without containment**: The AI can check if your dynamic containers use `contain: layout` or explicit dimensions.

### Fixing INP (Interaction to Next Paint)

INP measures the worst-case responsiveness to user input. If INP is high, the main thread is busy when the user interacts.

**Long tasks are the smoking gun**: The performance snapshot shows total blocking time and the longest task. If you have 800ms of blocking time from 12 long tasks, the AI knows exactly what to target.

**Heavy event handlers**: The AI can read your click and input handlers to find expensive operations (DOM manipulation, synchronous computation, large state updates) that should be deferred or moved to a Web Worker.

**Third-party scripts**: The network waterfall shows which third-party scripts are loading and how long their execution takes:

```js
observe({what: "third_party_audit"})
```

A third-party analytics script running 200ms of JavaScript on every page load directly impacts INP.

## Step 4: Make Changes and Compare

This is where Gasoline shines. After the AI makes a change:

```js
interact({action: "refresh"})
```

Gasoline automatically captures before and after performance snapshots and computes a diff. The result includes:

- **Per-metric comparison**: LCP went from 4200ms to 2800ms (-33%, improved, rating: needs_improvement)
- **Resource changes**: "Removed analytics-v2.js (180KB), resized bundle.js from 450KB to 320KB"
- **Verdict**: "improved" — more metrics got better than worse

The AI says: *"LCP improved from 4.2s to 2.8s after removing the synchronous analytics script. CLS dropped from 0.35 to 0.08 after adding image dimensions. INP is still 250ms — let me look at the long tasks."*

No re-running Lighthouse. No waiting. Instant feedback.

## Step 5: Profile Specific Interactions

If INP is the remaining problem, profile the actual interactions:

```js
interact({action: "click", selector: "text=Load More", analyze: true})
```

The `analyze: true` parameter captures before/after performance around that specific click. The AI sees exactly how much main-thread time that button click consumes.

## Step 6: Generate a PR Summary

When you're done optimizing:

```js
generate({format: "pr_summary"})
```

This produces a before/after performance summary suitable for your pull request description — showing stakeholders exactly what improved and by how much.

## Real-World Example: Fixing a Dashboard

Here's a real workflow condensed:

**Initial vitals**: LCP 5.1s, CLS 0.42, INP 380ms

**AI diagnosis**:
1. Network waterfall shows 3.2MB of JavaScript across 62 requests
2. TTFB is 1.8s — slow API call blocks server-side rendering
3. Five images without width/height attributes cause CLS
4. Long tasks total 1.2s of blocking time — mostly from a charting library initializing synchronously

**AI fixes**:
1. Adds `loading="lazy"` to below-fold charts, defers non-critical scripts → JS drops to 1.4MB initial
2. Adds Redis caching to the slow API endpoint → TTFB drops to 200ms
3. Adds explicit dimensions to all images → CLS drops to 0.02
4. Wraps chart initialization in `requestIdleCallback` → blocking time drops to 180ms

**Final vitals**: LCP 1.9s (good), CLS 0.02 (good), INP 150ms (good)

**Total time**: One conversation, about 20 minutes. Each fix was verified immediately with perf_diff.

## Why This Beats Lighthouse

| | Lighthouse | Gasoline |
|--|-----------|----------|
| **Speed** | 30s synthetic run per check | Real-time, instant |
| **Comparison** | Manual before/after | Automatic perf_diff |
| **Diagnosis** | Generic suggestions | Your actual bottlenecks |
| **Fix cycle** | Run → fix → re-run → check | Fix → refresh → see diff |
| **Context** | Score and suggestions | Full waterfall, timeline, long tasks |
| **Integration** | Separate tool | Same terminal as your AI assistant |

Lighthouse tells you your LCP is 4.2 seconds and suggests "reduce unused JavaScript." Gasoline tells your AI that `analytics-v2.js` (180KB) loads synchronously in the head, blocks FCP by 800ms, and can be deferred without breaking anything.

## Performance Budgets for Prevention

Set budgets in `.gasoline.json` to catch regressions automatically:

```json
{
  "budgets": {
    "default": {
      "lcp_ms": 2500,
      "cls": 0.1,
      "inp_ms": 200,
      "total_transfer_kb": 500
    },
    "routes": {
      "/": { "lcp_ms": 2000 },
      "/dashboard": { "lcp_ms": 3000, "total_transfer_kb": 800 }
    }
  }
}
```

When any metric exceeds its budget, the AI gets an alert. Regressions are caught during development, not after deploy.

## Get Started

1. Install Gasoline and connect your AI tool ([Quick Start](/getting-started/))
2. Navigate to your slowest page
3. Ask: *"What are the Web Vitals for this page, and what's causing the worst ones?"*

Your AI sees the numbers, identifies the bottlenecks, and starts fixing. Real metrics, real fixes, real-time feedback.
