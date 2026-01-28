package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// W7: Empty Result Hint Tests
// ============================================
// When an observe mode returns empty results, the response should include
// an actionable hint if a capture setting explains the empty result.

// --- Browser Logs ---

func TestToolGetBrowserLogs_EmptyWithErrorLevel(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Set log_level to "error" (the default, but explicit)
	mcp.toolHandler.captureOverrides.Set("log_level", "error")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "No browser logs found") {
		t.Error("Expected 'No browser logs found' message")
	}
	if !strings.Contains(text, `configure({action: "capture", settings: {log_level: "warn"}})`) {
		t.Error("Expected hint to suggest log_level warn")
	}
	if !strings.Contains(text, `configure({action: "capture", settings: {log_level: "all"}})`) {
		t.Error("Expected hint to suggest log_level all")
	}
}

func TestToolGetBrowserLogs_EmptyWithWarnLevel(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.captureOverrides.Set("log_level", "warn")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "No browser logs found") {
		t.Error("Expected 'No browser logs found' message")
	}
	if !strings.Contains(text, `configure({action: "capture", settings: {log_level: "all"}})`) {
		t.Error("Expected hint to suggest log_level all")
	}
}

func TestToolGetBrowserLogs_EmptyWithAllLevel(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.captureOverrides.Set("log_level", "all")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "No browser logs found") {
		t.Error("Expected 'No browser logs found' message")
	}
	// When log_level is "all", there's no higher setting to suggest
	if strings.Contains(text, "configure") {
		t.Error("Expected NO hint when log_level is 'all' — genuinely empty")
	}
}

func TestToolGetBrowserLogs_NonEmpty_NoHint(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Set restrictive level, but add matching entries
	mcp.toolHandler.captureOverrides.Set("log_level", "error")
	server.addEntries([]LogEntry{
		{"level": "error", "message": "test error"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	text := extractMCPText(t, resp)
	// Should NOT contain hint text when there ARE results
	if strings.Contains(text, "log_level is") {
		t.Error("Expected NO hint when results are present")
	}
}

// --- WebSocket Events ---

func TestToolGetWSEvents_EmptyWithOffHint(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.captureOverrides.Set("ws_mode", "off")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_events"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "No WebSocket events captured") {
		t.Error("Expected 'No WebSocket events captured' message")
	}
	if !strings.Contains(text, "WebSocket capture is OFF") {
		t.Error("Expected hint that WS capture is OFF")
	}
	if !strings.Contains(text, `configure({action: "capture", settings: {ws_mode: "lifecycle"}})`) {
		t.Error("Expected hint to suggest ws_mode lifecycle")
	}
}

func TestToolGetWSEvents_EmptyWithLifecycleNote(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.captureOverrides.Set("ws_mode", "lifecycle")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_events"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "No WebSocket events captured") {
		t.Error("Expected 'No WebSocket events captured' message")
	}
	if !strings.Contains(text, `configure({action: "capture", settings: {ws_mode: "messages"}})`) {
		t.Error("Expected hint to suggest ws_mode messages")
	}
}

func TestToolGetWSEvents_EmptyMessagesMode(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.captureOverrides.Set("ws_mode", "messages")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_events"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "No WebSocket events captured") {
		t.Error("Expected 'No WebSocket events captured' message")
	}
	// No hint when mode is already at maximum
	if strings.Contains(text, "configure") {
		t.Error("Expected NO hint when ws_mode is 'messages' — genuinely empty")
	}
}

// --- Network Bodies ---

func TestToolGetNetworkBodies_EmptyWithCaptureOff(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.captureOverrides.Set("network_bodies", "false")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network_bodies"}}`),
	})

	text := extractMCPText(t, resp)

	// Parse JSON response
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("Expected JSON response, got: %s", text)
	}

	// Verify empty state
	if count, ok := data["count"].(float64); !ok || count != 0 {
		t.Errorf("Expected count=0, got: %v", data["count"])
	}

	// Verify hint is present
	hint, ok := data["hint"].(string)
	if !ok {
		t.Error("Expected hint field when capture is OFF")
	}
	if !strings.Contains(hint, "Network body capture is OFF") {
		t.Error("Expected hint about network body capture being OFF")
	}
	if !strings.Contains(hint, `configure({action: "capture", settings: {network_bodies: "true"}})`) {
		t.Error("Expected hint to suggest enabling network_bodies")
	}
}

func TestToolGetNetworkBodies_EmptyWithCaptureOn(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// network_bodies defaults to "true", so no override needed
	// but let's be explicit
	mcp.toolHandler.captureOverrides.Set("network_bodies", "true")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network_bodies"}}`),
	})

	text := extractMCPText(t, resp)

	// Parse JSON response
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("Expected JSON response, got: %s", text)
	}

	// Verify empty state
	if count, ok := data["count"].(float64); !ok || count != 0 {
		t.Errorf("Expected count=0, got: %v", data["count"])
	}

	// When capture is on but no bodies found, hint about tab tracking
	hint, ok := data["hint"].(string)
	if !ok {
		t.Error("Expected tab-tracking hint when network_bodies is on but empty")
	} else if !strings.Contains(hint, "Track This Tab") {
		t.Errorf("Expected hint to mention tab tracking, got: %s", hint)
	}
}

// --- Enhanced Actions ---

func TestToolGetActions_EmptyWithReplayOff(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.captureOverrides.Set("action_replay", "false")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"actions"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "No") {
		t.Error("Expected empty result message")
	}
	if !strings.Contains(text, "Action replay capture is OFF") {
		t.Error("Expected hint that action replay is OFF")
	}
	if !strings.Contains(text, `configure({action: "capture", settings: {action_replay: "true"}})`) {
		t.Error("Expected hint to suggest action_replay true")
	}
}

func TestToolGetActions_EmptyWithReplayOn(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// action_replay defaults to "true"
	mcp.toolHandler.captureOverrides.Set("action_replay", "true")

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"actions"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "No") {
		t.Error("Expected empty result message")
	}
	// No hint when capture is on
	if strings.Contains(text, "configure") {
		t.Error("Expected NO hint when action_replay is 'true' — genuinely empty")
	}
}
