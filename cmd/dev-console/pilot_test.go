package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestPilotToolSchemaExists verifies that pilot tool schemas are registered.
func TestPilotToolSchemaExists(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize MCP
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Get tools list
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	// Check for pilot tools
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{"highlight_element", "manage_state", "execute_javascript"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("Expected pilot tool %q to be in tools list", name)
		}
	}
}

// TestHighlightElementSchema validates the highlight_element tool schema.
func TestHighlightElementSchema(t *testing.T) {
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

	var highlightTool struct {
		Name        string                 `json:"name"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	}

	for _, tool := range result.Tools {
		if tool.Name == "highlight_element" {
			highlightTool.Name = tool.Name
			highlightTool.InputSchema = tool.InputSchema
			break
		}
	}

	if highlightTool.Name == "" {
		t.Fatal("highlight_element tool not found")
	}

	// Check schema has required properties
	props, ok := highlightTool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("highlight_element should have properties")
	}

	if _, ok := props["selector"]; !ok {
		t.Error("highlight_element should have 'selector' parameter")
	}

	if _, ok := props["duration_ms"]; !ok {
		t.Error("highlight_element should have 'duration_ms' parameter")
	}

	// Check selector is required
	required, ok := highlightTool.InputSchema["required"].([]interface{})
	if !ok {
		t.Fatal("highlight_element should have required array")
	}

	selectorRequired := false
	for _, r := range required {
		if r == "selector" {
			selectorRequired = true
			break
		}
	}

	if !selectorRequired {
		t.Error("selector should be required for highlight_element")
	}
}

// TestManageStateSchema validates the manage_state tool schema.
func TestManageStateSchema(t *testing.T) {
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

	var manageStateTool struct {
		Name        string                 `json:"name"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	}

	for _, tool := range result.Tools {
		if tool.Name == "manage_state" {
			manageStateTool.Name = tool.Name
			manageStateTool.InputSchema = tool.InputSchema
			break
		}
	}

	if manageStateTool.Name == "" {
		t.Fatal("manage_state tool not found")
	}

	props, ok := manageStateTool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("manage_state should have properties")
	}

	if _, ok := props["action"]; !ok {
		t.Error("manage_state should have 'action' parameter")
	}

	if _, ok := props["snapshot_name"]; !ok {
		t.Error("manage_state should have 'snapshot_name' parameter")
	}
}

// TestExecuteJavascriptSchema validates the execute_javascript tool schema.
func TestExecuteJavascriptSchema(t *testing.T) {
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

	var execJsTool struct {
		Name        string                 `json:"name"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	}

	for _, tool := range result.Tools {
		if tool.Name == "execute_javascript" {
			execJsTool.Name = tool.Name
			execJsTool.InputSchema = tool.InputSchema
			break
		}
	}

	if execJsTool.Name == "" {
		t.Fatal("execute_javascript tool not found")
	}

	props, ok := execJsTool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("execute_javascript should have properties")
	}

	if _, ok := props["script"]; !ok {
		t.Error("execute_javascript should have 'script' parameter")
	}

	if _, ok := props["timeout_ms"]; !ok {
		t.Error("execute_javascript should have 'timeout_ms' parameter")
	}

	// Check script is required
	required, ok := execJsTool.InputSchema["required"].([]interface{})
	if !ok {
		t.Fatal("execute_javascript should have required array")
	}

	scriptRequired := false
	for _, r := range required {
		if r == "script" {
			scriptRequired = true
			break
		}
	}

	if !scriptRequired {
		t.Error("script should be required for execute_javascript")
	}
}

// TestPilotToolsReturnTimeoutWithoutExtension tests that pilot tools return timeout error when no extension is connected.
// Note: The "ai_web_pilot_disabled" error is only returned when the extension explicitly reports it's disabled.
// Timeout != disabled. We don't guess - the extension actively tells the server when the toggle is off.
func TestPilotToolsReturnTimeoutWithoutExtension(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Without an extension connected, pilot tools should timeout (not return "disabled")
	tests := []struct {
		name   string
		tool   string
		params string
	}{
		{
			name:   "highlight_element returns timeout error",
			tool:   "highlight_element",
			params: `{"selector":".test","duration_ms":5000}`,
		},
		{
			name:   "manage_state returns timeout error",
			tool:   "manage_state",
			params: `{"action":"save","snapshot_name":"test"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := mcp.HandleRequest(JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "tools/call",
				Params:  json.RawMessage(`{"name":"` + tc.tool + `","arguments":` + tc.params + `}`),
			})

			var result MCPToolResult
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if len(result.Content) == 0 {
				t.Fatal("Expected content in response")
			}

			responseText := result.Content[0].Text

			// Should contain timeout error (NOT "ai_web_pilot_disabled" - that requires explicit extension response)
			if !strings.Contains(responseText, "Timeout") &&
				!strings.Contains(responseText, "timeout") &&
				!strings.Contains(responseText, "extension") {
				t.Errorf("Expected timeout error, got: %s", responseText)
			}
		})
	}
}

// TestExecuteJavaScriptTimeout tests that execute_javascript returns timeout error when no extension.
func TestExecuteJavaScriptTimeout(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	// Set a short timeout for testing
	capture.SetQueryTimeout(100 * time.Millisecond)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"execute_javascript","arguments":{"script":"return 1","timeout_ms":100}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	responseText := result.Content[0].Text

	// Should contain timeout error when no extension connected
	if !strings.Contains(responseText, "Timeout") &&
		!strings.Contains(responseText, "timeout") &&
		!strings.Contains(responseText, "AI Web Pilot") {
		t.Errorf("Expected timeout or connection error, got: %s", responseText)
	}
}

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
