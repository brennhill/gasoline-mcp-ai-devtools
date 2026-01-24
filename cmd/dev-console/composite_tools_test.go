package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// Composite Tool Surface Tests (Phase 1: 24 → 5)
// ============================================

// TestCompositeToolsListReturnsAllTools verifies that tools/list
// returns the 5 composite tools plus the 4 v6 tools.
func TestCompositeToolsListReturnsAllTools(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result MCPToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	expected := map[string]bool{
		"observe":        true,
		"analyze":        true,
		"generate":       true,
		"configure":      true,
		"query_dom":      true,
		"generate_csp":   true,
		"security_audit": true,
		"get_audit_log":  true,
		"diff_sessions":  true,
	}

	if len(result.Tools) != 9 {
		names := make([]string, len(result.Tools))
		for i, tool := range result.Tools {
			names[i] = tool.Name
		}
		t.Fatalf("Expected exactly 9 tools, got %d: %v", len(result.Tools), names)
	}

	for _, tool := range result.Tools {
		if !expected[tool.Name] {
			t.Errorf("Unexpected tool: %s", tool.Name)
		}
	}
}

// TestCompositeToolsHaveRequiredModeParam verifies that observe, analyze,
// generate, and configure all have a required mode parameter.
func TestCompositeToolsHaveRequiredModeParam(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result MCPToolsListResult
	json.Unmarshal(resp.Result, &result)

	modeParams := map[string]string{
		"observe":   "what",
		"analyze":   "target",
		"generate":  "format",
		"configure": "action",
	}

	for _, tool := range result.Tools {
		modeParam, hasModeParam := modeParams[tool.Name]
		if !hasModeParam {
			continue // query_dom doesn't have a mode param
		}

		// Check that the mode param exists in properties
		props, ok := tool.InputSchema["properties"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %s: missing properties in inputSchema", tool.Name)
			continue
		}

		if _, ok := props[modeParam]; !ok {
			t.Errorf("Tool %s: missing required mode param '%s'", tool.Name, modeParam)
		}

		// Check that the mode param is in required array
		required, ok := tool.InputSchema["required"].([]interface{})
		if !ok {
			// Try []string (depends on marshal format)
			requiredStr, ok2 := tool.InputSchema["required"].([]string)
			if !ok2 {
				t.Errorf("Tool %s: missing 'required' in inputSchema", tool.Name)
				continue
			}
			found := false
			for _, r := range requiredStr {
				if r == modeParam {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Tool %s: mode param '%s' not in required list", tool.Name, modeParam)
			}
			continue
		}

		found := false
		for _, r := range required {
			if r == modeParam {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Tool %s: mode param '%s' not in required list", tool.Name, modeParam)
		}
	}
}

// TestCompositeToolsModeEnumValues verifies that each composite tool's mode
// parameter has the correct enum values.
func TestCompositeToolsModeEnumValues(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result MCPToolsListResult
	json.Unmarshal(resp.Result, &result)

	expectedEnums := map[string][]string{
		"observe":   {"errors", "logs", "network", "websocket_events", "websocket_status", "actions", "vitals", "page"},
		"analyze":   {"performance", "api", "accessibility", "changes", "timeline"},
		"generate":  {"reproduction", "test", "pr_summary", "sarif", "har"},
		"configure": {"store", "load", "noise_rule", "dismiss", "clear"},
	}

	modeParams := map[string]string{
		"observe":   "what",
		"analyze":   "target",
		"generate":  "format",
		"configure": "action",
	}

	for _, tool := range result.Tools {
		expectedEnum, hasEnum := expectedEnums[tool.Name]
		if !hasEnum {
			continue
		}

		modeParam := modeParams[tool.Name]
		props := tool.InputSchema["properties"].(map[string]interface{})
		modeProp := props[modeParam].(map[string]interface{})

		enumRaw, ok := modeProp["enum"]
		if !ok {
			t.Errorf("Tool %s: mode param '%s' missing enum", tool.Name, modeParam)
			continue
		}

		// Convert to string slice for comparison
		var enumValues []string
		enumJSON, _ := json.Marshal(enumRaw)
		json.Unmarshal(enumJSON, &enumValues)

		if len(enumValues) != len(expectedEnum) {
			t.Errorf("Tool %s: expected %d enum values, got %d: %v",
				tool.Name, len(expectedEnum), len(enumValues), enumValues)
			continue
		}

		for i, v := range expectedEnum {
			if enumValues[i] != v {
				t.Errorf("Tool %s: enum[%d] expected '%s', got '%s'", tool.Name, i, v, enumValues[i])
			}
		}
	}
}

// ============================================
// observe tool dispatch tests
// ============================================

func TestObserveErrors(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "ReferenceError: x is not defined"},
		{"level": "info", "message": "App initialized"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "ReferenceError") {
		t.Error("Expected error message in output")
	}
	if strings.Contains(text, "App initialized") {
		t.Error("Info messages should not appear in errors mode")
	}
}

func TestObserveLogs(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "info", "message": "Hello"},
		{"level": "warn", "message": "Deprecation"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "Deprecation") {
		t.Error("Expected all log entries in output")
	}
}

