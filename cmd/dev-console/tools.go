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
	"os"
	"path/filepath"
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
	Tools MCPToolsCapability `json:"tools"`
}

// MCPToolsCapability declares tool support.
type MCPToolsCapability struct{}

// MCPToolsListResult represents the result of a tools/list request.
type MCPToolsListResult struct {
	Tools []MCPTool `json:"tools"`
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
	resultJSON, _ := json.Marshal(result)
	return json.RawMessage(resultJSON)
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

	// Redaction engine for scrubbing sensitive data from tool responses
	redactionEngine *RedactionEngine
}

// NewToolHandler creates an MCP handler with composite tool capabilities
func NewToolHandler(server *Server, capture *Capture) *MCPHandler {
	handler := &ToolHandler{
		MCPHandler:       NewMCPHandler(server),
		capture:          capture,
		checkpoints:      NewCheckpointManager(server, capture),
		noise:            NewNoiseConfig(),
		captureOverrides: NewCaptureOverrides(),
		clusters:         NewClusterManager(),
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

	// Wire error clustering: feed error-level log entries into the cluster manager.
	server.onEntries = func(entries []LogEntry) {
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
	}

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
	for url, conn := range a.capture.connections {
		conns = append(conns, SnapshotWSConnection{
			URL:   url,
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

// computeDataCounts reads current buffer sizes from server and capture under read locks.
// Returns counts for each observable mode.
func (h *ToolHandler) computeDataCounts() (errorCount, logCount, networkCount, wsEventCount, wsStatusCount, actionCount, vitalCount, apiCount int) {
	h.MCPHandler.server.mu.RLock()
	logCount = len(h.MCPHandler.server.entries)
	for _, entry := range h.MCPHandler.server.entries {
		if level, ok := entry["level"].(string); ok && level == "error" {
			errorCount++
		}
	}
	h.MCPHandler.server.mu.RUnlock()

	h.capture.mu.RLock()
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
	errorCount, logCount, networkCount, wsEventCount, wsStatusCount, actionCount, vitalCount, apiCount := h.computeDataCounts()

	return []MCPTool{
		{
			Name:        "observe",
			Description: "Observe browser state: errors, logs, network traffic, WebSocket events, user actions, web vitals, or page info. Use the 'what' parameter to select what to observe.",
			Meta: map[string]interface{}{
				"data_counts": map[string]interface{}{
					"errors":           errorCount,
					"logs":             logCount,
					"network":          networkCount,
					"websocket_events": wsEventCount,
					"websocket_status": wsStatusCount,
					"actions":          actionCount,
					"vitals":           vitalCount,
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"what": map[string]interface{}{
						"type":        "string",
						"description": "What to observe",
						"enum":        []string{"errors", "logs", "network", "websocket_events", "websocket_status", "actions", "vitals", "page"},
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum entries to return (applies to logs, network, websocket_events, actions)",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring (applies to network, websocket_events, websocket_status, actions)",
					},
					"method": map[string]interface{}{
						"type":        "string",
						"description": "Filter by HTTP method (applies to network)",
					},
					"status_min": map[string]interface{}{
						"type":        "number",
						"description": "Minimum status code (applies to network)",
					},
					"status_max": map[string]interface{}{
						"type":        "number",
						"description": "Maximum status code (applies to network)",
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
						"description": "Return only the last N items (applies to actions)",
					},
				},
				"required": []string{"what"},
			},
		},
		{
			Name:        "analyze",
			Description: "Analyze browser data: performance metrics, API schema, accessibility audit, changes since checkpoint, or session timeline.",
			Meta: map[string]interface{}{
				"data_counts": map[string]interface{}{
					"performance": vitalCount,
					"api":         apiCount,
					"timeline":    actionCount,
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]interface{}{
						"type":        "string",
						"description": "What to analyze",
						"enum":        []string{"performance", "api", "accessibility", "changes", "timeline"},
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL path to check (applies to performance)",
					},
					"url_filter": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring (applies to api, timeline)",
					},
					"min_observations": map[string]interface{}{
						"type":        "number",
						"description": "Minimum times an endpoint must be observed (applies to api)",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "Output format for API schema: gasoline or openapi_stub (applies to api)",
						"enum":        []string{"gasoline", "openapi_stub"},
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector to scope the audit (applies to accessibility)",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "WCAG tags to test (applies to accessibility)",
						"items":       map[string]interface{}{"type": "string"},
					},
					"force_refresh": map[string]interface{}{
						"type":        "boolean",
						"description": "Bypass cache and re-run (applies to accessibility)",
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
					"last_n_actions": map[string]interface{}{
						"type":        "number",
						"description": "Only include the last N actions (applies to timeline)",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			Name:        "generate",
			Description: "Generate artifacts from captured data: reproduction scripts, Playwright tests, PR summaries, SARIF reports, or HAR archives.",
			Meta: map[string]interface{}{
				"data_counts": map[string]interface{}{
					"reproduction": actionCount,
					"test":         actionCount,
					"har":          networkCount,
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"format": map[string]interface{}{
						"type":        "string",
						"description": "What to generate",
						"enum":        []string{"reproduction", "test", "pr_summary", "sarif", "har"},
					},
					"error_message": map[string]interface{}{
						"type":        "string",
						"description": "Error message for context (applies to reproduction)",
					},
					"last_n_actions": map[string]interface{}{
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
				},
				"required": []string{"format"},
			},
		},
		{
			Name:        "configure",
			Description: "Configure the session: store/load persistent data, manage noise filtering rules, dismiss noise patterns, or clear browser logs.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Configuration action to perform",
						"enum":        []string{"store", "load", "noise_rule", "dismiss", "clear"},
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
								"matchSpec": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"messageRegex": map[string]interface{}{"type": "string"},
										"sourceRegex":  map[string]interface{}{"type": "string"},
										"urlRegex":     map[string]interface{}{"type": "string"},
										"method":       map[string]interface{}{"type": "string"},
										"statusMin":    map[string]interface{}{"type": "number"},
										"statusMax":    map[string]interface{}{"type": "number"},
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
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "query_dom",
			Description: "Query the live DOM in the browser using a CSS selector. Returns matching elements.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector to query",
					},
				},
				"required": []string{"selector"},
			},
		},
		{
			Name:        "generate_csp",
			Description: "Generate a Content-Security-Policy header from observed network origins. Accumulates origins over time and produces a CSP with confidence scoring.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "CSP strictness mode",
						"enum":        []string{"strict", "moderate", "report_only"},
					},
					"include_report_uri": map[string]interface{}{
						"type":        "boolean",
						"description": "Include report-uri directive in the generated CSP",
					},
					"exclude_origins": map[string]interface{}{
						"type":        "array",
						"description": "Origins to exclude from the generated CSP",
						"items":       map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		{
			Name:        "security_audit",
			Description: "Scan captured network traffic and console logs for security issues: exposed credentials, PII leakage, missing security headers, insecure cookies, transport security, and auth patterns.",
			Meta: map[string]interface{}{
				"data_counts": map[string]interface{}{
					"network_bodies": networkCount,
					"console_logs":   logCount,
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"checks": map[string]interface{}{
						"type":        "array",
						"description": "Which checks to run (default: all)",
						"items": map[string]interface{}{
							"type": "string",
							"enum": []string{"credentials", "pii", "headers", "cookies", "transport", "auth"},
						},
					},
					"url_filter": map[string]interface{}{
						"type":        "string",
						"description": "Only scan requests matching this URL substring",
					},
					"severity_min": map[string]interface{}{
						"type":        "string",
						"description": "Minimum severity to report",
						"enum":        []string{"critical", "high", "medium", "low", "info"},
					},
				},
			},
		},
		{
			Name:        "audit_third_parties",
			Description: "Audit third-party origins in captured network traffic. Classifies risk levels (critical/high/medium/low), detects data exfiltration, suspicious domains (DGA, abuse TLDs), and provides recommendations.",
			Meta: map[string]interface{}{
				"data_counts": map[string]interface{}{
					"network_bodies": networkCount,
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"first_party_origins": map[string]interface{}{
						"type":        "array",
						"description": "Origins to consider first-party (auto-detected from page URLs if omitted)",
						"items":       map[string]interface{}{"type": "string"},
					},
					"include_static": map[string]interface{}{
						"type":        "boolean",
						"description": "Include low-risk static-only origins (images, fonts) in results (default: true)",
					},
					"custom_lists": map[string]interface{}{
						"type":        "object",
						"description": "Custom allowed/blocked/internal domain lists",
						"properties": map[string]interface{}{
							"allowed":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
							"blocked":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
							"internal": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						},
					},
				},
			},
		},
		{
			Name:        "diff_security",
			Description: "Detect security regressions by comparing security posture snapshots. Captures headers, cookies, auth, and transport state; compares two snapshots to find regressions and improvements.",
			Meta: map[string]interface{}{
				"data_counts": map[string]interface{}{
					"network_bodies": networkCount,
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform",
						"enum":        []string{"snapshot", "compare", "list"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Snapshot name (required for 'snapshot' action)",
					},
					"compare_from": map[string]interface{}{
						"type":        "string",
						"description": "Baseline snapshot name (required for 'compare' action)",
					},
					"compare_to": map[string]interface{}{
						"type":        "string",
						"description": "Target snapshot name to compare against (omit to use current live state)",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "get_audit_log",
			Description: "Query the enterprise audit trail of MCP tool invocations. Returns entries filtered by session, tool name, or time range.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by MCP session ID",
					},
					"tool_name": map[string]interface{}{
						"type":        "string",
						"description": "Filter by tool name",
					},
					"since": map[string]interface{}{
						"type":        "string",
						"description": "Only return entries after this ISO 8601 timestamp",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of entries to return (default: 100)",
					},
				},
			},
		},
		{
			Name:        "diff_sessions",
			Description: "Capture browser state snapshots and compare them to detect regressions. Supports capture, compare, list, and delete actions.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform",
						"enum":        []string{"capture", "compare", "list", "delete"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Snapshot name (required for capture and delete)",
					},
					"compare_a": map[string]interface{}{
						"type":        "string",
						"description": "First snapshot name for comparison",
					},
					"compare_b": map[string]interface{}{
						"type":        "string",
						"description": "Second snapshot name for comparison",
					},
					"url_filter": map[string]interface{}{
						"type":        "string",
						"description": "Only include network requests matching this URL substring",
					},
				},
				"required": []string{"action"},
			},
		},
		// AI Web Pilot tools (Phase 1: stubs only)
		{
			Name:        "highlight_element",
			Description: "Highlight a DOM element on the page. Requires 'AI Web Pilot' to be enabled in the extension popup.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector for the element to highlight",
					},
					"duration_ms": map[string]interface{}{
						"type":        "number",
						"description": "How long to show the highlight in milliseconds (default: 5000)",
					},
				},
				"required": []string{"selector"},
			},
		},
		{
			Name:        "manage_state",
			Description: "Save, load, list, or delete page state snapshots. Requires 'AI Web Pilot' to be enabled in the extension popup.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform",
						"enum":        []string{"save", "load", "list", "delete"},
					},
					"snapshot_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the snapshot (required for save, load, delete)",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "execute_javascript",
			Description: "Execute JavaScript code in the page context. Requires 'AI Web Pilot' to be enabled in the extension popup.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"script": map[string]interface{}{
						"type":        "string",
						"description": "JavaScript code to execute",
					},
					"timeout_ms": map[string]interface{}{
						"type":        "number",
						"description": "Execution timeout in milliseconds (default: 5000)",
					},
				},
				"required": []string{"script"},
			},
		},
	}
}

