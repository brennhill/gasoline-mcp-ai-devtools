// ai_checkpoint_alerts.go — Push regression alert detection and delivery.
package ai

import (
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/performance"
	gasTypes "github.com/dev-console/dev-console/internal/types"
)

// ============================================
// Push Regression Alert Constants
// ============================================

const (
	maxPendingAlerts = 10

	// Regression thresholds (from spec)
	loadRegressionPct     = 20.0
	fcpRegressionPct      = 20.0
	lcpRegressionPct      = 20.0
	ttfbRegressionPct     = 50.0
	clsRegressionAbs      = 0.1
	transferRegressionPct = 25.0
)

// ============================================
// Push Regression Alert Detection
// ============================================

// DetectAndStoreAlerts checks the given performance snapshot against the given baseline
// and stores any regression alerts for delivery via get_changes_since.
// The baseline should be the state BEFORE the snapshot was incorporated.
func (cm *CheckpointManager) DetectAndStoreAlerts(snapshot performance.PerformanceSnapshot, baseline performance.PerformanceBaseline) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	url := snapshot.URL

	// Only alert if the baseline has more than 1 sample (first snapshot creates baseline, not alert)
	if baseline.SampleCount < 1 {
		return
	}

	// Detect regressions using push-notification thresholds
	metrics := cm.detectPushRegressions(snapshot, baseline)

	if len(metrics) == 0 {
		// No regression detected — check if any pending alert for this URL should be resolved
		cm.resolveAlertsForURL(url)
		return
	}

	// Remove any existing pending alert for this URL (replaced by the new one)
	cm.resolveAlertsForURL(url)

	// Build summary
	summary := cm.buildAlertSummary(url, metrics)

	// Create the alert
	cm.alertCounter++
	cm.alertDelivery++
	alert := gasTypes.PerformanceAlert{
		ID:             cm.alertCounter,
		Type:           "regression",
		URL:            url,
		DetectedAt:     time.Now().Format(time.RFC3339Nano),
		Summary:        summary,
		Metrics:        metrics,
		Recommendation: "Check recently added scripts or stylesheets. Use check_performance for full details.",
		DeliveredAt:    0, // not yet delivered
	}

	cm.pendingAlerts = append(cm.pendingAlerts, alert)

	// Cap at maxPendingAlerts, dropping oldest
	if len(cm.pendingAlerts) > maxPendingAlerts {
		keep := len(cm.pendingAlerts) - maxPendingAlerts
		surviving := make([]gasTypes.PerformanceAlert, maxPendingAlerts)
		copy(surviving, cm.pendingAlerts[keep:])
		cm.pendingAlerts = surviving
	}
}

// detectPushRegressions compares snapshot against baseline using the push-notification thresholds.
// Returns only metrics that exceed their thresholds.
func (cm *CheckpointManager) detectPushRegressions(snapshot performance.PerformanceSnapshot, baseline performance.PerformanceBaseline) map[string]gasTypes.AlertMetricDelta {
	metrics := make(map[string]gasTypes.AlertMetricDelta)

	// Percentage-based timing regressions (load, FCP, LCP, TTFB)
	timingChecks := cm.buildTimingChecks(snapshot, baseline)
	for _, check := range timingChecks {
		checkPctRegression(metrics, check.name, check.current, check.baseline, check.threshold)
	}

	// CLS: absolute increase threshold
	cm.checkCLSRegression(metrics, snapshot, baseline)

	// Transfer size: percentage-based
	cm.checkTransferRegression(metrics, snapshot, baseline)

	return metrics
}

// timingCheck holds the parameters for a single timing regression check.
type timingCheck struct {
	name      string
	current   float64
	baseline  float64
	threshold float64
}

// buildTimingChecks returns the list of timing metrics to evaluate.
func (cm *CheckpointManager) buildTimingChecks(snapshot performance.PerformanceSnapshot, baseline performance.PerformanceBaseline) []timingCheck {
	checks := []timingCheck{
		{"load", snapshot.Timing.Load, baseline.Timing.Load, loadRegressionPct},
		{"ttfb", snapshot.Timing.TimeToFirstByte, baseline.Timing.TimeToFirstByte, ttfbRegressionPct},
	}
	if snapshot.Timing.FirstContentfulPaint != nil && baseline.Timing.FirstContentfulPaint != nil {
		checks = append(checks, timingCheck{"fcp", *snapshot.Timing.FirstContentfulPaint, *baseline.Timing.FirstContentfulPaint, fcpRegressionPct})
	}
	if snapshot.Timing.LargestContentfulPaint != nil && baseline.Timing.LargestContentfulPaint != nil {
		checks = append(checks, timingCheck{"lcp", *snapshot.Timing.LargestContentfulPaint, *baseline.Timing.LargestContentfulPaint, lcpRegressionPct})
	}
	return checks
}