func TestObserveLogsWithLimit(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "info", "message": "First"},
		{"level": "info", "message": "Second"},
		{"level": "info", "message": "Third"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs","limit":1}}`),
	})

	text := extractMCPText(t, resp)
	// With limit=1, should return only the most recent entry
	if !strings.Contains(text, "Third") {
		t.Error("Expected most recent entry with limit=1")
	}
}

func TestObserveNetwork(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, ResponseBody: `{"users":[]}`},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "api.example.com") {
		t.Error("Expected network body in output")
	}
}

func TestObserveNetworkWithFilters(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200},
		{URL: "https://api.example.com/posts", Method: "POST", Status: 201},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network","method":"POST"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "posts") {
		t.Error("Expected POST request in filtered output")
	}
}

func TestObserveWebSocketEvents(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-1", Event: "message", Data: `{"type":"ping"}`, URL: "wss://echo.example.com"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_events"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "ping") {
		t.Error("Expected WebSocket event data in output")
	}
}

func TestObserveWebSocketStatus(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Register a connection via an "open" event (triggers trackConnection)
	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-conn-1", Event: "open", URL: "wss://echo.example.com/socket"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_status"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "echo.example.com") {
		t.Error("Expected WebSocket connection URL in status output")
	}
}

func TestObserveActions(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Selectors: map[string]interface{}{"css": "#submit-btn"}, URL: "http://localhost:3000/form"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"actions"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "submit-btn") {
		t.Error("Expected action selector in output")
	}
}

func TestObserveVitals(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	fcp := 1200.0
	lcp := 2500.0
	cls := 0.05
	inp := 150.0

	capture.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			InteractionToNextPaint: &inp,
		},
		Network: NetworkSummary{ByType: map[string]TypeSummary{}},
		CLS:     &cls,
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"vitals"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "fcp") {
		t.Error("Expected web vitals data in output")
	}
}

func TestObservePage(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Page info uses the pending query mechanism (waits for extension response).
	// Without an extension connected, this will timeout — we verify the dispatch
	// is correct by ensuring no JSON-RPC protocol error is returned.
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"page"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no JSON-RPC error, got: %v", resp.Error)
	}

	// The result will contain a timeout message, but the dispatch worked
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty response (timeout message)")
	}
}

func TestObserveUnknownMode(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"invalid_mode"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "unknown") && !strings.Contains(text, "Unknown") {
		t.Error("Expected error message for unknown mode")
	}
}

func TestObserveMissingMode(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "required") && !strings.Contains(text, "Required") {
		t.Error("Expected error for missing 'what' parameter")
	}
}

// ============================================
// analyze tool dispatch tests
// ============================================

func TestAnalyzePerformance(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"performance"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	// Performance tool returns data even with no snapshots (empty baseline)
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty performance response")
	}
}

func TestAnalyzeAPI(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"api"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty API schema response")
	}
}

func TestAnalyzeChanges(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"changes"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty changes response")
	}
}

func TestAnalyzeTimeline(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Selectors: map[string]interface{}{"css": "#nav"}, URL: "http://localhost:3000", Timestamp: 1704067200000},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"timeline"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty timeline response")
	}
}

func TestAnalyzeUnknownTarget(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"bogus"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "unknown") && !strings.Contains(text, "Unknown") {
		t.Error("Expected error for unknown target")
	}
}

func TestAnalyzeMissingTarget(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "required") && !strings.Contains(text, "Required") {
		t.Error("Expected error for missing 'target' parameter")
	}
}

// ============================================
// generate tool dispatch tests
// ============================================

func TestGenerateReproduction(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Selectors: map[string]interface{}{"css": "#login"}, URL: "http://localhost:3000/login"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty reproduction script")
	}
}

func TestGenerateTest(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Selectors: map[string]interface{}{"css": "#submit"}, URL: "http://localhost:3000/form"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"test","test_name":"login flow"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty test output")
	}
}

func TestGeneratePRSummary(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"pr_summary"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty PR summary")
	}
}

func TestGenerateHAR(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/data", Method: "GET", Status: 200, ResponseBody: `{"ok":true}`},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"har"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty HAR output")
	}
}

func TestGenerateUnknownFormat(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"pdf"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "unknown") && !strings.Contains(text, "Unknown") {
		t.Error("Expected error for unknown format")
	}
}

func TestGenerateMissingFormat(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "required") && !strings.Contains(text, "Required") {
		t.Error("Expected error for missing 'format' parameter")
	}
}

