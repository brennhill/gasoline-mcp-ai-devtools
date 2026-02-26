// Purpose: Defines test hook overrides for extension disconnect thresholds and poll intervals.
// Why: Allows deterministic testing of timeout/disconnect behavior without changing production constants.
// Docs: docs/features/feature/self-testing/index.md

package capture

import "time"

// SetExtensionDisconnectThresholdForTesting overrides the disconnect threshold and
// returns a restore function for test cleanup.
// NOTE: Tests that mutate this var must NOT use t.Parallel() since it is a
// package-level variable shared across all tests in the package.
func SetExtensionDisconnectThresholdForTesting(d time.Duration) func() {
	prev := extensionDisconnectThreshold
	extensionDisconnectThreshold = d
	return func() {
		extensionDisconnectThreshold = prev
	}
}
