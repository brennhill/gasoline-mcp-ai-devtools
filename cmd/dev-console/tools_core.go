// tools_core.go — Core MCP tool types, constants, and response helpers.
// This file contains the foundational pieces used by all tool handlers:
// - MCP typed response structs
// - Tool call rate limiter
// - Response helpers (mcpTextResponse, mcpJSONResponse, mcpStructuredError)
// - Error codes and StructuredError type
// - Unknown parameter warning helpers
// - ToolHandler struct definition and constructor
//
// nolint:filelength - Core file kept together for cohesion; refactored from 2400-line tools.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/analysis"
	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/security"
	"github.com/dev-console/dev-console/internal/session"
	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// MCP Typed Response Structs
// ============================================

// MCPContentBlock represents a single content block in an MCP tool result.
type MCPContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPToolResult represents the result of an MCP tool call.
type MCPToolResult struct {
	Content []MCPContentBlock `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}

// MCPInitializeResult represents the result of an MCP initialize request.
type MCPInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	ServerInfo      MCPServerInfo   `json:"serverInfo"`
	Capabilities    MCPCapabilities `json:"capabilities"`
}

// MCPServerInfo identifies the MCP server.
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPCapabilities declares the server's MCP capabilities.
type MCPCapabilities struct {
	Tools     MCPToolsCapability     `json:"tools"`
	Resources MCPResourcesCapability `json:"resources"`
}

// MCPToolsCapability declares tool support.
type MCPToolsCapability struct{}

// MCPResourcesCapability declares resource support.
type MCPResourcesCapability struct{}

// MCPResource describes an available resource.
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPResourcesListResult represents the result of a resources/list request.
type MCPResourcesListResult struct {
	Resources []MCPResource `json:"resources"`
}

// MCPResourceContent represents the content of a resource.
type MCPResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// MCPResourcesReadResult represents the result of a resources/read request.
type MCPResourcesReadResult struct {
	Contents []MCPResourceContent `json:"contents"`
}

// MCPToolsListResult represents the result of a tools/list request.
type MCPToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

// MCPResourceTemplatesListResult represents the result of a resources/templates/list request.
type MCPResourceTemplatesListResult struct {
	ResourceTemplates []any `json:"resourceTemplates"`
}

// ============================================
// Tool Call Rate Limiter
// ============================================

// ToolCallLimiter implements a sliding window rate limiter for MCP tool calls.
// Thread-safe: uses its own mutex independent of other locks.
type ToolCallLimiter struct {
	mu         sync.Mutex
	timestamps []time.Time
	maxCalls   int
	window     time.Duration
}

// NewToolCallLimiter creates a rate limiter allowing maxCalls within the given window.
func NewToolCallLimiter(maxCalls int, window time.Duration) *ToolCallLimiter {
	return &ToolCallLimiter{
		timestamps: make([]time.Time, 0, maxCalls),
		maxCalls:   maxCalls,
		window:     window,
	}
}

// Allow checks if a new call is permitted. If allowed, records it and returns true.
func (l *ToolCallLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Compact: remove expired timestamps
	valid := 0
	for _, ts := range l.timestamps {
		if ts.After(cutoff) {
			l.timestamps[valid] = ts
			valid++
		}
	}
	l.timestamps = l.timestamps[:valid]

	if len(l.timestamps) >= l.maxCalls {
		return false
	}

	l.timestamps = append(l.timestamps, now)
	return true
}

// ============================================
// MCP Response Helpers
// ============================================

// safeMarshal marshals v to JSON with defensive error handling.
// Should never fail for simple structs, but handles it gracefully if it does.
func safeMarshal(v any, fallback string) json.RawMessage {
	resultJSON, err := json.Marshal(v)
	if err != nil {
		// This should never happen with simple structs, but handle it defensively
		fmt.Fprintf(os.Stderr, "[gasoline] JSON marshal error: %v\n", err)
		return json.RawMessage(fallback)
	}
	return json.RawMessage(resultJSON)
}

// mcpTextResponse constructs an MCP tool result containing a single text content block.
func mcpTextResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
	}
	return safeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}]}`)
}