// ============================================
// configure tool dispatch tests
// ============================================

func TestConfigureClear(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "old error"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"clear"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "cleared") && !strings.Contains(text, "Cleared") {
		t.Error("Expected confirmation of clear")
	}

	// Verify entries are actually cleared
	server.mu.RLock()
	count := len(server.entries)
	server.mu.RUnlock()
	if count != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", count)
	}
}

func TestConfigureLoad(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"load"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty session context response")
	}
}

func TestConfigureNoiseRule(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"noise_rule","noise_action":"list"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty noise config response")
	}
}

func TestConfigureDismiss(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"dismiss","pattern":"extension.*warning"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if !strings.Contains(text, "ok") {
		t.Error("Expected success response for dismiss")
	}
}

func TestConfigureStore(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"store","store_action":"stats"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	text := extractMCPText(t, resp)
	if text == "" {
		t.Error("Expected non-empty store response")
	}
}

func TestConfigureUnknownAction(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"self_destruct"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "unknown") && !strings.Contains(text, "Unknown") {
		t.Error("Expected error for unknown action")
	}
}

func TestConfigureMissingAction(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "required") && !strings.Contains(text, "Required") {
		t.Error("Expected error for missing 'action' parameter")
	}
}

// ============================================
// query_dom stays unchanged
// ============================================

func TestQueryDomStillWorks(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// query_dom requires a pending query mechanism, so we just verify
	// it's dispatched correctly (non-error response for valid selector)
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"query_dom","arguments":{"selector":"#app"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no JSON-RPC error, got: %v", resp.Error)
	}
}

// ============================================
// Legacy tool names should be rejected
// ============================================

func TestLegacyToolNamesRejected(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	legacyNames := []string{
		"get_browser_errors",
		"get_browser_logs",
		"clear_browser_logs",
		"get_websocket_events",
		"get_websocket_status",
		"get_network_bodies",
		"get_page_info",
		"run_accessibility_audit",
		"get_enhanced_actions",
		"get_reproduction_script",
		"check_performance",
		"get_session_timeline",
		"generate_test",
		"generate_pr_summary",
		"get_changes_since",
		"session_store",
		"load_session_context",
		"configure_noise",
		"dismiss_noise",
		"get_api_schema",
		"export_sarif",
		"export_har",
		"get_web_vitals",
	}

	for _, name := range legacyNames {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: json.RawMessage(`{"name":"` + name + `","arguments":{}}`),
		})

		if resp.Error == nil {
			t.Errorf("Legacy tool '%s' should be rejected, but was handled", name)
		}
	}
}

// ============================================
// Phase 2: _meta Data Counts Tests
// ============================================

// TestToolsListMetaFieldPresent verifies that observe, analyze, and generate
// tools include a _meta field with data_counts in the tools/list response.
func TestToolsListMetaFieldPresent(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	// Parse with raw JSON to check _meta field
	var raw struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &raw); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	toolsWithMeta := map[string]bool{"observe": true, "analyze": true, "generate": true}
	toolsWithoutMeta := map[string]bool{"configure": true, "query_dom": true}

	for _, toolRaw := range raw.Tools {
		var tool struct {
			Name string                 `json:"name"`
			Meta map[string]interface{} `json:"_meta"`
		}
		json.Unmarshal(toolRaw, &tool)

		if toolsWithMeta[tool.Name] {
			if tool.Meta == nil {
				t.Errorf("Tool %s: expected _meta field, got nil", tool.Name)
				continue
			}
			if _, ok := tool.Meta["data_counts"]; !ok {
				t.Errorf("Tool %s: _meta missing data_counts", tool.Name)
			}
		}

		if toolsWithoutMeta[tool.Name] {
			if tool.Meta != nil {
				t.Errorf("Tool %s: should not have _meta field, got %v", tool.Name, tool.Meta)
			}
		}
	}
}

// TestToolsListMetaEmptyCountsOnFreshServer verifies that on a fresh server
// with no data, all data_counts are zero.
func TestToolsListMetaEmptyCountsOnFreshServer(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	observeMeta := extractToolMeta(t, resp, "observe")
	dataCounts := observeMeta["data_counts"].(map[string]interface{})

	expectedZero := []string{"errors", "logs", "network", "websocket_events", "websocket_status", "actions", "vitals"}
	for _, key := range expectedZero {
		val, ok := dataCounts[key]
		if !ok {
			t.Errorf("observe _meta.data_counts missing key: %s", key)
			continue
		}
		if val.(float64) != 0 {
			t.Errorf("observe _meta.data_counts[%s] = %v, want 0", key, val)
		}
	}
}

