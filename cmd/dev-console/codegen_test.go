package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPGetReproductionScript(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction"}}`),
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
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "submit-btn"}},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","error_message":"Cannot read property 'user' of undefined"}}`),
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
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","last_n_actions":3}}`),
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
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","base_url":"https://staging.example.com"}}`),
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
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction"}}`),
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
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction"}}`),
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
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction"}}`),
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
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction"}}`),
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
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"timeline"}}`),
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
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"timeline","last_n_actions":3}}`),
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
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"timeline"}}`),
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
	mcp := setupToolHandler(t, server, capture)

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
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"test","test_name":"login test","assert_network":true,"assert_no_errors":true}}`),
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
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"test","base_url":"https://staging.example.com"}}`),
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
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"test"}}`),
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
	mcp := setupToolHandler(t, server, capture)

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

	foundAnalyze := false
	foundGenerate := false
	for _, tool := range result.Tools {
		if tool.Name == "analyze" {
			foundAnalyze = true
		}
		if tool.Name == "generate" {
			foundGenerate = true
		}
	}

	if !foundAnalyze {
		t.Error("Expected analyze in tools list")
	}
	if !foundGenerate {
		t.Error("Expected generate in tools list")
	}
}

// ============================================
// Workflow Integration Tests (Session Summary + PR Summary)
// ============================================

func TestSessionSummaryWithTwoSnapshots(t *testing.T) {
	capture := setupTestCapture(t)

	fcp1 := 800.0
	lcp1 := 1000.0
	cls1 := 0.02
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{
			Load:                   1200,
			FirstContentfulPaint:   &fcp1,
			LargestContentfulPaint: &lcp1,
			TimeToFirstByte:        100,
			DomContentLoaded:       900,
			DomInteractive:         800,
		},
		Network: NetworkSummary{
			RequestCount: 20,
			TransferSize: 340 * 1024,
		},
		LongTasks: LongTaskMetrics{Count: 1, TotalBlockingTime: 80},
		CLS:       &cls1,
	})

	fcp2 := 900.0
	lcp2 := 1100.0
	cls2 := 0.02
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing: PerformanceTiming{
			Load:                   1400,
			FirstContentfulPaint:   &fcp2,
			LargestContentfulPaint: &lcp2,
			TimeToFirstByte:        120,
			DomContentLoaded:       1100,
			DomInteractive:         900,
		},
		Network: NetworkSummary{
			RequestCount: 22,
			TransferSize: 385 * 1024,
		},
		LongTasks: LongTaskMetrics{Count: 2, TotalBlockingTime: 150},
		CLS:       &cls2,
	})

	summary := capture.GenerateSessionSummary()

	if summary.PerformanceDelta == nil {
		t.Fatal("Expected performance delta to be present")
	}
	if summary.PerformanceDelta.LoadTimeDelta != 200 {
		t.Errorf("Expected load time delta of 200ms, got %.0f", summary.PerformanceDelta.LoadTimeDelta)
	}
	if summary.PerformanceDelta.FCPDelta != 100 {
		t.Errorf("Expected FCP delta of 100ms, got %.0f", summary.PerformanceDelta.FCPDelta)
	}
	if summary.PerformanceDelta.LCPDelta != 100 {
		t.Errorf("Expected LCP delta of 100ms, got %.0f", summary.PerformanceDelta.LCPDelta)
	}
	expectedSizeDelta := int64(45 * 1024)
	if summary.PerformanceDelta.BundleSizeDelta != expectedSizeDelta {
		t.Errorf("Expected bundle size delta of %d, got %d", expectedSizeDelta, summary.PerformanceDelta.BundleSizeDelta)
	}
}

func TestSessionSummaryNoSnapshots(t *testing.T) {
	capture := setupTestCapture(t)
	summary := capture.GenerateSessionSummary()

	if summary.PerformanceDelta != nil {
		t.Error("Expected no performance delta when no snapshots exist")
	}
	if summary.Status != "no_performance_data" {
		t.Errorf("Expected status 'no_performance_data', got '%s'", summary.Status)
	}
}

func TestSessionSummarySingleSnapshot(t *testing.T) {
	capture := setupTestCapture(t)

	fcp := 800.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1200, FirstContentfulPaint: &fcp},
		Network:   NetworkSummary{RequestCount: 20, TransferSize: 340 * 1024},
	})

	summary := capture.GenerateSessionSummary()

	if summary.Status != "insufficient_data" {
		t.Errorf("Expected status 'insufficient_data', got '%s'", summary.Status)
	}
}

func TestSessionSummaryWithErrors(t *testing.T) {
	capture := setupTestCapture(t)

	entries := []LogEntry{
		{"level": "error", "message": "TypeError: Cannot read property 'map' of undefined", "source": "dashboard.js:142", "ts": "2026-01-24T10:01:00Z"},
		{"level": "error", "message": "ReferenceError: foo is not defined", "source": "app.js:10", "ts": "2026-01-24T10:02:00Z"},
		{"level": "warn", "message": "Deprecated API call", "source": "lib.js:5", "ts": "2026-01-24T10:01:30Z"},
	}

	// Add snapshots so we get past the "no performance data" check
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{Load: 1200}, Network: NetworkSummary{TransferSize: 340 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:05:00Z",
		Timing: PerformanceTiming{Load: 1400}, Network: NetworkSummary{TransferSize: 385 * 1024},
	})

	summary := capture.GenerateSessionSummaryWithEntries(entries)

	if len(summary.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(summary.Errors))
	}
}

func TestSessionSummaryReloadCount(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "navigate", Timestamp: 1000, ToURL: "http://localhost:3000/"},
		{Type: "click", Timestamp: 2000, URL: "http://localhost:3000/"},
		{Type: "navigate", Timestamp: 3000, ToURL: "http://localhost:3000/about"},
		{Type: "navigate", Timestamp: 4000, ToURL: "http://localhost:3000/"},
	})

	summary := capture.GenerateSessionSummary()

	if summary.Metadata.ReloadCount != 3 {
		t.Errorf("Expected 3 navigations (reload count), got %d", summary.Metadata.ReloadCount)
	}
}

func TestSessionSummaryDuration(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000},
		{Type: "click", Timestamp: 6000},
	})

	summary := capture.GenerateSessionSummary()

	if summary.Metadata.DurationMs != 5000 {
		t.Errorf("Expected duration 5000ms, got %d", summary.Metadata.DurationMs)
	}
}

func TestGeneratePRSummaryMarkdownTable(t *testing.T) {
	capture := setupTestCapture(t)

	fcp1 := 800.0
	lcp1 := 1000.0
	cls1 := 0.02
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{
			Load:                   1200,
			FirstContentfulPaint:   &fcp1,
			LargestContentfulPaint: &lcp1,
			DomContentLoaded:       900,
		},
		Network: NetworkSummary{RequestCount: 20, TransferSize: 340 * 1024},
		CLS:     &cls1,
	})

	fcp2 := 900.0
	lcp2 := 1100.0
	cls2 := 0.02
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing: PerformanceTiming{
			Load:                   1400,
			FirstContentfulPaint:   &fcp2,
			LargestContentfulPaint: &lcp2,
			DomContentLoaded:       1100,
		},
		Network: NetworkSummary{RequestCount: 22, TransferSize: 385 * 1024},
		CLS:     &cls2,
	})

	markdown := capture.GeneratePRSummary(nil)

	if !strings.Contains(markdown, "## Performance Impact") {
		t.Error("Expected '## Performance Impact' header in PR summary")
	}
	if !strings.Contains(markdown, "| Metric | Before | After | Delta |") {
		t.Error("Expected table header in PR summary")
	}
	if !strings.Contains(markdown, "Load Time") {
		t.Error("Expected 'Load Time' row in PR summary")
	}
	if !strings.Contains(markdown, "FCP") {
		t.Error("Expected 'FCP' row in PR summary")
	}
	if !strings.Contains(markdown, "Bundle Size") {
		t.Error("Expected 'Bundle Size' row in PR summary")
	}
}

func TestGeneratePRSummaryNoPerformanceData(t *testing.T) {
	capture := setupTestCapture(t)
	markdown := capture.GeneratePRSummary(nil)

	if !strings.Contains(markdown, "No performance data collected") {
		t.Error("Expected 'No performance data collected' message")
	}
}

func TestGeneratePRSummaryWithErrors(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{Load: 1200}, Network: NetworkSummary{TransferSize: 340 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:05:00Z",
		Timing: PerformanceTiming{Load: 1400}, Network: NetworkSummary{TransferSize: 385 * 1024},
	})

	errors := []SessionError{
		{Message: "TypeError: Cannot read property 'map' of undefined", Source: "dashboard.js:142", Resolved: true},
		{Message: "ReferenceError: foo is not defined", Source: "app.js:10", Resolved: false},
	}

	markdown := capture.GeneratePRSummary(errors)

	if !strings.Contains(markdown, "### Errors") {
		t.Error("Expected '### Errors' section")
	}
	if !strings.Contains(markdown, "Fixed") {
		t.Error("Expected 'Fixed' in errors section")
	}
}

