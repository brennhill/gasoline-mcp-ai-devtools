// Extension internal logs endpoint
// Receives log entries from browser extension contexts (background, content scripts).
// Enables AI debugging of extension-internal behavior not visible through page-level capture.
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

// AddExtensionLogs ingests extension log entries into the ring buffer.
// Thread-safe: acquires write lock.
func (c *Capture) AddExtensionLogs(logs []ExtensionLog) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, log := range logs {
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		log = c.redactExtensionLog(log)
		c.elb.logs = append(c.elb.logs, log)

		// Amortized eviction: only compact when buffer exceeds 1.5x capacity.
		// Reduces allocation+copy from every sync to ~once every MaxExtensionLogs/2 syncs.
		evictionThreshold := MaxExtensionLogs + MaxExtensionLogs/2
		if len(c.elb.logs) > evictionThreshold {
			kept := make([]ExtensionLog, MaxExtensionLogs)
			copy(kept, c.elb.logs[len(c.elb.logs)-MaxExtensionLogs:])
			c.elb.logs = kept
		}
	}
}

// GetExtensionLogs returns all extension log entries.
// Thread-safe: acquires read lock.
func (c *Capture) GetExtensionLogs() []ExtensionLog {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]ExtensionLog, len(c.elb.logs))
	copy(result, c.elb.logs)
	return result
}
