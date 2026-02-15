// diff.go — Rich Action Results: performance diff computation.
// Computes before/after metric diffs, resource diffs, and generates
// human-readable summaries for AI consumption.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package performance

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ============================================
// Types for diff computation
// ============================================

// PageLoadMetrics is a simplified snapshot used for before/after comparison.
type PageLoadMetrics struct {
	URL          string        `json:"url"`
	Timestamp    int64         `json:"timestamp"`
	Timing       MetricsTiming `json:"timing"`
	CLS          *float64      `json:"cls,omitempty"`
	TransferSize int64         `json:"transfer_size"`
	RequestCount int           `json:"request_count"`
}

// MetricsTiming holds the core Web Vitals timing values.
type MetricsTiming struct {
	TTFB             float64  `json:"ttfb"`
	FCP              *float64 `json:"fcp,omitempty"`
	LCP              *float64 `json:"lcp,omitempty"`
	DomContentLoaded float64  `json:"dom_content_loaded"`
	Load             float64  `json:"load"`
}

// MetricDiff holds the before/after comparison for a single metric.
type MetricDiff struct {
	Before   float64 `json:"before"`
	After    float64 `json:"after"`
	Delta    float64 `json:"delta"`
	Pct      string  `json:"pct"`
	Unit     string  `json:"unit,omitempty"` // "ms", "KB", "count" — empty for unitless (CLS)
	Improved bool    `json:"improved"`
	Rating   string  `json:"rating,omitempty"` // "good", "needs_improvement", "poor" per Web Vitals thresholds
}

// PerfDiff holds the complete diff result with all metrics and resources.
type PerfDiff struct {
	Verdict   string                `json:"verdict"` // "improved", "regressed", "mixed", "unchanged"
	Metrics   map[string]MetricDiff `json:"metrics"`
	Resources ResourceDiff          `json:"resources"`
	Summary   string                `json:"summary"`
}

// ============================================
// SnapshotToPageLoadMetrics: type mapping
// ============================================

// SnapshotToPageLoadMetrics maps a PerformanceSnapshot (from the extension)
// to a PageLoadMetrics (used by ComputePerfDiff).
func SnapshotToPageLoadMetrics(s PerformanceSnapshot) PageLoadMetrics {
	m := PageLoadMetrics{
		URL:          s.URL,
		TransferSize: s.Network.TransferSize,
		RequestCount: s.Network.RequestCount,
		Timing: MetricsTiming{
			TTFB:             s.Timing.TimeToFirstByte,
			DomContentLoaded: s.Timing.DomContentLoaded,
			Load:             s.Timing.Load,
		},
	}
	if s.Timing.FirstContentfulPaint != nil {
		v := *s.Timing.FirstContentfulPaint
		m.Timing.FCP = &v
	}
	if s.Timing.LargestContentfulPaint != nil {
		v := *s.Timing.LargestContentfulPaint
		m.Timing.LCP = &v
	}
	if s.CLS != nil {
		v := *s.CLS
		m.CLS = &v
	}
	return m
}

// ============================================
// ComputePerfDiff: before/after metric comparison
// ============================================

// buildMetricDiff computes a single metric diff if at least one value is non-zero.
// Returns the diff and true if the metric is meaningful, false otherwise.
func buildMetricDiff(name string, beforeVal, afterVal float64) (MetricDiff, bool) {
	if beforeVal == 0 && afterVal == 0 {
		return MetricDiff{}, false
	}
	var d float64
	var pctStr string
	if beforeVal == 0 {
		d = round1(afterVal)
		pctStr = "n/a"
	} else {
		d = round1(afterVal - beforeVal)
		pctStr = formatPct((afterVal - beforeVal) / beforeVal * 100)
	}
	return MetricDiff{
		Before:   round1(beforeVal),
		After:    round1(afterVal),
		Delta:    d,
		Pct:      pctStr,
		Unit:     unitForMetric(name),
		Improved: d < 0,
		Rating:   rateMetric(name, round1(afterVal)),
	}, true
}

// addMetricIfValid adds a metric diff to the map if both values are meaningful.
func addMetricIfValid(metrics map[string]MetricDiff, name string, beforeVal, afterVal float64) {
	if md, ok := buildMetricDiff(name, beforeVal, afterVal); ok {
		metrics[name] = md
	}
}

// ptrPairValues returns the dereferenced values of two optional float64 pointers.
// Returns 0, 0, false if either is nil.
func ptrPairValues(a, b *float64) (float64, float64, bool) {
	if a == nil || b == nil {
		return 0, 0, false
	}
	return *a, *b, true
}

