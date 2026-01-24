// tools.go — MCP tool definitions, dispatch, and response helpers.
// Defines 5 composite tools (observe, analyze, generate, configure, query_dom)
// that replace the original 24 granular tools, reducing AI decision space by 79%.
// Each tool has a mode parameter that selects the sub-operation.
// Design: Tool schemas include live data_counts in _meta so the AI knows what
// data is available before calling. Dispatch is a single switch on tool name +
// mode parameter, keeping the handler flat and predictable.
package main

import (
	"encoding/json"
	"os"
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
	capture      *Capture
	checkpoints  *CheckpointManager
	sessionStore *SessionStore
	noise        *NoiseConfig
	AlertBuffer  // Embedded alert state for push-based notifications
}

// NewToolHandler creates an MCP handler with composite tool capabilities
func NewToolHandler(server *Server, capture *Capture) *MCPHandler {
	handler := &ToolHandler{
		MCPHandler:  NewMCPHandler(server),
		capture:     capture,
		checkpoints: NewCheckpointManager(server, capture),
		noise:       NewNoiseConfig(),
	}

	// Initialize persistent session store using CWD as project root.
	// If initialization fails (e.g., read-only filesystem), the server
	// continues without persistence — tool handlers check for nil.
	if cwd, err := os.Getwd(); err == nil {
		if store, err := NewSessionStore(cwd); err == nil {
			handler.sessionStore = store
		}
	}

	// Return as MCPHandler but with overridden methods via the wrapper
	return &MCPHandler{
		server:      server,
		toolHandler: handler,
	}
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

// toolsList returns the list of composite tools (5 tools replacing the 24 granular ones).
// This design reduces AI decision space by 79%, improving tool selection accuracy.
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


