---
status: shipped
scope: feature/spa-route-measurement/review
ai-priority: high
tags: [review, issues]
relates-to: [TECH_SPEC.md, PRODUCT_SPEC.md]
last-verified: 2026-01-31
---

# Review: SPA Route Measurement (tech-spec-spa-route-measurement.md)

## Executive Summary

This spec adds per-route performance measurement for single-page applications by intercepting `history.pushState`, `replaceState`, and `popstate`, then measuring time-to-interactive via a quiescence heuristic (network + render + main thread idle). The feature fills a genuine gap -- Gasoline currently sees only initial page loads -- but the quiescence detection algorithm has several conditions that will produce unreliable TTI values in real-world apps, and the route normalization logic is brittle.

## Critical Issues (Must Fix Before Implementation)

...existing code...

