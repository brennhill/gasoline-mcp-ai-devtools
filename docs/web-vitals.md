---
title: "Web Vitals"
description: "Monitor Core Web Vitals (LCP, CLS, INP, FCP) in real time. Your AI assistant sees performance metrics as you browse and identifies pages that need optimization."
keywords: "Core Web Vitals, LCP, CLS, INP, FCP, performance monitoring, web performance MCP, page speed, user experience metrics"
permalink: /web-vitals/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Real-time Core Web Vitals, assessed and queryable by your AI."
toc: true
toc_sticky: true
---

Gasoline captures Core Web Vitals as you browse and delivers them to your AI with quality assessments — so it can tell you exactly which pages need work.

## <i class="fas fa-exclamation-circle"></i> The Problem

Google's Core Web Vitals directly impact search ranking and user experience, but most developers don't monitor them during development. You ship a feature, then discover weeks later that LCP regressed from "good" to "poor" because of a new above-the-fold image or an unoptimized API call.

Running Lighthouse manually is slow, requires context-switching, and only shows a snapshot. You need continuous visibility into how your app performs as you build it.

## <i class="fas fa-heartbeat"></i> What Gets Measured

<img src="/assets/images/sparky/features/sparky-running-web.webp" alt="Sparky running for performance" style="float: right; width: 140px; margin: 0 0 20px 20px; border-radius: 6px;" />

| Metric | What It Measures | Good | Poor |
|--------|-----------------|------|------|
| **FCP** | First Contentful Paint — time until first text/image renders | < 1.8s | ≥ 3.0s |
| **LCP** | Largest Contentful Paint — time until largest element renders | < 2.5s | ≥ 4.0s |
| **CLS** | Cumulative Layout Shift — visual stability score | < 0.1 | ≥ 0.25 |
| **INP** | Interaction to Next Paint — input responsiveness | < 200ms | ≥ 500ms |

Each metric is automatically assessed as "good", "needs-improvement", or "poor" based on Google's thresholds.

## <i class="fas fa-cogs"></i> How It Works

1. <i class="fas fa-browser"></i> The extension captures performance timing data from the browser's Performance API
2. <i class="fas fa-server"></i> Snapshots are sent to the local Gasoline server with full timing breakdowns
3. <i class="fas fa-robot"></i> Your AI calls `observe` with `what: "vitals"` to see current metrics
4. <i class="fas fa-chart-line"></i> Each metric includes its value and quality assessment

## <i class="fas fa-terminal"></i> Usage

```json
// Observe current Web Vitals
{ "tool": "observe", "arguments": { "what": "vitals" } }
```

Response includes:

```json
{
  "fcp": { "value": 1200, "assessment": "good" },
  "lcp": { "value": 2800, "assessment": "needs-improvement" },
  "cls": { "value": 0.05, "assessment": "good" },
  "inp": { "value": 150, "assessment": "good" },
  "loadTime": { "value": 3200, "assessment": "needs-improvement" },
  "url": "https://myapp.com/dashboard"
}
```

## <i class="fas fa-search"></i> What Your AI Can Do With This

- **Identify slow pages** — "Your dashboard LCP is 2.8s (needs improvement). The hero image is 2.4MB uncompressed."
- **Track improvements** — "After lazy-loading the chart, LCP dropped from 2.8s to 1.9s (good)."
- **Catch regressions** — "The login page CLS jumped from 0.02 to 0.18 after your CSS change."
- **Prioritize fixes** — "Three pages have poor INP. The settings page is worst at 620ms due to synchronous validation."

## <i class="fas fa-layer-group"></i> Beyond Vitals

In addition to Core Web Vitals, each performance snapshot includes:

- **Navigation timing** — DOMContentLoaded, Load, TTFB, DomInteractive
- **Network summary** — total request count, transfer size
- **Long tasks** — count, total blocking time, longest task duration
- **Top resources** — largest assets by transfer size

This gives your AI full context to diagnose *why* a metric is poor, not just *that* it's poor.

## <i class="fas fa-link"></i> Related

- [Regression Detection](/regression-detection/) — Automatic alerts when vitals degrade
- [Performance SLOs](/performance-slos/) — Gasoline's own performance budgets
- [PR Summaries](/pr-summaries/) — Include vitals impact in pull requests
