package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// E2E Tests: get_changes_since via MCP JSON-RPC
// ============================================
//
// These tests exercise the get_changes_since tool through
// the full MCP JSON-RPC handler stack (HandleRequest → handleToolsCall
// → handleToolCall → toolGetChangesSince).

// helper: initialize MCP session
func initMCP(t *testing.T, mcp *MCPHandler) {
	t.Helper()
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})
	if resp.Error != nil {
		t.Fatalf("initialize failed: %s", resp.Error.Message)
	}
}

// helper: call observe with what:"changes" and given arguments JSON
func callGetChangesSince(t *testing.T, mcp *MCPHandler, argsJSON string) DiffResponse {
	t.Helper()
	// Inject "what":"changes" into the arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		t.Fatalf("Failed to unmarshal argsJSON: %v", err)
	}
	args["what"] = "changes"
	mergedArgs, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("Failed to marshal merged args: %v", err)
	}
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":` + string(mergedArgs) + `}`),
	})
	if resp.Error != nil {
		t.Fatalf("observe(what:changes) failed: %s", resp.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	// Strip the summary prefix line added by mcpJSONResponse
	text := result.Content[0].Text
	if idx := strings.Index(text, "\n"); idx >= 0 {
		text = text[idx+1:]
	}

	var diff DiffResponse
	if err := json.Unmarshal([]byte(text), &diff); err != nil {
		t.Fatalf("Failed to unmarshal DiffResponse: %v\nRaw: %s", err, text)
	}
	return diff
}

// --- Test: tool is discoverable in tools/list ---

func TestE2E_ToolAppearsInToolsList(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 3, Method: "tools/list",
		Params: json.RawMessage(`{}`),
	})
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %s", resp.Error.Message)
	}

	var toolsList struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &toolsList)

	found := false
	for _, tool := range toolsList.Tools {
		if tool.Name == "observe" {
			found = true
			break
		}
	}
	if !found {
		t.Error("observe not found in tools/list")
	}
}

// --- Test: first call with empty buffers ---

func TestE2E_FirstCallEmptyBuffers(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Severity != "clean" {
		t.Errorf("Expected severity 'clean', got %q", diff.Severity)
	}
	if diff.Console != nil {
		t.Error("Expected nil Console for empty buffers")
	}
	if diff.Network != nil {
		t.Error("Expected nil Network for empty buffers")
	}
	if diff.WebSocket != nil {
		t.Error("Expected nil WebSocket for empty buffers")
	}
	if diff.Actions != nil {
		t.Error("Expected nil Actions for empty buffers")
	}
}

// --- Test: first call with console errors ---

func TestE2E_FirstCallWithConsoleErrors(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Add console errors
	server.addEntries([]LogEntry{
		{"level": "error", "message": "TypeError: Cannot read property 'x' of null", "source": "app.js"},
		{"level": "error", "message": "ReferenceError: foo is not defined", "source": "utils.js"},
		{"level": "warn", "message": "Deprecation warning: use newMethod()"},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Severity != "error" {
		t.Errorf("Expected severity 'error', got %q", diff.Severity)
	}
	if diff.Console == nil {
		t.Fatal("Expected Console diff")
	}
	if diff.Console.TotalNew != 3 {
		t.Errorf("Expected 3 total_new, got %d", diff.Console.TotalNew)
	}
	if len(diff.Console.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(diff.Console.Errors))
	}
	if len(diff.Console.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(diff.Console.Warnings))
	}
}

// --- Test: auto-advance shows only new data ---

