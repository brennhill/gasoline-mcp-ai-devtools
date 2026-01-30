// api_schema_test.go — W2 Response Schema Documentation Tests + W4 Parameter Naming Tests.
// Verifies that tool descriptions include response format documentation,
// that every mode/action/format enum value is documented, and that
// documented column names match the actual handler output.
// W4: Verifies canonical parameter names and rejects old parameter names.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ============================================
// Existence Tests: descriptions contain response docs
// ============================================

// TestToolDescriptions_ContainResponseDocs verifies that each of the 4 tool
// descriptions contains response format documentation.
func TestToolDescriptions_ContainResponseDocs(t *testing.T) {
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

	checks := map[string]string{
		"observe":   "Mode responses",
		"generate":  "Format responses",
		"configure": "Returns",
		"interact":  "Action responses",
	}

	for _, tool := range result.Tools {
		expected, ok := checks[tool.Name]
		if !ok {
			continue
		}
		if !strings.Contains(tool.Description, expected) {
			t.Errorf("Tool %s: description missing response docs (expected substring %q)", tool.Name, expected)
		}
	}
}

// ============================================
// Completeness Tests: every enum value documented
// ============================================

// TestToolDescriptions_AllModesDocumented parses the 'what' enum from the
// observe tool's InputSchema, then verifies every enum value appears in the
// observe description text.
func TestToolDescriptions_AllModesDocumented(t *testing.T) {
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
		if tool.Name != "observe" {
			continue
		}
		enumValues := extractEnumValues(t, tool, "what")
		for _, val := range enumValues {
			if !strings.Contains(tool.Description, val) {
				t.Errorf("observe description missing documentation for mode %q", val)
			}
		}
	}
}

// TestToolDescriptions_AllActionsDocumented parses the 'action' enum from
// the configure tool's InputSchema, then verifies every enum value appears
// in the configure description text.
func TestToolDescriptions_AllActionsDocumented(t *testing.T) {
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
		for _, val := range enumValues {
			if !strings.Contains(tool.Description, val) {
				t.Errorf("configure description missing documentation for action %q", val)
			}
		}
	}
}

// TestToolDescriptions_AllInteractActionsDocumented parses the 'action' enum
// from the interact tool's InputSchema, then verifies every enum value appears
// in the interact description text.
func TestToolDescriptions_AllInteractActionsDocumented(t *testing.T) {
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
		if tool.Name != "interact" {
			continue
		}
		enumValues := extractEnumValues(t, tool, "action")
		for _, val := range enumValues {
			if !strings.Contains(tool.Description, val) {
				t.Errorf("interact description missing documentation for action %q", val)
			}
		}
	}
}

// TestToolDescriptions_AllFormatsDocumented parses the 'format' enum from
// the generate tool's InputSchema, then verifies every enum value appears
// in the generate description text.
func TestToolDescriptions_AllFormatsDocumented(t *testing.T) {
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
		if tool.Name != "generate" {
			continue
		}
		enumValues := extractEnumValues(t, tool, "format")
		for _, val := range enumValues {
			if !strings.Contains(tool.Description, val) {
				t.Errorf("generate description missing documentation for format %q", val)
			}
		}
	}
}

// ============================================
// Accuracy Tests: columns/fields match docs
// ============================================

// TestObserveErrors_ColumnsMatchDocs calls observe with what:"errors" and
// verifies the markdown table headers match what the response docs declare.
func TestObserveErrors_ColumnsMatchDocs(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "Test error", "source": "test.js", "ts": "00:00:00"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	text := extractMCPText(t, resp)
	headers := extractMarkdownTableHeaders(t, text)
	expected := []string{"Level", "Message", "Source", "Time", "Tab"}
	assertHeaders(t, "observe errors", headers, expected)
}

// TestObserveLogs_ColumnsMatchDocs calls observe with what:"logs" and
// verifies the markdown table headers match the documented columns.
func TestObserveLogs_ColumnsMatchDocs(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "info", "message": "Test log", "source": "app.js", "ts": "00:00:00"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	text := extractMCPText(t, resp)
	// Response is now JSON format instead of markdown table
	expectedFields := []string{`"level"`, `"message"`, `"source"`, `"timestamp"`, `"sequence"`}
	for _, field := range expectedFields {
		if !strings.Contains(text, field) {
			t.Errorf("Expected JSON field %s in logs response, got: %s", field, text)
		}
	}
}

