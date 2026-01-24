---
title: "Regression Detection"
description: "Automatic performance regression detection with adaptive baselines. Gasoline alerts your AI when page load times, Web Vitals, or network patterns degrade."
keywords: "performance regression detection, baseline comparison, performance alerts, adaptive baselines, performance monitoring, page speed regression"
permalink: /regression-detection/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Performance regressed? Your AI already knows."
toc: true
toc_sticky: true
---

Gasoline automatically builds performance baselines and flags regressions — so your AI catches slowdowns the moment they happen, not after users complain.

## <i class="fas fa-exclamation-circle"></i> The Problem

Performance regressions are silent killers. A new API dependency adds 500ms to page load. A CSS refactor triggers layout shifts. A third-party script bloats transfer size. None of these throw errors — they just quietly degrade the experience.

Without continuous monitoring, you only discover regressions when:
- A user files a complaint
- A Lighthouse audit weeks later shows declining scores
- Core Web Vitals drop in Search Console (30-day delay)

By then, the offending commit is buried under dozens of changes. Gasoline catches it immediately.

## <i class="fas fa-chart-line"></i> How Baselines Work

Gasoline maintains a **running average baseline** for each URL you visit:

1. **First visit** — The snapshot becomes the initial baseline
2. **Subsequent visits** — Simple average for the first 5 samples
3. **Established baseline** — Weighted average (80% existing, 20% new) to prevent sudden shifts

This adaptive approach means:
- Baselines stabilize after ~5 page loads
- Gradual improvements are reflected over time
- A single bad load doesn't corrupt the baseline
- Different pages have independent baselines

## <i class="fas fa-bell"></i> What Triggers an Alert

The `analyze` tool with `target: "changes"` compares the current state against a checkpoint and surfaces performance alerts when:

| Condition | Alert |
|-----------|-------|
| Load time > 2× baseline | "Page load time degraded" |
| LCP > 2× baseline | "LCP regressed" |
| New long tasks appeared | "Blocking JavaScript detected" |
| Transfer size > 2× baseline | "Page weight increased" |
| Endpoint latency > 3× baseline | "API endpoint degraded" |

## <i class="fas fa-terminal"></i> Usage

```json
// Analyze performance for a specific URL
{ "tool": "analyze", "arguments": { "target": "performance", "url": "/dashboard" } }
```

The response includes current metrics alongside the baseline:

```json
{
  "url": "/dashboard",
  "current": {
    "timing": { "load": 4200, "lcp": 3100 },
    "network": { "requestCount": 45, "transferSize": 2800000 }
  },
  "baseline": {
    "timing": { "load": 2100, "lcp": 1800 },
    "network": { "requestCount": 32, "transferSize": 1400000 },
    "sampleCount": 12
  },
  "regressions": [
    "Load time 2.0× baseline (4200ms vs 2100ms avg)",
    "Transfer size 2.0× baseline (2.8MB vs 1.4MB avg)"
  ]
}
```

## <i class="fas fa-search"></i> What Your AI Can Do With This

- **Catch regressions immediately** — "Your last code change doubled page load time. The new chart library added 1.4MB of JavaScript."
- **Identify root causes** — "LCP regressed because `hero-image.png` is now served uncompressed (was 200KB, now 1.8MB)."
- **Validate fixes** — "After adding lazy loading, load time is back to 2.2s (baseline: 2.1s). Regression resolved."
- **Track trends** — "Over the last 12 samples, TTFB has been creeping up. Current: 480ms, baseline: 320ms."

## <i class="fas fa-database"></i> Baseline Storage

- **Per-URL baselines** — Each page has its own performance profile
- **LRU eviction** — Up to 50 baselines stored (least-recently-used eviction)
- **Session-scoped** — Baselines reset when the server restarts
- **Resource fingerprinting** — Top 5 resources tracked by transfer size

## <i class="fas fa-link"></i> Related

- [Web Vitals](/web-vitals/) — The metrics that feed into regression detection
- [Session Checkpoints](/session-checkpoints/) — Named points for before/after comparison
- [PR Summaries](/pr-summaries/) — Summarize performance impact for code review
