// tools_test.go — Tests for MCP tool handlers.
// Covers core functionality: tool dispatch, error handling, and response formatting.
package main

import (
	"encoding/json"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// NewToolHandler Tests
// ============================================

func TestNewToolHandler(t *testing.T) {
	// Create minimal dependencies
	server, err := NewServer("/tmp/test-gasoline.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	cap := capture.NewCapture()
	registry := NewSSERegistry()

	// Create handler
	handler := NewToolHandler(server, cap, registry)

	if handler == nil {
		t.Fatal("NewToolHandler returned nil")
	}
	if handler.server != server {
		t.Error("MCPHandler.server not set correctly")
	}
}

// ============================================
// HandleToolCall Tests
// ============================================

func TestHandleToolCall_UnknownTool(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)

	// Get the tool handler
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	// Try to handle an unknown tool
	resp, handled := toolHandler.HandleToolCall(req, "unknown_tool", json.RawMessage(`{}`))

	if handled {
		t.Error("Expected handler to NOT handle unknown tool")
	}
	if resp.JSONRPC != "" {
		t.Error("Expected empty response for unhandled tool")
	}
}

func TestHandleToolCall_ObserveTool(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"what": "logs"}`)
	resp, handled := toolHandler.HandleToolCall(req, "observe", args)

	if !handled {
		t.Error("Expected handler to handle observe tool")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", resp.JSONRPC)
	}

	// Result should be valid JSON
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Errorf("Invalid result JSON: %v", err)
	}
}

func TestHandleToolCall_GenerateTool(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"format": "reproduction"}`)
	resp, handled := toolHandler.HandleToolCall(req, "generate", args)

	if !handled {
		t.Error("Expected handler to handle generate tool")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", resp.JSONRPC)
	}
}

func TestHandleToolCall_ConfigureTool(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"action": "health"}`)
	resp, handled := toolHandler.HandleToolCall(req, "configure", args)

	if !handled {
		t.Error("Expected handler to handle configure tool")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", resp.JSONRPC)
	}
}

func TestHandleToolCall_InteractTool(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	// list_states doesn't require pilot to be enabled
	args := json.RawMessage(`{"action": "list_states"}`)
	resp, handled := toolHandler.HandleToolCall(req, "interact", args)

	if !handled {
		t.Error("Expected handler to handle interact tool")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", resp.JSONRPC)
	}
}

// ============================================
// Observe Mode Tests
// ============================================

func TestToolObserve_MissingWhat(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolObserve(req, json.RawMessage(`{}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error response for missing 'what' parameter")
	}
}

func TestToolObserve_UnknownMode(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolObserve(req, json.RawMessage(`{"what": "invalid_mode"}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error response for unknown mode")
	}
}

func TestToolObserve_NetworkBodies(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolObserve(req, json.RawMessage(`{"what": "network_bodies"}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result.IsError {
		t.Error("Did not expect error for network_bodies mode")
	}
}

// ============================================
// Generate Mode Tests
// ============================================

func TestToolGenerate_MissingFormat(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolGenerate(req, json.RawMessage(`{}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error response for missing 'format' parameter")
	}
}

func TestToolGenerate_UnknownFormat(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolGenerate(req, json.RawMessage(`{"format": "invalid_format"}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error response for unknown format")
	}
}

// ============================================
// Configure Mode Tests
// ============================================

func TestToolConfigure_MissingAction(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolConfigure(req, json.RawMessage(`{}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error response for missing 'action' parameter")
	}
}

func TestToolConfigure_UnknownAction(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolConfigure(req, json.RawMessage(`{"action": "invalid_action"}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error response for unknown action")
	}
}

func TestToolConfigure_Health(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolConfigure(req, json.RawMessage(`{"action": "health"}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result.IsError {
		t.Error("Did not expect error for health action")
	}
}

// ============================================
// Interact Mode Tests
// ============================================

func TestToolInteract_MissingAction(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolInteract(req, json.RawMessage(`{}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error response for missing 'action' parameter")
	}
}

func TestToolInteract_UnknownAction(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolInteract(req, json.RawMessage(`{"action": "invalid_action"}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error response for unknown action")
	}
}

// ============================================
// ToolsList Tests
// ============================================

func TestToolsList(t *testing.T) {
	server, _ := NewServer("/tmp/test-gasoline.jsonl", 100)
	cap := capture.NewCapture()
	registry := NewSSERegistry()
	mcpHandler := NewToolHandler(server, cap, registry)
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	tools := toolHandler.ToolsList()

	if len(tools) != 4 {
		t.Errorf("Expected 4 tools, got %d", len(tools))
	}

	// Check tool names
	expectedTools := map[string]bool{
		"observe":   false,
		"generate":  false,
		"configure": false,
		"interact":  false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Expected tool '%s' not found in ToolsList", name)
		}
	}
}

// ============================================
// Response Helper Tests
// ============================================

func TestMcpTextResponse(t *testing.T) {
	resp := mcpTextResponse("Hello, World!")

	var result MCPToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(result.Content))
	}

	if result.Content[0].Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", result.Content[0].Type)
	}

	if result.Content[0].Text != "Hello, World!" {
		t.Errorf("Expected text 'Hello, World!', got '%s'", result.Content[0].Text)
	}

	if result.IsError {
		t.Error("Expected IsError to be false")
	}
}

func TestMcpErrorResponse(t *testing.T) {
	resp := mcpErrorResponse("Something went wrong")

	var result MCPToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}

	if result.Content[0].Text != "Something went wrong" {
		t.Errorf("Expected error text, got '%s'", result.Content[0].Text)
	}
}

func TestMcpJSONResponse(t *testing.T) {
	data := map[string]any{
		"status": "ok",
		"count":  42,
	}
	resp := mcpJSONResponse("Test summary", data)

	var result MCPToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(result.Content))
	}

	// Text should contain the summary and JSON
	text := result.Content[0].Text
	if text == "" {
		t.Error("Expected non-empty text")
	}

	// Should start with summary
	if len(text) < 12 || text[:12] != "Test summary" {
		t.Error("Expected text to start with summary")
	}
}

func TestMcpStructuredError(t *testing.T) {
	resp := mcpStructuredError(ErrMissingParam, "Missing parameter 'what'", "Add the 'what' parameter", withParam("what"), withHint("Valid values: logs, errors"))

	var result MCPToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}

	// Check that the text contains the error code
	text := result.Content[0].Text
	if text == "" {
		t.Error("Expected non-empty text")
	}

	// Should contain the error code and retry instruction
	if !containsSubstring(text, "missing_param") {
		t.Error("Expected text to contain error code 'missing_param'")
	}
}

// ============================================
// Rate Limiter Tests
// ============================================

func TestToolCallLimiter_Allow(t *testing.T) {
	limiter := NewToolCallLimiter(3, 1000) // 3 calls per second

	// First 3 calls should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Errorf("Call %d should be allowed", i+1)
		}
	}

	// 4th call should be rate limited
	if limiter.Allow() {
		t.Error("4th call should be rate limited")
	}
}

// ============================================
// Helper Functions
// ============================================

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
