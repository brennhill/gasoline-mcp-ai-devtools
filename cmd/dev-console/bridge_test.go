// bridge_test.go — Tests for bridge-layer helpers.
package main

import (
	"encoding/json"
	"testing"
)

// ============================================
// extractToolAction tests
// ============================================

func TestExtractToolAction_ConfigureRestart(t *testing.T) {
	params, _ := json.Marshal(map[string]any{
		"name":      "configure",
		"arguments": map[string]any{"action": "restart"},
	})
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  params,
	}
	tool, action := extractToolAction(req)
	if tool != "configure" {
		t.Errorf("tool = %q, want %q", tool, "configure")
	}
	if action != "restart" {
		t.Errorf("action = %q, want %q", action, "restart")
	}
}

func TestExtractToolAction_ConfigureHealth(t *testing.T) {
	params, _ := json.Marshal(map[string]any{
		"name":      "configure",
		"arguments": map[string]any{"action": "health"},
	})
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(2),
		Method:  "tools/call",
		Params:  params,
	}
	tool, action := extractToolAction(req)
	if tool != "configure" {
		t.Errorf("tool = %q, want %q", tool, "configure")
	}
	if action != "health" {
		t.Errorf("action = %q, want %q", action, "health")
	}
}

func TestExtractToolAction_NonConfigure(t *testing.T) {
	cases := []struct {
		name   string
		method string
		tool   string
	}{
		{"observe", "tools/call", "observe"},
		{"interact", "tools/call", "interact"},
		{"analyze", "tools/call", "analyze"},
		{"generate", "tools/call", "generate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params, _ := json.Marshal(map[string]any{
				"name":      tc.tool,
				"arguments": map[string]any{"what": "errors"},
			})
			req := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      float64(1),
				Method:  tc.method,
				Params:  params,
			}
			tool, action := extractToolAction(req)
			if tool != tc.tool {
				t.Errorf("tool = %q, want %q", tool, tc.tool)
			}
			if action != "" {
				t.Errorf("action = %q, want empty (no action field)", action)
			}
		})
	}
}

func TestExtractToolAction_NonToolsCall(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}
	tool, action := extractToolAction(req)
	if tool != "" {
		t.Errorf("tool = %q, want empty for non-tools/call", tool)
	}
	if action != "" {
		t.Errorf("action = %q, want empty for non-tools/call", action)
	}
}

func TestExtractToolAction_MalformedJSON(t *testing.T) {
	cases := []struct {
		name   string
		params json.RawMessage
	}{
		{"nil params", nil},
		{"empty params", json.RawMessage(`{}`)},
		{"invalid JSON", json.RawMessage(`{not json}`)},
		{"missing name", json.RawMessage(`{"arguments":{"action":"restart"}}`)},
		{"missing arguments", json.RawMessage(`{"name":"configure"}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      float64(1),
				Method:  "tools/call",
				Params:  tc.params,
			}
			tool, action := extractToolAction(req)
			// Should not panic, may return empty strings
			if tool == "configure" && action == "restart" {
				t.Error("should not extract configure+restart from malformed input")
			}
		})
	}
}

func TestExtractToolAction_ConfigureNoAction(t *testing.T) {
	params, _ := json.Marshal(map[string]any{
		"name":      "configure",
		"arguments": map[string]any{"buffer": "all"},
	})
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  params,
	}
	tool, action := extractToolAction(req)
	if tool != "configure" {
		t.Errorf("tool = %q, want %q", tool, "configure")
	}
	if action != "" {
		t.Errorf("action = %q, want empty (no action in args)", action)
	}
}
