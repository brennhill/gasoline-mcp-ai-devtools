package main

import "encoding/json"

// ============================================
// MCP Handler V4
// ============================================

// MCPHandlerV4 extends MCPHandler with v4 tools
type MCPHandlerV4 struct {
	*MCPHandler
	v4          *V4Server
	checkpoints *CheckpointManager
}

// NewMCPHandlerV4 creates an MCP handler with v4 capabilities
func NewMCPHandlerV4(server *Server, v4 *V4Server) *MCPHandler {
	handler := &MCPHandlerV4{
		MCPHandler:  NewMCPHandler(server),
		v4:          v4,
		checkpoints: NewCheckpointManager(server, v4),
	}
	// Return as MCPHandler but with overridden methods via the wrapper
	return &MCPHandler{
		server:      server,
		initialized: false,
		v4Handler:   handler,
	}
}

// v4ToolsList returns the list of v4 tools
func (h *MCPHandlerV4) v4ToolsList() []MCPTool {
	return []MCPTool{
		{
			Name:        "get_websocket_events",
			Description: "Get captured WebSocket events (messages, lifecycle). Useful for debugging real-time communication.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"connection_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by connection ID",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"direction": map[string]interface{}{
						"type":        "string",
						"description": "Filter by direction (incoming/outgoing)",
						"enum":        []string{"incoming", "outgoing"},
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum events to return (default: 50)",
					},
				},
			},
		},
		{
			Name:        "get_websocket_status",
			Description: "Get current WebSocket connection states, rates, and schemas.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"connection_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by connection ID",
					},
				},
			},
		},
		{
			Name:        "get_network_bodies",
			Description: "Get captured network request/response bodies.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"method": map[string]interface{}{
						"type":        "string",
						"description": "Filter by HTTP method",
					},
					"status_min": map[string]interface{}{
						"type":        "number",
						"description": "Minimum status code",
					},
					"status_max": map[string]interface{}{
						"type":        "number",
						"description": "Maximum status code",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum entries to return (default: 20)",
					},
				},
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
			Name:        "get_page_info",
			Description: "Get information about the current page (URL, title, viewport).",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "run_accessibility_audit",
			Description: "Run an accessibility audit on the current page or a scoped element.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector to scope the audit",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "WCAG tags to test (e.g., wcag2a, wcag2aa)",
						"items":       map[string]interface{}{"type": "string"},
					},
					"force_refresh": map[string]interface{}{
						"type":        "boolean",
						"description": "Bypass cache and re-run the audit",
					},
				},
			},
		},
		{
			Name:        "get_enhanced_actions",
			Description: "Get captured user actions with multi-strategy selectors. Useful for understanding what the user did before an error occurred.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"last_n": map[string]interface{}{
						"type":        "number",
						"description": "Return only the last N actions (default: all)",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring",
					},
				},
			},
		},
		{
			Name:        "get_reproduction_script",
			Description: "Generate a Playwright test script from captured user actions. Useful for reproducing bugs.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"error_message": map[string]interface{}{
						"type":        "string",
						"description": "Error message to include in the test (adds context comment)",
					},
					"last_n_actions": map[string]interface{}{
						"type":        "number",
						"description": "Use only the last N actions (default: all)",
					},
					"base_url": map[string]interface{}{
						"type":        "string",
						"description": "Replace the origin in URLs (e.g., 'https://staging.example.com')",
					},
				},
			},
		},
		{
			Name:        "check_performance",
			Description: "Get a performance snapshot of the current page including load timing, network weight, main-thread blocking, and regression detection against baselines.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL path to check (default: latest snapshot)",
					},
				},
			},
		},
		{
			Name:        "get_session_timeline",
			Description: "Get a unified timeline of user actions, network requests, and console errors sorted chronologically.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"last_n_actions": map[string]interface{}{
						"type":        "number",
						"description": "Only include the last N actions and events after them",
					},
					"url_filter": map[string]interface{}{
						"type":        "string",
						"description": "Filter entries by URL substring",
					},
					"include": map[string]interface{}{
						"type":        "array",
						"description": "Entry types to include: actions, network, console (default: all)",
						"items":       map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		{
			Name:        "generate_test",
			Description: "Generate a Playwright test from the session timeline with configurable assertions.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"test_name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the generated test",
					},
					"assert_network": map[string]interface{}{
						"type":        "boolean",
						"description": "Include network response assertions",
					},
					"assert_no_errors": map[string]interface{}{
						"type":        "boolean",
						"description": "Assert no console errors occurred",
					},
					"assert_response_shape": map[string]interface{}{
						"type":        "boolean",
						"description": "Assert response body shape matches",
					},
					"base_url": map[string]interface{}{
						"type":        "string",
						"description": "Replace origin in URLs",
					},
				},
			},
		},
		{
			Name:        "get_changes_since",
			Description: "Get a compressed diff of browser activity since the last checkpoint. Returns only new console errors, network failures, WebSocket disconnections, and user actions — deduplicated and severity-ranked. Call with no arguments for auto-advancing behavior, or pass a named checkpoint to compare against a stable reference point.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"checkpoint": map[string]interface{}{
						"type":        "string",
						"description": "Named checkpoint, ISO 8601 timestamp, or omit for auto-advance. Named checkpoints persist across calls.",
					},
					"include": map[string]interface{}{
						"type":        "array",
						"description": "Categories to include: console, network, websocket, actions. Omit for all.",
						"items":       map[string]interface{}{"type": "string"},
					},
					"severity": map[string]interface{}{
						"type":        "string",
						"description": "Minimum severity: all (default), warnings, errors_only",
						"enum":        []string{"all", "warnings", "errors_only"},
					},
				},
			},
		},
	}
}

// handleV4ToolCall handles a v4-specific tool call
func (h *MCPHandlerV4) handleV4ToolCall(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
	switch name {
	case "get_websocket_events":
		return h.toolGetWSEvents(req, args), true
	case "get_websocket_status":
		return h.toolGetWSStatus(req, args), true
	case "get_network_bodies":
		return h.toolGetNetworkBodies(req, args), true
	case "query_dom":
		return h.toolQueryDOM(req, args), true
	case "get_page_info":
		return h.toolGetPageInfo(req, args), true
	case "run_accessibility_audit":
		return h.toolRunA11yAudit(req, args), true
	case "get_enhanced_actions":
		return h.toolGetEnhancedActions(req, args), true
	case "get_reproduction_script":
		return h.toolGetReproductionScript(req, args), true
	case "check_performance":
		return h.toolCheckPerformance(req, args), true
	case "get_session_timeline":
		return h.toolGetSessionTimeline(req, args), true
	case "generate_test":
		return h.toolGenerateTest(req, args), true
	case "get_changes_since":
		return h.toolGetChangesSince(req, args), true
	}
	return JSONRPCResponse{}, false
}

// ============================================
// Compressed State Diffs
// ============================================

func (h *MCPHandlerV4) toolGetChangesSince(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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
	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(diffJSON)},
		},
	}

	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

// ============================================
// v5 MCP Tool Implementations