// TestObserveNetwork_ColumnsMatchDocs calls observe with what:"network_bodies" and
// verifies the JSON structure matches the documented schema.
func TestObserveNetwork_ColumnsMatchDocs(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/test", Method: "GET", Status: 200, Duration: 50},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network_bodies"}}`),
	})

	text := extractMCPText(t, resp)

	// Parse JSON response (skip summary line if present)
	lines := strings.Split(text, "\n")
	jsonText := text
	if len(lines) > 1 && !strings.HasPrefix(text, "{") {
		jsonText = strings.Join(lines[1:], "\n")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Expected JSON response, got: %s", text)
	}

	// Verify expected fields exist
	pairs, ok := data["network_request_response_pairs"].([]interface{})
	if !ok || len(pairs) != 1 {
		t.Fatal("Expected networkRequestResponsePairs array with 1 entry")
	}

	pair := pairs[0].(map[string]interface{})
	expectedFields := []string{"url", "method", "status", "duration_ms"}
	for _, field := range expectedFields {
		if _, ok := pair[field]; !ok {
			t.Errorf("Expected field '%s' in pair object", field)
		}
	}

	// Verify metadata fields
	if _, ok := data["max_request_body_bytes"]; !ok {
		t.Error("Expected maxRequestBodyBytes metadata field")
	}
	if _, ok := data["max_response_body_bytes"]; !ok {
		t.Error("Expected maxResponseBodyBytes metadata field")
	}
}

// TestObserveActions_ColumnsMatchDocs calls observe with what:"actions" and
// verifies the markdown table headers match the documented columns.
func TestObserveActions_ColumnsMatchDocs(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", URL: "http://localhost:3000/form", Selectors: map[string]interface{}{"css": "#btn"}},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"actions"}}`),
	})

	text := extractMCPText(t, resp)
	// Response is now JSON format instead of markdown table
	expectedFields := []string{`"type"`, `"url"`, `"selectors"`, `"timestamp"`, `"sequence"`}
	for _, field := range expectedFields {
		if !strings.Contains(text, field) {
			t.Errorf("Expected JSON field %s in actions response, got: %s", field, text)
		}
	}
}

// TestObserveWSEvents_ColumnsMatchDocs calls observe with what:"websocket_events"
// and verifies the markdown table headers match the documented columns.
func TestObserveWSEvents_ColumnsMatchDocs(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-1", Event: "message", Data: `{"type":"ping"}`, URL: "wss://echo.example.com", Direction: "incoming"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_events"}}`),
	})

	text := extractMCPText(t, resp)
	headers := extractMarkdownTableHeaders(t, text)
	expected := []string{"ID", "Event", "URL", "Direction", "Size", "Time", "Tab"}
	assertHeaders(t, "observe websocket_events", headers, expected)
}

// TestObserveHealth_FieldsMatchDocs calls configure with action:"health" and
// verifies the top-level JSON keys match the documented structure.
func TestObserveHealth_FieldsMatchDocs(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"health"}}`),
	})

	text := extractMCPText(t, resp)

	// The response is "Server health\n{...JSON...}"
	// Split on first newline to get the JSON part
	parts := strings.SplitN(text, "\n", 2)
	if len(parts) < 2 {
		t.Fatalf("Expected summary + JSON, got: %s", text)
	}

	var healthData map[string]interface{}
	if err := json.Unmarshal([]byte(parts[1]), &healthData); err != nil {
		t.Fatalf("Failed to parse health JSON: %v", err)
	}

	expectedKeys := []string{"server", "memory", "buffers", "rate_limiting", "audit", "pilot"}
	for _, key := range expectedKeys {
		if _, ok := healthData[key]; !ok {
			t.Errorf("Health response missing documented key %q", key)
		}
	}
}

// ============================================
// Test Helpers
// ============================================

// extractEnumValues extracts string enum values from a tool's InputSchema
// for the given property name.
func extractEnumValues(t *testing.T, tool MCPTool, propName string) []string {
	t.Helper()

	props, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("Tool %s: missing properties in inputSchema", tool.Name)
	}

	prop, ok := props[propName].(map[string]interface{})
	if !ok {
		t.Fatalf("Tool %s: missing property %q", tool.Name, propName)
	}

	enumRaw, ok := prop["enum"]
	if !ok {
		t.Fatalf("Tool %s: property %q missing enum", tool.Name, propName)
	}

	enumJSON, _ := json.Marshal(enumRaw)
	var values []string
	json.Unmarshal(enumJSON, &values)
	return values
}

// extractMarkdownTableHeaders parses the first markdown table header row
// from an MCP text response. The response format is:
// "summary\n\n| Col1 | Col2 | Col3 |\n| --- | --- | --- |\n..."
func extractMarkdownTableHeaders(t *testing.T, text string) []string {
	t.Helper()

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			// Parse header columns: "| Col1 | Col2 | Col3 |"
			parts := strings.Split(line, "|")
			var headers []string
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					headers = append(headers, trimmed)
				}
			}
			return headers
		}
	}
	t.Fatalf("No markdown table found in response:\n%s", text)
	return nil
}

// assertHeaders compares extracted headers against expected headers.
func assertHeaders(t *testing.T, context string, got, expected []string) {
	t.Helper()

	if len(got) != len(expected) {
		t.Errorf("%s: expected %d columns %v, got %d columns %v", context, len(expected), expected, len(got), got)
		return
	}

	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("%s: column %d expected %q, got %q", context, i, expected[i], got[i])
		}
	}
}

// ============================================
// W4: Parameter Naming Cleanup Tests
// ============================================

// oldParamNames lists the deprecated parameter names that must not appear
// in any JSON struct tag or schema property key.
var oldParamNames = []string{
	"url_filter", "url_pattern",
	"last_n_actions", "last_n_entries",
	"log_level", "min_level",
	"show_source", "with_source",
}

// TestNoOldParamNames_InStructTags scans all .go source files in
// cmd/dev-console/ for JSON struct tags containing old parameter names.
// Verifies zero matches — proving old names are fully removed from code.
func TestNoOldParamNames_InStructTags(t *testing.T) {
	t.Parallel()
	// Build regex to match old names inside JSON struct tags
	pattern := regexp.MustCompile(`json:"(url_filter|url_pattern|last_n_actions|last_n_entries|log_level|min_level|show_source|with_source)`)

	dir := filepath.Join(".")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Skip test files — they may reference old names in comments/strings
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", entry.Name(), err)
		}

		matches := pattern.FindAllString(string(data), -1)
		for _, match := range matches {
			t.Errorf("File %s contains old param name in struct tag: %s", entry.Name(), match)
		}
	}
}

