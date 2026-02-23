// Purpose: Provides capture-level wrappers around DebugLogger for polling debug instrumentation.
// Why: Preserves existing capture API call sites while delegating storage to the dedicated debug logger.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

// logPollingActivity delegates to the DebugLogger sub-struct.
// Safe to call with or without c.mu held — DebugLogger has its own lock.
func (c *Capture) logPollingActivity(entry PollingLogEntry) {
	c.debug.LogPollingActivity(entry)
}
