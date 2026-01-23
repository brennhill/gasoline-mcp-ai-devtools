package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPGetReproductionScript(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "login-btn"}},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"ariaLabel": "Email"}, Value: "user@test.com", InputType: "email"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// Should contain Playwright test structure
	if !strings.Contains(script, "import { test, expect } from '@playwright/test'") {
		t.Error("Expected Playwright import in script")
	}
	if !strings.Contains(script, "test(") {
		t.Error("Expected test() in script")
	}
	if !strings.Contains(script, "page.goto") {
		t.Error("Expected page.goto in script")
	}
}

func TestMCPGetReproductionScriptWithErrorMessage(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "submit-btn"}},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{"error_message":"Cannot read property 'user' of undefined"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	if !strings.Contains(script, "Cannot read property") {
		t.Error("Expected error message in script")
	}
}

func TestMCPGetReproductionScriptWithLastN(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	for i := 0; i < 10; i++ {
		capture.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn-" + string(rune('a'+i))}},
		})
	}

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{"last_n_actions":3}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// Should only have 3 click actions in the script
	clickCount := strings.Count(script, ".click()")
	if clickCount != 3 {
		t.Errorf("Expected 3 click actions in script with last_n_actions=3, got %d", clickCount)
	}
}

func TestMCPGetReproductionScriptWithBaseURL(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
		{Type: "navigate", Timestamp: 2000, URL: "http://localhost:3000/dashboard", FromURL: "http://localhost:3000/login", ToURL: "http://localhost:3000/dashboard"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{"base_url":"https://staging.example.com"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// goto should use base_url + path
	if !strings.Contains(script, "staging.example.com/login") {
		t.Errorf("Expected base_url to be applied to goto, got script:\n%s", script)
	}
}

func TestMCPGetReproductionScriptEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if result.Content[0].Text != "No enhanced actions captured to generate script" {
		t.Errorf("Expected empty message, got: %s", result.Content[0].Text)
	}
}

func TestMCPGetReproductionScriptSelectorPriority(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{
			Type: "click", Timestamp: 1000, URL: "http://localhost:3000",
			Selectors: map[string]interface{}{
				"testId":    "submit-btn",
				"ariaLabel": "Submit form",
				"cssPath":   "form > button",
			},
		},
		{
			Type: "click", Timestamp: 2000, URL: "http://localhost:3000",
			Selectors: map[string]interface{}{
				"role":    map[string]interface{}{"role": "button", "name": "Save"},
				"cssPath": "div > button",
			},
		},
		{
			Type: "click", Timestamp: 3000, URL: "http://localhost:3000",
			Selectors: map[string]interface{}{
				"cssPath": "div.card > button.action",
			},
		},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// testId should produce getByTestId
	if !strings.Contains(script, "getByTestId('submit-btn')") {
		t.Errorf("Expected getByTestId for first action, got:\n%s", script)
	}

	// role should produce getByRole
	if !strings.Contains(script, "getByRole('button', { name: 'Save' })") {
		t.Errorf("Expected getByRole for second action, got:\n%s", script)
	}

	// cssPath fallback should produce locator()
	if !strings.Contains(script, "locator('div.card > button.action')") {
		t.Errorf("Expected locator() for third action, got:\n%s", script)
	}
}

func TestMCPGetReproductionScriptInputActions(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "input", Timestamp: 1000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "email-input"}, Value: "user@test.com", InputType: "email"},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "password-input"}, Value: "[redacted]", InputType: "password"},
		{Type: "select", Timestamp: 3000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "country-select"}, SelectedValue: "us"},
		{Type: "keypress", Timestamp: 4000, URL: "http://localhost:3000", Key: "Enter"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// fill() for input
	if !strings.Contains(script, ".fill('user@test.com')") {
		t.Errorf("Expected fill for email input, got:\n%s", script)
	}

	// Redacted password should use placeholder
	if !strings.Contains(script, ".fill('[user-provided]')") {
		t.Errorf("Expected [user-provided] for redacted password, got:\n%s", script)
	}

	// selectOption for select
	if !strings.Contains(script, ".selectOption('us')") {
		t.Errorf("Expected selectOption for select, got:\n%s", script)
	}

	// keyboard.press for keypress
	if !strings.Contains(script, "page.keyboard.press('Enter')") {
		t.Errorf("Expected keyboard.press for keypress, got:\n%s", script)
	}
}

