// Purpose: Log-file persistence helpers for loading, saving, and writability checks.
// Why: Isolates disk persistence logic from in-memory server state orchestration.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// loadEntries reads existing log entries from file.
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

// saveEntries writes all entries to file (caller must hold s.mu).
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

func fallbackLogFilePath() string {
	return filepath.Join(os.TempDir(), "gasoline", "logs", "gasoline.jsonl")
}

func ensureLogFileWritable(path string) error {
	if path == "" {
		return fmt.Errorf("log_init: log file path is empty. Set a valid path via --log-file or GASOLINE_LOG_FILE")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- local path configured at startup
	if err != nil {
		return err
	}
	return f.Close()
}
