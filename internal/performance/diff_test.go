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

	// Should have metrics with "n/a" pct (no baseline to compare, but after values exist)
	if len(diff.Metrics) != 2 {
		t.Errorf("Expected 2 metrics (ttfb, load), got %d: %v", len(diff.Metrics), diff.Metrics)
	}
	if ttfb, ok := diff.Metrics["ttfb"]; ok {
		if ttfb.Pct != "n/a" {
			t.Errorf("TTFB pct should be 'n/a' when before=0, got %q", ttfb.Pct)
		}
	}
}

func TestComputePerfDiff_TTFBZeroNotSkipped(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 0, Load: 400},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 10, Load: 500},
	}

	diff := ComputePerfDiff(before, after)

	if _, ok := diff.Metrics["ttfb"]; !ok {
		t.Fatal("TTFB metric missing — TTFB=0 should not be skipped")
	}
	if diff.Metrics["ttfb"].Pct != "n/a" {
		t.Errorf("TTFB pct should be 'n/a' when before=0, got %q", diff.Metrics["ttfb"].Pct)
	}
}

func TestComputePerfDiff_BothZeroSkipped(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 0, Load: 400},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 0, Load: 500},
	}

	diff := ComputePerfDiff(before, after)

	if _, ok := diff.Metrics["ttfb"]; ok {
		t.Error("Both-zero TTFB should be skipped")
	}
	if _, ok := diff.Metrics["load"]; !ok {
		t.Error("Load metric should still be present")
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
// Verdict: top-level signal for LLM decision-making
// ============================================

func TestPerfDiff_Verdict_Improved(t *testing.T) {
	t.Parallel()
	lcp2800 := 2800.0
	lcp1200 := 1200.0

	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp2800, TTFB: 120, Load: 1500}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100}}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "improved" {
		t.Errorf("Verdict = %q, want 'improved' when all metrics improve", diff.Verdict)
	}
}

func TestPerfDiff_Verdict_Regressed(t *testing.T) {
	t.Parallel()
	lcp1200 := 1200.0
	lcp2800 := 2800.0

	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp2800, TTFB: 200, Load: 2500}}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "regressed" {
		t.Errorf("Verdict = %q, want 'regressed' when all metrics get worse", diff.Verdict)
	}
}

func TestPerfDiff_Verdict_Mixed(t *testing.T) {
	t.Parallel()
	lcp2800 := 2800.0
	lcp1200 := 1200.0

	// LCP improves, TTFB regresses
	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp2800, TTFB: 80, Load: 1100}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 200, Load: 1100}}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "mixed" {
		t.Errorf("Verdict = %q, want 'mixed' when some improve and some regress", diff.Verdict)
	}
}

func TestPerfDiff_Verdict_Unchanged(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{}
	after := PageLoadMetrics{}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "unchanged" {
		t.Errorf("Verdict = %q, want 'unchanged' when no metrics to compare", diff.Verdict)
	}
}

// ============================================
// Rating: Web Vitals thresholds for LLM context
// ============================================

func TestPerfDiff_LCP_Rating_Good(t *testing.T) {
	t.Parallel()
	lcp4000 := 4000.0
	lcp1200 := 1200.0

	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp4000, TTFB: 120, Load: 1500}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100}}

	diff := ComputePerfDiff(before, after)
	lcp := diff.Metrics["lcp"]
	if lcp.Rating != "good" {
		t.Errorf("LCP 1200ms rating = %q, want 'good' (<2500ms)", lcp.Rating)
	}
}

func TestPerfDiff_LCP_Rating_Poor(t *testing.T) {
	t.Parallel()
	lcp1200 := 1200.0
	lcp5000 := 5000.0

	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp5000, TTFB: 80, Load: 1100}}

	diff := ComputePerfDiff(before, after)
	lcp := diff.Metrics["lcp"]
	if lcp.Rating != "poor" {
		t.Errorf("LCP 5000ms rating = %q, want 'poor' (>4000ms)", lcp.Rating)
	}
}

