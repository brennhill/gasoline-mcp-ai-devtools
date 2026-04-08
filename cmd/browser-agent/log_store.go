// log_store.go — Focused log entry storage with TTL rotation and async channel pipeline.
// Why: Extracts log state from the Server god object into a single-purpose subsystem.

package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// LogEntry represents a single log entry (alias to internal/mcp).
type LogEntry = mcp.LogEntry

// defaultMaxFileSize is the log file size threshold for rotation (50MB).
const defaultMaxFileSize int64 = 50 * 1024 * 1024

// LogStore holds log entry state, TTL rotation, and async file I/O pipeline.
type LogStore struct {
	logFile     string
	maxEntries  int
	maxFileSize int64 // max log file size in bytes before rotation (0 = disabled)

	entries       []LogEntry
	logAddedAt    []time.Time // parallel slice: when each entry was added
	mu            sync.RWMutex
	logTotalAdded   int64            // monotonic counter of total entries ever added
	errorTotalAdded int64            // monotonic counter of error-level entries ever added
	telemetryMode   string           // telemetry summary verbosity: off|auto|full
	onEntries       func([]LogEntry) // optional callback when entries are added (e.g., for clustering)
	TTL             time.Duration    // TTL for read-time filtering (0 means unlimited)

	// Async logging
	logChan       chan []LogEntry // buffered channel for async log writes
	logDropCount  int64           // atomic counter for dropped logs (when channel full)
	logDone       chan struct{}   // signal when async logger exits
	logChanClosed atomic.Bool    // guards against double-close panic on logChan

	// addWarning is a callback to report warnings to the server.
	// Set during construction to avoid circular dependency.
	addWarning func(string)
}

// NewLogStore creates a new LogStore.
// The addWarning callback is used to surface diagnostics to the parent Server.
func NewLogStore(logFile string, maxEntries int, addWarning func(string)) *LogStore {
	return &LogStore{
		logFile:       logFile,
		maxEntries:    maxEntries,
		maxFileSize:   defaultMaxFileSize,
		entries:       make([]LogEntry, 0),
		telemetryMode: telemetryModeAuto,
		logChan:       make(chan []LogEntry, 10000), // 10k buffer for burst traffic
		logDone:       make(chan struct{}),
		addWarning:    addWarning,
	}
}

// Snapshot returns a thread-safe copy of all log entries.
func (ls *LogStore) Snapshot() []LogEntry {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	entries := make([]LogEntry, len(ls.entries))
	copy(entries, ls.entries)
	return entries
}

// SnapshotWithTimestamps returns a thread-safe copy of all log entries and their add-times.
func (ls *LogStore) SnapshotWithTimestamps() ([]LogEntry, []time.Time) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	entries := make([]LogEntry, len(ls.entries))
	copy(entries, ls.entries)
	addedAt := make([]time.Time, len(ls.logAddedAt))
	copy(addedAt, ls.logAddedAt)
	return entries, addedAt
}

// Len returns the current number of log entries (thread-safe).
func (ls *LogStore) Len() int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return len(ls.entries)
}

// SetOnEntries sets the callback invoked when new log entries are added.
// Thread-safe: acquires the write lock to avoid racing with addEntries.
func (ls *LogStore) SetOnEntries(cb func([]LogEntry)) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.onEntries = cb
}
