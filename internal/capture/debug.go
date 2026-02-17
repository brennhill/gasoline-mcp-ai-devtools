// Purpose: Owns debug.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// debug.go — Capture delegation methods for debug logging.
// Delegates to DebugLogger sub-struct. These methods exist for backward
// compatibility with callers that operate under c.mu and call c.logPollingActivity().
package capture

// logPollingActivity delegates to the DebugLogger sub-struct.
// Safe to call with or without c.mu held — DebugLogger has its own lock.
func (c *Capture) logPollingActivity(entry PollingLogEntry) {
	c.debug.LogPollingActivity(entry)
}

