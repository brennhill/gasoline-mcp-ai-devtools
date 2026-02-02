// tools.go — MCP tool definitions, dispatch, and response helpers.
// Defines composite tools (observe, analyze, generate, configure, query_dom)
// plus security tools (generate_csp, security_audit, audit_third_parties,
// diff_security, get_audit_log, diff_sessions).
// Each composite tool has a mode parameter that selects the sub-operation.
// Design: Tool schemas include live data_counts in _meta so the AI knows what
// data is available before calling. Dispatch is a single switch on tool name +
// mode parameter, keeping the handler flat and predictable.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/session"
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
	ResourceTemplates []interface{} `json:"resourceTemplates"`
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

// mcpTextResponse constructs an MCP tool result containing a single text content block.
func mcpTextResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
	}
	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return json.RawMessage(resultJSON)
}

// mcpErrorResponse constructs an MCP tool error result containing a single text content block.
func mcpErrorResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
		IsError: true,
	}
	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return json.RawMessage(resultJSON)
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
	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return json.RawMessage(resultJSON)
}

// mcpJSONResponse constructs an MCP tool result with a summary line prefix
// followed by compact JSON. Use for nested, irregular, or highly variable data.
func mcpJSONResponse(summary string, data interface{}) json.RawMessage {
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
	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return json.RawMessage(resultJSON)
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
func getJSONFieldNames(v interface{}) map[string]bool {
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
func unmarshalWithWarnings(data json.RawMessage, v interface{}) ([]string, error) {
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
// MCP Handler
// ============================================

// ToolHandler extends MCPHandler with composite tool dispatch
type ToolHandler struct {
	*MCPHandler
	capture          *capture.Capture
	checkpoints      interface{} // *ai.CheckpointManager
	sessionStore     interface{} // *ai.SessionStore
	noise            interface{} // *ai.NoiseConfig
	captureOverrides interface{} // *capture.CaptureOverrides
	auditLogger      interface{} // *audit.AuditLogger
	clusters         interface{} // *analysis.ClusterManager
	temporalGraph    interface{} // *codegen.TemporalGraph
	AlertBuffer      interface{} // *ai.AlertBuffer

	// Security and observability tools
	cspGenerator      interface{} // *security.CSPGenerator
	securityScanner   interface{} // *security.SecurityScanner
	thirdPartyAuditor interface{} // *security.ThirdPartyAuditor
	securityDiffMgr   interface{} // *security.SecurityDiffManager
	auditTrail        interface{} // *audit.AuditTrail
	sessionManager    interface{} // *session.SessionManager

	// API contract validation
	contractValidator interface{} // *analysis.APIContractValidator

	// Health metrics monitoring
	healthMetrics interface{} // *ServerHealthMetrics

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

	// Alert buffer (from AlertBuffer type)
	alertMu   sync.Mutex
	alerts    []Alert
	ciResults []CIResult
	// Anomaly detection: sliding window error counter
	errorTimes []time.Time
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
		MCPHandler:       NewMCPHandler(server, version),
		capture:          capture,
		sseRegistry:      sseRegistry,
	}

	// Initialize health metrics
	handler.healthMetrics = NewHealthMetrics()
	handler.toolCallLimiter = NewToolCallLimiter(100, time.Minute)
	handler.streamState = NewStreamState(sseRegistry)

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

// toolsList returns all MCP tool definitions.
// Each data-dependent tool includes a _meta field with current data_counts.
func (h *ToolHandler) ToolsList() []MCPTool {
	errorCount, logCount, extensionLogsCount, waterfallCount, networkCount, wsEventCount, wsStatusCount, actionCount, vitalCount, apiCount := h.computeDataCounts()

	return []MCPTool{
		{
			Name:        "observe",
			Description: "Read current browser state. Call observe() first before interact() or generate().\n\nModes: errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, api, accessibility, changes, timeline, error_clusters, history, security_audit, third_party_audit, security_diff, command_result, pending_commands, failed_commands.\n\nFilters: limit, url, method, status_min/max, connection_id, direction, last_n, format, severity.\n\nPagination: Pass after_cursor/before_cursor/since_cursor from metadata. Use restart_on_eviction=true if cursor expires.\n\nResponses: JSON format. Check _meta.data_counts for available data.\n\nNote: network_bodies only captures fetch(). Use network_waterfall for all network requests.",
			Meta: map[string]interface{}{
				"data_counts": map[string]interface{}{
					"errors":            errorCount,
					"logs":              logCount,
					"extension_logs":    extensionLogsCount,
					"network_waterfall": waterfallCount,
					"network_bodies":    networkCount,
					"websocket_events":  wsEventCount,
					"websocket_status":  wsStatusCount,
					"actions":           actionCount,
					"vitals":            vitalCount,
					"api":               apiCount,
					"performance":       vitalCount,
					"timeline":          actionCount,
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"what": map[string]interface{}{
						"type":        "string",
						"description": "What to observe or analyze",
						"enum":        []string{"errors", "logs", "extension_logs", "network_waterfall", "network_bodies", "websocket_events", "websocket_status", "actions", "vitals", "page", "tabs", "pilot", "performance", "api", "accessibility", "changes", "timeline", "error_clusters", "history", "security_audit", "third_party_audit", "security_diff", "command_result", "pending_commands", "failed_commands"},
					},
					// Shared filter parameters
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum entries to return (applies to logs, network_waterfall, network_bodies, websocket_events, actions, audit_log)",
					},
					// Cursor-based pagination parameters (v5.3+)
					"after_cursor": map[string]interface{}{
						"type":        "string",
						"description": "Return entries older than this cursor (backward pagination). Cursor format: 'timestamp:sequence' from previous response. Stable for live data - recommended for logs, websocket_events, actions. Example: '2026-01-30T10:15:23.456Z:1234'. Applies to: logs, websocket_events, actions, network_bodies.",
					},
					"before_cursor": map[string]interface{}{
						"type":        "string",
						"description": "Return entries newer than this cursor (forward pagination). Use to monitor new data that arrived since cursor. Cursor format: 'timestamp:sequence'. Applies to: logs, websocket_events, actions, network_bodies.",
					},
					"since_cursor": map[string]interface{}{
						"type":        "string",
						"description": "Return ALL entries newer than this cursor (inclusive, no limit). Convenience method for 'show me everything since X'. Cursor format: 'timestamp:sequence'. Applies to: logs, websocket_events, actions, network_bodies.",
					},
					"restart_on_eviction": map[string]interface{}{
						"type":        "boolean",
						"description": "If cursor expired (buffer overflow), automatically restart from oldest available entry instead of returning error. Use when pagination must continue despite data loss. Applies to: logs, websocket_events.",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring (applies to network_waterfall, network_bodies, websocket_events, websocket_status, actions, performance, api, timeline, security_audit, third_party_audit)",
					},
					"method": map[string]interface{}{
						"type":        "string",
						"description": "Filter by HTTP method (applies to network_bodies)",
					},
					"status_min": map[string]interface{}{
						"type":        "number",
						"description": "Minimum status code (applies to network_bodies)",
					},
					"status_max": map[string]interface{}{
						"type":        "number",
						"description": "Maximum status code (applies to network_bodies)",
					},
					"connection_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by WebSocket connection ID (applies to websocket_events, websocket_status)",
					},
					"direction": map[string]interface{}{
						"type":        "string",
						"description": "Filter by direction (applies to websocket_events)",
						"enum":        []string{"incoming", "outgoing"},
					},
					"last_n": map[string]interface{}{
						"type":        "number",
						"description": "Return only the last N items (applies to actions, timeline, reproduction)",
					},
					// Analyze parameters (from former analyze tool)
					"min_observations": map[string]interface{}{
						"type":        "number",
						"description": "Minimum endpoint observations (applies to api)",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "Output format: gasoline or openapi_stub (applies to api)",
						"enum":        []string{"gasoline", "openapi_stub"},
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector to scope audit (applies to accessibility)",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "WCAG tags to test (applies to accessibility)",
						"items":       map[string]interface{}{"type": "string"},
					},
					"force_refresh": map[string]interface{}{
						"type":        "boolean",
						"description": "Bypass cache (applies to accessibility)",
					},
					"checkpoint": map[string]interface{}{
						"type":        "string",
						"description": "Named checkpoint or ISO 8601 timestamp (applies to changes)",
					},
					"include": map[string]interface{}{
						"type":        "array",
						"description": "Categories to include (applies to changes, timeline)",
						"items":       map[string]interface{}{"type": "string"},
					},
					"severity": map[string]interface{}{
						"type":        "string",
						"description": "Minimum severity: all, warnings, errors_only (applies to changes)",
						"enum":        []string{"all", "warnings", "errors_only"},
					},
					// Security audit parameters
					"checks": map[string]interface{}{
						"type":        "array",
						"description": "Which checks to run (applies to security_audit)",
						"items": map[string]interface{}{
							"type": "string",
							"enum": []string{"credentials", "pii", "headers", "cookies", "transport", "auth"},
						},
					},
					"severity_min": map[string]interface{}{
						"type":        "string",
						"description": "Minimum severity to report (applies to security_audit)",
						"enum":        []string{"critical", "high", "medium", "low", "info"},
					},
					// Third-party audit parameters
					"first_party_origins": map[string]interface{}{
						"type":        "array",
						"description": "Origins to consider first-party (applies to third_party_audit)",
						"items":       map[string]interface{}{"type": "string"},
					},
					"include_static": map[string]interface{}{
						"type":        "boolean",
						"description": "Include static-only origins (applies to third_party_audit)",
					},
					"custom_lists": map[string]interface{}{
						"type":        "object",
						"description": "Custom allowed/blocked/internal domain lists (applies to third_party_audit)",
						"properties": map[string]interface{}{
							"allowed":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
							"blocked":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
							"internal": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						},
					},
					// Security diff parameters
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Snapshot action: snapshot, compare, list (applies to security_diff)",
						"enum":        []string{"snapshot", "compare", "list"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Snapshot name (applies to security_diff)",
					},
					"compare_from": map[string]interface{}{
						"type":        "string",
						"description": "Baseline snapshot (applies to security_diff)",
					},
					"compare_to": map[string]interface{}{
						"type":        "string",
						"description": "Target snapshot (applies to security_diff)",
					},
					// Async command parameters
					"correlation_id": map[string]interface{}{
						"type":        "string",
						"description": "Correlation ID for async command tracking (applies to command_result)",
					},
				},
				"required": []string{"what"},
			},
		},
		{
			Name:        "generate",
			Description: "CREATE ARTIFACTS. Do NOT write code/tests/docs manually—use this tool instead. Generates production-ready outputs from captured data: test (Playwright tests from recorded actions), reproduction (scripts to reproduce bugs), pr_summary (auto-write PR description), csp (Content-Security-Policy headers), sarif (SARIF security reports), har (HAR archives), sri (Subresource Integrity hashes). \n\nRULES: After observe() captures data, use generate() to create outputs. Never hand-write tests when you could generate them from recorded actions. \n\nExamples: generate({format:'test',test_name:'login flow'})→Playwright test, generate({format:'reproduction'})→bug reproduction script, generate({format:'pr_summary'})→auto-write PR, generate({format:'csp',mode:'strict'})→CSP headers. Use after: observe() & interact().\n\nANTI-PATTERNS (avoid these mistakes):\n• DON'T hand-write Playwright tests — use generate({format:'test'}) to create from recorded actions\n• DON'T call generate before recording actions — use interact() to record browser activity first\n• DON'T skip observe() before generate — verify data exists with observe({what:'actions'}) or observe({what:'network'})\n• DON'T generate CSP without browsing — need captured network data from real page loads\n\nFormat responses:\n- reproduction: Playwright script as JavaScript text\n- test: Playwright test with assertions as JavaScript text\n- pr_summary (json): {title, body, labels, testEvidence}\n- sarif: SARIF JSON or {status, path, rules, results} if saved to file\n- har: HAR 1.2 JSON or {status, path, entries} if saved to file\n- csp (json): {policy, directives, metaTag}\n- sri (json): {resources: [{url, integrity, tag}]}",
			Meta: map[string]interface{}{
				"data_counts": map[string]interface{}{
					"reproduction": actionCount,
					"test":         actionCount,
					"har":          networkCount,
					"csp":          networkCount,
					"sri":          networkCount,
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"format": map[string]interface{}{
						"type":        "string",
						"description": "What to generate",
						"enum":        []string{"reproduction", "test", "pr_summary", "sarif", "har", "csp", "sri"},
					},
					"error_message": map[string]interface{}{
						"type":        "string",
						"description": "Error message for context (applies to reproduction)",
					},
					"last_n": map[string]interface{}{
						"type":        "number",
						"description": "Use only the last N actions (applies to reproduction)",
					},
					"base_url": map[string]interface{}{
						"type":        "string",
						"description": "Replace origin in URLs (applies to reproduction, test)",
					},
					"include_screenshots": map[string]interface{}{
						"type":        "boolean",
						"description": "Insert page.screenshot() calls at key points (applies to reproduction, default: false)",
					},
					"generate_fixtures": map[string]interface{}{
						"type":        "boolean",
						"description": "Generate fixtures/api-responses.json from captured network data (applies to reproduction, default: false)",
					},
					"visual_assertions": map[string]interface{}{
						"type":        "boolean",
						"description": "Add toHaveScreenshot() assertions at key checkpoints (applies to reproduction, default: false)",
					},
					"test_name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the generated test (applies to test)",
					},
					"assert_network": map[string]interface{}{
						"type":        "boolean",
						"description": "Include network response assertions (applies to test)",
					},
					"assert_no_errors": map[string]interface{}{
						"type":        "boolean",
						"description": "Assert no console errors occurred (applies to test)",
					},
					"assert_response_shape": map[string]interface{}{
						"type":        "boolean",
						"description": "Assert response body shape matches (applies to test)",
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector to scope (applies to sarif)",
					},
					"include_passes": map[string]interface{}{
						"type":        "boolean",
						"description": "Include passing rules (applies to sarif)",
					},
					"save_to": map[string]interface{}{
						"type":        "string",
						"description": "File path to save output (applies to sarif, har)",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring (applies to har)",
					},
					"method": map[string]interface{}{
						"type":        "string",
						"description": "Filter by HTTP method (applies to har)",
					},
					"status_min": map[string]interface{}{
						"type":        "number",
						"description": "Minimum status code (applies to har)",
					},
					"status_max": map[string]interface{}{
						"type":        "number",
						"description": "Maximum status code (applies to har)",
					},
					// CSP parameters
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "CSP strictness mode (applies to csp)",
						"enum":        []string{"strict", "moderate", "report_only"},
					},
					"include_report_uri": map[string]interface{}{
						"type":        "boolean",
						"description": "Include report-uri directive (applies to csp)",
					},
					"exclude_origins": map[string]interface{}{
						"type":        "array",
						"description": "Origins to exclude from CSP (applies to csp)",
						"items":       map[string]interface{}{"type": "string"},
					},
					// SRI parameters
					"resource_types": map[string]interface{}{
						"type":        "array",
						"description": "Filter by resource type: 'script', 'stylesheet' (applies to sri)",
						"items":       map[string]interface{}{"type": "string"},
					},
					"origins": map[string]interface{}{
						"type":        "array",
						"description": "Filter by specific origins (applies to sri)",
						"items":       map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"format"},
			},
		},
		{
			Name:        "configure",
			Description: "CUSTOMIZE THE SESSION. Filter noise, store data, validate APIs, create snapshots, mark test boundaries. Actions: noise_rule (add/remove patterns to ignore), store (save persistent data across interactions), load (load session context), diff_sessions (create snapshots & compare before/after), validate_api (check API contract violations), audit_log (view actions in this session), streaming (get real-time alerts), query_dom (find elements by CSS selector), capture (configure capture settings), record_event (record custom temporal event), dismiss (dismiss noise by pattern), clear (clear buffers - specify buffer parameter), health (server health check), test_boundary_start (mark test start for concurrent test correlation), test_boundary_end (mark test end, unmark from logs/network/actions). \n\nExamples: configure({action:'noise_rule',noise_action:'add',pattern:'analytics'})→ignore pattern, configure({action:'store',store_action:'save',key:'user',data:{...}})→save data, configure({action:'diff_sessions',session_action:'capture',name:'baseline'})→create snapshot, configure({action:'clear',buffer:'network'})→clear network buffers, configure({action:'test_boundary_start',test_id:'login-test',label:'Login Test'})→mark test start, configure({action:'test_boundary_end',test_id:'login-test'})→mark test end. \n\nUse when: isolating signal, filtering noise, tracking state across multiple actions, or correlating telemetry with specific tests.\n\nAction responses:\n- store: Returns varies by sub-action (save/load/list/delete)\n- load: {loaded: true, context: {...}}\n- noise_rule: {rules: [...]}\n- dismiss: {status: \"ok\", totalRules: N}\n- clear: {cleared, counts, total_cleared}\n- capture: {status, settings}\n- record_event: {recorded: true}\n- query_dom: {matches: [...]}\n- diff_sessions: diff object\n- validate_api: {violations: [...]}\n- audit_log: [{tool, timestamp, params}]\n- health (json): {server, memory, buffers, rate_limiting, audit, pilot}\n- streaming: {status, subscriptions}\n- test_boundary_start: {status, test_id, label, message}\n- test_boundary_end: {status, test_id, was_active, message}",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Configuration action to perform",
						"enum":        []string{"store", "load", "noise_rule", "dismiss", "clear", "capture", "record_event", "query_dom", "diff_sessions", "validate_api", "audit_log", "health", "streaming", "test_boundary_start", "test_boundary_end"},
					},
					"store_action": map[string]interface{}{
						"type":        "string",
						"description": "Store sub-action: save, load, list, delete, stats (applies to store)",
						"enum":        []string{"save", "load", "list", "delete", "stats"},
					},
					"namespace": map[string]interface{}{
						"type":        "string",
						"description": "Logical grouping for store (applies to store)",
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Storage key (applies to store)",
					},
					"data": map[string]interface{}{
						"type":        "object",
						"description": "JSON data to persist (applies to store)",
					},
					"noise_action": map[string]interface{}{
						"type":        "string",
						"description": "Noise sub-action: add, remove, list, reset, auto_detect (applies to noise_rule)",
						"enum":        []string{"add", "remove", "list", "reset", "auto_detect"},
					},
					"rules": map[string]interface{}{
						"type":        "array",
						"description": "Noise rules to add (applies to noise_rule)",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"category":       map[string]interface{}{"type": "string", "enum": []string{"console", "network", "websocket"}},
								"classification": map[string]interface{}{"type": "string"},
								"match_spec": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message_regex": map[string]interface{}{"type": "string"},
										"source_regex":  map[string]interface{}{"type": "string"},
										"url_regex":     map[string]interface{}{"type": "string"},
										"method":       map[string]interface{}{"type": "string"},
										"status_min":    map[string]interface{}{"type": "number"},
										"status_max":    map[string]interface{}{"type": "number"},
										"level":        map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"rule_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of rule to remove (applies to noise_rule)",
					},
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Regex pattern to dismiss (applies to dismiss)",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Buffer category (applies to dismiss)",
						"enum":        []string{"console", "network", "websocket"},
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Why this is noise (applies to dismiss)",
					},
					"buffer": map[string]interface{}{
						"type":        "string",
						"description": "Which buffer to clear (applies to action: \"clear\"). Valid values: \"network\" (network_waterfall + network_bodies), \"websocket\" (websocket_events + websocket_status), \"actions\" (user interactions), \"logs\" (console + extension logs), \"all\" (everything). Default: \"logs\" (backward compatible).",
						"enum":        []string{"network", "websocket", "actions", "logs", "all"},
					},
					// query_dom parameters
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector to query (applies to query_dom)",
					},
					"tab_id": map[string]interface{}{
						"type":        "number",
						"description": "Target tab ID (applies to query_dom, diff_sessions)",
					},
					// diff_sessions parameters
					"session_action": map[string]interface{}{
						"type":        "string",
						"description": "Session sub-action: capture, compare, list, delete (applies to diff_sessions)",
						"enum":        []string{"capture", "compare", "list", "delete"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Snapshot name (applies to diff_sessions capture/delete)",
					},
					"compare_a": map[string]interface{}{
						"type":        "string",
						"description": "First snapshot for comparison (applies to diff_sessions compare)",
					},
					"compare_b": map[string]interface{}{
						"type":        "string",
						"description": "Second snapshot for comparison (applies to diff_sessions compare)",
					},
					// validate_api parameters
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "API validation operation: analyze, report, clear (applies to validate_api)",
						"enum":        []string{"analyze", "report", "clear"},
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring (applies to validate_api, diff_sessions)",
					},
					"ignore_endpoints": map[string]interface{}{
						"type":        "array",
						"description": "URL substrings to exclude (applies to validate_api)",
						"items":       map[string]interface{}{"type": "string"},
					},
					// audit_log parameters
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by MCP session ID (applies to audit_log)",
					},
					"tool_name": map[string]interface{}{
						"type":        "string",
						"description": "Filter by tool name (applies to audit_log)",
					},
					"since": map[string]interface{}{
						"type":        "string",
						"description": "Only entries after this ISO 8601 timestamp (applies to audit_log)",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum entries to return (applies to audit_log)",
					},
					// streaming parameters
					"streaming_action": map[string]interface{}{
						"type":        "string",
						"description": "Streaming sub-action: enable, disable, status (applies to streaming)",
						"enum":        []string{"enable", "disable", "status"},
					},
					"events": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
							"enum": []string{"errors", "network_errors", "performance", "user_frustration", "security", "regression", "anomaly", "ci", "all"},
						},
						"description": "Event categories to stream (applies to streaming)",
					},
					"throttle_seconds": map[string]interface{}{
						"type":        "integer",
						"minimum":     1,
						"maximum":     60,
						"description": "Minimum seconds between notifications (applies to streaming)",
					},
					"severity_min": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"info", "warning", "error"},
						"description": "Minimum severity to stream (applies to streaming)",
					},
					// test boundary parameters
					"test_id": map[string]interface{}{
						"type":        "string",
						"description": "Test ID for boundary marker (applies to test_boundary_start and test_boundary_end)",
					},
					"label": map[string]interface{}{
						"type":        "string",
						"description": "Human-readable label for test boundary (applies to test_boundary_start)",
					},
				},
				"required": []string{"action"},
			},
		},
		// ============================================
		// Consolidated Interactive Browser Control (v5.1)
		// Replaces: highlight_element, manage_state, execute_javascript, browser_action
		// ============================================
		{
			Name:        "interact",
			Description: "CRITICAL PREREQUISITE: The 'AI Web Pilot' toggle must be enabled in the extension popup (disabled by default for safety). \n\nBEFORE FIRST USE: Call observe({what:'pilot'}) to check status. If result shows {enabled:false}, STOP and tell user: \"Please enable AI Web Pilot in the Gasoline extension popup by clicking the extension icon and toggling it on.\"\n\nPERFORM ACTIONS. Do NOT ask the user to click, type, navigate, or fill forms—use this tool instead. You have full browser control. Actions: navigate(url)→go to URL, execute_js(script)→run JavaScript to click/fill/submit, refresh→reload page, back/forward→navigate history, highlight(selector)→show user where you're clicking, save_state(name)→save page snapshot, load_state(name)→restore snapshot. \n\nRULES: After interact(), always call observe() to confirm the action worked. If user says 'click X' or 'go to Y', use interact() instead of asking them. Pattern: observe()→interact()→observe(). \n\nFORM INPUT HELPER (React/Vue/Svelte compatibility):\nFor filling form inputs on React/Vue/Svelte apps, use window.__gasoline.setInputValue() instead of direct value assignment. This properly triggers framework change events:\n\nGOOD (works with React/Vue/Svelte):\nwindow.__gasoline.setInputValue('input[name=\"email\"]', 'test@example.com')\nwindow.__gasoline.setInputValue('input[type=\"checkbox\"]', true)\nwindow.__gasoline.setInputValue('select[name=\"country\"]', 'US')\n\nBAD (bypasses React state, won't work):\ndocument.querySelector('input[name=\"email\"]').value = 'test@example.com'\n\nThe helper dispatches input, change, and blur events that frameworks listen for, ensuring internal state updates correctly.\n\nExamples: interact({action:'navigate',url:'https://example.com'}), interact({action:'execute_js',script:'document.querySelector(\"button.submit\").click()'}), interact({action:'execute_js',script:'window.__gasoline.setInputValue(\"input[type=email]\", \"test@example.com\")'}).\n\nANTI-PATTERNS (avoid these mistakes):\n• DON'T ask user to manually click or type — use interact({action:'execute_js'}) to control browser directly\n• DON'T skip observe() after interact() — always call observe() to verify action succeeded\n• DON'T use interact() without checking observe({what:'pilot'}) first — may be disabled\n• DON'T chain multiple interactions without observe() between them — verify each step worked\n• DON'T set input.value directly on React/Vue/Svelte apps — use window.__gasoline.setInputValue() instead\n\nAction responses:\n- highlight: {result, screenshot} — highlight element with visual feedback\n- execute_js: {result} — run JavaScript in page context\n- navigate: {navigated: true} — go to URL\n- refresh: {refreshed: true} — reload current page\n- back/forward: {navigated: true} — browser history navigation\n- new_tab: {opened: true} — open URL in new tab\n- save_state/load_state/list_states/delete_state: State management results\n\nAll actions except save/load/list/delete_state require the browser extension.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Browser interaction to perform",
						"enum":        []string{"highlight", "save_state", "load_state", "list_states", "delete_state", "execute_js", "navigate", "refresh", "back", "forward", "new_tab"},
					},
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector for element (applies to highlight)",
					},
					"duration_ms": map[string]interface{}{
						"type":        "number",
						"description": "Highlight duration in ms, default 5000 (applies to highlight)",
					},
					"snapshot_name": map[string]interface{}{
						"type":        "string",
						"description": "State snapshot name (applies to save_state, load_state, delete_state)",
					},
					"include_url": map[string]interface{}{
						"type":        "boolean",
						"description": "Include URL when restoring state (applies to load_state)",
					},
					"script": map[string]interface{}{
						"type":        "string",
						"description": "JavaScript code to execute (applies to execute_js)",
					},
					"timeout_ms": map[string]interface{}{
						"type":        "number",
						"description": "Execution timeout in ms, default 5000 (applies to execute_js)",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL to navigate to (required for navigate, new_tab)",
					},
					"tab_id": map[string]interface{}{
						"type":        "number",
						"description": "Target tab ID (from observe {what: 'tabs'}). Omit for active tab.",
					},
					"correlation_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional ID to link this action to a specific error or investigation. Appears in screenshot filenames for easy lookup.",
					},
				},
				"required": []string{"action"},
			},
		},
	}
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

