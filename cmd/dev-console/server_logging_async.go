// Purpose: Async logging pipeline and in-memory entry bookkeeping.
// Why: Separates log ingestion/rotation mechanics from server construction and API surface.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// addEntries adds new entries and rotates if needed.
// #lizard forgives
func (s *Server) addEntries(newEntries []LogEntry) int {
	rotated, entriesToSave, appendOnly, cb := s.addEntriesInMemory(newEntries)

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

// addEntriesInMemory mutates log state under lock and returns snapshots for I/O outside the lock.
func (s *Server) addEntriesInMemory(newEntries []LogEntry) (rotated bool, entriesToSave []LogEntry, appendOnly []LogEntry, cb func([]LogEntry)) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	rotated = len(s.entries) > s.maxEntries
	if rotated {
		kept := make([]LogEntry, s.maxEntries)
		copy(kept, s.entries[len(s.entries)-s.maxEntries:])
		s.entries = kept
		keptAt := make([]time.Time, s.maxEntries)
		copy(keptAt, s.logAddedAt[len(s.logAddedAt)-s.maxEntries:])
		s.logAddedAt = keptAt
	}

	// Snapshot data for file I/O outside the lock
	if rotated {
		entriesToSave = make([]LogEntry, len(s.entries))
		copy(entriesToSave, s.entries)
	} else {
		appendOnly = make([]LogEntry, len(newEntries))
		copy(appendOnly, newEntries)
	}
	cb = s.onEntries
	return rotated, entriesToSave, appendOnly, cb
}

// asyncLoggerWorker runs in a background goroutine and handles all file I/O.
func (s *Server) asyncLoggerWorker() {
	defer close(s.logDone)

	for entries := range s.logChan {
		// Synchronous file I/O happens here (off the hot path)
		if err := s.appendToFileSync(entries); err != nil {
			s.AddWarning(fmt.Sprintf("log_write_failed: %v", err))
		}
	}
}

// appendToFileSync does synchronous file I/O (called by async worker only).
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

// appendToFile queues log entries for async writing (never blocks).
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
			stderrf("[gasoline] WARNING: Log buffer full, %d logs dropped\n", dropped)
		}

		return fmt.Errorf("log buffer full (%d total drops)", dropped)
	}
}

// clearEntries removes all entries.
func (s *Server) clearEntries() {
	s.clearEntriesInMemory()
	// Write empty file outside lock
	// #nosec G306 -- log files are owner-only (0600) for privacy
	if s.logFile != "" {
		if err := os.WriteFile(s.logFile, []byte{}, 0600); err != nil {
			s.AddWarning(fmt.Sprintf("log_clear_failed: %v", err))
		}
	}
}

func (s *Server) clearEntriesInMemory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	s.logAddedAt = nil
}

// shutdownAsyncLogger gracefully shuts down the async logger, draining remaining logs.
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
