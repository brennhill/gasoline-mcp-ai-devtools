// bridge_unit_test.go — Unit tests for bridge helper functions.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// NOTE: Tests that redirect os.Stdout cannot use t.Parallel().

func TestSendBridgeError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	sendBridgeError(42, -32603, "test error message")

	os.Stdout = origStdout
	_ = w.Close()

	output, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("failed to read pipe: %v", readErr)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("sendBridgeError output not valid JSON: %v; got: %q", err, string(output))
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got %q", resp.JSONRPC)
	}
	if resp.Error == nil {
		t.Fatal("expected error field to be set")
	}
	if resp.Error.Code != -32603 {
		t.Fatalf("expected code=-32603, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "test error message" {
		t.Fatalf("expected message=%q, got %q", "test error message", resp.Error.Message)
	}
}

func TestSendToolError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	sendToolError("req-1", "Server is starting up. Please retry.")

	os.Stdout = origStdout
	_ = w.Close()

	output, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("failed to read pipe: %v", readErr)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("sendToolError output not valid JSON: %v; got: %q", err, string(output))
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got %q", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Fatal("sendToolError should not set JSON-RPC error field (it's a soft error)")
	}
	if resp.Result == nil {
		t.Fatal("expected result field to be set")
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result["isError"] != true {
		t.Fatalf("expected isError=true, got %v", result["isError"])
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected content array, got %v", result["content"])
	}
	firstItem := content[0].(map[string]any)
	if firstItem["text"] != "Server is starting up. Please retry." {
		t.Fatalf("unexpected text: %v", firstItem["text"])
	}
}

// TestBridgeForwardRequest_LargeBodyRead verifies that bridgeForwardRequest
// reads the full response body even when the server writes it with a delay.
// This is a regression test for a bug where bridgeDoHTTP created a context
// with defer cancel(), which canceled before the caller read resp.Body.
func TestBridgeForwardRequest_LargeBodyRead(t *testing.T) {
	// Build a large JSON-RPC response (~20KB) to exceed any internal buffering
	largePayload := strings.Repeat(`"key":"value",`, 1000)
	rpcResponse := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{%s\"done\":true}"}],"isError":false}}`, largePayload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow write: headers first, then delayed body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(rpcResponse))
	}))
	defer srv.Close()

	// Capture stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	var wg sync.WaitGroup
	wg.Add(1)
	signalCalled := false
	signal := func() {
		signalCalled = true
		wg.Done()
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	line := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"configure","arguments":{"action":"health"}}}`)

	go func() {
		bridgeForwardRequest(&http.Client{}, srv.URL, req, line, 5*time.Second, nil, signal)
	}()

	wg.Wait()
	os.Stdout = origStdout
	_ = w.Close()

	output, _ := io.ReadAll(r)
	_ = r.Close()

	if !signalCalled {
		t.Fatal("signal was never called")
	}

	// Verify we got the full response, not a "Failed to read response: context canceled" error
	outputStr := string(output)
	if strings.Contains(outputStr, "context canceled") {
		t.Fatalf("got context canceled error — body read failed: %s", outputStr)
	}
	if strings.Contains(outputStr, "Failed to read response") {
		t.Fatalf("got body read error: %s", outputStr)
	}
	// "done":true is inside a JSON text field, so it appears escaped as \"done\":true
	if !strings.Contains(outputStr, `done`) {
		t.Fatalf("response body was truncated or missing: %s", outputStr[:min(len(outputStr), 200)])
	}
	if len(outputStr) < len(rpcResponse)/2 {
		t.Fatalf("response body appears truncated: got %d bytes, expected ~%d", len(outputStr), len(rpcResponse))
	}
}

// TestToolCallTimeout verifies the per-request timeout branching logic.
func TestToolCallTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		params   string
		expected time.Duration
	}{
		{"ping gets fast timeout", "ping", `{}`, 10 * time.Second},
		{"resources/read gets fast timeout", "resources/read", `{}`, 10 * time.Second},
		{"tools/list gets fast timeout", "tools/list", `{}`, 10 * time.Second},
		{"observe gets fast timeout", "tools/call", `{"name":"observe","arguments":{"what":"logs"}}`, 10 * time.Second},
		{"configure gets fast timeout", "tools/call", `{"name":"configure","arguments":{"action":"health"}}`, 10 * time.Second},
		{"generate gets fast timeout", "tools/call", `{"name":"generate","arguments":{"format":"reproduction"}}`, 10 * time.Second},
		{"analyze gets slow timeout", "tools/call", `{"name":"analyze","arguments":{"what":"dom"}}`, 35 * time.Second},
		{"interact gets slow timeout", "tools/call", `{"name":"interact","arguments":{"action":"click"}}`, 35 * time.Second},
		{"observe screenshot gets slow timeout", "tools/call", `{"name":"observe","arguments":{"what":"screenshot"}}`, 35 * time.Second},
		{"observe command_result non-annotation gets fast", "tools/call", `{"name":"observe","arguments":{"what":"command_result","correlation_id":"cmd_123"}}`, 10 * time.Second},
		{"observe command_result annotation gets blocking poll", "tools/call", `{"name":"observe","arguments":{"what":"command_result","correlation_id":"ann_detail_abc"}}`, 65 * time.Second},
		{"malformed params gets fast timeout", "tools/call", `{bad json}`, 10 * time.Second},
		{"unknown tool gets fast timeout", "tools/call", `{"name":"unknown_tool","arguments":{}}`, 10 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`1`),
				Method:  tc.method,
				Params:  json.RawMessage(tc.params),
			}
			got := toolCallTimeout(req)
			if got != tc.expected {
				t.Errorf("toolCallTimeout(%s, %s) = %v, want %v", tc.method, tc.params, got, tc.expected)
			}
		})
	}
}