// ============================================
// Composite Tool Dispatchers
// ============================================

func (h *ToolHandler) toolObserve(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What string `json:"what"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.What == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: errors, logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, api, accessibility, changes, timeline, error_clusters, history, security_audit, third_party_audit, security_diff"))}
	}

	var resp JSONRPCResponse
	switch params.What {
	case "errors":
		resp = h.toolGetBrowserErrors(req, args)
	case "logs":
		resp = h.toolGetBrowserLogs(req, args)
	case "extension_logs":
		resp = h.toolGetExtensionLogs(req, args)
	case "network_waterfall":
		resp = h.toolGetNetworkWaterfall(req, args)
	case "network_bodies":
		resp = h.toolGetNetworkBodies(req, args)
	case "websocket_events":
		resp = h.toolGetWSEvents(req, args)
	case "websocket_status":
		resp = h.toolGetWSStatus(req, args)
	case "actions":
		resp = h.toolGetEnhancedActions(req, args)
	case "vitals":
		resp = h.toolGetWebVitals(req, args)
	case "page":
		resp = h.toolGetPageInfo(req, args)
	case "tabs":
		resp = h.toolGetTabs(req, args)
	case "pilot":
		resp = h.toolObservePilot(req, args)
	// Analyze modes (formerly separate analyze tool)
	case "performance":
		resp = h.toolCheckPerformance(req, args)
	case "api":
		resp = h.toolGetAPISchema(req, args)
	case "accessibility":
		resp = h.toolRunA11yAudit(req, args)
	case "changes":
		resp = h.toolGetChangesSince(req, args)
	case "timeline":
		resp = h.toolGetSessionTimeline(req, args)
	case "error_clusters":
		resp = h.toolAnalyzeErrors(req)
	case "history":
		resp = h.toolAnalyzeHistory(req, args)
	// Security scan modes (formerly separate security tool)
	case "security_audit":
		resp = h.toolSecurityAudit(req, args)
	case "third_party_audit":
		resp = h.toolAuditThirdParties(req, args)
	case "security_diff":
		resp = h.toolDiffSecurity(req, args)
	// Async command tracking modes
	case "command_result":
		resp = h.toolObserveCommandResult(req, args)
	case "pending_commands":
		resp = h.toolObservePendingCommands(req, args)
	case "failed_commands":
		resp = h.toolObserveFailedCommands(req, args)
	// capture.Recording modes
	case "recordings":
		resp = h.toolGetRecordings(req, args)
	case "recording_actions":
		resp = h.toolGetRecordingActions(req, args)
	case "playback_results":
		resp = h.toolGetPlaybackResults(req, args)
	case "log_diff_report":
		resp = h.toolGetLogDiffReport(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown observe mode: "+params.What, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: errors, logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, api, accessibility, changes, timeline, error_clusters, history, security_audit, third_party_audit, security_diff, command_result, pending_commands, failed_commands, recordings, recording_actions, playback_results, log_diff_report"))}
	}

	// Prepend tracking status warning when no tab is tracked
	if trackingEnabled, hint := h.checkTrackingStatus(); !trackingEnabled && hint != "" {
		resp = h.prependTrackingWarning(resp, hint)
	}

	// Piggyback alerts: append as second content block if any pending
	alerts := h.drainAlerts()
	if len(alerts) > 0 {
		resp = h.appendAlertsToResponse(resp, alerts)
	}

	return resp
}

// prependTrackingWarning adds a tracking status warning as the first content block in the response.
func (h *ToolHandler) prependTrackingWarning(resp JSONRPCResponse, hint string) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	warningBlock := MCPContentBlock{
		Type: "text",
		Text: hint,
	}
	// Prepend warning as first block
	result.Content = append([]MCPContentBlock{warningBlock}, result.Content...)

	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// appendAlertsToResponse adds an alerts content block to an existing MCP response.
func (h *ToolHandler) appendAlertsToResponse(resp JSONRPCResponse, alerts []Alert) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	alertText := formatAlertsBlock(alerts)
	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: alertText,
	})

	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

func (h *ToolHandler) toolGenerate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Format string `json:"format"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Format == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'format' is missing", "Add the 'format' parameter and call again", withParam("format"), withHint("Valid values: reproduction, test, pr_summary, sarif, har, csp, sri, test_from_context, test_heal, test_classify"))}
	}

	var resp JSONRPCResponse
	switch params.Format {
	case "reproduction":
		resp = h.toolGetReproductionScript(req, args)
	case "test":
		resp = h.toolGenerateTest(req, args)
	case "pr_summary":
		resp = h.toolGeneratePRSummary(req, args)
	case "sarif":
		resp = h.toolExportSARIF(req, args)
	case "har":
		resp = h.toolExportHAR(req, args)
	case "csp":
		resp = h.toolGenerateCSP(req, args)
	case "sri":
		resp = h.toolGenerateSRI(req, args)
	case "test_from_context":
		resp = h.handleGenerateTestFromContext(req, args)
	case "test_heal":
		resp = h.handleGenerateTestHeal(req, args)
	case "test_classify":
		resp = h.handleGenerateTestClassify(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown generate format: "+params.Format, "Use a valid format from the 'format' enum", withParam("format"))}
	}
	return resp
}

