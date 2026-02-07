// diff_test.go — TDD tests for Rich Action Results diff computation.
// Tests the perf diff, resource diff, and summary generation functions.
// These tests define the contract — implementations in diff.go.
//
// Run: go test ./internal/performance/ -run "TestPerfDiff|TestResourceDiff|TestSummary" -v
package performance

import (
	"math"
	"strings"
	"testing"
)

// ============================================
// PerfDiff: Before/After Metric Comparison
// ============================================

func TestPerfDiff_BasicImprovement(t *testing.T) {
	t.Parallel()
	fcp900 := 900.0
	fcp800 := 800.0
	lcp2800 := 2800.0
	lcp1200 := 1200.0
	cls02 := 0.02
	cls01 := 0.01

	before := PageLoadMetrics{
		URL:       "https://example.com",
		Timestamp: 1000,
		Timing: MetricsTiming{
			TTFB: 120, FCP: &fcp900, LCP: &lcp2800,
			DomContentLoaded: 800, Load: 1500,
		},
		CLS:          &cls02,
		TransferSize: 768 * 1024,
		RequestCount: 58,
	}
	after := PageLoadMetrics{
		URL:       "https://example.com",
		Timestamp: 2000,
		Timing: MetricsTiming{
			TTFB: 80, FCP: &fcp800, LCP: &lcp1200,
			DomContentLoaded: 700, Load: 1100,
		},
		CLS:          &cls01,
		TransferSize: 512 * 1024,
		RequestCount: 42,
	}

	diff := ComputePerfDiff(before, after)

	// LCP improved 57%
	lcp := diff.Metrics["lcp"]
	if lcp.Before != 2800 {
		t.Errorf("lcp.Before = %v, want 2800", lcp.Before)
	}
	if lcp.After != 1200 {
		t.Errorf("lcp.After = %v, want 1200", lcp.After)
	}
	if lcp.Delta != -1600 {
		t.Errorf("lcp.Delta = %v, want -1600", lcp.Delta)
	}
	if !lcp.Improved {
		t.Error("lcp.Improved should be true (lower is better)")
	}

	// Transfer size decreased
	transfer := diff.Metrics["transfer_kb"]
	if !transfer.Improved {
		t.Error("transfer_kb.Improved should be true")
	}

	// Request count decreased
	requests := diff.Metrics["requests"]
	if requests.Before != 58 || requests.After != 42 {
		t.Errorf("requests = %v→%v, want 58→42", requests.Before, requests.After)
	}

	// Summary must exist and be non-empty
	if diff.Summary == "" {
		t.Error("Summary must not be empty")
	}
}

func TestPerfDiff_Regression(t *testing.T) {
	t.Parallel()
	lcp1200 := 1200.0
	lcp2800 := 2800.0

	before := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp2800, TTFB: 200, Load: 2500},
	}

	diff := ComputePerfDiff(before, after)

	lcp := diff.Metrics["lcp"]
	if lcp.Improved {
		t.Error("lcp.Improved should be false (LCP got worse)")
	}
	if lcp.Delta <= 0 {
		t.Errorf("lcp.Delta = %v, want positive (regression)", lcp.Delta)
	}

	// Summary should flag the regression
	if !strings.Contains(strings.ToLower(diff.Summary), "regress") &&
		!strings.Contains(strings.ToLower(diff.Summary), "worse") &&
		!strings.Contains(strings.ToLower(diff.Summary), "increased") {
		t.Errorf("Summary should flag regression. Got: %q", diff.Summary)
	}
}

func TestPerfDiff_NilLCP(t *testing.T) {
	t.Parallel()
	lcp := 1200.0

	before := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp, TTFB: 80, Load: 1100},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{LCP: nil, TTFB: 80, Load: 1100}, // LCP didn't fire
	}

	diff := ComputePerfDiff(before, after)

	// LCP should be absent (not zero, not crash)
	if _, exists := diff.Metrics["lcp"]; exists {
		t.Error("lcp should be omitted when after.LCP is nil")
	}
}

