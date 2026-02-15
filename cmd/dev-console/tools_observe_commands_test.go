// tools_observe_commands_test.go — Coverage tests for toolObserveFailedCommands.
package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// ============================================
// toolObserveFailedCommands — 67% → 100%
// ============================================

func TestToolObserveFailedCommands_NoFailed(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	args := json.RawMessage(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolObserveFailedCommands(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("failed_commands should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
	count, _ := data["count"].(float64)
	if count != 0 {
		t.Fatalf("count = %v, want 0", count)
	}
}

func TestToolObserveFailedCommands_WithFailed(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	// Create a pending query and then expire it to create a failed command
	query := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"1+1"}`),
		CorrelationID: "test-fail-cmd",
	}
	env.capture.CreatePendingQueryWithTimeout(query, 1*time.Millisecond, "")
	// Wait for expiry
	time.Sleep(10 * time.Millisecond)
	// Trigger cleanup by calling ExpireCommand
	env.capture.ExpireCommand("test-fail-cmd")

	args := json.RawMessage(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolObserveFailedCommands(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("failed_commands should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	count, _ := data["count"].(float64)
	if count < 1 {
		t.Fatalf("count = %v, want >= 1", count)
	}
}
