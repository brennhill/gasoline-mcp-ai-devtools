// tools_observe_scope_test.go — Tests for scope filtering in errors/logs observe handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/tools/observe"
)

func TestGetBrowserErrors_InvalidScope(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{"scope": "bogus"})
	resp := observe.GetBrowserErrors(h, req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for invalid scope")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("expected invalid_param error, got: %s", result.Content[0].Text)
	}
}

func TestGetBrowserErrors_ValidScopes(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, scope := range []string{"current_page", "all", ""} {
		args, _ := json.Marshal(map[string]any{"scope": scope})
		resp := observe.GetBrowserErrors(h, req, args)
		var result MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("scope=%q unmarshal: %v", scope, err)
		}
		if result.IsError {
			t.Errorf("scope=%q should not error", scope)
		}
	}
}

func TestGetBrowserLogs_InvalidScope(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{"scope": "invalid"})
	resp := observe.GetBrowserLogs(h, req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for invalid scope")
	}
}

func TestGetBrowserLogs_ValidScopes(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, scope := range []string{"current_page", "all", ""} {
		args, _ := json.Marshal(map[string]any{"scope": scope})
		resp := observe.GetBrowserLogs(h, req, args)
		var result MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("scope=%q unmarshal: %v", scope, err)
		}
		if result.IsError {
			t.Errorf("scope=%q should not error", scope)
		}
	}
}

func TestGetBrowserErrors_ScopeInResponse(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	args, _ := json.Marshal(map[string]any{"scope": "all"})
	resp := observe.GetBrowserErrors(h, req, args)
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, `"scope":"all"`) {
		t.Error("expected scope=all in response")
	}
}
