# Review: SPA Route Measurement (tech-spec-spa-route-measurement.md)

## Executive Summary

This spec adds per-route performance measurement for single-page applications by intercepting `history.pushState`, `replaceState`, and `popstate`, then measuring time-to-interactive via a quiescence heuristic (network + render + main thread idle). The feature fills a genuine gap -- Gasoline currently sees only initial page loads -- but the quiescence detection algorithm has several conditions that will produce unreliable TTI values in real-world apps, and the route normalization logic is brittle.

## Critical Issues (Must Fix Before Implementation)

...existing code...

