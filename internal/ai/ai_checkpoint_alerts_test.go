// ai_checkpoint_alerts_test.go — Tests for push regression alert detection and delivery.
package ai

import (
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/performance"
	gasTypes "github.com/dev-console/dev-console/internal/types"
)

// ============================================
// buildAlertSummary: load metric present
// ============================================

func TestBuildAlertSummary_LoadMetric(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	metrics := map[string]gasTypes.AlertMetricDelta{
		"load": {Baseline: 1000, Current: 1300, DeltaMs: 300, DeltaPct: 30},
		"fcp":  {Baseline: 500, Current: 700, DeltaMs: 200, DeltaPct: 40},
	}

	summary := cm.buildAlertSummary("https://example.com/page", metrics)

	if !strings.Contains(summary, "Load time regressed") {
		t.Errorf("summary = %q, want to contain 'Load time regressed'", summary)
	}
	if !strings.Contains(summary, "300ms") {
		t.Errorf("summary = %q, want to contain '300ms'", summary)
	}
	if !strings.Contains(summary, "1000ms") {
		t.Errorf("summary = %q, want to contain '1000ms'", summary)
	}
	if !strings.Contains(summary, "1300ms") {
		t.Errorf("summary = %q, want to contain '1300ms'", summary)
	}
	if !strings.Contains(summary, "https://example.com/page") {
		t.Errorf("summary = %q, want to contain URL", summary)
	}
}

// ============================================
// buildAlertSummary: no load metric (fallback)
// ============================================

func TestBuildAlertSummary_FallbackMetric(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	metrics := map[string]gasTypes.AlertMetricDelta{
		"ttfb": {Baseline: 200, Current: 400, DeltaMs: 200, DeltaPct: 100},
	}

	summary := cm.buildAlertSummary("https://example.com/slow", metrics)

	if !strings.Contains(summary, "ttfb") {
		t.Errorf("summary = %q, want to contain 'ttfb'", summary)
	}
	if !strings.Contains(summary, "regressed") {
		t.Errorf("summary = %q, want to contain 'regressed'", summary)
	}
	if !strings.Contains(summary, "https://example.com/slow") {
		t.Errorf("summary = %q, want to contain URL", summary)
	}
}

// ============================================
// buildAlertSummary: empty metrics
// ============================================

func TestBuildAlertSummary_EmptyMetrics(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	summary := cm.buildAlertSummary("https://example.com", map[string]gasTypes.AlertMetricDelta{})
	if !strings.Contains(summary, "Performance regression detected") {
		t.Errorf("summary = %q, want generic fallback", summary)
	}
}

// ============================================
// checkPctRegression: zero baseline
// ============================================

func TestCheckPctRegression_ZeroBaseline(t *testing.T) {
	t.Parallel()

	metrics := make(map[string]gasTypes.AlertMetricDelta)
	checkPctRegression(metrics, "load", 500, 0, 20.0)

	if len(metrics) != 0 {
		t.Errorf("zero baseline should not trigger regression, got %v", metrics)
	}
}

// ============================================
// checkPctRegression: negative baseline
// ============================================

func TestCheckPctRegression_NegativeBaseline(t *testing.T) {
	t.Parallel()

	metrics := make(map[string]gasTypes.AlertMetricDelta)
	checkPctRegression(metrics, "load", 500, -100, 20.0)

	if len(metrics) != 0 {
		t.Errorf("negative baseline should not trigger regression, got %v", metrics)
	}
}

// ============================================
// checkPctRegression: below threshold
// ============================================

func TestCheckPctRegression_BelowThreshold(t *testing.T) {
	t.Parallel()

	metrics := make(map[string]gasTypes.AlertMetricDelta)
	// 10% increase, threshold is 20%
	checkPctRegression(metrics, "load", 1100, 1000, 20.0)

	if len(metrics) != 0 {
		t.Errorf("10%% increase should not trigger 20%% threshold, got %v", metrics)
	}
}

