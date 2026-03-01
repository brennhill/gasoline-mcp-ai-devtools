// Purpose: Package performance — performance snapshot types, metric diffing, and wire format definitions.
// Why: Makes performance regressions measurable and comparable across baseline runs.
// Docs: docs/features/feature/performance-audit/index.md

/*
Package performance provides types and functions for performance monitoring,
baseline comparison, and regression detection.

Key types:
  - PerformanceSnapshot: captured page load metrics (timing, network, resources).
  - MetricDiff: before/after comparison of a single metric with percentage change.
  - WirePerformanceTiming: wire format for timing data received from the extension.

Key functions:
  - ComputeDiff: computes a rich diff between two performance snapshots.
  - FormatDiffSummary: generates a human-readable regression summary for AI consumption.
*/
package performance
