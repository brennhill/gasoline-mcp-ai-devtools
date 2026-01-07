// ci_test.go — Tests for Gasoline CI Infrastructure endpoints.
// Covers /snapshot (aggregated state retrieval), /clear (buffer reset),
// and /test-boundary (test correlation) endpoints.
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ============================================
// /snapshot endpoint tests
// ============================================

func TestHandleSnapshot_EmptyState(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	handler := handleSnapshot(server, capture)
	req := httptest.NewRequest("GET", "/snapshot", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp SnapshotResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if len(resp.Logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(resp.Logs))
	}
	if len(resp.WebSocket) != 0 {
		t.Errorf("expected 0 ws events, got %d", len(resp.WebSocket))
	}
	if len(resp.NetworkBodies) != 0 {
		t.Errorf("expected 0 network bodies, got %d", len(resp.NetworkBodies))
	}
	if resp.Stats.TotalLogs != 0 {
		t.Errorf("expected 0 total logs, got %d", resp.Stats.TotalLogs)
	}
}

func TestHandleSnapshot_WithData(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	// Add log entries
	server.addEntries([]LogEntry{
		{"level": "error", "message": "test error"},
		{"level": "warn", "message": "test warning"},
		{"level": "log", "message": "test info"},
	})

	// Add network bodies
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "http://example.com/api", Status: 200, Method: "GET"},
		{URL: "http://example.com/fail", Status: 500, Method: "POST"},
	})

	handler := handleSnapshot(server, capture)
	req := httptest.NewRequest("GET", "/snapshot", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp SnapshotResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(resp.Logs))
	}
	if len(resp.NetworkBodies) != 2 {
		t.Errorf("expected 2 network bodies, got %d", len(resp.NetworkBodies))
	}
	if resp.Stats.TotalLogs != 3 {
		t.Errorf("expected total_logs=3, got %d", resp.Stats.TotalLogs)
	}
	if resp.Stats.ErrorCount != 1 {
		t.Errorf("expected error_count=1, got %d", resp.Stats.ErrorCount)
	}
	if resp.Stats.WarningCount != 1 {
		t.Errorf("expected warning_count=1, got %d", resp.Stats.WarningCount)
	}
	if resp.Stats.NetworkFailures != 1 {
		t.Errorf("expected network_failures=1, got %d", resp.Stats.NetworkFailures)
	}
}

