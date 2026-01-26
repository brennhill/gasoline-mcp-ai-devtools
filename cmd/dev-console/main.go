// Gasoline - Browser observability for AI coding agents
// A zero-dependency server that receives logs from the browser extension
// and streams them to your AI coding agent via MCP.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// version is set at build time via -ldflags "-X main.version=..."
// Fallback used for `go run` and `make dev` (no ldflags).
var version = "5.0.0"

// startTime tracks when the server started for uptime calculation
var startTime = time.Now()

const (
	defaultPort       = 7890
	defaultMaxEntries = 1000
)

// LogEntry represents a single log entry
type LogEntry map[string]interface{}

// ============================================
// MCP Protocol Types and Handler
// ============================================

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPTool represents a tool in the MCP protocol
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Meta        map[string]interface{} `json:"_meta,omitempty"`
}

// MCPHandler handles MCP protocol messages
type MCPHandler struct {
	server      *Server
	toolHandler *ToolHandler
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(server *Server) *MCPHandler {
	return &MCPHandler{
		server: server,
	}
}

// HandleHTTP handles MCP requests over HTTP (POST /mcp)
func (h *MCPHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error: " + err.Error(),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp := h.HandleRequest(req)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleRequest processes an MCP request and returns a response
func (h *MCPHandler) HandleRequest(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return h.handleInitialize(req)
	case "initialized":
		// Client notification that initialization is complete
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)}
	case "tools/list":
		return h.handleToolsList(req)
	case "tools/call":
		return h.handleToolsCall(req)
	case "resources/list":
		return h.handleResourcesList(req)
	case "resources/read":
		return h.handleResourcesRead(req)
	default:
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found: " + req.Method,
			},
		}
	}
}

func (h *MCPHandler) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	result := MCPInitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: MCPServerInfo{
			Name:    "gasoline",
			Version: version,
		},
		Capabilities: MCPCapabilities{
			Tools:     MCPToolsCapability{},
			Resources: MCPResourcesCapability{},
		},
	}

	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleResourcesList(req JSONRPCRequest) JSONRPCResponse {
	resources := []MCPResource{
		{
			URI:         "gasoline://guide",
			Name:        "Gasoline Usage Guide",
			Description: "How to use Gasoline MCP tools for browser debugging",
			MimeType:    "text/markdown",
		},
	}
	result := MCPResourcesListResult{Resources: resources}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleResourcesRead(req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params: " + err.Error(),
			},
		}
	}

	if params.URI != "gasoline://guide" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Resource not found: " + params.URI,
			},
		}
	}

	guide := `# Gasoline MCP Tools

Browser observability for AI coding agents. See console errors, network failures, DOM state, and more.

## Quick Reference

| Tool | Purpose | Key Parameter |
|------|---------|---------------|
| ` + "`observe`" + ` | Get browser state | ` + "`what`" + `: errors, logs, network, websocket_events, websocket_status, actions, vitals, page |
| ` + "`analyze`" + ` | Analyze data | ` + "`target`" + `: performance, accessibility, changes, timeline, api |
| ` + "`generate`" + ` | Create artifacts | ` + "`format`" + `: test, reproduction, pr_summary, sarif, har |
| ` + "`configure`" + ` | Manage session | ` + "`action`" + `: store, noise_rule, dismiss, clear |
| ` + "`query_dom`" + ` | Query live DOM | ` + "`selector`" + `: CSS selector |

## Common Workflows

### See browser errors
` + "```" + `json
{ "tool": "observe", "arguments": { "what": "errors" } }
` + "```" + `

### Check failed network requests
` + "```" + `json
{ "tool": "observe", "arguments": { "what": "network", "status_min": 400 } }
` + "```" + `

### Run accessibility audit
` + "```" + `json
{ "tool": "analyze", "arguments": { "target": "accessibility" } }
` + "```" + `

### Query DOM element
` + "```" + `json
{ "tool": "query_dom", "arguments": { "selector": ".error-message" } }
` + "```" + `

### Generate Playwright test from session
` + "```" + `json
{ "tool": "generate", "arguments": { "format": "test", "test_name": "user_login" } }
` + "```" + `

### Check Web Vitals (LCP, CLS, INP, FCP)
` + "```" + `json
{ "tool": "observe", "arguments": { "what": "vitals" } }
` + "```" + `

## Tips

- Start with ` + "`observe`" + ` ` + "`what: \"errors\"`" + ` to see what's broken
- Use ` + "`what: \"page\"`" + ` to confirm which URL the browser is on
- The browser extension must show "Connected" for tools to work
- Data comes from the active browser tab
`

	result := MCPResourcesReadResult{
		Contents: []MCPResourceContent{
			{
				URI:      "gasoline://guide",
				MimeType: "text/markdown",
				Text:     guide,
			},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	var tools []MCPTool
	if h.toolHandler != nil {
		tools = h.toolHandler.toolsList()
	}

	result := MCPToolsListResult{Tools: tools}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleToolsCall(req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params: " + err.Error(),
			},
		}
	}

	if h.toolHandler != nil {
		if resp, handled := h.toolHandler.handleToolCall(req, params.Name, params.Arguments); handled {
			// Apply redaction to tool response before returning to AI client
			if h.toolHandler.redactionEngine != nil && resp.Result != nil {
				resp.Result = h.toolHandler.redactionEngine.RedactJSON(resp.Result)
			}
			return resp
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error: &JSONRPCError{
			Code:    -32601,
			Message: "Unknown tool: " + params.Name,
		},
	}
}

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
		s.entries = s.entries[len(s.entries)-s.maxEntries:]
	}

	return scanner.Err()
}

