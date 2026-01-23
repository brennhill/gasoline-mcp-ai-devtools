// Gasoline - Adding fuel to the AI fire
// A zero-dependency server that receives logs from the browser extension
// and writes them to a JSONL file for your AI coding assistant.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultPort       = 7890
	defaultMaxEntries = 1000
	version           = "3.0.2"
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
}

// MCPHandler handles MCP protocol messages
type MCPHandler struct {
	server      *Server
	initialized bool
	v4Handler   *MCPHandlerV4
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(server *Server) *MCPHandler {
	return &MCPHandler{
		server:      server,
		initialized: false,
	}
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
	h.initialized = true

	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "gasoline",
			"version": version,
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}

	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	tools := []MCPTool{
		{
			Name:        "get_browser_errors",
			Description: "Get recent browser errors (console errors, network failures, exceptions) from the Gasoline log. Useful for debugging web application issues.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_browser_logs",
			Description: "Get all browser logs from the Gasoline log, including errors, warnings, and info messages.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of log entries to return (default: all)",
					},
				},
			},
		},
		{
			Name:        "clear_browser_logs",
			Description: "Clear all browser logs from the Gasoline log file.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	// Add v4 tools if available
	if h.v4Handler != nil {
		tools = append(tools, h.v4Handler.v4ToolsList()...)
	}

	result := map[string]interface{}{"tools": tools}
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

	switch params.Name {
	case "get_browser_errors":
		return h.toolGetBrowserErrors(req, params.Arguments)
	case "get_browser_logs":
		return h.toolGetBrowserLogs(req, params.Arguments)
	case "clear_browser_logs":
		return h.toolClearBrowserLogs(req)
	default:
		// Try v4 handler
		if h.v4Handler != nil {
			if resp, handled := h.v4Handler.handleV4ToolCall(req, params.Name, params.Arguments); handled {
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
}

func (h *MCPHandler) toolGetBrowserErrors(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()

	// Filter for error-level entries only
	var errors []LogEntry
	for _, entry := range h.server.entries {
		if level, ok := entry["level"].(string); ok && level == "error" {
			errors = append(errors, entry)
		}
	}

	var contentText string
	if len(errors) == 0 {
		contentText = "No browser errors found"
	} else {
		errorsJSON, _ := json.Marshal(errors)
		contentText = string(errorsJSON)
	}

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": contentText},
		},
	}

	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) toolGetBrowserLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Limit int `json:"limit"`
	}
	json.Unmarshal(args, &arguments)

	h.server.mu.RLock()
	defer h.server.mu.RUnlock()

	entries := h.server.entries

	// Apply limit if specified
	if arguments.Limit > 0 && arguments.Limit < len(entries) {
		// Return the most recent entries
		entries = entries[len(entries)-arguments.Limit:]
	}

	var contentText string
	if len(entries) == 0 {
		contentText = "No browser logs found"
	} else {
		entriesJSON, _ := json.Marshal(entries)
		contentText = string(entriesJSON)
	}

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": contentText},
		},
	}

	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) toolClearBrowserLogs(req JSONRPCRequest) JSONRPCResponse {
	h.server.clearEntries()

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": "Browser logs cleared successfully"},
		},
	}

	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