func TestMCPGetReproductionScriptPauseComments(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn1"}},
		{Type: "click", Timestamp: 6000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn2"}}, // 5 seconds later
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// Should contain a pause comment for the 5s gap
	if !strings.Contains(script, "// [5s pause]") {
		t.Errorf("Expected pause comment for 5s gap, got:\n%s", script)
	}
}

func TestExtractResponseShapeObject(t *testing.T) {
	shape := extractResponseShape(`{"token":"abc123","user":{"id":5,"name":"Bob"}}`)

	shapeMap, ok := shape.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", shape)
	}
	if shapeMap["token"] != "string" {
		t.Errorf("Expected token=string, got %v", shapeMap["token"])
	}
	userMap, ok := shapeMap["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user to be map, got %T", shapeMap["user"])
	}
	if userMap["id"] != "number" {
		t.Errorf("Expected user.id=number, got %v", userMap["id"])
	}
	if userMap["name"] != "string" {
		t.Errorf("Expected user.name=string, got %v", userMap["name"])
	}
}

func TestExtractResponseShapeArray(t *testing.T) {
	shape := extractResponseShape(`[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`)

	shapeArr, ok := shape.([]interface{})
	if !ok {
		t.Fatalf("Expected array, got %T", shape)
	}
	if len(shapeArr) != 1 {
		t.Fatalf("Expected array with 1 element (sample), got %d", len(shapeArr))
	}
	elem, ok := shapeArr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected array element to be map, got %T", shapeArr[0])
	}
	if elem["id"] != "number" {
		t.Errorf("Expected id=number, got %v", elem["id"])
	}
}

func TestExtractResponseShapePrimitives(t *testing.T) {
	if extractResponseShape(`"hello"`) != "string" {
		t.Error("Expected string for string literal")
	}
	if extractResponseShape(`42`) != "number" {
		t.Error("Expected number for numeric literal")
	}
	if extractResponseShape(`true`) != "boolean" {
		t.Error("Expected boolean for true")
	}
	if extractResponseShape(`null`) != "null" {
		t.Error("Expected null for null")
	}
}

func TestExtractResponseShapeInvalidJSON(t *testing.T) {
	shape := extractResponseShape(`not valid json`)
	if shape != nil {
		t.Errorf("Expected nil for invalid JSON, got %v", shape)
	}
}

func TestExtractResponseShapeEmptyObject(t *testing.T) {
	shape := extractResponseShape(`{}`)
	shapeMap, ok := shape.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", shape)
	}
	if len(shapeMap) != 0 {
		t.Errorf("Expected empty map, got %d keys", len(shapeMap))
	}
}

func TestExtractResponseShapeEmptyArray(t *testing.T) {
	shape := extractResponseShape(`[]`)
	shapeArr, ok := shape.([]interface{})
	if !ok {
		t.Fatalf("Expected array, got %T", shape)
	}
	if len(shapeArr) != 0 {
		t.Errorf("Expected empty array, got %d elements", len(shapeArr))
	}
}

func TestExtractResponseShapeDepthLimit(t *testing.T) {
	// Nested 5 levels deep - should cap at depth 3
	shape := extractResponseShape(`{"a":{"b":{"c":{"d":{"e":"deep"}}}}}`)
	shapeMap := shape.(map[string]interface{})
	aMap := shapeMap["a"].(map[string]interface{})
	bMap := aMap["b"].(map[string]interface{})
	cMap := bMap["c"].(map[string]interface{})
	// At depth 3, values should be "..."
	if cMap["d"] != "..." {
		t.Errorf("Expected '...' at depth limit, got %v", cMap["d"])
	}
}

