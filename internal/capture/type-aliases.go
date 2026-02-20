// Purpose: Owns type-aliases.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// type-aliases.go — Type aliases for imported packages.
// These are real type aliases (= syntax), not forward declarations.
// They provide convenience by avoiding qualifying imported types everywhere.
package capture

import (
	"github.com/dev-console/dev-console/internal/performance"
	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/recording"
)

// Type aliases for imported packages to avoid qualifying every use.
// These are real type aliases (= syntax), not any forward declarations.
type (
	PerformanceSnapshot   = performance.PerformanceSnapshot   // Alias for convenience (avoid qualifying as performance.PerformanceSnapshot everywhere)
	PerformanceBaseline   = performance.PerformanceBaseline   // Alias for convenience
	PerformanceRegression = performance.PerformanceRegression // Alias for convenience
	ResourceEntry         = performance.ResourceEntry         // Alias for convenience
	ResourceDiff          = performance.ResourceDiff          // Alias for convenience
	CausalDiffResult      = performance.CausalDiffResult      // Alias for convenience
	Recording             = recording.Recording               // Alias for convenience (avoid qualifying as recording.Recording everywhere)
	RecordingAction       = recording.RecordingAction         // Alias for convenience
	PendingQueryResponse  = queries.PendingQueryResponse      // Alias for convenience (avoid qualifying as queries.PendingQueryResponse everywhere)
	PendingQuery          = queries.PendingQuery              // Alias for convenience
	CommandResult         = queries.CommandResult             // Alias for convenience (avoid qualifying as queries.CommandResult everywhere)

	// QueryDispatcher subsystem types — moved to internal/queries package.
	QueryDispatcher = queries.QueryDispatcher // Query lifecycle, result storage, async command tracking
	QuerySnapshot   = queries.QuerySnapshot   // Point-in-time view of query state for health reporting

	// Recording subsystem types — moved to internal/recording package.
	RecordingManager = recording.RecordingManager // Recording lifecycle, playback, and log-diff engine
	StorageInfo      = recording.StorageInfo      // Recording storage usage info
	PlaybackSession  = recording.PlaybackSession  // Active playback session state
	PlaybackResult   = recording.PlaybackResult   // Result of executing a single recorded action
	Coordinates      = recording.Coordinates      // X/Y position on the page
	LogDiffResult    = recording.LogDiffResult    // Comparison of two recordings
	DiffLogEntry     = recording.DiffLogEntry     // Single log entry for diff comparison
	ValueChange      = recording.ValueChange      // Field value change between recordings
	ActionComparison = recording.ActionComparison // Action counts and types between recordings
)
