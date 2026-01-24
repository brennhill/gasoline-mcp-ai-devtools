package main

import (
	"fmt"
	"encoding/json"
	"testing"
)

func TestPerformanceSnapshotJSONShape(t *testing.T) {
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
	tests := []struct {
		name       string
		fcp        float64
		lcp        float64
		cls        float64
		inp        float64
		expectFCP  string
		expectLCP  string
		expectCLS  string
		expectINP  string
	}{
		{
			name: "all good",
			fcp: 1000, lcp: 2000, cls: 0.05, inp: 100,
			expectFCP: "good", expectLCP: "good", expectCLS: "good", expectINP: "good",
		},
		{
			name: "all needs-improvement",
			fcp: 2500, lcp: 3500, cls: 0.15, inp: 350,
			expectFCP: "needs-improvement", expectLCP: "needs-improvement", expectCLS: "needs-improvement", expectINP: "needs-improvement",
		},
		{
			name: "all poor",
			fcp: 3500, lcp: 5000, cls: 0.3, inp: 600,
			expectFCP: "poor", expectLCP: "poor", expectCLS: "poor", expectINP: "poor",
		},
		{
			name: "boundary: exactly at good threshold",
			fcp: 1800, lcp: 2500, cls: 0.1, inp: 200,
			expectFCP: "needs-improvement", expectLCP: "needs-improvement", expectCLS: "needs-improvement", expectINP: "needs-improvement",
		},
		{
			name: "boundary: exactly at poor threshold",
			fcp: 3000, lcp: 4000, cls: 0.25, inp: 500,
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

	var vitals WebVitalsResult
	if err := json.Unmarshal([]byte(result.Content[0].Text), &vitals); err != nil {
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
	capture := NewCapture()
	server := &Server{}
	handler := &ToolHandler{
		MCPHandler: &MCPHandler{server: server},
		capture:    capture,
	}

	tools := handler.toolsList()
	found := false
	for _, tool := range tools {
		if tool.Name == "get_web_vitals" {
			found = true
			break
		}
	}
	if !found {
		t.Error("get_web_vitals tool should be registered in toolsList")
	}
}

func TestGetWebVitalsToolDispatch(t *testing.T) {
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
	args := json.RawMessage(`{}`)

	resp, handled := handler.handleToolCall(req, "get_web_vitals", args)
	if !handled {
		t.Error("get_web_vitals should be handled by handleToolCall")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestPerformanceTimingINPField(t *testing.T) {
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