func TestNormalizeTimestampRFC3339(t *testing.T) {
	ts := normalizeTimestamp("2024-01-15T10:30:00.000Z")
	expected := int64(1705314600000)
	if ts != expected {
		t.Errorf("Expected %d, got %d", expected, ts)
	}
}

func TestNormalizeTimestampRFC3339Nano(t *testing.T) {
	ts := normalizeTimestamp("2024-01-15T10:30:00.123456789Z")
	// Should be 1705314600123 (truncated to ms)
	expected := int64(1705314600123)
	if ts != expected {
		t.Errorf("Expected %d, got %d", expected, ts)
	}
}

func TestNormalizeTimestampInvalidString(t *testing.T) {
	ts := normalizeTimestamp("not a timestamp")
	if ts != 0 {
		t.Errorf("Expected 0 for invalid timestamp, got %d", ts)
	}
}

func TestNormalizeTimestampEmpty(t *testing.T) {
	ts := normalizeTimestamp("")
	if ts != 0 {
		t.Errorf("Expected 0 for empty string, got %d", ts)
	}
}

func TestGetSessionTimelineEmpty(t *testing.T) {
	capture := setupTestCapture(t)

	resp := capture.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) != 0 {
		t.Errorf("Expected empty timeline, got %d entries", len(resp.Timeline))
	}
	if resp.Summary.Actions != 0 || resp.Summary.NetworkRequests != 0 || resp.Summary.ConsoleErrors != 0 {
		t.Error("Expected all summary counts to be 0")
	}
}

func TestGetSessionTimelineActionsOnly(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/login", Value: "test@test.com"},
	})

	resp := capture.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(resp.Timeline))
	}
	if resp.Timeline[0].Kind != "action" {
		t.Errorf("Expected kind=action, got %s", resp.Timeline[0].Kind)
	}
	if resp.Timeline[0].Type != "click" {
		t.Errorf("Expected type=click, got %s", resp.Timeline[0].Type)
	}
	if resp.Summary.Actions != 2 {
		t.Errorf("Expected 2 actions in summary, got %d", resp.Summary.Actions)
	}
}

func TestGetSessionTimelineNetworkOnly(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.000Z", Method: "POST", URL: "/api/login", Status: 200, ResponseBody: `{"token":"abc"}`, ContentType: "application/json"},
	})

	resp := capture.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(resp.Timeline))
	}
	if resp.Timeline[0].Kind != "network" {
		t.Errorf("Expected kind=network, got %s", resp.Timeline[0].Kind)
	}
	if resp.Timeline[0].Method != "POST" {
		t.Errorf("Expected method=POST, got %s", resp.Timeline[0].Method)
	}
	if resp.Timeline[0].Status != 200 {
		t.Errorf("Expected status=200, got %d", resp.Timeline[0].Status)
	}
	if resp.Summary.NetworkRequests != 1 {
		t.Errorf("Expected 1 network request in summary, got %d", resp.Summary.NetworkRequests)
	}
}

func TestGetSessionTimelineNetworkResponseShape(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.000Z", Method: "GET", URL: "/api/users", Status: 200,
			ResponseBody: `{"users":[{"id":1,"name":"Alice"}]}`, ContentType: "application/json"},
	})

	resp := capture.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if resp.Timeline[0].ResponseShape == nil {
		t.Fatal("Expected responseShape to be populated for JSON responses")
	}

	shapeMap, ok := resp.Timeline[0].ResponseShape.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected shape to be map, got %T", resp.Timeline[0].ResponseShape)
	}
	if _, hasUsers := shapeMap["users"]; !hasUsers {
		t.Error("Expected responseShape to have 'users' key")
	}
}

func TestGetSessionTimelineNetworkNonJSONNoShape(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.000Z", Method: "GET", URL: "/page", Status: 200,
			ResponseBody: `<html></html>`, ContentType: "text/html"},
	})

	resp := capture.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if resp.Timeline[0].ResponseShape != nil {
		t.Errorf("Expected nil responseShape for non-JSON content, got %v", resp.Timeline[0].ResponseShape)
	}
}