// ============================================
// checkPctRegression: exact threshold (not triggered)
// ============================================

func TestCheckPctRegression_ExactThreshold(t *testing.T) {
	t.Parallel()

	metrics := make(map[string]gasTypes.AlertMetricDelta)
	// 20% increase = threshold -> not triggered (needs to EXCEED)
	checkPctRegression(metrics, "load", 1200, 1000, 20.0)

	if len(metrics) != 0 {
		t.Errorf("exact threshold should not trigger (> not >=), got %v", metrics)
	}
}

// ============================================
// checkPctRegression: above threshold
// ============================================

func TestCheckPctRegression_AboveThreshold(t *testing.T) {
	t.Parallel()

	metrics := make(map[string]gasTypes.AlertMetricDelta)
	// 25% increase, threshold is 20%
	checkPctRegression(metrics, "load", 1250, 1000, 20.0)

	if len(metrics) != 1 {
		t.Fatalf("25%% increase should trigger 20%% threshold, got %v", metrics)
	}
	m := metrics["load"]
	if m.Baseline != 1000 {
		t.Errorf("baseline = %f, want 1000", m.Baseline)
	}
	if m.Current != 1250 {
		t.Errorf("current = %f, want 1250", m.Current)
	}
	if m.DeltaMs != 250 {
		t.Errorf("delta_ms = %f, want 250", m.DeltaMs)
	}
	if m.DeltaPct != 25.0 {
		t.Errorf("delta_pct = %f, want 25.0", m.DeltaPct)
	}
}

// ============================================
// checkCLSRegression: nil values
// ============================================

func TestCheckCLSRegression_NilValues(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	metrics := make(map[string]gasTypes.AlertMetricDelta)

	// Nil CLS in snapshot
	cm.checkCLSRegression(metrics,
		performance.PerformanceSnapshot{CLS: nil},
		performance.PerformanceBaseline{CLS: float64Ptr(0.05)})
	if len(metrics) != 0 {
		t.Error("nil snapshot CLS should not trigger")
	}

	// Nil CLS in baseline
	cm.checkCLSRegression(metrics,
		performance.PerformanceSnapshot{CLS: float64Ptr(0.3)},
		performance.PerformanceBaseline{CLS: nil})
	if len(metrics) != 0 {
		t.Error("nil baseline CLS should not trigger")
	}
}

// ============================================
// checkCLSRegression: below threshold
// ============================================

func TestCheckCLSRegression_BelowThreshold(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	metrics := make(map[string]gasTypes.AlertMetricDelta)

	// Delta = 0.05, threshold is 0.1
	cm.checkCLSRegression(metrics,
		performance.PerformanceSnapshot{CLS: float64Ptr(0.10)},
		performance.PerformanceBaseline{CLS: float64Ptr(0.05)})

	if len(metrics) != 0 {
		t.Error("CLS delta of 0.05 should not trigger threshold of 0.1")
	}
}

// ============================================
// checkCLSRegression: above threshold with zero baseline
// ============================================

func TestCheckCLSRegression_AboveThresholdZeroBaseline(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	metrics := make(map[string]gasTypes.AlertMetricDelta)

	// Baseline CLS = 0, Snapshot CLS = 0.15 -> delta = 0.15 > 0.1
	cm.checkCLSRegression(metrics,
		performance.PerformanceSnapshot{CLS: float64Ptr(0.15)},
		performance.PerformanceBaseline{CLS: float64Ptr(0.0)})

	if len(metrics) != 1 {
		t.Fatalf("CLS delta > threshold should trigger, got %v", metrics)
	}
	m := metrics["cls"]
	if m.Baseline != 0 {
		t.Errorf("baseline = %f, want 0", m.Baseline)
	}
	if m.Current != 0.15 {
		t.Errorf("current = %f, want 0.15", m.Current)
	}
	// Percent should be 0 when baseline is 0
	if m.DeltaPct != 0 {
		t.Errorf("deltaPct = %f, want 0 for zero baseline", m.DeltaPct)
	}
}

