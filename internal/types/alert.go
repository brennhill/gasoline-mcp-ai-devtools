// alert.go — Alert types for proactive error notification and performance regression detection.
// Zero dependencies - foundational types used across ai, session, and observation packages.
package types

import "time"

// ============================================
// Immediate Alerts
// ============================================

// Alert represents a server-generated alert that piggybacks on observe responses.
// Typically created by monitoring incoming browser events and detecting errors, network failures, etc.
type Alert struct {
	Severity  string `json:"severity"`         // "info", "warning", "error"
	Category  string `json:"category"`         // "regression", "anomaly", "ci", "noise", "threshold"
	Title     string `json:"title"`            // Short summary
	Detail    string `json:"detail,omitempty"` // Longer explanation
	Timestamp string `json:"timestamp"`        // ISO 8601
	Source    string `json:"source"`           // What generated it
	Count     int    `json:"count,omitempty"`  // Deduplication count (>1 means repeated)
}

// ============================================
// CI/CD Integration
// ============================================

// CIResult stores a CI/CD webhook result.
type CIResult struct {
	Status     string      `json:"status"`      // "success", "failure", "error"
	Source     string      `json:"source"`      // "github-actions", "gitlab-ci", "custom"
	Ref        string      `json:"ref"`         // Branch ref
	Commit     string      `json:"commit"`      // Commit SHA
	Summary    string      `json:"summary"`     // Human-readable summary
	Failures   []CIFailure `json:"failures"`    // Failed tests
	URL        string      `json:"url"`         // Link to CI run
	DurationMs int         `json:"duration_ms"` // Build duration
	ReceivedAt time.Time   `json:"-"`           // When we received it
}

// CIFailure represents a single test failure in a CI result.
type CIFailure struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// ============================================
// Performance Regression Alerts
// ============================================

// PerformanceAlert represents a pending regression alert to be delivered via get_changes_since.
// Tracks performance regressions detected across development sessions.
type PerformanceAlert struct {
	ID             int64                   `json:"id"`
	Type           string                  `json:"type"`
	URL            string                  `json:"url"`
	DetectedAt     string                  `json:"detected_at"`
	Summary        string                  `json:"summary"`
	Metrics        map[string]AlertMetricDelta `json:"metrics"`
	Recommendation string                  `json:"recommendation"`
	// Internal tracking (not serialized to JSON response)
	DeliveredAt int64 // checkpoint counter at which this was delivered
}

// AlertMetricDelta describes the delta for a single regressed metric.
// Used within PerformanceAlert to show before/after and percentage change.
type AlertMetricDelta struct {
	Baseline float64 `json:"baseline"`
	Current  float64 `json:"current"`
	DeltaMs  float64 `json:"delta_ms"`
	DeltaPct float64 `json:"delta_pct"`
}
