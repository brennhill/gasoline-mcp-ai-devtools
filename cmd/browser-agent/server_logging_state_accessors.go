// Purpose: Exposes thread-safe accessors and mutators for in-memory logging state.
// Why: Keeps read/write state helpers separate from async queue and file I/O behavior.

package main

import "sync/atomic"

// getLogDropCount returns the total number of dropped log entries (thread-safe).
func (s *Server) getLogDropCount() int64 {
	return atomic.LoadInt64(&s.logDropCount)
}

// getEntryCount returns current entry count.
func (s *Server) getEntryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// getErrorTotalAdded returns the total number of error-level log entries ever added.
func (s *Server) getErrorTotalAdded() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errorTotalAdded
}

func (s *Server) getTelemetryMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.telemetryMode
}

func (s *Server) setTelemetryMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.telemetryMode = mode
}

// getEntries returns a copy of all entries.
func (s *Server) getEntries() []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]LogEntry, len(s.entries))
	copy(result, s.entries)
	return result
}
