// Purpose: Defines the Server struct and startup wiring for log, push, and annotation subsystems.
// Why: Centralizes top-level server state while detailed persistence and logging mechanics live in focused modules.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pty"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/push"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// LogEntry represents a single log entry (alias to internal/mcp).
type LogEntry = mcp.LogEntry

// defaultMaxFileSize is the log file size threshold for rotation (50MB).
const defaultMaxFileSize int64 = 50 * 1024 * 1024

// Server holds the server state.
type Server struct {
	logFile         string
	maxEntries      int
	maxFileSize     int64 // max log file size in bytes before rotation (0 = disabled)
	listenPort      int
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

	// Annotation store is server-scoped to avoid cross-session contamination.
	annotationStore *AnnotationStore

	// Push delivery pipeline
	pushInbox  *push.PushInbox
	pushRouter *push.Router

	// Terminal PTY session manager
	ptyManager *pty.Manager

	// Terminal server port (0 = terminal server not running)
	terminalPort int

	// Active codebase path — set via MCP configure(what='store', key='active_codebase')
	// or via the extension options page. Used as default CWD for terminal sessions.
	activeCodebaseMu sync.RWMutex
	activeCodebase   string
}

// NewServer creates a new server instance.
func NewServer(logFile string, maxEntries int) (*Server, error) {
	s := &Server{
		logFile:         logFile,
		maxEntries:      maxEntries,
		maxFileSize:     defaultMaxFileSize,
		listenPort:      defaultPort,
		entries:         make([]LogEntry, 0),
		telemetryMode:   telemetryModeAuto,
		logChan:         make(chan []LogEntry, 10000), // 10k buffer for burst traffic
		logDone:         make(chan struct{}),
		warningSeen:     make(map[string]struct{}),
		annotationStore: NewAnnotationStore(10 * time.Minute),
		pushInbox:       push.NewPushInbox(50),
		ptyManager:      pty.NewManager(),
	}

	// Initialize push router with capability sync callback
	caps := getPushClientCapabilities()
	s.pushRouter = push.NewRouter(s.pushInbox, &stdioSamplingSender{}, &stdioNotifier{}, caps)
	onPushCapabilitiesChange(func(newCaps push.ClientCapabilities) {
		s.pushRouter.UpdateCapabilities(newCaps)
	})

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

// setListenPort stores the active HTTP listener port for URL rewriting helpers.
func (s *Server) setListenPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if port > 0 {
		s.listenPort = port
	}
}

// getListenPort returns the active HTTP listener port.
func (s *Server) getListenPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listenPort <= 0 {
		return defaultPort
	}
	return s.listenPort
}

func (s *Server) getAnnotationStore() *AnnotationStore {
	if s == nil {
		return globalAnnotationStore
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.annotationStore == nil {
		s.annotationStore = NewAnnotationStore(10 * time.Minute)
	}
	return s.annotationStore
}

func (s *Server) closeAnnotationStore() {
	if s == nil {
		return
	}
	store := func() *AnnotationStore {
		s.mu.Lock()
		defer s.mu.Unlock()
		store := s.annotationStore
		s.annotationStore = nil
		return store
	}()
	if store != nil {
		store.Close()
	}
}

// setTerminalPort stores the port the terminal server is listening on.
func (s *Server) setTerminalPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.terminalPort = port
}

// getTerminalPort returns the terminal server port (0 if not running).
func (s *Server) getTerminalPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.terminalPort
}

// GetActiveCodebase returns the active codebase path (thread-safe).
func (s *Server) GetActiveCodebase() string {
	s.activeCodebaseMu.RLock()
	defer s.activeCodebaseMu.RUnlock()
	return s.activeCodebase
}

// SetActiveCodebase updates the active codebase path (thread-safe).
func (s *Server) SetActiveCodebase(path string) {
	s.activeCodebaseMu.Lock()
	defer s.activeCodebaseMu.Unlock()
	s.activeCodebase = path
}

// Close gracefully shuts down the server, draining the async log writer.
func (s *Server) Close() {
	s.shutdownAsyncLogger(2 * time.Second)
	s.closeAnnotationStore()
}

// SetOnEntries sets the callback invoked when new log entries are added.
// Thread-safe: acquires the write lock to avoid racing with addEntries.
func (s *Server) SetOnEntries(cb func([]LogEntry)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEntries = cb
}