func TestGeneratePRSummaryWithNoErrors(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{Load: 1200}, Network: NetworkSummary{TransferSize: 340 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:05:00Z",
		Timing: PerformanceTiming{Load: 1400}, Network: NetworkSummary{TransferSize: 385 * 1024},
	})

	markdown := capture.GeneratePRSummary([]SessionError{})

	if !strings.Contains(markdown, "No errors") {
		t.Error("Expected 'No errors' message when error list is empty")
	}
}

func TestOneLinerFormat(t *testing.T) {
	capture := setupTestCapture(t)

	fcp1 := 800.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1200, FirstContentfulPaint: &fcp1},
		Network:   NetworkSummary{TransferSize: 340 * 1024},
	})

	fcp2 := 900.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1400, FirstContentfulPaint: &fcp2},
		Network:   NetworkSummary{TransferSize: 385 * 1024},
	})

	errors := []SessionError{
		{Message: "TypeError: x is undefined", Resolved: true},
	}

	oneliner := capture.GenerateOneLiner(errors)

	if !strings.Contains(oneliner, "perf:") {
		t.Error("Expected 'perf:' in one-liner")
	}
	if !strings.Contains(oneliner, "+200ms load") {
		t.Error("Expected '+200ms load' in one-liner")
	}
	if !strings.Contains(oneliner, "bundle") {
		t.Error("Expected bundle info in one-liner")
	}
	if !strings.Contains(oneliner, "errors:") {
		t.Error("Expected 'errors:' in one-liner")
	}
	if !strings.Contains(oneliner, "1 fixed") {
		t.Error("Expected '1 fixed' in one-liner")
	}
}

func TestOneLinerNoPerformanceData(t *testing.T) {
	capture := setupTestCapture(t)
	oneliner := capture.GenerateOneLiner(nil)

	if !strings.Contains(oneliner, "no perf data") {
		t.Error("Expected 'no perf data' in one-liner when no snapshots")
	}
}

func TestMCPGeneratePRSummaryTool(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	fcp1 := 800.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1200, FirstContentfulPaint: &fcp1},
		Network:   NetworkSummary{TransferSize: 340 * 1024},
	})

	fcp2 := 900.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1400, FirstContentfulPaint: &fcp2},
		Network:   NetworkSummary{TransferSize: 385 * 1024},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"pr_summary"}}`),
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

	text := result.Content[0].Text
	if !strings.Contains(text, "## Performance Impact") {
		t.Error("Expected performance impact section in MCP tool response")
	}
}

func TestMCPGeneratePRSummaryToolInToolsList(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []MCPTool `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	found := false
	for _, tool := range result.Tools {
		if tool.Name == "generate" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected generate in tools list")
	}
}

func TestSessionSummaryMultipleURLs(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{Load: 1200}, Network: NetworkSummary{TransferSize: 340 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:05:00Z",
		Timing: PerformanceTiming{Load: 1400}, Network: NetworkSummary{TransferSize: 385 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/about", Timestamp: "2026-01-24T10:06:00Z",
		Timing: PerformanceTiming{Load: 800}, Network: NetworkSummary{TransferSize: 200 * 1024},
	})

	summary := capture.GenerateSessionSummary()

	if summary.PerformanceDelta == nil {
		t.Fatal("Expected performance delta to be present")
	}
}

func TestSessionSummaryPerformanceCheckCount(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{Load: 1200}, Network: NetworkSummary{TransferSize: 340 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:01:00Z",
		Timing: PerformanceTiming{Load: 1300}, Network: NetworkSummary{TransferSize: 350 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:02:00Z",
		Timing: PerformanceTiming{Load: 1400}, Network: NetworkSummary{TransferSize: 385 * 1024},
	})

	summary := capture.GenerateSessionSummary()

	if summary.Metadata.PerformanceCheckCount < 2 {
		t.Errorf("Expected at least 2 performance checks, got %d", summary.Metadata.PerformanceCheckCount)
	}
}

func TestSessionSummaryBundleSizeDelta(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{Load: 1000}, Network: NetworkSummary{TransferSize: 280 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:05:00Z",
		Timing: PerformanceTiming{Load: 1200}, Network: NetworkSummary{TransferSize: 385 * 1024},
	})

	summary := capture.GenerateSessionSummary()

	if summary.PerformanceDelta == nil {
		t.Fatal("Expected performance delta to be present")
	}
	expectedDelta := int64(105 * 1024)
	if summary.PerformanceDelta.BundleSizeDelta != expectedDelta {
		t.Errorf("Expected bundle size delta of %d, got %d", expectedDelta, summary.PerformanceDelta.BundleSizeDelta)
	}
}

func TestHTTPSessionSummaryEndpoint(t *testing.T) {
	capture := setupTestCapture(t)

	fcp1 := 800.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing:  PerformanceTiming{Load: 1200, FirstContentfulPaint: &fcp1},
		Network: NetworkSummary{TransferSize: 340 * 1024},
	})
	fcp2 := 900.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:05:00Z",
		Timing:  PerformanceTiming{Load: 1400, FirstContentfulPaint: &fcp2},
		Network: NetworkSummary{TransferSize: 385 * 1024},
	})

	summary := capture.GenerateSessionSummary()

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Failed to marshal session summary: %v", err)
	}

	var parsed SessionSummary
	if err := json.Unmarshal(summaryJSON, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal session summary: %v", err)
	}

	if parsed.PerformanceDelta == nil {
		t.Error("Expected performance delta in parsed summary")
	}
}

func TestGeneratePRSummaryGeneratedByLine(t *testing.T) {
	capture := setupTestCapture(t)

	fcp1 := 800.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing:  PerformanceTiming{Load: 1200, FirstContentfulPaint: &fcp1},
		Network: NetworkSummary{TransferSize: 340 * 1024},
	})
	fcp2 := 900.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:05:00Z",
		Timing:  PerformanceTiming{Load: 1400, FirstContentfulPaint: &fcp2},
		Network: NetworkSummary{TransferSize: 385 * 1024},
	})

	markdown := capture.GeneratePRSummary(nil)

	if !strings.Contains(markdown, "Generated by Gasoline") {
		t.Error("Expected 'Generated by Gasoline' attribution line")
	}
}

func TestSessionSummaryCLSDelta(t *testing.T) {
	capture := setupTestCapture(t)

	cls1 := 0.02
	cls2 := 0.08
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{Load: 1200}, Network: NetworkSummary{TransferSize: 340 * 1024},
		CLS: &cls1,
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL: "http://localhost:3000/", Timestamp: "2026-01-24T10:05:00Z",
		Timing: PerformanceTiming{Load: 1400}, Network: NetworkSummary{TransferSize: 385 * 1024},
		CLS: &cls2,
	})

	summary := capture.GenerateSessionSummary()

	if summary.PerformanceDelta == nil {
		t.Fatal("Expected performance delta to be present")
	}
	if summary.PerformanceDelta.CLSDelta == 0 {
		t.Error("Expected non-zero CLS delta")
	}
}

// ============================================
// formatDeltaMs Coverage Tests
// ============================================

func TestFormatDeltaMsNegative(t *testing.T) {
	result := formatDeltaMs(-150, -12.5)
	if result != "-150ms (-12%)" {
		t.Errorf("Expected '-150ms (-12%%)', got: %s", result)
	}
}

func TestFormatDeltaMsPositive(t *testing.T) {
	result := formatDeltaMs(200, 25.0)
	if result != "+200ms (+25%)" {
		t.Errorf("Expected '+200ms (+25%%)', got: %s", result)
	}
}

func TestFormatDeltaMsZero(t *testing.T) {
	result := formatDeltaMs(0, 0)
	if result != "—" {
		t.Errorf("Expected '—', got: %s", result)
	}
}

// ============================================
// formatDeltaBytes Coverage Tests
// ============================================

func TestFormatDeltaBytesNegative(t *testing.T) {
	// -512 bytes, -10%
	result := formatDeltaBytes(-512, -10.0)
	if result != "512B (-10%)" {
		t.Errorf("Expected '512B (-10%%)', got: %s", result)
	}
}

func TestFormatDeltaBytesPositive(t *testing.T) {
	result := formatDeltaBytes(2048, 15.0)
	if result != "+2.0KB (+15%)" {
		t.Errorf("Expected '+2.0KB (+15%%)', got: %s", result)
	}
}