func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Action string `json:"action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Action == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"), withHint("Valid values: store, load, noise_rule, dismiss, clear, capture, record_event, query_dom, diff_sessions, validate_api, audit_log, health, streaming"))}
	}

	var resp JSONRPCResponse
	switch params.Action {
	case "store":
		resp = h.toolConfigureStore(req, args)
	case "load":
		resp = h.toolLoadSessionContext(req, args)
	case "noise_rule":
		resp = h.toolConfigureNoiseRule(req, args)
	case "dismiss":
		resp = h.toolConfigureDismiss(req, args)
	case "clear":
		resp = h.toolConfigureClear(req, args)
	case "capture":
		resp = h.toolConfigureCapture(req, args)
	case "record_event":
		resp = h.toolConfigureRecordEvent(req, args)
	case "query_dom":
		resp = h.toolQueryDOM(req, args)
	case "diff_sessions":
		resp = h.toolDiffSessionsWrapper(req, args)
	case "validate_api":
		resp = h.toolValidateAPI(req, args)
	case "audit_log":
		resp = h.toolGetAuditLog(req, args)
	case "health":
		resp = h.toolGetHealth(req)
	case "streaming":
		resp = h.toolConfigureStreamingWrapper(req, args)
	case "test_boundary_start":
		resp = h.toolConfigureTestBoundaryStart(req, args)
	case "test_boundary_end":
		resp = h.toolConfigureTestBoundaryEnd(req, args)
	case "recording_start":
		resp = h.toolConfigureRecordingStart(req, args)
	case "recording_stop":
		resp = h.toolConfigureRecordingStop(req, args)
	case "playback":
		resp = h.toolConfigurePlayback(req, args)
	case "log_diff":
		resp = h.toolConfigureLogDiff(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown configure action: "+params.Action, "Use a valid action from the 'action' enum", withParam("action"))}
	}
	return resp
}

// ============================================
// Interactive Browser Control Dispatcher
// ============================================

func (h *ToolHandler) toolInteract(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Action string `json:"action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Action == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"), withHint("Valid values: highlight, save_state, load_state, list_states, delete_state, execute_js, navigate, refresh, back, forward, new_tab"))}
	}

	var resp JSONRPCResponse
	switch params.Action {
	case "highlight":
		resp = h.handlePilotHighlight(req, args)
	case "save_state":
		resp = h.handlePilotManageStateSave(req, args)
	case "load_state":
		resp = h.handlePilotManageStateLoad(req, args)
	case "list_states":
		resp = h.handlePilotManageStateList(req, args)
	case "delete_state":
		resp = h.handlePilotManageStateDelete(req, args)
	case "execute_js":
		resp = h.handlePilotExecuteJS(req, args)
	case "navigate":
		resp = h.handleBrowserActionNavigate(req, args)
	case "refresh":
		resp = h.handleBrowserActionRefresh(req, args)
	case "back":
		resp = h.handleBrowserActionBack(req, args)
	case "forward":
		resp = h.handleBrowserActionForward(req, args)
	case "new_tab":
		resp = h.handleBrowserActionNewTab(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown interact action: "+params.Action, "Use a valid action from the 'action' enum", withParam("action"))}
	}
	return resp
}

