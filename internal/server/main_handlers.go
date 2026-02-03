// main_handlers.go — Server struct and core HTTP handlers.
// Contains the Server type (log entry management), entry manipulation methods,
// and the screenshot endpoint.
// Extracted from cmd/gasoline/main.go during Phase 4 refactoring.
package server

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// LogEntry is a type alias for the canonical definition in internal/types
type LogEntry = types.LogEntry

// Server holds the server state
type Server struct {
	logFile       string
	maxEntries    int
	entries       []LogEntry
	logAddedAt    []time.Time // parallel slice: when each entry was added
	mu            sync.RWMutex
	logTotalAdded int64 // monotonic counter of total entries ever added
	onEntries     func([]LogEntry) // optional callback when entries are added (e.g., for clustering)
	TTL                 time.Duration // TTL for read-time filtering (0 means unlimited)
	redactionConfigPath string        // path to redaction config JSON file (optional)
}

// NewServer creates a new server instance
func NewServer(logFile string, maxEntries int) (*Server, error) {
	s := &Server{
		logFile:    logFile,
		maxEntries: maxEntries,
		entries:    make([]LogEntry, 0),
	}

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

	// Bound entries (file may have more from append-only writes between rotations)
	if len(s.entries) > s.maxEntries {
		kept := make([]LogEntry, s.maxEntries)
		copy(kept, s.entries[len(s.entries)-s.maxEntries:])
		s.entries = kept
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

// sanitizeFilename removes characters unsafe for filenames
var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeForFilename(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

const maxPostBodySize = 10 * 1024 * 1024 // 10MB

// handleScreenshot saves a screenshot JPEG to disk and returns the filename
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		DataURL       string `json:"data_url"`
		URL           string `json:"url"`
		CorrelationID string `json:"correlation_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.DataURL == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing dataUrl"})
		return
	}

	// Extract base64 data from data URL
	parts := strings.SplitN(body.DataURL, ",", 2)
	if len(parts) != 2 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid dataUrl format"})
		return
	}

	imageData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid base64 data"})
		return
	}

	// Build filename: [website]-[timestamp]-[correlationId].jpg or [website]-[timestamp].jpg
	hostname := "unknown"
	if body.URL != "" {
		if u, err := url.Parse(body.URL); err == nil && u.Host != "" {
			hostname = u.Host
		}
	}

	timestamp := time.Now().Format("20060102-150405")

	var filename string
	if body.CorrelationID != "" {
		filename = fmt.Sprintf("%s-%s-%s.jpg",
			sanitizeForFilename(hostname),
			timestamp,
			sanitizeForFilename(body.CorrelationID))
	} else {
		filename = fmt.Sprintf("%s-%s.jpg",
			sanitizeForFilename(hostname),
			timestamp)
	}

	// Save to same directory as log file
	dir := filepath.Dir(s.logFile)
	savePath := filepath.Join(dir, filename)

	// #nosec G306 -- screenshots are intentionally world-readable
	if err := os.WriteFile(savePath, imageData, 0o644); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save screenshot"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"filename":       filename,
		"path":           savePath,
		"correlation_id": body.CorrelationID,
	})
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

// jsonResponse is a JSON response helper
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error encoding JSON response: %v\n", err)
	}
}
