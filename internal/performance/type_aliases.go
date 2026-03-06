// Purpose: Provides non-stuttering type aliases for the performance package.
// Why: Improves call-site readability while preserving backward compatibility for existing API names.

package performance

type (
	Snapshot   = PerformanceSnapshot
	Timing     = PerformanceTiming
	Baseline   = PerformanceBaseline
	Regression = PerformanceRegression
)
