// Purpose: Defines test hook overrides for extension disconnect thresholds and poll intervals.
// Why: Allows deterministic testing of timeout/disconnect behavior without changing production constants.
// Docs: docs/features/feature/self-testing/index.md

package capture

import "time"

// SetExtensionDisconnectThresholdForTesting overrides the disconnect threshold and
// returns a restore function for test cleanup.
func SetExtensionDisconnectThresholdForTesting(d time.Duration) func() {
	prev := extensionDisconnectThreshold
	extensionDisconnectThreshold = d
	return func() {
		extensionDisconnectThreshold = prev
	}
}

// SetReadinessGatePollIntervalForTesting overrides the readiness gate poll interval
// and returns a restore function for test cleanup.
func SetReadinessGatePollIntervalForTesting(d time.Duration) func() {
	prev := readinessGatePollInterval
	readinessGatePollInterval = d
	return func() {
		readinessGatePollInterval = prev
	}
}
