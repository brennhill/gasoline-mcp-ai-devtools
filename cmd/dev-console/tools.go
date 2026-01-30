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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
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
	ErrCodePilotDisabled = "pilot_disabled" // Named ErrCodePilotDisabled to avoid collision with var ErrPilotDisabled in pilot.go
	ErrRateLimited       = "rate_limited"

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
	capture          *Capture
	checkpoints      *CheckpointManager
	sessionStore     *SessionStore
	noise            *NoiseConfig
	captureOverrides *CaptureOverrides
	auditLogger      *AuditLogger
	clusters         *ClusterManager
	temporalGraph    *TemporalGraph
	AlertBuffer      // Embedded alert state for push-based notifications

	// Security and observability tools
	cspGenerator      *CSPGenerator
	securityScanner   *SecurityScanner
	thirdPartyAuditor *ThirdPartyAuditor
	securityDiffMgr   *SecurityDiffManager
	auditTrail        *AuditTrail
	sessionManager    *SessionManager

	// API contract validation
	contractValidator *APIContractValidator

	// Health metrics monitoring
	healthMetrics *HealthMetrics

	// Verification loop for fix verification
	verificationMgr *VerificationManager

	// Redaction engine for scrubbing sensitive data from tool responses
	redactionEngine *RedactionEngine

	// Rate limiter for MCP tool calls (sliding window)
	toolCallLimiter *ToolCallLimiter

	// SSE registry for MCP streaming transport
	sseRegistry *SSERegistry

	// Context streaming: active push notifications via MCP
	streamState *StreamState
}

// NewToolHandler creates an MCP handler with composite tool capabilities
func NewToolHandler(server *Server, capture *Capture, sseRegistry *SSERegistry) *MCPHandler {
	handler := &ToolHandler{
		MCPHandler:       NewMCPHandler(server),
		capture:          capture,
		checkpoints:      NewCheckpointManager(server, capture),
		noise:            NewNoiseConfig(),
		captureOverrides: NewCaptureOverrides(),
		clusters:         NewClusterManager(),
		sseRegistry:      sseRegistry,
	}

	// Initialize persistent session store and temporal graph using CWD as project root.
	// If initialization fails (e.g., read-only filesystem), the server
	// continues without persistence — tool handlers check for nil.
	if cwd, err := os.Getwd(); err == nil {
		if store, err := NewSessionStore(cwd); err == nil {
			handler.sessionStore = store
		}
		gasolineDir := filepath.Join(cwd, ".gasoline")
		handler.temporalGraph = NewTemporalGraph(gasolineDir)
	}

	// Initialize audit logger. Best-effort — if it fails, capture control
	// still works without auditing.
	if home, err := os.UserHomeDir(); err == nil {
		auditPath := filepath.Join(home, ".gasoline", "audit.jsonl")
		if logger, err := NewAuditLogger(auditPath); err == nil {
			handler.auditLogger = logger
		}
	}

	// Initialize redaction engine (always active with built-in patterns).
	// Custom patterns loaded from server.redactionConfigPath if set.
	handler.redactionEngine = NewRedactionEngine(server.redactionConfigPath)

	handler.cspGenerator = NewCSPGenerator()
	capture.cspGen = handler.cspGenerator
	handler.securityScanner = NewSecurityScanner()
	handler.thirdPartyAuditor = NewThirdPartyAuditor()
	handler.securityDiffMgr = NewSecurityDiffManager()
	handler.auditTrail = NewAuditTrail(AuditConfig{MaxEntries: 10000, Enabled: true, RedactParams: true})
	handler.sessionManager = NewSessionManager(10, &captureStateAdapter{capture: capture, server: server})
	handler.contractValidator = NewAPIContractValidator()
	handler.healthMetrics = NewHealthMetrics()
	handler.verificationMgr = NewVerificationManager(&captureStateAdapter{capture: capture, server: server})
	handler.toolCallLimiter = NewToolCallLimiter(100, time.Minute)
	handler.streamState = NewStreamState(sseRegistry)

	// Wire error clustering: feed error-level log entries into the cluster manager.
	// Use SetOnEntries for thread-safe assignment (avoids racing with addEntries).
	server.SetOnEntries(func(entries []LogEntry) {
		for _, entry := range entries {
			level, _ := entry["level"].(string)
			if level != "error" {
				continue
			}
			var msg, stack, source string
			msg, _ = entry["message"].(string)
			stack, _ = entry["stack"].(string)
			source, _ = entry["source"].(string)

			// Extract from args array (extension format: args[0] is string or Error object)
			if args, ok := entry["args"].([]interface{}); ok && len(args) > 0 {
				switch first := args[0].(type) {
				case string:
					if msg == "" {
						msg = first
					}
				case map[string]interface{}:
					// Serialized Error object: {name, message, stack}
					if m, ok := first["message"].(string); ok && msg == "" {
						msg = m
					}
					if s, ok := first["stack"].(string); ok && stack == "" {
						stack = s
					}
				}
			}

			if msg == "" {
				continue
			}
			handler.clusters.AddError(ErrorInstance{
				Message:   msg,
				Stack:     stack,
				Source:    source,
				Timestamp: time.Now(),
				Severity:  "error",
			})
		}
	})

	// Return as MCPHandler but with overridden methods via the wrapper
	return &MCPHandler{
		server:      server,
		toolHandler: handler,
	}
}