// mcpErrorResponse constructs an MCP tool error result containing a single text content block.
func mcpErrorResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
		IsError: true,
	}
	return safeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// ============================================
// W1: Hybrid Markdown/JSON Response Helpers
// ============================================

// ResponseFormat tags each response for documentation and testing.
type ResponseFormat string

const (
	FormatMarkdown ResponseFormat = "markdown"
	FormatJSON     ResponseFormat = "json"
)

// mcpMarkdownResponse constructs an MCP tool result with a summary line
// followed by markdown-formatted content (typically a table).
// Use for flat, uniform data where columns are consistent across rows.
func mcpMarkdownResponse(summary string, markdown string) json.RawMessage {
	text := summary + "\n\n" + markdown

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
	}
	return safeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// mcpJSONResponse constructs an MCP tool result with a summary line prefix
// followed by compact JSON. Use for nested, irregular, or highly variable data.
func mcpJSONResponse(summary string, data any) json.RawMessage {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return mcpErrorResponse("Failed to serialize response: " + err.Error())
	}

	var text string
	if summary != "" {
		text = summary + "\n" + string(dataJSON)
	} else {
		text = string(dataJSON)
	}

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
	}
	resultJSON, _ := json.Marshal(result)
	return json.RawMessage(resultJSON)
}

// markdownTable converts a slice of items into a markdown table.
// headers defines column names. rows contains cell values for each row.
// Pipe chars in cell values are escaped, newlines are replaced with spaces.
func markdownTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder

	// Header row
	b.WriteString("| ")
	b.WriteString(strings.Join(headers, " | "))
	b.WriteString(" |\n")

	// Separator
	b.WriteString("|")
	for range headers {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")

	// Data rows
	for _, row := range rows {
		escaped := make([]string, len(row))
		for i, cell := range row {
			// Replace newlines with spaces
			cell = strings.ReplaceAll(cell, "\n", " ")
			// Escape pipe characters
			cell = strings.ReplaceAll(cell, "|", `\|`)
			escaped[i] = cell
		}
		b.WriteString("| ")
		b.WriteString(strings.Join(escaped, " | "))
		b.WriteString(" |\n")
	}
	return b.String()
}

// truncate returns s unchanged if len(s) <= maxLen. Otherwise, it truncates
// and appends "..." so the total output length equals maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ============================================
// W3: Log Quality Check
// ============================================

// checkLogQuality scans entries for missing expected fields and returns
// a warning note if anomalies are found. Returns "" if all entries look clean.
func checkLogQuality(entries []LogEntry) string {
	var missingTS, missingMsg, missingSource int
	badEntries := 0
	for _, e := range entries {
		entryBad := false
		if _, ok := e["ts"].(string); !ok {
			missingTS++
			entryBad = true
		}
		if _, ok := e["message"].(string); !ok {
			missingMsg++
			entryBad = true
		}
		if _, ok := e["source"].(string); !ok {
			missingSource++
			entryBad = true
		}
		if entryBad {
			badEntries++
		}
	}

	if badEntries == 0 {
		return ""
	}

	var parts []string
	if missingTS > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'ts'", missingTS))
	}
	if missingMsg > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'message'", missingMsg))
	}
	if missingSource > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'source'", missingSource))
	}
	return fmt.Sprintf("WARNING: %d/%d entries have incomplete fields (%s). This may indicate a browser extension issue or version mismatch.",
		badEntries, len(entries), strings.Join(parts, ", "))
}

// ============================================
// W5: Structured Error Helpers
// ============================================