func TestHandleSnapshot_MethodNotAllowed(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	handler := handleSnapshot(server, capture)
	req := httptest.NewRequest("POST", "/snapshot", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleSnapshot_SinceFilter(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	// Add entries with timestamps
	now := time.Now().UTC()
	server.addEntries([]LogEntry{
		{"level": "error", "message": "old error", "ts": now.Add(-10 * time.Second).Format(time.RFC3339Nano)},
	})
	// Small delay to separate timestamps
	time.Sleep(5 * time.Millisecond)
	cutoff := time.Now().UTC()
	time.Sleep(5 * time.Millisecond)
	server.addEntries([]LogEntry{
		{"level": "error", "message": "new error", "ts": time.Now().UTC().Format(time.RFC3339Nano)},
	})

	handler := handleSnapshot(server, capture)
	req := httptest.NewRequest("GET", "/snapshot?since="+cutoff.Format(time.RFC3339Nano), nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp SnapshotResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Logs) != 1 {
		t.Errorf("expected 1 log after since filter, got %d", len(resp.Logs))
	}
}

func TestHandleSnapshot_InvalidSince(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	handler := handleSnapshot(server, capture)
	req := httptest.NewRequest("GET", "/snapshot?since=not-a-date", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ============================================
// /clear endpoint tests
// ============================================

func TestHandleClear_EmptyState(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	handler := handleClear(server, capture)
	req := httptest.NewRequest("POST", "/clear", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["cleared"] != true {
		t.Error("expected cleared=true")
	}
}

func TestHandleClear_WithData(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	// Add data
	server.addEntries([]LogEntry{
		{"level": "error", "message": "test"},
		{"level": "warn", "message": "test2"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "http://example.com", Status: 200, Method: "GET"},
	})

	handler := handleClear(server, capture)
	req := httptest.NewRequest("POST", "/clear", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check entries were removed
	removed, ok := resp["entries_removed"].(float64) // JSON numbers are float64
	if !ok || removed != 2 {
		t.Errorf("expected entries_removed=2, got %v", resp["entries_removed"])
	}

	// Verify state is actually cleared
	if server.getEntryCount() != 0 {
		t.Errorf("expected 0 logs after clear, got %d", server.getEntryCount())
	}
	bodies := capture.GetNetworkBodies(NetworkBodyFilter{})
	if len(bodies) != 0 {
		t.Errorf("expected 0 network bodies after clear, got %d", len(bodies))
	}
}

func TestHandleClear_MethodNotAllowed(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	handler := handleClear(server, capture)
	req := httptest.NewRequest("GET", "/clear", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleClear_DeleteMethod(t *testing.T) {
	server, _ := NewServer("", 1000)
	capture := NewCapture()

	server.addEntries([]LogEntry{{"level": "log", "message": "test"}})

	handler := handleClear(server, capture)
	req := httptest.NewRequest("DELETE", "/clear", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for DELETE, got %d", w.Code)
	}
}

// ============================================
// /test-boundary endpoint tests
// ============================================

func TestHandleTestBoundary_Start(t *testing.T) {
	capture := NewCapture()

	handler := handleTestBoundary(capture)
	body := `{"test_id": "login-flow", "action": "start"}`
	req := httptest.NewRequest("POST", "/test-boundary", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["test_id"] != "login-flow" {
		t.Errorf("expected test_id=login-flow, got %v", resp["test_id"])
	}
	if resp["action"] != "start" {
		t.Errorf("expected action=start, got %v", resp["action"])
	}

	// Verify current test ID is set
	if capture.GetCurrentTestID() != "login-flow" {
		t.Errorf("expected current test ID to be login-flow, got %q", capture.GetCurrentTestID())
	}
}

func TestHandleTestBoundary_End(t *testing.T) {
	capture := NewCapture()
	capture.SetCurrentTestID("login-flow")

	handler := handleTestBoundary(capture)
	body := `{"test_id": "login-flow", "action": "end"}`
	req := httptest.NewRequest("POST", "/test-boundary", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify current test ID is cleared
	if capture.GetCurrentTestID() != "" {
		t.Errorf("expected empty test ID after end, got %q", capture.GetCurrentTestID())
	}
}

func TestHandleTestBoundary_InvalidAction(t *testing.T) {
	capture := NewCapture()

	handler := handleTestBoundary(capture)
	body := `{"test_id": "test", "action": "pause"}`
	req := httptest.NewRequest("POST", "/test-boundary", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid action, got %d", w.Code)
	}
}

func TestHandleTestBoundary_MissingTestID(t *testing.T) {
	capture := NewCapture()

	handler := handleTestBoundary(capture)
	body := `{"action": "start"}`
	req := httptest.NewRequest("POST", "/test-boundary", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing test_id, got %d", w.Code)
	}
}

func TestHandleTestBoundary_MethodNotAllowed(t *testing.T) {
	capture := NewCapture()

	handler := handleTestBoundary(capture)
	req := httptest.NewRequest("GET", "/test-boundary", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// ============================================
// computeSnapshotStats tests
// ============================================

func TestComputeSnapshotStats(t *testing.T) {
	logs := []LogEntry{
		{"level": "error", "message": "err1"},
		{"level": "error", "message": "err2"},
		{"level": "warn", "message": "warn1"},
		{"level": "log", "message": "info1"},
	}
	networkBodies := []NetworkBody{
		{Status: 200},
		{Status: 404},
		{Status: 500},
	}

	stats := computeSnapshotStats(logs, nil, networkBodies)

	if stats.TotalLogs != 4 {
		t.Errorf("expected total_logs=4, got %d", stats.TotalLogs)
	}
	if stats.ErrorCount != 2 {
		t.Errorf("expected error_count=2, got %d", stats.ErrorCount)
	}
	if stats.WarningCount != 1 {
		t.Errorf("expected warning_count=1, got %d", stats.WarningCount)
	}
	if stats.NetworkFailures != 2 {
		t.Errorf("expected network_failures=2, got %d", stats.NetworkFailures)
	}
}

// ============================================
// clearAll tests
// ============================================

func TestCaptureClearAll(t *testing.T) {
	capture := NewCapture()

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "http://example.com", Status: 200, Method: "GET"},
	})
	capture.AddWebSocketEvents([]WebSocketEvent{
		{URL: "ws://example.com", Type: "open"},
	})
	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click"},
	})

	// Set a test ID
	capture.SetCurrentTestID("test-1")

	capture.ClearAll()

	if len(capture.GetNetworkBodies(NetworkBodyFilter{})) != 0 {
		t.Error("expected 0 network bodies after ClearAll")
	}
	if len(capture.GetWebSocketEvents(WebSocketEventFilter{})) != 0 {
		t.Error("expected 0 ws events after ClearAll")
	}
	if len(capture.GetEnhancedActions(EnhancedActionFilter{})) != 0 {
		t.Error("expected 0 actions after ClearAll")
	}
	if capture.GetCurrentTestID() != "" {
		t.Error("expected empty test ID after ClearAll")
	}
}
