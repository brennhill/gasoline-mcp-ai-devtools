// types.go — Performance monitoring types
// Handles performance snapshots, baselines, regression detection, and
// causal diffing for identifying performance bottlenecks.
package performance

// ============================================
// Performance Budget Types
// ============================================

// PerformanceSnapshot represents a captured performance snapshot from a page load
type PerformanceSnapshot struct {
	URL       string            `json:"url"`
	Timestamp string            `json:"timestamp"`
	Timing    PerformanceTiming `json:"timing"`
	Network   NetworkSummary    `json:"network"`
	LongTasks LongTaskMetrics   `json:"long_tasks"`
	CLS       *float64          `json:"cumulative_layout_shift,omitempty"` // snake_case (from browser LayoutShift)
	Resources []ResourceEntry   `json:"resources,omitempty"`
}

// PerformanceTiming holds navigation timing metrics
type PerformanceTiming struct {
	DomContentLoaded       float64  `json:"dom_content_loaded"`              // snake_case (from browser PerformanceTiming)
	Load                   float64  `json:"load"`                          // snake_case (from browser PerformanceTiming)
	FirstContentfulPaint   *float64 `json:"first_contentful_paint"`          // snake_case (from browser PerformancePaintTiming)
	LargestContentfulPaint *float64 `json:"largest_contentful_paint"`        // snake_case (from browser LargestContentfulPaint)
	InteractionToNextPaint *float64 `json:"interaction_to_next_paint,omitempty"` // snake_case (from browser EventTiming)
	TimeToFirstByte        float64  `json:"time_to_first_byte"`               // snake_case (from browser PerformanceTiming)
	DomInteractive         float64  `json:"dom_interactive"`                // snake_case (from browser PerformanceTiming)
}

// NetworkSummary holds aggregated network resource metrics
type NetworkSummary struct {
	RequestCount    int                    `json:"request_count"`
	TransferSize    int64                  `json:"transfer_size"`
	DecodedSize     int64                  `json:"decoded_size"`
	ByType          map[string]TypeSummary `json:"by_type"`
	SlowestRequests []SlowRequest          `json:"slowest_requests"`
}

// TypeSummary holds per-type resource metrics
type TypeSummary struct {
	Count int   `json:"count"`
	Size  int64 `json:"size"`
}

// SlowRequest represents one of the slowest network requests
type SlowRequest struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
}

// LongTaskMetrics holds accumulated long task data
type LongTaskMetrics struct {
	Count             int     `json:"count"`
	TotalBlockingTime float64 `json:"total_blocking_time"`
	Longest           float64 `json:"longest"`
}

// PerformanceBaseline holds averaged performance data for a URL path
type PerformanceBaseline struct {
	URL         string          `json:"url"`
	SampleCount int             `json:"sample_count"`
	LastUpdated string          `json:"last_updated"`
	Timing      BaselineTiming  `json:"timing"`
	Network     BaselineNetwork `json:"network"`
	LongTasks   LongTaskMetrics `json:"long_tasks"`
	CLS         *float64        `json:"cumulative_layout_shift,omitempty"` // snake_case (from browser LayoutShift)
	Resources   []ResourceEntry `json:"resources,omitempty"`
}

// BaselineTiming holds averaged timing metrics
type BaselineTiming struct {
	DomContentLoaded       float64  `json:"dom_content_loaded"`              // snake_case (from browser PerformanceTiming)
	Load                   float64  `json:"load"`                          // snake_case (from browser PerformanceTiming)
	FirstContentfulPaint   *float64 `json:"first_contentful_paint"`          // snake_case (from browser PerformancePaintTiming)
	LargestContentfulPaint *float64 `json:"largest_contentful_paint"`        // snake_case (from browser LargestContentfulPaint)
	TimeToFirstByte        float64  `json:"time_to_first_byte"`               // snake_case (from browser PerformanceTiming)
	DomInteractive         float64  `json:"dom_interactive"`                // snake_case (from browser PerformanceTiming)
}

// BaselineNetwork holds averaged network metrics
type BaselineNetwork struct {
	RequestCount int   `json:"request_count"`
	TransferSize int64 `json:"transfer_size"`
}

// PerformanceRegression describes a detected performance regression
type PerformanceRegression struct {
	Metric         string  `json:"metric"`
	Current        float64 `json:"current"`
	Baseline       float64 `json:"baseline"`
	ChangePercent  float64 `json:"change_percent"`
	AbsoluteChange float64 `json:"absolute_change"`
}

// ============================================
// Causal Diffing Types
// ============================================

// ResourceEntry represents a single resource in a performance snapshot fingerprint
type ResourceEntry struct {
	URL            string  `json:"url"`
	Type           string  `json:"type"`
	TransferSize   int64   `json:"transfer_size"`              // snake_case (from browser PerformanceResourceTiming)
	Duration       float64 `json:"duration"`                  // snake_case (from browser PerformanceResourceTiming)
	RenderBlocking bool    `json:"renderBlocking,omitempty"`  // snake_case (from browser PerformanceResourceTiming)
}

// ResourceDiff holds the categorized differences between baseline and current resources
type ResourceDiff struct {
	Added   []AddedResource   `json:"added"`
	Removed []RemovedResource `json:"removed"`
	Resized []ResizedResource `json:"resized"`
	Retimed []RetimedResource `json:"retimed"`
}

// AddedResource is a resource present in current but not in baseline
type AddedResource struct {
	URL            string  `json:"url"`
	Type           string  `json:"type"`
	SizeBytes      int64   `json:"size_bytes"`
	DurationMs     float64 `json:"duration_ms"`
	RenderBlocking bool    `json:"render_blocking"`
}

// RemovedResource is a resource present in baseline but not in current
type RemovedResource struct {
	URL       string `json:"url"`
	Type      string `json:"type"`
	SizeBytes int64  `json:"size_bytes"`
}

// ResizedResource is a resource present in both with significant size change
type ResizedResource struct {
	URL           string `json:"url"`
	BaselineBytes int64  `json:"baseline_bytes"`
	CurrentBytes  int64  `json:"current_bytes"`
	DeltaBytes    int64  `json:"delta_bytes"`
}

// RetimedResource is a resource present in both with significant duration change
type RetimedResource struct {
	URL        string  `json:"url"`
	BaselineMs float64 `json:"baseline_ms"`
	CurrentMs  float64 `json:"current_ms"`
	DeltaMs    float64 `json:"delta_ms"`
}

// TimingDelta holds the timing differences between baseline and current
type TimingDelta struct {
	LoadMs float64 `json:"load_ms"`
	FCPMs  float64 `json:"fcp_ms"`
	LCPMs  float64 `json:"lcp_ms"`
}

// CausalDiffResult is the full response from the get_causal_diff tool
type CausalDiffResult struct {
	URL             string       `json:"url"`
	TimingDelta     TimingDelta  `json:"timing_delta"`
	ResourceChanges ResourceDiff `json:"resource_changes"`
	ProbableCause   string       `json:"probable_cause"`
	Recommendations []string     `json:"recommendations"`
}

// ============================================
// Constants
// ============================================

const (
	// MaxPerfSnapshots is the maximum number of performance snapshots to retain
	MaxPerfSnapshots = 20

	// MaxPerfBaselines is the maximum number of performance baselines to retain
	MaxPerfBaselines = 20
)
