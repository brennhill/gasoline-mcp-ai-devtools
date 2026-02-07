// diff.go — Rich Action Results: performance diff computation.
// Computes before/after metric diffs, resource diffs, and generates
// human-readable summaries for AI consumption.
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

// ComputePerfDiff compares two page load snapshots and returns per-metric diffs.
// Metrics with no baseline (before is zero/nil) are omitted.
// Metrics where after is nil (didn't fire) are omitted.
func ComputePerfDiff(before, after PageLoadMetrics) PerfDiff {
	diff := PerfDiff{
		Metrics: make(map[string]MetricDiff),
	}

	// Helper: add a metric if both values are meaningful
	addMetric := func(name string, beforeVal, afterVal float64) {
		if beforeVal == 0 {
			return // no baseline
		}
		d := round1(afterVal - beforeVal)
		pct := (afterVal - beforeVal) / beforeVal * 100
		diff.Metrics[name] = MetricDiff{
			Before:   round1(beforeVal),
			After:    round1(afterVal),
			Delta:    d,
			Pct:      formatPct(pct),
			Unit:     unitForMetric(name),
			Improved: d < 0, // lower is better for timing/size
			Rating:   rateMetric(name, round1(afterVal)),
		}
	}

	// TTFB
	addMetric("ttfb", before.Timing.TTFB, after.Timing.TTFB)

	// FCP (optional)
	if before.Timing.FCP != nil && after.Timing.FCP != nil {
		addMetric("fcp", *before.Timing.FCP, *after.Timing.FCP)
	}

	// LCP (optional)
	if before.Timing.LCP != nil && after.Timing.LCP != nil {
		addMetric("lcp", *before.Timing.LCP, *after.Timing.LCP)
	}

	// DomContentLoaded
	addMetric("dom_content_loaded", before.Timing.DomContentLoaded, after.Timing.DomContentLoaded)

	// Load
	addMetric("load", before.Timing.Load, after.Timing.Load)

	// CLS (lower is better)
	if before.CLS != nil && after.CLS != nil {
		addMetric("cls", *before.CLS, *after.CLS)
	}

	// Transfer size in KB
	if before.TransferSize > 0 {
		addMetric("transfer_kb", float64(before.TransferSize)/1024, float64(after.TransferSize)/1024)
	}

	// Request count
	if before.RequestCount > 0 {
		addMetric("requests", float64(before.RequestCount), float64(after.RequestCount))
	}

	// Compute verdict from metric directions
	improved, regressed := 0, 0
	for _, md := range diff.Metrics {
		if md.Delta < 0 {
			improved++
		} else if md.Delta > 0 {
			regressed++
		}
	}
	switch {
	case len(diff.Metrics) == 0:
		diff.Verdict = "unchanged"
	case improved > 0 && regressed == 0:
		diff.Verdict = "improved"
	case regressed > 0 && improved == 0:
		diff.Verdict = "regressed"
	case improved > 0 && regressed > 0:
		diff.Verdict = "mixed"
	default:
		diff.Verdict = "unchanged"
	}

	diff.Summary = GeneratePerfSummary(diff)
	return diff
}

// ============================================
// ComputeResourceDiffForNav: added/removed/resized resources
// ============================================