// captureStateAdapter bridges the Capture/Server data to the CaptureStateReader interface
// required by SessionManager.
type captureStateAdapter struct {
	capture *Capture
	server  *Server
}

func (a *captureStateAdapter) GetConsoleErrors() []SnapshotError {
	a.server.mu.RLock()
	defer a.server.mu.RUnlock()
	var errors []SnapshotError
	for _, entry := range a.server.entries {
		if level, _ := entry["level"].(string); level == "error" {
			msg, _ := entry["message"].(string)
			errors = append(errors, SnapshotError{Type: "error", Message: msg, Count: 1})
		}
	}
	return errors
}

func (a *captureStateAdapter) GetConsoleWarnings() []SnapshotError {
	a.server.mu.RLock()
	defer a.server.mu.RUnlock()
	var warnings []SnapshotError
	for _, entry := range a.server.entries {
		if level, _ := entry["level"].(string); level == "warn" {
			msg, _ := entry["message"].(string)
			warnings = append(warnings, SnapshotError{Type: "warning", Message: msg, Count: 1})
		}
	}
	return warnings
}

func (a *captureStateAdapter) GetNetworkRequests() []SnapshotNetworkRequest {
	a.capture.mu.RLock()
	defer a.capture.mu.RUnlock()
	var requests []SnapshotNetworkRequest
	for _, body := range a.capture.networkBodies {
		requests = append(requests, SnapshotNetworkRequest{
			Method:   body.Method,
			URL:      body.URL,
			Status:   body.Status,
			Duration: body.Duration,
		})
	}
	return requests
}

func (a *captureStateAdapter) GetWSConnections() []SnapshotWSConnection {
	a.capture.mu.RLock()
	defer a.capture.mu.RUnlock()
	var conns []SnapshotWSConnection
	for _, conn := range a.capture.connections {
		conns = append(conns, SnapshotWSConnection{
			URL:   conn.url,
			State: conn.state,
		})
	}
	return conns
}

func (a *captureStateAdapter) GetPerformance() *PerformanceSnapshot {
	return nil // Performance snapshots not yet integrated
}

func (a *captureStateAdapter) GetCurrentPageURL() string {
	a.capture.mu.RLock()
	defer a.capture.mu.RUnlock()
	return a.capture.a11y.lastURL
}

