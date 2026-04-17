// Purpose: Exposes thread-safe accessors and mutators for in-memory logging state.
// Why: Keeps read/write state helpers separate from async queue and file I/O behavior.

package main

import "sync/atomic"

// getLogDropCount returns the total number of dropped log entries (thread-safe).
func (ls *LogStore) getLogDropCount() int64 {
	return atomic.LoadInt64(&ls.logDropCount)
}

// getEntryCount returns current entry count.
func (ls *LogStore) getEntryCount() int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return len(ls.entries)
}

// getErrorTotalAdded returns the total number of error-level log entries ever added.
func (ls *LogStore) getErrorTotalAdded() int64 {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.errorTotalAdded
}

func (ls *LogStore) getTelemetryMode() string {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.telemetryMode
}

func (ls *LogStore) setTelemetryMode(mode string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.telemetryMode = mode
}

// getEntries returns a copy of all entries.
func (ls *LogStore) getEntries() []LogEntry {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	result := make([]LogEntry, len(ls.entries))
	copy(result, ls.entries)
	return result
}