// handleToolCall dispatches composite tool calls by mode parameter.
func (h *ToolHandler) handleToolCall(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
	switch name {
	case "observe":
		return h.toolObserve(req, args), true
	case "analyze":
		return h.toolAnalyze(req, args), true
	case "generate":
		return h.toolGenerate(req, args), true
	case "configure":
		return h.toolConfigure(req, args), true
	case "query_dom":
		return h.toolQueryDOM(req, args), true
	case "generate_csp":
		return h.toolGenerateCSP(req, args), true
	case "security_audit":
		return h.toolSecurityAudit(req, args), true
	case "audit_third_parties":
		return h.toolAuditThirdParties(req, args), true
	case "diff_security":
		return h.toolDiffSecurity(req, args), true
	case "get_audit_log":
		return h.toolGetAuditLog(req, args), true
	case "diff_sessions":
		return h.toolDiffSessions(req, args), true
	// AI Web Pilot tools
	case "highlight_element":
		return h.handlePilotHighlight(req, args), true
	case "manage_state":
		return h.handlePilotManageState(req, args), true
	case "execute_javascript":
		return h.handlePilotExecuteJS(req, args), true
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
	_ = json.Unmarshal(args, &params)

	if params.What == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Required parameter 'what' is missing")}
	}

	var resp JSONRPCResponse
	switch params.What {
	case "errors":
		resp = h.toolGetBrowserErrors(req, args)
	case "logs":
		resp = h.toolGetBrowserLogs(req, args)
	case "network":
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
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Unknown observe mode: " + params.What)}
	}

	// Piggyback alerts: append as second content block if any pending
	alerts := h.drainAlerts()
	if len(alerts) > 0 {
		resp = h.appendAlertsToResponse(resp, alerts)
	}

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

	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

func (h *ToolHandler) toolAnalyze(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Target string `json:"target"`
	}
	_ = json.Unmarshal(args, &params)

	if params.Target == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Required parameter 'target' is missing")}
	}

	switch params.Target {
	case "performance":
		return h.toolCheckPerformance(req, args)
	case "api":
		return h.toolGetAPISchema(req, args)
	case "accessibility":
		return h.toolRunA11yAudit(req, args)
	case "changes":
		return h.toolGetChangesSince(req, args)
	case "timeline":
		return h.toolGetSessionTimeline(req, args)
	case "errors":
		return h.toolAnalyzeErrors(req)
	case "history":
		return h.toolAnalyzeHistory(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Unknown analyze target: " + params.Target)}
	}
}