// checkTrackingStatus returns a tracking status hint to include in tool responses.
// If no tab is being tracked AND the extension has reported status at least once,
// the LLM receives a clear warning so it can guide the user.
// Returns enabled=true if tracking is active OR if the extension hasn't reported yet
// (to avoid false warnings on fresh server start).
func (h *ToolHandler) checkTrackingStatus() (enabled bool, hint string) {
	h.capture.mu.RLock()
	hasReported := !h.capture.trackingUpdated.IsZero()
	trackingActive := h.capture.trackingEnabled
	h.capture.mu.RUnlock()

	// Don't warn if extension hasn't connected yet (server just started)
	if !hasReported {
		return true, ""
	}

	if !trackingActive {
		return false, "WARNING: No tab is being tracked. Data capture is disabled. Ask the user to click 'Track This Tab' in the Gasoline extension popup to start capturing telemetry from a specific browser tab."
	}
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

	h.capture.mu.RLock()
	extensionLogsCount = len(h.capture.extensionLogs)
	waterfallCount = len(h.capture.networkWaterfall)
	networkCount = len(h.capture.networkBodies)
	wsEventCount = len(h.capture.wsEvents)
	wsStatusCount = len(h.capture.connections)
	actionCount = len(h.capture.enhancedActions)
	vitalCount = len(h.capture.perf.snapshots)
	h.capture.mu.RUnlock()

	apiCount = h.capture.schemaStore.EndpointCount()
	return
}