// ============================================
// checkTransferRegression: zero baseline
// ============================================

func TestCheckTransferRegression_ZeroBaseline(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	metrics := make(map[string]gasTypes.AlertMetricDelta)

	cm.checkTransferRegression(metrics,
		performance.PerformanceSnapshot{Network: performance.NetworkSummary{TransferSize: 5000}},
		performance.PerformanceBaseline{Network: performance.BaselineNetwork{TransferSize: 0}})

	if len(metrics) != 0 {
		t.Error("zero baseline transfer size should not trigger regression")
	}
}

// ============================================
// checkTransferRegression: below threshold
// ============================================

func TestCheckTransferRegression_BelowThreshold(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	metrics := make(map[string]gasTypes.AlertMetricDelta)

	// 10% increase, threshold is 25%
	cm.checkTransferRegression(metrics,
		performance.PerformanceSnapshot{Network: performance.NetworkSummary{TransferSize: 1100}},
		performance.PerformanceBaseline{Network: performance.BaselineNetwork{TransferSize: 1000}})

	if len(metrics) != 0 {
		t.Error("10%% increase should not trigger 25%% threshold")
	}
}

// ============================================
// checkTransferRegression: above threshold
// ============================================

func TestCheckTransferRegression_AboveThreshold(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	metrics := make(map[string]gasTypes.AlertMetricDelta)

	// 30% increase, threshold is 25%
	cm.checkTransferRegression(metrics,
		performance.PerformanceSnapshot{Network: performance.NetworkSummary{TransferSize: 1300}},
		performance.PerformanceBaseline{Network: performance.BaselineNetwork{TransferSize: 1000}})

	if len(metrics) != 1 {
		t.Fatalf("30%% increase should trigger 25%% threshold, got %v", metrics)
	}
	m := metrics["transfer_bytes"]
	if m.Baseline != 1000 {
		t.Errorf("baseline = %f, want 1000", m.Baseline)
	}
	if m.Current != 1300 {
		t.Errorf("current = %f, want 1300", m.Current)
	}
}

// ============================================
// DetectAndStoreAlerts: resolves existing alerts for same URL before adding new
// ============================================

func TestDetectAndStoreAlerts_ResolvesBeforeAdding(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	baseline := performance.PerformanceBaseline{
		SampleCount: 2,
		Timing:      performance.BaselineTiming{Load: 1000},
	}

	// First regression
	cm.DetectAndStoreAlerts(performance.PerformanceSnapshot{
		URL:    "https://example.com/page",
		Timing: performance.PerformanceTiming{Load: 1300},
	}, baseline)

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(cm.pendingAlerts))
	}
	firstID := cm.pendingAlerts[0].ID

	// Second regression for same URL: replaces first
	cm.DetectAndStoreAlerts(performance.PerformanceSnapshot{
		URL:    "https://example.com/page",
		Timing: performance.PerformanceTiming{Load: 1500},
	}, baseline)

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("expected 1 alert after replacement, got %d", len(cm.pendingAlerts))
	}
	if cm.pendingAlerts[0].ID == firstID {
		t.Error("alert should be replaced with a new one")
	}
}

// ============================================
// DetectAndStoreAlerts: FCP and LCP regression with nil pointers
// ============================================