// checkPctRegression adds a metric delta if current exceeds baseline by more than threshold percent.
func checkPctRegression(metrics map[string]gasTypes.AlertMetricDelta, name string, current, baseline, threshold float64) {
	if baseline <= 0 {
		return
	}
	delta := current - baseline
	pct := delta / baseline * 100
	if pct > threshold {
		metrics[name] = gasTypes.AlertMetricDelta{
			Baseline: baseline,
			Current:  current,
			DeltaMs:  delta,
			DeltaPct: pct,
		}
	}
}

// checkCLSRegression checks for CLS absolute increase (>0.1).
func (cm *CheckpointManager) checkCLSRegression(metrics map[string]gasTypes.AlertMetricDelta, snapshot performance.PerformanceSnapshot, baseline performance.PerformanceBaseline) {
	if snapshot.CLS == nil || baseline.CLS == nil {
		return
	}
	delta := *snapshot.CLS - *baseline.CLS
	if delta <= clsRegressionAbs {
		return
	}
	pct := 0.0
	if *baseline.CLS > 0 {
		pct = delta / *baseline.CLS * 100
	}
	metrics["cls"] = gasTypes.AlertMetricDelta{
		Baseline: *baseline.CLS,
		Current:  *snapshot.CLS,
		DeltaMs:  delta, // for CLS this is the absolute delta, not ms
		DeltaPct: pct,
	}
}

// checkTransferRegression checks for total transfer size increase (>25%).
func (cm *CheckpointManager) checkTransferRegression(metrics map[string]gasTypes.AlertMetricDelta, snapshot performance.PerformanceSnapshot, baseline performance.PerformanceBaseline) {
	if baseline.Network.TransferSize <= 0 {
		return
	}
	delta := float64(snapshot.Network.TransferSize - baseline.Network.TransferSize)
	pct := delta / float64(baseline.Network.TransferSize) * 100
	if pct > transferRegressionPct {
		metrics["transfer_bytes"] = gasTypes.AlertMetricDelta{
			Baseline: float64(baseline.Network.TransferSize),
			Current:  float64(snapshot.Network.TransferSize),
			DeltaMs:  delta, // for transfer this is the byte delta
			DeltaPct: pct,
		}
	}
}

// resolveAlertsForURL removes any pending alerts for the given URL
func (cm *CheckpointManager) resolveAlertsForURL(url string) {
	// Use new slice to allow GC of resolved alerts (avoids [:0] backing-array pinning)
	filtered := make([]gasTypes.PerformanceAlert, 0, len(cm.pendingAlerts))
	for _, alert := range cm.pendingAlerts {
		if alert.URL != url {
			filtered = append(filtered, alert)
		}
	}
	cm.pendingAlerts = filtered
}

// buildAlertSummary generates a human-readable summary for an alert
func (cm *CheckpointManager) buildAlertSummary(url string, metrics map[string]gasTypes.AlertMetricDelta) string {
	if loadMetric, ok := metrics["load"]; ok {
		return fmt.Sprintf("Load time regressed by %.0fms (%.0fms -> %.0fms) on %s",
			loadMetric.DeltaMs, loadMetric.Baseline, loadMetric.Current, url)
	}
	// Fallback: mention the first metric found
	for name, metric := range metrics {
		return fmt.Sprintf("%s regressed by %.1f%% on %s", name, metric.DeltaPct, url)
	}
	return fmt.Sprintf("Performance regression detected on %s", url)
}

// ============================================
// Push Regression Alert Delivery
// ============================================

// getPendingAlerts returns alerts that should be included in the response
// based on the checkpoint's alertDelivery counter.
func (cm *CheckpointManager) getPendingAlerts(checkpointDelivery int64) []gasTypes.PerformanceAlert {
	var result []gasTypes.PerformanceAlert
	for i := range cm.pendingAlerts {
		// Include alerts that haven't been delivered yet, or were delivered after this checkpoint
		if cm.pendingAlerts[i].DeliveredAt == 0 || cm.pendingAlerts[i].DeliveredAt > checkpointDelivery {
			result = append(result, cm.pendingAlerts[i])
		}
	}
	return result
}

// markAlertsDelivered marks all pending alerts as delivered at the current delivery counter
func (cm *CheckpointManager) markAlertsDelivered() {
	for i := range cm.pendingAlerts {
		if cm.pendingAlerts[i].DeliveredAt == 0 {
			cm.pendingAlerts[i].DeliveredAt = cm.alertDelivery
		}
	}
}