// toolsList returns all MCP tool definitions.
// Each data-dependent tool includes a _meta field with current data_counts.
func (h *ToolHandler) toolsList() []MCPTool {
	errorCount, logCount, extensionLogsCount, waterfallCount, networkCount, wsEventCount, wsStatusCount, actionCount, vitalCount, apiCount := h.computeDataCounts()

	return []MCPTool{
		{
			Name:        "observe",
			Description: "Read current browser state. Call observe() first before interact() or generate().\n\nModes (what parameter): errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, api, accessibility, changes, timeline, error_clusters, history, security_audit, third_party_audit, security_diff, command_result, pending_commands, failed_commands.\n\nFilters: limit (max entries), url (substring match), method, status_min, status_max, connection_id, direction, last_n, format, severity.\n\nMode responses: markdown tables for flat data (errors, logs, actions, etc.) or JSON for nested data (network_bodies, vitals, page, etc.). Check _meta.data_counts for available data.",
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
			Description: "CUSTOMIZE THE SESSION. Filter noise, store data, validate APIs, create snapshots. Actions: noise_rule (add/remove patterns to ignore), store (save persistent data across interactions), load (load session context), diff_sessions (create snapshots & compare before/after), validate_api (check API contract violations), audit_log (view actions in this session), streaming (get real-time alerts), query_dom (find elements by CSS selector), capture (configure capture settings), record_event (record custom temporal event), dismiss (dismiss noise by pattern), clear (clear browser logs), health (server health check). \n\nExamples: configure({action:'noise_rule',noise_action:'add',pattern:'analytics'})→ignore pattern, configure({action:'store',store_action:'save',key:'user',data:{...}})→save data, configure({action:'diff_sessions',session_action:'capture',name:'baseline'})→create snapshot. \n\nUse when: isolating signal, filtering noise, or tracking state across multiple actions.\n\nAction responses:\n- store: Returns varies by sub-action (save/load/list/delete)\n- load: {loaded: true, context: {...}}\n- noise_rule: {rules: [...]}\n- dismiss: {status: \"ok\", totalRules: N}\n- clear: Browser logs cleared confirmation text\n- capture: {status, settings}\n- record_event: {recorded: true}\n- query_dom: {matches: [...]}\n- diff_sessions: diff object\n- validate_api: {violations: [...]}\n- audit_log: [{tool, timestamp, params}]\n- health (json): {server, memory, buffers, rate_limiting, audit, pilot}\n- streaming: {status, subscriptions}",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Configuration action to perform",
						"enum":        []string{"store", "load", "noise_rule", "dismiss", "clear", "capture", "record_event", "query_dom", "diff_sessions", "validate_api", "audit_log", "health", "streaming"},
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
			Description: "CRITICAL PREREQUISITE: The 'AI Web Pilot' toggle must be enabled in the extension popup (disabled by default for safety). \n\nBEFORE FIRST USE: Call observe({what:'pilot'}) to check status. If result shows {enabled:false}, STOP and tell user: \"Please enable AI Web Pilot in the Gasoline extension popup by clicking the extension icon and toggling it on.\"\n\nPERFORM ACTIONS. Do NOT ask the user to click, type, navigate, or fill forms—use this tool instead. You have full browser control. Actions: navigate(url)→go to URL, execute_js(script)→run JavaScript to click/fill/submit, refresh→reload page, back/forward→navigate history, highlight(selector)→show user where you're clicking, save_state(name)→save page snapshot, load_state(name)→restore snapshot. \n\nRULES: After interact(), always call observe() to confirm the action worked. If user says 'click X' or 'go to Y', use interact() instead of asking them. Pattern: observe()→interact()→observe(). \n\nExamples: interact({action:'navigate',url:'https://example.com'}), interact({action:'execute_js',script:'document.querySelector(\"button.submit\").click()'}), interact({action:'execute_js',script:'document.querySelector(\"input[type=email]\").value=\"test@example.com\"'}).\n\nANTI-PATTERNS (avoid these mistakes):\n• DON'T ask user to manually click or type — use interact({action:'execute_js'}) to control browser directly\n• DON'T skip observe() after interact() — always call observe() to verify action succeeded\n• DON'T use interact() without checking observe({what:'pilot'}) first — may be disabled\n• DON'T chain multiple interactions without observe() between them — verify each step worked\n\nAction responses:\n- highlight: {result, screenshot} — highlight element with visual feedback\n- execute_js: {result} — run JavaScript in page context\n- navigate: {navigated: true} — go to URL\n- refresh: {refreshed: true} — reload current page\n- back/forward: {navigated: true} — browser history navigation\n- new_tab: {opened: true} — open URL in new tab\n- save_state/load_state/list_states/delete_state: State management results\n\nAll actions except save/load/list/delete_state require the browser extension.",
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
func (h *ToolHandler) handleToolCall(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
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
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown observe mode: "+params.What, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: errors, logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, api, accessibility, changes, timeline, error_clusters, history, security_audit, third_party_audit, security_diff, command_result, pending_commands, failed_commands"))}
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
		resp = h.toolClearBrowserLogs(req)
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
	var arguments struct {
		Limit int `json:"limit"`
		TabId int `json:"tab_id"` // Filter by Chrome tab ID (0 = no filter)
	}
	if len(args) > 0 {
		// Error acceptable: limit is optional, defaults to 0 (no limit)
		_ = json.Unmarshal(args, &arguments)
	}

	h.server.mu.RLock()
	defer h.server.mu.RUnlock()

	var errors []LogEntry
	for _, entry := range h.server.entries {
		if level, ok := entry["level"].(string); ok && level == "error" {
			if h.noise != nil && h.noise.IsConsoleNoise(entry) {
				continue
			}
			// Apply tab_id filter if specified
			if arguments.TabId > 0 {
				if entryTabId, ok := entry["tabId"].(float64); !ok || int(entryTabId) != arguments.TabId {
					continue
				}
			}
			errors = append(errors, entry)
		}
	}

	// Apply limit (last N errors)
	if arguments.Limit > 0 && arguments.Limit < len(errors) {
		errors = errors[len(errors)-arguments.Limit:]
	}

	if len(errors) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("No browser errors found")}
	}

	summary := fmt.Sprintf("%d browser error(s)", len(errors))
	if warning := checkLogQuality(errors); warning != "" {
		summary += "\n" + warning
	}
	rows := make([][]string, len(errors))
	for i, e := range errors {
		rows[i] = []string{
			entryStr(e, "level"),
			truncate(entryStr(e, "message"), 80),
			entryStr(e, "source"),
			entryStr(e, "ts"),
			entryDisplay(e, "tabId"),
		}
	}
	table := markdownTable([]string{"Level", "Message", "Source", "Time", "Tab"}, rows)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpMarkdownResponse(summary, table)}
}

