package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/queries"
)

func TestMaybeWaitForCommand_SyncByDefault(t *testing.T) {
	// Setup
	cap := capture.NewCapture()
	handler := &ToolHandler{capture: cap}
	req := JSONRPCRequest{ID: 1, ClientID: "test-client"}
	correlationID := "test-sync-123"

	// Mock extension connection manually to avoid nil pointer in HandleSync
	// (Internal knowledge: IsExtensionConnected checks lastSyncSeen)
	// We'll use the proper way to simulate connection: a Sync request.
	
	// Create a result after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cap.CompleteCommand(correlationID, json.RawMessage(`{"success":true,"message":"instant result"}`), "")
	}()

	// Simulate extension connection via a Sync call so IsExtensionConnected() returns true
	reqBody := `{"session_id":"test"}`
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(reqBody))
	httpReq.Header.Set("X-Gasoline-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)

	// Call with no explicit sync param (should default to true)
	resp := handler.maybeWaitForCommand(req, correlationID, json.RawMessage(`{}`), "Queued")

	// Verify
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["correlation_id"] != correlationID {
		t.Errorf("Expected correlation_id %s, got %v", correlationID, result["correlation_id"])
	}
	if result["status"] != "complete" {
		t.Errorf("Expected status complete (sync), got %v", result["status"])
	}
}

func TestMaybeWaitForCommand_BackgroundOverride(t *testing.T) {
	cap := capture.NewCapture()
	handler := &ToolHandler{capture: cap}
	req := JSONRPCRequest{ID: 1}
	correlationID := "test-bg-123"

	// Call with background: true
	resp := handler.maybeWaitForCommand(req, correlationID, json.RawMessage(`{"background":true}`), "Queued")

	var result map[string]any
	_ = json.Unmarshal(resp.Result, &result)

	if result["status"] != "queued" {
		t.Errorf("Expected status queued (background), got %v", result["status"])
	}
}

func TestMaybeWaitForCommand_TimeoutGracefulFallback(t *testing.T) {
	cap := capture.NewCapture()
	handler := &ToolHandler{capture: cap}
	req := JSONRPCRequest{ID: 1}
	correlationID := "test-timeout-123"

	// Note: We'd need to mock the 15s timeout to be shorter for testing, 
	// or just verify it returns "pending" if we don't complete it.
	// For this unit test, we'll assume the 15s wait happens.
	
	// Start a goroutine to check if it's blocking
	start := time.Now()
	_ = handler.maybeWaitForCommand(req, correlationID, json.RawMessage(`{"sync":true}`), "Queued")
	duration := time.Since(start)

	// Since we didn't mock connection or result, it should fail fast or timeout.
	// Current impl fails fast if extension not connected.
	if duration > 1*time.Second {
		t.Errorf("Should have failed fast since extension is not connected, took %v", duration)
	}
}

// parseMCPResponseData extracts the JSON data map from an MCP tool response.
func parseMCPResponseData(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var mcpResult MCPToolResult
	if err := json.Unmarshal(raw, &mcpResult); err != nil {
		t.Fatalf("Failed to parse MCPToolResult: %v", err)
	}
	if len(mcpResult.Content) == 0 {
		t.Fatal("MCPToolResult has no content blocks")
	}
	jsonStr := extractJSONFromMCPText(mcpResult.Content[0].Text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse embedded JSON: %v\nText: %s", err, jsonStr)
	}
	return data
}

func TestFormatCommandResult_FinalField(t *testing.T) {
	cap := capture.NewCapture()
	server, err := NewServer("/tmp/test-final-field.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	tests := []struct {
		name      string
		cmd       queries.CommandResult
		wantFinal bool
		wantError bool // true if response uses mcpStructuredError (no final field)
	}{
		{
			name: "complete has final=true",
			cmd: queries.CommandResult{
				CorrelationID: "test-1",
				Status:        "complete",
				Result:        json.RawMessage(`{"success":true}`),
				CreatedAt:     time.Now(),
			},
			wantFinal: true,
		},
		{
			name: "error has final=true",
			cmd: queries.CommandResult{
				CorrelationID: "test-2",
				Status:        "error",
				Error:         "element_not_found",
				CreatedAt:     time.Now(),
			},
			wantFinal: true,
		},
		{
			name: "pending has final=false",
			cmd: queries.CommandResult{
				CorrelationID: "test-3",
				Status:        "pending",
				CreatedAt:     time.Now(),
			},
			wantFinal: false,
		},
		{
			name: "expired uses mcpStructuredError",
			cmd: queries.CommandResult{
				CorrelationID: "test-4",
				Status:        "expired",
				Error:         "timed out",
				CreatedAt:     time.Now(),
			},
			wantError: true,
		},
		{
			name: "timeout uses mcpStructuredError",
			cmd: queries.CommandResult{
				CorrelationID: "test-5",
				Status:        "timeout",
				Error:         "no response",
				CreatedAt:     time.Now(),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := handler.formatCommandResult(req, tt.cmd, tt.cmd.CorrelationID)

			var mcpResult MCPToolResult
			if err := json.Unmarshal(resp.Result, &mcpResult); err != nil {
				t.Fatalf("Failed to parse MCPToolResult: %v", err)
			}

			if tt.wantError {
				if !mcpResult.IsError {
					t.Errorf("Expected isError=true for %s status", tt.cmd.Status)
				}
				return
			}

			data := parseMCPResponseData(t, resp.Result)

			finalVal, ok := data["final"].(bool)
			if !ok {
				t.Fatalf("Expected final field to be bool, got %T (%v)", data["final"], data["final"])
			}
			if finalVal != tt.wantFinal {
				t.Errorf("Expected final=%v for %s status, got %v", tt.wantFinal, tt.cmd.Status, finalVal)
			}
		})
	}
}
