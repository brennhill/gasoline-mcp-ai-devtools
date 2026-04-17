// bridge_fastpath_telemetry_test.go — Test helpers for resetting fast-path telemetry counters.
// Why: Global counters persist across tests; these helpers isolate test state.

package bridge

// resetFastPathResourceReadCounters zeroes the resource-read counters between tests.
func resetFastPathResourceReadCounters() {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	fastPathResourceReadCounters.success = 0
	fastPathResourceReadCounters.failure = 0
}

// resetFastPathCounters zeroes the general fast-path event counters between tests.
func resetFastPathCounters() {
	fastPathCounters.mu.Lock()
	defer fastPathCounters.mu.Unlock()
	fastPathCounters.success = 0
	fastPathCounters.failure = 0
}
