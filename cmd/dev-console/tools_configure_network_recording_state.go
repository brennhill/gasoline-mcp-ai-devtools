// Purpose: In-memory state container for network recording lifecycle.
// Why: Keeps concurrency-safe start/stop/status state transitions separate from tool handler orchestration.

package main

import (
	"sync"
	"time"
)

// recordingSnapshot holds the captured state from a recording session.
type recordingSnapshot struct {
	Active    bool
	StartTime time.Time
	Domain    string
	Method    string
}

// networkRecordingState tracks an active network recording session.
type networkRecordingState struct {
	mu        sync.Mutex
	active    bool
	startTime time.Time
	domain    string // optional domain filter
	method    string // optional HTTP method filter
}

// tryStart atomically checks if recording is inactive and starts it.
// Returns (startTime, true) on success, or (zero, false) if already active.
func (s *networkRecordingState) tryStart(domain, method string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return time.Time{}, false
	}
	s.active = true
	s.startTime = time.Now()
	s.domain = domain
	s.method = method
	return s.startTime, true
}

// stop atomically stops recording and returns a snapshot of the session state.
// Returns (snapshot, true) if recording was active, or (zero, false) if not.
func (s *networkRecordingState) stop() (recordingSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return recordingSnapshot{}, false
	}
	snap := recordingSnapshot{
		Active:    true,
		StartTime: s.startTime,
		Domain:    s.domain,
		Method:    s.method,
	}
	s.active = false
	s.startTime = time.Time{}
	s.domain = ""
	s.method = ""
	return snap, true
}

// info returns a snapshot of the current recording state.
func (s *networkRecordingState) info() recordingSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return recordingSnapshot{
		Active:    s.active,
		StartTime: s.startTime,
		Domain:    s.domain,
		Method:    s.method,
	}
}
