// Purpose: ToolHandler lifecycle and shared accessor helpers.
// Why: Keeps boilerplate getters/lifecycle methods out of the core wiring file.

package main

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// Close cancels the shutdown context, unblocking any in-flight readiness gates.
func (h *ToolHandler) Close() {
	if h.shutdownCancel != nil {
		h.shutdownCancel()
	}
}

// GetCapture returns the capture instance.
func (h *ToolHandler) GetCapture() *capture.Capture {
	return h.capture
}

// GetLogEntries returns a snapshot of the server's log entries and their timestamps.
// The returned slices are copies — safe to use without holding the server lock.
func (h *ToolHandler) GetLogEntries() ([]LogEntry, []time.Time) {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	addedAt := make([]time.Time, len(h.server.logAddedAt))
	copy(addedAt, h.server.logAddedAt)
	return entries, addedAt
}

// GetLogTotalAdded returns the monotonic counter of total log entries ever added.
func (h *ToolHandler) GetLogTotalAdded() int64 {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()
	return h.server.logTotalAdded
}

// GetAnnotationStore returns the annotation store for draw mode data.
func (h *ToolHandler) GetAnnotationStore() *AnnotationStore {
	return h.annotationStore
}

// GetToolCallLimiter returns the tool call limiter.
func (h *ToolHandler) GetToolCallLimiter() RateLimiter {
	return h.toolCallLimiter
}

// GetRedactionEngine returns the redaction engine.
func (h *ToolHandler) GetRedactionEngine() RedactionEngine {
	return h.redactionEngine
}

// newPlaybackSessionsMap returns an initialized playback sessions map.
// Separated to avoid the parameter name "capture" shadowing the package import.
func newPlaybackSessionsMap() map[string]*capture.PlaybackSession {
	return make(map[string]*capture.PlaybackSession)
}
