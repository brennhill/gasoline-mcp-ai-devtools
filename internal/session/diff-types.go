// Purpose: Implements session lifecycle, snapshots, and diff state management.
// Docs: docs/features/feature/observe/index.md
// Docs: docs/features/feature/pagination/index.md

// diff-types.go — Diff computation types.
// SessionDiffResult, ErrorDiff, SessionNetworkDiff, PerformanceDiff, etc.
package session

// SessionDiffResult is the full comparison result between two snapshots.
type SessionDiffResult struct {
	A           string             `json:"a"`
	B           string             `json:"b"`
	Errors      ErrorDiff          `json:"errors"`
	Network     SessionNetworkDiff `json:"network"`
	Performance PerformanceDiff    `json:"performance"`
	Summary     DiffSummary        `json:"summary"`
}

// ErrorDiff holds the error comparison between two snapshots.
type ErrorDiff struct {
	New       []SnapshotError `json:"new"`
	Resolved  []SnapshotError `json:"resolved"`
	Unchanged []SnapshotError `json:"unchanged"`
}

// SessionNetworkDiff holds the network comparison between two snapshots.
type SessionNetworkDiff struct {
	NewErrors        []SnapshotNetworkRequest `json:"new_errors"`
	StatusChanges    []SessionNetworkChange   `json:"status_changes"`
	NewEndpoints     []SnapshotNetworkRequest `json:"new_endpoints"`
	MissingEndpoints []SnapshotNetworkRequest `json:"missing_endpoints"`
}

// SessionNetworkChange represents a status code change for the same endpoint.
type SessionNetworkChange struct {
	Method         string `json:"method"`
	URL            string `json:"url"`
	BeforeStatus   int    `json:"before"`
	AfterStatus    int    `json:"after"`
	DurationChange string `json:"duration_change,omitempty"`
}

// PerformanceDiff holds performance metric comparisons.
type PerformanceDiff struct {
	LoadTime     *MetricChange `json:"load_time,omitempty"`
	RequestCount *MetricChange `json:"request_count,omitempty"`
	TransferSize *MetricChange `json:"transfer_size,omitempty"`
}

// MetricChange holds before/after values for a numeric metric.
type MetricChange struct {
	Before     float64 `json:"before"`
	After      float64 `json:"after"`
	Change     string  `json:"change"`
	Regression bool    `json:"regression"`
}

// DiffSummary holds aggregate comparison stats and verdict.
type DiffSummary struct {
	Verdict                string `json:"verdict"`
	NewErrors              int    `json:"new_errors"`
	ResolvedErrors         int    `json:"resolved_errors"`
	PerformanceRegressions int    `json:"performance_regressions"`
	NewNetworkErrors       int    `json:"new_network_errors"`
}
