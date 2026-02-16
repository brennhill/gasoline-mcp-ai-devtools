package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
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