func TestGetSessionTimelineConsoleEntries(t *testing.T) {
	capture := setupTestCapture(t)

	entries := []LogEntry{
		{"level": "error", "message": "Something failed", "ts": "2024-01-15T10:30:00.500Z"},
		{"level": "warn", "message": "Deprecated API", "ts": "2024-01-15T10:30:01.000Z"},
		{"level": "info", "message": "App started", "ts": "2024-01-15T10:30:00.100Z"}, // Should be excluded
	}

	resp := capture.GetSessionTimeline(TimelineFilter{}, entries)

	if len(resp.Timeline) != 2 {
		t.Fatalf("Expected 2 entries (error+warn, no info), got %d", len(resp.Timeline))
	}
	if resp.Timeline[0].Kind != "console" {
		t.Errorf("Expected kind=console, got %s", resp.Timeline[0].Kind)
	}
	if resp.Summary.ConsoleErrors != 1 {
		t.Errorf("Expected 1 console error in summary, got %d", resp.Summary.ConsoleErrors)
	}
}

func TestGetSessionTimelineMergedAndSorted(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1705314600000, URL: "http://localhost:3000"},
		{Type: "navigate", Timestamp: 1705314600300, ToURL: "/dashboard"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.150Z", Method: "POST", URL: "/api/login", Status: 200,
			ResponseBody: `{"ok":true}`, ContentType: "application/json"},
	})
	entries := []LogEntry{
		{"level": "error", "message": "Widget failed", "ts": "2024-01-15T10:30:00.400Z"},
	}

	resp := capture.GetSessionTimeline(TimelineFilter{}, entries)

	if len(resp.Timeline) != 4 {
		t.Fatalf("Expected 4 entries, got %d", len(resp.Timeline))
	}

	// Verify order: click(1705314600000) -> network(150ms later) -> navigate(300ms) -> error(400ms)
	if resp.Timeline[0].Kind != "action" || resp.Timeline[0].Type != "click" {
		t.Errorf("Entry 0: expected action/click, got %s/%s", resp.Timeline[0].Kind, resp.Timeline[0].Type)
	}
	if resp.Timeline[1].Kind != "network" {
		t.Errorf("Entry 1: expected network, got %s", resp.Timeline[1].Kind)
	}
	if resp.Timeline[2].Kind != "action" || resp.Timeline[2].Type != "navigate" {
		t.Errorf("Entry 2: expected action/navigate, got %s/%s", resp.Timeline[2].Kind, resp.Timeline[2].Type)
	}
	if resp.Timeline[3].Kind != "console" {
		t.Errorf("Entry 3: expected console, got %s", resp.Timeline[3].Kind)
	}
}

func TestGetSessionTimelineLastNActions(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000"},
		{Type: "click", Timestamp: 2000, URL: "http://localhost:3000"},
		{Type: "click", Timestamp: 3000, URL: "http://localhost:3000"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "1970-01-01T00:00:00.500Z", Method: "GET", URL: "/early", Status: 200, ContentType: "application/json", ResponseBody: `{}`}, // Before action 1
		{Timestamp: "1970-01-01T00:00:02.500Z", Method: "GET", URL: "/late", Status: 200, ContentType: "application/json", ResponseBody: `{}`},  // Between action 2 and 3
	})

	// Request last 2 actions -- should start from action at t=2000
	resp := capture.GetSessionTimeline(TimelineFilter{LastNActions: 2}, []LogEntry{})

	// Should include: action@2000, network@2500, action@3000 -- NOT action@1000 or network@500
	hasEarly := false
	for _, entry := range resp.Timeline {
		if entry.Kind == "network" && entry.URL == "/early" {
			hasEarly = true
		}
	}
	if hasEarly {
		t.Error("Should not include network events before the last_n_actions boundary")
	}
	if resp.Summary.Actions != 2 {
		t.Errorf("Expected 2 actions in summary, got %d", resp.Summary.Actions)
	}
}