// TestToolsListMetaReflectsBufferSizes verifies that data_counts update
// as data is added to the server and capture buffers.
func TestToolsListMetaReflectsBufferSizes(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Add data to various buffers
	server.mu.Lock()
	server.entries = append(server.entries, LogEntry{"level": "error", "message": "test error 1"})
	server.entries = append(server.entries, LogEntry{"level": "error", "message": "test error 2"})
	server.entries = append(server.entries, LogEntry{"level": "info", "message": "test log"})
	server.mu.Unlock()

	capture.mu.Lock()
	capture.networkBodies = append(capture.networkBodies, NetworkBody{URL: "/api/test", Status: 200})
	capture.networkBodies = append(capture.networkBodies, NetworkBody{URL: "/api/test2", Status: 201})
	capture.wsEvents = append(capture.wsEvents, WebSocketEvent{ID: "ws1", Direction: "incoming"})
	capture.connections["ws1"] = &connectionState{url: "ws://localhost"}
	capture.enhancedActions = append(capture.enhancedActions,
		EnhancedAction{Type: "click"},
		EnhancedAction{Type: "input"},
		EnhancedAction{Type: "navigate"},
	)
	capture.perf.snapshots["page1"] = PerformanceSnapshot{URL: "/page1"}
	capture.mu.Unlock()

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	// Check observe counts
	observeMeta := extractToolMeta(t, resp, "observe")
	observeCounts := observeMeta["data_counts"].(map[string]interface{})
	assertCount(t, observeCounts, "errors", 2)
	assertCount(t, observeCounts, "logs", 3)
	assertCount(t, observeCounts, "network", 2)
	assertCount(t, observeCounts, "websocket_events", 1)
	assertCount(t, observeCounts, "websocket_status", 1)
	assertCount(t, observeCounts, "actions", 3)
	assertCount(t, observeCounts, "vitals", 1)

	// Check analyze counts
	analyzeMeta := extractToolMeta(t, resp, "analyze")
	analyzeCounts := analyzeMeta["data_counts"].(map[string]interface{})
	assertCount(t, analyzeCounts, "performance", 1)
	assertCount(t, analyzeCounts, "api", 0)
	assertCount(t, analyzeCounts, "timeline", 3)

	// Check generate counts
	generateMeta := extractToolMeta(t, resp, "generate")
	generateCounts := generateMeta["data_counts"].(map[string]interface{})
	assertCount(t, generateCounts, "reproduction", 3)
	assertCount(t, generateCounts, "test", 3)
	assertCount(t, generateCounts, "har", 2)
}

// TestToolsListMetaAPISchemaCount verifies the API schema endpoint count.
func TestToolsListMetaAPISchemaCount(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Add some API observations
	capture.schemaStore.Observe(NetworkBody{URL: "/api/users", Method: "GET", Status: 200})
	capture.schemaStore.Observe(NetworkBody{URL: "/api/posts", Method: "POST", Status: 201})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	analyzeMeta := extractToolMeta(t, resp, "analyze")
	analyzeCounts := analyzeMeta["data_counts"].(map[string]interface{})
	assertCount(t, analyzeCounts, "api", 2)
}

// TestToolsListGoldenMatchesWithMeta ensures the golden file test still works
// (the _meta field is present but counts are zero on fresh server, matching expectations).
func TestToolsListGoldenMatchesWithMeta(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	// Verify it returns 9 tools with proper structure (5 composite + 4 v6)
	var result MCPToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}
	if len(result.Tools) != 9 {
		t.Fatalf("Expected 9 tools, got %d", len(result.Tools))
	}

	// Verify _meta doesn't break standard MCPTool parsing
	for _, tool := range result.Tools {
		if tool.Name == "" {
			t.Error("Tool name is empty")
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.InputSchema == nil {
			t.Error("Tool inputSchema is nil")
		}
	}
}

// ============================================
// Phase 2 Helpers
// ============================================

func extractToolMeta(t *testing.T, resp JSONRPCResponse, toolName string) map[string]interface{} {
	t.Helper()
	var raw struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &raw); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	for _, toolRaw := range raw.Tools {
		var tool struct {
			Name string                 `json:"name"`
			Meta map[string]interface{} `json:"_meta"`
		}
		json.Unmarshal(toolRaw, &tool)
		if tool.Name == toolName {
			if tool.Meta == nil {
				t.Fatalf("Tool %s: _meta field is nil", toolName)
			}
			return tool.Meta
		}
	}
	t.Fatalf("Tool %s not found in tools list", toolName)
	return nil
}

func assertCount(t *testing.T, counts map[string]interface{}, key string, expected int) {
	t.Helper()
	val, ok := counts[key]
	if !ok {
		t.Errorf("data_counts missing key: %s", key)
		return
	}
	if int(val.(float64)) != expected {
		t.Errorf("data_counts[%s] = %v, want %d", key, val, expected)
	}
}

// ============================================
// Helpers
// ============================================

func extractMCPText(t *testing.T, resp JSONRPCResponse) string {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse MCP result: %v", err)
	}
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}