func (h *ToolHandler) toolGenerate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Format string `json:"format"`
	}
	_ = json.Unmarshal(args, &params)

	if params.Format == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Required parameter 'format' is missing")}
	}

	switch params.Format {
	case "reproduction":
		return h.toolGetReproductionScript(req, args)
	case "test":
		return h.toolGenerateTest(req, args)
	case "pr_summary":
		return h.toolGeneratePRSummary(req, args)
	case "sarif":
		return h.toolExportSARIF(req, args)
	case "har":
		return h.toolExportHAR(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Unknown generate format: " + params.Format)}
	}
}

func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal(args, &params)

	if params.Action == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Required parameter 'action' is missing")}
	}

	switch params.Action {
	case "store":
		return h.toolConfigureStore(req, args)
	case "load":
		return h.toolLoadSessionContext(req, args)
	case "noise_rule":
		return h.toolConfigureNoiseRule(req, args)
	case "dismiss":
		return h.toolConfigureDismiss(req, args)
	case "clear":
		return h.toolClearBrowserLogs(req)
	case "capture":
		return h.toolConfigureCapture(req, args)
	case "record_event":
		return h.toolConfigureRecordEvent(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Unknown configure action: " + params.Action)}
	}
}

// ============================================
// Observe sub-handlers (browser errors/logs moved from main.go)
// ============================================

