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
