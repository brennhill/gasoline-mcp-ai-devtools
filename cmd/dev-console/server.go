// server.go — Server struct and core data management methods.
// Handles log entry storage, rotation, and file persistence.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// LogEntry represents a single log entry
type LogEntry map[string]any

// Server holds the server state
type Server struct {
	logFile             string
	maxEntries          int
	entries             []LogEntry
	logAddedAt          []time.Time // parallel slice: when each entry was added
	mu                  sync.RWMutex
	logTotalAdded       int64            // monotonic counter of total entries ever added
	onEntries           func([]LogEntry) // optional callback when entries are added (e.g., for clustering)
	TTL                 time.Duration    // TTL for read-time filtering (0 means unlimited)
	redactionConfigPath string           // path to redaction config JSON file (optional)

	// Async logging
	logChan      chan []LogEntry // buffered channel for async log writes
	logDropCount int64           // atomic counter for dropped logs (when channel full)
	logDone      chan struct{}   // signal when async logger exits
}

// NewServer creates a new server instance
func NewServer(logFile string, maxEntries int) (*Server, error) {
	s := &Server{
		logFile:    logFile,
		maxEntries: maxEntries,
		entries:    make([]LogEntry, 0),
		logChan:    make(chan []LogEntry, 10000), // 10k buffer for burst traffic
		logDone:    make(chan struct{}),
	}

	// Start async logger goroutine
	go s.asyncLoggerWorker()

	// Ensure log directory exists
	dir := filepath.Dir(logFile)
	// #nosec G301 -- 0o755 is appropriate for log directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Load existing entries
	if err := s.loadEntries(); err != nil {
		// File might not exist yet, that's OK
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load existing entries: %w", err)
		}
	}

	return s, nil
}

// SetOnEntries sets the callback invoked when new log entries are added.
// Thread-safe: acquires the write lock to avoid racing with addEntries.
func (s *Server) SetOnEntries(cb func([]LogEntry)) {
	s.mu.Lock()
	s.onEntries = cb
	s.mu.Unlock()
}

// loadEntries reads existing log entries from file
func (s *Server) loadEntries() error {
	file, err := os.Open(s.logFile)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // deferred close

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // Allow up to 10MB per line (screenshots can be large)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip malformed lines
		}
		s.entries = append(s.entries, entry)
	}

	// Initialize logAddedAt for loaded entries (we don't have actual add times,
	// but the slice must have the same length as entries for rotation to work)
	s.logAddedAt = make([]time.Time, len(s.entries))

	// Bound entries (file may have more from append-only writes between rotations)
	if len(s.entries) > s.maxEntries {
		kept := make([]LogEntry, s.maxEntries)
		copy(kept, s.entries[len(s.entries)-s.maxEntries:])
		s.entries = kept
		s.logAddedAt = make([]time.Time, s.maxEntries)
	}

	return scanner.Err()
}

// saveEntries writes all entries to file (caller must hold s.mu)
func (s *Server) saveEntries() error {
	return s.saveEntriesCopy(s.entries)
}

// saveEntriesCopy writes the given entries to file without acquiring the lock.
// The caller is responsible for providing a snapshot of the entries.
func (s *Server) saveEntriesCopy(entries []LogEntry) error {
	file, err := os.Create(s.logFile)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Error closing log file: %v\n", closeErr)
		}
	}()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if _, err := file.Write(data); err != nil {
			return err
		}
		if _, err := file.WriteString("\n"); err != nil {
			return err
		}
	}

	return nil
}

// addEntries adds new entries and rotates if needed
func (s *Server) addEntries(newEntries []LogEntry) int {
	s.mu.Lock()

	s.logTotalAdded += int64(len(newEntries))
	now := time.Now()
	for range newEntries {
		s.logAddedAt = append(s.logAddedAt, now)
	}
	s.entries = append(s.entries, newEntries...)

	// Rotate if needed — copy to new slice to allow GC of evicted entries
	rotated := len(s.entries) > s.maxEntries
	if rotated {
		kept := make([]LogEntry, s.maxEntries)
		copy(kept, s.entries[len(s.entries)-s.maxEntries:])
		s.entries = kept
		keptAt := make([]time.Time, s.maxEntries)
		copy(keptAt, s.logAddedAt[len(s.logAddedAt)-s.maxEntries:])
		s.logAddedAt = keptAt
	}

	// Snapshot data for file I/O outside the lock
	var entriesToSave []LogEntry
	var appendOnly []LogEntry
	if rotated {
		entriesToSave = make([]LogEntry, len(s.entries))
		copy(entriesToSave, s.entries)
	} else {
		appendOnly = make([]LogEntry, len(newEntries))
		copy(appendOnly, newEntries)
	}
	cb := s.onEntries
	s.mu.Unlock()

	// File I/O outside lock
	if rotated {
		if err := s.saveEntriesCopy(entriesToSave); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Error saving entries: %v\n", err)
		}
	} else {
		if err := s.appendToFile(appendOnly); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Error saving entries: %v\n", err)
		}
	}

	// Notify listeners outside the lock (e.g., cluster manager)
	if cb != nil {
		cb(newEntries)
	}

	return len(newEntries)
}