// ComputeResourceDiffForNav compares resource lists and categorizes changes.
// Resources are matched by URL. Small changes (<10% AND <1KB) are ignored.
func ComputeResourceDiffForNav(before, after []ResourceEntry) ResourceDiff {
	diff := ResourceDiff{}

	beforeMap := make(map[string]ResourceEntry, len(before))
	for _, r := range before {
		beforeMap[r.URL] = r
	}

	afterMap := make(map[string]ResourceEntry, len(after))
	for _, r := range after {
		afterMap[r.URL] = r
	}

	// Removed: in before but not in after
	for _, r := range before {
		if _, exists := afterMap[r.URL]; !exists {
			diff.Removed = append(diff.Removed, RemovedResource{
				URL:       r.URL,
				Type:      r.Type,
				SizeBytes: r.TransferSize,
			})
		}
	}

	// Added: in after but not in before
	for _, r := range after {
		if _, exists := beforeMap[r.URL]; !exists {
			diff.Added = append(diff.Added, AddedResource{
				URL:            r.URL,
				Type:           r.Type,
				SizeBytes:      r.TransferSize,
				DurationMs:     r.Duration,
				RenderBlocking: r.RenderBlocking,
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
		if delta == 0 {
			continue
		}
		absDelta := delta
		if absDelta < 0 {
			absDelta = -absDelta
		}
		// Skip small changes: <10% AND <1KB
		pctChange := float64(absDelta) / float64(beforeR.TransferSize) * 100
		if pctChange < 10 && absDelta < 1024 {
			continue
		}
		diff.Resized = append(diff.Resized, ResizedResource{
			URL:           afterR.URL,
			BaselineBytes: beforeR.TransferSize,
			CurrentBytes:  afterR.TransferSize,
			DeltaBytes:    delta,
		})
	}

	return diff
}

// ============================================
// GeneratePerfSummary: human-readable summary
// ============================================

// GeneratePerfSummary generates a concise (<200 char) summary of the diff.
// Leads with the biggest improvement, mentions resource changes, flags regressions.
func GeneratePerfSummary(diff PerfDiff) string {
	if len(diff.Metrics) == 0 && len(diff.Resources.Removed) == 0 &&
		len(diff.Resources.Added) == 0 && len(diff.Resources.Resized) == 0 {
		return "No performance changes detected."
	}

	var parts []string

	// Sort metrics by absolute percentage change (biggest first)
	type metricEntry struct {
		name string
		md   MetricDiff
	}
	sorted := make([]metricEntry, 0, len(diff.Metrics))
	for name, md := range diff.Metrics {
		sorted = append(sorted, metricEntry{name, md})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return parsePctAbs(sorted[i].md) > parsePctAbs(sorted[j].md)
	})

	// Check for regressions
	hasRegression := false
	for _, e := range sorted {
		if !e.md.Improved {
			hasRegression = true
			break
		}
	}

	// Lead with biggest change (absolute percentage, no redundant sign)
	if len(sorted) > 0 {
		top := sorted[0]
		label := strings.ToUpper(top.name)
		absPct := fmt.Sprintf("%.0f%%", parsePctAbs(top.md))
		direction := "improved"
		if !top.md.Improved {
			direction = "regressed"
		}
		entry := fmt.Sprintf("%s %s %s", label, direction, absPct)
		if top.md.Rating != "" {
			entry += fmt.Sprintf(" (%s)", top.md.Rating)
		}
		parts = append(parts, entry)
	}

	// Mention removed resources (most impactful change)
	if len(diff.Resources.Removed) > 0 {
		r := diff.Resources.Removed[0]
		name := lastPathSegment(r.URL)
		parts = append(parts, fmt.Sprintf("removed %s", name))
	}

	// Flag regression warning
	if hasRegression && (len(sorted) == 0 || sorted[0].md.Improved) {
		// Regression exists but isn't the lead — add warning
		for _, e := range sorted {
			if !e.md.Improved {
				absPct := fmt.Sprintf("%.0f%%", parsePctAbs(e.md))
				parts = append(parts, fmt.Sprintf("warning: %s regressed %s", strings.ToUpper(e.name), absPct))
				break
			}
		}
	}

	summary := strings.Join(parts, "; ")

	// Truncate to 200 chars
	if len(summary) > 200 {
		summary = summary[:197] + "..."
	}

	return summary
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

// rateMetric returns a Web Vitals rating for the given metric's current value.
// Returns "" for metrics without standard thresholds.
func rateMetric(name string, value float64) string {
	switch name {
	case "lcp":
		if value < 2500 {
			return "good"
		} else if value <= 4000 {
			return "needs_improvement"
		}
		return "poor"
	case "fcp":
		if value < 1800 {
			return "good"
		} else if value <= 3000 {
			return "needs_improvement"
		}
		return "poor"
	case "ttfb":
		if value < 800 {
			return "good"
		} else if value <= 1800 {
			return "needs_improvement"
		}
		return "poor"
	case "cls":
		if value < 0.1 {
			return "good"
		} else if value <= 0.25 {
			return "needs_improvement"
		}
		return "poor"
	default:
		return ""
	}
}

// parsePctAbs extracts the absolute percentage value from a formatted string like "+50%" or "-33%".
func parsePctAbs(md MetricDiff) float64 {
	if md.Before == 0 {
		return 0
	}
	return math.Abs(md.Delta / md.Before * 100)
}