func TestPerfDiff_FirstLoad_NoPrevious(t *testing.T) {
	t.Parallel()
	lcp := 1200.0

	// Empty before (first page load, no baseline)
	before := PageLoadMetrics{}
	after := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp, TTFB: 80, Load: 1100},
	}

	diff := ComputePerfDiff(before, after)

	// Should have no meaningful metrics (no baseline to compare)
	if len(diff.Metrics) > 0 {
		t.Errorf("First load should have no diff metrics, got %d", len(diff.Metrics))
	}
}

func TestPerfDiff_PercentageCalculation(t *testing.T) {
	t.Parallel()
	lcp100 := 100.0
	lcp50 := 50.0

	before := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp100, TTFB: 200, Load: 1000},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp50, TTFB: 100, Load: 500},
	}

	diff := ComputePerfDiff(before, after)

	lcp := diff.Metrics["lcp"]
	// 50→100 is -50%, should show as "-50%"
	if !strings.Contains(lcp.Pct, "-50") {
		t.Errorf("lcp.Pct = %q, want contains '-50'", lcp.Pct)
	}

	ttfb := diff.Metrics["ttfb"]
	if !strings.Contains(ttfb.Pct, "-50") {
		t.Errorf("ttfb.Pct = %q, want contains '-50'", ttfb.Pct)
	}
}

// ============================================
// ResourceDiff: Added/Removed/Resized Resources
// ============================================

func TestResourceDiff_RemovedResource(t *testing.T) {
	t.Parallel()
	before := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
		{URL: "/old-bundle.js", Type: "script", TransferSize: 512000, Duration: 200},
		{URL: "/style.css", Type: "stylesheet", TransferSize: 20000, Duration: 50},
	}
	after := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
		{URL: "/style.css", Type: "stylesheet", TransferSize: 20000, Duration: 50},
	}

	diff := ComputeResourceDiffForNav(before, after)

	if len(diff.Removed) != 1 {
		t.Fatalf("Expected 1 removed, got %d", len(diff.Removed))
	}
	if diff.Removed[0].URL != "/old-bundle.js" {
		t.Errorf("Removed URL = %q, want /old-bundle.js", diff.Removed[0].URL)
	}
	if diff.Removed[0].SizeBytes != 512000 {
		t.Errorf("Removed size = %d, want 512000", diff.Removed[0].SizeBytes)
	}
}

func TestResourceDiff_AddedResource(t *testing.T) {
	t.Parallel()
	before := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
	}
	after := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
		{URL: "/analytics.js", Type: "script", TransferSize: 45000, Duration: 80},
	}

	diff := ComputeResourceDiffForNav(before, after)

	if len(diff.Added) != 1 {
		t.Fatalf("Expected 1 added, got %d", len(diff.Added))
	}
	if diff.Added[0].URL != "/analytics.js" {
		t.Errorf("Added URL = %q, want /analytics.js", diff.Added[0].URL)
	}
}

func TestResourceDiff_ResizedResource(t *testing.T) {
	t.Parallel()
	before := []ResourceEntry{
		{URL: "/main.js", Type: "script", TransferSize: 512000, Duration: 200},
	}
	after := []ResourceEntry{
		{URL: "/main.js", Type: "script", TransferSize: 256000, Duration: 150},
	}

	diff := ComputeResourceDiffForNav(before, after)

	if len(diff.Resized) != 1 {
		t.Fatalf("Expected 1 resized, got %d", len(diff.Resized))
	}
	if diff.Resized[0].URL != "/main.js" {
		t.Errorf("Resized URL = %q, want /main.js", diff.Resized[0].URL)
	}
	if diff.Resized[0].BaselineBytes != 512000 {
		t.Errorf("Baseline = %d, want 512000", diff.Resized[0].BaselineBytes)
	}
	if diff.Resized[0].CurrentBytes != 256000 {
		t.Errorf("Current = %d, want 256000", diff.Resized[0].CurrentBytes)
	}
}

func TestResourceDiff_SmallChangeIgnored(t *testing.T) {
	t.Parallel()
	// <10% change AND <1KB should be ignored
	before := []ResourceEntry{
		{URL: "/tiny.js", Type: "script", TransferSize: 500, Duration: 10},
	}
	after := []ResourceEntry{
		{URL: "/tiny.js", Type: "script", TransferSize: 520, Duration: 10},
	}

	diff := ComputeResourceDiffForNav(before, after)

	if len(diff.Resized) != 0 {
		t.Errorf("Tiny change should be ignored, got %d resized", len(diff.Resized))
	}
}