func (h *ToolHandler) toolGetBrowserErrors(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	h.MCPHandler.server.mu.RLock()
	defer h.MCPHandler.server.mu.RUnlock()

	var errors []LogEntry
	for _, entry := range h.MCPHandler.server.entries {
		if level, ok := entry["level"].(string); ok && level == "error" {
			if h.noise != nil && h.noise.IsConsoleNoise(entry) {
				continue
			}
			errors = append(errors, entry)
		}
	}

	if len(errors) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("No browser errors found")}
	}

	errorsJSON, _ := json.Marshal(errors)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(errorsJSON))}
}

func (h *ToolHandler) toolGetBrowserLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(args, &arguments)

	h.MCPHandler.server.mu.RLock()
	defer h.MCPHandler.server.mu.RUnlock()

	entries := h.MCPHandler.server.entries

	if h.noise != nil {
		var filtered []LogEntry
		for _, entry := range entries {
			if !h.noise.IsConsoleNoise(entry) {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	if arguments.Limit > 0 && arguments.Limit < len(entries) {
		entries = entries[len(entries)-arguments.Limit:]
	}

	if len(entries) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("No browser logs found")}
	}

	entriesJSON, _ := json.Marshal(entries)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(entriesJSON))}
}

func (h *ToolHandler) toolClearBrowserLogs(req JSONRPCRequest) JSONRPCResponse {
	h.MCPHandler.server.clearEntries()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("Browser logs cleared successfully")}
}

// ============================================
// Configure sub-handlers (adapted from session_store/noise)
// ============================================

func (h *ToolHandler) toolConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionStore == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Session store not initialized")}
	}

	// Map composite fields to SessionStoreArgs
	var compositeArgs struct {
		StoreAction string          `json:"store_action"`
		Namespace   string          `json:"namespace"`
		Key         string          `json:"key"`
		Data        json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(args, &compositeArgs)

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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Error: " + err.Error())}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(result))}
}

func (h *ToolHandler) toolConfigureNoiseRule(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract the noise_action field as the action for configure_noise
	var compositeArgs struct {
		NoiseAction string `json:"noise_action"`
	}
	_ = json.Unmarshal(args, &compositeArgs)

	// Rewrite args to have "action" field that toolConfigureNoise expects
	var rawMap map[string]interface{}
	_ = json.Unmarshal(args, &rawMap)
	rawMap["action"] = compositeArgs.NoiseAction
	if rawMap["action"] == "" {
		rawMap["action"] = "list"
	}
	rewrittenArgs, _ := json.Marshal(rawMap)

	return h.toolConfigureNoise(req, rewrittenArgs)
}

func (h *ToolHandler) toolConfigureDismiss(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.toolDismissNoise(req, args)
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
	_ = json.Unmarshal(args, &arguments)

	params := GetChangesSinceParams{
		Checkpoint: arguments.Checkpoint,
		Include:    arguments.Include,
		Severity:   arguments.Severity,
	}

	diff := h.checkpoints.GetChangesSince(params)

	diffJSON, _ := json.Marshal(diff)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(diffJSON))}
}

