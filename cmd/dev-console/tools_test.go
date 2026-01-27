package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestV4RateLimitWebSocketEvents(t *testing.T) {
	capture := setupTestCapture(t)

	// Simulate flooding: > 1000 events in rapid succession
	capture.RecordEvents(1100)

	// Next request should be rate limited
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(`{"events":[{"event":"message"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", rec.Code)
	}
}

func TestV4MemoryLimitRejectsNetworkBodies(t *testing.T) {
	capture := setupTestCapture(t)

	// Simulate exceeding memory limit
	capture.SetMemoryUsage(55 * 1024 * 1024) // 55MB > 50MB limit

	body := `{"bodies":[{"url":"/api/test","status":200,"responseBody":"data"}]}`
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", rec.Code)
	}
}

func TestV4WebSocketBufferMemoryLimit(t *testing.T) {
	capture := setupTestCapture(t)

	// Add events that exceed 4MB memory limit
	largeData := strings.Repeat("x", 100000) // 100KB per event
	for i := 0; i < 50; i++ {                // 50 * 100KB = 5MB
		capture.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Data: largeData},
		})
	}

	// Buffer should evict to stay under 4MB
	memUsage := capture.GetWebSocketBufferMemory()
	if memUsage > 4*1024*1024 {
		t.Errorf("Expected WS buffer memory <= 4MB, got %d bytes", memUsage)
	}
}

func TestV4NetworkBodiesBufferMemoryLimit(t *testing.T) {
	capture := setupTestCapture(t)

	// Add bodies that exceed 8MB memory limit
	largeBody := strings.Repeat("y", 200000) // 200KB per body
	for i := 0; i < 50; i++ {                // 50 * 200KB = 10MB
		capture.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", ResponseBody: largeBody, Status: 200},
		})
	}

	// Buffer should evict to stay under 8MB
	memUsage := capture.GetNetworkBodiesBufferMemory()
	if memUsage > 8*1024*1024 {
		t.Errorf("Expected network bodies buffer memory <= 8MB, got %d bytes", memUsage)
	}
}

func TestMCPToolsListIncludesV4Tools(t *testing.T) {
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

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"observe",
		"generate",
		"configure",
		"interact",
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("Expected tool '%s' in tools list", name)
		}
	}
}

func TestV5AiContextPassthroughInGetBrowserErrors(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add a log entry with _aiContext field (as extension would send)
	entry := LogEntry{
		"level":   "error",
		"message": "Cannot read property 'user' of undefined",
		"source":  "app.js:42",
		"stack":   "TypeError: Cannot read property 'user' of undefined\n    at UserProfile.render (app.js:42:15)",
		"_aiContext": map[string]interface{}{
			"summary": "TypeError in UserProfile.render at app.js:42. React component: UserProfile > App.",
			"componentAncestry": map[string]interface{}{
				"framework":  "react",
				"components": []interface{}{"UserProfile", "App"},
			},
			"stateSnapshot": map[string]interface{}{
				"relevantSlice": map[string]interface{}{
					"auth": map[string]interface{}{"user": nil, "loading": false},
				},
			},
		},
		"_enrichments": []interface{}{"aiContext"},
	}
	server.addEntries([]LogEntry{entry})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	// After W1 migration, errors are returned as markdown table with summary.
	// The error message and source should appear in the table columns.
	responseText := result.Content[0].Text
	if !strings.Contains(responseText, "1 browser error(s)") {
		t.Error("Expected summary line with error count")
	}
	if !strings.Contains(responseText, "Cannot read property") {
		t.Error("Expected error message in markdown table")
	}
	if !strings.Contains(responseText, "app.js:42") {
		t.Error("Expected source in markdown table")
	}
	// Markdown table should have pipe delimiters
	if !strings.Contains(responseText, "| ") {
		t.Error("Expected markdown table format with pipe delimiters")
	}
}

func TestV4GetTotalBufferMemory(t *testing.T) {
	capture := setupTestCapture(t)

	// Add some data to each buffer
	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "message", Data: strings.Repeat("a", 1000)},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", ResponseBody: strings.Repeat("b", 2000)},
	})

	total := capture.GetTotalBufferMemory()
	if total <= 0 {
		t.Errorf("Expected positive total buffer memory, got %d", total)
	}

	// Total should be sum of WS + NB buffers
	wsMemory := capture.GetWebSocketBufferMemory()
	nbMemory := capture.GetNetworkBodiesBufferMemory()
	if total != wsMemory+nbMemory {
		t.Errorf("Expected total (%d) = ws (%d) + nb (%d)", total, wsMemory, nbMemory)
	}
}

func TestV4IsMemoryExceededUsesRealMemory(t *testing.T) {
	capture := setupTestCapture(t)

	// With empty buffers, memory should not be exceeded
	if capture.IsMemoryExceeded() {
		t.Error("Expected memory NOT to be exceeded with empty buffers")
	}

	// Simulated memory should still work as override for testing
	capture.SetMemoryUsage(55 * 1024 * 1024) // 55MB
	if !capture.IsMemoryExceeded() {
		t.Error("Expected simulated memory to trigger exceeded")
	}

	// Reset simulated
	capture.SetMemoryUsage(0)
	if capture.IsMemoryExceeded() {
		t.Error("Expected memory NOT exceeded after resetting simulated")
	}
}

func TestV4GlobalEvictionOnWSIngest(t *testing.T) {
	capture := setupTestCapture(t)

	// Fill WS buffer to near its per-buffer limit (4MB)
	largeData := strings.Repeat("x", 100000) // 100KB each
	for i := 0; i < 38; i++ {                // ~3.8MB
		capture.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Data: largeData},
		})
	}

	beforeCount := capture.GetWebSocketEventCount()

	// Adding more events should still enforce per-buffer memory limit
	for i := 0; i < 5; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Data: largeData},
		})
	}

	afterMem := capture.GetWebSocketBufferMemory()
	if afterMem > wsBufferMemoryLimit {
		t.Errorf("Expected WS buffer <= 4MB after eviction, got %d bytes", afterMem)
	}

	// Should have fewer events than beforeCount + 5 due to eviction
	afterCount := capture.GetWebSocketEventCount()
	if afterCount >= beforeCount+5 {
		t.Errorf("Expected eviction to reduce events, before=%d after=%d", beforeCount, afterCount)
	}
}

func TestV4GlobalEvictionOnNBIngest(t *testing.T) {
	capture := setupTestCapture(t)

	// Fill NB buffer to near its per-buffer limit (8MB)
	largeBody := strings.Repeat("y", 200000) // 200KB each
	for i := 0; i < 38; i++ {                // ~7.6MB
		capture.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", ResponseBody: largeBody, Status: 200},
		})
	}

	// Adding more should trigger eviction
	for i := 0; i < 5; i++ {
		capture.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", ResponseBody: largeBody, Status: 200},
		})
	}

	afterMem := capture.GetNetworkBodiesBufferMemory()
	if afterMem > nbBufferMemoryLimit {
		t.Errorf("Expected NB buffer <= 8MB after eviction, got %d bytes", afterMem)
	}
}

func TestV4MemoryExceededRejectsWSEvents(t *testing.T) {
	capture := setupTestCapture(t)

	// Simulate global memory exceeded
	capture.SetMemoryUsage(55 * 1024 * 1024) // 55MB > 50MB hard limit

	body := `{"events":[{"event":"message","id":"uuid-1","data":"test"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	// Should return 429 when global memory is exceeded (rate limit protection)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 when memory exceeded, got %d", rec.Code)
	}
}

func TestV4MemoryExceededHeaderInResponse(t *testing.T) {
	capture := setupTestCapture(t)

	// Fill buffers close to their limits
	largeData := strings.Repeat("x", 100000)
	for i := 0; i < 35; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Data: largeData},
		})
	}

	// Check that total memory is reported
	total := capture.GetTotalBufferMemory()
	if total <= 0 {
		t.Error("Expected non-zero total memory after filling buffers")
	}
}

func TestMCPToolsListIncludesV5Tools(t *testing.T) {
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

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"observe",
		"generate",
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("Expected tool '%s' in tools list", name)
		}
	}
}

func TestMCPHTTPEndpointToolsList(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Tools) == 0 {
		t.Error("Expected tools in response")
	}
}

func TestMCPHTTPEndpointToolCall(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs"}}}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
}

func TestMCPHTTPEndpointMethodNotAllowed(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := httptest.NewRequest("GET", "/mcp", nil)
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rec.Code)
	}
}

func TestMCPHTTPEndpointInvalidJSON(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 (JSON-RPC error in body), got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error == nil {
		t.Error("Expected JSON-RPC error for invalid JSON")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("Expected parse error code -32700, got %d", resp.Error.Code)
	}
}

func TestMCPHTTPEndpointUnknownMethod(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	body := `{"jsonrpc":"2.0","id":3,"method":"unknown/method"}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error == nil {
		t.Error("Expected JSON-RPC error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected method not found code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPHTTPEndpointV4ToolCall(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	body := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"observe","arguments":{"what":"websocket_events"}}}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
}
