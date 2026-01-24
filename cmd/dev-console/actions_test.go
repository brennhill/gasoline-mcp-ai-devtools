package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestV5EnhancedActionsBuffer(t *testing.T) {
	capture := setupTestCapture(t)

	actions := []EnhancedAction{
		{
			Type:      "click",
			Timestamp: 1705312200000,
			URL:       "http://localhost:3000/login",
			Selectors: map[string]interface{}{
				"testId":  "login-btn",
				"cssPath": "form > button.primary",
			},
		},
		{
			Type:      "input",
			Timestamp: 1705312201000,
			URL:       "http://localhost:3000/login",
			Selectors: map[string]interface{}{
				"ariaLabel": "Email address",
				"cssPath":   "#email",
			},
			Value:     "user@example.com",
			InputType: "email",
		},
	}

	capture.AddEnhancedActions(actions)

	if capture.GetEnhancedActionCount() != 2 {
		t.Errorf("Expected 2 actions, got %d", capture.GetEnhancedActionCount())
	}
}

func TestV5EnhancedActionsBufferRotation(t *testing.T) {
	capture := setupTestCapture(t)

	// Add more than max (50) actions
	actions := make([]EnhancedAction, 60)
	for i := range actions {
		actions[i] = EnhancedAction{
			Type:      "click",
			Timestamp: int64(1705312200000 + i*1000),
			URL:       "http://localhost:3000/page",
			Selectors: map[string]interface{}{"cssPath": "button"},
		}
	}

	capture.AddEnhancedActions(actions)

	if capture.GetEnhancedActionCount() != 50 {
		t.Errorf("Expected 50 actions after rotation, got %d", capture.GetEnhancedActionCount())
	}
}

func TestV5EnhancedActionsGetAll(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/a"},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/b"},
		{Type: "navigate", Timestamp: 3000, URL: "http://localhost:3000/c"},
	})

	actions := capture.GetEnhancedActions(EnhancedActionFilter{})
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions))
	}
}

func TestV5EnhancedActionsFilterByLastN(t *testing.T) {
	capture := setupTestCapture(t)

	for i := 0; i < 10; i++ {
		capture.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000"},
		})
	}

	actions := capture.GetEnhancedActions(EnhancedActionFilter{LastN: 3})
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions with lastN filter, got %d", len(actions))
	}

	// Should return the most recent 3
	if actions[0].Timestamp != 7000 {
		t.Errorf("Expected oldest of last 3 to be timestamp 7000, got %d", actions[0].Timestamp)
	}
}

func TestV5EnhancedActionsFilterByURL(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login"},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/dashboard"},
		{Type: "click", Timestamp: 3000, URL: "http://localhost:3000/login"},
	})

	actions := capture.GetEnhancedActions(EnhancedActionFilter{URLFilter: "login"})
	if len(actions) != 2 {
		t.Errorf("Expected 2 actions matching 'login', got %d", len(actions))
	}
}

func TestV5EnhancedActionsNewestLast(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000"},
		{Type: "input", Timestamp: 3000, URL: "http://localhost:3000"},
		{Type: "click", Timestamp: 2000, URL: "http://localhost:3000"},
	})

	actions := capture.GetEnhancedActions(EnhancedActionFilter{})
	// Actions should preserve insertion order (chronological from extension)
	if actions[0].Timestamp != 1000 || actions[2].Timestamp != 2000 {
		t.Error("Expected actions in insertion order")
	}
}

func TestV5EnhancedActionsPasswordRedaction(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "input", Timestamp: 1000, URL: "http://localhost:3000", InputType: "password", Value: "secret123"},
	})

	actions := capture.GetEnhancedActions(EnhancedActionFilter{})
	// Server should preserve what extension sent (extension already redacts)
	// But server should also redact if inputType is password
	if actions[0].Value != "[redacted]" {
		t.Errorf("Expected password value to be redacted, got %s", actions[0].Value)
	}
}

func TestV5PostEnhancedActionsEndpoint(t *testing.T) {
	capture := setupTestCapture(t)

	body := `{"actions":[{"type":"click","timestamp":1705312200000,"url":"http://localhost:3000/login","selectors":{"testId":"login-btn","cssPath":"button.primary"}}]}`
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleEnhancedActions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if capture.GetEnhancedActionCount() != 1 {
		t.Errorf("Expected 1 action stored, got %d", capture.GetEnhancedActionCount())
	}
}

func TestV5PostEnhancedActionsMultiple(t *testing.T) {
	capture := setupTestCapture(t)

	body := `{"actions":[
		{"type":"click","timestamp":1000,"url":"http://localhost:3000","selectors":{"cssPath":"button"}},
		{"type":"input","timestamp":2000,"url":"http://localhost:3000","selectors":{"ariaLabel":"Email"},"value":"test@example.com","inputType":"email"},
		{"type":"keypress","timestamp":3000,"url":"http://localhost:3000","key":"Enter"}
	]}`
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleEnhancedActions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if capture.GetEnhancedActionCount() != 3 {
		t.Errorf("Expected 3 actions stored, got %d", capture.GetEnhancedActionCount())
	}
}

func TestV5PostEnhancedActionsInvalidJSON(t *testing.T) {
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleEnhancedActions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestV5PostEnhancedActionsPasswordRedaction(t *testing.T) {
	capture := setupTestCapture(t)

	body := `{"actions":[{"type":"input","timestamp":1000,"url":"http://localhost:3000","selectors":{},"inputType":"password","value":"mysecret"}]}`
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleEnhancedActions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	actions := capture.GetEnhancedActions(EnhancedActionFilter{})
	if actions[0].Value != "[redacted]" {
		t.Errorf("Expected password to be redacted on ingest, got %s", actions[0].Value)
	}
}

func TestMCPGetEnhancedActions(t *testing.T) {
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
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"actions"}}`),
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

	var actions []EnhancedAction
	if err := json.Unmarshal([]byte(result.Content[0].Text), &actions); err != nil {
		t.Fatalf("Expected valid JSON actions, got error: %v", err)
	}

	if len(actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(actions))
	}
}

func TestMCPGetEnhancedActionsWithLastN(t *testing.T) {
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
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"actions","last_n":5}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var actions []EnhancedAction
	json.Unmarshal([]byte(result.Content[0].Text), &actions)

	if len(actions) != 5 {
		t.Errorf("Expected 5 actions with last_n filter, got %d", len(actions))
	}
}

func TestMCPGetEnhancedActionsEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"actions"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if result.Content[0].Text != "No enhanced actions captured" {
		t.Errorf("Expected empty message, got: %s", result.Content[0].Text)
	}
}