// saveEntries writes all entries to file
func (s *Server) saveEntries() error {
	file, err := os.Create(s.logFile)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // deferred close

	for _, entry := range s.entries {
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

// handleScreenshot saves a screenshot JPEG to disk and returns the filename
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		DataURL   string `json:"dataUrl"`
		URL       string `json:"url"`
		ErrorID   string `json:"errorId"`
		ErrorType string `json:"errorType"`
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

	// Build filename: [website]-[timestamp]-[errortype]-[errorid].jpg
	hostname := "unknown"
	if body.URL != "" {
		if u, err := url.Parse(body.URL); err == nil && u.Host != "" {
			hostname = u.Host
		}
	}

	timestamp := time.Now().Format("20060102-150405")
	errorType := "unknown"
	if body.ErrorType != "" {
		errorType = sanitizeForFilename(body.ErrorType)
	}
	errorID := "manual"
	if body.ErrorID != "" {
		errorID = sanitizeForFilename(body.ErrorID)
	}

	filename := fmt.Sprintf("%s-%s-%s-%s.jpg",
		sanitizeForFilename(hostname), timestamp, errorType, errorID)

	// Save to same directory as log file
	dir := filepath.Dir(s.logFile)
	savePath := filepath.Join(dir, filename)

	// #nosec G306 -- screenshots are intentionally world-readable
	if err := os.WriteFile(savePath, imageData, 0o644); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save screenshot"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"filename": filename,
		"path":     savePath,
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

	// Rotate if needed — requires full rewrite
	rotated := len(s.entries) > s.maxEntries
	if rotated {
		s.entries = s.entries[len(s.entries)-s.maxEntries:]
		s.logAddedAt = s.logAddedAt[len(s.logAddedAt)-s.maxEntries:]
	}

	var err error
	if rotated {
		err = s.saveEntries()
	} else {
		err = s.appendToFile(newEntries)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error saving entries: %v\n", err)
	}

	cb := s.onEntries
	s.mu.Unlock()

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
	defer s.mu.Unlock()

	s.entries = make([]LogEntry, 0)
	if err := s.saveEntries(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error saving entries: %v\n", err)
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

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Gasoline-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// JSON response helper
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error encoding JSON response: %v\n", err)
	}
}

func main() {
	// Install panic recovery with diagnostic logging
	defer func() {
		if r := recover(); r != nil {
			// Get stack trace
			stack := make([]byte, 4096)
			n := runtime.Stack(stack, false)
			stack = stack[:n]

			// Log to stderr
			fmt.Fprintf(os.Stderr, "\n[gasoline] FATAL PANIC: %v\n", r)
			fmt.Fprintf(os.Stderr, "[gasoline] Stack trace:\n%s\n", stack)

			// Try to log to file
			home, _ := os.UserHomeDir()
			logFile := filepath.Join(home, "gasoline-logs.jsonl")
			entry := map[string]interface{}{
				"type":       "lifecycle",
				"event":      "crash",
				"reason":     fmt.Sprintf("%v", r),
				"stack":      string(stack),
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
				"go_version": runtime.Version(),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
			}
			if data, err := json.Marshal(entry); err == nil {
				// #nosec G302 G304 -- crash logs are intentionally world-readable for debugging
				if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err == nil {
					_, _ = f.Write(append(data, '\n')) // #nosec G104 -- best-effort crash logging
					_ = f.Close()                      // #nosec G104 -- best-effort crash logging
				}
			}

			// Also write to a dedicated crash file for easy discovery
			crashFile := filepath.Join(home, "gasoline-crash.log")
			crashContent := fmt.Sprintf("GASOLINE CRASH at %s\nPanic: %v\nStack:\n%s\n",
				time.Now().Format(time.RFC3339), r, stack)
			_ = os.WriteFile(crashFile, []byte(crashContent), 0644) // #nosec G104 G306 -- best-effort crash logging; intentionally world-readable

			fmt.Fprintf(os.Stderr, "[gasoline] Crash details written to: %s\n", crashFile)
			os.Exit(1)
		}
	}()

	// Parse flags
	port := flag.Int("port", defaultPort, "Port to listen on")
	logFile := flag.String("log-file", "", "Path to log file (default: ~/gasoline-logs.jsonl)")
	maxEntries := flag.Int("max-entries", defaultMaxEntries, "Max log entries before rotation")
	showVersion := flag.Bool("version", false, "Show version")
	showHelp := flag.Bool("help", false, "Show help")
	serverOnly := flag.Bool("server", false, "Run in HTTP-only mode (no MCP)")
	apiKey := flag.String("api-key", "", "API key for HTTP authentication (optional)")
	flag.Bool("mcp", false, "Run in MCP mode (default, kept for backwards compatibility)")

	flag.Parse()

	if *showVersion {
		fmt.Printf("gasoline v%s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	// Default log file to home directory
	if *logFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			os.Exit(1)
		}
		*logFile = filepath.Join(home, "gasoline-logs.jsonl")
	}

	// Create server
	server, err := NewServer(*logFile, *maxEntries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	// Determine mode:
	// 1. --server flag → foreground HTTP server with banner
	// 2. stdin is a terminal (user typed "gasoline") → daemonize HTTP server
	// 3. stdin is a pipe (MCP host launched us) → MCP mode
	if !*serverOnly {
		stat, _ := os.Stdin.Stat()
		isTTY := (stat.Mode() & os.ModeCharDevice) != 0
		stdinMode := stat.Mode()

		// Log mode detection for diagnostics
		_ = server.appendToFile([]LogEntry{{
			"type":       "lifecycle",
			"event":      "mode_detection",
			"is_tty":     isTTY,
			"stdin_mode": fmt.Sprintf("%v", stdinMode),
			"server_flag": *serverOnly,
			"pid":        os.Getpid(),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		}})

		if isTTY {
			// User ran "gasoline" directly - start server as background process
			exe, _ := os.Executable()
			args := []string{"--server", "--port", fmt.Sprintf("%d", *port), "--log-file", *logFile, "--max-entries", fmt.Sprintf("%d", *maxEntries)}
			if *apiKey != "" {
				args = append(args, "--api-key", *apiKey)
			}
			cmd := exec.Command(exe, args...) // #nosec G204 -- exe is our own binary path from os.Executable()
			cmd.Stdout = nil
			cmd.Stderr = nil
			cmd.Stdin = nil
			setDetachedProcess(cmd)
			if err := cmd.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting background server: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("[gasoline] Server started (pid %d), HTTP on port %d, log file: %s\n", cmd.Process.Pid, *port, *logFile)
			fmt.Println("[gasoline] Stop with: kill", cmd.Process.Pid)
			os.Exit(0)
		}

		// stdin is piped → MCP mode (will shut down when stdin closes)
		fmt.Fprintf(os.Stderr, "[gasoline] Starting in MCP mode (stdin is pipe, will exit when MCP client disconnects)\n")
		runMCPMode(server, *port, *apiKey)
		return
	}

	// Log that we're starting in server-only mode
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "mode_detection",
		"mode":      "http_only",
		"server_flag": true,
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})

	// HTTP-only server mode (--server)
	// Setup routes
	capture := NewCapture()
	setupHTTPRoutes(server, capture)

	// Print banner
	fmt.Println()
	fmt.Printf("  Gasoline v%s (pid %d)\n", version, os.Getpid())
	fmt.Println("  Browser observability for AI coding agents")
	fmt.Println()
	fmt.Printf("  Server:  http://127.0.0.1:%d\n", *port)
	fmt.Printf("  Logs:    %s\n", *logFile)
	fmt.Printf("  Max:     %d entries\n", *maxEntries)
	fmt.Println()
	fmt.Println("  Ready. Press Ctrl+C to stop.")
	fmt.Println()

	// Log startup
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "startup",
		"mode":      "http_only",
		"version":   version,
		"port":      *port,
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})

	// Setup graceful shutdown on signals
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Start server (localhost only for security)
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	srv := &http.Server{
		Addr:         addr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      AuthMiddleware(*apiKey)(http.DefaultServeMux),
	}

	// Run server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for signal or server error
	select {
	case s := <-sig:
		fmt.Fprintf(os.Stderr, "\n[gasoline] Received %s, shutting down gracefully...\n", s)
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "shutdown",
			"reason":    s.String(),
			"mode":      "http_only",
			"pid":       os.Getpid(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx) // #nosec G104 -- shutdown error is logged but not actionable
		fmt.Fprintf(os.Stderr, "[gasoline] Shutdown complete\n")
	case err := <-serverErr:
		fmt.Fprintf(os.Stderr, "[gasoline] Server error: %v\n", err)
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "crash",
			"reason":    err.Error(),
			"mode":      "http_only",
			"pid":       os.Getpid(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		os.Exit(1)
	}
}

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
// If stdin closes (EOF), the HTTP server keeps running until killed.
func runMCPMode(server *Server, port int, apiKey string) {
	// Create capture buffers for WebSocket, network, and actions
	capture := NewCapture()

	// Start HTTP server in background for browser extension
	httpReady := make(chan error, 1)
	go func() {
		setupHTTPRoutes(server, capture)
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			httpReady <- err
			return
		}
		httpReady <- nil
		// #nosec G114 -- localhost-only MCP background server
		if err := http.Serve(ln, AuthMiddleware(apiKey)(http.DefaultServeMux)); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] HTTP server error: %v\n", err)
		}
	}()

	// Wait for HTTP server to bind before proceeding
	if err := <-httpReady; err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Fatal: cannot bind port %d: %v\n", port, err)
		fmt.Fprintf(os.Stderr, "[gasoline] Fix: kill existing process with: lsof -ti :%d | xargs kill\n", port)
		fmt.Fprintf(os.Stderr, "[gasoline] Or use a different port: --port %d\n", port+1)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[gasoline] v%s — HTTP on port %d, log: %s\n", version, port, server.logFile)
	fmt.Fprintf(os.Stderr, "[gasoline] Verify: curl http://localhost:%d/health\n", port)
	_ = server.appendToFile([]LogEntry{{"type": "lifecycle", "event": "startup", "version": version, "port": port, "timestamp": time.Now().UTC().Format(time.RFC3339)}})

	// Run MCP protocol over stdin/stdout
	mcp := NewToolHandler(server, capture)
	scanner := bufio.NewScanner(os.Stdin)

	// Increase scanner buffer for large messages
	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Send error response for malformed JSON
			errResp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &JSONRPCError{
					Code:    -32700,
					Message: "Parse error: " + err.Error(),
				},
			}
			respJSON, _ := json.Marshal(errResp)
			fmt.Println(string(respJSON))
			continue
		}

		resp := mcp.HandleRequest(req)
		respJSON, _ := json.Marshal(resp)
		fmt.Println(string(respJSON))
	}

	// stdin closed (MCP client disconnected) — exit after brief grace period
	// This frees the port so the next AI session can spawn a fresh process.
	// The extension will auto-reconnect to the new instance.
	// Grace period is kept short (100ms) to avoid race conditions when starting new sessions.
	fmt.Fprintf(os.Stderr, "[gasoline] MCP disconnected, shutting down in 100ms (port %d will be freed)\n", port)
	_ = server.appendToFile([]LogEntry{{"type": "lifecycle", "event": "mcp_disconnect", "timestamp": time.Now().UTC().Format(time.RFC3339), "port": port}})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	select {
	case s := <-sig:
		fmt.Fprintf(os.Stderr, "[gasoline] Received %s, shutting down\n", s)
		_ = server.appendToFile([]LogEntry{{"type": "lifecycle", "event": "shutdown", "reason": s.String(), "timestamp": time.Now().UTC().Format(time.RFC3339)}})
	case <-time.After(100 * time.Millisecond):
		fmt.Fprintf(os.Stderr, "[gasoline] Shutdown complete\n")
		_ = server.appendToFile([]LogEntry{{"type": "lifecycle", "event": "shutdown", "reason": "mcp_disconnect_grace", "timestamp": time.Now().UTC().Format(time.RFC3339)}})
	}
}

