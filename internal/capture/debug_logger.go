// debug_logger.go — Circular buffer debug logging for polling and HTTP activity.
// Extracted from the Capture god object to reduce field count and mutex scope.
// Thread-safe: uses its own sync.Mutex independent of Capture.mu.
package capture

import (
	"fmt"
	"os"
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
	dl.pollingLog[dl.pollingLogIndex] = entry
	dl.pollingLogIndex = (dl.pollingLogIndex + 1) % debugLogSize
	dl.mu.Unlock()
}

// LogHTTPDebugEntry adds an entry to the circular HTTP debug log buffer.
func (dl *DebugLogger) LogHTTPDebugEntry(entry HTTPDebugEntry) {
	dl.mu.Lock()
	dl.httpDebugLog[dl.httpDebugLogIndex] = entry
	dl.httpDebugLogIndex = (dl.httpDebugLogIndex + 1) % debugLogSize
	dl.mu.Unlock()
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

// PrintHTTPDebug prints an HTTP debug entry to stderr.
// Must be called WITHOUT holding any lock.
// Quiet mode: only errors (non-2xx status codes) are printed.
func PrintHTTPDebug(entry HTTPDebugEntry) {
	if entry.ResponseStatus >= 400 {
		fmt.Fprintf(os.Stderr, "[gasoline] HTTP %s %s | status=%d\n",
			entry.Method, entry.Endpoint, entry.ResponseStatus)
		if entry.Error != "" {
			fmt.Fprintf(os.Stderr, "[gasoline]   Error: %s\n", entry.Error)
		}
	}
}