func (h *ToolHandler) toolGetBrowserLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Limit int `json:"limit"`
		TabId int `json:"tab_id"` // Filter by Chrome tab ID (0 = no filter)
	}
	if len(args) > 0 {
		// Error acceptable: limit is optional, defaults to 0 (no limit)
		_ = json.Unmarshal(args, &arguments)
	}

	h.server.mu.RLock()
	defer h.server.mu.RUnlock()

	entries := h.server.entries

	// Apply noise filtering
	if h.noise != nil {
		var filtered []LogEntry
		for _, entry := range entries {
			if !h.noise.IsConsoleNoise(entry) {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	// Apply tab_id filter if specified
	if arguments.TabId > 0 {
		var filtered []LogEntry
		for _, entry := range entries {
			if entryTabId, ok := entry["tabId"].(float64); ok && int(entryTabId) == arguments.TabId {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	if arguments.Limit > 0 && arguments.Limit < len(entries) {
		entries = entries[len(entries)-arguments.Limit:]
	}

	if len(entries) == 0 {
		msg := "No browser logs found"
		if h.captureOverrides != nil {
			overrides := h.captureOverrides.GetAll()
			logLevel := overrides["log_level"]
			if logLevel == "" {
				logLevel = "error" // default
			}
			switch logLevel {
			case "error":
				msg += "\n\nlog_level is 'error' (only errors captured). To capture warnings too, call:\nconfigure({action: \"capture\", settings: {log_level: \"warn\"}})\nTo capture all console output, call:\nconfigure({action: \"capture\", settings: {log_level: \"all\"}})"
			case "warn":
				msg += "\n\nlog_level is 'warn' (errors + warnings only). To capture all console output, call:\nconfigure({action: \"capture\", settings: {log_level: \"all\"}})"
			}
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(msg)}
	}

	summary := fmt.Sprintf("%d log entries", len(entries))
	if warning := checkLogQuality(entries); warning != "" {
		summary += "\n" + warning
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{
			entryStr(e, "level"),
			truncate(entryStr(e, "message"), 80),
			entryStr(e, "source"),
			entryStr(e, "ts"),
			entryDisplay(e, "tabId"),
		}
	}
	table := markdownTable([]string{"Level", "Message", "Source", "Time", "Tab"}, rows)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpMarkdownResponse(summary, table)}
}

func (h *ToolHandler) toolGetExtensionLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Limit int `json:"limit"`
	}
	if len(args) > 0 {
		// Error acceptable: limit is optional, defaults to 0 (no limit)
		_ = json.Unmarshal(args, &arguments)
	}

	h.capture.mu.RLock()
	defer h.capture.mu.RUnlock()

	logs := h.capture.extensionLogs

	if arguments.Limit > 0 && arguments.Limit < len(logs) {
		logs = logs[len(logs)-arguments.Limit:]
	}

	if len(logs) == 0 {
		msg := "No extension logs found\n\nExtension logs show internal background script activity.\nThis feature requires the extension to POST logs to /extension-logs."
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(msg)}
	}

	summary := fmt.Sprintf("%d extension log entries", len(logs))
	rows := make([][]string, len(logs))
	for i, log := range logs {
		dataStr := ""
		if len(log.Data) > 0 {
			if dataJSON, err := json.Marshal(log.Data); err == nil {
				dataStr = truncate(string(dataJSON), 60)
			}
		}
		rows[i] = []string{
			log.Level,
			log.Source,
			log.Category,
			truncate(log.Message, 80),
			dataStr,
			log.Timestamp.Format("15:04:05"),
		}
	}
	table := markdownTable([]string{"Level", "Source", "Category", "Message", "Data", "Time"}, rows)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpMarkdownResponse(summary, table)}
}