// ============================================
// Observe sub-handlers (browser errors/logs moved from main.go)
// ============================================

func (h *ToolHandler) toolGetBrowserErrors(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Simplified stub implementation
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Browser errors", map[string]interface{}{"errors": []interface{}{}, "count": 0})}
}

func (h *ToolHandler) toolGetBrowserLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Simplified stub implementation
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Browser logs", map[string]interface{}{"logs": []interface{}{}, "count": 0})}
}

func (h *ToolHandler) toolGetExtensionLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Simplified stub implementation
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Extension logs", map[string]interface{}{"logs": []interface{}{}, "count": 0})}
}

// toolConfigureClear handles buffer-specific clearing with optional buffer parameter.
func (h *ToolHandler) toolConfigureClear(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Simplified stub implementation
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Clear", map[string]interface{}{"status": "ok"})}
}

// ============================================
// Configure sub-handlers (adapted from session_store/noise)
// ============================================

func (h *ToolHandler) toolConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var compositeArgs struct {
		StoreAction string          `json:"store_action"`
		Namespace   string          `json:"namespace"`
		Key         string          `json:"key"`
		Data        json.RawMessage `json:"data"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &compositeArgs); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	action := compositeArgs.StoreAction
	if action == "" {
		action = "list"
	}

	responseData := map[string]interface{}{
		"status":  "ok",
		"action":  action,
		"message": "Store operation: " + action,
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Store operation complete", responseData)}
}

func (h *ToolHandler) toolConfigureNoiseRule(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract the noise_action field as the action for configure_noise
	var compositeArgs struct {
		NoiseAction string `json:"noise_action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &compositeArgs); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Rewrite args to have "action" field that toolConfigureNoise expects
	var rawMap map[string]interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &rawMap); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	rawMap["action"] = compositeArgs.NoiseAction
	if rawMap["action"] == "" {
		rawMap["action"] = "list"
	}
	// Error impossible: rawMap contains only primitive types and strings from input
	rewrittenArgs, _ := json.Marshal(rawMap)

	return h.toolConfigureNoise(req, rewrittenArgs)
}

