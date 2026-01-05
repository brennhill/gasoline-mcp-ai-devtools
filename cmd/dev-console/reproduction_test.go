// reproduction_test.go — Tests for reproduction script enhancements.
// Tests screenshot insertion, fixture generation, and visual assertions.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// Screenshot Insertion Tests
// ============================================

func TestReproductionScriptWithScreenshots(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "login-btn"}},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "email"}, Value: "user@test.com"},
		{Type: "click", Timestamp: 3000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "submit-btn"}},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","include_screenshots":true}}`),
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

	// Should contain screenshot after navigation
	if !strings.Contains(script, "await page.screenshot(") {
		t.Error("Expected screenshot call in script when include_screenshots=true")
	}

	// Should have screenshot after goto
	if !strings.Contains(script, "page.goto") || !strings.Contains(script, "step-1") {
		t.Errorf("Expected screenshot step-1 after navigation, got:\n%s", script)
	}

	// Should have multiple screenshots for significant actions
	screenshotCount := strings.Count(script, "page.screenshot(")
	if screenshotCount < 2 {
		t.Errorf("Expected at least 2 screenshot calls, got %d", screenshotCount)
	}
}

func TestReproductionScriptScreenshotsDefaultOff(t *testing.T) {
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

	// Without include_screenshots option
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

	// Should NOT contain screenshot by default
	if strings.Contains(script, "page.screenshot(") {
		t.Error("Expected no screenshot calls when include_screenshots is not specified")
	}
}

// ============================================
// Fixture Generation Tests
// ============================================

func TestReproductionScriptWithFixtures(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add actions and network data
	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/users", Selectors: map[string]interface{}{"testId": "load-users"}},
	})

	// Add network body with API response
	capture.AddNetworkBodies([]NetworkBody{{
		Timestamp:    "2024-01-01T00:00:01Z",
		Method:       "GET",
		URL:          "http://localhost:3000/api/users",
		Status:       200,
		ContentType:  "application/json",
		ResponseBody: `{"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]}`,
	}})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","generate_fixtures":true}}`),
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

	// Result should have script and fixtures
	if len(result.Content) < 2 {
		t.Fatalf("Expected at least 2 content blocks (script + fixtures), got %d", len(result.Content))
	}

	script := result.Content[0].Text
	fixtures := result.Content[1].Text

	// Script should reference fixtures
	if !strings.Contains(script, "fixtures") {
		t.Error("Expected script to reference fixtures")
	}

	// Script should contain page.route for mocking
	if !strings.Contains(script, "page.route") {
		t.Error("Expected page.route in script for API mocking")
	}

	// Fixtures should contain the API response data
	if !strings.Contains(fixtures, "users") {
		t.Error("Expected fixtures to contain 'users' key from API response")
	}

	// Fixtures should be valid JSON
	var fixtureData map[string]interface{}
	if err := json.Unmarshal([]byte(fixtures), &fixtureData); err != nil {
		t.Errorf("Expected fixtures to be valid JSON, got error: %v", err)
	}
}

func TestReproductionScriptFixturesDefaultOff(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/users", Selectors: map[string]interface{}{"testId": "btn"}},
	})

	capture.AddNetworkBodies([]NetworkBody{{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users",
		Status:       200,
		ContentType:  "application/json",
		ResponseBody: `{"users":[]}`,
	}})

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

	// Should NOT contain page.route or fixtures reference by default
	if strings.Contains(script, "page.route") {
		t.Error("Expected no page.route when generate_fixtures is not specified")
	}
}

// ============================================
// Visual Assertions Tests
// ============================================