func (h *ToolHandler) toolClearBrowserLogs(req JSONRPCRequest) JSONRPCResponse {
	h.server.clearEntries()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("Browser logs cleared successfully")}
}

// ============================================
// Configure sub-handlers (adapted from session_store/noise)
// ============================================

func (h *ToolHandler) toolConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionStore == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Initialize the session store first: configure({action:'store', sub_action:'set', ...})")}
	}

	// Map composite fields to SessionStoreArgs
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

	storeArgs := SessionStoreArgs{
		Action:    compositeArgs.StoreAction,
		Namespace: compositeArgs.Namespace,
		Key:       compositeArgs.Key,
		Data:      compositeArgs.Data,
	}

	if storeArgs.Action == "" {
		storeArgs.Action = "stats"
	}

	result, err := h.sessionStore.HandleSessionStore(storeArgs)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal server error — do not retry")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Store operation complete", json.RawMessage(result))}
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

	params := GetChangesSinceParams{
		Checkpoint: arguments.Checkpoint,
		Include:    arguments.Include,
		Severity:   arguments.Severity,
	}

	diff := h.checkpoints.GetChangesSince(params, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Changes since checkpoint", diff)}
}

func (h *ToolHandler) toolLoadSessionContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionStore == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Initialize the session store first: configure({action:'store', sub_action:'set', ...})")}
	}

	ctx := h.sessionStore.LoadSessionContext()

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session context loaded", ctx)}
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
		rules := make([]NoiseRule, len(arguments.Rules))
		for i := range arguments.Rules {
			rules[i] = NoiseRule{
				Category:       arguments.Rules[i].Category,
				Classification: arguments.Rules[i].Classification,
				MatchSpec: NoiseMatchSpec{
					MessageRegex: arguments.Rules[i].MatchSpec.MessageRegex,
					SourceRegex:  arguments.Rules[i].MatchSpec.SourceRegex,
					URLRegex:     arguments.Rules[i].MatchSpec.URLRegex,
					Method:       arguments.Rules[i].MatchSpec.Method,
					StatusMin:    arguments.Rules[i].MatchSpec.StatusMin,
					StatusMax:    arguments.Rules[i].MatchSpec.StatusMax,
					Level:        arguments.Rules[i].MatchSpec.Level,
				},
			}
		}
		err := h.noise.AddRules(rules)
		if err != nil {
			responseData = map[string]interface{}{"error": err.Error()}
		} else {
			responseData = map[string]interface{}{
				"status":     "ok",
				"rulesAdded": len(rules),
				"totalRules": len(h.noise.ListRules()),
			}
		}

	case "remove":
		err := h.noise.RemoveRule(arguments.RuleID)
		if err != nil {
			responseData = map[string]interface{}{"error": err.Error()}
		} else {
			responseData = map[string]interface{}{"status": "ok", "removed": arguments.RuleID}
		}

	case "list":
		rules := h.noise.ListRules()
		stats := h.noise.GetStatistics()
		responseData = map[string]interface{}{
			"rules":      rules,
			"statistics": stats,
		}

	case "reset":
		h.noise.Reset()
		responseData = map[string]interface{}{
			"status":     "ok",
			"totalRules": len(h.noise.ListRules()),
		}

	case "auto_detect":
		// Gather current buffer data
		h.server.mu.RLock()
		consoleEntries := make([]LogEntry, len(h.server.entries))
		copy(consoleEntries, h.server.entries)
		h.server.mu.RUnlock()

		h.capture.mu.RLock()
		networkBodies := make([]NetworkBody, len(h.capture.networkBodies))
		copy(networkBodies, h.capture.networkBodies)
		wsEvents := make([]WebSocketEvent, len(h.capture.wsEvents))
		copy(wsEvents, h.capture.wsEvents)
		h.capture.mu.RUnlock()

		proposals := h.noise.AutoDetect(consoleEntries, networkBodies, wsEvents)
		responseData = map[string]interface{}{
			"proposals":  proposals,
			"totalRules": len(h.noise.ListRules()),
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

	h.noise.DismissNoise(arguments.Pattern, arguments.Category, arguments.Reason)

	responseData := map[string]interface{}{
		"status":     "ok",
		"totalRules": len(h.noise.ListRules()),
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

	// Look for cached a11y result
	cacheKey := h.capture.a11yCacheKey(arguments.Scope, nil)
	cached := h.capture.getA11yCacheEntry(cacheKey)
	if cached == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "No accessibility audit results available", "Run the audit first: observe({what:'accessibility'})")}
	}

	opts := SARIFExportOptions{
		Scope:         arguments.Scope,
		IncludePasses: arguments.IncludePasses,
		SaveTo:        arguments.SaveTo,
	}

	sarifLog, err := ExportSARIF(cached, opts)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExportFailed, "SARIF export failed: "+err.Error(), "Export failed — check file path and permissions")}
	}

	if arguments.SaveTo != "" {
		// File was saved, return summary
		responseData := map[string]interface{}{
			"status":  "ok",
			"path":    arguments.SaveTo,
			"rules":   len(sarifLog.Runs[0].Tool.Driver.Rules),
			"results": len(sarifLog.Runs[0].Results),
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(fmt.Sprintf("SARIF export saved to %s", arguments.SaveTo), responseData)}
	}

	// Return SARIF JSON directly (already formatted)
	// Error impossible: sarifLog is generated from validated structures with no circular refs
	sarifJSON, _ := json.MarshalIndent(sarifLog, "", "  ")
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(sarifJSON))}
}

