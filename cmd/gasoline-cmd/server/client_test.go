// client_test.go — Tests for JSON-RPC client and server lifecycle.
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallToolSuccess(t *testing.T) {
	t.Parallel()

	// Mock MCP server that returns a successful tool result
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}

		if req.Method != "tools/call" {
			t.Errorf("expected method 'tools/call', got %q", req.Method)
		}

		result := MCPToolResult{
			Content: []MCPContentBlock{
				{Type: "text", Text: `{"entries":[],"total":0}`},
			},
		}
		resultJSON, _ := json.Marshal(result)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Extract port from test server URL
	client := NewClient(srv.URL)

	args := map[string]any{
		"what":  "logs",
		"limit": 10,
	}

	result, err := client.CallTool("observe", args)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if result.IsError {
		t.Error("expected success, got error result")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if !strings.Contains(result.Content[0].Text, "entries") {
		t.Errorf("expected entries in response, got: %s", result.Content[0].Text)
	}
}

func TestCallToolServerError(t *testing.T) {
	t.Parallel()

	// Mock MCP server that returns a JSON-RPC error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params: missing 'what' parameter",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)

	_, err := client.CallTool("observe", nil)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
	if !strings.Contains(err.Error(), "Invalid params") {
		t.Errorf("expected error message about invalid params, got: %v", err)
	}
}

func TestCallToolNetworkError(t *testing.T) {
	t.Parallel()

	// Client pointing to a server that doesn't exist
	client := NewClient("http://127.0.0.1:19999")

	_, err := client.CallTool("observe", map[string]any{"what": "logs"})
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestCallToolMultipleContentBlocks(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		result := MCPToolResult{
			Content: []MCPContentBlock{
				{Type: "text", Text: "First block"},
				{Type: "text", Text: "Second block"},
			},
		}
		resultJSON, _ := json.Marshal(result)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.CallTool("observe", map[string]any{"what": "logs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
	}
}

func TestCallToolIsErrorFlag(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		result := MCPToolResult{
			Content: []MCPContentBlock{
				{Type: "text", Text: "Element not found"},
			},
			IsError: true,
		}
		resultJSON, _ := json.Marshal(result)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.CallTool("interact", map[string]any{
		"action":   "click",
		"selector": "#bad",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true")
	}
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	// Mock server with /health endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept both ping via MCP and health check
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}

		// MCP endpoint for ping
		var req JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "ping" {
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	ok := client.HealthCheck()
	if !ok {
		t.Error("expected health check to succeed")
	}
}

func TestHealthCheckFailed(t *testing.T) {
	t.Parallel()

	client := NewClient("http://127.0.0.1:19999")
	ok := client.HealthCheck()
	if ok {
		t.Error("expected health check to fail for unreachable server")
	}
}

func TestBuildRequestJSON(t *testing.T) {
	t.Parallel()

	client := NewClient("http://localhost:7890")
	args := map[string]any{
		"what":  "logs",
		"limit": 10,
	}

	body, err := client.buildRequest("observe", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %q", req.JSONRPC)
	}
	if req.Method != "tools/call" {
		t.Errorf("expected method tools/call, got %q", req.Method)
	}

	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("failed to parse params: %v", err)
	}
	if params.Name != "observe" {
		t.Errorf("expected tool name 'observe', got %q", params.Name)
	}
}

func TestInitialize(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Method == "initialize" {
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: json.RawMessage(`{
					"protocolVersion": "2024-11-05",
					"serverInfo": {"name": "gasoline", "version": "5.8.0"},
					"capabilities": {"tools": {}, "resources": {}}
				}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
}