func TestPerfDiff_CLS_Rating_NeedsImprovement(t *testing.T) {
	t.Parallel()
	cls01 := 0.01
	cls015 := 0.15

	before := PageLoadMetrics{
		CLS:   &cls01,
		Timing: MetricsTiming{TTFB: 80, Load: 1100},
	}
	after := PageLoadMetrics{
		CLS:   &cls015,
		Timing: MetricsTiming{TTFB: 80, Load: 1100},
	}

	diff := ComputePerfDiff(before, after)
	cls := diff.Metrics["cls"]
	if cls.Rating != "needs_improvement" {
		t.Errorf("CLS 0.15 rating = %q, want 'needs_improvement' (0.1-0.25)", cls.Rating)
	}
}

// ============================================
// Summary: percentage-based sort
// ============================================

func TestSummary_SortsByPercentageNotAbsoluteDelta(t *testing.T) {
	t.Parallel()
	// CLS has tiny absolute delta (0.2) but huge percentage (+200%)
	// TTFB has large absolute delta (100) but small percentage (+50%)
	// Summary should lead with CLS because percentage is bigger
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"cls":  {Before: 0.1, After: 0.3, Delta: 0.2, Pct: "+200%", Improved: false},
			"ttfb": {Before: 200, After: 300, Delta: 100, Pct: "+50%", Improved: false},
		},
	}

	summary := GeneratePerfSummary(diff)
	if !strings.HasPrefix(strings.ToUpper(summary), "CLS") {
		t.Errorf("Summary should lead with highest percentage (CLS +200%%), not highest delta (TTFB +100ms). Got: %q", summary)
	}
}

// ============================================
// Unit: metric values must carry units for LLM clarity
// ============================================

func TestPerfDiff_MetricUnit(t *testing.T) {
	t.Parallel()
	lcp2800 := 2800.0
	lcp1200 := 1200.0
	cls02 := 0.02
	cls01 := 0.01

	before := PageLoadMetrics{
		Timing:       MetricsTiming{LCP: &lcp2800, TTFB: 120, DomContentLoaded: 800, Load: 1500},
		CLS:          &cls02,
		TransferSize: 768 * 1024,
		RequestCount: 58,
	}
	after := PageLoadMetrics{
		Timing:       MetricsTiming{LCP: &lcp1200, TTFB: 80, DomContentLoaded: 700, Load: 1100},
		CLS:          &cls01,
		TransferSize: 512 * 1024,
		RequestCount: 42,
	}

	diff := ComputePerfDiff(before, after)

	checks := map[string]string{
		"lcp":                "ms",
		"ttfb":               "ms",
		"load":               "ms",
		"dom_content_loaded": "ms",
		"transfer_kb":        "KB",
		"requests":           "count",
	}
	for name, wantUnit := range checks {
		md, ok := diff.Metrics[name]
		if !ok {
			t.Errorf("metric %q missing", name)
			continue
		}
		if md.Unit != wantUnit {
			t.Errorf("%s.Unit = %q, want %q", name, md.Unit, wantUnit)
		}
	}
	// CLS is unitless — no unit string
	if diff.Metrics["cls"].Unit != "" {
		t.Errorf("cls.Unit = %q, want empty (unitless)", diff.Metrics["cls"].Unit)
	}
}

// ============================================
// Summary: no redundant sign, includes rating
// ============================================

func TestSummary_NoRedundantSign(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp": {Before: 2800, After: 1200, Delta: -1600, Pct: "-57%", Improved: true, Rating: "good"},
		},
	}
	summary := GeneratePerfSummary(diff)
	// "improved" already conveys direction — sign is redundant noise
	if strings.Contains(summary, "improved -") || strings.Contains(summary, "improved +") {
		t.Errorf("Summary has redundant sign after direction word. Got: %q", summary)
	}
}

func TestSummary_IncludesRating(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp": {Before: 4000, After: 1200, Delta: -2800, Pct: "-70%", Improved: true, Rating: "good"},
		},
	}
	summary := GeneratePerfSummary(diff)
	if !strings.Contains(summary, "good") {
		t.Errorf("Summary should include Web Vitals rating. Got: %q", summary)
	}
}

