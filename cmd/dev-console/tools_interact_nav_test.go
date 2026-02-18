// tools_interact_nav_test.go — Coverage tests for back/forward/newTab success paths.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// handleBrowserActionBack — success path
// (pilot disabled already covered by tools_interact_audit_test.go)
// ============================================

func TestHandleBrowserActionBack_Success(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"back"}`)
	if !ok {
		t.Fatal("back should return result")
	}
	if result.IsError {
		t.Fatalf("back should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("back should create a pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("pending query type = %q, want browser_action", pq.Type)
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["action"] != "back" {
		t.Fatalf("params action = %v, want back", params["action"])
	}

	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "queued") {
		t.Errorf("response should mention queued, got: %s", text)
	}
}

// ============================================
// handleBrowserActionForward — success path
// ============================================

func TestHandleBrowserActionForward_Success(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"forward"}`)
	if !ok {
		t.Fatal("forward should return result")
	}
	if result.IsError {
		t.Fatalf("forward should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("forward should create a pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("pending query type = %q, want browser_action", pq.Type)
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["action"] != "forward" {
		t.Fatalf("params action = %v, want forward", params["action"])
	}
}

// ============================================
// handleBrowserActionNewTab — success + edge cases
// ============================================

func TestHandleBrowserActionNewTab_Success(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"new_tab","url":"https://example.com"}`)
	if !ok {
		t.Fatal("new_tab should return result")
	}
	if result.IsError {
		t.Fatalf("new_tab should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("new_tab should create a pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("pending query type = %q, want browser_action", pq.Type)
	}

	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "queued") {
		t.Errorf("response should mention queued, got: %s", text)
	}
}

func TestHandleBrowserActionNewTab_NoURL(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// URL is optional for new_tab
	result, ok := env.callInteract(t, `{"action":"new_tab"}`)
	if !ok {
		t.Fatal("new_tab without url should return result")
	}
	if result.IsError {
		t.Fatalf("new_tab without url should not error, got: %s", result.Content[0].Text)
	}
}

func TestHandleBrowserActionNewTab_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.handleBrowserActionNewTab(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// CR-5: refresh/back/forward must forward user args
// ============================================

func TestCR5_Refresh_ForwardsUserArgs(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// User provides timeout_ms and reason alongside action
	result, ok := env.callInteract(t, `{"action":"refresh","timeout_ms":10000,"reason":"testing refresh"}`)
	if !ok {
		t.Fatal("refresh should return result")
	}
	if result.IsError {
		t.Fatalf("refresh should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("refresh should create a pending query")
	}

	var params map[string]json.RawMessage
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	// Action must be present
	if string(params["action"]) != `"refresh"` {
		t.Fatalf("params action = %s, want \"refresh\"", params["action"])
	}

	// User's timeout_ms must be forwarded, not dropped
	if string(params["timeout_ms"]) != "10000" {
		t.Errorf("params timeout_ms = %s, want 10000 — user arg was dropped", params["timeout_ms"])
	}

	// User's reason must be forwarded
	if string(params["reason"]) != `"testing refresh"` {
		t.Errorf("params reason = %s, want \"testing refresh\" — user arg was dropped", params["reason"])
	}
}

func TestCR5_Back_ForwardsUserArgs(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"back","timeout_ms":5000}`)
	if !ok {
		t.Fatal("back should return result")
	}
	if result.IsError {
		t.Fatalf("back should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("back should create a pending query")
	}

	var params map[string]json.RawMessage
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	if string(params["timeout_ms"]) != "5000" {
		t.Errorf("params timeout_ms = %s, want 5000 — user arg was dropped", params["timeout_ms"])
	}
}

func TestCR5_Forward_ForwardsUserArgs(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"forward","timeout_ms":5000}`)
	if !ok {
		t.Fatal("forward should return result")
	}
	if result.IsError {
		t.Fatalf("forward should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("forward should create a pending query")
	}

	var params map[string]json.RawMessage
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	if string(params["timeout_ms"]) != "5000" {
		t.Errorf("params timeout_ms = %s, want 5000 — user arg was dropped", params["timeout_ms"])
	}
}