// appendToFile writes only the new entries to the file (append-only, no rewrite)
// asyncLoggerWorker runs in a background goroutine and handles all file I/O
func (s *Server) asyncLoggerWorker() {
	defer close(s.logDone)

	for entries := range s.logChan {
		// Synchronous file I/O happens here (off the hot path)
		if err := s.appendToFileSync(entries); err != nil {
			// Log to stderr but don't crash
			fmt.Fprintf(os.Stderr, "[gasoline] Async logger error: %v\n", err)
		}
	}
}

// appendToFileSync does synchronous file I/O (called by async worker only)
func (s *Server) appendToFileSync(entries []LogEntry) error {
	f, err := os.OpenFile(s.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- path set at startup
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Error closing log file: %v\n", closeErr)
		}
	}()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	return nil
}

// appendToFile queues log entries for async writing (never blocks)
func (s *Server) appendToFile(entries []LogEntry) error {
	select {
	case s.logChan <- entries:
		// Queued successfully
		return nil
	default:
		// Channel full - drop log to maintain availability
		dropped := atomic.AddInt64(&s.logDropCount, 1)

		// Alert to stderr (but don't spam)
		if dropped%1000 == 1 { // Alert on 1st, 1001st, 2001st, etc.
			fmt.Fprintf(os.Stderr, "[gasoline] WARNING: Log buffer full, %d logs dropped\n", dropped)
		}

		return fmt.Errorf("log buffer full (%d total drops)", dropped)
	}
}

// clearEntries removes all entries
func (s *Server) clearEntries() {
	s.mu.Lock()
	s.entries = nil
	s.logAddedAt = nil
	s.mu.Unlock()
	// Write empty file outside lock
	// #nosec G306 -- log files are owner-only (0600) for privacy
	if s.logFile != "" {
		if err := os.WriteFile(s.logFile, []byte{}, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Error clearing log file: %v\n", err)
		}
	}
}

// shutdownAsyncLogger gracefully shuts down the async logger, draining remaining logs
func (s *Server) shutdownAsyncLogger(timeout time.Duration) {
	// Close channel to signal worker to exit after draining
	close(s.logChan)

	// Wait for worker to finish draining, with timeout
	select {
	case <-s.logDone:
		// Worker exited cleanly
		dropped := atomic.LoadInt64(&s.logDropCount)
		if dropped > 0 {
			fmt.Fprintf(os.Stderr, "[gasoline] Async logger shutdown: %d logs were dropped during session\n", dropped)
		}
	case <-time.After(timeout):
		// Timeout - worker still draining, but we need to exit
		fmt.Fprintf(os.Stderr, "[gasoline] Async logger shutdown timeout, %d logs may be lost\n", len(s.logChan))
	}
}

// getEntryCount returns current entry count
func (s *Server) getEntryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// getEntries returns a copy of all entries
func (s *Server) getEntries() []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]LogEntry, len(s.entries))
	copy(result, s.entries)
	return result
}

// validLogLevels defines accepted log level values.
var validLogLevels = map[string]bool{
	"error": true,
	"warn":  true,
	"info":  true,
	"debug": true,
	"log":   true,
}

// maxEntrySize is the maximum serialized size of a single log entry (1MB).
const maxEntrySize = 1024 * 1024

// validateLogEntry checks if a log entry meets the contract requirements.
// Returns true if the entry is valid, false otherwise.
func validateLogEntry(entry LogEntry) bool {
	// Required: level field must exist and be a known value
	level, ok := entry["level"].(string)
	if !ok || !validLogLevels[level] {
		return false
	}

	// Fast path: if total string content is under half the limit,
	// the entry can't exceed maxEntrySize even with JSON escaping overhead
	var stringBytes int
	for _, v := range entry {
		if s, ok := v.(string); ok {
			stringBytes += len(s)
		}
	}
	if stringBytes < maxEntrySize/2 {
		return true
	}

	// Slow path: might be large — check precisely via marshal
	data, err := json.Marshal(entry)
	if err != nil {
		return false
	}
	return len(data) <= maxEntrySize
}

// validateLogEntries filters entries, returning only valid ones and a count of rejected.
func validateLogEntries(entries []LogEntry) (valid []LogEntry, rejected int) {
	valid = make([]LogEntry, 0, len(entries))
	for _, entry := range entries {
		if validateLogEntry(entry) {
			valid = append(valid, entry)
		} else {
			rejected++
		}
	}
	return valid, rejected
}
