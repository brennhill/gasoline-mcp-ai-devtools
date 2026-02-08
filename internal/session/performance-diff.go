// performance-diff.go — Performance diff computation.
// diffPerformance and computeMetricChange functions.
package session

import (
	"fmt"
)

// diffPerformance compares performance metrics between two snapshots.
func (sm *SessionManager) diffPerformance(a, b *NamedSnapshot) PerformanceDiff {
	diff := PerformanceDiff{}

	if a.Performance == nil || b.Performance == nil {
		return diff
	}

	// Load time comparison
	if a.Performance.Timing.Load > 0 || b.Performance.Timing.Load > 0 {
		diff.LoadTime = computeMetricChange(
			a.Performance.Timing.Load,
			b.Performance.Timing.Load,
		)
	}

	// Request count comparison
	if a.Performance.Network.RequestCount > 0 || b.Performance.Network.RequestCount > 0 {
		diff.RequestCount = computeMetricChange(
			float64(a.Performance.Network.RequestCount),
			float64(b.Performance.Network.RequestCount),
		)
	}

	// Transfer size comparison
	if a.Performance.Network.TransferSize > 0 || b.Performance.Network.TransferSize > 0 {
		diff.TransferSize = computeMetricChange(
			float64(a.Performance.Network.TransferSize),
			float64(b.Performance.Network.TransferSize),
		)
	}

	return diff
}

// computeMetricChange creates a MetricChange comparing two values.
func computeMetricChange(before, after float64) *MetricChange {
	mc := &MetricChange{
		Before: before,
		After:  after,
	}

	if before == 0 {
		if after > 0 {
			mc.Change = "+inf"
			mc.Regression = true
		} else {
			mc.Change = "0%"
		}
		return mc
	}

	pctChange := ((after - before) / before) * 100
	if pctChange >= 0 {
		mc.Change = fmt.Sprintf("+%.0f%%", pctChange)
	} else {
		mc.Change = fmt.Sprintf("%.0f%%", pctChange)
	}

	// Regression = after > before * threshold
	mc.Regression = after > before*perfRegressionRatio

	return mc
}