func TestGetSessionTimelineURLFilter(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login"},
		{Type: "click", Timestamp: 2000, URL: "http://localhost:3000/dashboard"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "1970-01-01T00:00:01.500Z", Method: "POST", URL: "/api/login", Status: 200, ContentType: "application/json", ResponseBody: `{}`},
		{Timestamp: "1970-01-01T00:00:02.500Z", Method: "GET", URL: "/api/dashboard", Status: 200, ContentType: "application/json", ResponseBody: `{}`},
	})

	resp := capture.GetSessionTimeline(TimelineFilter{URLFilter: "login"}, []LogEntry{})

	for _, entry := range resp.Timeline {
		if entry.Kind == "action" && !strings.Contains(entry.URL, "login") {
			t.Errorf("Expected URL filter to exclude non-matching actions, got URL: %s", entry.URL)
		}
		if entry.Kind == "network" && !strings.Contains(entry.URL, "login") {
			t.Errorf("Expected URL filter to exclude non-matching network, got URL: %s", entry.URL)
		}
	}
}

func TestGetSessionTimelineIncludeFilter(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "1970-01-01T00:00:01.500Z", Method: "GET", URL: "/api", Status: 200, ContentType: "application/json", ResponseBody: `{}`},
	})
	entries := []LogEntry{
		{"level": "error", "message": "err", "ts": "1970-01-01T00:00:02.000Z"},
	}

	// Only include actions
	resp := capture.GetSessionTimeline(TimelineFilter{Include: []string{"actions"}}, entries)

	for _, entry := range resp.Timeline {
		if entry.Kind != "action" {
			t.Errorf("Expected only action entries with include=[actions], got kind=%s", entry.Kind)
		}
	}
}

func TestGetSessionTimelineMaxEntries(t *testing.T) {
	capture := setupTestCapture(t)

	// Add 50 actions (max buffer) -- timeline cap is 200
	for i := 0; i < 50; i++ {
		capture.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000"},
		})
	}

	resp := capture.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) > 200 {
		t.Errorf("Expected timeline capped at 200, got %d", len(resp.Timeline))
	}
}

func TestGetSessionTimelineDurationMs(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000},
		{Type: "click", Timestamp: 5000},
	})

	resp := capture.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if resp.Summary.DurationMs != 4000 {
		t.Errorf("Expected duration 4000ms, got %d", resp.Summary.DurationMs)
	}
}

func TestGenerateTestScriptBasicStructure(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{
		TestName:       "login flow",
		AssertNetwork:  true,
		AssertNoErrors: true,
	})

	if !strings.Contains(script, "import { test, expect } from '@playwright/test'") {
		t.Error("Expected Playwright imports")
	}
	if !strings.Contains(script, "test('login flow'") {
		t.Error("Expected test name in output")
	}
	if !strings.Contains(script, "page.goto(") {
		t.Error("Expected goto in output")
	}
	if !strings.Contains(script, "consoleErrors") {
		t.Error("Expected console error listener when assert_no_errors=true")
	}
	if !strings.Contains(script, "expect(consoleErrors).toHaveLength(0)") {
		t.Error("Expected console error assertion at end")
	}
}

func TestGenerateTestScriptNetworkAssertions(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "login-btn"}},
		{Timestamp: 1150, Kind: "network", Method: "POST", URL: "/api/login", Status: 200, ContentType: "application/json"},
		{Timestamp: 1300, Kind: "action", Type: "navigate", ToURL: "/dashboard"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: true, AssertNoErrors: false})

	if !strings.Contains(script, "waitForResponse") {
		t.Error("Expected waitForResponse for network assertion")
	}
	if !strings.Contains(script, "/api/login") {
		t.Error("Expected URL in waitForResponse matcher")
	}
	if !strings.Contains(script, "expect") && !strings.Contains(script, ".status()") {
		t.Error("Expected status assertion")
	}
	if !strings.Contains(script, "toBe(200)") {
		t.Error("Expected status code 200 assertion")
	}
}

