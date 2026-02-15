// tools_generate_har_test.go — MCP integration tests for HAR export.
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/export"
	"github.com/dev-console/dev-console/internal/types"
)

func setupHARTestHandler(t *testing.T) *ToolHandler {
	t.Helper()
	srv, err := NewServer("/tmp/test-har-export.jsonl", 10)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(srv, cap)
	return mcpHandler.toolHandler.(*ToolHandler)
}

func TestToolExportHAR_ReturnsHARJSON(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)
	handler.capture.AddNetworkBodiesForTest([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200, Duration: 50},
	})

	args, _ := json.Marshal(map[string]any{})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}

	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var harLog export.HARLog
	if err := json.Unmarshal([]byte(jsonPart), &harLog); err != nil {
		t.Fatalf("response text is not valid HAR JSON: %v", err)
	}
	if len(harLog.Log.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(harLog.Log.Entries))
	}
}

func TestToolExportHAR_SaveToFile(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)
	handler.capture.AddNetworkBodiesForTest([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200},
	})

	tmpFile := os.TempDir() + "/test-tool-export.har"
	defer os.Remove(tmpFile)

	args, _ := json.Marshal(map[string]any{"save_to": tmpFile})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var summary export.HARExportResult
	if err := json.Unmarshal([]byte(jsonPart), &summary); err != nil {
		t.Fatalf("response text is not valid summary JSON: %v", err)
	}
	if summary.SavedTo != tmpFile {
		t.Errorf("expected saved_to %s, got %s", tmpFile, summary.SavedTo)
	}
	if summary.EntriesCount != 1 {
		t.Errorf("expected entries_count 1, got %d", summary.EntriesCount)
	}
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("expected file to exist")
	}
}

func TestToolExportHAR_Filters(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)
	handler.capture.AddNetworkBodiesForTest([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200},
		{Timestamp: "2026-01-23T10:30:01.000Z", Method: "POST", URL: "https://example.com/api", Status: 500},
		{Timestamp: "2026-01-23T10:30:02.000Z", Method: "GET", URL: "https://example.com/static", Status: 200},
	})

	args, _ := json.Marshal(map[string]any{"method": "POST", "status_min": 400})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var harLog export.HARLog
	json.Unmarshal([]byte(jsonPart), &harLog)

	if len(harLog.Log.Entries) != 1 {
		t.Fatalf("expected 1 entry with filters, got %d", len(harLog.Log.Entries))
	}
	if harLog.Log.Entries[0].Request.Method != "POST" {
		t.Errorf("expected POST, got %s", harLog.Log.Entries[0].Request.Method)
	}
}

func TestToolExportHAR_PathTraversal(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)

	args, _ := json.Marshal(map[string]any{"save_to": "../../etc/passwd"})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`5`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Error("expected error response for path traversal")
	}
}

func TestToolExportHAR_EmptyCapture(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)

	args, _ := json.Marshal(map[string]any{})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error for empty HAR export, got: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var harLog export.HARLog
	if err := json.Unmarshal([]byte(jsonPart), &harLog); err != nil {
		t.Fatalf("Expected valid HAR JSON, got parse error: %v", err)
	}
	if len(harLog.Log.Entries) != 0 {
		t.Errorf("Expected 0 entries in empty HAR, got %d", len(harLog.Log.Entries))
	}
}
