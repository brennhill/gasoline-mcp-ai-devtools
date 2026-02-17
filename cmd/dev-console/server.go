// Purpose: Owns server.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

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

	"github.com/dev-console/dev-console/internal/util"
)

// LogEntry represents a single log entry
type LogEntry map[string]any

// defaultMaxFileSize is the log file size threshold for rotation (50MB).
const defaultMaxFileSize int64 = 50 * 1024 * 1024

// Server holds the server state
type Server struct {
	logFile         string
	maxEntries      int
	maxFileSize     int64 // max log file size in bytes before rotation (0 = disabled)
	entries         []LogEntry
	logAddedAt      []time.Time // parallel slice: when each entry was added
	mu              sync.RWMutex
	logTotalAdded   int64            // monotonic counter of total entries ever added
	errorTotalAdded int64            // monotonic counter of error-level entries ever added
	telemetryMode   string           // telemetry summary verbosity: off|auto|full
	onEntries       func([]LogEntry) // optional callback when entries are added (e.g., for clustering)
	TTL             time.Duration    // TTL for read-time filtering (0 means unlimited)

	// Async logging
	logChan      chan []LogEntry // buffered channel for async log writes
	logDropCount int64           // atomic counter for dropped logs (when channel full)
	logDone      chan struct{}   // signal when async logger exits

	// One-shot warnings surfaced via MCP tool responses.
	warningsMu  sync.Mutex
	warnings    []string
	warningSeen map[string]struct{}
}

// NewServer creates a new server instance
func NewServer(logFile string, maxEntries int) (*Server, error) {
	s := &Server{
		logFile:       logFile,
		maxEntries:    maxEntries,
		maxFileSize:   defaultMaxFileSize,
		entries:       make([]LogEntry, 0),
		telemetryMode: telemetryModeAuto,
		logChan:       make(chan []LogEntry, 10000), // 10k buffer for burst traffic
		logDone:       make(chan struct{}),
		warningSeen:   make(map[string]struct{}),
	}

	// Start async logger goroutine
	util.SafeGo(func() { s.asyncLoggerWorker() })

	// Ensure log directory exists
	if s.logFile != "" {
		dir := filepath.Dir(s.logFile)
		// #nosec G301 -- log directory: owner rwx, group rx for diagnostics
		if err := os.MkdirAll(dir, 0o750); err != nil {
			fallback := fallbackLogFilePath()
			s.AddWarning(fmt.Sprintf("state_dir_not_writable: %v; falling back to %s", err, fallback))
			s.logFile = fallback
			_ = os.MkdirAll(filepath.Dir(s.logFile), 0o750)
		}
		if err := ensureLogFileWritable(s.logFile); err != nil {
			fallback := fallbackLogFilePath()
			s.AddWarning(fmt.Sprintf("state_dir_not_writable: %v; falling back to %s", err, fallback))
			s.logFile = fallback
			if err := os.MkdirAll(filepath.Dir(s.logFile), 0o750); err != nil {
				s.AddWarning(fmt.Sprintf("log_persistence_disabled: %v", err))
				s.logFile = ""
			} else if err := ensureLogFileWritable(s.logFile); err != nil {
				s.AddWarning(fmt.Sprintf("log_persistence_disabled: %v", err))
				s.logFile = ""
			}
		}
	}

	// Load existing entries
	if s.logFile != "" {
		if err := s.loadEntries(); err != nil {
			// File might not exist yet, that's OK
			if !os.IsNotExist(err) {
				s.AddWarning(fmt.Sprintf("log_load_failed: %v", err))
			}
		}
	}

	return s, nil
}

// Close gracefully shuts down the server, draining the async log writer.
func (s *Server) Close() {
	s.shutdownAsyncLogger(2 * time.Second)
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
// Uses atomic write pattern: write to temp file then rename for safety.
func (s *Server) saveEntriesCopy(entries []LogEntry) error {
	if s.logFile == "" {
		return nil
	}
	// Write to temporary file first, then atomically rename
	// This ensures log file remains intact if write fails partway through
	tmpFile := s.logFile + ".tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			s.AddWarning(fmt.Sprintf("log_temp_close_failed: %v", closeErr))
		}
	}()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if _, err := file.Write(data); err != nil {
			// Clean up temp file on write failure
			_ = os.Remove(tmpFile)
			return err
		}
		if _, err := file.WriteString("\n"); err != nil {
			// Clean up temp file on write failure
			_ = os.Remove(tmpFile)
			return err
		}
	}

	// Atomic rename: ensures log file is only updated if write succeeded
	if err := os.Rename(tmpFile, s.logFile); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}

	return nil
}

