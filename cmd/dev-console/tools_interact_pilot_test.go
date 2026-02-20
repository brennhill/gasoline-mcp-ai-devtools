// tools_interact_pilot_test.go — Coverage tests for navigate and executeJS success paths.
package main

import (
	"encoding/json"
	"testing"
)

// ============================================
// handleBrowserActionNavigate — success + invalidJSON
// (pilot disabled + missing URL already covered by tools_interact_audit_test.go)
// ============================================

func TestHandleBrowserActionNavigate_Success(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com/page"}`)
	if !ok {
		t.Fatal("navigate should return result")
	}
	if result.IsError {
		t.Fatalf("navigate should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("navigate should create a pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("pending query type = %q, want browser_action", pq.Type)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "queued" {
		t.Fatalf("status = %q, want queued", status)
	}
	if _, ok := data["correlation_id"]; !ok {
		t.Error("response should contain correlation_id")
	}
}

func TestHandleBrowserActionNavigate_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.handleBrowserActionNavigate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// handlePilotExecuteJS — success + invalidJSON
// (pilot disabled, missing script, invalid/valid worlds already covered by audit tests)
// ============================================

func TestHandlePilotExecuteJS_Success(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"execute_js","script":"document.title"}`)
	if !ok {
		t.Fatal("execute_js should return result")
	}
	if result.IsError {
		t.Fatalf("execute_js should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "queued" {
		t.Fatalf("status = %q, want queued", status)
	}
	if _, ok := data["correlation_id"]; !ok {
		t.Error("response should contain correlation_id")
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("execute_js should create a pending query")
	}
	if pq.Type != "execute" {
		t.Fatalf("pending query type = %q, want execute", pq.Type)
	}
}

func TestHandlePilotExecuteJS_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.handlePilotExecuteJS(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}