func TestReproductionScriptWithVisualAssertions(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/checkout", Selectors: map[string]interface{}{"testId": "checkout-btn"}},
		{Type: "navigate", Timestamp: 2000, URL: "http://localhost:3000/checkout", ToURL: "http://localhost:3000/confirmation"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","visual_assertions":true}}`),
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

	// Should contain toHaveScreenshot assertions
	if !strings.Contains(script, "toHaveScreenshot(") {
		t.Error("Expected toHaveScreenshot assertion when visual_assertions=true")
	}

	// Should have meaningful snapshot names
	if !strings.Contains(script, ".png") {
		t.Error("Expected .png extension in screenshot names")
	}
}

func TestReproductionScriptVisualAssertionsDefaultOff(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/checkout", Selectors: map[string]interface{}{"testId": "btn"}},
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

	// Should NOT contain toHaveScreenshot by default
	if strings.Contains(script, "toHaveScreenshot") {
		t.Error("Expected no toHaveScreenshot when visual_assertions is not specified")
	}
}

// ============================================
// Combined Options Tests
// ============================================

func TestReproductionScriptCombinedOptions(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/users", Selectors: map[string]interface{}{"testId": "load-btn"}},
		{Type: "navigate", Timestamp: 2000, URL: "http://localhost:3000/users", ToURL: "http://localhost:3000/users/list"},
	})

	capture.AddNetworkBodies([]NetworkBody{{
		Timestamp:    "2024-01-01T00:00:01Z",
		Method:       "GET",
		URL:          "http://localhost:3000/api/users",
		Status:       200,
		ContentType:  "application/json",
		ResponseBody: `{"users":[{"id":1,"name":"Test"}]}`,
	}})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","include_screenshots":true,"generate_fixtures":true,"visual_assertions":true}}`),
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

	if len(result.Content) < 2 {
		t.Fatalf("Expected at least 2 content blocks, got %d", len(result.Content))
	}

	script := result.Content[0].Text

	// Should have screenshots
	if !strings.Contains(script, "page.screenshot(") {
		t.Error("Expected page.screenshot when include_screenshots=true")
	}

	// Should have visual assertions
	if !strings.Contains(script, "toHaveScreenshot(") {
		t.Error("Expected toHaveScreenshot when visual_assertions=true")
	}

	// Should have fixture mocking
	if !strings.Contains(script, "page.route") {
		t.Error("Expected page.route when generate_fixtures=true")
	}
}

// ============================================
// Fixture Content Tests
// ============================================

func TestFixtureGenerationMultipleEndpoints(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/app", Selectors: map[string]interface{}{"testId": "btn"}},
	})

	// Multiple API endpoints
	capture.AddNetworkBodies([]NetworkBody{
		{
			Timestamp:    "2024-01-01T00:00:01Z",
			Method:       "GET",
			URL:          "http://localhost:3000/api/users",
			Status:       200,
			ContentType:  "application/json",
			ResponseBody: `{"users":[{"id":1}]}`,
		},
		{
			Timestamp:    "2024-01-01T00:00:02Z",
			Method:       "GET",
			URL:          "http://localhost:3000/api/products",
			Status:       200,
			ContentType:  "application/json",
			ResponseBody: `{"products":[{"id":100,"name":"Widget"}]}`,
		},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","generate_fixtures":true}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) < 2 {
		t.Fatalf("Expected at least 2 content blocks, got %d", len(result.Content))
	}

	fixtures := result.Content[1].Text

	// Should contain both endpoints
	var fixtureData map[string]interface{}
	if err := json.Unmarshal([]byte(fixtures), &fixtureData); err != nil {
		t.Fatalf("Invalid fixture JSON: %v", err)
	}

	if len(fixtureData) < 2 {
		t.Errorf("Expected fixtures for 2 endpoints, got %d", len(fixtureData))
	}
}

func TestFixtureGenerationSkipsNonJSON(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/app", Selectors: map[string]interface{}{"testId": "btn"}},
	})

	// JSON response
	capture.AddNetworkBodies([]NetworkBody{
		{
			Method:       "GET",
			URL:          "http://localhost:3000/api/data",
			Status:       200,
			ContentType:  "application/json",
			ResponseBody: `{"key":"value"}`,
		},
		// Non-JSON response (should be skipped)
		{
			Method:       "GET",
			URL:          "http://localhost:3000/styles.css",
			Status:       200,
			ContentType:  "text/css",
			ResponseBody: "body { margin: 0; }",
		},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","generate_fixtures":true}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) < 2 {
		t.Fatalf("Expected at least 2 content blocks, got %d", len(result.Content))
	}

	fixtures := result.Content[1].Text

	var fixtureData map[string]interface{}
	if err := json.Unmarshal([]byte(fixtures), &fixtureData); err != nil {
		t.Fatalf("Invalid fixture JSON: %v", err)
	}

	// Should only have 1 fixture (the JSON one)
	if len(fixtureData) != 1 {
		t.Errorf("Expected 1 fixture (JSON only), got %d", len(fixtureData))
	}
}

// ============================================
// Screenshot Naming Tests
// ============================================

func TestScreenshotNaming(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "submit"}},
		{Type: "navigate", Timestamp: 2000, URL: "http://localhost:3000/login", ToURL: "http://localhost:3000/dashboard"},
		{Type: "click", Timestamp: 3000, URL: "http://localhost:3000/dashboard", Selectors: map[string]interface{}{"testId": "logout"}},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate","arguments":{"format":"reproduction","include_screenshots":true}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// Screenshots should have descriptive names
	if !strings.Contains(script, "step-") {
		t.Error("Expected step-based screenshot naming")
	}

	// Should include action type in name
	if !strings.Contains(script, "navigation") || !strings.Contains(script, "click") {
		t.Errorf("Expected action type in screenshot names, got:\n%s", script)
	}
}
