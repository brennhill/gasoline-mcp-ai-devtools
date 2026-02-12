// tools_interact_coverage_test.go — Coverage tests for interact sub-handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// handleSubtitle — 0% → 100%
// ============================================

func TestHandleSubtitle_SetText(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"subtitle","text":"Hello world"}`)
	if !ok {
		t.Fatal("subtitle should return result")
	}
	if result.IsError {
		t.Fatalf("subtitle should not error, got: %s", result.Content[0].Text)
	}

	// Verify pending query was created
	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("subtitle should create a pending query")
	}
	if pq.Type != "subtitle" {
		t.Fatalf("pending query type = %q, want subtitle", pq.Type)
	}

	// Verify response mentions subtitle
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "subtitle") {
		t.Errorf("response should mention subtitle, got: %s", text)
	}
}

func TestHandleSubtitle_ClearText(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"subtitle","text":""}`)
	if !ok {
		t.Fatal("subtitle clear should return result")
	}
	if result.IsError {
		t.Fatalf("subtitle clear should not error, got: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "clear") {
		t.Errorf("response should mention clear, got: %s", text)
	}
}

func TestHandleSubtitle_MissingText(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"subtitle"}`)
	if !ok {
		t.Fatal("subtitle missing text should return result")
	}
	if !result.IsError {
		t.Fatal("subtitle without text should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "text") {
		t.Errorf("error should mention 'text' param, got: %s", text)
	}
}

func TestHandleSubtitle_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{invalid}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.handleSubtitle(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// handleListInteractive — 40% → 80%+
// ============================================

func TestHandleListInteractive_PilotEnabled(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"list_interactive"}`)
	if !ok {
		t.Fatal("list_interactive should return result")
	}
	if result.IsError {
		t.Fatalf("list_interactive with pilot enabled should not error, got: %s", result.Content[0].Text)
	}

	// Verify pending query was created
	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("list_interactive should create a pending query")
	}
	if pq.Type != "dom_action" {
		t.Fatalf("pending query type = %q, want dom_action", pq.Type)
	}

	// Verify response mentions queued
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "queued") {
		t.Errorf("response should mention queued, got: %s", text)
	}
}

func TestHandleListInteractive_PilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	// Pilot disabled by default

	result, ok := env.callInteract(t, `{"action":"list_interactive"}`)
	if !ok {
		t.Fatal("list_interactive should return result")
	}
	if !result.IsError {
		t.Fatal("list_interactive with pilot disabled should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "pilot") {
		t.Errorf("error should mention pilot, got: %s", text)
	}
}

func TestHandleListInteractive_WithTabID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"list_interactive","tab_id":42}`)
	if !ok {
		t.Fatal("list_interactive with tab_id should return result")
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("should create pending query")
	}
	if pq.TabID != 42 {
		t.Fatalf("pending query TabID = %d, want 42", pq.TabID)
	}
}

func TestHandleListInteractive_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.handleListInteractive(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// handlePilotHighlight — 50% → 80%+
// ============================================

func TestHandlePilotHighlight_Success(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"highlight","selector":"#main"}`)
	if !ok {
		t.Fatal("highlight should return result")
	}
	if result.IsError {
		t.Fatalf("highlight with pilot enabled should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("highlight should create a pending query")
	}
	if pq.Type != "highlight" {
		t.Fatalf("pending query type = %q, want highlight", pq.Type)
	}
}

func TestHandlePilotHighlight_MissingSelector(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"highlight"}`)
	if !ok {
		t.Fatal("highlight without selector should return result")
	}
	if !result.IsError {
		t.Fatal("highlight without selector should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "selector") {
		t.Errorf("error should mention selector, got: %s", text)
	}
}

func TestHandlePilotHighlight_PilotDisabledWithSelector(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	// Pilot disabled by default

	result, ok := env.callInteract(t, `{"action":"highlight","selector":"div"}`)
	if !ok {
		t.Fatal("highlight should return result")
	}
	if !result.IsError {
		t.Fatal("highlight with pilot disabled should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "pilot") {
		t.Errorf("error should mention pilot, got: %s", text)
	}
}

func TestHandlePilotHighlight_WithTabID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"highlight","selector":".btn","tab_id":99}`)
	if !ok {
		t.Fatal("highlight with tab_id should return result")
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("should create pending query")
	}
	if pq.TabID != 99 {
		t.Fatalf("TabID = %d, want 99", pq.TabID)
	}
}

func TestHandlePilotHighlight_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{bad}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.handlePilotHighlight(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}