// Server holds the server state
type Server struct {
	logFile    string
	maxEntries int
	entries    []LogEntry
	mu         sync.RWMutex
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
	if err := os.MkdirAll(dir, 0755); err != nil {
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
	defer file.Close()

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

	return scanner.Err()
}

// saveEntries writes all entries to file
func (s *Server) saveEntries() error {
	file, err := os.Create(s.logFile)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, entry := range s.entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		file.Write(data)
		file.WriteString("\n")
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

	var body struct {
		DataUrl   string `json:"dataUrl"`
		URL       string `json:"url"`
		ErrorID   string `json:"errorId"`
		ErrorType string `json:"errorType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.DataUrl == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing dataUrl"})
		return
	}

	// Extract base64 data from data URL
	parts := strings.SplitN(body.DataUrl, ",", 2)
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

	if err := os.WriteFile(savePath, imageData, 0644); err != nil {
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
	defer s.mu.Unlock()

	s.entries = append(s.entries, newEntries...)

	// Rotate if needed
	if len(s.entries) > s.maxEntries {
		s.entries = s.entries[len(s.entries)-s.maxEntries:]
	}

	s.saveEntries()
	return len(newEntries)
}

// clearEntries removes all entries
func (s *Server) clearEntries() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = make([]LogEntry, 0)
	s.saveEntries()
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

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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
	json.NewEncoder(w).Encode(data)
}

func main() {
	// Parse flags
	port := flag.Int("port", defaultPort, "Port to listen on")
	logFile := flag.String("log-file", "", "Path to log file (default: ~/gasoline-logs.jsonl)")
	maxEntries := flag.Int("max-entries", defaultMaxEntries, "Max log entries before rotation")
	showVersion := flag.Bool("version", false, "Show version")
	showHelp := flag.Bool("help", false, "Show help")
	serverOnly := flag.Bool("server", false, "Run in HTTP-only mode (no MCP)")
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

		if isTTY {
			// User ran "gasoline" directly - start server as background process
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "--server", "--port", fmt.Sprintf("%d", *port), "--log-file", *logFile, "--max-entries", fmt.Sprintf("%d", *maxEntries))
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

		// stdin is piped → MCP mode
		runMCPMode(server, *port)
		return
	}

	// HTTP-only server mode (--server)
	// Setup routes
	v4 := NewV4Server()
	setupHTTPRoutes(server, v4)

	// Print banner
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                       Gasoline                             ║")
	fmt.Println("║              Adding fuel to the AI fire                   ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("✓ Server listening on http://127.0.0.1:%d\n", *port)
	fmt.Printf("✓ Writing logs to %s\n", *logFile)
	fmt.Printf("✓ Max entries: %d\n", *maxEntries)
	fmt.Println()
	fmt.Println("Ready to receive logs from browser extension.")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	// Start server (localhost only for security)
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
func runMCPMode(server *Server, port int) {
	fmt.Fprintf(os.Stderr, "[gasoline] Starting MCP server, HTTP on port %d, log file: %s\n", port, server.logFile)

	// Create v4 server for WebSocket/network body capture
	v4 := NewV4Server()

	// Start HTTP server in background for browser extension
	go func() {
		setupHTTPRoutes(server, v4)
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		http.ListenAndServe(addr, nil)
	}()

	// Run MCP protocol over stdin/stdout (with v4 tools)
	mcp := NewMCPHandlerV4(server, v4)
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
}

// setupHTTPRoutes configures the HTTP routes (extracted for reuse)
func setupHTTPRoutes(server *Server, v4 *V4Server) {
	// V4 routes
	if v4 != nil {
		http.HandleFunc("/websocket-events", corsMiddleware(v4.HandleWebSocketEvents))
		http.HandleFunc("/websocket-status", corsMiddleware(v4.HandleWebSocketStatus))
		http.HandleFunc("/network-bodies", corsMiddleware(v4.HandleNetworkBodies))
		http.HandleFunc("/pending-queries", corsMiddleware(v4.HandlePendingQueries))
		http.HandleFunc("/dom-result", corsMiddleware(v4.HandleDOMResult))
		http.HandleFunc("/a11y-result", corsMiddleware(v4.HandleA11yResult))
		http.HandleFunc("/enhanced-actions", corsMiddleware(v4.HandleEnhancedActions))
	}

	http.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"status":     "ok",
			"entries":    server.getEntryCount(),
			"maxEntries": server.maxEntries,
			"logFile":    server.logFile,
		})
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

			received := server.addEntries(body.Entries)
			jsonResponse(w, http.StatusOK, map[string]int{"received": received})

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
Gasoline - Adding fuel to the AI fire

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
        "args": ["gasoline-cli"]
      }
    }
  }
`)
}