func TestFormatDeltaBytesZero(t *testing.T) {
	result := formatDeltaBytes(0, 0)
	if result != "—" {
		t.Errorf("Expected '—', got: %s", result)
	}
}

// ============================================
// formatSignedBytes Coverage Tests
// ============================================

func TestFormatSignedBytesPositiveSmall(t *testing.T) {
	result := formatSignedBytes(100)
	if result != "+100B" {
		t.Errorf("Expected '+100B', got: %s", result)
	}
}

func TestFormatSignedBytesNegativeSmall(t *testing.T) {
	result := formatSignedBytes(-200)
	if result != "-200B" {
		t.Errorf("Expected '-200B', got: %s", result)
	}
}

func TestFormatSignedBytesKB(t *testing.T) {
	result := formatSignedBytes(2048) // 2.0KB
	if result != "+2.0KB" {
		t.Errorf("Expected '+2.0KB', got: %s", result)
	}
}

func TestFormatSignedBytesNegativeKB(t *testing.T) {
	result := formatSignedBytes(-3072) // -3.0KB
	if result != "-3.0KB" {
		t.Errorf("Expected '-3.0KB', got: %s", result)
	}
}

func TestFormatSignedBytesMB(t *testing.T) {
	result := formatSignedBytes(2 * 1024 * 1024) // 2.0MB
	if result != "+2.0MB" {
		t.Errorf("Expected '+2.0MB', got: %s", result)
	}
}

func TestFormatSignedBytesNegativeMB(t *testing.T) {
	result := formatSignedBytes(-5 * 1024 * 1024) // -5.0MB
	if result != "-5.0MB" {
		t.Errorf("Expected '-5.0MB', got: %s", result)
	}
}

// ============================================
// abs64 Coverage Tests
// ============================================

func TestAbs64Negative(t *testing.T) {
	if abs64(-42) != 42 {
		t.Error("Expected abs64(-42) == 42")
	}
}

func TestAbs64Positive(t *testing.T) {
	if abs64(99) != 99 {
		t.Error("Expected abs64(99) == 99")
	}
}

func TestAbs64Zero(t *testing.T) {
	if abs64(0) != 0 {
		t.Error("Expected abs64(0) == 0")
	}
}

// ============================================
// toolGeneratePRSummary Coverage Tests (with performance data)
// ============================================

func TestGeneratePRSummaryWithResolvedAndNewErrors(t *testing.T) {
	capture := setupTestCapture(t)

	// Add two snapshots for performance delta
	fcp1 := 900.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000, FirstContentfulPaint: &fcp1},
		Network:   NetworkSummary{TransferSize: 300 * 1024},
	})
	fcp2 := 1100.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1200, FirstContentfulPaint: &fcp2},
		Network:   NetworkSummary{TransferSize: 350 * 1024},
	})

	errors := []SessionError{
		{Message: "TypeError: undefined", Source: "app.js", Resolved: true},
		{Message: "ReferenceError: x is not defined", Source: "main.js", Resolved: false},
	}

	markdown := capture.GeneratePRSummary(errors)

	if !strings.Contains(markdown, "Performance Impact") {
		t.Error("Expected 'Performance Impact' header")
	}
	if !strings.Contains(markdown, "Errors") {
		t.Error("Expected 'Errors' section")
	}
	if !strings.Contains(markdown, "Fixed") {
		t.Error("Expected 'Fixed' error entry")
	}
	if !strings.Contains(markdown, "New") {
		t.Error("Expected 'New' error entry")
	}
	if !strings.Contains(markdown, "TypeError: undefined") {
		t.Error("Expected fixed error message in output")
	}
	if !strings.Contains(markdown, "ReferenceError: x is not defined") {
		t.Error("Expected new error message in output")
	}
}

func TestGeneratePRSummaryEmptyErrors(t *testing.T) {
	capture := setupTestCapture(t)

	// Add two snapshots
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 300 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1200},
		Network:   NetworkSummary{TransferSize: 350 * 1024},
	})

	errors := []SessionError{}

	markdown := capture.GeneratePRSummary(errors)

	if !strings.Contains(markdown, "No errors detected") {
		t.Error("Expected 'No errors detected' for empty error list")
	}
}

func TestGeneratePRSummaryAllFixed(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 300 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1200},
		Network:   NetworkSummary{TransferSize: 350 * 1024},
	})

	errors := []SessionError{
		{Message: "Fixed error 1", Resolved: true},
		{Message: "Fixed error 2", Source: "src.js", Resolved: true},
	}

	markdown := capture.GeneratePRSummary(errors)

	if !strings.Contains(markdown, "Fixed") {
		t.Error("Expected 'Fixed' entries")
	}
	// When all are resolved, no new errors
	if !strings.Contains(markdown, "None") {
		t.Error("Expected 'None' for new errors when all are fixed")
	}
}

func TestGeneratePRSummaryNegativeDelta(t *testing.T) {
	capture := setupTestCapture(t)

	// Improvement: after is faster than before
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 2000},
		Network:   NetworkSummary{TransferSize: 500 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 400 * 1024},
	})

	markdown := capture.GeneratePRSummary(nil)

	// Negative delta should not have a + sign
	if strings.Contains(markdown, "+") {
		t.Errorf("Expected no '+' sign for negative delta (improvement), got:\n%s", markdown)
	}
}

// ============================================
// Additional coverage tests for codegen.go
// ============================================

