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

	result, ok := env.callInteract(t, `{"what":"back"}`)
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

	var params map[string]string
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["action"] != "back" {
		t.Fatalf("params action = %q, want back", params["action"])
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

	result, ok := env.callInteract(t, `{"what":"forward"}`)
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

	var params map[string]string
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["action"] != "forward" {
		t.Fatalf("params action = %q, want forward", params["action"])
	}
}

// ============================================
// handleBrowserActionNewTab — success + edge cases
// ============================================

func TestHandleBrowserActionNewTab_Success(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"new_tab","url":"https://example.com"}`)
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
	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if action, _ := params["action"].(string); action != "new_tab" {
		t.Fatalf("params action = %q, want new_tab", action)
	}
	if url, _ := params["url"].(string); url != "https://example.com" {
		t.Fatalf("params url = %q, want https://example.com", url)
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
	result, ok := env.callInteract(t, `{"what":"new_tab"}`)
	if !ok {
		t.Fatal("new_tab without url should return result")
	}
	if result.IsError {
		t.Fatalf("new_tab without url should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("new_tab without url should create a pending query")
	}
	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if action, _ := params["action"].(string); action != "new_tab" {
		t.Fatalf("params action = %q, want new_tab", action)
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

func TestHandleBrowserActionSwitchTab_WithTabID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"switch_tab","tab_id":42}`)
	if !ok {
		t.Fatal("switch_tab should return result")
	}
	if result.IsError {
		t.Fatalf("switch_tab should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("switch_tab should create a pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("pending query type = %q, want browser_action", pq.Type)
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if action, _ := params["action"].(string); action != "switch_tab" {
		t.Fatalf("params action = %q, want switch_tab", action)
	}
	if tabID, _ := params["tab_id"].(float64); int(tabID) != 42 {
		t.Fatalf("params tab_id = %v, want 42", params["tab_id"])
	}
}

func TestHandleBrowserActionSwitchTab_WithTabIndex(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"switch_tab","tab_index":1}`)
	if !ok {
		t.Fatal("switch_tab should return result")
	}
	if result.IsError {
		t.Fatalf("switch_tab should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("switch_tab should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if action, _ := params["action"].(string); action != "switch_tab" {
		t.Fatalf("params action = %q, want switch_tab", action)
	}
	if tabIndex, _ := params["tab_index"].(float64); int(tabIndex) != 1 {
		t.Fatalf("params tab_index = %v, want 1", params["tab_index"])
	}
}

func TestHandleBrowserActionCloseTab_WithTabID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"close_tab","tab_id":55}`)
	if !ok {
		t.Fatal("close_tab should return result")
	}
	if result.IsError {
		t.Fatalf("close_tab should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("close_tab should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if action, _ := params["action"].(string); action != "close_tab" {
		t.Fatalf("params action = %q, want close_tab", action)
	}
	if tabID, _ := params["tab_id"].(float64); int(tabID) != 55 {
		t.Fatalf("params tab_id = %v, want 55", params["tab_id"])
	}
}

func TestHandleBrowserActionNavigate_NewTabFlagPreserved(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com/path","new_tab":true}`)
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

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if action, _ := params["action"].(string); action != "navigate" {
		t.Fatalf("params action = %q, want navigate", action)
	}
	if newTab, _ := params["new_tab"].(bool); !newTab {
		t.Fatalf("params new_tab = %v, want true", params["new_tab"])
	}
}
