package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestErrPilotDisabledMessage tests the error message format.
func TestErrPilotDisabledMessage(t *testing.T) {
	if ErrPilotDisabled == nil {
		t.Fatal("ErrPilotDisabled should be defined")
	}

	errMsg := ErrPilotDisabled.Error()

	if !strings.Contains(errMsg, "ai_web_pilot_disabled") {
		t.Errorf("Error message should contain 'ai_web_pilot_disabled', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "AI Web Pilot") {
		t.Errorf("Error message should mention 'AI Web Pilot', got: %s", errMsg)
	}
}

// ============================================
// Pilot Status Tests (NEW)
// ============================================

// TestObservePilotMode verifies that observe tool supports "pilot" mode.
func TestObservePilotMode(t *testing.T) {
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
			Name        string                 `json:"name"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	// Find observe tool
	var observeTool struct {
		Name        string                 `json:"name"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	}

	for _, tool := range result.Tools {
		if tool.Name == "observe" {
			observeTool.Name = tool.Name
			observeTool.InputSchema = tool.InputSchema
			break
		}
	}

	if observeTool.Name == "" {
		t.Fatal("observe tool not found")
	}

	// Check that "pilot" is in the what enum
	props, ok := observeTool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("observe should have properties")
	}

	whatProp, ok := props["what"].(map[string]interface{})
	if !ok {
		t.Fatal("observe should have 'what' parameter")
	}

	enum, ok := whatProp["enum"].([]interface{})
	if !ok {
		t.Fatal("what parameter should have enum")
	}

	hasPilot := false
	for _, val := range enum {
		if val == "pilot" {
			hasPilot = true
			break
		}
	}

	if !hasPilot {
		t.Error("'pilot' should be in observe 'what' enum")
	}
}

// TestObservePilotResponseSchema verifies the response schema of observe {what: "pilot"}.
func TestObservePilotResponseSchema(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call observe tool with what: "pilot"
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"pilot"}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	// Parse the JSON response
	var statusResp struct {
		Enabled            bool   `json:"enabled"`
		Source             string `json:"source"`
		ExtensionConnected bool   `json:"extension_connected"`
		LastPollAgo        string `json:"last_poll_ago,omitempty"`
		LastUpdate         string `json:"last_update,omitempty"`
	}

	pilotText := result.Content[0].Text
	pilotJSON := pilotText
	if pLines := strings.SplitN(pilotText, "\n", 2); len(pLines) == 2 {
		pilotJSON = pLines[1]
	}
	if err := json.Unmarshal([]byte(pilotJSON), &statusResp); err != nil {
		t.Fatalf("Failed to parse pilot status response: %v", err)
	}

	// Verify required fields exist
	if statusResp.Source == "" {
		t.Error("Source field should not be empty")
	}

	validSources := map[string]bool{
		"extension_poll":    true,
		"stale":             true,
		"never_connected":   true,
	}
	if !validSources[statusResp.Source] {
		t.Errorf("Source should be one of: extension_poll, stale, never_connected. Got: %s", statusResp.Source)
	}
}

// TestGetPilotStatusWithExtensionConnected verifies enabled state when extension polled recently.
func TestGetPilotStatusWithExtensionConnected(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Simulate extension poll by updating lastPollAt and setting pilotEnabled
	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"pilot"}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	var statusResp struct {
		Enabled            bool   `json:"enabled"`
		Source             string `json:"source"`
		ExtensionConnected bool   `json:"extension_connected"`
	}

	pilotText := result.Content[0].Text
	pilotJSON := pilotText
	if pLines := strings.SplitN(pilotText, "\n", 2); len(pLines) == 2 {
		pilotJSON = pLines[1]
	}
	if err := json.Unmarshal([]byte(pilotJSON), &statusResp); err != nil {
		t.Fatalf("Failed to parse pilot status response: %v", err)
	}

	if !statusResp.Enabled {
		t.Error("Expected enabled=true when pilotEnabled is true")
	}

	if statusResp.Source != "extension_poll" {
		t.Errorf("Expected source='extension_poll' for recent poll, got: %s", statusResp.Source)
	}

	if !statusResp.ExtensionConnected {
		t.Error("Expected extension_connected=true for recent poll")
	}
}