func TestE2E_AutoAdvance(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Add initial error
	server.addEntries([]LogEntry{
		{"level": "error", "message": "Initial error"},
	})

	// First call sees the error and advances
	diff1 := callGetChangesSince(t, mcp, `{}`)
	if diff1.Console == nil || len(diff1.Console.Errors) != 1 {
		t.Fatal("First call should see 1 error")
	}

	// Second call with no new data → clean
	diff2 := callGetChangesSince(t, mcp, `{}`)
	if diff2.Severity != "clean" {
		t.Errorf("Expected clean after auto-advance, got %q", diff2.Severity)
	}
	if diff2.Console != nil {
		t.Error("Expected nil Console after auto-advance with no new data")
	}

	// Add another error
	server.addEntries([]LogEntry{
		{"level": "error", "message": "New error after advance"},
	})

	// Third call sees only the new error
	diff3 := callGetChangesSince(t, mcp, `{}`)
	if diff3.Console == nil {
		t.Fatal("Third call should see console diff")
	}
	if len(diff3.Console.Errors) != 1 {
		t.Errorf("Expected 1 new error, got %d", len(diff3.Console.Errors))
	}
	if diff3.Console.Errors[0].Message != "New error after advance" {
		t.Errorf("Unexpected error message: %s", diff3.Console.Errors[0].Message)
	}
}

// --- Test: named checkpoint query does not advance auto-checkpoint ---

func TestE2E_NamedCheckpointDoesNotAdvanceAuto(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Add initial error
	server.addEntries([]LogEntry{
		{"level": "error", "message": "Error 1"},
	})

	// Call with a named checkpoint — does NOT advance auto-checkpoint
	callGetChangesSince(t, mcp, `{"checkpoint":"some-name"}`)

	// Add another error
	server.addEntries([]LogEntry{
		{"level": "error", "message": "Error 2"},
	})

	// Auto-checkpoint call should still see BOTH errors (auto wasn't advanced)
	diff := callGetChangesSince(t, mcp, `{}`)
	if diff.Console == nil {
		t.Fatal("Expected console diff")
	}
	if diff.Console.TotalNew != 2 {
		t.Errorf("Expected 2 total_new (auto not advanced by named query), got %d", diff.Console.TotalNew)
	}
}

// --- Test: severity filter errors_only ---

func TestE2E_SeverityFilterErrorsOnly(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "Critical failure"},
		{"level": "warn", "message": "Minor deprecation"},
		{"level": "info", "message": "Just FYI"},
	})

	diff := callGetChangesSince(t, mcp, `{"severity":"errors_only"}`)

	if diff.Console == nil {
		t.Fatal("Expected console diff")
	}
	if len(diff.Console.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(diff.Console.Errors))
	}
	if len(diff.Console.Warnings) != 0 {
		t.Errorf("Expected 0 warnings with errors_only, got %d", len(diff.Console.Warnings))
	}
}

// --- Test: severity filter warnings ---

func TestE2E_SeverityFilterWarnings(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "Error here"},
		{"level": "warn", "message": "Warning here"},
		{"level": "info", "message": "Info message"},
	})

	diff := callGetChangesSince(t, mcp, `{"severity":"warnings"}`)

	if diff.Console == nil {
		t.Fatal("Expected console diff")
	}
	if len(diff.Console.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(diff.Console.Errors))
	}
	if len(diff.Console.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(diff.Console.Warnings))
	}
}

// --- Test: include filter (console only) ---

func TestE2E_IncludeFilterConsoleOnly(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Add data to multiple categories
	server.addEntries([]LogEntry{
		{"level": "error", "message": "Console error"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/data", Method: "GET", Status: 500, Duration: 100},
	})

	diff := callGetChangesSince(t, mcp, `{"include":["console"]}`)

	if diff.Console == nil {
		t.Fatal("Expected console diff with include filter")
	}
	if diff.Network != nil {
		t.Error("Expected nil Network with console-only include filter")
	}
}

// --- Test: include filter (network only) ---

func TestE2E_IncludeFilterNetworkOnly(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "Console error"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/data", Method: "GET", Status: 500, Duration: 100},
	})

	diff := callGetChangesSince(t, mcp, `{"include":["network"]}`)

	if diff.Console != nil {
		t.Error("Expected nil Console with network-only include filter")
	}
	if diff.Network == nil {
		t.Fatal("Expected network diff with network include filter")
	}
}

// --- Test: network failure regression (was 200, now 500) ---