// computeVerdict derives the overall verdict from metric deltas.
func computeVerdict(metrics map[string]MetricDiff) string {
	if len(metrics) == 0 {
		return "unchanged"
	}
	improved, regressed := 0, 0
	for _, md := range metrics {
		if md.Delta < 0 {
			improved++
		} else if md.Delta > 0 {
			regressed++
		}
	}
	switch {
	case improved > 0 && regressed == 0:
		return "improved"
	case regressed > 0 && improved == 0:
		return "regressed"
	case improved > 0 && regressed > 0:
		return "mixed"
	default:
		return "unchanged"
	}
}

// ComputePerfDiff compares two page load snapshots and returns per-metric diffs.
// Metrics with no baseline (before is zero/nil) are omitted.
// Metrics where after is nil (didn't fire) are omitted.
func ComputePerfDiff(before, after PageLoadMetrics) PerfDiff {
	diff := PerfDiff{Metrics: make(map[string]MetricDiff)}

	addMetricIfValid(diff.Metrics, "ttfb", before.Timing.TTFB, after.Timing.TTFB)
	if bv, av, ok := ptrPairValues(before.Timing.FCP, after.Timing.FCP); ok {
		addMetricIfValid(diff.Metrics, "fcp", bv, av)
	}
	if bv, av, ok := ptrPairValues(before.Timing.LCP, after.Timing.LCP); ok {
		addMetricIfValid(diff.Metrics, "lcp", bv, av)
	}
	addMetricIfValid(diff.Metrics, "dom_content_loaded", before.Timing.DomContentLoaded, after.Timing.DomContentLoaded)
	addMetricIfValid(diff.Metrics, "load", before.Timing.Load, after.Timing.Load)
	if bv, av, ok := ptrPairValues(before.CLS, after.CLS); ok {
		addMetricIfValid(diff.Metrics, "cls", bv, av)
	}
	if before.TransferSize > 0 {
		addMetricIfValid(diff.Metrics, "transfer_kb", float64(before.TransferSize)/1024, float64(after.TransferSize)/1024)
	}
	if before.RequestCount > 0 {
		addMetricIfValid(diff.Metrics, "requests", float64(before.RequestCount), float64(after.RequestCount))
	}

	diff.Verdict = computeVerdict(diff.Metrics)
	diff.Summary = GeneratePerfSummary(diff)
	return diff
}

// ============================================
// ComputeResourceDiffForNav: added/removed/resized resources
// ============================================

// buildResourceMap indexes resource entries by URL.
func buildResourceMap(entries []ResourceEntry) map[string]ResourceEntry {
	m := make(map[string]ResourceEntry, len(entries))
	for _, r := range entries {
		m[r.URL] = r
	}
	return m
}

// isSignificantSizeChange returns true if the size delta is >= 10% OR >= 1KB.
func isSignificantSizeChange(beforeSize, delta int64) bool {
	absDelta := delta
	if absDelta < 0 {
		absDelta = -absDelta
	}
	if beforeSize == 0 {
		return absDelta >= 1024
	}
	pctChange := float64(absDelta) / float64(beforeSize) * 100
	return pctChange >= 10 || absDelta >= 1024
}

// ComputeResourceDiffForNav compares resource lists and categorizes changes.
// Resources are matched by URL. Small changes (<10% AND <1KB) are ignored.
func ComputeResourceDiffForNav(before, after []ResourceEntry) ResourceDiff {
	diff := ResourceDiff{}
	beforeMap := buildResourceMap(before)
	afterMap := buildResourceMap(after)

	// Removed: in before but not in after
	for _, r := range before {
		if _, exists := afterMap[r.URL]; !exists {
			diff.Removed = append(diff.Removed, RemovedResource{
				URL: r.URL, Type: r.Type, SizeBytes: r.TransferSize,
			})
		}
	}

	// Added: in after but not in before
	for _, r := range after {
		if _, exists := beforeMap[r.URL]; !exists {
			diff.Added = append(diff.Added, AddedResource{
				URL: r.URL, Type: r.Type, SizeBytes: r.TransferSize,
				DurationMs: r.Duration, RenderBlocking: r.RenderBlocking,
			})
		}
	}

	// Resized: in both with significant size change
	for _, afterR := range after {
		beforeR, exists := beforeMap[afterR.URL]
		if !exists {
			continue
		}
		delta := afterR.TransferSize - beforeR.TransferSize
		if delta == 0 || !isSignificantSizeChange(beforeR.TransferSize, delta) {
			continue
		}
		diff.Resized = append(diff.Resized, ResizedResource{
			URL: afterR.URL, BaselineBytes: beforeR.TransferSize,
			CurrentBytes: afterR.TransferSize, DeltaBytes: delta,
		})
	}

	return diff
}

// ============================================
// GeneratePerfSummary: human-readable summary
// ============================================