// TestGetPilotStatusStale verifies source='stale' when last poll is old.
func TestGetPilotStatusStale(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Simulate extension poll that happened 5 seconds ago
	capture.mu.Lock()
	capture.lastPollAt = time.Now().Add(-5 * time.Second)
	capture.pilotEnabled = true
	capture.mu.Unlock()

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"pilot"}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	var statusResp struct {
		Enabled            bool   `json:"enabled"`
		Source             string `json:"source"`
		ExtensionConnected bool   `json:"extension_connected"`
	}

	pilotText := result.Content[0].Text
	pilotJSON := pilotText
	if pLines := strings.SplitN(pilotText, "\n", 2); len(pLines) == 2 {
		pilotJSON = pLines[1]
	}
	if err := json.Unmarshal([]byte(pilotJSON), &statusResp); err != nil {
		t.Fatalf("Failed to parse pilot status response: %v", err)
	}

	if statusResp.Source != "stale" {
		t.Errorf("Expected source='stale' for old poll, got: %s", statusResp.Source)
	}

	if statusResp.ExtensionConnected {
		t.Error("Expected extension_connected=false for stale poll")
	}
}

// TestGetPilotStatusNeverConnected verifies source='never_connected' when no polls.
func TestGetPilotStatusNeverConnected(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Don't set lastPollAt - leave it as zero value
	// Verify it's zero
	capture.mu.RLock()
	isZero := capture.lastPollAt.IsZero()
	capture.mu.RUnlock()

	if !isZero {
		t.Fatal("Expected lastPollAt to be zero value for fresh capture")
	}

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"pilot"}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	var statusResp struct {
		Source string `json:"source"`
	}

	pilotText := result.Content[0].Text
	pilotJSON := pilotText
	if pLines := strings.SplitN(pilotText, "\n", 2); len(pLines) == 2 {
		pilotJSON = pLines[1]
	}
	if err := json.Unmarshal([]byte(pilotJSON), &statusResp); err != nil {
		t.Fatalf("Failed to parse pilot status response: %v", err)
	}

	if statusResp.Source != "never_connected" {
		t.Errorf("Expected source='never_connected' for zero lastPollAt, got: %s", statusResp.Source)
	}
}

// TestGetPilotStatusThreadSafety verifies concurrent access is safe.
func TestGetPilotStatusThreadSafety(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Simulate concurrent access from multiple goroutines
	done := make(chan bool, 200)

	// Goroutines that call the MCP tool
	for i := 0; i < 100; i++ {
		go func() {
			resp := mcp.HandleRequest(JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "tools/call",
				Params:  json.RawMessage(`{"name":"get_pilot_status","arguments":{}}`),
			})
			var result MCPToolResult
			_ = json.Unmarshal(resp.Result, &result)
			done <- true
		}()
	}

	// Goroutines that modify capture state
	for i := 0; i < 100; i++ {
		go func(idx int) {
			capture.mu.Lock()
			capture.lastPollAt = time.Now()
			capture.pilotEnabled = idx%2 == 0
			capture.mu.Unlock()
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < 200; i++ {
		<-done
	}
	// If we get here without a race condition, test passes
}

// TestHealthResponseIncludesPilot verifies that get_health includes pilot field.
func TestHealthResponseIncludesPilot(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Set pilot state
	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	// Call toolGetHealth directly (it's a helper function, not an MCP tool)
	resp := mcp.toolHandler.toolGetHealth(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
	})

	if resp.Error != nil {
		t.Fatalf("toolGetHealth returned error: %d %s", resp.Error.Code, resp.Error.Message)
	}

	if len(resp.Result) == 0 {
		t.Fatal("toolGetHealth response has empty result")
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse response: %v (raw: %s)", err, string(resp.Result))
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in health response")
	}

	// Strip summary line before parsing JSON
	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var healthResp map[string]interface{}
	if err := json.Unmarshal([]byte(jsonPart), &healthResp); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	// Verify pilot field exists
	if _, ok := healthResp["pilot"]; !ok {
		t.Error("Health response should include 'pilot' field")
	}

	// Verify pilot field structure
	if pilotData, ok := healthResp["pilot"]; ok {
		pilotMap, ok := pilotData.(map[string]interface{})
		if !ok {
			t.Fatal("pilot field should be an object")
		}

		if _, ok := pilotMap["enabled"]; !ok {
			t.Error("pilot.enabled field should exist")
		}

		if _, ok := pilotMap["source"]; !ok {
			t.Error("pilot.source field should exist")
		}

		if _, ok := pilotMap["extension_connected"]; !ok {
			t.Error("pilot.extension_connected field should exist")
		}
	}
}