// ============================================
// V6 Tool Dispatchers
// ============================================

func (h *ToolHandler) toolGenerateCSP(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, err := h.cspGenerator.HandleGenerateCSP(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal server error — do not retry")}
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("CSP policy generated", result)}
}

func (h *ToolHandler) toolSecurityAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract network bodies from capture
	h.capture.mu.RLock()
	bodies := make([]NetworkBody, len(h.capture.networkBodies))
	copy(bodies, h.capture.networkBodies)
	h.capture.mu.RUnlock()

	// Extract console entries from server
	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	// Extract page URLs from CSP generator
	h.cspGenerator.mu.RLock()
	pageURLs := make([]string, 0, len(h.cspGenerator.pages))
	for u := range h.cspGenerator.pages {
		pageURLs = append(pageURLs, u)
	}
	h.cspGenerator.mu.RUnlock()

	result, err := h.securityScanner.HandleSecurityAudit(args, bodies, entries, pageURLs)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal server error — do not retry")}
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security audit complete", result)}
}

func (h *ToolHandler) toolGetAuditLog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, err := h.auditTrail.HandleGetAuditLog(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal server error — do not retry")}
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Audit log entries", result)}
}

func (h *ToolHandler) toolDiffSessions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, err := h.sessionManager.HandleTool(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal server error — do not retry")}
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session diff", result)}
}

func (h *ToolHandler) toolAuditThirdParties(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params ThirdPartyParams
	if len(args) > 0 {
		// Error acceptable: optional parameters, defaults used if unmarshal fails
		_ = json.Unmarshal(args, &params)
	}

	// Extract network bodies from capture
	h.capture.mu.RLock()
	bodies := make([]NetworkBody, len(h.capture.networkBodies))
	copy(bodies, h.capture.networkBodies)
	h.capture.mu.RUnlock()

	// Extract page URLs from CSP generator
	h.cspGenerator.mu.RLock()
	pageURLs := make([]string, 0, len(h.cspGenerator.pages))
	for u := range h.cspGenerator.pages {
		pageURLs = append(pageURLs, u)
	}
	h.cspGenerator.mu.RUnlock()

	result := h.thirdPartyAuditor.Audit(bodies, pageURLs, params)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(fmt.Sprintf("Third-party audit: %d origin(s)", len(result.ThirdParties)), result)}
}