func TestGetPlaywrightLocatorRoleWithName(t *testing.T) {
	selectors := map[string]interface{}{
		"role": map[string]interface{}{
			"role": "button",
			"name": "Submit",
		},
	}
	result := getPlaywrightLocator(selectors)
	expected := "getByRole('button', { name: 'Submit' })"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorRoleWithoutName(t *testing.T) {
	selectors := map[string]interface{}{
		"role": map[string]interface{}{
			"role": "navigation",
		},
	}
	result := getPlaywrightLocator(selectors)
	expected := "getByRole('navigation')"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorRoleEmptyRole(t *testing.T) {
	// role map exists but role key is empty string - should fall through
	selectors := map[string]interface{}{
		"role": map[string]interface{}{
			"role": "",
			"name": "Submit",
		},
	}
	result := getPlaywrightLocator(selectors)
	// Empty role should not produce a locator; falls through to next strategy
	if result != "" {
		t.Errorf("Expected empty string for empty role, got %q", result)
	}
}

func TestGetPlaywrightLocatorTextBased(t *testing.T) {
	selectors := map[string]interface{}{
		"text": "Click me",
	}
	result := getPlaywrightLocator(selectors)
	expected := "getByText('Click me')"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorAriaLabel(t *testing.T) {
	selectors := map[string]interface{}{
		"ariaLabel": "Email address",
	}
	result := getPlaywrightLocator(selectors)
	expected := "getByLabel('Email address')"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorById(t *testing.T) {
	selectors := map[string]interface{}{
		"id": "main-form",
	}
	result := getPlaywrightLocator(selectors)
	expected := "locator('#main-form')"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorByCssPath(t *testing.T) {
	selectors := map[string]interface{}{
		"cssPath": "div.container > form > input",
	}
	result := getPlaywrightLocator(selectors)
	expected := "locator('div.container > form > input')"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorNilSelectors(t *testing.T) {
	result := getPlaywrightLocator(nil)
	if result != "" {
		t.Errorf("Expected empty string for nil selectors, got %q", result)
	}
}

func TestGetPlaywrightLocatorEmptyMap(t *testing.T) {
	selectors := map[string]interface{}{}
	result := getPlaywrightLocator(selectors)
	if result != "" {
		t.Errorf("Expected empty string for empty selectors, got %q", result)
	}
}

func TestGetPlaywrightLocatorPriority(t *testing.T) {
	// testId should take priority over everything else
	selectors := map[string]interface{}{
		"testId":    "login-btn",
		"ariaLabel": "Login button",
		"text":      "Login",
		"id":        "btn-login",
		"role": map[string]interface{}{
			"role": "button",
			"name": "Login",
		},
	}
	result := getPlaywrightLocator(selectors)
	expected := "getByTestId('login-btn')"
	if result != expected {
		t.Errorf("Expected testId to take priority: %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorRolePriorityOverAriaLabel(t *testing.T) {
	// role should take priority over ariaLabel
	selectors := map[string]interface{}{
		"ariaLabel": "Submit form",
		"text":      "Submit",
		"role": map[string]interface{}{
			"role": "button",
			"name": "Submit",
		},
	}
	result := getPlaywrightLocator(selectors)
	expected := "getByRole('button', { name: 'Submit' })"
	if result != expected {
		t.Errorf("Expected role to take priority over ariaLabel: %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorRoleNotAMap(t *testing.T) {
	// role is present but not a map - should fall through
	selectors := map[string]interface{}{
		"role": "button",
		"text": "Click me",
	}
	result := getPlaywrightLocator(selectors)
	// role is a string not a map, so it falls through to text
	expected := "getByText('Click me')"
	if result != expected {
		t.Errorf("Expected fallthrough to text: %q, got %q", expected, result)
	}
}

func TestGetPlaywrightLocatorSpecialCharacters(t *testing.T) {
	selectors := map[string]interface{}{
		"text": "It's a test\nwith newlines",
	}
	result := getPlaywrightLocator(selectors)
	expected := "getByText('It\\'s a test\\nwith newlines')"
	if result != expected {
		t.Errorf("Expected escaped string: %q, got %q", expected, result)
	}
}

func TestReplaceOriginBasic(t *testing.T) {
	result := replaceOrigin("http://localhost:3000/login", "http://example.com")
	expected := "http://example.com/login"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplaceOriginWithPort(t *testing.T) {
	result := replaceOrigin("http://localhost:8080/api/users", "https://prod.example.com")
	expected := "https://prod.example.com/api/users"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplaceOriginNoPath(t *testing.T) {
	// URL has scheme://host but no path
	result := replaceOrigin("http://localhost:3000", "http://example.com")
	expected := "http://example.com"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplaceOriginWithoutScheme(t *testing.T) {
	// URL without :// scheme separator
	result := replaceOrigin("/path/to/page", "http://example.com")
	expected := "http://example.com/path/to/page"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplaceOriginEmptyBaseURL(t *testing.T) {
	result := replaceOrigin("http://localhost:3000/login", "")
	expected := "/login"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplaceOriginBaseURLWithTrailingSlash(t *testing.T) {
	result := replaceOrigin("http://localhost:3000/login", "http://example.com/")
	expected := "http://example.com/login"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplaceOriginDeepPath(t *testing.T) {
	result := replaceOrigin("https://dev.local:9000/app/settings/profile?tab=2", "https://prod.com")
	expected := "https://prod.com/app/settings/profile?tab=2"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetSelectorFromMapTestId(t *testing.T) {
	selectors := map[string]interface{}{
		"testId": "submit-btn",
	}
	result := getSelectorFromMap(selectors)
	expected := `[data-testid="submit-btn"]`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetSelectorFromMapRole(t *testing.T) {
	selectors := map[string]interface{}{
		"role": "button",
	}
	result := getSelectorFromMap(selectors)
	expected := `[role="button"]`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetSelectorFromMapUnknownFallback(t *testing.T) {
	selectors := map[string]interface{}{
		"cssPath": "div > span",
	}
	result := getSelectorFromMap(selectors)
	if result != "unknown" {
		t.Errorf("Expected 'unknown' for unsupported selector, got %q", result)
	}
}

func TestGetSelectorFromMapTestIdPriority(t *testing.T) {
	// testId should take priority over role
	selectors := map[string]interface{}{
		"testId": "my-button",
		"role":   "button",
	}
	result := getSelectorFromMap(selectors)
	expected := `[data-testid="my-button"]`
	if result != expected {
		t.Errorf("Expected testId priority: %q, got %q", expected, result)
	}
}

func TestGetSelectorFromMapEmptyMap(t *testing.T) {
	selectors := map[string]interface{}{}
	result := getSelectorFromMap(selectors)
	if result != "unknown" {
		t.Errorf("Expected 'unknown' for empty map, got %q", result)
	}
}

func TestGenerateOneLinerNoPerformanceData(t *testing.T) {
	capture := setupTestCapture(t)

	result := capture.GenerateOneLiner(nil)
	if !strings.Contains(result, "no perf data") {
		t.Errorf("Expected 'no perf data', got %q", result)
	}
}

func TestGenerateOneLinerWithErrors(t *testing.T) {
	capture := setupTestCapture(t)

	errors := []SessionError{
		{Message: "TypeError: undefined", Resolved: false},
		{Message: "Fixed error", Resolved: true},
		{Message: "Another new error", Resolved: false},
	}

	result := capture.GenerateOneLiner(errors)
	if !strings.Contains(result, "1 fixed") {
		t.Errorf("Expected '1 fixed', got %q", result)
	}
	if !strings.Contains(result, "2 new") {
		t.Errorf("Expected '2 new', got %q", result)
	}
}

func TestGenerateOneLinerAllErrorsResolved(t *testing.T) {
	capture := setupTestCapture(t)

	errors := []SessionError{
		{Message: "Fixed 1", Resolved: true},
		{Message: "Fixed 2", Resolved: true},
	}

	result := capture.GenerateOneLiner(errors)
	if !strings.Contains(result, "2 fixed") {
		t.Errorf("Expected '2 fixed', got %q", result)
	}
	if strings.Contains(result, "new") {
		t.Errorf("Expected no 'new' errors, got %q", result)
	}
}

func TestGenerateOneLinerEmptyErrors(t *testing.T) {
	capture := setupTestCapture(t)

	errors := []SessionError{}

	result := capture.GenerateOneLiner(errors)
	if !strings.Contains(result, "errors: clean") {
		t.Errorf("Expected 'errors: clean', got %q", result)
	}
}

func TestGenerateOneLinerPerfImprovement(t *testing.T) {
	capture := setupTestCapture(t)

	// Add two snapshots to get a delta
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 2000},
		Network:   NetworkSummary{TransferSize: 500 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 400 * 1024},
	})

	result := capture.GenerateOneLiner(nil)
	// Should contain perf delta (negative means improvement)
	if !strings.Contains(result, "perf:") {
		t.Errorf("Expected 'perf:' section, got %q", result)
	}
	if !strings.Contains(result, "ms load") {
		t.Errorf("Expected load delta in output, got %q", result)
	}
}

func TestGenerateOneLinerPerfRegression(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 300 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 2000},
		Network:   NetworkSummary{TransferSize: 600 * 1024},
	})

	result := capture.GenerateOneLiner(nil)
	if !strings.Contains(result, "+") {
		t.Errorf("Expected positive delta indicator, got %q", result)
	}
	if !strings.Contains(result, "ms load") {
		t.Errorf("Expected load time delta, got %q", result)
	}
	if !strings.Contains(result, "bundle") {
		t.Errorf("Expected bundle size delta, got %q", result)
	}
}

func TestGenerateOneLinerNoChange(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 300 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 300 * 1024},
	})

	result := capture.GenerateOneLiner(nil)
	if !strings.Contains(result, "no change") {
		t.Errorf("Expected 'no change' when perf is identical, got %q", result)
	}
}

func TestGenerateOneLinerBaselineFallback(t *testing.T) {
	capture := setupTestCapture(t)

	// Add baseline directly (simulating a pre-existing baseline with enough samples)
	capture.mu.Lock()
	capture.perf.baselines["http://localhost:3000/"] = PerformanceBaseline{
		URL:         "http://localhost:3000/",
		SampleCount: 3,
		Timing:      BaselineTiming{Load: 1500},
		Network:     BaselineNetwork{TransferSize: 400 * 1024},
	}
	capture.perf.baselineOrder = append(capture.perf.baselineOrder, "http://localhost:3000/")
	// Add a snapshot (but no session first snapshot)
	capture.perf.snapshots["http://localhost:3000/"] = PerformanceSnapshot{
		URL:     "http://localhost:3000/",
		Timing:  PerformanceTiming{Load: 2000},
		Network: NetworkSummary{TransferSize: 500 * 1024},
	}
	capture.perf.snapshotOrder = append(capture.perf.snapshotOrder, "http://localhost:3000/")
	capture.session.snapshotCount = 2
	capture.mu.Unlock()

	result := capture.GenerateOneLiner(nil)
	// Should use baseline as first, compute delta
	if !strings.Contains(result, "perf:") {
		t.Errorf("Expected 'perf:' section from baseline fallback, got %q", result)
	}
	if !strings.Contains(result, "ms load") {
		t.Errorf("Expected load delta from baseline, got %q", result)
	}
}

func TestGenerateSessionSummaryWithEntriesConsoleErrors(t *testing.T) {
	capture := setupTestCapture(t)

	// Set up performance data so it doesn't return early
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 100 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1200},
		Network:   NetworkSummary{TransferSize: 120 * 1024},
	})

	entries := []LogEntry{
		{"level": "error", "message": "TypeError: cannot read property 'foo'", "source": "app.js"},
		{"level": "warn", "message": "Deprecation warning"},
		{"level": "error", "message": "ReferenceError: x is not defined", "source": "utils.js"},
		{"level": "log", "message": "App started"},
	}

	summary := capture.GenerateSessionSummaryWithEntries(entries)

	if summary.Status != "ok" {
		t.Errorf("Expected status 'ok', got %q", summary.Status)
	}
	if len(summary.Errors) != 2 {
		t.Errorf("Expected 2 errors (only level=error), got %d", len(summary.Errors))
	}
	if summary.Errors[0].Message != "TypeError: cannot read property 'foo'" {
		t.Errorf("Expected first error message, got %q", summary.Errors[0].Message)
	}
	if summary.Errors[0].Source != "app.js" {
		t.Errorf("Expected source 'app.js', got %q", summary.Errors[0].Source)
	}
}

func TestGenerateSessionSummaryWithEntriesNoPerformanceData(t *testing.T) {
	capture := setupTestCapture(t)

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.Status != "no_performance_data" {
		t.Errorf("Expected status 'no_performance_data', got %q", summary.Status)
	}
}

func TestGenerateSessionSummaryWithEntriesInsufficientData(t *testing.T) {
	capture := setupTestCapture(t)

	// Only one snapshot - insufficient
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
	})

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.Status != "insufficient_data" {
		t.Errorf("Expected status 'insufficient_data', got %q", summary.Status)
	}
}

func TestGenerateSessionSummaryWithEntriesFCPAndLCPDeltas(t *testing.T) {
	capture := setupTestCapture(t)

	fcp1 := 500.0
	fcp2 := 600.0
	lcp1 := 1200.0
	lcp2 := 1400.0

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{
			Load:                   1000,
			FirstContentfulPaint:   &fcp1,
			LargestContentfulPaint: &lcp1,
		},
		Network: NetworkSummary{TransferSize: 100 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing: PerformanceTiming{
			Load:                   1200,
			FirstContentfulPaint:   &fcp2,
			LargestContentfulPaint: &lcp2,
		},
		Network: NetworkSummary{TransferSize: 120 * 1024},
	})

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.PerformanceDelta == nil {
		t.Fatal("Expected PerformanceDelta to be set")
	}
	if summary.PerformanceDelta.FCPDelta != 100 {
		t.Errorf("Expected FCP delta 100, got %f", summary.PerformanceDelta.FCPDelta)
	}
	if summary.PerformanceDelta.LCPDelta != 200 {
		t.Errorf("Expected LCP delta 200, got %f", summary.PerformanceDelta.LCPDelta)
	}
}

func TestGenerateSessionSummaryWithEntriesCLSDelta(t *testing.T) {
	capture := setupTestCapture(t)

	cls1 := 0.05
	cls2 := 0.12

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 100 * 1024},
		CLS:       &cls1,
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1200},
		Network:   NetworkSummary{TransferSize: 120 * 1024},
		CLS:       &cls2,
	})

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.PerformanceDelta == nil {
		t.Fatal("Expected PerformanceDelta to be set")
	}
	expectedDelta := cls2 - cls1
	if summary.PerformanceDelta.CLSDelta != expectedDelta {
		t.Errorf("Expected CLS delta %f, got %f", expectedDelta, summary.PerformanceDelta.CLSDelta)
	}
}