func (h *ToolHandler) toolLoadSessionContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionStore == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Session store not initialized")}
	}

	ctx := h.sessionStore.LoadSessionContext()
	ctxJSON, _ := json.Marshal(ctx)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(ctxJSON))}
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
				MessageRegex string `json:"messageRegex"`
				SourceRegex  string `json:"sourceRegex"`
				URLRegex     string `json:"urlRegex"`
				Method       string `json:"method"`
				StatusMin    int    `json:"statusMin"`
				StatusMax    int    `json:"statusMax"`
				Level        string `json:"level"`
			} `json:"matchSpec"`
		} `json:"rules"`
		RuleID string `json:"rule_id"`
	}
	_ = json.Unmarshal(args, &arguments)

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

	respJSON, _ := json.Marshal(responseData)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
}

func (h *ToolHandler) toolDismissNoise(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Pattern  string `json:"pattern"`
		Category string `json:"category"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal(args, &arguments)

	h.noise.DismissNoise(arguments.Pattern, arguments.Category, arguments.Reason)

	responseData := map[string]interface{}{
		"status":     "ok",
		"totalRules": len(h.noise.ListRules()),
	}
	respJSON, _ := json.Marshal(responseData)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
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
	_ = json.Unmarshal(args, &arguments)

	// Look for cached a11y result
	cacheKey := h.capture.a11yCacheKey(arguments.Scope, nil)
	cached := h.capture.getA11yCacheEntry(cacheKey)
	if cached == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("No accessibility audit results available. Run run_accessibility_audit first.")}
	}

	opts := SARIFExportOptions{
		Scope:         arguments.Scope,
		IncludePasses: arguments.IncludePasses,
		SaveTo:        arguments.SaveTo,
	}

	sarifLog, err := ExportSARIF(cached, opts)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("SARIF export failed: " + err.Error())}
	}

	if arguments.SaveTo != "" {
		// File was saved, return summary
		responseData := map[string]interface{}{
			"status":  "ok",
			"path":    arguments.SaveTo,
			"rules":   len(sarifLog.Runs[0].Tool.Driver.Rules),
			"results": len(sarifLog.Runs[0].Results),
		}
		respJSON, _ := json.Marshal(responseData)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
	}

	// Return SARIF JSON directly
	sarifJSON, _ := json.MarshalIndent(sarifLog, "", "  ")
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(sarifJSON))}
}

// ============================================
// V6 Tool Dispatchers
// ============================================

func (h *ToolHandler) toolGenerateCSP(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, err := h.cspGenerator.HandleGenerateCSP(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(err.Error())}
	}
	respJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
}

func (h *ToolHandler) toolSecurityAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract network bodies from capture
	h.capture.mu.RLock()
	bodies := make([]NetworkBody, len(h.capture.networkBodies))
	copy(bodies, h.capture.networkBodies)
	h.capture.mu.RUnlock()

	// Extract console entries from server
	h.MCPHandler.server.mu.RLock()
	entries := make([]LogEntry, len(h.MCPHandler.server.entries))
	copy(entries, h.MCPHandler.server.entries)
	h.MCPHandler.server.mu.RUnlock()

	// Extract page URLs from CSP generator
	h.cspGenerator.mu.RLock()
	pageURLs := make([]string, 0, len(h.cspGenerator.pages))
	for u := range h.cspGenerator.pages {
		pageURLs = append(pageURLs, u)
	}
	h.cspGenerator.mu.RUnlock()

	result, err := h.securityScanner.HandleSecurityAudit(args, bodies, entries, pageURLs)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(err.Error())}
	}
	respJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
}

func (h *ToolHandler) toolGetAuditLog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, err := h.auditTrail.HandleGetAuditLog(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(err.Error())}
	}
	respJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
}

func (h *ToolHandler) toolDiffSessions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, err := h.sessionManager.HandleTool(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(err.Error())}
	}
	respJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
}

func (h *ToolHandler) toolAuditThirdParties(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params ThirdPartyParams
	if len(args) > 0 {
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
	respJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
}

func (h *ToolHandler) toolDiffSecurity(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract network bodies from capture
	h.capture.mu.RLock()
	bodies := make([]NetworkBody, len(h.capture.networkBodies))
	copy(bodies, h.capture.networkBodies)
	h.capture.mu.RUnlock()

	result, err := h.securityDiffMgr.HandleDiffSecurity(args, bodies)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(err.Error())}
	}
	respJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
}
