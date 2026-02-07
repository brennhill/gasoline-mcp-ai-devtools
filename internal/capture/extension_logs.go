// Extension internal logs endpoint
// Receives log entries from browser extension contexts (background, content scripts).
// Enables AI debugging of extension-internal behavior not visible through page-level capture.
package capture

import (
	"encoding/json"
	"net/http"
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

// HandleExtensionLogs processes log entries from extension contexts
func (c *Capture) HandleExtensionLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Logs []ExtensionLog `json:"logs"`
	}

	// Parse JSON payload
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Process each log entry
	for _, log := range payload.Logs {
		// Set server-side timestamp if not provided
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}

		// Append to ring buffer with capacity enforcement
		c.elb.logs = append(c.elb.logs, log)

		// Evict oldest entries if over capacity.
		// Allocate new slice to release old backing array for GC.
		if len(c.elb.logs) > MaxExtensionLogs {
			kept := make([]ExtensionLog, MaxExtensionLogs)
			copy(kept, c.elb.logs[len(c.elb.logs)-MaxExtensionLogs:])
			c.elb.logs = kept
		}
	}

	w.WriteHeader(http.StatusOK)
	//nolint:errcheck -- HTTP response encoding errors are logged by client; no recovery possible
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"logs_stored": len(payload.Logs),
	})
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
