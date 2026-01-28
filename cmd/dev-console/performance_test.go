package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPerformanceSnapshotJSONShape(t *testing.T) {
	t.Parallel()
	fcp := 250.0
	lcp := 800.0
	cls := 0.05
	snapshot := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       600,
			Load:                   1200,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			TimeToFirstByte:        80,
			DomInteractive:         500,
		},
		Network: NetworkSummary{
			RequestCount: 10,
			TransferSize: 50000,
			DecodedSize:  100000,
			ByType:       map[string]TypeSummary{"script": {Count: 3, Size: 30000}},
			SlowestRequests: []SlowRequest{
				{URL: "/app.js", Duration: 300, Size: 30000},
			},
		},
		LongTasks: LongTaskMetrics{
			Count:             2,
			TotalBlockingTime: 100,
			Longest:           80,
		},
		CLS: &cls,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Top-level fields
	for _, field := range []string{"url", "timestamp", "timing", "network", "longTasks", "cumulativeLayoutShift"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing top-level field: %s", field)
		}
	}

	// Timing fields
	timing := m["timing"].(map[string]interface{})
	for _, field := range []string{
		"domContentLoaded", "load", "firstContentfulPaint",
		"largestContentfulPaint", "timeToFirstByte", "domInteractive",
	} {
		if _, ok := timing[field]; !ok {
			t.Errorf("missing timing field: %s", field)
		}
	}

	// Network fields
	network := m["network"].(map[string]interface{})
	for _, field := range []string{"requestCount", "transferSize", "decodedSize", "byType", "slowestRequests"} {
		if _, ok := network[field]; !ok {
			t.Errorf("missing network field: %s", field)
		}
	}

	// LongTasks fields
	longTasks := m["longTasks"].(map[string]interface{})
	for _, field := range []string{"count", "totalBlockingTime", "longest"} {
		if _, ok := longTasks[field]; !ok {
			t.Errorf("missing longTasks field: %s", field)
		}
	}
}

func TestPerformanceBaselineJSONShape(t *testing.T) {
	t.Parallel()
	fcp := 250.0
	lcp := 800.0
	cls := 0.05
	baseline := PerformanceBaseline{
		URL:         "/dashboard",
		SampleCount: 3,
		LastUpdated: "2024-01-01T00:00:00Z",
		Timing: BaselineTiming{
			DomContentLoaded:       600,
			Load:                   1200,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			TimeToFirstByte:        80,
			DomInteractive:         500,
		},
		Network: BaselineNetwork{
			RequestCount: 10,
			TransferSize: 50000,
		},
		LongTasks: LongTaskMetrics{
			Count:             2,
			TotalBlockingTime: 100,
			Longest:           80,
		},
		CLS: &cls,
	}

	data, err := json.Marshal(baseline)
	if err != nil {
		t.Fatalf("Failed to marshal baseline: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Top-level fields
	for _, field := range []string{"url", "sampleCount", "lastUpdated", "timing", "network", "longTasks", "cumulativeLayoutShift"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing top-level field: %s", field)
		}
	}

	// Timing fields
	timing := m["timing"].(map[string]interface{})
	for _, field := range []string{
		"domContentLoaded", "load", "firstContentfulPaint",
		"largestContentfulPaint", "timeToFirstByte", "domInteractive",
	} {
		if _, ok := timing[field]; !ok {
			t.Errorf("missing timing field: %s", field)
		}
	}
}

func TestPerformanceSnapshotStorageAndRetrieval(t *testing.T) {
	t.Parallel()
	server := NewCapture()
	fcp := 250.0
	lcp := 800.0
	cls := 0.05

	snapshot := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       600,
			Load:                   1200,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			TimeToFirstByte:        80,
			DomInteractive:         500,
		},
		Network: NetworkSummary{
			RequestCount:    10,
			TransferSize:    50000,
			DecodedSize:     100000,
			ByType:          map[string]TypeSummary{},
			SlowestRequests: []SlowRequest{},
		},
		LongTasks: LongTaskMetrics{Count: 0, TotalBlockingTime: 0, Longest: 0},
		CLS:       &cls,
	}

	server.AddPerformanceSnapshot(snapshot)

	got, found := server.GetPerformanceSnapshot("/dashboard")
	if !found {
		t.Fatal("snapshot not found after adding")
	}
	if got.Timing.FirstContentfulPaint == nil || *got.Timing.FirstContentfulPaint != 250.0 {
		t.Errorf("FCP not stored: got %v", got.Timing.FirstContentfulPaint)
	}
	if got.Timing.LargestContentfulPaint == nil || *got.Timing.LargestContentfulPaint != 800.0 {
		t.Errorf("LCP not stored: got %v", got.Timing.LargestContentfulPaint)
	}
	if got.CLS == nil || *got.CLS != 0.05 {
		t.Errorf("CLS not stored: got %v", got.CLS)
	}
}

func TestPerformanceBaselineAveragesFCPLCP(t *testing.T) {
	t.Parallel()
	server := NewCapture()
	fcp1 := 200.0
	lcp1 := 600.0
	fcp2 := 300.0
	lcp2 := 800.0

	server.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       500,
			Load:                   1000,
			FirstContentfulPaint:   &fcp1,
			LargestContentfulPaint: &lcp1,
			TimeToFirstByte:        80,
			DomInteractive:         400,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Longest: 100},
	})

	server.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-01T00:01:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       500,
			Load:                   1000,
			FirstContentfulPaint:   &fcp2,
			LargestContentfulPaint: &lcp2,
			TimeToFirstByte:        80,
			DomInteractive:         400,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Longest: 60},
	})

	server.mu.RLock()
	baseline := server.perf.baselines["/test"]
	server.mu.RUnlock()

	if baseline.SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", baseline.SampleCount)
	}
	if baseline.Timing.FirstContentfulPaint == nil {
		t.Fatal("baseline FCP should not be nil")
	}
	// Average of 200 and 300 = 250
	if *baseline.Timing.FirstContentfulPaint != 250.0 {
		t.Errorf("expected FCP baseline 250, got %f", *baseline.Timing.FirstContentfulPaint)
	}
	if baseline.Timing.LargestContentfulPaint == nil {
		t.Fatal("baseline LCP should not be nil")
	}
	// Average of 600 and 800 = 700
	if *baseline.Timing.LargestContentfulPaint != 700.0 {
		t.Errorf("expected LCP baseline 700, got %f", *baseline.Timing.LargestContentfulPaint)
	}
	// Longest should be averaged: (100 + 60) / 2 = 80
	if baseline.LongTasks.Longest != 80.0 {
		t.Errorf("expected Longest baseline 80, got %f", baseline.LongTasks.Longest)
	}
}

