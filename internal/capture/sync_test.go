package capture

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if cap.ext.extensionSession != "test_session_123" {
		t.Errorf("Expected session to be 'test_session_123', got '%s'", cap.ext.extensionSession)
	}
	if !cap.ext.pilotEnabled {
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

func TestHandleSync_WithExtensionLogs_RedactsSensitiveData(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	const (
		bearer = "Bearer tokenValue1234567890abcdef"
		awsKey = "AKIA1234567890ABCDEF"
	)

	req := SyncRequest{
		SessionID: "test_session",
		ExtensionLogs: []ExtensionLog{
			{
				Level:    "debug",
				Message:  "sync saw " + bearer,
				Source:   "background",
				Category: "AUTH",
				Data:     json.RawMessage(`{"aws":"` + awsKey + `","header":"` + bearer + `"}`),
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	logs := cap.GetExtensionLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	entry := logs[0]
	if strings.Contains(entry.Message, bearer) {
		t.Fatalf("Message should be redacted, got %q", entry.Message)
	}
	if !strings.Contains(entry.Message, "[REDACTED:bearer-token]") {
		t.Fatalf("Expected bearer token marker in message, got %q", entry.Message)
	}

	dataText := string(entry.Data)
	if strings.Contains(dataText, bearer) || strings.Contains(dataText, awsKey) {
		t.Fatalf("Expected redacted data, got %s", dataText)
	}
}

func TestHandleSync_UpdatesLastPollAt(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Initially lastPollAt should be zero
	cap.mu.RLock()
	initialPollAt := cap.ext.lastPollAt
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
	newPollAt := cap.ext.lastPollAt
	cap.mu.RUnlock()

	if newPollAt.IsZero() {
		t.Error("Expected lastPollAt to be set after sync")
	}
}

// ============================================
// Adaptive Polling Interval Tests
// ============================================

func TestHandleSync_AdaptivePoll_FastWhenPendingCommands(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a pending query so there are commands waiting
	cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	// Sync should return fast poll interval (200ms) since commands are pending
	req := SyncRequest{SessionID: "test_session"}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Commands) == 0 {
		t.Fatal("Expected at least one command in response")
	}
	if resp.NextPollMs != 200 {
		t.Errorf("Expected NextPollMs to be 200 when commands pending, got %d", resp.NextPollMs)
	}
}

func TestHandleSync_CommandsIncludeTabID(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	cap.CreatePendingQuery(queries.PendingQuery{
		Type:          "dom_action",
		Params:        json.RawMessage(`{"action":"click","selector":"#submit"}`),
		TabID:         42,
		CorrelationID: "corr-tab-42",
	})

	req := SyncRequest{SessionID: "test_session"}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(resp.Commands))
	}
	if resp.Commands[0].TabID != 42 {
		t.Fatalf("Expected command tab_id 42, got %d", resp.Commands[0].TabID)
	}
	if resp.Commands[0].CorrelationID != "corr-tab-42" {
		t.Fatalf("Expected correlation_id corr-tab-42, got %q", resp.Commands[0].CorrelationID)
	}
}

func TestHandleSync_AdaptivePoll_SlowWhenNoCommands(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// No pending queries — should get default 1000ms interval
	req := SyncRequest{SessionID: "test_session"}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()

	cap.HandleSync(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Commands) != 0 {
		t.Errorf("Expected no commands, got %d", len(resp.Commands))
	}
	if resp.NextPollMs != 1000 {
		t.Errorf("Expected NextPollMs to be 1000 when idle, got %d", resp.NextPollMs)
	}
}

func TestHandleSync_AdaptivePoll_RevertsAfterResultDelivered(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a pending query
	queryID := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	// First sync: should be fast (200ms) — commands pending
	req1 := SyncRequest{SessionID: "test_session"}
	body1, _ := json.Marshal(req1)
	httpReq1 := httptest.NewRequest("POST", "/sync", bytes.NewReader(body1))
	w1 := httptest.NewRecorder()
	cap.HandleSync(w1, httpReq1)

	var resp1 SyncResponse
	json.NewDecoder(w1.Body).Decode(&resp1)
	if resp1.NextPollMs != 200 {
		t.Errorf("First sync: expected NextPollMs 200, got %d", resp1.NextPollMs)
	}

	// Extension delivers result via second sync
	resultBytes, _ := json.Marshal(map[string]string{"html": "<body>test</body>"})
	req2 := SyncRequest{
		SessionID: "test_session",
		CommandResults: []SyncCommandResult{
			{ID: queryID, Status: "complete", Result: resultBytes},
		},
	}
	body2, _ := json.Marshal(req2)
	httpReq2 := httptest.NewRequest("POST", "/sync", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	cap.HandleSync(w2, httpReq2)

	var resp2 SyncResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	// After result delivered, no more pending commands — should revert to 1000ms
	if resp2.NextPollMs != 1000 {
		t.Errorf("Second sync (after result): expected NextPollMs 1000, got %d", resp2.NextPollMs)
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
		"page_url": "https://example.com",
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

func TestHandleSync_LastCommandAckPreventsRedelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	queryID := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})
	if queryID == "" {
		t.Fatal("expected query ID")
	}

	firstReqBody, _ := json.Marshal(SyncRequest{SessionID: "ack-session"})
	firstReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(firstReqBody))
	firstResp := httptest.NewRecorder()
	cap.HandleSync(firstResp, firstReq)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first sync status = %d, want 200", firstResp.Code)
	}

	var first SyncResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if len(first.Commands) == 0 || first.Commands[0].ID != queryID {
		t.Fatalf("first sync should return query %q, got %+v", queryID, first.Commands)
	}

	ackReqBody, _ := json.Marshal(SyncRequest{
		SessionID:      "ack-session",
		LastCommandAck: queryID,
	})
	ackReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(ackReqBody))
	ackResp := httptest.NewRecorder()
	cap.HandleSync(ackResp, ackReq)
	if ackResp.Code != http.StatusOK {
		t.Fatalf("ack sync status = %d, want 200", ackResp.Code)
	}

	var second SyncResponse
	if err := json.NewDecoder(ackResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if len(second.Commands) != 0 {
		t.Fatalf("acknowledged command %q should not be redelivered, got %+v", queryID, second.Commands)
	}
}