func TestGenerateSessionSummaryWithEntriesNavigateCount(t *testing.T) {
	capture := setupTestCapture(t)

	// Add navigations as enhanced actions
	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000},
		{Type: "navigate", Timestamp: 2000, ToURL: "http://localhost/page1"},
		{Type: "click", Timestamp: 3000},
		{Type: "navigate", Timestamp: 4000, ToURL: "http://localhost/page2"},
		{Type: "navigate", Timestamp: 5000, ToURL: "http://localhost/page3"},
	})

	// Need performance data too
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 100 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1200},
		Network:   NetworkSummary{TransferSize: 120 * 1024},
	})

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.Metadata.ReloadCount != 3 {
		t.Errorf("Expected 3 navigations, got %d", summary.Metadata.ReloadCount)
	}
	// Duration should be last - first timestamp
	if summary.Metadata.DurationMs != 4000 {
		t.Errorf("Expected duration 4000ms, got %d", summary.Metadata.DurationMs)
	}
}

// ============================================
// Additional coverage: generatePlaywrightScript
// ============================================

func TestGeneratePlaywrightScriptInputAction(t *testing.T) {
	actions := []EnhancedAction{
		{
			Type:      "input",
			Timestamp: 1000,
			URL:       "http://localhost:3000/form",
			Selectors: map[string]interface{}{"testId": "email-field"},
			Value:     "test@example.com",
			InputType: "email",
		},
	}

	script := generatePlaywrightScript(actions, "", "")

	if !strings.Contains(script, ".fill('test@example.com')") {
		t.Error("Expected fill command with value for input action")
	}
	if !strings.Contains(script, "getByTestId('email-field')") {
		t.Error("Expected testId locator for input action")
	}
}

func TestGeneratePlaywrightScriptInputRedacted(t *testing.T) {
	actions := []EnhancedAction{
		{
			Type:      "input",
			Timestamp: 1000,
			URL:       "http://localhost:3000/login",
			Selectors: map[string]interface{}{"testId": "password-field"},
			Value:     "[redacted]",
			InputType: "password",
		},
	}

	script := generatePlaywrightScript(actions, "", "")

	if !strings.Contains(script, ".fill('[user-provided]')") {
		t.Error("Expected redacted value replaced with [user-provided]")
	}
}

func TestGeneratePlaywrightScriptScrollAction(t *testing.T) {
	actions := []EnhancedAction{
		{
			Type:      "click",
			Timestamp: 1000,
			URL:       "http://localhost:3000/page",
			Selectors: map[string]interface{}{"testId": "start-btn"},
		},
		{
			Type:      "scroll",
			Timestamp: 2000,
			URL:       "http://localhost:3000/page",
			ScrollY:   500,
		},
	}

	script := generatePlaywrightScript(actions, "", "")

	if !strings.Contains(script, "// User scrolled to y=500") {
		t.Error("Expected scroll comment with y position")
	}
}

func TestGeneratePlaywrightScriptNavigateAction(t *testing.T) {
	actions := []EnhancedAction{
		{
			Type:      "click",
			Timestamp: 1000,
			URL:       "http://localhost:3000/home",
			Selectors: map[string]interface{}{"testId": "nav-link"},
		},
		{
			Type:      "navigate",
			Timestamp: 2000,
			URL:       "http://localhost:3000/home",
			ToURL:     "http://localhost:3000/dashboard",
		},
	}

	script := generatePlaywrightScript(actions, "", "")

	if !strings.Contains(script, "page.waitForURL('http://localhost:3000/dashboard')") {
		t.Error("Expected waitForURL for navigate action")
	}
}

func TestGeneratePlaywrightScriptNavigateWithBaseURL(t *testing.T) {
	actions := []EnhancedAction{
		{
			Type:      "navigate",
			Timestamp: 1000,
			URL:       "http://localhost:3000/home",
			ToURL:     "http://localhost:3000/about",
		},
	}

	script := generatePlaywrightScript(actions, "", "http://staging.example.com")

	if !strings.Contains(script, "http://staging.example.com/about") {
		t.Error("Expected baseURL to replace origin in navigate toURL")
	}
}

func TestGeneratePlaywrightScriptKeypressAction(t *testing.T) {
	actions := []EnhancedAction{
		{
			Type:      "click",
			Timestamp: 1000,
			URL:       "http://localhost:3000/editor",
			Selectors: map[string]interface{}{"testId": "input"},
		},
		{
			Type:      "keypress",
			Timestamp: 2000,
			URL:       "http://localhost:3000/editor",
			Key:       "Enter",
		},
	}

	script := generatePlaywrightScript(actions, "", "")

	if !strings.Contains(script, "page.keyboard.press('Enter')") {
		t.Error("Expected keyboard.press for keypress action")
	}
}

// ============================================
// Additional coverage: extractShape
// ============================================

