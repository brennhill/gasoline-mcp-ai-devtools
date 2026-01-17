// types.go — Session-level tracking for workflow integration
// Tracks performance changes across development sessions, detecting
// regressions and generating alerts.
package session

// ============================================
// Session Tracking Types
// ============================================

// NOTE: SessionTracker is defined in internal/capture/types.go to avoid circular imports
// (capture needs SessionTracker, but session/client_registry imports capture for NetworkBody types)

// SessionSummary represents a compiled summary of a development session
type SessionSummary struct {
	Status           string            `json:"status"` // "ok", "no_performance_data", "insufficient_data"
	PerformanceDelta *PerformanceDelta `json:"performance_delta,omitempty"`
	Errors           []SessionError    `json:"errors,omitempty"`
	Metadata         SessionMetadata   `json:"metadata"`
}

// PerformanceDelta represents the net change in performance metrics during a session
type PerformanceDelta struct {
	LoadTimeBefore   float64 `json:"load_time_before"`
	LoadTimeAfter    float64 `json:"load_time_after"`
	LoadTimeDelta    float64 `json:"load_time_delta"`
	FCPBefore        float64 `json:"fcp_before,omitempty"`
	FCPAfter         float64 `json:"fcp_after,omitempty"`
	FCPDelta         float64 `json:"fcp_delta,omitempty"`
	LCPBefore        float64 `json:"lcp_before,omitempty"`
	LCPAfter         float64 `json:"lcp_after,omitempty"`
	LCPDelta         float64 `json:"lcp_delta,omitempty"`
	CLSBefore        float64 `json:"cls_before,omitempty"`
	CLSAfter         float64 `json:"cls_after,omitempty"`
	CLSDelta         float64 `json:"cls_delta,omitempty"`
	BundleSizeBefore int64   `json:"bundle_size_before"`
	BundleSizeAfter  int64   `json:"bundle_size_after"`
	BundleSizeDelta  int64   `json:"bundle_size_delta"`
}

// SessionError represents an error observed during a session
type SessionError struct {
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
	Resolved bool   `json:"resolved"`
}

// SessionMetadata holds session-level aggregate stats
type SessionMetadata struct {
	DurationMs            int64 `json:"duration_ms"`
	ReloadCount           int   `json:"reload_count"`
	PerformanceCheckCount int   `json:"performance_check_count"`
}

// ============================================
// Push Regression Alert Types
// ============================================

// PerformanceAlert represents a pending regression alert to be delivered via get_changes_since
type PerformanceAlert struct {
	ID             int64                       `json:"id"`
	Type           string                      `json:"type"`
	URL            string                      `json:"url"`
	DetectedAt     string                      `json:"detected_at"`
	Summary        string                      `json:"summary"`
	Metrics        map[string]AlertMetricDelta `json:"metrics"`
	Recommendation string                      `json:"recommendation"`
	// Internal tracking (not serialized to JSON response)
	DeliveredAt int64 // checkpoint counter at which this was delivered
}

// AlertMetricDelta describes the delta for a single regressed metric
type AlertMetricDelta struct {
	Baseline float64 `json:"baseline"`
	Current  float64 `json:"current"`
	DeltaMs  float64 `json:"delta_ms"`
	DeltaPct float64 `json:"delta_pct"`
}