func TestE2E_NetworkFailureRegression(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// First: endpoint returns 200, establish baseline via auto-checkpoint
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, Duration: 50},
	})
	callGetChangesSince(t, mcp, `{}`) // advances auto-checkpoint, saves known endpoints

	// Now the same endpoint fails with 500
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 500, Duration: 200},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Network == nil {
		t.Fatal("Expected network diff")
	}
	if len(diff.Network.Failures) == 0 {
		t.Fatal("Expected network failure regression (was 200, now 500)")
	}

	found := false
	for _, f := range diff.Network.Failures {
		if f.Status == 500 && f.PreviousStatus == 200 {
			found = true
		}
	}
	if !found {
		t.Error("Expected failure with status=500 and previousStatus=200")
	}
}

// --- Test: new failing endpoints appear in NewEndpoints ---

func TestE2E_NetworkNewFailingEndpoints(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// A brand new endpoint that immediately returns 500
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/orders", Method: "POST", Status: 500, Duration: 200},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Network == nil {
		t.Fatal("Expected network diff")
	}
	// New endpoints that fail immediately go to NewEndpoints (not Failures)
	if len(diff.Network.NewEndpoints) == 0 {
		t.Error("Expected new endpoint entry for first-time failing endpoint")
	}
}

// --- Test: WebSocket disconnections ---

func TestE2E_WebSocketDisconnections(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "ws-1", Event: "close", CloseCode: 1006, CloseReason: "abnormal"},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.WebSocket == nil {
		t.Fatal("Expected websocket diff")
	}
	if len(diff.WebSocket.Disconnections) == 0 {
		t.Error("Expected WebSocket disconnections for abnormal close (1006)")
	}
}

// --- Test: deduplication of repeated console errors ---

func TestE2E_ConsoleDeduplication(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Same error repeated 5 times
	for i := 0; i < 5; i++ {
		server.addEntries([]LogEntry{
			{"level": "error", "message": "TypeError: Cannot read property 'x' of null"},
		})
	}

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Console == nil {
		t.Fatal("Expected console diff")
	}
	// Deduplication should collapse to 1 entry with count=5
	if len(diff.Console.Errors) != 1 {
		t.Errorf("Expected 1 deduplicated error, got %d", len(diff.Console.Errors))
	}
	if diff.Console.Errors[0].Count != 5 {
		t.Errorf("Expected count=5 for deduplicated error, got %d", diff.Console.Errors[0].Count)
	}
}

// --- Test: multiple categories in one response ---

func TestE2E_MultipleCategoriesCombined(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Add data to all categories
	server.addEntries([]LogEntry{
		{"level": "error", "message": "JS error"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/fail", Method: "GET", Status: 500, Duration: 100},
	})
	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-1", Event: "error", URL: "wss://example.com/ws"},
	})
	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Selectors: map[string]interface{}{"css": ".btn-submit"}},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Console == nil {
		t.Error("Expected console diff")
	}
	if diff.Network == nil {
		t.Error("Expected network diff")
	}
	if diff.WebSocket == nil {
		t.Error("Expected websocket diff")
	}
	if diff.Actions == nil {
		t.Error("Expected actions diff")
	}
	if diff.TokenCount == 0 {
		t.Error("Expected non-zero token count")
	}
}

// --- Test: timestamp-based since ---

func TestE2E_TimestampBased(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Add data - the timestamp in the checkpoint param refers to wall clock time
	// when the data was added, so we add data sequentially
	server.addEntries([]LogEntry{
		{"level": "error", "message": "Old error"},
	})

	// Use a very old timestamp to get all data
	diff := callGetChangesSince(t, mcp, `{"checkpoint":"2020-01-01T00:00:00Z"}`)

	if diff.Console == nil {
		t.Fatal("Expected console diff for old timestamp")
	}
	if len(diff.Console.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(diff.Console.Errors))
	}
}

// --- Test: unknown tool returns error ---