// setupHTTPRoutes configures the HTTP routes (extracted for reuse)
func setupHTTPRoutes(server *Server, capture *Capture) {
	// V4 routes
	if capture != nil {
		http.HandleFunc("/websocket-events", corsMiddleware(capture.HandleWebSocketEvents))
		http.HandleFunc("/websocket-status", corsMiddleware(capture.HandleWebSocketStatus))
		http.HandleFunc("/network-bodies", corsMiddleware(capture.HandleNetworkBodies))
		http.HandleFunc("/pending-queries", corsMiddleware(capture.HandlePendingQueries))
		http.HandleFunc("/dom-result", corsMiddleware(capture.HandleDOMResult))
		http.HandleFunc("/a11y-result", corsMiddleware(capture.HandleA11yResult))
		http.HandleFunc("/state-result", corsMiddleware(capture.HandleStateResult))
		http.HandleFunc("/execute-result", corsMiddleware(capture.HandleExecuteResult))
		http.HandleFunc("/highlight-result", corsMiddleware(capture.HandleHighlightResult))
		http.HandleFunc("/enhanced-actions", corsMiddleware(capture.HandleEnhancedActions))
		http.HandleFunc("/performance-snapshot", corsMiddleware(capture.HandlePerformanceSnapshot))
	}

	// MCP over HTTP endpoint
	mcp := NewToolHandler(server, capture)
	http.HandleFunc("/mcp", corsMiddleware(mcp.HandleHTTP))

	// CI/CD webhook endpoint for push-based alerts
	if mcp.toolHandler != nil {
		http.HandleFunc("/ci-result", corsMiddleware(mcp.toolHandler.handleCIWebhook))
	}

	// Settings endpoint for extension polling (capture overrides)
	if mcp.toolHandler != nil && mcp.toolHandler.captureOverrides != nil {
		overrides := mcp.toolHandler.captureOverrides
		http.HandleFunc("/settings", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
				return
			}
			resp := buildSettingsResponse(overrides)
			jsonResponse(w, http.StatusOK, resp)
		}))
	}

	http.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		resp := map[string]interface{}{
			"status":  "ok",
			"version": version,
			"logs": map[string]interface{}{
				"entries":    server.getEntryCount(),
				"maxEntries": server.maxEntries,
				"logFile":    server.logFile,
			},
		}

		if capture != nil {
			health := capture.GetHealthStatus()
			capture.mu.RLock()
			resp["buffers"] = map[string]interface{}{
				"websocket_events": len(capture.wsEvents),
				"network_bodies":   len(capture.networkBodies),
				"actions":          len(capture.enhancedActions),
				"connections":      len(capture.connections),
			}
			lastPoll := capture.lastPollAt
			extSession := capture.extensionSession
			sessionChangedAt := capture.sessionChangedAt
			capture.mu.RUnlock()

			// Extension connection status (critical for debugging)
			if lastPoll.IsZero() {
				resp["extension"] = map[string]interface{}{
					"connected": false,
					"status":    "not_polled",
					"message":   "Extension has not connected. Reload extension or refresh page.",
				}
			} else {
				sincePoll := time.Since(lastPoll)
				connected := sincePoll < 3*time.Second
				capture.mu.RLock()
				pilotEnabled := capture.pilotEnabled
				capture.mu.RUnlock()
				extInfo := map[string]interface{}{
					"connected":     connected,
					"status":        map[bool]string{true: "connected", false: "stale"}[connected],
					"last_poll_ms":  int(sincePoll.Milliseconds()),
					"pilot_enabled": pilotEnabled,
				}
				if extSession != "" {
					extInfo["session_id"] = extSession
					if !sessionChangedAt.IsZero() {
						extInfo["session_started"] = sessionChangedAt.Format(time.RFC3339)
					}
				}
				resp["extension"] = extInfo
			}

			resp["circuit"] = map[string]interface{}{
				"open":         health.CircuitOpen,
				"current_rate": health.CurrentRate,
				"memory_bytes": health.MemoryBytes,
				"reason":       health.Reason,
				"opened_at":    health.OpenedAt,
			}
		}

		jsonResponse(w, http.StatusOK, resp)
	}))

	// Diagnostics endpoint for bug reports - comprehensive server state dump
	http.HandleFunc("/diagnostics", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		now := time.Now()
		resp := map[string]interface{}{
			"generated_at":   now.Format(time.RFC3339),
			"version":        version,
			"uptime_seconds": int(now.Sub(startTime).Seconds()),
			"system": map[string]interface{}{
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
				"go_version": runtime.Version(),
				"goroutines": runtime.NumGoroutine(),
			},
			"logs": map[string]interface{}{
				"entries":     server.getEntryCount(),
				"max_entries": server.maxEntries,
				"log_file":    server.logFile,
			},
		}

		if capture != nil {
			health := capture.GetHealthStatus()

			// Buffer counts
			capture.mu.RLock()
			resp["buffers"] = map[string]interface{}{
				"websocket_events": len(capture.wsEvents),
				"network_bodies":   len(capture.networkBodies),
				"actions":          len(capture.enhancedActions),
				"pending_queries":  len(capture.pendingQueries),
				"query_results":    len(capture.queryResults),
			}

			// WebSocket connection info (sanitized - no sensitive data)
			wsConnections := make([]map[string]interface{}, 0, len(capture.connections))
			for connID, conn := range capture.connections {
				wsConnections = append(wsConnections, map[string]interface{}{
					"id":    connID,
					"url":   conn.url,
					"state": conn.state,
				})
			}
			resp["websocket_connections"] = wsConnections

			// Query timeout config
			resp["config"] = map[string]interface{}{
				"query_timeout": capture.queryTimeout.String(),
			}

			// Extension polling info
			lastPoll := capture.lastPollAt
			capture.mu.RUnlock()

			// Extension status for debugging
			if lastPoll.IsZero() {
				resp["extension"] = map[string]interface{}{
					"polling":      false,
					"last_poll_at": nil,
					"status":       "Extension has not polled /pending-queries yet. Reload extension and refresh page.",
				}
			} else {
				sincePoll := time.Since(lastPoll)
				polling := sincePoll < 3*time.Second // Should poll every 1s
				capture.mu.RLock()
				pilotEnabled := capture.pilotEnabled
				capture.mu.RUnlock()
				resp["extension"] = map[string]interface{}{
					"polling":       polling,
					"last_poll_at":  lastPoll.Format(time.RFC3339),
					"seconds_ago":   int(sincePoll.Seconds()),
					"status":        map[bool]string{true: "connected", false: "stale - extension may have disconnected"}[polling],
					"pilot_enabled": pilotEnabled,
				}
			}

			// Circuit breaker state
			resp["circuit"] = map[string]interface{}{
				"open":         health.CircuitOpen,
				"current_rate": health.CurrentRate,
				"memory_bytes": health.MemoryBytes,
				"reason":       health.Reason,
			}
		}

		// Last events - for verifying data flow without manual inspection
		lastEvents := map[string]interface{}{}

		// Last console log/error
		server.mu.RLock()
		if len(server.entries) > 0 {
			last := server.entries[len(server.entries)-1]
			// Truncate args for display
			args := last["args"]
			if argsSlice, ok := args.([]interface{}); ok && len(argsSlice) > 0 {
				if s, ok := argsSlice[0].(string); ok && len(s) > 100 {
					args = s[:100] + "..."
				} else {
					args = argsSlice[0]
				}
			}
			lastEvents["console"] = map[string]interface{}{
				"level":   last["level"],
				"message": args,
				"ts":      last["ts"],
			}
		}
		server.mu.RUnlock()

		// Last network request, action, websocket
		if capture != nil {
			capture.mu.RLock()
			if len(capture.networkBodies) > 0 {
				last := capture.networkBodies[len(capture.networkBodies)-1]
				// Truncate URL for display
				url := last.URL
				if len(url) > 80 {
					url = url[:80] + "..."
				}
				lastEvents["network"] = map[string]interface{}{
					"method": last.Method,
					"url":    url,
					"status": last.Status,
				}
			}
			if len(capture.enhancedActions) > 0 {
				last := capture.enhancedActions[len(capture.enhancedActions)-1]
				lastEvents["action"] = map[string]interface{}{
					"type": last.Type,
					"ts":   last.Timestamp,
				}
			}
			if len(capture.wsEvents) > 0 {
				last := capture.wsEvents[len(capture.wsEvents)-1]
				lastEvents["websocket"] = map[string]interface{}{
					"type":      last.Type,
					"direction": last.Direction,
				}
			}
			capture.mu.RUnlock()
		}
		resp["last_events"] = lastEvents

		jsonResponse(w, http.StatusOK, resp)
	}))

	http.HandleFunc("/logs", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			entries := server.getEntries()
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"entries": entries,
				"count":   len(entries),
			})

		case "POST":
			r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
			var body struct {
				Entries []LogEntry `json:"entries"`
			}

			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
				return
			}

			if body.Entries == nil {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing entries array"})
				return
			}

			valid, rejected := validateLogEntries(body.Entries)
			received := server.addEntries(valid)
			jsonResponse(w, http.StatusOK, map[string]int{
				"received": received,
				"rejected": rejected,
				"entries":  server.getEntryCount(),
			})

		case "DELETE":
			server.clearEntries()
			jsonResponse(w, http.StatusOK, map[string]bool{"cleared": true})

		default:
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		}
	}))

	http.HandleFunc("/screenshots", corsMiddleware(server.handleScreenshot))

	http.HandleFunc("/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{
			"name":    "gasoline",
			"version": version,
			"health":  "/health",
			"logs":    "/logs",
		})
	}))
}

func printHelp() {
	fmt.Print(`
Gasoline - Browser observability for AI coding agents

Usage: gasoline [options]

Options:
  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: ~/gasoline-logs.jsonl)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --server               Run in HTTP-only mode (no MCP, just the log server)
  --version              Show version
  --help                 Show this help message

By default, gasoline runs in MCP mode: the HTTP server starts in the
background (for the browser extension) and MCP protocol runs over stdio
(for Claude Code, Cursor, etc.).

Example:
  gasoline                           # MCP mode (default)
  gasoline --server                  # HTTP-only server mode
  gasoline --port 8080 --max-entries 500

MCP Configuration:
  Add to your Claude Code settings.json or project .mcp.json:
  {
    "mcpServers": {
      "gasoline": {
        "command": "npx",
        "args": ["gasoline-mcp"]
      }
    }
  }
`)
}