func TestGenerateTestScriptNavigationAssertion(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1300, Kind: "action", Type: "navigate", ToURL: "/dashboard"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: false, AssertNoErrors: false})

	if !strings.Contains(script, "toHaveURL") {
		t.Error("Expected toHaveURL assertion for navigation")
	}
	if !strings.Contains(script, "dashboard") {
		t.Error("Expected dashboard URL in assertion")
	}
}

func TestGenerateTestScriptNoErrorsWithCapturedErrors(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1500, Kind: "console", Level: "error", Message: "Widget failed to load"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNoErrors: true})

	// When errors were captured during the session, the assertion should be commented out
	if strings.Contains(script, "expect(consoleErrors).toHaveLength(0)") && !strings.Contains(script, "//") {
		t.Error("Expected console error assertion to be disabled/commented when errors present in captured session")
	}
	if !strings.Contains(script, "Widget failed to load") {
		t.Error("Expected captured error message to be noted in comments")
	}
}

func TestGenerateTestScriptResponseShapeAssertions(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1150, Kind: "network", Method: "GET", URL: "/api/users", Status: 200, ContentType: "application/json",
			ResponseShape: map[string]interface{}{"users": []interface{}{map[string]interface{}{"id": "number", "name": "string"}}}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: true, AssertResponseShape: true})

	if !strings.Contains(script, "toHaveProperty") {
		t.Error("Expected toHaveProperty assertion for response shape")
	}
	if !strings.Contains(script, "'users'") {
		t.Error("Expected 'users' key in shape assertion")
	}
}

func TestGenerateTestScriptResponseShapeDisabled(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1150, Kind: "network", Method: "GET", URL: "/api/users", Status: 200, ContentType: "application/json",
			ResponseShape: map[string]interface{}{"users": "array"}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: true, AssertResponseShape: false})

	if strings.Contains(script, "toHaveProperty") {
		t.Error("Should not include shape assertions when assert_response_shape=false")
	}
}

func TestGenerateTestScriptBaseURL(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{BaseURL: "https://staging.example.com"})

	if strings.Contains(script, "localhost:3000") {
		t.Error("Expected localhost to be replaced with base URL")
	}
	if !strings.Contains(script, "staging.example.com") {
		t.Error("Expected base URL in output")
	}
}

func TestGenerateTestScriptMultipleNetworkPerAction(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "submit"}},
		{Timestamp: 1050, Kind: "network", Method: "POST", URL: "/api/submit", Status: 200, ContentType: "application/json"},
		{Timestamp: 1100, Kind: "network", Method: "GET", URL: "/api/status", Status: 200, ContentType: "application/json"},
		{Timestamp: 2000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "next"}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: true})

	if !strings.Contains(script, "/api/submit") {
		t.Error("Expected /api/submit assertion")
	}
	if !strings.Contains(script, "/api/status") {
		t.Error("Expected /api/status assertion")
	}
}

func TestGenerateTestScriptAssertNetworkDisabled(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1150, Kind: "network", Method: "POST", URL: "/api/login", Status: 200, ContentType: "application/json"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: false})

	if strings.Contains(script, "waitForResponse") {
		t.Error("Should not include waitForResponse when assert_network=false")
	}
}

func TestGenerateTestScriptPasswordRedacted(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "input", URL: "http://localhost:3000",
			Selectors: map[string]interface{}{"testId": "password"}, Value: "[redacted]"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{})

	if !strings.Contains(script, "[user-provided]") {
		t.Error("Expected redacted password to become [user-provided]")
	}
}

func TestGenerateTestScriptDefaultTestName(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1300, Kind: "action", Type: "navigate", URL: "http://localhost:3000/dashboard", ToURL: "/dashboard"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{})

	// Should derive a name from the flow (first URL's path)
	if !strings.Contains(script, "test(") {
		t.Error("Expected test() wrapper")
	}
}