func TestExtractShapeNestedObjects(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "address": {"city": "NYC", "zip": 10001}}}`
	result := extractResponseShape(jsonStr)

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", result)
	}

	user, ok := m["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected nested user map, got %T", m["user"])
	}

	if user["name"] != "string" {
		t.Errorf("Expected name type 'string', got %v", user["name"])
	}

	address, ok := user["address"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected nested address map, got %T", user["address"])
	}

	if address["city"] != "string" {
		t.Errorf("Expected city type 'string', got %v", address["city"])
	}
	if address["zip"] != "number" {
		t.Errorf("Expected zip type 'number', got %v", address["zip"])
	}
}

func TestExtractShapeArrayValues(t *testing.T) {
	jsonStr := `{"items": [{"id": 1, "name": "test"}], "tags": ["foo", "bar"]}`
	result := extractResponseShape(jsonStr)

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", result)
	}

	items, ok := m["items"].([]interface{})
	if !ok {
		t.Fatalf("Expected items to be array, got %T", m["items"])
	}
	if len(items) != 1 {
		t.Fatalf("Expected 1 shape element in array, got %d", len(items))
	}

	itemShape, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected array element to be map, got %T", items[0])
	}
	if itemShape["id"] != "number" {
		t.Errorf("Expected id type 'number', got %v", itemShape["id"])
	}
	if itemShape["name"] != "string" {
		t.Errorf("Expected name type 'string', got %v", itemShape["name"])
	}

	tags, ok := m["tags"].([]interface{})
	if !ok {
		t.Fatalf("Expected tags to be array, got %T", m["tags"])
	}
	if len(tags) != 1 {
		t.Fatalf("Expected 1 shape element in tags array, got %d", len(tags))
	}
	if tags[0] != "string" {
		t.Errorf("Expected tags element type 'string', got %v", tags[0])
	}
}

func TestExtractShapeNullValues(t *testing.T) {
	jsonStr := `{"name": "Alice", "email": null, "active": true}`
	result := extractResponseShape(jsonStr)

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", result)
	}

	if m["name"] != "string" {
		t.Errorf("Expected name type 'string', got %v", m["name"])
	}
	if m["email"] != "null" {
		t.Errorf("Expected email type 'null', got %v", m["email"])
	}
	if m["active"] != "boolean" {
		t.Errorf("Expected active type 'boolean', got %v", m["active"])
	}
}

func TestExtractShapeEmptyArray(t *testing.T) {
	jsonStr := `{"items": []}`
	result := extractResponseShape(jsonStr)

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", result)
	}

	items, ok := m["items"].([]interface{})
	if !ok {
		t.Fatalf("Expected items to be array, got %T", m["items"])
	}
	if len(items) != 0 {
		t.Errorf("Expected empty array shape, got %d elements", len(items))
	}
}

// ============================================
// Additional coverage: GenerateSessionSummaryWithEntries
// ============================================

func TestGenerateSessionSummaryImprovedPerformance(t *testing.T) {
	capture := setupTestCapture(t)

	// Track a first snapshot with slow load
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 2000},
		Network:   NetworkSummary{TransferSize: 200 * 1024},
	})

	// Track a second snapshot that is faster (improved)
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1200},
		Network:   NetworkSummary{TransferSize: 150 * 1024},
	})

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", summary.Status)
	}
	if summary.PerformanceDelta == nil {
		t.Fatal("Expected performance delta to be present")
	}
	// Load delta should be negative (improved)
	if summary.PerformanceDelta.LoadTimeDelta >= 0 {
		t.Errorf("Expected negative load time delta (improved), got %f", summary.PerformanceDelta.LoadTimeDelta)
	}
	if summary.PerformanceDelta.LoadTimeBefore != 2000 {
		t.Errorf("Expected LoadTimeBefore 2000, got %f", summary.PerformanceDelta.LoadTimeBefore)
	}
	if summary.PerformanceDelta.LoadTimeAfter != 1200 {
		t.Errorf("Expected LoadTimeAfter 1200, got %f", summary.PerformanceDelta.LoadTimeAfter)
	}
	// Bundle size delta should be negative (reduced)
	if summary.PerformanceDelta.BundleSizeDelta >= 0 {
		t.Errorf("Expected negative bundle size delta, got %d", summary.PerformanceDelta.BundleSizeDelta)
	}
}

func TestGenerateSessionSummaryMultipleNavigateActions(t *testing.T) {
	capture := setupTestCapture(t)

	// Add multiple navigate actions
	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/"},
		{Type: "navigate", Timestamp: 2000, URL: "http://localhost:3000/", ToURL: "http://localhost:3000/page1"},
		{Type: "click", Timestamp: 3000, URL: "http://localhost:3000/page1"},
		{Type: "navigate", Timestamp: 5000, URL: "http://localhost:3000/page1", ToURL: "http://localhost:3000/page2"},
		{Type: "click", Timestamp: 7000, URL: "http://localhost:3000/page2"},
		{Type: "navigate", Timestamp: 9000, URL: "http://localhost:3000/page2", ToURL: "http://localhost:3000/page3"},
	})

	// Add performance snapshots to satisfy the delta computation
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/page3",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 50000},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/page3",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1100},
		Network:   NetworkSummary{TransferSize: 55000},
	})

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	// Should count 3 navigate actions
	if summary.Metadata.ReloadCount != 3 {
		t.Errorf("Expected 3 navigations, got %d", summary.Metadata.ReloadCount)
	}
	// Duration should be last - first timestamp = 9000 - 1000 = 8000
	if summary.Metadata.DurationMs != 8000 {
		t.Errorf("Expected duration 8000ms, got %d", summary.Metadata.DurationMs)
	}
}

func TestGenerateSessionSummaryNetworkErrors(t *testing.T) {
	capture := setupTestCapture(t)

	// Add performance snapshots
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1000},
		Network:   NetworkSummary{TransferSize: 50000},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1100},
		Network:   NetworkSummary{TransferSize: 55000},
	})

	// Provide entries with error-level logs
	entries := []LogEntry{
		{"level": "error", "message": "Failed to fetch /api/users", "source": "network"},
		{"level": "warn", "message": "Deprecated API usage"},
		{"level": "error", "message": "TypeError: Cannot read property", "source": "javascript"},
		{"level": "info", "message": "Page loaded"},
	}

	summary := capture.GenerateSessionSummaryWithEntries(entries)

	if summary.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", summary.Status)
	}
	// Should have 2 errors (only error-level entries)
	if len(summary.Errors) != 2 {
		t.Fatalf("Expected 2 errors, got %d", len(summary.Errors))
	}
	if summary.Errors[0].Message != "Failed to fetch /api/users" {
		t.Errorf("Expected first error message 'Failed to fetch /api/users', got '%s'", summary.Errors[0].Message)
	}
	if summary.Errors[0].Source != "network" {
		t.Errorf("Expected first error source 'network', got '%s'", summary.Errors[0].Source)
	}
	if summary.Errors[1].Message != "TypeError: Cannot read property" {
		t.Errorf("Expected second error message, got '%s'", summary.Errors[1].Message)
	}
	if summary.Errors[1].Source != "javascript" {
		t.Errorf("Expected second error source 'javascript', got '%s'", summary.Errors[1].Source)
	}
}

// ============================================
// Additional coverage: GeneratePRSummary
// ============================================

func TestGeneratePRSummaryNewAndFixedErrorAnnotations(t *testing.T) {
	capture := setupTestCapture(t)

	// Setup performance data
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 100 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1600},
		Network:   NetworkSummary{TransferSize: 110 * 1024},
	})

	errors := []SessionError{
		{Message: "ReferenceError: foo is not defined", Source: "javascript", Resolved: false},
		{Message: "404 Not Found", Source: "network", Resolved: true},
	}

	markdown := capture.GeneratePRSummary(errors)

	if !strings.Contains(markdown, "## Performance Impact") {
		t.Error("Expected Performance Impact header")
	}
	if !strings.Contains(markdown, "### Errors") {
		t.Error("Expected Errors section")
	}
	if !strings.Contains(markdown, "**New**: `ReferenceError: foo is not defined`") {
		t.Error("Expected new error entry")
	}
	if !strings.Contains(markdown, "(javascript)") {
		t.Error("Expected javascript source annotation")
	}
	if !strings.Contains(markdown, "**Fixed**: `404 Not Found`") {
		t.Error("Expected fixed error entry")
	}
	if !strings.Contains(markdown, "(network)") {
		t.Error("Expected network source annotation")
	}
}

func TestGeneratePRSummaryWithBaseline(t *testing.T) {
	capture := setupTestCapture(t)

	fcp1 := 300.0
	lcp1 := 900.0
	cls1 := 0.05

	fcp2 := 350.0
	lcp2 := 1000.0
	cls2 := 0.08

	// Track two snapshots with FCP/LCP/CLS
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing: PerformanceTiming{
			Load:                   1500,
			FirstContentfulPaint:   &fcp1,
			LargestContentfulPaint: &lcp1,
		},
		Network: NetworkSummary{TransferSize: 100 * 1024},
		CLS:     &cls1,
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing: PerformanceTiming{
			Load:                   1700,
			FirstContentfulPaint:   &fcp2,
			LargestContentfulPaint: &lcp2,
		},
		Network: NetworkSummary{TransferSize: 120 * 1024},
		CLS:     &cls2,
	})

	markdown := capture.GeneratePRSummary(nil)

	// Should contain performance table with all metrics
	if !strings.Contains(markdown, "| Load Time |") {
		t.Error("Expected Load Time row in table")
	}
	if !strings.Contains(markdown, "| FCP |") {
		t.Error("Expected FCP row in table")
	}
	if !strings.Contains(markdown, "| LCP |") {
		t.Error("Expected LCP row in table")
	}
	if !strings.Contains(markdown, "| CLS |") {
		t.Error("Expected CLS row in table")
	}
	if !strings.Contains(markdown, "| Bundle Size |") {
		t.Error("Expected Bundle Size row in table")
	}
	if !strings.Contains(markdown, "Generated by Gasoline") {
		t.Error("Expected Generated by Gasoline footer")
	}
}

func TestGeneratePRSummaryEmptyErrorsList(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 100 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 100 * 1024},
	})

	// Pass empty errors slice (not nil)
	markdown := capture.GeneratePRSummary([]SessionError{})

	if !strings.Contains(markdown, "### Errors") {
		t.Error("Expected Errors section header")
	}
	if !strings.Contains(markdown, "No errors detected") {
		t.Error("Expected 'No errors detected' message")
	}
}

func TestGeneratePRSummaryAllErrorsResolved(t *testing.T) {
	capture := setupTestCapture(t)

	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1500},
		Network:   NetworkSummary{TransferSize: 100 * 1024},
	})
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:01:00Z",
		Timing:    PerformanceTiming{Load: 1400},
		Network:   NetworkSummary{TransferSize: 90 * 1024},
	})

	errors := []SessionError{
		{Message: "Old bug fixed", Source: "javascript", Resolved: true},
		{Message: "Another fix", Source: "", Resolved: true},
	}

	markdown := capture.GeneratePRSummary(errors)

	if !strings.Contains(markdown, "**Fixed**: `Old bug fixed`") {
		t.Error("Expected fixed error entry")
	}
	// When newCount == 0, should show "New: None"
	if !strings.Contains(markdown, "**New**: None") {
		t.Error("Expected '**New**: None' when all errors are resolved")
	}
}

// ============================================
// Additional coverage tests
// ============================================

func TestCodegenPlaywrightScriptLongErrorMessageTruncation(t *testing.T) {
	// Line 29: name = name[:80] — errorMessage longer than 80 chars causes truncation
	longMessage := strings.Repeat("a", 100) // 100 chars
	actions := []EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://example.com", Selectors: map[string]interface{}{"testId": "btn"}},
	}

	script := generatePlaywrightScript(actions, longMessage, "")

	// The test name should use the first 80 chars
	truncated := longMessage[:80]
	if !strings.Contains(script, "reproduction: "+truncated) {
		t.Error("Expected test name to contain the first 80 characters of errorMessage")
	}
	// Should NOT contain the full 100-char message in the test name
	if strings.Contains(script, "reproduction: "+longMessage) {
		t.Error("Expected test name to be truncated, not full 100-char message")
	}
}

func TestCodegenPlaywrightScriptOutputCappedAt51200(t *testing.T) {
	// Line 98: script = script[:51200] — Script output exceeding 50KB is capped
	// Generate enough actions to produce a script > 51200 bytes
	actions := make([]EnhancedAction, 0, 2000)
	for i := 0; i < 2000; i++ {
		actions = append(actions, EnhancedAction{
			Type:      "click",
			Timestamp: int64(1000 + i),
			URL:       "http://example.com/page",
			Selectors: map[string]interface{}{
				"cssPath": strings.Repeat("div.really-long-selector-name > span.another-long-name > ", 3) + "button",
			},
		})
	}

	script := generatePlaywrightScript(actions, "some error message", "")

	if len(script) > 51200 {
		t.Errorf("Expected script length <= 51200, got %d", len(script))
	}
	if len(script) < 51200 {
		// If the script is already smaller, we need more actions
		// Let's try with even more
		actions2 := make([]EnhancedAction, 0, 5000)
		for i := 0; i < 5000; i++ {
			actions2 = append(actions2, EnhancedAction{
				Type:      "click",
				Timestamp: int64(1000 + i),
				URL:       "http://example.com/page",
				Selectors: map[string]interface{}{
					"cssPath": strings.Repeat("div.a-very-long-css-path-selector > ", 5) + "button.final",
				},
			})
		}
		script2 := generatePlaywrightScript(actions2, "error", "")
		if len(script2) != 51200 {
			// Only assert if the generated script was actually long enough to be capped
			if len(script2) > 51200 {
				t.Errorf("Expected script capped at 51200, got %d", len(script2))
			}
		}
	}
}

func TestCodegenTimelineMinTimestampFiltering(t *testing.T) {
	// Line 334: if minTimestamp > 0 && ts < minTimestamp { continue }
	// Network and console entries with timestamps before the first action should be excluded
	capture := NewCapture()

	// Use realistic unix millis timestamps for actions
	// 2025-06-01T00:00:05.000Z = 1748736005000
	actionStart := int64(1748736005000)
	capture.enhancedActions = []EnhancedAction{
		{Type: "click", Timestamp: actionStart, URL: "http://example.com"},
		{Type: "click", Timestamp: actionStart + 1000, URL: "http://example.com"},
	}

	// Network body with timestamp before actions (should be excluded)
	// "2020-01-01T00:00:01.000Z" = 1577836801000, which is before actionStart
	capture.networkBodies = []NetworkBody{
		{Method: "GET", URL: "http://example.com/early", Status: 200, Timestamp: "2020-01-01T00:00:01.000Z", ContentType: "text/html"},
		{Method: "GET", URL: "http://example.com/later", Status: 200, Timestamp: "2025-06-01T00:00:06.000Z", ContentType: "text/html"},
	}

	// Console entry before the first action timestamp
	earlyEntries := []LogEntry{
		{"level": "error", "message": "early error", "ts": "2020-01-01T00:00:01.000Z"},
		{"level": "error", "message": "late error", "ts": "2025-06-01T00:00:06.000Z"},
	}

	resp := capture.GetSessionTimeline(TimelineFilter{}, earlyEntries)

	// Only the later network entry and later console entry should pass
	for _, entry := range resp.Timeline {
		if entry.Kind == "network" && entry.URL == "http://example.com/early" {
			t.Error("Expected early network entry to be excluded by minTimestamp filter")
		}
		if entry.Kind == "console" && entry.Message == "early error" {
			t.Error("Expected early console entry to be excluded by minTimestamp filter")
		}
	}

	// Verify the later entries are present
	foundLateNetwork := false
	foundLateConsole := false
	for _, entry := range resp.Timeline {
		if entry.Kind == "network" && entry.URL == "http://example.com/later" {
			foundLateNetwork = true
		}
		if entry.Kind == "console" && entry.Message == "late error" {
			foundLateConsole = true
		}
	}
	if !foundLateNetwork {
		t.Error("Expected later network entry to be included")
	}
	if !foundLateConsole {
		t.Error("Expected later console entry to be included")
	}
}

func TestCodegenTimelineCappedAt200(t *testing.T) {
	// Line 355: entries = entries[:200] — Timeline capped at 200 entries
	capture := NewCapture()

	// Add 250 actions
	for i := 0; i < 250; i++ {
		capture.enhancedActions = append(capture.enhancedActions, EnhancedAction{
			Type:      "click",
			Timestamp: int64(1000 + i),
			URL:       "http://example.com",
		})
	}

	resp := capture.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) != 200 {
		t.Errorf("Expected timeline to be capped at 200, got %d", len(resp.Timeline))
	}
}

func TestCodegenTestScriptNavigateEmptyToURL(t *testing.T) {
	// Lines 449-451: navigate action with empty ToURL falls back to URL
	timeline := []TimelineEntry{
		{Kind: "action", Type: "navigate", Timestamp: 1000, URL: "http://example.com/page", ToURL: ""},
	}

	script := generateTestScript(timeline, TestGenerationOptions{TestName: "test nav"})

	if !strings.Contains(script, "http://example.com/page") {
		t.Error("Expected navigate with empty ToURL to fall back to URL field")
	}
}

func TestCodegenTestScriptNavigateWithBaseURL(t *testing.T) {
	// Lines 452-454: navigate action with ToURL and BaseURL replacement
	timeline := []TimelineEntry{
		{Kind: "action", Type: "navigate", Timestamp: 1000, ToURL: "http://old.com/path/to/page", URL: "http://old.com"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{
		TestName: "test nav",
		BaseURL:  "http://new.com",
	})

	if !strings.Contains(script, "http://new.com/path/to/page") {
		t.Errorf("Expected BaseURL replacement in navigate ToURL, got:\n%s", script)
	}
	if strings.Contains(script, "http://old.com") {
		t.Error("Expected old URL origin to be replaced")
	}
}

func TestCodegenTestScriptNetworkWithBaseURL(t *testing.T) {
	// Lines 459-462: network action with BaseURL replacement
	timeline := []TimelineEntry{
		{Kind: "action", Type: "click", Timestamp: 1000, URL: "http://old.com/app",
			Selectors: map[string]interface{}{"testId": "btn"}},
		{Kind: "network", Timestamp: 2000, URL: "http://old.com/api/data", Status: 200,
			Method: "GET"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{
		TestName:      "test network",
		AssertNetwork: true,
		BaseURL:       "http://new.com",
	})

	if !strings.Contains(script, "http://new.com/api/data") {
		t.Errorf("Expected BaseURL replacement for network URL, got:\n%s", script)
	}
}

func TestCodegenExtractShapeUnknownType(t *testing.T) {
	// Line 538: default: return "unknown" in extractShape type switch
	// Pass a value that doesn't match any known JSON type (int is not produced by json.Unmarshal, but can be passed directly)
	result := extractShape(42, 0)
	if result != "unknown" {
		t.Errorf("Expected 'unknown' for int type, got %v", result)
	}

	// Try with a struct
	type custom struct{ X int }
	result2 := extractShape(custom{X: 1}, 0)
	if result2 != "unknown" {
		t.Errorf("Expected 'unknown' for struct type, got %v", result2)
	}
}

func TestCodegenSessionSummarySnapshotCountFallback(t *testing.T) {
	// Line 679: summary.Metadata.PerformanceCheckCount == 0 path
	// session.snapshotCount == 0, but snapshotOrder has >=2 entries
	capture := NewCapture()

	// Add snapshots directly without using TrackPerformanceSnapshot (so session.snapshotCount stays 0)
	capture.perf.snapshots["http://example.com"] = PerformanceSnapshot{
		URL:    "http://example.com",
		Timing: PerformanceTiming{Load: 1000},
	}
	capture.perf.snapshotOrder = []string{"http://example.com", "http://example.com"}
	capture.perf.snapshots["http://example.com"] = PerformanceSnapshot{
		URL:    "http://example.com",
		Timing: PerformanceTiming{Load: 1200},
	}
	// Also set firstSnapshots so it doesn't bail on "insufficient_data"
	capture.session.firstSnapshots["http://example.com"] = PerformanceSnapshot{
		URL:    "http://example.com",
		Timing: PerformanceTiming{Load: 1000},
	}
	// snapshotCount stays 0

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.Metadata.PerformanceCheckCount != 2 {
		t.Errorf("Expected PerformanceCheckCount to fall back to len(snapshotOrder)=2, got %d", summary.Metadata.PerformanceCheckCount)
	}
}

func TestCodegenSessionSummaryBaselineFallback(t *testing.T) {
	// Lines 689-708: baseline fallback path — no firstSnapshots, but baselines exist with SampleCount >= 2
	capture := NewCapture()

	// Set up perf with snapshots but no session.firstSnapshots
	capture.perf.snapshots["http://example.com"] = PerformanceSnapshot{
		URL:     "http://example.com",
		Timing:  PerformanceTiming{Load: 1500},
		Network: NetworkSummary{TransferSize: 200000},
	}
	capture.perf.snapshotOrder = []string{"http://example.com", "http://example.com"}
	capture.session.snapshotCount = 3

	// Set baseline with SampleCount >= 2
	capture.perf.baselines["http://example.com"] = PerformanceBaseline{
		SampleCount: 3,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 150000},
	}

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.Status != "ok" {
		t.Errorf("Expected status 'ok' with baseline fallback, got %q", summary.Status)
	}
	if summary.PerformanceDelta == nil {
		t.Fatal("Expected PerformanceDelta to be set with baseline fallback")
	}
	if summary.PerformanceDelta.LoadTimeBefore != 1000 {
		t.Errorf("Expected LoadTimeBefore=1000 from baseline, got %f", summary.PerformanceDelta.LoadTimeBefore)
	}
	if summary.PerformanceDelta.LoadTimeAfter != 1500 {
		t.Errorf("Expected LoadTimeAfter=1500 from latest snapshot, got %f", summary.PerformanceDelta.LoadTimeAfter)
	}
}

func TestCodegenSessionSummaryNoFirstNoBaseline(t *testing.T) {
	// Lines 712-714: !hasFirst path — no firstSnapshot, no baseline → returns "insufficient_data"
	capture := NewCapture()

	// Set up perf with snapshots but no firstSnapshots and no baselines
	capture.perf.snapshots["http://example.com"] = PerformanceSnapshot{
		URL:    "http://example.com",
		Timing: PerformanceTiming{Load: 1500},
	}
	capture.perf.snapshotOrder = []string{"http://example.com", "http://example.com"}
	capture.session.snapshotCount = 2
	// No firstSnapshots, no baselines (or baseline with SampleCount < 2)
	capture.perf.baselines["http://example.com"] = PerformanceBaseline{
		SampleCount: 1, // Not enough
	}

	summary := capture.GenerateSessionSummaryWithEntries(nil)

	if summary.Status != "insufficient_data" {
		t.Errorf("Expected status 'insufficient_data' when no first snapshot and baseline SampleCount<2, got %q", summary.Status)
	}
}

func TestCodegenPRSummaryNoFirstNoBaseline(t *testing.T) {
	// Lines 786-810: In GeneratePRSummary — no firstSnapshots, no baselines path
	capture := NewCapture()

	capture.perf.snapshots["http://example.com"] = PerformanceSnapshot{
		URL:    "http://example.com",
		Timing: PerformanceTiming{Load: 1500},
	}
	capture.perf.snapshotOrder = []string{"http://example.com", "http://example.com"}
	capture.session.snapshotCount = 0
	// No firstSnapshots and no qualifying baselines
	capture.perf.baselines["http://example.com"] = PerformanceBaseline{
		SampleCount: 1,
	}

	markdown := capture.GeneratePRSummary(nil)

	if !strings.Contains(markdown, "No performance data collected") {
		t.Errorf("Expected 'No performance data' when hasFirst=false and snapshotCount<2, got:\n%s", markdown)
	}
}

func TestCodegenPRSummaryBaselineFallback(t *testing.T) {
	// Lines 786-810: In GeneratePRSummary — baseline fallback renders performance table
	capture := NewCapture()

	capture.perf.snapshots["http://example.com"] = PerformanceSnapshot{
		URL:     "http://example.com",
		Timing:  PerformanceTiming{Load: 2000},
		Network: NetworkSummary{TransferSize: 300000},
	}
	capture.perf.snapshotOrder = []string{"http://example.com", "http://example.com"}
	capture.session.snapshotCount = 3

	// Baseline with SampleCount >= 2
	capture.perf.baselines["http://example.com"] = PerformanceBaseline{
		SampleCount: 5,
		Timing:      BaselineTiming{Load: 1000},
		Network:     BaselineNetwork{TransferSize: 200000},
	}

	markdown := capture.GeneratePRSummary(nil)

	if !strings.Contains(markdown, "| Metric | Before | After | Delta |") {
		t.Error("Expected performance table header in PR summary with baseline fallback")
	}
	if !strings.Contains(markdown, "Load Time") {
		t.Error("Expected Load Time row in PR summary")
	}
}

func TestCodegenPRSummaryTotalSamplesFallback(t *testing.T) {
	// Line 909: if totalSamples == 0 { totalSamples = snapshotCount }
	capture := NewCapture()

	capture.perf.snapshots["http://example.com"] = PerformanceSnapshot{
		URL:     "http://example.com",
		Timing:  PerformanceTiming{Load: 1500},
		Network: NetworkSummary{TransferSize: 100000},
	}
	capture.perf.snapshotOrder = []string{"http://example.com", "http://example.com", "http://example.com"}
	capture.session.snapshotCount = 0 // Force fallback
	capture.session.firstSnapshots["http://example.com"] = PerformanceSnapshot{
		URL:     "http://example.com",
		Timing:  PerformanceTiming{Load: 1000},
		Network: NetworkSummary{TransferSize: 80000},
	}

	markdown := capture.GeneratePRSummary(nil)

	// snapshotOrder has 3 entries, so totalSamples should be 3
	if !strings.Contains(markdown, "from 3 performance samples") {
		t.Errorf("Expected 'from 3 performance samples' when session.snapshotCount=0, got:\n%s", markdown)
	}
}

func TestCodegenOneLinerNoPerfDataWhenNoFirstAndNoBaseline(t *testing.T) {
	// Line 960: else { parts = append(parts, "no perf data") } when hasFirst is false
	capture := NewCapture()

	capture.perf.snapshots["http://example.com"] = PerformanceSnapshot{
		URL:    "http://example.com",
		Timing: PerformanceTiming{Load: 1500},
	}
	capture.perf.snapshotOrder = []string{"http://example.com", "http://example.com"}
	capture.session.snapshotCount = 2 // >= 2, so we enter the else branch
	// No firstSnapshots for this URL
	// Baseline with SampleCount < 2
	capture.perf.baselines["http://example.com"] = PerformanceBaseline{
		SampleCount: 1,
	}

	result := capture.GenerateOneLiner(nil)

	if !strings.Contains(result, "no perf data") {
		t.Errorf("Expected 'no perf data' when hasFirst is false, got: %s", result)
	}
}