func TestDetectAndStoreAlerts_FCPLCPNilPointers(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	// Baseline has FCP/LCP, snapshot doesn't -> should still detect load regression
	baseline := performance.PerformanceBaseline{
		SampleCount: 2,
		Timing: performance.BaselineTiming{
			Load:                   1000,
			FirstContentfulPaint:   float64Ptr(800),
			LargestContentfulPaint: float64Ptr(1200),
		},
	}

	snapshot := performance.PerformanceSnapshot{
		URL: "https://example.com/test",
		Timing: performance.PerformanceTiming{
			Load:                   1300,
			FirstContentfulPaint:   nil, // nil FCP
			LargestContentfulPaint: nil, // nil LCP
		},
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("expected 1 alert for load regression, got %d", len(cm.pendingAlerts))
	}
	// Should have load metric but NOT fcp/lcp
	if _, ok := cm.pendingAlerts[0].Metrics["load"]; !ok {
		t.Error("expected load regression metric")
	}
	if _, ok := cm.pendingAlerts[0].Metrics["fcp"]; ok {
		t.Error("fcp should not be detected when snapshot has nil FCP")
	}
	if _, ok := cm.pendingAlerts[0].Metrics["lcp"]; ok {
		t.Error("lcp should not be detected when snapshot has nil LCP")
	}
}

// ============================================
// DetectAndStoreAlerts: alert fields populated correctly
// ============================================

func TestDetectAndStoreAlerts_AlertFieldsPopulated(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	baseline := performance.PerformanceBaseline{
		SampleCount: 2,
		Timing:      performance.BaselineTiming{Load: 1000},
	}

	snapshot := performance.PerformanceSnapshot{
		URL:    "https://example.com/fields-test",
		Timing: performance.PerformanceTiming{Load: 1300},
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(cm.pendingAlerts))
	}

	alert := cm.pendingAlerts[0]
	if alert.ID != 1 {
		t.Errorf("alert.ID = %d, want 1", alert.ID)
	}
	if alert.Type != "regression" {
		t.Errorf("alert.Type = %q, want 'regression'", alert.Type)
	}
	if alert.URL != "https://example.com/fields-test" {
		t.Errorf("alert.URL = %q, want correct URL", alert.URL)
	}
	if alert.DetectedAt == "" {
		t.Error("alert.DetectedAt should not be empty")
	}
	if alert.Summary == "" {
		t.Error("alert.Summary should not be empty")
	}
	if alert.Recommendation == "" {
		t.Error("alert.Recommendation should not be empty")
	}
	if alert.DeliveredAt != 0 {
		t.Errorf("alert.DeliveredAt = %d, want 0 (not yet delivered)", alert.DeliveredAt)
	}
}

// ============================================
// resolveAlertsForURL: selective removal
// ============================================

func TestResolveAlertsForURL_SelectiveRemoval(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	cm.pendingAlerts = []gasTypes.PerformanceAlert{
		{ID: 1, URL: "https://a.com"},
		{ID: 2, URL: "https://b.com"},
		{ID: 3, URL: "https://a.com"},
	}

	cm.resolveAlertsForURL("https://a.com")

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("expected 1 remaining alert, got %d", len(cm.pendingAlerts))
	}
	if cm.pendingAlerts[0].URL != "https://b.com" {
		t.Errorf("remaining alert URL = %q, want https://b.com", cm.pendingAlerts[0].URL)
	}
}

// ============================================
// resolveAlertsForURL: no matching URL
// ============================================

func TestResolveAlertsForURL_NoMatch(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	cm.pendingAlerts = []gasTypes.PerformanceAlert{
		{ID: 1, URL: "https://a.com"},
		{ID: 2, URL: "https://b.com"},
	}

	cm.resolveAlertsForURL("https://c.com")

	if len(cm.pendingAlerts) != 2 {
		t.Fatalf("expected 2 remaining alerts, got %d", len(cm.pendingAlerts))
	}
}

// ============================================
// getPendingAlerts: delivery tracking
// ============================================

