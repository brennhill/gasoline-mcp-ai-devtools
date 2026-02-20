// tools_configure_capabilities_test.go — Tests for describe_capabilities handler.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDescribeCapabilities_ResponseStructure(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.handleDescribeCapabilities(req, json.RawMessage(`{}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatal("expected non-error response")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}

	text := result.Content[0].Text

	// Must contain the 5 core tools
	for _, tool := range []string{"observe", "generate", "configure", "interact", "analyze"} {
		if !strings.Contains(text, `"`+tool+`"`) {
			t.Errorf("expected tool %q in capabilities", tool)
		}
	}

	// Must contain version and protocol_version
	if !strings.Contains(text, `"version"`) {
		t.Error("expected version field")
	}
	if !strings.Contains(text, `"protocol_version"`) {
		t.Error("expected protocol_version field")
	}

	// Parse deeper to check structure
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("no JSON in response")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse capabilities JSON: %v", err)
	}

	tools, ok := data["tools"].(map[string]any)
	if !ok {
		t.Fatal("expected tools map")
	}

	// Each tool should have dispatch_param, modes, params, description
	for name, toolData := range tools {
		td, ok := toolData.(map[string]any)
		if !ok {
			t.Errorf("tool %s: expected map", name)
			continue
		}
		if _, ok := td["dispatch_param"]; !ok {
			t.Errorf("tool %s: missing dispatch_param", name)
		}
		if _, ok := td["description"]; !ok {
			t.Errorf("tool %s: missing description", name)
		}
		if _, ok := td["params"]; !ok {
			t.Errorf("tool %s: missing params", name)
		}
	}
}

func TestDescribeCapabilities_ToolsHaveModes(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.handleDescribeCapabilities(req, json.RawMessage(`{}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	var data map[string]any
	json.Unmarshal([]byte(text[idx:]), &data)
	tools := data["tools"].(map[string]any)

	// observe tool should have modes (what enum)
	observeTool := tools["observe"].(map[string]any)
	modes, ok := observeTool["modes"].([]any)
	if !ok || len(modes) == 0 {
		t.Error("observe tool should have non-empty modes list")
	}

	// Check that modes include known values
	modeSet := make(map[string]bool)
	for _, m := range modes {
		modeSet[m.(string)] = true
	}
	if !modeSet["errors"] {
		t.Error("observe modes should include 'errors'")
	}
	if !modeSet["logs"] {
		t.Error("observe modes should include 'logs'")
	}
}

func TestDescribeCapabilities_ConfigureIncludesModeParameterDetails(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.handleDescribeCapabilities(req, json.RawMessage(`{}`))

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("no JSON in response")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse capabilities JSON: %v", err)
	}

	tools := data["tools"].(map[string]any)
	configureTool := tools["configure"].(map[string]any)

	modeParamsRaw, ok := configureTool["mode_params"]
	if !ok {
		t.Fatal("configure capabilities should include mode_params")
	}
	modeParams := modeParamsRaw.(map[string]any)
	storeMode := modeParams["store"].(map[string]any)

	params := storeMode["params"].(map[string]any)
	namespaceMeta := params["namespace"].(map[string]any)
	if namespaceMeta["type"] != "string" {
		t.Fatalf("store.namespace type = %v, want string", namespaceMeta["type"])
	}

	storeActionMeta := params["store_action"].(map[string]any)
	if storeActionMeta["default"] != "list" {
		t.Fatalf("store.store_action default = %v, want list", storeActionMeta["default"])
	}
}
