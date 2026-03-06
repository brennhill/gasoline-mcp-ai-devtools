// Purpose: Implements ingestion and retrieval of extension-internal debug logs captured via sync channels.
// Why: Enables debugging of extension runtime behavior that page-level console capture cannot observe.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"
)

// ============================================
// Extension Logs Handler
// ============================================
// Receives log entries from the browser extension's background script,
// content script, and other extension contexts.
//
// This enables AI debugging of extension-internal behavior that isn't
// visible through page-level console capture.

// AddExtensionLogs ingests extension runtime logs into bounded in-memory buffer.
//
// Invariants:
// - Logs are redacted before storage.
// - Buffer compaction keeps the newest MaxExtensionLogs entries.
//
// Failure semantics:
// - Missing timestamps are filled with server receive time.
func (c *Capture) AddExtensionLogs(logs []ExtensionLog) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, log := range logs {
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		log = c.redactExtensionLog(log)
		c.extensionLogs.append(log)
	}
}

// GetExtensionLogs returns a detached copy of extension logs.
//
// Failure semantics:
// - Returns empty slice when buffer is empty.
func (c *Capture) GetExtensionLogs() []ExtensionLog {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionLogs.snapshot()
}
