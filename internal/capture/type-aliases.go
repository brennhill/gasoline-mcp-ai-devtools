// Purpose: Re-exports subsystem type aliases so capture can compose extracted packages without churn.
// Why: Preserves capture package API/readability during modularization into circuit/queries/recording packages.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/circuit"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/performance"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/recording"
)

// Type aliases for imported packages to avoid qualifying every use.
// These are real type aliases (= syntax), not any forward declarations.
type (
	// Store is the preferred non-stuttering name for the package's primary state container.
	// Backward compatibility: Capture remains available as an alias target.
	Store = Capture
	// Snapshot is the preferred non-stuttering name for CaptureSnapshot.
	// Backward compatibility: CaptureSnapshot remains available as an alias target.
	Snapshot = CaptureSnapshot

	PerformanceSnapshot   = performance.Snapshot         // Alias for convenience (avoid qualifying as performance.Snapshot everywhere)
	PerformanceBaseline   = performance.Baseline         // Alias for convenience
	PerformanceRegression = performance.Regression       // Alias for convenience
	ResourceEntry         = performance.ResourceEntry    // Alias for convenience
	ResourceDiff          = performance.ResourceDiff     // Alias for convenience
	CausalDiffResult      = performance.CausalDiffResult // Alias for convenience
	Recording             = recording.Item               // Alias for convenience (avoid qualifying as recording.Item everywhere)
	RecordingAction       = recording.Action             // Alias for convenience
	PendingQueryResponse  = queries.PendingQueryResponse // Alias for convenience (avoid qualifying as queries.PendingQueryResponse everywhere)
	PendingQuery          = queries.PendingQuery         // Alias for convenience
	CommandResult         = queries.CommandResult        // Alias for convenience (avoid qualifying as queries.CommandResult everywhere)

	// QueryDispatcher subsystem types — moved to internal/queries package.
	QueryDispatcher = queries.QueryDispatcher // Query lifecycle, result storage, async command tracking
	QuerySnapshot   = queries.QuerySnapshot   // Point-in-time view of query state for health reporting

	// Circuit breaker subsystem types — moved to internal/circuit package.
	CircuitBreaker    = circuit.CircuitBreaker    // Rate limiting + circuit breaker state machine
	HealthResponse    = circuit.HealthResponse    // GET /health response
	RateLimitResponse = circuit.RateLimitResponse // 429 response body

	// Recording subsystem types — moved to internal/recording package.
	RecordingManager = recording.Manager          // Recording lifecycle, playback, and log-diff engine
	StorageInfo      = recording.StorageInfo      // Recording storage usage info
	PlaybackSession  = recording.PlaybackSession  // Active playback session state
	PlaybackResult   = recording.PlaybackResult   // Result of executing a single recorded action
	Coordinates      = recording.Coordinates      // X/Y position on the page
	LogDiffResult    = recording.LogDiffResult    // Comparison of two recordings
	DiffLogEntry     = recording.DiffLogEntry     // Single log entry for diff comparison
	ValueChange      = recording.ValueChange      // Field value change between recordings
	ActionComparison = recording.ActionComparison // Action counts and types between recordings
)

// NewCircuitBreaker is re-exported from internal/circuit for backward compatibility.
var NewCircuitBreaker = circuit.NewCircuitBreaker

// NewStore is the preferred constructor name for Store.
// Backward compatibility: NewCapture remains available.
func NewStore() *Store { return NewCapture() }
