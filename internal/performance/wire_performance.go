// wire_performance.go — Wire types for performance snapshots over HTTP.
// Defines the JSON fields sent by the extension for performance data.
// Changes here MUST be mirrored in src/types/wire-performance-snapshot.ts.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
package performance

// WirePerformanceTiming holds navigation timing metrics from the extension.
type WirePerformanceTiming struct {
	DomContentLoaded       float64  `json:"dom_content_loaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"first_contentful_paint"`
	LargestContentfulPaint *float64 `json:"largest_contentful_paint"`
	InteractionToNextPaint *float64 `json:"interaction_to_next_paint,omitempty"`
	TimeToFirstByte        float64  `json:"time_to_first_byte"`
	DomInteractive         float64  `json:"dom_interactive"`
}

// WireTypeSummary holds per-type resource metrics.
type WireTypeSummary struct {
	Count int   `json:"count"`
	Size  int64 `json:"size"`
}

// WireSlowRequest represents one of the slowest network requests.
type WireSlowRequest struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
}

// WireNetworkSummary holds aggregated network resource metrics from the extension.
type WireNetworkSummary struct {
	RequestCount    int                        `json:"request_count"`
	TransferSize    int64                      `json:"transfer_size"`
	DecodedSize     int64                      `json:"decoded_size"`
	ByType          map[string]WireTypeSummary `json:"by_type"`
	SlowestRequests []WireSlowRequest          `json:"slowest_requests"`
}

// WireLongTaskMetrics holds accumulated long task data from the extension.
type WireLongTaskMetrics struct {
	Count             int     `json:"count"`
	TotalBlockingTime float64 `json:"total_blocking_time"`
	Longest           float64 `json:"longest"`
}

// WireUserTimingEntry represents a single performance mark or measure.
type WireUserTimingEntry struct {
	Name      string  `json:"name"`
	StartTime float64 `json:"start_time"`
	Duration  float64 `json:"duration,omitempty"`
}

// WireUserTimingData holds captured performance.mark() and performance.measure() entries.
type WireUserTimingData struct {
	Marks    []WireUserTimingEntry `json:"marks"`
	Measures []WireUserTimingEntry `json:"measures"`
}

// WirePerformanceSnapshot is the canonical wire format for performance data.
type WirePerformanceSnapshot struct {
	URL        string              `json:"url"`
	Timestamp  string              `json:"timestamp"`
	Timing     WirePerformanceTiming `json:"timing"`
	Network    WireNetworkSummary  `json:"network"`
	LongTasks  WireLongTaskMetrics `json:"long_tasks"`
	CLS        *float64            `json:"cumulative_layout_shift,omitempty"`
	UserTiming *WireUserTimingData `json:"user_timing,omitempty"`
}