// metricEntry pairs a metric name with its diff for sorting.
type metricEntry struct {
	name string
	md   MetricDiff
}

// sortedMetrics returns metrics sorted by absolute percentage change (biggest first).
func sortedMetrics(metrics map[string]MetricDiff) []metricEntry {
	sorted := make([]metricEntry, 0, len(metrics))
	for name, md := range metrics {
		sorted = append(sorted, metricEntry{name, md})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return parsePctAbs(sorted[i].md) > parsePctAbs(sorted[j].md)
	})
	return sorted
}

// metricDirection returns "improved", "regressed", or "unchanged" for a metric diff.
func metricDirection(md MetricDiff) string {
	if md.Delta == 0 {
		return "unchanged"
	}
	if md.Improved {
		return "improved"
	}
	return "regressed"
}

// formatTopMetric formats the biggest-change metric for the summary lead.
func formatTopMetric(top metricEntry) string {
	entry := fmt.Sprintf("%s %s %.0f%%", strings.ToUpper(top.name), metricDirection(top.md), parsePctAbs(top.md))
	if top.md.Rating != "" {
		entry += fmt.Sprintf(" (%s)", top.md.Rating)
	}
	return entry
}

// firstRegressionWarning returns a warning string for the first regressed metric, or "".
func firstRegressionWarning(sorted []metricEntry) string {
	for _, e := range sorted {
		if !e.md.Improved && e.md.Delta != 0 {
			return fmt.Sprintf("warning: %s regressed %.0f%%", strings.ToUpper(e.name), parsePctAbs(e.md))
		}
	}
	return ""
}

// truncate trims a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// GeneratePerfSummary generates a concise (<200 char) summary of the diff.
// Leads with the biggest improvement, mentions resource changes, flags regressions.
func GeneratePerfSummary(diff PerfDiff) string {
	if len(diff.Metrics) == 0 && len(diff.Resources.Removed) == 0 &&
		len(diff.Resources.Added) == 0 && len(diff.Resources.Resized) == 0 {
		return "No performance changes detected."
	}

	sorted := sortedMetrics(diff.Metrics)
	var parts []string

	if len(sorted) > 0 {
		parts = append(parts, formatTopMetric(sorted[0]))
	}

	if len(diff.Resources.Removed) > 0 {
		parts = append(parts, fmt.Sprintf("removed %s", lastPathSegment(diff.Resources.Removed[0].URL)))
	}

	// Flag regression if the lead metric is an improvement (regression is secondary)
	if warn := firstRegressionWarning(sorted); warn != "" {
		if len(sorted) == 0 || sorted[0].md.Improved {
			parts = append(parts, warn)
		}
	}

	return truncate(strings.Join(parts, "; "), 200)
}

// ============================================
// Helpers
// ============================================

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func formatPct(pct float64) string {
	rounded := math.Round(pct)
	if rounded >= 0 {
		return fmt.Sprintf("+%.0f%%", rounded)
	}
	return fmt.Sprintf("%.0f%%", rounded)
}

func lastPathSegment(url string) string {
	i := strings.LastIndex(url, "/")
	if i >= 0 && i < len(url)-1 {
		return url[i+1:]
	}
	return url
}

// unitForMetric returns the unit string for a given metric name.
func unitForMetric(name string) string {
	switch name {
	case "ttfb", "fcp", "lcp", "dom_content_loaded", "load":
		return "ms"
	case "transfer_kb":
		return "KB"
	case "requests":
		return "count"
	default:
		return "" // CLS is unitless
	}
}

// metricThreshold defines Web Vitals thresholds for a metric.
type metricThreshold struct {
	good float64 // values below this are "good"
	ni   float64 // values at or below this are "needs_improvement"; above is "poor"
}

// webVitalsThresholds maps metric names to their Web Vitals thresholds.
var webVitalsThresholds = map[string]metricThreshold{
	"lcp":  {good: 2500, ni: 4000},
	"fcp":  {good: 1800, ni: 3000},
	"ttfb": {good: 800, ni: 1800},
	"cls":  {good: 0.1, ni: 0.25},
}

// rateMetric returns a Web Vitals rating for the given metric's current value.
// Returns "" for metrics without standard thresholds.
func rateMetric(name string, value float64) string {
	t, ok := webVitalsThresholds[name]
	if !ok {
		return ""
	}
	if value < t.good {
		return "good"
	}
	if value <= t.ni {
		return "needs_improvement"
	}
	return "poor"
}

// parsePctAbs extracts the absolute percentage value from a formatted string like "+50%" or "-33%".
func parsePctAbs(md MetricDiff) float64 {
	if md.Before == 0 {
		return 0
	}
	return math.Abs(md.Delta / md.Before * 100)
}
