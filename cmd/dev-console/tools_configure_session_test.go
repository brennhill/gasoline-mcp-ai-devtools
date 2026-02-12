// tools_configure_session_test.go — Coverage tests for load session and test boundary handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// toolLoadSessionContext — 58% → 100%
// ============================================

func TestToolLoadSessionContext_WithStore(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"load"}`)
	if !ok {
		t.Fatal("load should return result")
	}
	if result.IsError {
		t.Fatalf("load should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
}

func TestToolLoadSessionContext_Fallback(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	// Force fallback path by setting sessionStoreImpl to nil
	env.handler.sessionStoreImpl = nil

	args := json.RawMessage(`{"action":"load"}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolLoadSessionContext(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("load fallback should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
	if msg, _ := data["message"].(string); !strings.Contains(msg, "not initialized") {
		t.Fatalf("message = %q, want mention of not initialized", msg)
	}
}

// ============================================
// toolConfigureTestBoundaryEnd — 63% → 100%
// ============================================

func TestToolConfigureTestBoundaryEnd_Success(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"test_boundary_end","test_id":"test-123"}`)
	if !ok {
		t.Fatal("test_boundary_end should return result")
	}
	if result.IsError {
		t.Fatalf("test_boundary_end should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
	if testID, _ := data["test_id"].(string); testID != "test-123" {
		t.Fatalf("test_id = %q, want test-123", testID)
	}
	if wasActive, _ := data["was_active"].(bool); !wasActive {
		t.Fatal("was_active should be true")
	}
}

func TestToolConfigureTestBoundaryEnd_MissingTestID(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"test_boundary_end"}`)
	if !ok {
		t.Fatal("test_boundary_end should return result")
	}
	if !result.IsError {
		t.Fatal("test_boundary_end without test_id should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "test_id") {
		t.Errorf("error should mention test_id, got: %s", text)
	}
}

func TestToolConfigureTestBoundaryEnd_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolConfigureTestBoundaryEnd(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}