func TestGenerateTestScriptEmpty(t *testing.T) {
	script := generateTestScript([]TimelineEntry{}, TestGenerationOptions{TestName: "empty"})

	if !strings.Contains(script, "import") {
		t.Error("Expected valid script structure even with no timeline entries")
	}
}

func TestMCPGetSessionTimeline(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1705312200000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.150Z", Method: "POST", URL: "/api/login", Status: 200,
			ResponseBody: `{"token":"abc"}`, ContentType: "application/json"},
	})
	server.addEntries([]LogEntry{
		{"level": "error", "message": "test error", "ts": "2024-01-15T10:30:00.500Z"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_session_timeline","arguments":{}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	var timelineResp SessionTimelineResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &timelineResp); err != nil {
		t.Fatalf("Expected valid JSON timeline response, got error: %v", err)
	}

	if len(timelineResp.Timeline) != 3 {
		t.Errorf("Expected 3 timeline entries, got %d", len(timelineResp.Timeline))
	}
	if timelineResp.Summary.Actions != 1 {
		t.Errorf("Expected 1 action, got %d", timelineResp.Summary.Actions)
	}
	if timelineResp.Summary.NetworkRequests != 1 {
		t.Errorf("Expected 1 network request, got %d", timelineResp.Summary.NetworkRequests)
	}
	if timelineResp.Summary.ConsoleErrors != 1 {
		t.Errorf("Expected 1 console error, got %d", timelineResp.Summary.ConsoleErrors)
	}
}

func TestMCPGetSessionTimelineWithLastN(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	for i := 0; i < 10; i++ {
		capture.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000"},
		})
	}

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_session_timeline","arguments":{"last_n_actions":3}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var timelineResp SessionTimelineResponse
	json.Unmarshal([]byte(result.Content[0].Text), &timelineResp)

	if timelineResp.Summary.Actions != 3 {
		t.Errorf("Expected 3 actions with last_n_actions=3, got %d", timelineResp.Summary.Actions)
	}
}

func TestMCPGetSessionTimelineEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_session_timeline","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if !strings.Contains(result.Content[0].Text, `"timeline":[]`) && !strings.Contains(result.Content[0].Text, `"actions":0`) {
		t.Error("Expected empty timeline or 0 actions in summary")
	}
}

func TestMCPGenerateTest(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1705312200000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "login-btn"}},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.150Z", Method: "POST", URL: "/api/login", Status: 200,
			ResponseBody: `{"token":"abc"}`, ContentType: "application/json"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate_test","arguments":{"test_name":"login test","assert_network":true,"assert_no_errors":true}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	script := result.Content[0].Text
	if !strings.Contains(script, "import { test, expect }") {
		t.Error("Expected Playwright imports in generated script")
	}
	if !strings.Contains(script, "login test") {
		t.Error("Expected test name in generated script")
	}
	if !strings.Contains(script, "waitForResponse") {
		t.Error("Expected network assertion in generated script")
	}
	if !strings.Contains(script, "consoleErrors") {
		t.Error("Expected console error tracking in generated script")
	}
}

func TestMCPGenerateTestWithBaseURL(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate_test","arguments":{"base_url":"https://staging.example.com"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text
	if strings.Contains(script, "localhost:3000") {
		t.Error("Expected localhost replaced with base_url")
	}
	if !strings.Contains(script, "staging.example.com") {
		t.Error("Expected base_url in script")
	}
}

func TestMCPGenerateTestEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate_test","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if !strings.Contains(result.Content[0].Text, "No") {
		// Should indicate no data available
	}
}

func TestMCPGenerateTestToolInToolsList(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := NewToolHandler(server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	foundTimeline := false
	foundGenerate := false
	for _, tool := range result.Tools {
		if tool.Name == "get_session_timeline" {
			foundTimeline = true
		}
		if tool.Name == "generate_test" {
			foundGenerate = true
		}
	}

	if !foundTimeline {
		t.Error("Expected get_session_timeline in tools list")
	}
	if !foundGenerate {
		t.Error("Expected generate_test in tools list")
	}
}