func (h *ToolHandler) toolDiffSecurity(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract network bodies from capture
	h.capture.mu.RLock()
	bodies := make([]NetworkBody, len(h.capture.networkBodies))
	copy(bodies, h.capture.networkBodies)
	h.capture.mu.RUnlock()

	result, err := h.securityDiffMgr.HandleDiffSecurity(args, bodies)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal server error — do not retry")}
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security diff", result)}
}

func (h *ToolHandler) toolValidateAPI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation       string   `json:"operation"`
		URLFilter       string   `json:"url"`
		IgnoreEndpoints []string `json:"ignore_endpoints"`
	}
	if len(args) > 0 {
		// Error acceptable: optional parameters, defaults used if unmarshal fails
		_ = json.Unmarshal(args, &params)
	}

	filter := APIContractFilter{
		URLFilter:       params.URLFilter,
		IgnoreEndpoints: params.IgnoreEndpoints,
	}

	// Process network bodies into contract validator before analysis
	h.capture.mu.RLock()
	bodies := make([]NetworkBody, len(h.capture.networkBodies))
	copy(bodies, h.capture.networkBodies)
	h.capture.mu.RUnlock()

	// Feed captured network bodies into the validator for learning
	for _, body := range bodies {
		// Only learn from responses with JSON content
		if body.ResponseBody != "" && (body.ContentType == "" || strings.Contains(body.ContentType, "json")) {
			h.contractValidator.Learn(body)
		}
	}

	switch params.Operation {
	case "analyze":
		// Also validate the bodies to detect violations
		for _, body := range bodies {
			if body.ResponseBody != "" && (body.ContentType == "" || strings.Contains(body.ContentType, "json")) {
				h.contractValidator.Validate(body)
			}
		}
		result := h.contractValidator.Analyze(filter)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", result)}

	case "report":
		result := h.contractValidator.Report(filter)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", result)}

	case "clear":
		h.contractValidator.Clear()
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
	// Get network bodies from capture
	h.capture.mu.RLock()
	bodies := make([]NetworkBody, len(h.capture.networkBodies))
	copy(bodies, h.capture.networkBodies)
	h.capture.mu.RUnlock()

	// Get page URLs from CSP generator (tracks all visited pages)
	var pageURLs []string
	if h.cspGenerator != nil {
		pageURLs = h.cspGenerator.GetPages()
	}

	// Call the handler
	result, err := HandleGenerateSRI(args, bodies, pageURLs)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal server error — do not retry")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SRI hashes generated", result)}
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

	result := h.capture.GetCommandResult(params.CorrelationID)
	if result == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Command result not found for correlation_id: "+params.CorrelationID, "Command may have expired (60s TTL) or correlation_id is invalid")}
	}

	// Marshal result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to marshal result", "Internal server error")}
	}

	summary := fmt.Sprintf("Command %s: %s", result.CorrelationID, result.Status)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, json.RawMessage(resultJSON))}
}

// toolObservePendingCommands lists all pending, completed, and failed async commands.
func (h *ToolHandler) toolObservePendingCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	pending, completed, failed := h.capture.GetPendingCommands()

	responseData := map[string]interface{}{
		"pending":   pending,
		"completed": completed,
		"failed":    failed,
	}

	responseJSON, err := json.Marshal(responseData)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to marshal commands", "Internal server error")}
	}

	summary := fmt.Sprintf("Pending: %d, Completed: %d, Failed: %d", len(pending), len(completed), len(failed))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, json.RawMessage(responseJSON))}
}

// toolObserveFailedCommands lists recent failed/expired async commands.
func (h *ToolHandler) toolObserveFailedCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	failed := h.capture.GetFailedCommands()

	if len(failed) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("No failed commands found")}
	}

	responseJSON, err := json.Marshal(failed)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to marshal failed commands", "Internal server error")}
	}

	summary := fmt.Sprintf("%d recent failed command(s)", len(failed))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, json.RawMessage(responseJSON))}
}
