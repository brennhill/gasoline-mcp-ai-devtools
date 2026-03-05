// Purpose: Shared observe helpers (limit clamping, constants).
// Why: Provides reusable bounds logic for observe mode handlers without coupling to dispatch.
// Docs: docs/features/feature/observe/index.md

package observe

// MaxObserveLimit caps the limit parameter to prevent oversized responses.
const MaxObserveLimit = 1000

// clampLimit applies default and max bounds to a limit parameter.
func clampLimit(limit, defaultVal int) int {
	if limit <= 0 {
		return defaultVal
	}
	if limit > MaxObserveLimit {
		return MaxObserveLimit
	}
	return limit
}
