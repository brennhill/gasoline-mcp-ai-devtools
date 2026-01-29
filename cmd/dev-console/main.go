// Gasoline - Browser observability for AI coding agents
// A zero-dependency server that receives logs from the browser extension
// and streams them to your AI coding agent via MCP.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
var version = "5.2.0"

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
	JSONRPC  string          `json:"jsonrpc"` // camelCase: JSON-RPC 2.0 spec standard
	ID       interface{}     `json:"id"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params,omitempty"`
	ClientID string          `json:"-"` // per-request client ID for multi-client isolation (not serialized)
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"` // camelCase: JSON-RPC 2.0 spec standard
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
	InputSchema map[string]interface{} `json:"inputSchema"` // camelCase: MCP spec standard
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
	startTime := time.Now()
	sessionID := r.Header.Get("X-Gasoline-Session")
	clientID := r.Header.Get("X-Gasoline-Client")

	// Collect all headers for debug logging (redact auth)
	headers := make(map[string]string)
	for name, values := range r.Header {
		if strings.Contains(strings.ToLower(name), "auth") || strings.Contains(strings.ToLower(name), "token") {
			headers[name] = "[REDACTED]"
		} else if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Read body for logging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		if h.toolHandler != nil && h.toolHandler.capture != nil {
			duration := time.Since(startTime)
			debugEntry := HTTPDebugEntry{
				Timestamp:      startTime,
				Endpoint:       "/mcp",
				Method:         "POST",
				SessionID:      sessionID,
				ClientID:       clientID,
				Headers:        headers,
				ResponseStatus: http.StatusBadRequest,
				DurationMs:     duration.Milliseconds(),
				Error:          fmt.Sprintf("Could not read body: %v", err),
			}
			h.toolHandler.capture.mu.Lock()
			h.toolHandler.capture.logHTTPDebugEntry(debugEntry)
			h.toolHandler.capture.mu.Unlock()
			printHTTPDebug(debugEntry)
		}
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Read error: " + err.Error(),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	requestPreview := string(bodyBytes)
	if len(requestPreview) > 1000 {
		requestPreview = requestPreview[:1000] + "..."
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		if h.toolHandler != nil && h.toolHandler.capture != nil {
			duration := time.Since(startTime)
			debugEntry := HTTPDebugEntry{
				Timestamp:      startTime,
				Endpoint:       "/mcp",
				Method:         "POST",
				SessionID:      sessionID,
				ClientID:       clientID,
				Headers:        headers,
				RequestBody:    requestPreview,
				ResponseStatus: http.StatusBadRequest,
				DurationMs:     duration.Milliseconds(),
				Error:          fmt.Sprintf("Parse error: %v", err),
			}
			h.toolHandler.capture.mu.Lock()
			h.toolHandler.capture.logHTTPDebugEntry(debugEntry)
			h.toolHandler.capture.mu.Unlock()
			printHTTPDebug(debugEntry)
		}
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

	// Extract client ID for multi-client isolation (stored on the request, not the handler)
	req.ClientID = clientID

	resp := h.HandleRequest(req)

	// Log debug entry
	if h.toolHandler != nil && h.toolHandler.capture != nil {
		duration := time.Since(startTime)
		responseJSON, _ := json.Marshal(resp)
		responsePreview := string(responseJSON)
		if len(responsePreview) > 1000 {
			responsePreview = responsePreview[:1000] + "..."
		}

		debugEntry := HTTPDebugEntry{
			Timestamp:      startTime,
			Endpoint:       "/mcp",
			Method:         "POST",
			SessionID:      sessionID,
			ClientID:       clientID,
			Headers:        headers,
			RequestBody:    requestPreview,
			ResponseStatus: http.StatusOK,
			ResponseBody:   responsePreview,
			DurationMs:     duration.Milliseconds(),
		}
		h.toolHandler.capture.mu.Lock()
		h.toolHandler.capture.logHTTPDebugEntry(debugEntry)
		h.toolHandler.capture.mu.Unlock()
		printHTTPDebug(debugEntry)
	}

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
	case "resources/templates/list":
		return h.handleResourcesTemplatesList(req)
	case "ping":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)}
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
	const supportedVersion = "2024-11-05"

	// Parse client's requested protocol version (best-effort; missing/empty is fine)
	var initParams struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &initParams)
	}

	// Negotiate: echo client's version if supported, otherwise respond with our latest
	negotiatedVersion := supportedVersion
	if initParams.ProtocolVersion == supportedVersion {
		negotiatedVersion = initParams.ProtocolVersion
	}

	result := MCPInitializeResult{
		ProtocolVersion: negotiatedVersion,
		ServerInfo: MCPServerInfo{
			Name:    "gasoline",
			Version: version,
		},
		Capabilities: MCPCapabilities{
			Tools:     MCPToolsCapability{},
			Resources: MCPResourcesCapability{},
		},
	}

	// Error impossible: MCPInitResult is a simple struct with no circular refs or unsupported types
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
	// Error impossible: MCPResourcesListResult is a simple struct with no circular refs or unsupported types
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
				Code:    -32002,
				Message: "Resource not found: " + params.URI,
			},
		}
	}

	guide := `# Gasoline MCP Tools

Browser observability for AI coding agents. See console errors, network failures, DOM state, and more.

## Quick Reference

| Tool | Purpose | Key Parameter |
|------|---------|---------------|
| ` + "`observe`" + ` | Read browser state & analyze | ` + "`what`" + `: errors, logs, network, vitals, page, performance, accessibility, api, changes, timeline, security_audit |
| ` + "`generate`" + ` | Create artifacts | ` + "`format`" + `: test, reproduction, pr_summary, sarif, har, csp, sri |
| ` + "`configure`" + ` | Manage session & settings | ` + "`action`" + `: store, noise_rule, dismiss, clear, query_dom, health |
| ` + "`interact`" + ` | Control the browser | ` + "`action`" + `: highlight, save_state, load_state, execute_js, navigate, refresh |

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
{ "tool": "observe", "arguments": { "what": "accessibility" } }
` + "```" + `

### Query DOM element
` + "```" + `json
{ "tool": "configure", "arguments": { "action": "query_dom", "selector": ".error-message" } }
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
	// Error impossible: MCPResourceContentResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleResourcesTemplatesList(req JSONRPCRequest) JSONRPCResponse {
	result := MCPResourceTemplatesListResult{ResourceTemplates: []interface{}{}}
	// Error impossible: MCPResourceTemplatesListResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	var tools []MCPTool
	if h.toolHandler != nil {
		tools = h.toolHandler.toolsList()
	}

	result := MCPToolsListResult{Tools: tools}
	// Error impossible: MCPToolsListResult is a simple struct with no circular refs or unsupported types
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

	// Check tool call rate limit before dispatch
	if h.toolHandler != nil && h.toolHandler.toolCallLimiter != nil {
		if !h.toolHandler.toolCallLimiter.Allow() {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32603,
					Message: "Tool call rate limit exceeded (100 calls/minute). Please wait before retrying.",
				},
			}
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

// isAllowedOrigin checks if an Origin header value is from localhost or a browser extension.
// Returns true for empty origin (CLI/curl), localhost variants, and browser extension origins.
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}

	// Browser extension origins - validate specific ID if configured
	if strings.HasPrefix(origin, "chrome-extension://") {
		expectedID := os.Getenv("GASOLINE_EXTENSION_ID")
		if expectedID != "" {
			return origin == "chrome-extension://"+expectedID
		}
		return true // Permissive mode when not configured
	}
	if strings.HasPrefix(origin, "moz-extension://") {
		expectedID := os.Getenv("GASOLINE_FIREFOX_EXTENSION_ID")
		if expectedID != "" {
			return origin == "moz-extension://"+expectedID
		}
		return true // Permissive mode when not configured
	}

	// Parse the origin URL to extract the hostname
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	hostname := u.Hostname()
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// isAllowedHost checks if the Host header is a localhost variant.
// Returns true for empty host (HTTP/1.0 clients), localhost, 127.0.0.1, and [::1]
// with any port. This prevents DNS rebinding attacks where attacker.com resolves
// to 127.0.0.1 — the browser sends Host: attacker.com which we reject.
func isAllowedHost(host string) bool {
	if host == "" {
		return true
	}

	// Strip port if present. net.SplitHostPort fails for hosts without port,
	// so we check both forms.
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}

	// Strip IPv6 brackets (e.g., "[::1]" → "::1") for bare IPv6 without port
	hostname = strings.TrimPrefix(hostname, "[")
	hostname = strings.TrimSuffix(hostname, "]")

	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// CORS middleware with Host and Origin validation for DNS rebinding protection
// (MCP spec §base/transports H-2/H-3).
//
// Security: Two layers of protection against DNS rebinding:
//  1. Host header validation — rejects requests where Host is not a localhost variant.
//  2. Origin validation — rejects requests from non-local, non-extension origins.
//  3. CORS origin echo — returns the specific allowed origin, never wildcard "*".
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Layer 1: Validate Host header (DNS rebinding protection)
		if !isAllowedHost(r.Host) {
			http.Error(w, "Invalid Host header", http.StatusForbidden)
			return
		}

		// Layer 2: Validate Origin header — if present and invalid, reject with 403
		origin := r.Header.Get("Origin")
		if origin != "" && !isAllowedOrigin(origin) {
			http.Error(w, `{"error":"forbidden: invalid origin"}`, http.StatusForbidden)
			return
		}

		// Layer 3: Echo back the specific allowed origin (never wildcard "*")
		// Only set ACAO when an Origin header is present and valid.
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
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

// findMCPConfig checks for MCP configuration files in common locations
// Returns the path if found, empty string otherwise
func findMCPConfig() string {
	// Claude Code - project-local config
	if _, err := os.Stat(".mcp.json"); err == nil {
		return ".mcp.json"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check common MCP config locations
	locations := []string{
		filepath.Join(home, ".cursor", "mcp.json"),                // Cursor
		filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"), // Windsurf
		filepath.Join(home, ".continue", "config.json"),            // Continue
		filepath.Join(home, ".config", "zed", "settings.json"),     // Zed
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			// Verify it actually contains gasoline config
			// #nosec G304 -- paths are from a fixed list of known MCP config locations, not user input
			data, err := os.ReadFile(path)
			if err == nil && (strings.Contains(string(data), "gasoline") || strings.Contains(string(data), "gasoline-mcp")) {
				return path
			}
		}
	}

	return ""
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
					_, _ = f.Write(data)         // #nosec G104 -- best-effort crash logging
					_, _ = f.Write([]byte{'\n'}) // #nosec G104 -- best-effort crash logging
					_ = f.Close()                // #nosec G104 -- best-effort crash logging
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
	apiKey := flag.String("api-key", "", "API key for HTTP authentication (optional)")
	checkSetup := flag.Bool("check", false, "Verify setup: check if port is available and print status")
	persistMode := flag.Bool("persist", true, "Keep server running after MCP client disconnects (default: true)")
	connectMode := flag.Bool("connect", false, "Connect to existing server (multi-client mode)")
	clientID := flag.String("client-id", "", "Override client ID (default: derived from CWD)")
	bridgeMode := flag.Bool("bridge", false, "Run as stdio-to-HTTP bridge (spawns daemon if needed)")
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

	if *checkSetup {
		runSetupCheck(*port)
		os.Exit(0)
	}

	// Connect mode: forward MCP to existing server
	if *connectMode {
		// Error acceptable: cwd defaults to empty string if inaccessible; DeriveClientID handles this
		cwd, _ := os.Getwd()
		id := *clientID
		if id == "" {
			id = DeriveClientID(cwd)
		}
		runConnectMode(*port, id, cwd)
		return
	}

	// Default log file to home directory
	if *logFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Error getting home directory: %v\n", err)
			os.Exit(1)
		}
		*logFile = filepath.Join(home, "gasoline-logs.jsonl")
	}

	// Create server
	server, err := NewServer(*logFile, *maxEntries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error creating server: %v\n", err)
		os.Exit(1)
	}

	// Always run in MCP mode: HTTP server for browser extension + MCP protocol over stdio
	// Bridge mode: stdio-to-HTTP proxy (spawns daemon if needed)
	if *bridgeMode {
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "bridge_mode_start",
			"pid":       os.Getpid(),
			"port":      *port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		fmt.Fprintf(os.Stderr, "[gasoline] Starting in bridge mode (stdio -> HTTP)\n")
		runBridgeMode(*port)
		return
	}

	// Determine if stdin is TTY (user ran "gasoline" interactively) or piped (launched by MCP host)
	stat, err := os.Stdin.Stat()
	var isTTY bool
	var stdinMode os.FileMode
	if err == nil {
		isTTY = (stat.Mode() & os.ModeCharDevice) != 0
		stdinMode = stat.Mode()
	}

	// Log mode detection for diagnostics
	_ = server.appendToFile([]LogEntry{{
		"type":       "lifecycle",
		"event":      "mode_detection",
		"is_tty":     isTTY,
		"stdin_mode": fmt.Sprintf("%v", stdinMode),
		"pid":        os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}})

	if isTTY {
		// User ran "gasoline" directly - start server as background process (MCP mode)
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "spawn_background",
			"pid":       os.Getpid(),
			"port":      *port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})

		// Pre-flight check: Warn if MCP config exists (manual start will conflict)
		if mcpConfigPath := findMCPConfig(); mcpConfigPath != "" {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: MCP configuration detected at %s\n", mcpConfigPath)
			fmt.Fprintf(os.Stderr, "   Manual start may conflict with MCP server management.\n")
			fmt.Fprintf(os.Stderr, "   Recommended: Let your AI tool spawn gasoline automatically.\n")
			fmt.Fprintf(os.Stderr, "   Continuing anyway...\n\n")
			_ = server.appendToFile([]LogEntry{{
				"type":        "lifecycle",
				"event":       "mcp_config_detected",
				"config_path": mcpConfigPath,
				"pid":         os.Getpid(),
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			}})
		}

		// Pre-flight check: Is port already in use?
		testAddr := fmt.Sprintf("127.0.0.1:%d", *port)
		if ln, err := net.Listen("tcp", testAddr); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Port %d is already in use\n", *port)
			fmt.Fprintf(os.Stderr, "  Fix: kill existing process with: lsof -ti :%d | xargs kill\n", *port)
			fmt.Fprintf(os.Stderr, "  Or use a different port: gasoline --port %d\n", *port+1)
			_ = server.appendToFile([]LogEntry{{
				"type":      "lifecycle",
				"event":     "preflight_failed",
				"error":     "port already in use",
				"port":      *port,
				"pid":       os.Getpid(),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}})
			os.Exit(1)
		} else {
			_ = ln.Close() //nolint:errcheck -- pre-flight check; port will be re-bound by child process
		}

		exe, _ := os.Executable()
		args := []string{"--port", fmt.Sprintf("%d", *port), "--log-file", *logFile, "--max-entries", fmt.Sprintf("%d", *maxEntries)}
		if !*persistMode {
			args = append(args, "--persist=false")
		}
		if *apiKey != "" {
			args = append(args, "--api-key", *apiKey)
		}

		// Spawn background process with piped stdin (so it detects as MCP mode, not TTY)
		cmd := exec.Command(exe, args...) // #nosec G204 -- exe is our own binary path from os.Executable()
		cmd.Stdout = nil
		cmd.Stderr = nil
		// Create a pipe for stdin so child process sees piped input (not TTY)
		_, err := cmd.StdinPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Failed to create stdin pipe: %v\n", err)
			os.Exit(1)
		}
		setDetachedProcess(cmd)
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Failed to spawn background server: %v\n", err)
			_ = server.appendToFile([]LogEntry{{
				"type":      "lifecycle",
				"event":     "spawn_failed",
				"error":     err.Error(),
				"pid":       os.Getpid(),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}})
			os.Exit(1)
		}

		backgroundPID := cmd.Process.Pid
		_ = server.appendToFile([]LogEntry{{
			"type":           "lifecycle",
			"event":          "spawn_success",
			"foreground_pid": os.Getpid(),
			"background_pid": backgroundPID,
			"port":           *port,
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
		}})

		// Wait for server to be ready (health check)
		healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", *port)
		maxAttempts := 30 // 3 seconds total (100ms * 30)
		fmt.Printf("⏳ Starting server (pid %d)...\n", backgroundPID)

		for attempt := 0; attempt < maxAttempts; attempt++ {
			time.Sleep(100 * time.Millisecond)

			// Check if process is still alive
			if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
				fmt.Fprintf(os.Stderr, "✗ Background server (pid %d) died during startup\n", backgroundPID)
				fmt.Fprintf(os.Stderr, "  Check logs: tail -20 %s\n", *logFile)
				_ = server.appendToFile([]LogEntry{{
					"type":      "lifecycle",
					"event":     "startup_failed_process_died",
					"pid":       backgroundPID,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}})
				os.Exit(1)
			}

			// Try health check
			client := &http.Client{Timeout: 200 * time.Millisecond}
			resp, err := client.Get(healthURL)
			if err == nil && resp.StatusCode == 200 {
				_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after health check success
				fmt.Printf("✓ Server ready on http://127.0.0.1:%d\n", *port)
				fmt.Printf("  Log file: %s\n", *logFile)
				fmt.Printf("  Stop with: kill %d\n", backgroundPID)
				_ = server.appendToFile([]LogEntry{{
					"type":      "lifecycle",
					"event":     "startup_verified",
					"pid":       backgroundPID,
					"port":      *port,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}})
				os.Exit(0)
			}
			if resp != nil {
				_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after health check
			}
		}

		// Timeout: server didn't become ready
		fmt.Fprintf(os.Stderr, "✗ Server (pid %d) failed to respond within 3 seconds\n", backgroundPID)
		fmt.Fprintf(os.Stderr, "  The process is still running but not responding to health checks\n")
		fmt.Fprintf(os.Stderr, "  Check logs: tail -20 %s\n", *logFile)
		fmt.Fprintf(os.Stderr, "  Kill it with: kill %d\n", backgroundPID)
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "startup_timeout",
			"pid":       backgroundPID,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		os.Exit(1)
	}

	// stdin is piped → MCP mode (HTTP + MCP protocol)
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "mcp_mode_start",
		"pid":       os.Getpid(),
		"port":      *port,
		"persist":   *persistMode,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	fmt.Fprintf(os.Stderr, "[gasoline] Starting in MCP mode (HTTP + MCP protocol, persist=%v)\n", *persistMode)
	runMCPMode(server, *port, *apiKey, *persistMode)
}

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
// If stdin closes (EOF), the HTTP server keeps running until killed.
func runMCPMode(server *Server, port int, apiKey string, persist bool) {
	// Create capture buffers for WebSocket, network, and actions
	capture := NewCapture()

	// Start async command result cleanup goroutine (60s TTL)
	stopResultCleanup := capture.startResultCleanup()
	defer stopResultCleanup()

	// Start consolidated pending query cleanup goroutine (5s interval)
	stopQueryCleanup := capture.startQueryCleanup()
	defer stopQueryCleanup()

	// Load cached settings from disk (pilot state, etc.)
	// See docs/plugin-server-communications.md for protocol details
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "loading_settings",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	capture.LoadSettingsFromDisk()

	// Log settings load result
	capture.mu.RLock()
	settingsLoaded := !capture.pilotUpdatedAt.IsZero()
	pilotEnabled := capture.pilotEnabled
	settingsAge := time.Since(capture.pilotUpdatedAt).Seconds()
	capture.mu.RUnlock()

	_ = server.appendToFile([]LogEntry{{
		"type":            "lifecycle",
		"event":           "settings_loaded",
		"pid":             os.Getpid(),
		"loaded":          settingsLoaded,
		"pilot_enabled":   pilotEnabled,
		"settings_age_s":  settingsAge,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}})

	// Create SSE registry for MCP connections
	sseRegistry := NewSSERegistry()

	// Register HTTP routes before starting the goroutine.
	// This avoids fragile ordering where route registration in a goroutine
	// relies on implicit happens-before guarantees from the channel.
	setupHTTPRoutes(server, capture, sseRegistry)

	// Start HTTP server in background for browser extension
	httpReady := make(chan error, 1)
	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			httpReady <- err
			return
		}
		httpReady <- nil
		srv := &http.Server{
			ReadTimeout:  2 * time.Second,  // Force fast request reads
			WriteTimeout: 2 * time.Second,  // Force responses within 2s (async command pattern)
			IdleTimeout:  120 * time.Second, // Keep-alive for polling connections
			Handler:      AuthMiddleware(apiKey)(http.DefaultServeMux),
		}
		// #nosec G114 -- localhost-only MCP background server
		if err := srv.Serve(ln); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] HTTP server error: %v\n", err)
		}
	}()

	// Wait for HTTP server to bind before proceeding
	if err := <-httpReady; err != nil {
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "http_bind_failed",
			"pid":       os.Getpid(),
			"port":      port,
			"error":     err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		fmt.Fprintf(os.Stderr, "[gasoline] Fatal: cannot bind port %d: %v\n", port, err)
		fmt.Fprintf(os.Stderr, "[gasoline] Fix: kill existing process with: lsof -ti :%d | xargs kill\n", port)
		fmt.Fprintf(os.Stderr, "[gasoline] Or use a different port: --port %d\n", port+1)
		os.Exit(1)
	}

	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "http_bind_success",
		"pid":       os.Getpid(),
		"port":      port,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	fmt.Fprintf(os.Stderr, "[gasoline] v%s — HTTP on port %d, log: %s\n", version, port, server.logFile)
	fmt.Fprintf(os.Stderr, "[gasoline] Verify: curl http://localhost:%d/health\n", port)

	// Show first-run help if log file is new/empty
	if fi, err := os.Stat(server.logFile); err != nil || fi.Size() == 0 {
		fmt.Fprintf(os.Stderr, "[gasoline] ─────────────────────────────────────────────────\n")
		fmt.Fprintf(os.Stderr, "[gasoline] First run? Next steps:\n")
		fmt.Fprintf(os.Stderr, "[gasoline]   1. Install extension: chrome://extensions → Load unpacked → extension/\n")
		fmt.Fprintf(os.Stderr, "[gasoline]   2. Open any website in Chrome\n")
		fmt.Fprintf(os.Stderr, "[gasoline]   3. Extension popup should show 'Connected'\n")
		fmt.Fprintf(os.Stderr, "[gasoline] ─────────────────────────────────────────────────\n")
	}

	_ = server.appendToFile([]LogEntry{{"type": "lifecycle", "event": "startup", "version": version, "port": port, "timestamp": time.Now().UTC().Format(time.RFC3339)}})

	// MCP SSE transport ready
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "mcp_sse_ready",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	fmt.Fprintf(os.Stderr, "[gasoline] MCP SSE handler ready at /mcp/sse\n")

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	s := <-sig
	fmt.Fprintf(os.Stderr, "[gasoline] Received %s, shutting down\n", s)
	_ = server.appendToFile([]LogEntry{{"type": "lifecycle", "event": "shutdown", "reason": s.String(), "timestamp": time.Now().UTC().Format(time.RFC3339)}})
	fmt.Fprintf(os.Stderr, "[gasoline] Shutdown complete\n")
}

// runConnectMode connects to an existing Gasoline server as an MCP client.
// This enables multiple Claude Code sessions to share a single server.
// The client ID is sent via X-Gasoline-Client header for state isolation.
func runConnectMode(port int, clientID string, cwd string) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Check if server is running
	healthURL := serverURL + "/health"
	resp, err := http.Get(healthURL) // #nosec G107 -- localhost URL constructed from port flag
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Cannot connect to server at %s: %v\n", serverURL, err)
		fmt.Fprintf(os.Stderr, "[gasoline] Start a server first: gasoline --server --port %d\n", port)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "[gasoline] Server health check failed: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	// Register this client with the server
	registerURL := serverURL + "/clients"
	regBody, _ := json.Marshal(map[string]string{"cwd": cwd})
	// Error unlikely: URL is constructed from port flag, method and header are literals
	req, _ := http.NewRequest("POST", registerURL, strings.NewReader(string(regBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gasoline-Client", clientID)
	regResp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Server might not have /clients endpoint yet (backwards compat)
		fmt.Fprintf(os.Stderr, "[gasoline] Warning: could not register client: %v\n", err)
	} else {
		_ = regResp.Body.Close() //nolint:errcheck -- best-effort cleanup after client registration
	}

	fmt.Fprintf(os.Stderr, "[gasoline] Connected to %s (client: %s)\n", serverURL, clientID)

	// Run MCP protocol over stdin/stdout, forwarding to HTTP server
	scanner := bufio.NewScanner(os.Stdin)
	const maxScanTokenSize = 10 * 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	mcpURL := serverURL + "/mcp"

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Forward request to server with client ID header
		req, err := http.NewRequest("POST", mcpURL, strings.NewReader(line))
		if err != nil {
			sendMCPError(nil, -32603, "Internal error: "+err.Error())
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gasoline-Client", clientID)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			// Try to extract request ID for error response
			var jsonReq JSONRPCRequest
			if json.Unmarshal([]byte(line), &jsonReq) == nil {
				sendMCPError(jsonReq.ID, -32603, "Server connection error: "+err.Error())
			} else {
				sendMCPError(nil, -32603, "Server connection error: "+err.Error())
			}
			continue
		}

		// Stream response back to stdout
		var respData json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after decode error
			var jsonReq JSONRPCRequest
			if json.Unmarshal([]byte(line), &jsonReq) == nil {
				sendMCPError(jsonReq.ID, -32603, "Invalid server response")
			}
			continue
		}
		_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after successful decode

		fmt.Println(string(respData))
	}

	// stdin closed - unregister and exit
	if clientID != "" {
		unregURL := serverURL + "/clients/" + clientID
		// Error unlikely: URL is constructed from port flag and clientID, method is literal
		req, _ := http.NewRequest("DELETE", unregURL, nil)
		req.Header.Set("X-Gasoline-Client", clientID)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after unregister
		}
	}

	fmt.Fprintf(os.Stderr, "[gasoline] Disconnected from %s\n", serverURL)
}

// sendMCPError sends a JSON-RPC error response to stdout (used in connect mode)
func sendMCPError(id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	respJSON, _ := json.Marshal(resp)
	fmt.Println(string(respJSON))
}

// setupHTTPRoutes configures the HTTP routes (extracted for reuse)
func setupHTTPRoutes(server *Server, capture *Capture, sseRegistry *SSERegistry) {
	// V4 routes
	if capture != nil {
		http.HandleFunc("/websocket-events", corsMiddleware(capture.HandleWebSocketEvents))
		http.HandleFunc("/websocket-status", corsMiddleware(capture.HandleWebSocketStatus))
		http.HandleFunc("/network-bodies", corsMiddleware(capture.HandleNetworkBodies))
		http.HandleFunc("/network-waterfall", corsMiddleware(capture.HandleNetworkWaterfall))
		http.HandleFunc("/extension-logs", corsMiddleware(capture.HandleExtensionLogs))
		http.HandleFunc("/pending-queries", corsMiddleware(capture.HandlePendingQueries))
		http.HandleFunc("/pilot-status", corsMiddleware(capture.HandlePilotStatus))
		http.HandleFunc("/dom-result", corsMiddleware(capture.HandleDOMResult))
		http.HandleFunc("/a11y-result", corsMiddleware(capture.HandleA11yResult))
		http.HandleFunc("/state-result", corsMiddleware(capture.HandleStateResult))
		http.HandleFunc("/execute-result", corsMiddleware(capture.HandleExecuteResult))
		http.HandleFunc("/highlight-result", corsMiddleware(capture.HandleHighlightResult))
		http.HandleFunc("/enhanced-actions", corsMiddleware(capture.HandleEnhancedActions))
		http.HandleFunc("/performance-snapshots", corsMiddleware(capture.HandlePerformanceSnapshots))
		http.HandleFunc("/api/extension-status", corsMiddleware(capture.HandleExtensionStatus))

		// Multi-client management endpoints
		http.HandleFunc("/clients", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				// List all registered clients
				clients := capture.clientRegistry.List()
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"clients": clients,
					"count":   len(clients),
				})
			case "POST":
				// Register a new client
				var body struct {
					CWD string `json:"cwd"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
					return
				}
				cs := capture.clientRegistry.Register(body.CWD)
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"id":         cs.ID,
					"cwd":        cs.CWD,
					"created_at": cs.CreatedAt.Format(time.RFC3339),
				})
			default:
				jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			}
		}))

		// Client-specific endpoint with ID in path
		http.HandleFunc("/clients/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
			// Extract client ID from path: /clients/{id}
			clientID := strings.TrimPrefix(r.URL.Path, "/clients/")
			if clientID == "" {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing client ID"})
				return
			}

			switch r.Method {
			case "GET":
				// Get specific client
				cs := capture.clientRegistry.Get(clientID)
				if cs == nil {
					jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
					return
				}
				cs.mu.RLock()
				info := ClientInfo{
					ID:         cs.ID,
					CWD:        cs.CWD,
					CreatedAt:  cs.CreatedAt.Format(time.RFC3339),
					LastSeenAt: cs.LastSeenAt.Format(time.RFC3339),
					IdleFor:    time.Since(cs.LastSeenAt).Round(time.Second).String(),
				}
				cs.mu.RUnlock()
				jsonResponse(w, http.StatusOK, info)
			case "DELETE":
				// Unregister client
				capture.clientRegistry.Unregister(clientID)
				jsonResponse(w, http.StatusOK, map[string]bool{"unregistered": true})
			default:
				jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			}
		}))

		// CI Infrastructure endpoints
		http.HandleFunc("/snapshot", corsMiddleware(handleSnapshot(server, capture)))
		http.HandleFunc("/clear", corsMiddleware(handleClear(server, capture)))
		http.HandleFunc("/test-boundary", corsMiddleware(handleTestBoundary(capture)))
	}

	// MCP over HTTP endpoint (for browser extension backward compatibility)
	mcp := NewToolHandler(server, capture, sseRegistry)
	http.HandleFunc("/mcp", corsMiddleware(mcp.HandleHTTP))

	// MCP SSE transport endpoints
	http.HandleFunc("/mcp/sse", corsMiddleware(handleMCPSSE(sseRegistry, server)))
	http.HandleFunc("/mcp/messages/", corsMiddleware(handleMCPMessages(sseRegistry, mcp)))

	// CI/CD webhook endpoint for push-based alerts
	if mcp.toolHandler != nil {
		http.HandleFunc("/ci-result", corsMiddleware(mcp.toolHandler.handleCIWebhook))
	}

	// Settings endpoint for extension settings synchronization
	// Implements POST /settings protocol documented in docs/plugin-server-communications.md
	http.HandleFunc("/settings", corsMiddleware(capture.HandleSettings))

	http.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		logFileSize := int64(0)
		if fi, err := os.Stat(server.logFile); err == nil {
			logFileSize = fi.Size()
		}

		resp := map[string]interface{}{
			"status":  "ok",
			"version": version,
			"logs": map[string]interface{}{
				"entries":     server.getEntryCount(),
				"maxEntries":  server.maxEntries,
				"logFile":     server.logFile,
				"logFileSize": logFileSize,
			},
		}

		if capture != nil {
			// Single lock acquisition for all capture state
			capture.mu.RLock()
			wsEventCount := len(capture.wsEvents)
			nbCount := len(capture.networkBodies)
			actionCount := len(capture.enhancedActions)
			connCount := len(capture.connections)
			lastPoll := capture.lastPollAt
			extSession := capture.extensionSession
			sessionChangedAt := capture.sessionChangedAt
			pilotEnabled := capture.pilotEnabled
			circuitOpen := capture.circuitOpen
			currentRate := capture.windowEventCount
			memoryBytes := capture.getMemoryForCircuit()
			circuitReason := capture.circuitReason
			var circuitOpenedAt string
			if circuitOpen {
				circuitOpenedAt = capture.circuitOpenedAt.Format(time.RFC3339)
			}
			capture.mu.RUnlock()

			resp["buffers"] = map[string]interface{}{
				"websocket_events": wsEventCount,
				"network_bodies":   nbCount,
				"actions":          actionCount,
				"connections":      connCount,
			}

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
				"open":         circuitOpen,
				"current_rate": currentRate,
				"memory_bytes": memoryBytes,
				"reason":       circuitReason,
				"opened_at":    circuitOpenedAt,
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
			// Single lock acquisition for all capture state
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
			pilotEnabled := capture.pilotEnabled
			diagCircuitOpen := capture.circuitOpen
			diagCurrentRate := capture.windowEventCount
			diagMemoryBytes := capture.getMemoryForCircuit()
			diagCircuitReason := capture.circuitReason
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
				"open":         diagCircuitOpen,
				"current_rate": diagCurrentRate,
				"memory_bytes": diagMemoryBytes,
				"reason":       diagCircuitReason,
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

		// HTTP debug log (last 50 requests/responses)
		if capture != nil {
			httpDebugLog := capture.GetHTTPDebugLog()
			resp["http_debug_log"] = map[string]interface{}{
				"count":   len(httpDebugLog),
				"entries": httpDebugLog,
			}
		}

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
  --persist              Keep server running after MCP client disconnects (default: true)
  --persist=false        Exit after MCP client disconnects
  --api-key <key>        Require API key for HTTP requests (optional)
  --connect              Connect to existing server (multi-client mode)
  --client-id <id>       Override client ID (default: derived from CWD)
  --check                Verify setup (check port availability, print status)
  --version              Show version
  --help                 Show this help message

Gasoline always runs in MCP mode: the HTTP server starts in the background
(for the browser extension) and MCP protocol runs over stdio (for Claude Code, Cursor, etc.).
The server persists by default, even after the MCP client disconnects.

Examples:
  gasoline                              # MCP mode (default, persist on)
  gasoline --persist=false              # Exit when MCP client disconnects
  gasoline --api-key s3cret             # MCP mode with API key auth
  gasoline --connect --port 7890        # Connect to existing server
  gasoline --check                      # Verify setup before running
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

// runSetupCheck verifies the setup and prints diagnostic information
func runSetupCheck(port int) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  GASOLINE SETUP CHECK                                          ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Port:    %d\n", port)
	fmt.Println()

	// Check 1: Port availability
	fmt.Print("Checking port availability... ")
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Port %d is already in use.\n", port)
		fmt.Printf("  Fix: lsof -ti :%d | xargs kill\n", port)
		fmt.Printf("  Or use a different port: --port %d\n", port+1)
		fmt.Println()
	} else {
		_ = ln.Close() //nolint:errcheck -- pre-flight check; port availability test only
		fmt.Println("OK")
		fmt.Printf("  Port %d is available.\n", port)
		fmt.Println()
	}

	// Check 2: Log file directory
	fmt.Print("Checking log file directory... ")
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Cannot determine home directory: %v\n", err)
		fmt.Println()
	} else {
		logFile := filepath.Join(home, "gasoline-logs.jsonl")
		fmt.Println("OK")
		fmt.Printf("  Log file: %s\n", logFile)
		fmt.Println()
	}

	// Summary
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start server:    npx gasoline-mcp")
	fmt.Println("  2. Install extension:")
	fmt.Println("     - Open chrome://extensions")
	fmt.Println("     - Enable Developer mode")
	fmt.Println("     - Click 'Load unpacked' → select extension/ folder")
	fmt.Println("  3. Open any website")
	fmt.Println("  4. Extension popup should show 'Connected'")
	fmt.Println()
	fmt.Printf("Verify:  curl http://localhost:%d/health\n", port)
	fmt.Println()
}