func TestResourceDiff_EmptyBaseline(t *testing.T) {
	t.Parallel()
	after := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
	}

	diff := ComputeResourceDiffForNav(nil, after)

	// All resources are "added" when baseline is empty
	if len(diff.Added) != 1 {
		t.Errorf("Empty baseline: all resources should be added, got %d", len(diff.Added))
	}
}

// ============================================
// Summary Generation
// ============================================

func TestSummary_LeadsWithBiggestImprovement(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp":  {Before: 2800, After: 1200, Delta: -1600, Pct: "-57%", Improved: true},
			"ttfb": {Before: 120, After: 110, Delta: -10, Pct: "-8%", Improved: true},
		},
	}

	summary := GeneratePerfSummary(diff)

	// Should lead with LCP (biggest improvement)
	if !strings.HasPrefix(strings.ToUpper(summary), "LCP") {
		t.Errorf("Summary should lead with biggest improvement (LCP). Got: %q", summary)
	}
}

func TestSummary_MentionsResourceChanges(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"transfer_kb": {Before: 768, After: 512, Delta: -256, Pct: "-33%", Improved: true},
		},
		Resources: ResourceDiff{
			Removed: []RemovedResource{
				{URL: "/old-bundle.js", Type: "script", SizeBytes: 262144},
			},
		},
	}

	summary := GeneratePerfSummary(diff)

	if !strings.Contains(summary, "old-bundle.js") {
		t.Errorf("Summary should mention removed resource. Got: %q", summary)
	}
}

func TestSummary_FlagsRegression(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"cls": {Before: 0.01, After: 0.03, Delta: 0.02, Pct: "+200%", Improved: false},
		},
	}

	summary := GeneratePerfSummary(diff)

	lower := strings.ToLower(summary)
	if !strings.Contains(lower, "regress") && !strings.Contains(lower, "warning") && !strings.Contains(lower, "worse") {
		t.Errorf("Summary should flag CLS regression. Got: %q", summary)
	}
}

func TestSummary_Under200Chars(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp":         {Before: 2800, After: 1200, Delta: -1600, Pct: "-57%", Improved: true},
			"fcp":         {Before: 900, After: 800, Delta: -100, Pct: "-11%", Improved: true},
			"cls":         {Before: 0.02, After: 0.01, Delta: -0.01, Pct: "-50%", Improved: true},
			"ttfb":        {Before: 120, After: 80, Delta: -40, Pct: "-33%", Improved: true},
			"load":        {Before: 1500, After: 1100, Delta: -400, Pct: "-27%", Improved: true},
			"transfer_kb": {Before: 768, After: 512, Delta: -256, Pct: "-33%", Improved: true},
			"requests":    {Before: 58, After: 42, Delta: -16, Pct: "-28%", Improved: true},
		},
		Resources: ResourceDiff{
			Removed: []RemovedResource{
				{URL: "/old-bundle.js", SizeBytes: 262144},
				{URL: "/legacy-polyfill.js", SizeBytes: 131072},
			},
		},
	}

	summary := GeneratePerfSummary(diff)

	if len(summary) > 200 {
		t.Errorf("Summary is %d chars, max 200. Got: %q", len(summary), summary)
	}
}

// ============================================
// Types: PageLoadMetrics and PerfDiff structs
// ============================================

func TestMetricDiff_Round(t *testing.T) {
	t.Parallel()
	// MetricDiff values should be rounded to avoid floating point noise
	fcp := 123.456789
	before := PageLoadMetrics{
		Timing: MetricsTiming{FCP: &fcp, TTFB: 80.123456, Load: 1000},
	}

	fcp2 := 100.654321
	after := PageLoadMetrics{
		Timing: MetricsTiming{FCP: &fcp2, TTFB: 70.987654, Load: 900},
	}

	diff := ComputePerfDiff(before, after)

	fcp_diff := diff.Metrics["fcp"]
	// Values should be rounded (no more than 1 decimal place for ms values)
	if fcp_diff.Before != math.Round(fcp_diff.Before*10)/10 {
		t.Errorf("fcp.Before not rounded: %v", fcp_diff.Before)
	}
}