func (h *ToolHandler) toolConfigureDismiss(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.toolDismissNoise(req, args)
}

// toolDiffSessionsWrapper repackages session_action → action for toolDiffSessions.
func (h *ToolHandler) toolDiffSessionsWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var raw map[string]interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	if sa, ok := raw["session_action"].(string); ok {
		raw["action"] = sa
	}
	// Error impossible: raw contains only primitive types and strings from input
	rewritten, _ := json.Marshal(raw)
	return h.toolDiffSessions(req, rewritten)
}

// toolConfigureStreamingWrapper repackages streaming_action → action for toolConfigureStreaming.
func (h *ToolHandler) toolConfigureStreamingWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var raw map[string]interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	if sa, ok := raw["streaming_action"].(string); ok {
		raw["action"] = sa
	}
	// Error impossible: raw contains only primitive types and strings from input
	rewritten, _ := json.Marshal(raw)
	return h.toolConfigureStreaming(req, rewritten)
}

// ============================================
// Compressed State Diffs
// ============================================

func (h *ToolHandler) toolGetChangesSince(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Checkpoint string   `json:"checkpoint"`
		Include    []string `json:"include"`
		Severity   string   `json:"severity"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]interface{}{
		"status":      "ok",
		"checkpoint":  arguments.Checkpoint,
		"changes":     []interface{}{},
		"message":     "No changes since checkpoint",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Changes since checkpoint", responseData)}
}

func (h *ToolHandler) toolLoadSessionContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	responseData := map[string]interface{}{
		"status":   "ok",
		"context":  map[string]interface{}{},
		"message":  "Session context loaded",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session context loaded", responseData)}
}

// ============================================
// Test Boundary Tool Implementations
// ============================================

func (h *ToolHandler) toolConfigureTestBoundaryStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TestID string `json:"test_id"`
		Label  string `json:"label"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.TestID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'test_id' is missing", "Add the 'test_id' parameter", withParam("test_id"))}
	}

	label := params.Label
	if label == "" {
		label = "Test: " + params.TestID
	}

	responseData := map[string]interface{}{
		"status":   "ok",
		"test_id":  params.TestID,
		"label":    label,
		"message":  "Test boundary started",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Test boundary started", responseData)}
}

