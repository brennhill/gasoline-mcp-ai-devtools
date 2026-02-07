package capture

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dev-console/dev-console/internal/queries"
)

func TestHandleSync_BasicRequest(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a sync request
	req := SyncRequest{
		SessionID: "test_session_123",
		Settings: &SyncSettings{
			PilotEnabled:    true,
			TrackingEnabled: false,
			TrackedTabID:    0,
			TrackedTabURL:   "",
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Call handler
	cap.HandleSync(w, httpReq)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response
	if !resp.Ack {
		t.Error("Expected Ack to be true")
	}
	if resp.NextPollMs != 1000 {
		t.Errorf("Expected NextPollMs to be 1000, got %d", resp.NextPollMs)
	}
	if resp.ServerTime == "" {
		t.Error("Expected ServerTime to be set")
	}

	// Verify state was updated
	cap.mu.RLock()
	if cap.extensionSession != "test_session_123" {
		t.Errorf("Expected session to be 'test_session_123', got '%s'", cap.extensionSession)
	}
	if !cap.pilotEnabled {
		t.Error("Expected pilotEnabled to be true")
	}
	cap.mu.RUnlock()
}

func TestHandleSync_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Try GET instead of POST
	httpReq := httptest.NewRequest("GET", "/sync", nil)
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleSync_InvalidJSON(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Send invalid JSON
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader([]byte("not json")))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSync_WithExtensionLogs(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create request with extension logs
	req := SyncRequest{
		SessionID: "test_session",
		ExtensionLogs: []ExtensionLog{
			{
				Level:    "info",
				Message:  "Test log message",
				Source:   "background",
				Category: "test",
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify logs were stored
	logs := cap.GetExtensionLogs()
	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}
	if logs[0].Message != "Test log message" {
		t.Errorf("Expected log message 'Test log message', got '%s'", logs[0].Message)
	}
}

func TestHandleSync_UpdatesLastPollAt(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Initially lastPollAt should be zero
	cap.mu.RLock()
	initialPollAt := cap.lastPollAt
	cap.mu.RUnlock()

	if !initialPollAt.IsZero() {
		t.Error("Expected initial lastPollAt to be zero")
	}

	// Send sync request
	req := SyncRequest{SessionID: "test"}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	// Verify lastPollAt was updated
	cap.mu.RLock()
	newPollAt := cap.lastPollAt
	cap.mu.RUnlock()

	if newPollAt.IsZero() {
		t.Error("Expected lastPollAt to be set after sync")
	}
}

// ============================================
// Waterfall On-Demand Tests via Sync
// ============================================

// TestHandleSync_WaterfallQueryDelivery verifies that waterfall queries
// are delivered to extension via sync response commands.
func TestHandleSync_WaterfallQueryDelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a waterfall query (simulating MCP requesting fresh data)
	queryID := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "waterfall",
		Params: json.RawMessage(`{}`),
	})
	if queryID == "" {
		t.Fatal("Failed to create waterfall query")
	}

	// Extension polls /sync and receives the command
	req := SyncRequest{SessionID: "test_session"}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Parse response and verify waterfall command is present
	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Commands) == 0 {
		t.Fatal("Expected at least one command in sync response")
	}

	found := false
	for _, cmd := range resp.Commands {
		if cmd.Type == "waterfall" && cmd.ID == queryID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Waterfall command not found in sync response. Commands: %v", resp.Commands)
	}
}

// TestHandleSync_WaterfallResultDelivery verifies that waterfall results
// are stored correctly when extension posts them via sync.
func TestHandleSync_WaterfallResultDelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a waterfall query
	queryID := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "waterfall",
		Params: json.RawMessage(`{}`),
	})

	// Simulate extension returning waterfall data via sync
	waterfallResult := map[string]any{
		"entries": []map[string]any{
			{"url": "https://api.example.com/users", "duration": 150.5},
		},
		"pageURL": "https://example.com",
	}
	resultBytes, _ := json.Marshal(waterfallResult)

	req := SyncRequest{
		SessionID: "test_session",
		CommandResults: []SyncCommandResult{
			{
				ID:     queryID,
				Status: "complete",
				Result: resultBytes,
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Verify result was stored
	result, found := cap.GetQueryResult(queryID)
	if !found {
		t.Fatal("Expected query result to be stored")
	}

	// Verify result content
	var storedResult map[string]any
	if err := json.Unmarshal(result, &storedResult); err != nil {
		t.Fatalf("Failed to unmarshal stored result: %v", err)
	}

	entries, ok := storedResult["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Errorf("Expected 1 entry in result, got: %v", storedResult)
	}
}