func TestGetPendingAlerts_DeliveryTracking(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	cm.pendingAlerts = []gasTypes.PerformanceAlert{
		{ID: 1, DeliveredAt: 0},          // not delivered
		{ID: 2, DeliveredAt: 5},          // delivered at counter 5
		{ID: 3, DeliveredAt: 0},          // not delivered
	}

	// Checkpoint delivery counter = 0: all non-delivered + those delivered after 0
	alerts := cm.getPendingAlerts(0)
	if len(alerts) != 3 {
		t.Errorf("getPendingAlerts(0) = %d, want 3 (all)", len(alerts))
	}

	// Checkpoint delivery counter = 5: only non-delivered
	alerts = cm.getPendingAlerts(5)
	if len(alerts) != 2 {
		t.Errorf("getPendingAlerts(5) = %d, want 2 (non-delivered)", len(alerts))
	}

	// Checkpoint delivery counter = 10: only non-delivered
	alerts = cm.getPendingAlerts(10)
	if len(alerts) != 2 {
		t.Errorf("getPendingAlerts(10) = %d, want 2", len(alerts))
	}
}

// ============================================
// markAlertsDelivered: idempotent
// ============================================

func TestMarkAlertsDelivered_Idempotent(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	cm.alertDelivery = 7
	cm.pendingAlerts = []gasTypes.PerformanceAlert{
		{ID: 1, DeliveredAt: 0},
		{ID: 2, DeliveredAt: 3}, // already delivered
	}

	cm.markAlertsDelivered()

	if cm.pendingAlerts[0].DeliveredAt != 7 {
		t.Errorf("alert[0].DeliveredAt = %d, want 7", cm.pendingAlerts[0].DeliveredAt)
	}
	if cm.pendingAlerts[1].DeliveredAt != 3 {
		t.Errorf("alert[1].DeliveredAt = %d, want 3 (unchanged)", cm.pendingAlerts[1].DeliveredAt)
	}

	// Mark again (should not change already-delivered alerts)
	cm.alertDelivery = 10
	cm.markAlertsDelivered()

	if cm.pendingAlerts[0].DeliveredAt != 7 {
		t.Errorf("already-delivered alert[0].DeliveredAt = %d, want 7 (unchanged)", cm.pendingAlerts[0].DeliveredAt)
	}
}

// ============================================
// buildTimingChecks: nil FCP and LCP
// ============================================

func TestBuildTimingChecks_NilFCPAndLCP(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	snapshot := performance.PerformanceSnapshot{
		Timing: performance.PerformanceTiming{
			Load:                   1000,
			TimeToFirstByte:        200,
			FirstContentfulPaint:   nil,
			LargestContentfulPaint: nil,
		},
	}
	baseline := performance.PerformanceBaseline{
		SampleCount: 2,
		Timing: performance.BaselineTiming{
			Load:                   800,
			TimeToFirstByte:        150,
			FirstContentfulPaint:   nil,
			LargestContentfulPaint: nil,
		},
	}

	checks := cm.buildTimingChecks(snapshot, baseline)
	if len(checks) != 2 {
		t.Errorf("expected 2 checks (load, ttfb) when FCP/LCP nil, got %d", len(checks))
	}
}

// ============================================
// buildTimingChecks: all fields present
// ============================================

func TestBuildTimingChecks_AllPresent(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	snapshot := performance.PerformanceSnapshot{
		Timing: performance.PerformanceTiming{
			Load:                   1000,
			TimeToFirstByte:        200,
			FirstContentfulPaint:   float64Ptr(800),
			LargestContentfulPaint: float64Ptr(1200),
		},
	}
	baseline := performance.PerformanceBaseline{
		SampleCount: 2,
		Timing: performance.BaselineTiming{
			Load:                   800,
			TimeToFirstByte:        150,
			FirstContentfulPaint:   float64Ptr(600),
			LargestContentfulPaint: float64Ptr(1000),
		},
	}

	checks := cm.buildTimingChecks(snapshot, baseline)
	if len(checks) != 4 {
		t.Errorf("expected 4 checks (load, ttfb, fcp, lcp), got %d", len(checks))
	}

	// Verify check names
	names := make(map[string]bool)
	for _, c := range checks {
		names[c.name] = true
	}
	for _, expected := range []string{"load", "ttfb", "fcp", "lcp"} {
		if !names[expected] {
			t.Errorf("missing timing check: %s", expected)
		}
	}
}
