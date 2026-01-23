package main

import (
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
	baseline := server.perfBaselines["/test"]
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