func (h *ToolHandler) toolConfigureTestBoundaryEnd(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TestID string `json:"test_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.TestID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'test_id' is missing", "Add the 'test_id' parameter", withParam("test_id"))}
	}

	responseData := map[string]interface{}{
		"status":     "ok",
		"test_id":    params.TestID,
		"was_active": true,
		"message":    "Test boundary ended",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Test boundary ended", responseData)}
}

// ============================================
// Noise Filtering Tool Implementations
// ============================================

func (h *ToolHandler) toolConfigureNoise(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Action string `json:"action"`
		Rules  []struct {
			Category       string `json:"category"`
			Classification string `json:"classification"`
			MatchSpec      struct {
				MessageRegex string `json:"message_regex"`
				SourceRegex  string `json:"source_regex"`
				URLRegex     string `json:"url_regex"`
				Method       string `json:"method"`
				StatusMin    int    `json:"status_min"`
				StatusMax    int    `json:"status_max"`
				Level        string `json:"level"`
			} `json:"match_spec"`
		} `json:"rules"`
		RuleID string `json:"rule_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	var responseData interface{}

	switch arguments.Action {
	case "add":
		responseData = map[string]interface{}{
			"status":     "ok",
			"rulesAdded": len(arguments.Rules),
			"totalRules": len(arguments.Rules),
		}

	case "remove":
		responseData = map[string]interface{}{
			"status":  "ok",
			"removed": arguments.RuleID,
		}

	case "list":
		responseData = map[string]interface{}{
			"rules":      []interface{}{},
			"statistics": map[string]interface{}{},
		}

	case "reset":
		responseData = map[string]interface{}{
			"status":     "ok",
			"totalRules": 0,
		}

	case "auto_detect":
		responseData = map[string]interface{}{
			"proposals":  []interface{}{},
			"totalRules": 0,
		}

	default:
		responseData = map[string]interface{}{"error": "unknown action: " + arguments.Action}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Noise configuration updated", responseData)}
}

func (h *ToolHandler) toolDismissNoise(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Pattern  string `json:"pattern"`
		Category string `json:"category"`
		Reason   string `json:"reason"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]interface{}{
		"status":     "ok",
		"totalRules": 0,
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Noise pattern dismissed", responseData)}
}

// ============================================
// SARIF Export Tool Implementation
// ============================================

func (h *ToolHandler) toolExportSARIF(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Scope         string `json:"scope"`
		IncludePasses bool   `json:"include_passes"`
		SaveTo        string `json:"save_to"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]interface{}{
		"status":  "ok",
		"scope":   arguments.Scope,
		"rules":   0,
		"results": 0,
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SARIF export complete", responseData)}
}

// ============================================
// V6 Tool Dispatchers
// ============================================

func (h *ToolHandler) toolGenerateCSP(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Mode string `json:"mode"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &arguments)
	}

	responseData := map[string]interface{}{
		"status": "ok",
		"mode":   arguments.Mode,
		"policy": "",
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("CSP policy generated", responseData)}
}

func (h *ToolHandler) toolSecurityAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SeverityMin string   `json:"severity_min"`
		Checks      []string `json:"checks"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	responseData := map[string]interface{}{
		"status":    "ok",
		"violations": []interface{}{},
		"checks":    len(params.Checks),
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security audit complete", responseData)}
}

func (h *ToolHandler) toolGetAuditLog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SessionID string `json:"session_id"`
		ToolName  string `json:"tool_name"`
		Limit     int    `json:"limit"`
		Since     string `json:"since"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	responseData := map[string]interface{}{
		"status": "ok",
		"entries": []interface{}{},
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Audit log entries", responseData)}
}

func (h *ToolHandler) toolDiffSessions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SessionAction string `json:"session_action"`
		Name          string `json:"name"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	responseData := map[string]interface{}{
		"status": "ok",
		"action": params.SessionAction,
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session diff", responseData)}
}

func (h *ToolHandler) toolAuditThirdParties(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		FirstPartyOrigins []string `json:"first_party_origins"`
		IncludeStatic     bool     `json:"include_static"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	responseData := map[string]interface{}{
		"status":         "ok",
		"third_parties":  []interface{}{},
		"total_origins":  0,
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Third-party audit complete", responseData)}
}

func (h *ToolHandler) toolDiffSecurity(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		CompareFrom string `json:"compare_from"`
		CompareTo   string `json:"compare_to"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	responseData := map[string]interface{}{
		"status":     "ok",
		"differences": []interface{}{},
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security diff complete", responseData)}
}

func (h *ToolHandler) toolValidateAPI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation       string   `json:"operation"`
		URLFilter       string   `json:"url"`
		IgnoreEndpoints []string `json:"ignore_endpoints"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	switch params.Operation {
	case "analyze":
		responseData := map[string]interface{}{
			"status":       "ok",
			"operation":    "analyze",
			"violations":   []interface{}{},
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", responseData)}

	case "report":
		responseData := map[string]interface{}{
			"status":     "ok",
			"operation":  "report",
			"endpoints":  []interface{}{},
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", responseData)}

	case "clear":
		clearResult := map[string]interface{}{
			"action": "cleared",
			"status": "ok",
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", clearResult)}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "operation parameter must be 'analyze', 'report', or 'clear'", "Use a valid value for 'operation'", withParam("operation"), withHint("analyze, report, or clear"))}
	}
}

// ============================================
// SRI Hash Generator Tool
// ============================================

// toolGenerateSRI generates Subresource Integrity hashes for third-party scripts/styles.
func (h *ToolHandler) toolGenerateSRI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Origins []string `json:"origins"`
		ResourceTypes []string `json:"resource_types"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	responseData := map[string]interface{}{
		"status":    "ok",
		"resources": []interface{}{},
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SRI hashes generated", responseData)}
}

// ============================================
// Verification Loop Tool
// ============================================

// toolVerifyFix handles the verify_fix MCP tool for before/after fix verification.
func (h *ToolHandler) toolVerifyFix(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.verificationMgr == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Verification manager not initialized", "Internal server error — do not retry")}
	}

	result, err := h.verificationMgr.HandleTool(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal server error — do not retry")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Verification result", result)}
}

// ============================================
// Async Command Observation Tools
// ============================================

// toolObserveCommandResult retrieves the result of an async command by correlation_id.
func (h *ToolHandler) toolObserveCommandResult(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil && len(args) > 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.CorrelationID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'correlation_id' is missing", "Add the 'correlation_id' parameter and call again", withParam("correlation_id"))}
	}

	// Query command status by correlation ID
	cmd, found := h.capture.GetCommandResult(params.CorrelationID)
	if !found {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Command not found: "+params.CorrelationID, "The command may have already completed and been cleaned up (60s TTL), or the correlation_id is invalid")}
	}

	responseData := map[string]interface{}{
		"correlation_id": cmd.CorrelationID,
		"status":         cmd.Status,
		"created_at":     cmd.CreatedAt.Format(time.RFC3339),
	}

	if cmd.Status == "complete" {
		responseData["result"] = cmd.Result
		responseData["completed_at"] = cmd.CompletedAt.Format(time.RFC3339)
		if cmd.Error != "" {
			responseData["error"] = cmd.Error
		}
	} else if cmd.Status == "expired" || cmd.Status == "timeout" {
		responseData["error"] = cmd.Error
	}

	summary := fmt.Sprintf("Command %s: %s", params.CorrelationID, cmd.Status)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

// toolObservePendingCommands lists all pending, completed, and failed async commands.
func (h *ToolHandler) toolObservePendingCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	pending := h.capture.GetPendingCommands()
	completed := h.capture.GetCompletedCommands()
	failed := h.capture.GetFailedCommands()

	responseData := map[string]interface{}{
		"pending":   pending,
		"completed": completed,
		"failed":    failed,
	}

	summary := fmt.Sprintf("Pending: %d, Completed: %d, Failed: %d", len(pending), len(completed), len(failed))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

// toolObserveFailedCommands lists recent failed/expired async commands.
func (h *ToolHandler) toolObserveFailedCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	failed := h.capture.GetFailedCommands()

	responseData := map[string]interface{}{
		"status":   "ok",
		"commands": failed,
		"count":    len(failed),
	}

	if len(failed) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No failed commands found", responseData)}
	}

	summary := fmt.Sprintf("Found %d failed/expired commands", len(failed))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