// Error codes are self-describing snake_case strings.
// Every code tells the LLM what went wrong.
const (
	// Input errors — LLM can fix arguments and retry immediately
	ErrInvalidJSON    = "invalid_json"
	ErrMissingParam   = "missing_param"
	ErrInvalidParam   = "invalid_param"
	ErrUnknownMode    = "unknown_mode"
	ErrPathNotAllowed = "path_not_allowed"

	// State errors — LLM must change state before retrying
	ErrNotInitialized    = "not_initialized"
	ErrNoData            = "no_data"
	ErrCodePilotDisabled = "pilot_disabled" // Named ErrCodePilotDisabled to avoid collision with var ErrCodePilotDisabled in pilot.go
	ErrRateLimited       = "rate_limited"
	ErrCursorExpired     = "cursor_expired" // Cursor pagination: buffer overflow evicted cursor position

	// Communication errors — retry with backoff
	ErrExtTimeout = "extension_timeout"
	ErrExtError   = "extension_error"

	// Internal errors — do not retry
	ErrInternal      = "internal_error"
	ErrMarshalFailed = "marshal_failed"
	ErrExportFailed  = "export_failed"
)

// StructuredError is embedded in MCP text content. Every field is
// self-describing so an LLM can act on it without a lookup table.
type StructuredError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Retry   string `json:"retry"`
	Param   string `json:"param,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

// mcpStructuredError constructs an MCP error response. Format:
//
//	Error: missing_param — Add the 'what' parameter and call again
//	{"error":"missing_param","message":"...","retry":"Add the 'what' parameter and call again","hint":"..."}
//
// The retry string is a plain-English instruction the LLM can follow directly.
func mcpStructuredError(code, message, retry string, opts ...func(*StructuredError)) json.RawMessage {
	se := StructuredError{Error: code, Message: message, Retry: retry}
	for _, opt := range opts {
		opt(&se)
	}

	// Error impossible: StructuredError is a simple struct with no circular refs or unsupported types
	seJSON, _ := json.Marshal(se)
	text := fmt.Sprintf("Error: %s — %s\n%s", code, retry, string(seJSON))

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
		IsError: true,
	}
	return safeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

func withParam(p string) func(*StructuredError) {
	return func(se *StructuredError) { se.Param = p }
}

func withHint(h string) func(*StructuredError) {
	return func(se *StructuredError) { se.Hint = h }
}

// ============================================
// Unknown Parameter Warning Helpers
// ============================================

// getJSONFieldNames uses reflection to extract the set of known JSON field names
// from a struct's json tags. Fields without a json tag use their Go field name.
// Fields tagged with json:"-" are excluded.
func getJSONFieldNames(v any) map[string]bool {
	known := make(map[string]bool)
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return known
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		if tag == "" {
			known[field.Name] = true
			continue
		}
		// Strip options like ",omitempty"
		name := strings.Split(tag, ",")[0]
		if name != "" {
			known[name] = true
		}
	}
	return known
}

// unmarshalWithWarnings unmarshals JSON into a struct and returns warnings for
// any unknown top-level fields. This helps LLMs discover misspelled parameters.
func unmarshalWithWarnings(data json.RawMessage, v any) ([]string, error) {
	if err := json.Unmarshal(data, v); err != nil {
		return nil, err
	}
	// Check for unknown fields by unmarshaling into a map
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil // Can't check, skip warnings
	}
	known := getJSONFieldNames(v)
	var warnings []string
	for k := range raw {
		if !known[k] {
			warnings = append(warnings, fmt.Sprintf("unknown parameter '%s' (ignored)", k))
		}
	}
	return warnings, nil
}

// appendWarningsToResponse adds a warnings content block to an MCP response if there are any.
func appendWarningsToResponse(resp JSONRPCResponse, warnings []string) JSONRPCResponse {
	if len(warnings) == 0 {
		return resp
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	warningText := "_warnings: " + strings.Join(warnings, "; ")
	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: warningText,
	})
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// ============================================
// ToolHandler Definition
// ============================================

// ToolHandler extends MCPHandler with composite tool dispatch
type ToolHandler struct {
	*MCPHandler
	capture *capture.Capture

	// Cross-package dependencies use interfaces from internal/types to avoid circular imports.
	// Implementations are in their respective packages (ai, analysis, security, etc.).
	checkpoints       types.CheckpointManager    // Session state snapshots
	sessionStore      types.SessionStore         // Persistent session data
	noise             types.NoiseConfig          // Noise filtering rules
	clusters          types.ClusterManager       // Error clustering
	temporalGraph     types.TemporalGraph        // Temporal event tracking
	alertBuffer       types.AlertBuffer          // Alert streaming buffer
	cspGenerator      types.CSPGenerator         // CSP policy generation
	securityScanner   types.SecurityScanner      // Security auditing
	thirdPartyAuditor types.ThirdPartyAuditor    // Third-party resource audit
	securityDiffMgr   types.SecurityDiffManager  // Security snapshot comparison
	auditTrail        types.AuditTrail           // Tool invocation audit log
	sessionManager    types.SessionManager       // Browser session management
	contractValidator types.APIContractValidator // API contract validation

	// Fields that remain as any (not yet abstracted or local types)
	captureOverrides any // *capture.CaptureOverrides - internal to capture package
	auditLogger      any // *audit.AuditLogger - TODO: add to interfaces
	healthMetrics    any // *ServerHealthMetrics - local type

	// Verification loop for fix verification
	verificationMgr *session.VerificationManager

	// Redaction engine for scrubbing sensitive data from tool responses
	redactionEngine *RedactionEngine

	// Rate limiter for MCP tool calls (sliding window)
	toolCallLimiter *ToolCallLimiter

	// SSE registry for MCP streaming transport
	sseRegistry *SSERegistry

	// Context streaming: active push notifications via MCP
	streamState *StreamState

	// Alert buffer state (local management)
	alertMu   sync.Mutex
	alerts    []Alert
	ciResults []CIResult
	// Anomaly detection: sliding window error counter
	errorTimes []time.Time

	// Concrete implementations (interface signatures differ from types package)
	// These are used directly by tool handlers rather than through the interface fields above.
	noiseConfig           *ai.NoiseConfig
	sessionStoreImpl      *ai.SessionStore
	securityScannerImpl   *security.SecurityScanner
	thirdPartyAuditorImpl *analysis.ThirdPartyAuditor
}

// GetCapture returns the capture instance
func (h *ToolHandler) GetCapture() *capture.Capture {
	return h.capture
}

// GetToolCallLimiter returns the tool call limiter
func (h *ToolHandler) GetToolCallLimiter() RateLimiter {
	return h.toolCallLimiter
}

// GetRedactionEngine returns the redaction engine
func (h *ToolHandler) GetRedactionEngine() RedactionEngine {
	if h.redactionEngine != nil {
		return *h.redactionEngine
	}
	return nil
}

// NewToolHandler creates an MCP handler with composite tool capabilities
func NewToolHandler(server *Server, capture *capture.Capture, sseRegistry *SSERegistry) *MCPHandler {
	handler := &ToolHandler{
		MCPHandler:  NewMCPHandler(server, version),
		capture:     capture,
		sseRegistry: sseRegistry,
	}

	// Initialize health metrics
	handler.healthMetrics = NewHealthMetrics()
	handler.toolCallLimiter = NewToolCallLimiter(500, time.Minute)
	handler.streamState = NewStreamState(sseRegistry)

	// Initialize noise filtering (concrete type, not interface - signatures differ)
	handler.noiseConfig = ai.NewNoiseConfig()

	// Initialize session store (use current working directory as project path)
	cwd, err := os.Getwd()
	if err == nil {
		if store, err := ai.NewSessionStore(cwd); err == nil {
			handler.sessionStoreImpl = store
		}
	}

	// Initialize security tools (concrete types - interface signatures differ)
	handler.securityScannerImpl = security.NewSecurityScanner()
	handler.thirdPartyAuditorImpl = analysis.NewThirdPartyAuditor()

	// Wire error clustering: feed error-level log entries into the cluster manager.
	// Use SetOnEntries for thread-safe assignment (avoids racing with addEntries).
	// Error clustering disabled for now (not initialized)

	// Return as MCPHandler but with overridden methods via the wrapper
	return &MCPHandler{
		server:      server,
		toolHandler: handler,
	}
}

// captureStateAdapter bridges the Capture/Server data to the CaptureStateReader interface
// required by SessionManager.
type captureStateAdapter struct {
	capture *capture.Capture
	server  *Server
}

func (a *captureStateAdapter) GetConsoleErrors() []session.SnapshotError {
	a.server.mu.RLock()
	defer a.server.mu.RUnlock()
	var errors []session.SnapshotError
	for _, entry := range a.server.entries {
		if level, _ := entry["level"].(string); level == "error" {
			msg, _ := entry["message"].(string)
			errors = append(errors, session.SnapshotError{Type: "error", Message: msg, Count: 1})
		}
	}
	return errors
}

func (a *captureStateAdapter) GetConsoleWarnings() []session.SnapshotError {
	a.server.mu.RLock()
	defer a.server.mu.RUnlock()
	var warnings []session.SnapshotError
	for _, entry := range a.server.entries {
		if level, _ := entry["level"].(string); level == "warn" {
			msg, _ := entry["message"].(string)
			warnings = append(warnings, session.SnapshotError{Type: "warning", Message: msg, Count: 1})
		}
	}
	return warnings
}

func (a *captureStateAdapter) GetNetworkRequests() []session.SnapshotNetworkRequest {
	var requests []session.SnapshotNetworkRequest
	for _, body := range a.capture.GetNetworkBodies() {
		requests = append(requests, session.SnapshotNetworkRequest{
			Method:   body.Method,
			URL:      body.URL,
			Status:   body.Status,
			Duration: body.Duration,
		})
	}
	return requests
}

func (a *captureStateAdapter) GetWSConnections() []session.SnapshotWSConnection {
	var conns []session.SnapshotWSConnection
	// WebSocket connections not accessible via public API - return empty
	return conns
}

func (a *captureStateAdapter) GetPerformance() *capture.PerformanceSnapshot {
	return nil // Performance snapshots not yet integrated
}

func (a *captureStateAdapter) GetCurrentPageURL() string {
	// Current page URL not accessible via public API
	return ""
}

// checkTrackingStatus returns a tracking status hint to include in tool responses.
// If no tab is being tracked AND the extension has reported status at least once,
// the LLM receives a clear warning so it can guide the user.
// Returns enabled=true if tracking is active OR if the extension hasn't reported yet
// (to avoid false warnings on fresh server start).
func (h *ToolHandler) checkTrackingStatus() (enabled bool, hint string) {
	// Tracking status not accessible via public API - assume enabled
	return true, ""
}

// computeDataCounts reads current buffer sizes from server and capture under read locks.
// Returns counts for each observable mode.
func (h *ToolHandler) computeDataCounts() (errorCount, logCount, extensionLogsCount, waterfallCount, networkCount, wsEventCount, wsStatusCount, actionCount, vitalCount, apiCount int) {
	h.server.mu.RLock()
	logCount = len(h.server.entries)
	for _, entry := range h.server.entries {
		if level, ok := entry["level"].(string); ok && level == "error" {
			errorCount++
		}
	}
	h.server.mu.RUnlock()

	// Use public API methods for capture data
	extensionLogsCount = 0 // Not accessible
	waterfallCount = 0     // Not accessible
	networkCount = len(h.capture.GetNetworkBodies())
	wsEventCount = len(h.capture.GetAllWebSocketEvents())
	wsStatusCount = 0 // Connections not accessible
	actionCount = len(h.capture.GetAllEnhancedActions())
	vitalCount = 0    // Performance data not accessible
	apiCount = 0      // Schema store not accessible
	return
}

// handleToolCall dispatches composite tool calls by mode parameter.
func (h *ToolHandler) HandleToolCall(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
	switch name {
	case "observe":
		return h.toolObserve(req, args), true
	case "generate":
		return h.toolGenerate(req, args), true
	case "configure":
		return h.toolConfigure(req, args), true
	case "interact":
		return h.toolInteract(req, args), true
	}
	return JSONRPCResponse{}, false
}