// TestNoOldParamNames_InSchemaProperties calls toolsList() and walks all
// tool InputSchema properties maps, collecting every property key. Verifies
// none match old deprecated parameter names.
func TestNoOldParamNames_InSchemaProperties(t *testing.T) {
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

	oldNames := map[string]bool{}
	for _, name := range oldParamNames {
		oldNames[name] = true
	}

	for _, tool := range result.Tools {
		props, ok := tool.InputSchema["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		for key := range props {
			if oldNames[key] {
				t.Errorf("Tool %s: schema property %q uses deprecated name", tool.Name, key)
			}
		}
	}
}

// --- Positive tests: new canonical names work ---

// TestObserveNetwork_URLParam verifies that observe({what:"network_bodies", url:"api"})
// correctly filters network entries by the canonical "url" parameter.
func TestObserveNetwork_URLParam(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, Duration: 50},
		{URL: "https://cdn.example.com/style.css", Method: "GET", Status: 200, Duration: 10},
		{URL: "https://api.example.com/orders", Method: "POST", Status: 201, Duration: 80},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network_bodies","url":"api"}}`),
	})

	text := extractMCPText(t, resp)

	// Should contain the two API entries but not the CDN entry
	if !strings.Contains(text, "api.example.com/users") {
		t.Error("Expected api.example.com/users in filtered response")
	}
	if !strings.Contains(text, "api.example.com/orders") {
		t.Error("Expected api.example.com/orders in filtered response")
	}
	if strings.Contains(text, "cdn.example.com") {
		t.Error("cdn.example.com should be filtered out by url=\"api\"")
	}
}

// TestGenerateTest_LastNParam verifies that generate({format:"test", last_n:2})
// limits output to the last 2 actions using the canonical "last_n" parameter.
func TestGenerateTest_LastNParam(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add 5 actions
	for i := 0; i < 5; i++ {
		capture.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		})
	}

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","last_n":2}}`),
	})

	text := extractMCPText(t, resp)
	clickCount := strings.Count(text, ".click()")
	if clickCount != 2 {
		t.Errorf("Expected 2 click actions with last_n=2, got %d", clickCount)
	}
}

// TestObserveTimeline_LastNParam verifies that observe({what:"timeline",
// last_n:3}) applies the canonical "last_n" parameter for timeline filtering.
func TestObserveTimeline_LastNParam(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	for i := 0; i < 10; i++ {
		capture.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000"},
		})
	}

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"timeline","last_n":3}}`),
	})

	text := extractMCPText(t, resp)

	var timelineResp SessionTimelineResponse
	if err := json.Unmarshal([]byte(text), &timelineResp); err != nil {
		t.Fatalf("Failed to parse timeline response: %v", err)
	}

	if timelineResp.Summary.Actions != 3 {
		t.Errorf("Expected 3 actions with last_n=3, got %d", timelineResp.Summary.Actions)
	}
}

// --- Negative tests: old names are silently ignored ---

// TestObserveNetwork_OldURLFilterRejected verifies that passing the old
// "url_filter" parameter has no filtering effect — all entries are returned.
func TestObserveNetwork_OldURLFilterRejected(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, Duration: 50},
		{URL: "https://cdn.example.com/style.css", Method: "GET", Status: 200, Duration: 10},
		{URL: "https://api.example.com/orders", Method: "POST", Status: 201, Duration: 80},
	})

	// Use the OLD parameter name "url_filter" — should be ignored
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network_bodies","url_filter":"api"}}`),
	})

	text := extractMCPText(t, resp)

	// ALL 3 entries should be returned since url_filter is not recognized
	if !strings.Contains(text, "3 network request") {
		t.Errorf("Expected all 3 network entries (old param ignored), got: %s", text)
	}
}
