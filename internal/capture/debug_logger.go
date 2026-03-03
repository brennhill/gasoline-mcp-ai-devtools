// Purpose: Implements bounded debug-log ring buffers for polling and HTTP diagnostic entries.
// Why: Provides lightweight operational diagnostics without unbounded memory growth.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"sync"
)

const debugLogSize = 50

// DebugLogger manages two circular buffers for operator debugging:
// polling activity (sync/settings calls) and HTTP request/response logging.
type DebugLogger struct {
	mu                sync.Mutex
	pollingLog        []PollingLogEntry
	pollingLogIndex   int
	httpDebugLog      []HTTPDebugEntry
	httpDebugLogIndex int
}

// NewDebugLogger creates a DebugLogger with pre-allocated circular buffers.
func NewDebugLogger() DebugLogger {
	return DebugLogger{
		pollingLog:   make([]PollingLogEntry, debugLogSize),
		httpDebugLog: make([]HTTPDebugEntry, debugLogSize),
	}
}

// LogPollingActivity adds an entry to the circular polling log buffer.
func (dl *DebugLogger) LogPollingActivity(entry PollingLogEntry) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.pollingLog[dl.pollingLogIndex] = entry
	dl.pollingLogIndex = (dl.pollingLogIndex + 1) % debugLogSize
}

// LogHTTPDebugEntry adds an entry to the circular HTTP debug log buffer.
func (dl *DebugLogger) LogHTTPDebugEntry(entry HTTPDebugEntry) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.httpDebugLog[dl.httpDebugLogIndex] = entry
	dl.httpDebugLogIndex = (dl.httpDebugLogIndex + 1) % debugLogSize
}

// GetPollingLog returns a copy of the polling activity log.
func (dl *DebugLogger) GetPollingLog() []PollingLogEntry {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	result := make([]PollingLogEntry, len(dl.pollingLog))
	copy(result, dl.pollingLog)
	return result
}

// GetHTTPDebugLog returns an independent copy of the HTTP debug log.
func (dl *DebugLogger) GetHTTPDebugLog() []HTTPDebugEntry {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	result := make([]HTTPDebugEntry, len(dl.httpDebugLog))
	copy(result, dl.httpDebugLog)
	return result
}