func TestSummary_RegressionShowsAbsolutePercentage(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp": {Before: 1200, After: 4000, Delta: 2800, Pct: "+233%", Improved: false, Rating: "poor"},
		},
	}
	summary := GeneratePerfSummary(diff)
	// Should say "regressed 233%" not "regressed +233%"
	if strings.Contains(summary, "regressed +") {
		t.Errorf("Summary has redundant + sign after 'regressed'. Got: %q", summary)
	}
	if !strings.Contains(summary, "233%") {
		t.Errorf("Summary should include percentage. Got: %q", summary)
	}
}

func TestSummary_DeltaZeroSaysUnchanged(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"load": {Before: 200, After: 200, Delta: 0, Pct: "+0%", Improved: false},
		},
	}
	summary := GeneratePerfSummary(diff)
	// delta=0 should NOT say "regressed" — it's unchanged
	if strings.Contains(strings.ToLower(summary), "regress") {
		t.Errorf("Summary says 'regressed' for delta=0, should say 'unchanged'. Got: %q", summary)
	}
}

func TestPerfDiff_DeltaZeroVerdict(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 80, Load: 200},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 80, Load: 200},
	}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "unchanged" {
		t.Errorf("Verdict = %q, want 'unchanged' when all deltas are 0", diff.Verdict)
	}
	// Summary should not claim regression
	if strings.Contains(strings.ToLower(diff.Summary), "regress") {
		t.Errorf("Summary claims regression for identical metrics. Got: %q", diff.Summary)
	}
}

// ============================================
// Types: PageLoadMetrics and PerfDiff structs
// ============================================

// ============================================
// SnapshotToPageLoadMetrics: type mapping
// ============================================

func TestSnapshotToPageLoadMetrics(t *testing.T) {
	t.Parallel()
	fcp := 900.0
	lcp := 2800.0
	cls := 0.15

	snap := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			TimeToFirstByte:        120,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			DomContentLoaded:       800,
			Load:                   1500,
		},
		Network: NetworkSummary{
			TransferSize: 768 * 1024,
			RequestCount: 58,
		},
		CLS: &cls,
	}

	m := SnapshotToPageLoadMetrics(snap)

	if m.URL != "/dashboard" {
		t.Errorf("URL = %q, want /dashboard", m.URL)
	}
	if m.Timing.TTFB != 120 {
		t.Errorf("TTFB = %v, want 120", m.Timing.TTFB)
	}
	if m.Timing.FCP == nil || *m.Timing.FCP != 900 {
		t.Errorf("FCP = %v, want 900", m.Timing.FCP)
	}
	if m.Timing.LCP == nil || *m.Timing.LCP != 2800 {
		t.Errorf("LCP = %v, want 2800", m.Timing.LCP)
	}
	if m.Timing.DomContentLoaded != 800 {
		t.Errorf("DCL = %v, want 800", m.Timing.DomContentLoaded)
	}
	if m.Timing.Load != 1500 {
		t.Errorf("Load = %v, want 1500", m.Timing.Load)
	}
	if m.CLS == nil || *m.CLS != 0.15 {
		t.Errorf("CLS = %v, want 0.15", m.CLS)
	}
	if m.TransferSize != 768*1024 {
		t.Errorf("TransferSize = %d, want %d", m.TransferSize, 768*1024)
	}
	if m.RequestCount != 58 {
		t.Errorf("RequestCount = %d, want 58", m.RequestCount)
	}
}

func TestSnapshotToPageLoadMetrics_NilOptionals(t *testing.T) {
	t.Parallel()
	snap := PerformanceSnapshot{
		URL: "/page",
		Timing: PerformanceTiming{
			TimeToFirstByte:        100,
			DomContentLoaded:       500,
			Load:                   1000,
			FirstContentfulPaint:   nil,
			LargestContentfulPaint: nil,
		},
		// CLS is nil
	}

	m := SnapshotToPageLoadMetrics(snap)

	if m.Timing.FCP != nil {
		t.Errorf("FCP should be nil, got %v", m.Timing.FCP)
	}
	if m.Timing.LCP != nil {
		t.Errorf("LCP should be nil, got %v", m.Timing.LCP)
	}
	if m.CLS != nil {
		t.Errorf("CLS should be nil, got %v", m.CLS)
	}
}

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