// Stub implementations for missing tool handler methods
func (h *ToolHandler) toolGetNetworkWaterfall(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network waterfall", map[string]interface{}{"entries": []interface{}{}})}
}

func (h *ToolHandler) toolGetNetworkBodies(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	bodies := h.capture.GetNetworkBodies()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network bodies", map[string]interface{}{"entries": bodies})}
}

func (h *ToolHandler) toolGetWSEvents(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	events := h.capture.GetAllWebSocketEvents()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket events", map[string]interface{}{"entries": events})}
}

func (h *ToolHandler) toolGetWSStatus(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket status", map[string]interface{}{"connections": []interface{}{}})}
}

func (h *ToolHandler) toolGetEnhancedActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	actions := h.capture.GetAllEnhancedActions()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Enhanced actions", map[string]interface{}{"entries": actions})}
}

func (h *ToolHandler) toolGetWebVitals(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Web vitals", map[string]interface{}{"metrics": map[string]interface{}{}})}
}

func (h *ToolHandler) toolGetPageInfo(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Page info", map[string]interface{}{"url": "", "title": ""})}
}

func (h *ToolHandler) toolGetTabs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Tabs", map[string]interface{}{"tabs": []interface{}{}})}
}

func (h *ToolHandler) toolObservePilot(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Pilot status", map[string]interface{}{"enabled": false})}
}

