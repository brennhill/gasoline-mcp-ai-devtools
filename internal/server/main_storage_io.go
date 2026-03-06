// Purpose: Implements persistent log storage read/write routines.
// Why: Isolates file I/O concerns from in-memory entry mutation and response helpers.
// Docs: docs/features/feature/backend-log-streaming/index.md

package server

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
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
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip malformed lines
		}
		s.entries = append(s.entries, entry)
	}

	// Bound entries (file may have more from append-only writes between rotations).
	if len(s.entries) > s.maxEntries {
		kept := make([]LogEntry, s.maxEntries)
		copy(kept, s.entries[len(s.entries)-s.maxEntries:])
		s.entries = kept
	}

	return scanner.Err()
}

// saveEntries writes all entries to file (caller must hold s.mu).
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
	defer file.Close() //nolint:errcheck // deferred close

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

// appendToFile writes only the new entries to the file (append-only, no rewrite).
func (s *Server) appendToFile(entries []LogEntry) error {
	f, err := os.OpenFile(s.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) // #nosec G302 G304 -- log files are intentionally world-readable; path set at startup
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // deferred close

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
