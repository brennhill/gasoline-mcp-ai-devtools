// bridge_unit_test.go — Unit tests for bridge helper functions.
package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

// NOTE: These tests cannot use t.Parallel() because they redirect os.Stdout.

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