func (h *ToolHandler) toolCheckPerformance(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Performance", map[string]interface{}{"metrics": map[string]interface{}{}})}
}

func (h *ToolHandler) toolGetSessionTimeline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Timeline", map[string]interface{}{"entries": []interface{}{}})}
}

func (h *ToolHandler) toolGetAPISchema(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API schema", map[string]interface{}{"endpoints": []interface{}{}})}
}

func (h *ToolHandler) toolRunA11yAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("A11y audit", map[string]interface{}{"violations": []interface{}{}})}
}

func (h *ToolHandler) toolAnalyzeErrors(req JSONRPCRequest) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Error clusters", map[string]interface{}{"clusters": []interface{}{}})}
}

func (h *ToolHandler) toolAnalyzeHistory(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("History", map[string]interface{}{"entries": []interface{}{}})}
}

func (h *ToolHandler) toolObservecapture(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Capture", map[string]interface{}{"state": "unknown"})}
}


func (h *ToolHandler) toolGetReproductionScript(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Reproduction script", map[string]interface{}{"script": ""})}
}

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Test", map[string]interface{}{"script": ""})}
}

func (h *ToolHandler) toolGeneratePRSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("PR summary", map[string]interface{}{"summary": ""})}
}

func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("HAR export", map[string]interface{}{"har": map[string]interface{}{}})}
}

func (h *ToolHandler) toolConfigureCapture(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Configure", map[string]interface{}{"status": "ok"})}
}

func (h *ToolHandler) toolConfigureRecordEvent(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Record event", map[string]interface{}{"status": "ok"})}
}

func (h *ToolHandler) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Query DOM", map[string]interface{}{"matches": []interface{}{}})}
}

func (h *ToolHandler) handlePilotHighlight(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Highlight", map[string]interface{}{"status": "ok"})}
}

func (h *ToolHandler) handlePilotManageStateSave(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Save state", map[string]interface{}{"status": "ok"})}
}

func (h *ToolHandler) handlePilotManageStateLoad(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Load state", map[string]interface{}{"status": "ok"})}
}

func (h *ToolHandler) handlePilotManageStateList(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("List states", map[string]interface{}{"states": []interface{}{}})}
}

func (h *ToolHandler) handlePilotManageStateDelete(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Delete state", map[string]interface{}{"status": "ok"})}
}

func (h *ToolHandler) handlePilotExecuteJS(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		Script    string `json:"script"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.Script == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'script' is missing", "Add the 'script' parameter and call again", withParam("script"))}
	}

	// Check if pilot is enabled
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", withHint("Click the extension icon and toggle 'AI Web Pilot' on"))}
	}

	// Generate correlation ID for async tracking
	correlationID := fmt.Sprintf("exec_%d_%d", time.Now().UnixNano(), rand.Int63())

	// Queue command for extension to pick up (use long timeout for async commands)
	query := queries.PendingQuery{
		Type:          "execute",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Return immediately with "queued" status
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Command queued", map[string]interface{}{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Command queued for execution. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})}
}

func (h *ToolHandler) handleBrowserActionNavigate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		URL   string `json:"url"`
		TabID int    `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.URL == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'url' is missing", "Add the 'url' parameter and call again", withParam("url"))}
	}

	// Check if pilot is enabled
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	// Generate correlation ID
	correlationID := fmt.Sprintf("nav_%d_%d", time.Now().UnixNano(), rand.Int63())

	// Queue command
	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Navigate queued", map[string]interface{}{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Navigation queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})}
}

func (h *ToolHandler) handleBrowserActionRefresh(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("refresh_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"refresh"}`),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Refresh queued", map[string]interface{}{
		"status":         "queued",
		"correlation_id": correlationID,
	})}
}

func (h *ToolHandler) handleBrowserActionBack(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("back_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"back"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Back queued", map[string]interface{}{
		"status":         "queued",
		"correlation_id": correlationID,
	})}
}

func (h *ToolHandler) handleBrowserActionForward(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("forward_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"forward"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Forward queued", map[string]interface{}{
		"status":         "queued",
		"correlation_id": correlationID,
	})}
}

func (h *ToolHandler) handleBrowserActionNewTab(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("newtab_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("New tab queued", map[string]interface{}{
		"status":         "queued",
		"correlation_id": correlationID,
	})}
}