// addEntries adds new entries and rotates if needed
// #lizard forgives
func (s *Server) addEntries(newEntries []LogEntry) int {
	s.mu.Lock()

	s.logTotalAdded += int64(len(newEntries))
	for _, entry := range newEntries {
		level, ok := entry["level"].(string)
		if ok && level == "error" {
			s.errorTotalAdded++
		}
	}
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

	// File I/O outside lock — snapshot protects consistency
	// Note: If clearEntries() is called between unlock and file I/O, the file may temporarily contain
	// stale entries that were cleared from memory. This is acceptable because:
	// 1. In-memory entries are always consistent (cleared immediately)
	// 2. On rotation, the entire file is rewritten with fresh data
	// 3. The window is very short (microseconds typically)
	if rotated {
		if err := s.saveEntriesCopy(entriesToSave); err != nil {
			s.AddWarning(fmt.Sprintf("log_save_failed: %v", err))
		}
	} else {
		if err := s.appendToFile(appendOnly); err != nil {
			s.AddWarning(fmt.Sprintf("log_append_failed: %v", err))
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
			s.AddWarning(fmt.Sprintf("log_write_failed: %v", err))
		}
	}
}

// appendToFileSync does synchronous file I/O (called by async worker only)
func (s *Server) appendToFileSync(entries []LogEntry) error {
	if s.logFile == "" {
		return nil
	}
	f, err := os.OpenFile(s.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- path set at startup
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			s.AddWarning(fmt.Sprintf("log_close_failed: %v", closeErr))
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

	// Check file size and rotate if needed (non-blocking, off the hot path)
	s.maybeRotateLogFile(f)

	return nil
}

// maybeRotateLogFile checks the log file size and rotates if it exceeds maxFileSize.
// Rotation renames the current file to .jsonl.old and lets the next write create a fresh file.
// Called only from the async logger worker, so no additional locking is needed for file I/O.
func (s *Server) maybeRotateLogFile(f *os.File) {
	if s.maxFileSize <= 0 {
		return
	}

	fi, err := f.Stat()
	if err != nil {
		return
	}
	if fi.Size() <= s.maxFileSize {
		return
	}

	oldFile := s.logFile + ".old"
	// Rename overwrites any existing .old file atomically on POSIX systems
	if err := os.Rename(s.logFile, oldFile); err != nil { // #nosec G703 -- s.logFile is configured by local operator, not remote input
		s.AddWarning(fmt.Sprintf("log_rotate_failed: %v", err))
		return
	}
	s.AddWarning(fmt.Sprintf("log_rotated: %s -> %s (%d bytes)", s.logFile, oldFile, fi.Size()))
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
			s.AddWarning(fmt.Sprintf("log_clear_failed: %v", err))
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
			s.AddWarning(fmt.Sprintf("log_drops: %d logs were dropped during session", dropped))
		}
	case <-time.After(timeout):
		// Timeout - worker still draining, but we need to exit
		s.AddWarning(fmt.Sprintf("log_shutdown_timeout: %d logs may be lost", len(s.logChan)))
	}
}

// AddWarning stores a one-shot warning to be surfaced in the next tool response.
func (s *Server) AddWarning(msg string) {
	if msg == "" {
		return
	}
	s.warningsMu.Lock()
	defer s.warningsMu.Unlock()
	if s.warningSeen == nil {
		s.warningSeen = make(map[string]struct{})
	}
	if _, ok := s.warningSeen[msg]; ok {
		return
	}
	s.warningSeen[msg] = struct{}{}
	s.warnings = append(s.warnings, msg)
}

// TakeWarnings returns pending warnings and clears the pending list.
func (s *Server) TakeWarnings() []string {
	s.warningsMu.Lock()
	defer s.warningsMu.Unlock()
	if len(s.warnings) == 0 {
		return nil
	}
	out := make([]string, len(s.warnings))
	copy(out, s.warnings)
	s.warnings = nil
	return out
}

func fallbackLogFilePath() string {
	return filepath.Join(os.TempDir(), "gasoline", "logs", "gasoline.jsonl")
}

func ensureLogFileWritable(path string) error {
	if path == "" {
		return fmt.Errorf("empty log file path")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- local path configured at startup
	if err != nil {
		return err
	}
	return f.Close()
}

// getLogDropCount returns the total number of dropped log entries (thread-safe).
func (s *Server) getLogDropCount() int64 {
	return atomic.LoadInt64(&s.logDropCount)
}

// getEntryCount returns current entry count
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