func TestE2E_UnknownToolReturnsError(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"nonexistent_tool","arguments":{}}`),
	})

	if resp.Error == nil {
		t.Fatal("Expected error for unknown tool")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
}

// --- Test: summary field is populated ---

func TestE2E_SummaryPopulated(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "Something broke"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/fail", Method: "GET", Status: 503, Duration: 5000},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Summary == "" {
		t.Error("Expected non-empty summary")
	}
}

// --- Test: degraded endpoint detected (slow response) ---

func TestE2E_DegradedEndpoints(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// First call establishes baseline with fast endpoint via auto-checkpoint
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/data", Method: "GET", Status: 200, Duration: 50},
	})
	callGetChangesSince(t, mcp, `{}`) // advances auto, saves KnownEndpoints with duration=50

	// Now add a much slower request to the same endpoint (>3x baseline)
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/data", Method: "GET", Status: 200, Duration: 500},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Network == nil {
		t.Fatal("Expected network diff for degraded endpoint")
	}
	if len(diff.Network.Degraded) == 0 {
		t.Error("Expected degraded endpoint entry for 10x slower request")
	}
}

// --- Test: from/to timestamps populated ---

func TestE2E_FromToTimestamps(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "test"},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.From.IsZero() {
		t.Error("Expected non-zero From timestamp")
	}
	if diff.To.IsZero() {
		t.Error("Expected non-zero To timestamp")
	}
	if diff.DurationMs < 0 {
		t.Error("Expected non-negative DurationMs")
	}
}

// --- Test: actions diff ---

func TestE2E_ActionsDiff(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Selectors: map[string]interface{}{"css": "#submit-btn"}, URL: "https://app.example.com/form"},
		{Type: "input", Selectors: map[string]interface{}{"css": "#email"}, URL: "https://app.example.com/form"},
		{Type: "click", Selectors: map[string]interface{}{"css": ".nav-link"}, URL: "https://app.example.com/home"},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Actions == nil {
		t.Fatal("Expected actions diff")
	}
	if diff.Actions.TotalNew != 3 {
		t.Errorf("Expected 3 total_new actions, got %d", diff.Actions.TotalNew)
	}
}

// --- Test: WebSocket errors (not just disconnections) ---

func TestE2E_WebSocketErrors(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-1", Event: "error", URL: "wss://example.com/ws"},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.WebSocket == nil {
		t.Fatal("Expected websocket diff for error events")
	}
	if len(diff.WebSocket.Errors) == 0 {
		t.Error("Expected WebSocket errors")
	}
}

// --- Test: new connections tracked ---

func TestE2E_WebSocketNewConnections(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "ws-2", Event: "open", URL: "wss://other.com/realtime"},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.WebSocket == nil {
		t.Fatal("Expected websocket diff")
	}
	if len(diff.WebSocket.Connections) < 2 {
		t.Errorf("Expected at least 2 new connections, got %d", len(diff.WebSocket.Connections))
	}
}

// --- Test: auto-checkpoint persists correctly across multiple calls ---

func TestE2E_AutoCheckpointPersistence(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Add initial data
	server.addEntries([]LogEntry{
		{"level": "error", "message": "Error 1"},
	})

	// First call: sees Error 1, advances auto-checkpoint
	diff1 := callGetChangesSince(t, mcp, `{}`)
	if diff1.Console == nil || len(diff1.Console.Errors) != 1 {
		t.Fatal("First call should see 1 error")
	}

	// Add more data
	server.addEntries([]LogEntry{
		{"level": "error", "message": "Error 2"},
		{"level": "error", "message": "Error 3"},
	})

	// Second call: sees only Error 2 and Error 3
	diff2 := callGetChangesSince(t, mcp, `{}`)
	if diff2.Console == nil {
		t.Fatal("Expected console diff")
	}
	if len(diff2.Console.Errors) != 2 {
		t.Errorf("Expected 2 new errors, got %d", len(diff2.Console.Errors))
	}

	// Third call: nothing new
	diff3 := callGetChangesSince(t, mcp, `{}`)
	if diff3.Severity != "clean" {
		t.Errorf("Expected clean after consuming all data, got %q", diff3.Severity)
	}
}

// --- Test: network new endpoints ---

func TestE2E_NetworkNewEndpoints(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, Duration: 50},
		{URL: "https://api.example.com/posts", Method: "GET", Status: 200, Duration: 80},
	})

	diff := callGetChangesSince(t, mcp, `{}`)

	if diff.Network == nil {
		t.Fatal("Expected network diff")
	}
	if len(diff.Network.NewEndpoints) < 2 {
		t.Errorf("Expected at least 2 new endpoints, got %d", len(diff.Network.NewEndpoints))
	}
}