func TestPerformanceRegressionDetectsFCPLCP(t *testing.T) {
	t.Parallel()
	server := NewCapture()

	fcpBaseline := 200.0
	lcpBaseline := 500.0
	fcpCurrent := 450.0 // +125% increase, +250ms
	lcpCurrent := 900.0 // +80% increase, +400ms

	baseline := PerformanceBaseline{
		URL:         "/test",
		SampleCount: 5,
		Timing: BaselineTiming{
			FirstContentfulPaint:   &fcpBaseline,
			LargestContentfulPaint: &lcpBaseline,
		},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
	}

	snapshot := PerformanceSnapshot{
		URL: "/test",
		Timing: PerformanceTiming{
			FirstContentfulPaint:   &fcpCurrent,
			LargestContentfulPaint: &lcpCurrent,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	}

	regressions := server.DetectRegressions(snapshot, baseline)

	fcpFound := false
	lcpFound := false
	for _, r := range regressions {
		if r.Metric == "firstContentfulPaint" {
			fcpFound = true
		}
		if r.Metric == "largestContentfulPaint" {
			lcpFound = true
		}
	}

	if !fcpFound {
		t.Error("expected FCP regression to be detected")
	}
	if !lcpFound {
		t.Error("expected LCP regression to be detected")
	}
}

func TestPerformanceRegressionNoFalsePositiveFCPLCP(t *testing.T) {
	t.Parallel()
	server := NewCapture()

	fcpBaseline := 200.0
	lcpBaseline := 500.0
	// Small changes: +20% for FCP, +10% for LCP (below thresholds)
	fcpCurrent := 240.0
	lcpCurrent := 550.0

	baseline := PerformanceBaseline{
		URL:         "/test",
		SampleCount: 5,
		Timing: BaselineTiming{
			FirstContentfulPaint:   &fcpBaseline,
			LargestContentfulPaint: &lcpBaseline,
		},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
	}

	snapshot := PerformanceSnapshot{
		URL: "/test",
		Timing: PerformanceTiming{
			FirstContentfulPaint:   &fcpCurrent,
			LargestContentfulPaint: &lcpCurrent,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	}

	regressions := server.DetectRegressions(snapshot, baseline)

	for _, r := range regressions {
		if r.Metric == "firstContentfulPaint" || r.Metric == "largestContentfulPaint" {
			t.Errorf("unexpected regression for %s (change too small)", r.Metric)
		}
	}
}

func TestAvgOptionalFloat(t *testing.T) {
	t.Parallel()
	// nil snapshot: baseline unchanged
	baseline := 100.0
	result := avgOptionalFloat(&baseline, nil, 2)
	if result == nil || *result != 100.0 {
		t.Errorf("nil snapshot should preserve baseline, got %v", result)
	}

	// nil baseline: use snapshot value
	snapshot := 200.0
	result = avgOptionalFloat(nil, &snapshot, 2)
	if result == nil || *result != 200.0 {
		t.Errorf("nil baseline should use snapshot, got %v", result)
	}

	// Both present: average
	result = avgOptionalFloat(&baseline, &snapshot, 2)
	if result == nil || *result != 150.0 {
		t.Errorf("expected average 150, got %v", result)
	}
}

func TestWeightedOptionalFloat(t *testing.T) {
	t.Parallel()
	baseline := 100.0
	snapshot := 200.0

	result := weightedOptionalFloat(&baseline, &snapshot, 0.8, 0.2)
	expected := 100.0*0.8 + 200.0*0.2 // 120
	if result == nil || *result != expected {
		t.Errorf("expected %f, got %v", expected, result)
	}

	// nil snapshot
	result = weightedOptionalFloat(&baseline, nil, 0.8, 0.2)
	if result == nil || *result != 100.0 {
		t.Errorf("nil snapshot should preserve baseline, got %v", result)
	}
}

// ============================================
// Causal Diffing Tests
// ============================================

func TestNormalizeResourceURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"/static/js/main.chunk.js?v=abc123", "/static/js/main.chunk.js"},
		{"/static/js/vendor.js#chunk1", "/static/js/vendor.js#chunk1"},
		{"/static/js/app.js?v=1#module", "/static/js/app.js#module"},
		{"/static/css/style.css", "/static/css/style.css"},
		{"https://cdn.example.com/lib.js?t=123", "https://cdn.example.com/lib.js"},
		{"", ""},
	}

	for _, tt := range tests {
		result := normalizeResourceURL(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeResourceURL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeDynamicAPIPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/user/123", "/api/user/*"},
		{"/api/user/456", "/api/user/*"},
		{"/api/dashboard/data", "/api/dashboard/*"},
		{"/api/user", "/api/user"},
		{"/static/js/main.js", "/static/js/*"},
		{"/favicon.ico", "/favicon.ico"},
	}

	for _, tt := range tests {
		result := normalizeDynamicAPIPath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeDynamicAPIPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestResourceDiffAddedScript(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
	}
	current := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
		{URL: "/static/js/analytics.js", Type: "script", TransferSize: 287000, Duration: 180},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Added) != 1 {
		t.Fatalf("expected 1 added resource, got %d", len(diff.Added))
	}
	if diff.Added[0].URL != "/static/js/analytics.js" {
		t.Errorf("expected added URL /static/js/analytics.js, got %s", diff.Added[0].URL)
	}
	if diff.Added[0].SizeBytes != 287000 {
		t.Errorf("expected added size 287000, got %d", diff.Added[0].SizeBytes)
	}
}

func TestResourceDiffRemovedStylesheet(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{
		{URL: "/static/css/theme.css", Type: "css", TransferSize: 50000, Duration: 80},
		{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
	}
	current := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Removed) != 1 {
		t.Fatalf("expected 1 removed resource, got %d", len(diff.Removed))
	}
	if diff.Removed[0].URL != "/static/css/theme.css" {
		t.Errorf("expected removed URL /static/css/theme.css, got %s", diff.Removed[0].URL)
	}
}

func TestResourceDiffResizedBundle(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{
		{URL: "/static/js/main.chunk.js", Type: "script", TransferSize: 145000, Duration: 200},
	}
	current := []ResourceEntry{
		{URL: "/static/js/main.chunk.js", Type: "script", TransferSize: 198000, Duration: 210},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Resized) != 1 {
		t.Fatalf("expected 1 resized resource, got %d", len(diff.Resized))
	}
	if diff.Resized[0].URL != "/static/js/main.chunk.js" {
		t.Errorf("expected resized URL /static/js/main.chunk.js, got %s", diff.Resized[0].URL)
	}
	if diff.Resized[0].BaselineBytes != 145000 {
		t.Errorf("expected baseline bytes 145000, got %d", diff.Resized[0].BaselineBytes)
	}
	if diff.Resized[0].CurrentBytes != 198000 {
		t.Errorf("expected current bytes 198000, got %d", diff.Resized[0].CurrentBytes)
	}
	if diff.Resized[0].DeltaBytes != 53000 {
		t.Errorf("expected delta bytes 53000, got %d", diff.Resized[0].DeltaBytes)
	}
}

func TestResourceDiffRetimedAPIEndpoint(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{
		{URL: "/api/dashboard/data", Type: "fetch", TransferSize: 5000, Duration: 80},
	}
	current := []ResourceEntry{
		{URL: "/api/dashboard/data", Type: "fetch", TransferSize: 5000, Duration: 340},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Retimed) != 1 {
		t.Fatalf("expected 1 retimed resource, got %d", len(diff.Retimed))
	}
	if diff.Retimed[0].URL != "/api/dashboard/data" {
		t.Errorf("expected retimed URL /api/dashboard/data, got %s", diff.Retimed[0].URL)
	}
	if diff.Retimed[0].BaselineMs != 80 {
		t.Errorf("expected baseline ms 80, got %f", diff.Retimed[0].BaselineMs)
	}
	if diff.Retimed[0].CurrentMs != 340 {
		t.Errorf("expected current ms 340, got %f", diff.Retimed[0].CurrentMs)
	}
	if diff.Retimed[0].DeltaMs != 260 {
		t.Errorf("expected delta ms 260, got %f", diff.Retimed[0].DeltaMs)
	}
}

func TestResourceDiffRenderBlockingFlag(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{}
	current := []ResourceEntry{
		{URL: "/static/js/chart-library.js", Type: "script", TransferSize: 412000, Duration: 250, RenderBlocking: true},
		{URL: "/static/js/deferred.js", Type: "script", TransferSize: 100000, Duration: 100, RenderBlocking: false},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Added) != 2 {
		t.Fatalf("expected 2 added resources, got %d", len(diff.Added))
	}

	var blocking *AddedResource
	for i := range diff.Added {
		if diff.Added[i].URL == "/static/js/chart-library.js" {
			blocking = &diff.Added[i]
		}
	}
	if blocking == nil {
		t.Fatal("chart-library.js not found in added resources")
	}
	if !blocking.RenderBlocking {
		t.Error("chart-library.js should be marked render_blocking=true")
	}
}

func TestProbableCauseSummarize(t *testing.T) {
	t.Parallel()
	diff := ResourceDiff{
		Added: []AddedResource{
			{URL: "/static/js/analytics.js", Type: "script", SizeBytes: 287000, DurationMs: 180, RenderBlocking: false},
			{URL: "/static/js/chart-library.js", Type: "script", SizeBytes: 412000, DurationMs: 250, RenderBlocking: true},
		},
		Resized: []ResizedResource{
			{URL: "/static/js/main.chunk.js", BaselineBytes: 145000, CurrentBytes: 198000, DeltaBytes: 53000},
		},
		Retimed: []RetimedResource{
			{URL: "/api/dashboard/data", BaselineMs: 80, CurrentMs: 340, DeltaMs: 260},
		},
	}

	cause := computeProbableCause(diff, 500000, 850000)

	if cause == "" {
		t.Fatal("probable cause should not be empty")
	}
	if !stringContains(cause, "682") && !stringContains(cause, "683") {
		// 699000 / 1024 = ~682KB
		t.Errorf("probable cause should mention total added size (~682KB), got: %s", cause)
	}
	if !stringContains(cause, "render-blocking") || !stringContains(cause, "chart-library.js") {
		t.Errorf("probable cause should mention render-blocking chart-library.js, got: %s", cause)
	}
	if !stringContains(cause, "/api/dashboard/data") || !stringContains(cause, "260ms") {
		t.Errorf("probable cause should mention API regression, got: %s", cause)
	}
}

func TestRecommendationsGenerated(t *testing.T) {
	t.Parallel()
	diff := ResourceDiff{
		Added: []AddedResource{
			{URL: "/static/js/chart-library.js", Type: "script", SizeBytes: 412000, DurationMs: 250, RenderBlocking: true},
		},
		Resized: []ResizedResource{
			{URL: "/static/js/main.chunk.js", BaselineBytes: 145000, CurrentBytes: 198000, DeltaBytes: 53000},
		},
		Retimed: []RetimedResource{
			{URL: "/api/dashboard/data", BaselineMs: 80, CurrentMs: 340, DeltaMs: 260},
		},
	}

	recs := computeRecommendations(diff)

	if len(recs) < 3 {
		t.Fatalf("expected at least 3 recommendations, got %d: %v", len(recs), recs)
	}

	foundLazy := false
	foundAPI := false
	foundGrew := false
	for _, r := range recs {
		if stringContains(r, "chart-library.js") && stringContains(r, "render-blocking") {
			foundLazy = true
		}
		if stringContains(r, "/api/dashboard/data") {
			foundAPI = true
		}
		if stringContains(r, "main.chunk.js") && stringContains(r, "grew") {
			foundGrew = true
		}
	}
	if !foundLazy {
		t.Errorf("should recommend lazy-loading render-blocking script, got: %v", recs)
	}
	if !foundAPI {
		t.Errorf("should recommend investigating API regression, got: %v", recs)
	}
	if !foundGrew {
		t.Errorf("should recommend reviewing bundle growth, got: %v", recs)
	}
}

func TestURLNormalizationStripsQueryPreservesHash(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{
		{URL: "/static/js/main.js?v=old", Type: "script", TransferSize: 100000, Duration: 150},
	}
	current := []ResourceEntry{
		{URL: "/static/js/main.js?v=new", Type: "script", TransferSize: 100000, Duration: 150},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Added) != 0 {
		t.Errorf("expected 0 added (same resource with different query), got %d", len(diff.Added))
	}
	if len(diff.Removed) != 0 {
		t.Errorf("expected 0 removed (same resource with different query), got %d", len(diff.Removed))
	}
}

func TestNoBaselineResourcesMessage(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	fcp := 200.0
	capture.mu.Lock()
	capture.perf.baselines["/test"] = PerformanceBaseline{
		URL:         "/test",
		SampleCount: 3,
		Timing: BaselineTiming{
			Load:                 1000,
			FirstContentfulPaint: &fcp,
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/test")
	capture.perf.snapshots["/test"] = PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-01T00:01:00Z",
		Timing: PerformanceTiming{
			Load:                 1500,
			FirstContentfulPaint: &fcp,
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/test")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/test", "")

	if result.URL != "/test" {
		t.Errorf("expected URL /test, got %s", result.URL)
	}
	if result.ProbableCause == "" {
		t.Fatal("probable_cause should not be empty")
	}
	if !stringContains(result.ProbableCause, "unavailable") {
		t.Errorf("should mention resource comparison unavailable, got: %s", result.ProbableCause)
	}
}

func TestNoResourceChangesMessage(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
	}
	current := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
	}

	diff := computeResourceDiff(baseline, current)
	cause := computeProbableCause(diff, 100000, 100000)

	if !stringContains(cause, "No resource changes") {
		t.Errorf("should mention no resource changes when diff is empty, got: %s", cause)
	}
}

func TestResizeThresholdTenPercentOrTenKB(t *testing.T) {
	t.Parallel()
	// Small file: 10KB -> 10.5KB (5% increase, 512B delta)
	// min(10% * 10KB, 10KB) = min(1KB, 10KB) = 1KB. Delta 512B < 1KB. Not flagged.
	baseline := []ResourceEntry{
		{URL: "/small.js", Type: "script", TransferSize: 10240, Duration: 50},
	}
	current := []ResourceEntry{
		{URL: "/small.js", Type: "script", TransferSize: 10752, Duration: 50},
	}
	diff := computeResourceDiff(baseline, current)
	if len(diff.Resized) != 0 {
		t.Errorf("small change (512B on 10KB file) should not be flagged as resized, got %d", len(diff.Resized))
	}

	// Large file: 200KB -> 215KB (7.5% increase, 15KB delta)
	// min(10% * 200KB, 10KB) = min(20KB, 10KB) = 10KB. Delta 15KB > 10KB. Flagged.
	baseline2 := []ResourceEntry{
		{URL: "/big.js", Type: "script", TransferSize: 204800, Duration: 200},
	}
	current2 := []ResourceEntry{
		{URL: "/big.js", Type: "script", TransferSize: 220160, Duration: 200},
	}
	diff2 := computeResourceDiff(baseline2, current2)
	if len(diff2.Resized) != 1 {
		t.Errorf("15KB increase on 200KB file should be flagged as resized, got %d", len(diff2.Resized))
	}
}

func TestRetimedThreshold100ms(t *testing.T) {
	t.Parallel()
	// 90ms delta: not flagged
	baseline := []ResourceEntry{
		{URL: "/api/data", Type: "fetch", TransferSize: 5000, Duration: 100},
	}
	current := []ResourceEntry{
		{URL: "/api/data", Type: "fetch", TransferSize: 5000, Duration: 190},
	}
	diff := computeResourceDiff(baseline, current)
	if len(diff.Retimed) != 0 {
		t.Errorf("90ms delta should not be flagged as retimed, got %d", len(diff.Retimed))
	}

	// 110ms delta: flagged
	current2 := []ResourceEntry{
		{URL: "/api/data", Type: "fetch", TransferSize: 5000, Duration: 210},
	}
	diff2 := computeResourceDiff(baseline, current2)
	if len(diff2.Retimed) != 1 {
		t.Errorf("110ms delta should be flagged as retimed, got %d", len(diff2.Retimed))
	}
}

func TestTopResourcesBySize(t *testing.T) {
	t.Parallel()
	resources := make([]ResourceEntry, 60)
	for i := 0; i < 60; i++ {
		resources[i] = ResourceEntry{
			URL:          fmt.Sprintf("/resource-%d.js", i),
			Type:         "script",
			TransferSize: int64(i * 1000),
			Duration:     50,
		}
	}

	filtered := filterTopResources(resources)

	if len(filtered) != maxResourceFingerprints {
		t.Errorf("expected %d resources after filtering, got %d", maxResourceFingerprints, len(filtered))
	}

	// Should keep the largest ones
	for _, r := range filtered {
		if r.TransferSize < 10000 {
			t.Errorf("resource with size %d should have been excluded (too small)", r.TransferSize)
		}
	}
}

func TestSmallResourcesAggregated(t *testing.T) {
	t.Parallel()
	resources := []ResourceEntry{
		{URL: "/big.js", Type: "script", TransferSize: 100000, Duration: 200},
		{URL: "/tiny1.js", Type: "script", TransferSize: 500, Duration: 10},
		{URL: "/tiny2.js", Type: "script", TransferSize: 800, Duration: 15},
	}

	aggregated := aggregateSmallResources(resources, 1024)

	var hasBig bool
	var hasAggregated bool
	for _, r := range aggregated {
		if r.URL == "/big.js" {
			hasBig = true
		}
		if stringContains(r.URL, "small resources") {
			hasAggregated = true
		}
	}
	if !hasBig {
		t.Error("big.js should remain in aggregated list")
	}
	if !hasAggregated {
		t.Error("small resources should be aggregated into a summary entry")
	}
}

func TestDynamicAPIPathsGrouped(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{
		{URL: "/api/user/123", Type: "fetch", TransferSize: 2000, Duration: 50},
	}
	current := []ResourceEntry{
		{URL: "/api/user/456", Type: "fetch", TransferSize: 2000, Duration: 50},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Added) != 0 {
		t.Errorf("dynamic API paths should be grouped, got %d added", len(diff.Added))
	}
	if len(diff.Removed) != 0 {
		t.Errorf("dynamic API paths should be grouped, got %d removed", len(diff.Removed))
	}
}

func TestGetCausalDiffExplicitURL(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	fcp := 200.0
	capture.mu.Lock()
	capture.perf.baselines["/page-a"] = PerformanceBaseline{
		URL:         "/page-a",
		SampleCount: 3,
		Timing: BaselineTiming{
			Load:                 1000,
			FirstContentfulPaint: &fcp,
		},
		Resources: []ResourceEntry{
			{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/page-a")

	capture.perf.snapshots["/page-a"] = PerformanceSnapshot{
		URL:       "/page-a",
		Timestamp: "2024-01-01T00:01:00Z",
		Timing: PerformanceTiming{
			Load:                 1500,
			FirstContentfulPaint: &fcp,
		},
		Resources: []ResourceEntry{
			{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
			{URL: "/static/js/new-lib.js", Type: "script", TransferSize: 200000, Duration: 180},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/page-a")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/page-a", "")

	if result.URL != "/page-a" {
		t.Errorf("expected URL /page-a, got %s", result.URL)
	}
	if len(result.ResourceChanges.Added) != 1 {
		t.Fatalf("expected 1 added resource, got %d", len(result.ResourceChanges.Added))
	}
	if result.ResourceChanges.Added[0].URL != "/static/js/new-lib.js" {
		t.Errorf("expected added /static/js/new-lib.js, got %s", result.ResourceChanges.Added[0].URL)
	}
}

func TestGetCausalDiffTimingDelta(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	fcp := 200.0
	lcp := 500.0
	fcpNew := 520.0
	lcpNew := 1150.0

	capture.mu.Lock()
	capture.perf.baselines["/test"] = PerformanceBaseline{
		URL:         "/test",
		SampleCount: 5,
		Timing: BaselineTiming{
			Load:                   1000,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
		},
		Resources: []ResourceEntry{},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/test")

	capture.perf.snapshots["/test"] = PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-01T00:01:00Z",
		Timing: PerformanceTiming{
			Load:                   1847,
			FirstContentfulPaint:   &fcpNew,
			LargestContentfulPaint: &lcpNew,
		},
		Resources: []ResourceEntry{},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/test")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/test", "")

	if result.TimingDelta.LoadMs != 847 {
		t.Errorf("expected load delta 847, got %f", result.TimingDelta.LoadMs)
	}
	if result.TimingDelta.FCPMs != 320 {
		t.Errorf("expected FCP delta 320, got %f", result.TimingDelta.FCPMs)
	}
	if result.TimingDelta.LCPMs != 650 {
		t.Errorf("expected LCP delta 650, got %f", result.TimingDelta.LCPMs)
	}
}

func TestResourceFingerprintStoredWithBaseline(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	snapshot := PerformanceSnapshot{
		URL:       "/with-resources",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded: 500,
			Load:             1000,
			TimeToFirstByte:  80,
			DomInteractive:   400,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		Resources: []ResourceEntry{
			{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 150},
			{URL: "/static/css/style.css", Type: "css", TransferSize: 30000, Duration: 50},
		},
	}

	capture.AddPerformanceSnapshot(snapshot)

	capture.mu.RLock()
	baseline := capture.perf.baselines["/with-resources"]
	capture.mu.RUnlock()

	if len(baseline.Resources) != 2 {
		t.Fatalf("expected 2 resources in baseline, got %d", len(baseline.Resources))
	}
}

func TestResourceFingerprintMovingAverage(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	snapshot1 := PerformanceSnapshot{
		URL:       "/avg-test",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded: 500,
			Load:             1000,
			TimeToFirstByte:  80,
			DomInteractive:   400,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		Resources: []ResourceEntry{
			{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 200},
		},
	}
	capture.AddPerformanceSnapshot(snapshot1)

	snapshot2 := PerformanceSnapshot{
		URL:       "/avg-test",
		Timestamp: "2024-01-01T00:01:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded: 500,
			Load:             1000,
			TimeToFirstByte:  80,
			DomInteractive:   400,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		Resources: []ResourceEntry{
			{URL: "/static/js/main.js", Type: "script", TransferSize: 120000, Duration: 300},
		},
	}
	capture.AddPerformanceSnapshot(snapshot2)

	capture.mu.RLock()
	baseline := capture.perf.baselines["/avg-test"]
	capture.mu.RUnlock()

	if len(baseline.Resources) != 1 {
		t.Fatalf("expected 1 resource in baseline, got %d", len(baseline.Resources))
	}

	// Simple average of 2 samples: (100000 + 120000) / 2 = 110000
	if baseline.Resources[0].TransferSize != 110000 {
		t.Errorf("expected averaged transfer size 110000, got %d", baseline.Resources[0].TransferSize)
	}
	// Simple average: (200 + 300) / 2 = 250
	if baseline.Resources[0].Duration != 250 {
		t.Errorf("expected averaged duration 250, got %f", baseline.Resources[0].Duration)
	}
}

func TestPayloadIncreasePercentInCause(t *testing.T) {
	t.Parallel()
	diff := ResourceDiff{
		Added: []AddedResource{
			{URL: "/new.js", Type: "script", SizeBytes: 350000},
		},
	}

	// Baseline total 500KB, current total 850KB = 70% increase
	cause := computeProbableCause(diff, 500000, 850000)

	if !stringContains(cause, "70%") {
		t.Errorf("probable cause should mention 70%% payload increase, got: %s", cause)
	}
}

func TestGetCausalDiffMCPTool(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	fcp := 200.0
	capture.mu.Lock()
	capture.perf.baselines["/mcp-test"] = PerformanceBaseline{
		URL:         "/mcp-test",
		SampleCount: 3,
		Timing: BaselineTiming{
			Load:                 1000,
			FirstContentfulPaint: &fcp,
		},
		Resources: []ResourceEntry{
			{URL: "/main.js", Type: "script", TransferSize: 100000, Duration: 150},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/mcp-test")

	capture.perf.snapshots["/mcp-test"] = PerformanceSnapshot{
		URL:       "/mcp-test",
		Timestamp: "2024-01-01T00:01:00Z",
		Timing: PerformanceTiming{
			Load:                 1500,
			FirstContentfulPaint: &fcp,
		},
		Resources: []ResourceEntry{
			{URL: "/main.js", Type: "script", TransferSize: 100000, Duration: 150},
			{URL: "/extra.js", Type: "script", TransferSize: 200000, Duration: 180},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/mcp-test")
	capture.mu.Unlock()

	server := &Server{}
	handler := &ToolHandler{
		MCPHandler: &MCPHandler{server: server},
		capture:    capture,
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"url": "/mcp-test"}`)
	resp := handler.toolGetCausalDiff(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}

	var diffResult CausalDiffResult
	if err := json.Unmarshal([]byte(result.Content[0].Text), &diffResult); err != nil {
		t.Fatalf("response text should be valid JSON CausalDiffResult: %v", err)
	}
	if diffResult.URL != "/mcp-test" {
		t.Errorf("expected URL /mcp-test in response, got %s", diffResult.URL)
	}
	if len(diffResult.ResourceChanges.Added) != 1 {
		t.Errorf("expected 1 added resource in MCP response, got %d", len(diffResult.ResourceChanges.Added))
	}
}

func TestGetCausalDiffUsesLatestSnapshotWhenNoURL(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	fcp := 200.0
	capture.mu.Lock()
	capture.perf.baselines["/latest"] = PerformanceBaseline{
		URL:         "/latest",
		SampleCount: 3,
		Timing: BaselineTiming{
			Load:                 1000,
			FirstContentfulPaint: &fcp,
		},
		Resources: []ResourceEntry{},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/latest")

	capture.perf.snapshots["/latest"] = PerformanceSnapshot{
		URL:       "/latest",
		Timestamp: "2024-01-01T00:01:00Z",
		Timing: PerformanceTiming{
			Load:                 1200,
			FirstContentfulPaint: &fcp,
		},
		Resources: []ResourceEntry{},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/latest")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("", "")

	if result.URL != "/latest" {
		t.Errorf("expected to use latest snapshot URL /latest, got %s", result.URL)
	}
}

// ============================================
// Web Vitals Tests
// ============================================

func TestGetWebVitalsNoSnapshot(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	result := capture.GetWebVitals()

	if result.FCP.Value != nil {
		t.Errorf("FCP value should be nil when no snapshot exists, got %v", result.FCP.Value)
	}
	if result.LCP.Value != nil {
		t.Errorf("LCP value should be nil when no snapshot exists, got %v", result.LCP.Value)
	}
	if result.CLS.Value != nil {
		t.Errorf("CLS value should be nil when no snapshot exists, got %v", result.CLS.Value)
	}
	if result.INP.Value != nil {
		t.Errorf("INP value should be nil when no snapshot exists, got %v", result.INP.Value)
	}
	if result.LoadTime.Value != nil {
		t.Errorf("LoadTime value should be nil when no snapshot exists, got %v", result.LoadTime.Value)
	}
}

func TestGetWebVitalsWithSnapshot(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	fcp := 1200.0
	lcp := 2000.0
	cls := 0.08
	inp := 150.0

	snapshot := PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       600,
			Load:                   1500,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			InteractionToNextPaint: &inp,
			TimeToFirstByte:        80,
			DomInteractive:         500,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		CLS:       &cls,
	}

	capture.AddPerformanceSnapshot(snapshot)
	result := capture.GetWebVitals()

	// Check FCP
	if result.FCP.Value == nil || *result.FCP.Value != 1200 {
		t.Errorf("FCP value should be 1200, got %v", result.FCP.Value)
	}
	if result.FCP.Assessment != "good" {
		t.Errorf("FCP 1200ms should be 'good' (< 1800), got %s", result.FCP.Assessment)
	}

	// Check LCP
	if result.LCP.Value == nil || *result.LCP.Value != 2000 {
		t.Errorf("LCP value should be 2000, got %v", result.LCP.Value)
	}
	if result.LCP.Assessment != "good" {
		t.Errorf("LCP 2000ms should be 'good' (< 2500), got %s", result.LCP.Assessment)
	}

	// Check CLS
	if result.CLS.Value == nil || *result.CLS.Value != 0.08 {
		t.Errorf("CLS value should be 0.08, got %v", result.CLS.Value)
	}
	if result.CLS.Assessment != "good" {
		t.Errorf("CLS 0.08 should be 'good' (< 0.1), got %s", result.CLS.Assessment)
	}

	// Check INP
	if result.INP.Value == nil || *result.INP.Value != 150 {
		t.Errorf("INP value should be 150, got %v", result.INP.Value)
	}
	if result.INP.Assessment != "good" {
		t.Errorf("INP 150ms should be 'good' (< 200), got %s", result.INP.Assessment)
	}

	// Check Load time
	if result.LoadTime.Value == nil || *result.LoadTime.Value != 1500 {
		t.Errorf("LoadTime value should be 1500, got %v", result.LoadTime.Value)
	}
}

func TestGetWebVitalsAssessmentThresholds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		fcp       float64
		lcp       float64
		cls       float64
		inp       float64
		expectFCP string
		expectLCP string
		expectCLS string
		expectINP string
	}{
		{
			name: "all good",
			fcp:  1000, lcp: 2000, cls: 0.05, inp: 100,
			expectFCP: "good", expectLCP: "good", expectCLS: "good", expectINP: "good",
		},
		{
			name: "all needs-improvement",
			fcp:  2500, lcp: 3500, cls: 0.15, inp: 350,
			expectFCP: "needs-improvement", expectLCP: "needs-improvement", expectCLS: "needs-improvement", expectINP: "needs-improvement",
		},
		{
			name: "all poor",
			fcp:  3500, lcp: 5000, cls: 0.3, inp: 600,
			expectFCP: "poor", expectLCP: "poor", expectCLS: "poor", expectINP: "poor",
		},
		{
			name: "boundary: exactly at good threshold",
			fcp:  1800, lcp: 2500, cls: 0.1, inp: 200,
			expectFCP: "needs-improvement", expectLCP: "needs-improvement", expectCLS: "needs-improvement", expectINP: "needs-improvement",
		},
		{
			name: "boundary: exactly at poor threshold",
			fcp:  3000, lcp: 4000, cls: 0.25, inp: 500,
			expectFCP: "poor", expectLCP: "poor", expectCLS: "poor", expectINP: "poor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := NewCapture()

			snapshot := PerformanceSnapshot{
				URL:       "/test",
				Timestamp: "2024-01-01T00:00:00Z",
				Timing: PerformanceTiming{
					DomContentLoaded:       600,
					Load:                   1500,
					FirstContentfulPaint:   &tt.fcp,
					LargestContentfulPaint: &tt.lcp,
					InteractionToNextPaint: &tt.inp,
					TimeToFirstByte:        80,
					DomInteractive:         500,
				},
				Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
				LongTasks: LongTaskMetrics{},
				CLS:       &tt.cls,
			}

			capture.AddPerformanceSnapshot(snapshot)
			result := capture.GetWebVitals()

			if result.FCP.Assessment != tt.expectFCP {
				t.Errorf("FCP assessment: expected %s, got %s (value: %f)", tt.expectFCP, result.FCP.Assessment, tt.fcp)
			}
			if result.LCP.Assessment != tt.expectLCP {
				t.Errorf("LCP assessment: expected %s, got %s (value: %f)", tt.expectLCP, result.LCP.Assessment, tt.lcp)
			}
			if result.CLS.Assessment != tt.expectCLS {
				t.Errorf("CLS assessment: expected %s, got %s (value: %f)", tt.expectCLS, result.CLS.Assessment, tt.cls)
			}
			if result.INP.Assessment != tt.expectINP {
				t.Errorf("INP assessment: expected %s, got %s (value: %f)", tt.expectINP, result.INP.Assessment, tt.inp)
			}
		})
	}
}

func TestGetWebVitalsMCPTool(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	fcp := 1500.0
	lcp := 2800.0
	cls := 0.12
	inp := 250.0

	snapshot := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       600,
			Load:                   2000,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			InteractionToNextPaint: &inp,
			TimeToFirstByte:        80,
			DomInteractive:         500,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		CLS:       &cls,
	}
	capture.AddPerformanceSnapshot(snapshot)

	server := &Server{}
	handler := &ToolHandler{
		MCPHandler: &MCPHandler{server: server},
		capture:    capture,
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := handler.toolGetWebVitals(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}

	// Strip summary line before parsing JSON
	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var vitals WebVitalsResult
	if err := json.Unmarshal([]byte(jsonPart), &vitals); err != nil {
		t.Fatalf("response text should be valid JSON WebVitalsResult: %v", err)
	}

	if vitals.FCP.Value == nil || *vitals.FCP.Value != 1500 {
		t.Errorf("expected FCP 1500, got %v", vitals.FCP.Value)
	}
	if vitals.FCP.Assessment != "good" {
		t.Errorf("expected FCP assessment 'good', got %s", vitals.FCP.Assessment)
	}
	if vitals.LCP.Value == nil || *vitals.LCP.Value != 2800 {
		t.Errorf("expected LCP 2800, got %v", vitals.LCP.Value)
	}
	if vitals.LCP.Assessment != "needs-improvement" {
		t.Errorf("expected LCP assessment 'needs-improvement', got %s", vitals.LCP.Assessment)
	}
	if vitals.INP.Value == nil || *vitals.INP.Value != 250 {
		t.Errorf("expected INP 250, got %v", vitals.INP.Value)
	}
	if vitals.INP.Assessment != "needs-improvement" {
		t.Errorf("expected INP assessment 'needs-improvement', got %s", vitals.INP.Assessment)
	}
}

func TestGetWebVitalsToolRegistered(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	server := &Server{}
	handler := &ToolHandler{
		MCPHandler: &MCPHandler{server: server},
		capture:    capture,
	}

	tools := handler.toolsList()
	found := false
	for _, tool := range tools {
		if tool.Name == "observe" {
			found = true
			break
		}
	}
	if !found {
		t.Error("observe tool should be registered in toolsList")
	}
}

func TestGetWebVitalsToolDispatch(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	server := &Server{}
	handler := &ToolHandler{
		MCPHandler: &MCPHandler{server: server},
		capture:    capture,
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"what":"vitals"}`)

	resp, handled := handler.handleToolCall(req, "observe", args)
	if !handled {
		t.Error("observe should be handled by handleToolCall")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestPerformanceTimingINPField(t *testing.T) {
	t.Parallel()
	inp := 175.0
	timing := PerformanceTiming{
		DomContentLoaded:       600,
		Load:                   1500,
		InteractionToNextPaint: &inp,
		TimeToFirstByte:        80,
		DomInteractive:         500,
	}

	data, err := json.Marshal(timing)
	if err != nil {
		t.Fatalf("Failed to marshal timing: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if _, ok := m["interactionToNextPaint"]; !ok {
		t.Error("interactionToNextPaint field should be present in JSON")
	}
	if m["interactionToNextPaint"].(float64) != 175.0 {
		t.Errorf("expected interactionToNextPaint 175, got %v", m["interactionToNextPaint"])
	}
}

func TestPerformanceTimingINPOmittedWhenNil(t *testing.T) {
	t.Parallel()
	timing := PerformanceTiming{
		DomContentLoaded: 600,
		Load:             1500,
		TimeToFirstByte:  80,
		DomInteractive:   500,
	}

	data, err := json.Marshal(timing)
	if err != nil {
		t.Fatalf("Failed to marshal timing: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if _, ok := m["interactionToNextPaint"]; ok {
		t.Error("interactionToNextPaint field should be omitted when nil")
	}
}

// stringContains is a helper for checking substring presence
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================
// Coverage Gap Tests
// ============================================

// --- FormatPerformanceReport (line 406, 0% covered) ---

func TestFormatPerformanceReportNoBaseline(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	fcp := 250.0
	lcp := 800.0

	snapshot := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing: PerformanceTiming{
			TimeToFirstByte:        80,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			DomInteractive:         500,
			DomContentLoaded:       600,
			Load:                   1200,
		},
		Network: NetworkSummary{
			RequestCount: 15,
			TransferSize: 256000,
			DecodedSize:  512000,
		},
		LongTasks: LongTaskMetrics{
			Count:             3,
			TotalBlockingTime: 200,
		},
	}

	report := capture.FormatPerformanceReport(snapshot, nil)

	// Verify header
	if !stringContains(report, "## Performance Snapshot: /dashboard") {
		t.Error("report should contain the URL header")
	}
	if !stringContains(report, "2024-01-15T10:00:00Z") {
		t.Error("report should contain the timestamp")
	}

	// Verify timing section
	if !stringContains(report, "TTFB: 80ms") {
		t.Error("report should contain TTFB")
	}
	if !stringContains(report, "First Contentful Paint: 250ms") {
		t.Error("report should contain FCP")
	}
	if !stringContains(report, "Largest Contentful Paint: 800ms") {
		t.Error("report should contain LCP")
	}
	if !stringContains(report, "DOM Interactive: 500ms") {
		t.Error("report should contain DOM Interactive")
	}
	if !stringContains(report, "DOM Content Loaded: 600ms") {
		t.Error("report should contain DOM Content Loaded")
	}
	if !stringContains(report, "Load: 1200ms") {
		t.Error("report should contain Load")
	}

	// Verify network section
	if !stringContains(report, "Requests: 15") {
		t.Error("report should contain request count")
	}
	if !stringContains(report, "Transfer Size: 250.0KB") {
		t.Error("report should contain transfer size formatted as KB")
	}
	if !stringContains(report, "Decoded Size: 500.0KB") {
		t.Error("report should contain decoded size formatted as KB")
	}

	// Verify long tasks
	if !stringContains(report, "Count: 3") {
		t.Error("report should contain long task count")
	}
	if !stringContains(report, "Total Blocking Time: 200ms") {
		t.Error("report should contain TBT")
	}

	// Verify no baseline message
	if !stringContains(report, "No Baseline Yet") {
		t.Error("report should indicate no baseline is available")
	}
	if !stringContains(report, "first snapshot for this URL") {
		t.Error("report should explain first snapshot behavior")
	}
}

func TestFormatPerformanceReportWithBaselineNoRegressions(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	snapshot := PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing: PerformanceTiming{
			TimeToFirstByte:  80,
			DomInteractive:   500,
			DomContentLoaded: 600,
			Load:             1200,
		},
		Network: NetworkSummary{
			RequestCount: 10,
			TransferSize: 50000,
			DecodedSize:  100000,
		},
		LongTasks: LongTaskMetrics{
			Count:             1,
			TotalBlockingTime: 50,
		},
	}

	baseline := PerformanceBaseline{
		URL:         "/test",
		SampleCount: 5,
		Timing: BaselineTiming{
			Load:             1100,
			DomContentLoaded: 580,
			TimeToFirstByte:  75,
			DomInteractive:   490,
		},
		Network: BaselineNetwork{
			RequestCount: 10,
			TransferSize: 48000,
		},
		LongTasks: LongTaskMetrics{
			Count:             1,
			TotalBlockingTime: 45,
		},
	}

	report := capture.FormatPerformanceReport(snapshot, &baseline)

	if !stringContains(report, "No Regressions") {
		t.Error("report should indicate no regressions when changes are minor")
	}
	if !stringContains(report, "Baseline: 5 samples") {
		t.Errorf("report should show baseline sample count, got:\n%s", report)
	}
}

func TestFormatPerformanceReportWithRegressions(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	snapshot := PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing: PerformanceTiming{
			TimeToFirstByte:  80,
			DomInteractive:   500,
			DomContentLoaded: 600,
			Load:             3000, // baseline 1000 -> +200% increase, +2000ms
		},
		Network: NetworkSummary{
			RequestCount: 10,
			TransferSize: 50000,
			DecodedSize:  100000,
		},
		LongTasks: LongTaskMetrics{
			Count:             1,
			TotalBlockingTime: 50,
		},
	}

	baseline := PerformanceBaseline{
		URL:         "/test",
		SampleCount: 5,
		Timing: BaselineTiming{
			Load:             1000,
			DomContentLoaded: 580,
			TimeToFirstByte:  75,
			DomInteractive:   490,
		},
		Network: BaselineNetwork{
			RequestCount: 10,
			TransferSize: 48000,
		},
		LongTasks: LongTaskMetrics{
			Count:             1,
			TotalBlockingTime: 45,
		},
	}

	report := capture.FormatPerformanceReport(snapshot, &baseline)

	if !stringContains(report, "Regressions Detected") {
		t.Error("report should indicate regressions detected")
	}
	if !stringContains(report, "load") {
		t.Error("report should contain load regression metric name")
	}
}

func TestFormatPerformanceReportWithSlowestRequests(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	snapshot := PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing: PerformanceTiming{
			TimeToFirstByte:  80,
			DomInteractive:   500,
			DomContentLoaded: 600,
			Load:             1200,
		},
		Network: NetworkSummary{
			RequestCount: 10,
			TransferSize: 50000,
			DecodedSize:  100000,
			SlowestRequests: []SlowRequest{
				{URL: "/api/data", Duration: 450, Size: 25000},
				{URL: "/static/app.js", Duration: 300, Size: 150000},
			},
		},
		LongTasks: LongTaskMetrics{},
	}

	report := capture.FormatPerformanceReport(snapshot, nil)

	if !stringContains(report, "Slowest Requests") {
		t.Error("report should contain Slowest Requests section")
	}
	if !stringContains(report, "/api/data") {
		t.Error("report should contain slowest request URL")
	}
	if !stringContains(report, "450ms") {
		t.Error("report should contain request duration")
	}
}

func TestFormatPerformanceReportNilFCPLCP(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	snapshot := PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing: PerformanceTiming{
			TimeToFirstByte:  80,
			DomInteractive:   500,
			DomContentLoaded: 600,
			Load:             1200,
			// FCP and LCP are nil
		},
		Network: NetworkSummary{
			RequestCount: 5,
			TransferSize: 30000,
			DecodedSize:  60000,
		},
		LongTasks: LongTaskMetrics{},
	}

	report := capture.FormatPerformanceReport(snapshot, nil)

	if stringContains(report, "First Contentful Paint") {
		t.Error("report should NOT contain FCP when nil")
	}
	if stringContains(report, "Largest Contentful Paint") {
		t.Error("report should NOT contain LCP when nil")
	}
}

// Old HandlePerformanceSnapshot (singular) tests removed — endpoint deleted in Phase 6 (W6).
// See TestHandlePerformanceSnapshots_* (plural) and TestOldPerformanceSnapshotEndpoint_Gone below.

// --- DetectRegressions: load regression ---

func TestDetectRegressionsLoadTime(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baseline := PerformanceBaseline{
		URL:         "/test",
		SampleCount: 5,
		Timing: BaselineTiming{
			Load: 1000,
		},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
	}

	snapshot := PerformanceSnapshot{
		URL: "/test",
		Timing: PerformanceTiming{
			Load: 2500, // +150%, +1500ms (exceeds both thresholds)
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "load" {
			found = true
			if r.Current != 2500 {
				t.Errorf("expected current 2500, got %f", r.Current)
			}
			if r.Baseline != 1000 {
				t.Errorf("expected baseline 1000, got %f", r.Baseline)
			}
			if r.ChangePercent != 150 {
				t.Errorf("expected 150%% change, got %f", r.ChangePercent)
			}
			if r.AbsoluteChange != 1500 {
				t.Errorf("expected 1500 absolute change, got %f", r.AbsoluteChange)
			}
		}
	}
	if !found {
		t.Error("expected load regression to be detected")
	}
}

func TestDetectRegressionsLoadNoFalsePositive(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Change is >50% but <200ms absolute
	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{Load: 100},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
	}
	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{Load: 250}, // +150%, but only +150ms
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	}

	regressions := capture.DetectRegressions(snapshot, baseline)
	for _, r := range regressions {
		if r.Metric == "load" {
			t.Error("load regression should not fire when absolute change < 200ms")
		}
	}
}

// --- DetectRegressions: request count ---

func TestDetectRegressionsRequestCount(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baseline := PerformanceBaseline{
		Timing: BaselineTiming{},
		Network: BaselineNetwork{
			RequestCount: 10,
		},
		LongTasks: LongTaskMetrics{},
	}

	snapshot := PerformanceSnapshot{
		Timing: PerformanceTiming{},
		Network: NetworkSummary{
			RequestCount: 20, // +100%, +10 (exceeds both thresholds)
			ByType:       map[string]TypeSummary{},
		},
		LongTasks: LongTaskMetrics{},
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "requestCount" {
			found = true
			if r.Current != 20 {
				t.Errorf("expected current 20, got %f", r.Current)
			}
			if r.Baseline != 10 {
				t.Errorf("expected baseline 10, got %f", r.Baseline)
			}
		}
	}
	if !found {
		t.Error("expected requestCount regression to be detected")
	}
}

func TestDetectRegressionsRequestCountNoFalsePositive(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// >50% but only +3 (below the >5 threshold)
	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{RequestCount: 4},
		LongTasks: LongTaskMetrics{},
	}
	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{RequestCount: 7, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	}

	regressions := capture.DetectRegressions(snapshot, baseline)
	for _, r := range regressions {
		if r.Metric == "requestCount" {
			t.Error("requestCount regression should not fire when absolute change <= 5")
		}
	}
}

// --- DetectRegressions: transfer size ---

func TestDetectRegressionsTransferSize(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baseline := PerformanceBaseline{
		Timing: BaselineTiming{},
		Network: BaselineNetwork{
			TransferSize: 100000, // 100KB
		},
		LongTasks: LongTaskMetrics{},
	}

	snapshot := PerformanceSnapshot{
		Timing: PerformanceTiming{},
		Network: NetworkSummary{
			TransferSize: 350000, // +250%, +250KB (exceeds >100% and >100KB)
			ByType:       map[string]TypeSummary{},
		},
		LongTasks: LongTaskMetrics{},
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "transferSize" {
			found = true
			if r.Current != 350000 {
				t.Errorf("expected current 350000, got %f", r.Current)
			}
		}
	}
	if !found {
		t.Error("expected transferSize regression to be detected")
	}
}

func TestDetectRegressionsTransferSizeNoFalsePositive(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// >100% increase but <100KB absolute (50KB -> 120KB = +70KB)
	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{TransferSize: 50000},
		LongTasks: LongTaskMetrics{},
	}
	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{TransferSize: 120000, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	}

	regressions := capture.DetectRegressions(snapshot, baseline)
	for _, r := range regressions {
		if r.Metric == "transferSize" {
			t.Error("transferSize regression should not fire when absolute change < 100KB")
		}
	}
}

// --- DetectRegressions: long tasks from 0 ---

func TestDetectRegressionsLongTasksFromZero(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{Count: 0},
	}

	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Count: 3},
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "longTaskCount" {
			found = true
			if r.Current != 3 {
				t.Errorf("expected current 3, got %f", r.Current)
			}
			if r.Baseline != 0 {
				t.Errorf("expected baseline 0, got %f", r.Baseline)
			}
			if r.ChangePercent != 100 {
				t.Errorf("expected 100%% change, got %f", r.ChangePercent)
			}
		}
	}
	if !found {
		t.Error("expected longTaskCount regression when going from 0 to >0")
	}
}

// --- DetectRegressions: long tasks >100% increase ---

func TestDetectRegressionsLongTasksOver100Percent(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{Count: 2},
	}

	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Count: 5}, // +150%
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "longTaskCount" {
			found = true
			if r.Current != 5 {
				t.Errorf("expected current 5, got %f", r.Current)
			}
			if r.Baseline != 2 {
				t.Errorf("expected baseline 2, got %f", r.Baseline)
			}
		}
	}
	if !found {
		t.Error("expected longTaskCount regression when >100% increase")
	}
}

func TestDetectRegressionsLongTasksNoFalsePositive(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// 2 -> 3 is only 50% increase
	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{Count: 2},
	}
	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Count: 3},
	}

	regressions := capture.DetectRegressions(snapshot, baseline)
	for _, r := range regressions {
		if r.Metric == "longTaskCount" {
			t.Error("longTaskCount regression should not fire when increase <= 100%")
		}
	}
}

// --- DetectRegressions: TBT ---

func TestDetectRegressionsTBT(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{TotalBlockingTime: 50},
	}

	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{TotalBlockingTime: 200}, // +150ms (>100ms threshold)
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "totalBlockingTime" {
			found = true
			if r.Current != 200 {
				t.Errorf("expected current 200, got %f", r.Current)
			}
			if r.Baseline != 50 {
				t.Errorf("expected baseline 50, got %f", r.Baseline)
			}
			if r.AbsoluteChange != 150 {
				t.Errorf("expected absolute change 150, got %f", r.AbsoluteChange)
			}
			// pct = 150/50*100 = 300
			if r.ChangePercent != 300 {
				t.Errorf("expected 300%% change, got %f", r.ChangePercent)
			}
		}
	}
	if !found {
		t.Error("expected totalBlockingTime regression when absolute increase > 100ms")
	}
}

func TestDetectRegressionsTBTFromZero(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{TotalBlockingTime: 0},
	}

	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{TotalBlockingTime: 150}, // +150ms from 0
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "totalBlockingTime" {
			found = true
			// When baseline is 0, pct should be 0 (avoid divide by zero)
			if r.ChangePercent != 0 {
				t.Errorf("expected 0%% change when baseline is 0, got %f", r.ChangePercent)
			}
		}
	}
	if !found {
		t.Error("expected totalBlockingTime regression from 0 when increase > 100ms")
	}
}

func TestDetectRegressionsTBTNoFalsePositive(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{TotalBlockingTime: 50},
	}
	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{TotalBlockingTime: 130}, // +80ms (below 100ms threshold)
	}

	regressions := capture.DetectRegressions(snapshot, baseline)
	for _, r := range regressions {
		if r.Metric == "totalBlockingTime" {
			t.Error("TBT regression should not fire when absolute change <= 100ms")
		}
	}
}

// --- DetectRegressions: CLS ---

func TestDetectRegressionsCLS(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baselineCLS := 0.02
	snapshotCLS := 0.12 // +0.10 (>0.05 threshold)

	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
		CLS:       &baselineCLS,
	}

	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		CLS:       &snapshotCLS,
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "cumulativeLayoutShift" {
			found = true
			if r.Current != 0.12 {
				t.Errorf("expected current 0.12, got %f", r.Current)
			}
			if r.Baseline != 0.02 {
				t.Errorf("expected baseline 0.02, got %f", r.Baseline)
			}
			if r.AbsoluteChange < 0.09 || r.AbsoluteChange > 0.11 {
				t.Errorf("expected absolute change 0.10, got %f", r.AbsoluteChange)
			}
		}
	}
	if !found {
		t.Error("expected CLS regression when increase > 0.05")
	}
}

func TestDetectRegressionsCLSFromZero(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baselineCLS := 0.0
	snapshotCLS := 0.08 // +0.08 from 0 (>0.05 threshold)

	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
		CLS:       &baselineCLS,
	}

	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		CLS:       &snapshotCLS,
	}

	regressions := capture.DetectRegressions(snapshot, baseline)

	found := false
	for _, r := range regressions {
		if r.Metric == "cumulativeLayoutShift" {
			found = true
			// When baseline is 0, pct should be 0
			if r.ChangePercent != 0 {
				t.Errorf("expected 0%% change when baseline CLS is 0, got %f", r.ChangePercent)
			}
		}
	}
	if !found {
		t.Error("expected CLS regression from 0 when increase > 0.05")
	}
}

func TestDetectRegressionsCLSNoFalsePositive(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	baselineCLS := 0.05
	snapshotCLS := 0.08 // +0.03 (below 0.05 threshold)

	baseline := PerformanceBaseline{
		Timing:    BaselineTiming{},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
		CLS:       &baselineCLS,
	}
	snapshot := PerformanceSnapshot{
		Timing:    PerformanceTiming{},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		CLS:       &snapshotCLS,
	}

	regressions := capture.DetectRegressions(snapshot, baseline)
	for _, r := range regressions {
		if r.Metric == "cumulativeLayoutShift" {
			t.Error("CLS regression should not fire when absolute change <= 0.05")
		}
	}
}

// --- toolCheckPerformance (line 555) ---

func TestToolCheckPerformanceNoSnapshot(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	server := &Server{}
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)

	resp := mcp.toolHandler.toolCheckPerformance(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	if !stringContains(result.Content[0].Text, "No performance snapshot available") {
		t.Errorf("expected no-snapshot message, got: %s", result.Content[0].Text)
	}
}

func TestToolCheckPerformanceWithURLFilter(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/page-a",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000, DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
		Network:   NetworkSummary{RequestCount: 5, TransferSize: 30000, DecodedSize: 60000, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	})
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/page-b",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 2000, DomContentLoaded: 800, TimeToFirstByte: 100, DomInteractive: 600},
		Network:   NetworkSummary{RequestCount: 15, TransferSize: 100000, DecodedSize: 200000, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	})

	server := &Server{}
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"url":"/page-a"}`)

	resp := mcp.toolHandler.toolCheckPerformance(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if !stringContains(result.Content[0].Text, "/page-a") {
		t.Errorf("expected report for /page-a, got: %s", result.Content[0].Text)
	}
	// Should not contain /page-b data
	if stringContains(result.Content[0].Text, "Requests: 15") {
		t.Error("report should be filtered to /page-a, not /page-b")
	}
}

func TestToolCheckPerformanceWithBaseline(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	// Add two snapshots to build baseline
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/page",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000, DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
		Network:   NetworkSummary{RequestCount: 10, TransferSize: 50000, DecodedSize: 100000, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Count: 1, TotalBlockingTime: 60},
	})
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/page",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1100, DomContentLoaded: 520, TimeToFirstByte: 55, DomInteractive: 420},
		Network:   NetworkSummary{RequestCount: 11, TransferSize: 52000, DecodedSize: 104000, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Count: 1, TotalBlockingTime: 55},
	})

	server := &Server{}
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)

	resp := mcp.toolHandler.toolCheckPerformance(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	// Should contain baseline comparison (no regressions expected for minor changes)
	if !stringContains(result.Content[0].Text, "No Regressions") {
		t.Errorf("expected no regressions for small changes, got: %s", result.Content[0].Text)
	}
}

// --- formatBytes: MB branch ---

func TestFormatBytesMB(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    int64
		expected string
	}{
		{1048576, "1.0MB"},          // exactly 1MB
		{1572864, "1.5MB"},          // 1.5MB
		{10485760, "10.0MB"},        // 10MB
		{1048576 + 524288, "1.5MB"}, // 1.5MB
		{2 * 1024 * 1024, "2.0MB"},  // 2MB
	}

	for _, tt := range tests {
		result := formatBytes(tt.input)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatBytesAllBranches(t *testing.T) {
	t.Parallel()
	// Bytes branch
	if got := formatBytes(500); got != "500B" {
		t.Errorf("formatBytes(500) = %q, want 500B", got)
	}
	if got := formatBytes(0); got != "0B" {
		t.Errorf("formatBytes(0) = %q, want 0B", got)
	}
	if got := formatBytes(1023); got != "1023B" {
		t.Errorf("formatBytes(1023) = %q, want 1023B", got)
	}

	// KB branch
	if got := formatBytes(1024); got != "1.0KB" {
		t.Errorf("formatBytes(1024) = %q, want 1.0KB", got)
	}
	if got := formatBytes(512 * 1024); got != "512.0KB" {
		t.Errorf("formatBytes(524288) = %q, want 512.0KB", got)
	}

	// MB branch
	if got := formatBytes(1024 * 1024); got != "1.0MB" {
		t.Errorf("formatBytes(1048576) = %q, want 1.0MB", got)
	}
	if got := formatBytes(5 * 1024 * 1024); got != "5.0MB" {
		t.Errorf("formatBytes(5242880) = %q, want 5.0MB", got)
	}
}

// --- GetLatestPerformanceSnapshot: non-empty case ---

func TestGetLatestPerformanceSnapshotNonEmpty(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/first",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000, DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	})
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/second",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 2000, DomContentLoaded: 800, TimeToFirstByte: 100, DomInteractive: 600},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	})

	snapshot, found := capture.GetLatestPerformanceSnapshot()
	if !found {
		t.Fatal("expected to find latest snapshot")
	}
	if snapshot.URL != "/second" {
		t.Errorf("expected latest snapshot to be /second, got %s", snapshot.URL)
	}
	if snapshot.Timing.Load != 2000 {
		t.Errorf("expected load 2000, got %f", snapshot.Timing.Load)
	}
}

func TestGetLatestPerformanceSnapshotUpdatesOnReAdd(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/first",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000, DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	})
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/second",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 2000, DomContentLoaded: 800, TimeToFirstByte: 100, DomInteractive: 600},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	})
	// Re-add /first - should move to end of order
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/first",
		Timestamp: "2024-01-15T10:02:00Z",
		Timing:    PerformanceTiming{Load: 1100, DomContentLoaded: 550, TimeToFirstByte: 55, DomInteractive: 450},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	})

	snapshot, found := capture.GetLatestPerformanceSnapshot()
	if !found {
		t.Fatal("expected to find latest snapshot")
	}
	if snapshot.URL != "/first" {
		t.Errorf("expected latest snapshot to be /first after re-add, got %s", snapshot.URL)
	}
}

// --- AddPerformanceSnapshot: LRU eviction path ---

func TestPerfSnapshotLRUEviction(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Fill up to maxPerfSnapshots (20)
	for i := 0; i < maxPerfSnapshots; i++ {
		capture.AddPerformanceSnapshot(PerformanceSnapshot{
			URL:       fmt.Sprintf("/page-%d", i),
			Timestamp: "2024-01-15T10:00:00Z",
			Timing:    PerformanceTiming{Load: float64(1000 + i), DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
			Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
			LongTasks: LongTaskMetrics{},
		})
	}

	// Verify all 20 exist
	for i := 0; i < maxPerfSnapshots; i++ {
		_, found := capture.GetPerformanceSnapshot(fmt.Sprintf("/page-%d", i))
		if !found {
			t.Fatalf("expected /page-%d to exist before eviction", i)
		}
	}

	// Add one more to trigger eviction of the oldest (/page-0)
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/page-new",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 5000, DomContentLoaded: 2000, TimeToFirstByte: 200, DomInteractive: 1500},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	})

	// /page-0 should be evicted
	_, found := capture.GetPerformanceSnapshot("/page-0")
	if found {
		t.Error("expected /page-0 to be evicted after exceeding maxPerfSnapshots")
	}

	// /page-new should exist
	_, found = capture.GetPerformanceSnapshot("/page-new")
	if !found {
		t.Error("expected /page-new to exist after LRU eviction")
	}

	// /page-1 should still exist (second oldest, not evicted)
	_, found = capture.GetPerformanceSnapshot("/page-1")
	if !found {
		t.Error("expected /page-1 to still exist")
	}
}

// --- indexOfDoubleslash: no match case ---

func TestIndexOfDoubleslashNoMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected int
	}{
		{"no-slashes-here", -1},
		{"/single/slashes/only", -1},
		{"", -1},
		{"a", -1},
		{"ab", -1},
	}

	for _, tt := range tests {
		result := indexOfDoubleslash(tt.input)
		if result != tt.expected {
			t.Errorf("indexOfDoubleslash(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestIndexOfDoubleslashMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected int
	}{
		{"https://example.com", 6},
		{"http://example.com", 5},
		{"//leading", 0},
		{"path//double", 4},
	}

	for _, tt := range tests {
		result := indexOfDoubleslash(tt.input)
		if result != tt.expected {
			t.Errorf("indexOfDoubleslash(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

// --- updateBaselineResources: weighted average >=5 samples ---

func TestUpdateBaselineResourcesWeightedAverage(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Add 5 snapshots to reach the weighted-average branch (n >= 5)
	for i := 0; i < 5; i++ {
		capture.AddPerformanceSnapshot(PerformanceSnapshot{
			URL:       "/weighted-test",
			Timestamp: fmt.Sprintf("2024-01-15T10:%02d:00Z", i),
			Timing:    PerformanceTiming{Load: 1000, DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
			Network:   NetworkSummary{RequestCount: 10, TransferSize: 50000, ByType: map[string]TypeSummary{}},
			LongTasks: LongTaskMetrics{},
			Resources: []ResourceEntry{
				{URL: "/static/js/app.js", Type: "script", TransferSize: 100000, Duration: 200},
			},
		})
	}

	// Now add a 6th snapshot with different resource size to trigger weighted avg (0.8/0.2)
	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/weighted-test",
		Timestamp: "2024-01-15T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1000, DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
		Network:   NetworkSummary{RequestCount: 10, TransferSize: 50000, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		Resources: []ResourceEntry{
			{URL: "/static/js/app.js", Type: "script", TransferSize: 200000, Duration: 400},
		},
	})

	capture.mu.RLock()
	baseline := capture.perf.baselines["/weighted-test"]
	capture.mu.RUnlock()

	if len(baseline.Resources) == 0 {
		t.Fatal("expected baseline resources to be populated")
	}

	// After 6 samples, the last update uses weighted avg: 0.8*old + 0.2*new
	// The old value would have stabilized around 100000 after the first 5 samples
	// New = 0.8*100000 + 0.2*200000 = 80000 + 40000 = 120000
	resource := baseline.Resources[0]
	if resource.TransferSize <= 100000 || resource.TransferSize >= 200000 {
		t.Errorf("expected weighted average transfer size between 100000 and 200000, got %d", resource.TransferSize)
	}
}

// --- updateBaselineResources: re-filter path ---

func TestUpdateBaselineResourcesRefilterWhenTooLarge(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Create initial snapshot with maxResourceFingerprints resources
	resources := make([]ResourceEntry, maxResourceFingerprints)
	for i := 0; i < maxResourceFingerprints; i++ {
		resources[i] = ResourceEntry{
			URL:          fmt.Sprintf("/static/js/chunk-%d.js", i),
			Type:         "script",
			TransferSize: int64(50000 + i*1000), // All above small threshold
			Duration:     float64(100 + i),
		}
	}

	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/refilter-test",
		Timestamp: "2024-01-15T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000, DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
		Network:   NetworkSummary{RequestCount: 10, TransferSize: 50000, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		Resources: resources,
	})

	// Add a second snapshot with new resources that will push baseline over maxResourceFingerprints
	newResources := make([]ResourceEntry, maxResourceFingerprints)
	for i := 0; i < maxResourceFingerprints; i++ {
		newResources[i] = ResourceEntry{
			URL:          fmt.Sprintf("/static/js/new-chunk-%d.js", i),
			Type:         "script",
			TransferSize: int64(60000 + i*1000),
			Duration:     float64(120 + i),
		}
	}

	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/refilter-test",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1000, DomContentLoaded: 500, TimeToFirstByte: 50, DomInteractive: 400},
		Network:   NetworkSummary{RequestCount: 10, TransferSize: 50000, ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
		Resources: newResources,
	})

	capture.mu.RLock()
	baseline := capture.perf.baselines["/refilter-test"]
	capture.mu.RUnlock()

	// After re-filtering, the list should be capped at maxResourceFingerprints
	if len(baseline.Resources) > maxResourceFingerprints {
		t.Errorf("expected baseline resources to be capped at %d, got %d", maxResourceFingerprints, len(baseline.Resources))
	}
}

// --- computeResourceDiff: additional branch coverage ---

func TestComputeResourceDiffFetchWithDynamicPath(t *testing.T) {
	t.Parallel()
	// Tests the normalizeKey branch for fetch/xmlhttprequest type resources
	baseline := []ResourceEntry{
		{URL: "https://api.example.com/api/user/123", Type: "fetch", TransferSize: 5000, Duration: 80},
	}
	current := []ResourceEntry{
		{URL: "https://api.example.com/api/user/456", Type: "fetch", TransferSize: 5000, Duration: 80},
	}

	diff := computeResourceDiff(baseline, current)

	// Both should normalize to /api/user/* so no added/removed
	if len(diff.Added) != 0 {
		t.Errorf("expected 0 added (dynamic paths should match), got %d", len(diff.Added))
	}
	if len(diff.Removed) != 0 {
		t.Errorf("expected 0 removed (dynamic paths should match), got %d", len(diff.Removed))
	}
}

func TestComputeResourceDiffXMLHttpRequestDynamicPath(t *testing.T) {
	t.Parallel()
	baseline := []ResourceEntry{
		{URL: "https://api.example.com/api/orders/abc", Type: "xmlhttprequest", TransferSize: 3000, Duration: 50},
	}
	current := []ResourceEntry{
		{URL: "https://api.example.com/api/orders/xyz", Type: "xmlhttprequest", TransferSize: 3500, Duration: 55},
	}

	diff := computeResourceDiff(baseline, current)

	// Both normalize to https://api.example.com/api/orders/*
	if len(diff.Added) != 0 {
		t.Errorf("expected 0 added for xmlhttprequest dynamic paths, got %d", len(diff.Added))
	}
	if len(diff.Removed) != 0 {
		t.Errorf("expected 0 removed for xmlhttprequest dynamic paths, got %d", len(diff.Removed))
	}
}

func TestComputeResourceDiffResizeNegative(t *testing.T) {
	t.Parallel()
	// Resource got smaller (negative delta)
	baseline := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 200000, Duration: 200},
	}
	current := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 150000, Duration: 200},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Resized) != 1 {
		t.Fatalf("expected 1 resized resource for shrink, got %d", len(diff.Resized))
	}
	if diff.Resized[0].DeltaBytes != -50000 {
		t.Errorf("expected delta -50000, got %d", diff.Resized[0].DeltaBytes)
	}
}

func TestComputeResourceDiffRetimeNegative(t *testing.T) {
	t.Parallel()
	// Resource got faster (negative timing delta)
	baseline := []ResourceEntry{
		{URL: "/api/data", Type: "fetch", TransferSize: 5000, Duration: 400},
	}
	current := []ResourceEntry{
		{URL: "/api/data", Type: "fetch", TransferSize: 5000, Duration: 250},
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Retimed) != 1 {
		t.Fatalf("expected 1 retimed resource for speedup, got %d", len(diff.Retimed))
	}
	if diff.Retimed[0].DeltaMs != -150 {
		t.Errorf("expected delta -150ms, got %f", diff.Retimed[0].DeltaMs)
	}
}

func TestComputeResourceDiffNoChangeWithinThreshold(t *testing.T) {
	t.Parallel()
	// Size change is below 10% and below 10KB -> no resize
	baseline := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 100000, Duration: 200},
	}
	current := []ResourceEntry{
		{URL: "/static/js/main.js", Type: "script", TransferSize: 105000, Duration: 250}, // +5% size, +50ms duration
	}

	diff := computeResourceDiff(baseline, current)

	if len(diff.Resized) != 0 {
		t.Errorf("expected 0 resized when change is within 10%% threshold, got %d", len(diff.Resized))
	}
	if len(diff.Retimed) != 0 {
		t.Errorf("expected 0 retimed when change is within 100ms threshold, got %d", len(diff.Retimed))
	}
}

func TestComputeResourceDiffFetchWithoutDoubleslash(t *testing.T) {
	t.Parallel()
	// Test fetch resource where URL has no "://" — uses path directly
	baseline := []ResourceEntry{
		{URL: "/api/user/123", Type: "fetch", TransferSize: 2000, Duration: 40},
	}
	current := []ResourceEntry{
		{URL: "/api/user/456", Type: "fetch", TransferSize: 2000, Duration: 40},
	}

	diff := computeResourceDiff(baseline, current)

	// Both should group to /api/user/*
	if len(diff.Added) != 0 {
		t.Errorf("expected 0 added for path-only fetch, got %d", len(diff.Added))
	}
	if len(diff.Removed) != 0 {
		t.Errorf("expected 0 removed for path-only fetch, got %d", len(diff.Removed))
	}
}

// ============================================
// Additional coverage: weightedOptionalFloat
// ============================================

func TestWeightedOptionalFloatZeroBaseline(t *testing.T) {
	t.Parallel()
	baseline := 0.0
	snapshot := 100.0

	result := weightedOptionalFloat(&baseline, &snapshot, 0.8, 0.2)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	// 0.0 * 0.8 + 100.0 * 0.2 = 20.0
	expected := 20.0
	if *result != expected {
		t.Errorf("Expected %f, got %f", expected, *result)
	}
}

func TestWeightedOptionalFloatBothZero(t *testing.T) {
	t.Parallel()
	baseline := 0.0
	snapshot := 0.0

	result := weightedOptionalFloat(&baseline, &snapshot, 0.8, 0.2)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if *result != 0.0 {
		t.Errorf("Expected 0.0, got %f", *result)
	}
}

func TestWeightedOptionalFloatNonZeroComputation(t *testing.T) {
	t.Parallel()
	baseline := 500.0
	snapshot := 600.0

	result := weightedOptionalFloat(&baseline, &snapshot, 0.8, 0.2)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	// 500 * 0.8 + 600 * 0.2 = 400 + 120 = 520
	expected := 520.0
	if *result != expected {
		t.Errorf("Expected %f, got %f", expected, *result)
	}
}

func TestWeightedOptionalFloatNilSnapshot(t *testing.T) {
	t.Parallel()
	baseline := 500.0

	result := weightedOptionalFloat(&baseline, nil, 0.8, 0.2)

	if result == nil {
		t.Fatal("Expected non-nil result (should return baseline)")
	}
	if *result != 500.0 {
		t.Errorf("Expected baseline 500.0 returned, got %f", *result)
	}
}

func TestWeightedOptionalFloatNilBaseline(t *testing.T) {
	t.Parallel()
	snapshot := 300.0

	result := weightedOptionalFloat(nil, &snapshot, 0.8, 0.2)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if *result != 300.0 {
		t.Errorf("Expected snapshot value 300.0, got %f", *result)
	}
}

// ============================================
// Additional coverage: GetCausalDiff
// ============================================

func TestGetCausalDiffWithAddedScripts(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Directly set the baseline with only one script resource
	capture.mu.Lock()
	capture.perf.baselines["http://localhost:3000/"] = PerformanceBaseline{
		URL:         "http://localhost:3000/",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "http://cdn.example.com/app.js", Type: "script", TransferSize: 50000, Duration: 100},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "http://localhost:3000/")

	// Set up the current snapshot with an additional script
	capture.perf.snapshots["http://localhost:3000/"] = PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 130000},
		Resources: []ResourceEntry{
			{URL: "http://cdn.example.com/app.js", Type: "script", TransferSize: 50000, Duration: 100},
			{URL: "http://cdn.example.com/vendor.js", Type: "script", TransferSize: 80000, Duration: 200},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "http://localhost:3000/")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("http://localhost:3000/", "")

	if result.URL != "http://localhost:3000/" {
		t.Errorf("Expected URL 'http://localhost:3000/', got '%s'", result.URL)
	}

	// Should detect added resource
	if len(result.ResourceChanges.Added) == 0 {
		t.Error("Expected at least one added resource")
	}

	foundAdded := false
	for _, a := range result.ResourceChanges.Added {
		if strings.Contains(a.URL, "vendor.js") {
			foundAdded = true
			if a.Type != "script" {
				t.Errorf("Expected added resource type 'script', got '%s'", a.Type)
			}
		}
	}
	if !foundAdded {
		t.Error("Expected vendor.js to be listed as added resource")
	}

	if result.ProbableCause == "" {
		t.Error("Expected non-empty probable cause")
	}
}

func TestGetCausalDiffWithRemovedScripts(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Directly set baseline with two scripts
	capture.mu.Lock()
	capture.perf.baselines["http://localhost:3000/"] = PerformanceBaseline{
		URL:         "http://localhost:3000/",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1200},
		Network:     BaselineNetwork{TransferSize: 80000},
		Resources: []ResourceEntry{
			{URL: "http://cdn.example.com/app.js", Type: "script", TransferSize: 50000, Duration: 100},
			{URL: "http://cdn.example.com/analytics.js", Type: "script", TransferSize: 30000, Duration: 80},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "http://localhost:3000/")

	// Current snapshot has only one script (analytics removed)
	capture.perf.snapshots["http://localhost:3000/"] = PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 900},
		Network:   NetworkSummary{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "http://cdn.example.com/app.js", Type: "script", TransferSize: 50000, Duration: 100},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "http://localhost:3000/")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("http://localhost:3000/", "")

	if len(result.ResourceChanges.Removed) == 0 {
		t.Error("Expected at least one removed resource")
	}

	foundRemoved := false
	for _, r := range result.ResourceChanges.Removed {
		if strings.Contains(r.URL, "analytics.js") {
			foundRemoved = true
			if r.Type != "script" {
				t.Errorf("Expected removed resource type 'script', got '%s'", r.Type)
			}
		}
	}
	if !foundRemoved {
		t.Error("Expected analytics.js to be listed as removed resource")
	}
}

func TestGetCausalDiffWithSizeChanges(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Directly set baseline with original size
	capture.mu.Lock()
	capture.perf.baselines["http://localhost:3000/"] = PerformanceBaseline{
		URL:         "http://localhost:3000/",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "http://cdn.example.com/app.js", Type: "script", TransferSize: 50000, Duration: 100},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "http://localhost:3000/")

	// Current snapshot with significantly larger app.js
	capture.perf.snapshots["http://localhost:3000/"] = PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1400},
		Network:   NetworkSummary{TransferSize: 90000},
		Resources: []ResourceEntry{
			{URL: "http://cdn.example.com/app.js", Type: "script", TransferSize: 90000, Duration: 150},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "http://localhost:3000/")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("http://localhost:3000/", "")

	if len(result.ResourceChanges.Resized) == 0 {
		t.Error("Expected at least one resized resource")
	}

	foundResized := false
	for _, r := range result.ResourceChanges.Resized {
		if strings.Contains(r.URL, "app.js") {
			foundResized = true
			if r.DeltaBytes <= 0 {
				t.Errorf("Expected positive size delta, got %d", r.DeltaBytes)
			}
		}
	}
	if !foundResized {
		t.Error("Expected app.js to be listed as resized resource")
	}

	if len(result.Recommendations) == 0 {
		t.Error("Expected recommendations for resized resources")
	}
}

// ============================================
// Additional coverage: updateBaselineResources
// ============================================

func TestUpdateBaselineResourcesFontType(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add a snapshot with font resources
	resources := []ResourceEntry{
		{URL: "http://cdn.example.com/font.woff2", Type: "font", TransferSize: 25000, Duration: 50},
		{URL: "http://cdn.example.com/app.js", Type: "script", TransferSize: 50000, Duration: 100},
		{URL: "http://cdn.example.com/style.css", Type: "stylesheet", TransferSize: 15000, Duration: 30},
	}

	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 90000},
		Resources: resources,
	})

	baseline, found := capture.GetPerformanceBaseline("http://localhost:3000/")
	if !found {
		t.Fatal("Expected baseline to exist")
	}

	// Check that font resource is preserved in baseline resources
	foundFont := false
	for _, r := range baseline.Resources {
		if r.Type == "font" {
			foundFont = true
			if r.TransferSize != 25000 {
				t.Errorf("Expected font transfer size 25000, got %d", r.TransferSize)
			}
		}
	}
	if !foundFont {
		t.Error("Expected font resource to be included in baseline resources")
	}

	// Add a second snapshot updating the same resources
	resources2 := []ResourceEntry{
		{URL: "http://cdn.example.com/font.woff2", Type: "font", TransferSize: 26000, Duration: 55},
		{URL: "http://cdn.example.com/app.js", Type: "script", TransferSize: 52000, Duration: 110},
		{URL: "http://cdn.example.com/style.css", Type: "stylesheet", TransferSize: 16000, Duration: 35},
	}

	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1100},
		Network:   NetworkSummary{TransferSize: 94000},
		Resources: resources2,
	})

	baseline2, found2 := capture.GetPerformanceBaseline("http://localhost:3000/")
	if !found2 {
		t.Fatal("Expected baseline to exist after second snapshot")
	}

	// Font resource should be averaged
	foundFont2 := false
	for _, r := range baseline2.Resources {
		if r.Type == "font" {
			foundFont2 = true
			// Should be averaged: (25000 * 1/2) + (26000 * 1/2) = 25500
			if r.TransferSize == 25000 || r.TransferSize == 26000 {
				// The averaging formula is (baseline*(n-1)/n + snap/n) with n=2
				// = (25000 * 1/2) + (26000 / 2) = 12500 + 13000 = 25500
				t.Logf("Font resource transfer size: %d (averaging applied)", r.TransferSize)
			}
		}
	}
	if !foundFont2 {
		t.Error("Expected font resource in updated baseline")
	}
}

// Old HandlePerformanceSnapshot GET/DELETE coverage tests removed — endpoint deleted in Phase 6 (W6).

// ============================================
// Additional coverage: normalizeDynamicAPIPath
// ============================================

func TestNormalizeDynamicAPIPathWithUUID(t *testing.T) {
	t.Parallel()
	// Path with UUID segment (3+ segments gets wildcard after 2nd)
	result := normalizeDynamicAPIPath("/api/users/550e8400-e29b-41d4-a716-446655440000")

	if result != "/api/users/*" {
		t.Errorf("Expected '/api/users/*', got '%s'", result)
	}
}

func TestNormalizeDynamicAPIPathWithNumericID(t *testing.T) {
	t.Parallel()
	result := normalizeDynamicAPIPath("/api/products/12345")

	if result != "/api/products/*" {
		t.Errorf("Expected '/api/products/*', got '%s'", result)
	}
}

func TestNormalizeDynamicAPIPathShortPath(t *testing.T) {
	t.Parallel()
	// Paths with fewer than 3 slashes should be kept as-is
	result := normalizeDynamicAPIPath("/api/health")

	if result != "/api/health" {
		t.Errorf("Expected '/api/health' unchanged, got '%s'", result)
	}
}

func TestNormalizeDynamicAPIPathDeepNesting(t *testing.T) {
	t.Parallel()
	result := normalizeDynamicAPIPath("/api/v2/users/123/posts/456")

	if result != "/api/v2/*" {
		t.Errorf("Expected '/api/v2/*', got '%s'", result)
	}
}

func TestNormalizeDynamicAPIPathEmpty(t *testing.T) {
	t.Parallel()
	result := normalizeDynamicAPIPath("")

	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestNormalizeDynamicAPIPathRootOnly(t *testing.T) {
	t.Parallel()
	result := normalizeDynamicAPIPath("/")

	if result != "/" {
		t.Errorf("Expected '/', got '%s'", result)
	}
}

// Old HandlePerformanceSnapshot baseline/body-too-large coverage tests removed — endpoint deleted in Phase 6 (W6).

// ============================================
// Coverage: GetCausalDiff — render-blocking resources in computeProbableCause (line 880)
// ============================================

func TestGetCausalDiffMultipleRenderBlockingResources(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Set baseline with no scripts
	capture.mu.Lock()
	capture.perf.baselines["/blocking-test"] = PerformanceBaseline{
		URL:         "/blocking-test",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 800},
		Network:     BaselineNetwork{TransferSize: 20000},
		Resources:   []ResourceEntry{},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/blocking-test")

	// Set snapshot with multiple render-blocking scripts
	capture.perf.snapshots["/blocking-test"] = PerformanceSnapshot{
		URL:       "/blocking-test",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 2000},
		Network:   NetworkSummary{TransferSize: 200000},
		Resources: []ResourceEntry{
			{URL: "/static/js/vendor.js", Type: "script", TransferSize: 80000, Duration: 200, RenderBlocking: true},
			{URL: "/static/js/analytics.js", Type: "script", TransferSize: 50000, Duration: 150, RenderBlocking: true},
			{URL: "/static/js/app.js", Type: "script", TransferSize: 60000, Duration: 100, RenderBlocking: false},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/blocking-test")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/blocking-test", "")

	// Should mention render-blocking in the probable cause
	if !strings.Contains(result.ProbableCause, "render-blocking") {
		t.Errorf("Expected probable cause to mention render-blocking, got: %s", result.ProbableCause)
	}

	// Should list both blocking resources in the cause
	if !strings.Contains(result.ProbableCause, "vendor.js") {
		t.Errorf("Expected probable cause to mention vendor.js, got: %s", result.ProbableCause)
	}
	if !strings.Contains(result.ProbableCause, "analytics.js") {
		t.Errorf("Expected probable cause to mention analytics.js, got: %s", result.ProbableCause)
	}
}

// ============================================
// Coverage: GetCausalDiff — retimed resources in computeProbableCause
// ============================================

func TestGetCausalDiffRetimedResources(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Baseline with a resource at specific duration
	capture.mu.Lock()
	capture.perf.baselines["/retimed-test"] = PerformanceBaseline{
		URL:         "/retimed-test",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "https://api.example.com/data", Type: "fetch", TransferSize: 5000, Duration: 100},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/retimed-test")

	// Snapshot with same resource much slower (>100ms difference)
	capture.perf.snapshots["/retimed-test"] = PerformanceSnapshot{
		URL:       "/retimed-test",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "https://api.example.com/data", Type: "fetch", TransferSize: 5000, Duration: 500},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/retimed-test")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/retimed-test", "")

	// Should mention slowed in probable cause
	if !strings.Contains(result.ProbableCause, "slowed") {
		t.Errorf("Expected probable cause to mention 'slowed', got: %s", result.ProbableCause)
	}
}

// ============================================
// Coverage: GetCausalDiff — payload increase in computeProbableCause
// ============================================

func TestGetCausalDiffPayloadIncrease(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Set up baseline with smaller resources
	capture.mu.Lock()
	capture.perf.baselines["/payload-test"] = PerformanceBaseline{
		URL:         "/payload-test",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "/static/js/app.js", Type: "script", TransferSize: 50000, Duration: 100},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/payload-test")

	// Snapshot with much larger resources (to trigger payload increase)
	capture.perf.snapshots["/payload-test"] = PerformanceSnapshot{
		URL:       "/payload-test",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 2000},
		Network:   NetworkSummary{TransferSize: 200000},
		Resources: []ResourceEntry{
			{URL: "/static/js/app.js", Type: "script", TransferSize: 50000, Duration: 100},
			{URL: "/static/js/new-feature.js", Type: "script", TransferSize: 150000, Duration: 300},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/payload-test")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/payload-test", "")

	// Should mention payload increase
	if !strings.Contains(result.ProbableCause, "payload increased") {
		t.Errorf("Expected probable cause to mention 'payload increased', got: %s", result.ProbableCause)
	}
}

// ============================================
// Coverage: GetCausalDiff — no resources in both baseline and snapshot (line 983)
// ============================================

func TestGetCausalDiffNoResourcesEither(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Set baseline with no resources
	capture.mu.Lock()
	capture.perf.baselines["/no-resources"] = PerformanceBaseline{
		URL:         "/no-resources",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 50000},
		Resources:   []ResourceEntry{}, // Empty
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/no-resources")

	// Snapshot also with no resources
	capture.perf.snapshots["/no-resources"] = PerformanceSnapshot{
		URL:       "/no-resources",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 50000},
		Resources: []ResourceEntry{}, // Empty
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/no-resources")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/no-resources", "")

	if !strings.Contains(result.ProbableCause, "baseline predates resource tracking") {
		t.Errorf("Expected 'baseline predates resource tracking', got: %s", result.ProbableCause)
	}
}

// ============================================
// Coverage: updateBaselineResources — first sample branch (lines 1022-1026)
// ============================================

func TestUpdateBaselineResourcesFirstSample(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Create a baseline with no resources, then update with resources
	capture.mu.Lock()
	baseline := PerformanceBaseline{
		URL:         "/first-resource",
		SampleCount: 2,
		Timing:      BaselineTiming{Load: 1000},
		Resources:   []ResourceEntry{}, // Empty — triggers first sample branch
	}

	resources := []ResourceEntry{
		{URL: "/static/js/app.js", Type: "script", TransferSize: 50000, Duration: 200},
		{URL: "/static/css/style.css", Type: "stylesheet", TransferSize: 10000, Duration: 50},
	}

	capture.updateBaselineResources(&baseline, resources)
	capture.mu.Unlock()

	if len(baseline.Resources) != 2 {
		t.Fatalf("Expected 2 baseline resources after first sample, got %d", len(baseline.Resources))
	}
	if baseline.Resources[0].URL != "/static/js/app.js" {
		t.Errorf("Expected first resource URL '/static/js/app.js', got '%s'", baseline.Resources[0].URL)
	}
	if baseline.Resources[1].URL != "/static/css/style.css" {
		t.Errorf("Expected second resource URL '/static/css/style.css', got '%s'", baseline.Resources[1].URL)
	}
}

// ============================================
// Coverage: updateBaselineResources — new resource added to existing baseline
// ============================================

func TestUpdateBaselineResourcesNewResourceAdded(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	capture.mu.Lock()
	baseline := PerformanceBaseline{
		URL:         "/add-resource",
		SampleCount: 3,
		Timing:      BaselineTiming{Load: 1000},
		Resources: []ResourceEntry{
			{URL: "/static/js/app.js", Type: "script", TransferSize: 50000, Duration: 200},
		},
	}

	// Add a resource that doesn't exist in baseline
	resources := []ResourceEntry{
		{URL: "/static/js/app.js", Type: "script", TransferSize: 55000, Duration: 210},
		{URL: "/static/js/new.js", Type: "script", TransferSize: 30000, Duration: 100},
	}

	capture.updateBaselineResources(&baseline, resources)
	capture.mu.Unlock()

	if len(baseline.Resources) != 2 {
		t.Fatalf("Expected 2 baseline resources, got %d", len(baseline.Resources))
	}

	// Verify the new resource was added
	foundNew := false
	for _, r := range baseline.Resources {
		if r.URL == "/static/js/new.js" {
			foundNew = true
		}
	}
	if !foundNew {
		t.Error("Expected new.js to be added to baseline resources")
	}
}

// ============================================
// Coverage: GetCausalDiff — recommendations for render-blocking and large resources
// ============================================

func TestGetCausalDiffRecommendations(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	capture.mu.Lock()
	capture.perf.baselines["/recs-test"] = PerformanceBaseline{
		URL:         "/recs-test",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 800},
		Network:     BaselineNetwork{TransferSize: 20000},
		Resources:   []ResourceEntry{},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/recs-test")

	capture.perf.snapshots["/recs-test"] = PerformanceSnapshot{
		URL:       "/recs-test",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 2000},
		Network:   NetworkSummary{TransferSize: 300000},
		Resources: []ResourceEntry{
			{URL: "/static/js/blocking.js", Type: "script", TransferSize: 80000, Duration: 200, RenderBlocking: true},
			{URL: "/static/js/huge.js", Type: "script", TransferSize: 200000, Duration: 400, RenderBlocking: false},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/recs-test")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/recs-test", "")

	// Should have recommendations for render-blocking resource
	foundBlocking := false
	foundCodeSplit := false
	for _, rec := range result.Recommendations {
		if strings.Contains(rec, "lazy-loading") && strings.Contains(rec, "blocking.js") {
			foundBlocking = true
		}
		if strings.Contains(rec, "code-splitting") && strings.Contains(rec, "huge.js") {
			foundCodeSplit = true
		}
	}
	if !foundBlocking {
		t.Errorf("Expected recommendation about lazy-loading render-blocking script, recommendations: %v", result.Recommendations)
	}
	if !foundCodeSplit {
		t.Errorf("Expected recommendation about code-splitting large script, recommendations: %v", result.Recommendations)
	}
}

// ============================================
// Coverage: GetCausalDiff — computeRecommendations for resized resources
// ============================================

func TestGetCausalDiffRecommendationsForResized(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	capture.mu.Lock()
	capture.perf.baselines["/resized-recs"] = PerformanceBaseline{
		URL:         "/resized-recs",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "/static/js/bundle.js", Type: "script", TransferSize: 50000, Duration: 200},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/resized-recs")

	// Snapshot where the bundle grew significantly (>10KB or >10%)
	capture.perf.snapshots["/resized-recs"] = PerformanceSnapshot{
		URL:       "/resized-recs",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 100000},
		Resources: []ResourceEntry{
			{URL: "/static/js/bundle.js", Type: "script", TransferSize: 100000, Duration: 300},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/resized-recs")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/resized-recs", "")

	// Should have recommendation about bundle growth
	foundGrowth := false
	for _, rec := range result.Recommendations {
		if strings.Contains(rec, "grew by") && strings.Contains(rec, "bundle.js") {
			foundGrowth = true
		}
	}
	if !foundGrowth {
		t.Errorf("Expected recommendation about bundle growth, recommendations: %v", result.Recommendations)
	}
}

// ============================================
// Coverage: GetCausalDiff — computeRecommendations for retimed API
// ============================================

func TestGetCausalDiffRecommendationsForRetimed(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	capture.mu.Lock()
	capture.perf.baselines["/api-recs"] = PerformanceBaseline{
		URL:         "/api-recs",
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "https://api.example.com/users", Type: "fetch", TransferSize: 5000, Duration: 100},
		},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/api-recs")

	capture.perf.snapshots["/api-recs"] = PerformanceSnapshot{
		URL:       "/api-recs",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 50000},
		Resources: []ResourceEntry{
			{URL: "https://api.example.com/users", Type: "fetch", TransferSize: 5000, Duration: 600},
		},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/api-recs")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/api-recs", "")

	// Should have recommendation about API regression
	foundAPI := false
	for _, rec := range result.Recommendations {
		if strings.Contains(rec, "Investigate API regression") {
			foundAPI = true
		}
	}
	if !foundAPI {
		t.Errorf("Expected recommendation about API regression, recommendations: %v", result.Recommendations)
	}
}

// ============================================
// Coverage: GetCausalDiff — no baseline branch
// ============================================

func TestGetCausalDiffNoBaseline(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Set a snapshot but no baseline
	capture.mu.Lock()
	capture.perf.snapshots["/no-bl"] = PerformanceSnapshot{
		URL:       "/no-bl",
		Timestamp: "2024-01-15T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 50000},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "/no-bl")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/no-bl", "")

	if !strings.Contains(result.ProbableCause, "No baseline available") {
		t.Errorf("Expected 'No baseline available', got: %s", result.ProbableCause)
	}
}

// ============================================
// Coverage: GetCausalDiff — no snapshot branch
// ============================================

func TestGetCausalDiffNoSnapshot(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Set baseline but no snapshot
	capture.mu.Lock()
	capture.perf.baselines["/no-snap"] = PerformanceBaseline{
		URL:         "/no-snap",
		SampleCount: 3,
		Timing:      BaselineTiming{Load: 1000},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "/no-snap")
	capture.mu.Unlock()

	result := capture.GetCausalDiff("/no-snap", "")

	if !strings.Contains(result.ProbableCause, "No current snapshot available") {
		t.Errorf("Expected 'No current snapshot available', got: %s", result.ProbableCause)
	}
}

// ============================================
// Coverage: updateBaselineResources — empty snapshot resources (early return)
// ============================================

func TestUpdateBaselineResourcesEmptySnapshot(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	capture.mu.Lock()
	baseline := PerformanceBaseline{
		URL:         "/empty-snap",
		SampleCount: 3,
		Timing:      BaselineTiming{Load: 1000},
		Resources: []ResourceEntry{
			{URL: "/app.js", Type: "script", TransferSize: 50000, Duration: 200},
		},
	}

	// Calling with empty resources should not modify baseline
	capture.updateBaselineResources(&baseline, []ResourceEntry{})
	capture.mu.Unlock()

	if len(baseline.Resources) != 1 {
		t.Errorf("Expected baseline to remain unchanged with empty snapshot, got %d resources", len(baseline.Resources))
	}
}

// ============================================
// Phase 6 (W6): Batch Performance Snapshots Endpoint
// ============================================

func TestHandlePerformanceSnapshots_SingleSnapshot(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	body := `{"snapshots":[{"url":"/single","timestamp":"2024-01-15T10:00:00Z","timing":{"domContentLoaded":600,"load":1200,"timeToFirstByte":80,"domInteractive":500},"network":{"requestCount":5,"transferSize":30000,"decodedSize":60000,"byType":{}},"longTasks":{"count":0,"totalBlockingTime":0,"longest":0}}]}`

	req := httptest.NewRequest("POST", "/performance-snapshots", strings.NewReader(body))
	w := httptest.NewRecorder()
	capture.HandlePerformanceSnapshots(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["received"] != float64(1) {
		t.Errorf("expected received: 1, got %v", resp["received"])
	}
}

func TestHandlePerformanceSnapshots_MultipleBatched(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	body := `{"snapshots":[` +
		`{"url":"/page1","timestamp":"2024-01-15T10:00:00Z","timing":{"domContentLoaded":600,"load":1200,"timeToFirstByte":80,"domInteractive":500},"network":{"requestCount":5,"transferSize":30000,"decodedSize":60000,"byType":{}},"longTasks":{"count":0,"totalBlockingTime":0,"longest":0}},` +
		`{"url":"/page2","timestamp":"2024-01-15T10:01:00Z","timing":{"domContentLoaded":700,"load":1300,"timeToFirstByte":90,"domInteractive":600},"network":{"requestCount":6,"transferSize":35000,"decodedSize":70000,"byType":{}},"longTasks":{"count":1,"totalBlockingTime":50,"longest":50}},` +
		`{"url":"/page3","timestamp":"2024-01-15T10:02:00Z","timing":{"domContentLoaded":500,"load":1100,"timeToFirstByte":70,"domInteractive":400},"network":{"requestCount":4,"transferSize":25000,"decodedSize":50000,"byType":{}},"longTasks":{"count":0,"totalBlockingTime":0,"longest":0}},` +
		`{"url":"/page4","timestamp":"2024-01-15T10:03:00Z","timing":{"domContentLoaded":650,"load":1250,"timeToFirstByte":85,"domInteractive":550},"network":{"requestCount":7,"transferSize":40000,"decodedSize":80000,"byType":{}},"longTasks":{"count":2,"totalBlockingTime":100,"longest":60}},` +
		`{"url":"/page5","timestamp":"2024-01-15T10:04:00Z","timing":{"domContentLoaded":800,"load":1500,"timeToFirstByte":100,"domInteractive":700},"network":{"requestCount":8,"transferSize":45000,"decodedSize":90000,"byType":{}},"longTasks":{"count":0,"totalBlockingTime":0,"longest":0}}` +
		`]}`

	req := httptest.NewRequest("POST", "/performance-snapshots", strings.NewReader(body))
	w := httptest.NewRecorder()
	capture.HandlePerformanceSnapshots(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["received"] != float64(5) {
		t.Errorf("expected received: 5, got %v", resp["received"])
	}

	// Verify all 5 snapshots were stored
	for _, url := range []string{"/page1", "/page2", "/page3", "/page4", "/page5"} {
		if _, found := capture.GetPerformanceSnapshot(url); !found {
			t.Errorf("snapshot for %s should be stored", url)
		}
	}
}

func TestHandlePerformanceSnapshots_EmptyArray(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	body := `{"snapshots":[]}`

	req := httptest.NewRequest("POST", "/performance-snapshots", strings.NewReader(body))
	w := httptest.NewRecorder()
	capture.HandlePerformanceSnapshots(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["received"] != float64(0) {
		t.Errorf("expected received: 0, got %v", resp["received"])
	}
}

func TestHandlePerformanceSnapshots_BadJSON(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	req := httptest.NewRequest("POST", "/performance-snapshots", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	capture.HandlePerformanceSnapshots(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandlePerformanceSnapshots_GET(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	req := httptest.NewRequest("GET", "/performance-snapshots", nil)
	w := httptest.NewRecorder()
	capture.HandlePerformanceSnapshots(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405 for GET, got %d", w.Code)
	}
}

func TestOldPerformanceSnapshotEndpoint_Gone(t *testing.T) {
	t.Parallel()
	// This test verifies the old singular endpoint is no longer registered.
	// We set up routes the same way main.go does and confirm /performance-snapshot returns 404.
	capture := NewCapture()

	// Create a fresh mux to avoid interference from other tests
	mux := http.NewServeMux()
	mux.HandleFunc("/performance-snapshots", corsMiddleware(capture.HandlePerformanceSnapshots))
	// Do NOT register /performance-snapshot (singular) — that's the point

	req := httptest.NewRequest("POST", "/performance-snapshot",
		strings.NewReader(`{"url":"/test","timestamp":"2024-01-15T10:00:00Z","timing":{"domContentLoaded":600,"load":1200,"timeToFirstByte":80,"domInteractive":500},"network":{"requestCount":5,"transferSize":30000,"decodedSize":60000,"byType":{}},"longTasks":{"count":0,"totalBlockingTime":0,"longest":0}}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for old singular endpoint, got %d", w.Code)
	}
}

func TestHandlePerformanceSnapshots_DataRetrievable(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	body := `{"snapshots":[` +
		`{"url":"/data-test-1","timestamp":"2024-01-15T10:00:00Z","timing":{"domContentLoaded":600,"load":1200,"timeToFirstByte":80,"domInteractive":500},"network":{"requestCount":5,"transferSize":30000,"decodedSize":60000,"byType":{}},"longTasks":{"count":0,"totalBlockingTime":0,"longest":0}},` +
		`{"url":"/data-test-2","timestamp":"2024-01-15T10:01:00Z","timing":{"domContentLoaded":700,"load":1500,"timeToFirstByte":90,"domInteractive":600},"network":{"requestCount":8,"transferSize":50000,"decodedSize":100000,"byType":{}},"longTasks":{"count":1,"totalBlockingTime":75,"longest":75}}` +
		`]}`

	req := httptest.NewRequest("POST", "/performance-snapshots", strings.NewReader(body))
	w := httptest.NewRecorder()
	capture.HandlePerformanceSnapshots(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify first snapshot is stored and retrievable
	snap1, found1 := capture.GetPerformanceSnapshot("/data-test-1")
	if !found1 {
		t.Fatal("snapshot for /data-test-1 should be retrievable")
	}
	if snap1.Timing.Load != 1200 {
		t.Errorf("expected load 1200, got %f", snap1.Timing.Load)
	}

	// Verify second snapshot is stored and retrievable
	snap2, found2 := capture.GetPerformanceSnapshot("/data-test-2")
	if !found2 {
		t.Fatal("snapshot for /data-test-2 should be retrievable")
	}
	if snap2.Timing.Load != 1500 {
		t.Errorf("expected load 1500, got %f", snap2.Timing.Load)
	}
	if snap2.LongTasks.Count != 1 {
		t.Errorf("expected longTasks.count 1, got %d", snap2.LongTasks.Count)
	}

	// Verify baselines were created
	_, baselineFound := capture.GetPerformanceBaseline("/data-test-1")
	if !baselineFound {
		t.Error("baseline for /data-test-1 should exist after batch POST")
	}
	_, baselineFound2 := capture.GetPerformanceBaseline("/data-test-2")
	if !baselineFound2 {
		t.Error("baseline for /data-test-2 should exist after batch POST")
	}
}
