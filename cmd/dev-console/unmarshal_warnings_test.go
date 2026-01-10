// unmarshal_warnings_test.go — Tests for unknown parameter warnings (Issue 3),
// configure enum/dispatch consistency (Issue 2), and observe description size (Issue 1).
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// Issue 1: Observe description size limit
// ============================================

func TestObserveDescriptionUnder800Chars(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result MCPToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	for _, tool := range result.Tools {
		if tool.Name == "observe" {
			charCount := len(tool.Description)
			if charCount > 800 {
				t.Errorf("observe description is %d characters, should be under 800. Description:\n%s", charCount, tool.Description)
			}
			// Must still mention key modes
			for _, mode := range []string{"errors", "logs", "network_bodies", "page"} {
				if !strings.Contains(tool.Description, mode) {
					t.Errorf("observe description should mention mode %q", mode)
				}
			}
			// Must NOT contain anti-pattern guidance
			antiPatterns := []string{"DON'T", "WRONG:", "CORRECT:", "ANTI-PATTERN", "anti-pattern"}
			for _, ap := range antiPatterns {
				if strings.Contains(tool.Description, ap) {
					t.Errorf("observe description should not contain verbose anti-pattern text %q", ap)
				}
			}
			return
		}
	}
	t.Fatal("observe tool not found in tools list")
}

// ============================================
// Issue 2: Configure enum matches dispatch switch
// ============================================

func TestConfigureEnumMatchesDispatchSwitch(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result MCPToolsListResult
	json.Unmarshal(resp.Result, &result)

	for _, tool := range result.Tools {
		if tool.Name != "configure" {
			continue
		}

		enumValues := extractEnumValues(t, tool, "action")

		// These are all valid actions based on the dispatch switch in toolConfigure.
		// If any are missing from the enum, the LLM can't discover them.
		expectedInEnum := []string{
			"store", "load", "noise_rule", "dismiss", "clear",
			"capture", "record_event", "query_dom", "diff_sessions",
			"validate_api", "audit_log", "health", "streaming",
		}

		enumSet := make(map[string]bool)
		for _, v := range enumValues {
			enumSet[v] = true
		}

		for _, expected := range expectedInEnum {
			if !enumSet[expected] {
				t.Errorf("configure action enum missing dispatch case %q", expected)
			}
		}

		return
	}
	t.Fatal("configure tool not found in tools list")
}

func TestConfigureErrorHintListsAllActions(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Call configure without an action to trigger the error hint
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{}}`),
	})

	text := extractMCPText(t, resp)

	expectedActions := []string{
		"store", "load", "noise_rule", "dismiss", "clear",
		"capture", "record_event", "query_dom", "diff_sessions",
		"validate_api", "audit_log", "health", "streaming",
	}

	for _, action := range expectedActions {
		if !strings.Contains(text, action) {
			t.Errorf("configure error hint missing action %q in text: %s", action, text)
		}
	}
}

// ============================================
// Issue 3: unmarshalWithWarnings helper
// ============================================

func TestUnmarshalWithWarnings_NoUnknownFields(t *testing.T) {
	t.Parallel()
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := json.RawMessage(`{"name":"test","value":42}`)
	var s TestStruct
	warnings, err := unmarshalWithWarnings(data, &s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
	if s.Name != "test" || s.Value != 42 {
		t.Errorf("unexpected values: name=%q value=%d", s.Name, s.Value)
	}
}

func TestUnmarshalWithWarnings_UnknownField(t *testing.T) {
	t.Parallel()
	type TestStruct struct {
		Name string `json:"name"`
	}

	data := json.RawMessage(`{"name":"test","naem":"typo","extra":true}`)
	var s TestStruct
	warnings, err := unmarshalWithWarnings(data, &s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}

	// Check that unknown field names appear in warnings
	combined := strings.Join(warnings, " ")
	if !strings.Contains(combined, "naem") {
		t.Errorf("expected warning about 'naem', got: %v", warnings)
	}
	if !strings.Contains(combined, "extra") {
		t.Errorf("expected warning about 'extra', got: %v", warnings)
	}
}

func TestUnmarshalWithWarnings_InvalidJSON(t *testing.T) {
	t.Parallel()
	type TestStruct struct {
		Name string `json:"name"`
	}

	data := json.RawMessage(`{invalid`)
	var s TestStruct
	_, err := unmarshalWithWarnings(data, &s)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUnmarshalWithWarnings_EmptyInput(t *testing.T) {
	t.Parallel()
	type TestStruct struct {
		Name string `json:"name"`
	}

	data := json.RawMessage(`{}`)
	var s TestStruct
	warnings, err := unmarshalWithWarnings(data, &s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestUnmarshalWithWarnings_OmitemptyField(t *testing.T) {
	t.Parallel()
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value,omitempty"`
	}

	data := json.RawMessage(`{"name":"test","value":1}`)
	var s TestStruct
	warnings, err := unmarshalWithWarnings(data, &s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestUnmarshalWithWarnings_NestedStruct(t *testing.T) {
	t.Parallel()
	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		Name  string `json:"name"`
		Inner Inner  `json:"inner"`
	}

	// "typo_field" at the top level should be flagged
	data := json.RawMessage(`{"name":"test","inner":{"x":1},"typo_field":true}`)
	var s Outer
	warnings, err := unmarshalWithWarnings(data, &s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "typo_field") {
		t.Errorf("expected warning about 'typo_field', got: %s", warnings[0])
	}
}

func TestGetJSONFieldNames(t *testing.T) {
	t.Parallel()
	type TestStruct struct {
		Name     string `json:"name"`
		Value    int    `json:"value,omitempty"`
		NoTag    string
		Ignored  string `json:"-"`
		RawField string `json:"raw_field"`
	}

	known := getJSONFieldNames(&TestStruct{})

	if !known["name"] {
		t.Error("expected 'name' to be known")
	}
	if !known["value"] {
		t.Error("expected 'value' to be known")
	}
	if !known["raw_field"] {
		t.Error("expected 'raw_field' to be known")
	}
	// Fields without json tag use field name
	if !known["NoTag"] {
		t.Error("expected 'NoTag' to be known")
	}
	// Ignored fields should not be known
	if known["-"] {
		t.Error("field with json:\"-\" should not be known")
	}
}

// ============================================
// Issue 3: Integration test — misspelled param produces warning in tool response
// ============================================

func TestObserveWithMisspelledParamProducesWarning(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add some errors so the observe call has data to return
	server.addEntries([]LogEntry{
		{"level": "error", "message": "Test error", "source": "test.js", "ts": "00:00:00"},
	})

	// Call observe with a misspelled parameter "limt" instead of "limit"
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors","limt":5}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// The response should contain a warning about the unknown parameter "limt"
	found := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "limt") && strings.Contains(block.Text, "unknown") {
			found = true
			break
		}
	}
	if !found {
		texts := make([]string, len(result.Content))
		for i, b := range result.Content {
			texts[i] = b.Text
		}
		t.Errorf("Expected warning about unknown parameter 'limt', got content blocks: %v", texts)
	}
}

func TestConfigureWithMisspelledParamProducesWarning(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Call configure with a misspelled parameter "acton" instead of "action"
	// But we need a valid action, so pass both "action" (valid) and "acton" (typo)
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"health","acton":"health"}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	found := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "acton") && strings.Contains(block.Text, "unknown") {
			found = true
			break
		}
	}
	if !found {
		texts := make([]string, len(result.Content))
		for i, b := range result.Content {
			texts[i] = b.Text
		}
		t.Errorf("Expected warning about unknown parameter 'acton', got content blocks: %v", texts)
	}
}
