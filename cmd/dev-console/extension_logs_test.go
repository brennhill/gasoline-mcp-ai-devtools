package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================
// Extension Logs Tests (TDD Phase 2)
// ============================================
// These tests verify the extension logs capture system for AI debugging
// of extension internal behavior.

// ============================================
// HTTP Handler Tests
// ============================================

func TestHandleExtensionLogs_AcceptsValidPayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := map[string]interface{}{
		"logs": []map[string]interface{}{
			{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "debug",
				"message":   "Starting settings heartbeat",
				"source":    "background",
				"category":  "CONNECTION",
				"data": map[string]interface{}{
					"serverUrl": "http://localhost:7890",
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleExtensionLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify entry was stored
	capture.mu.RLock()
	count := len(capture.extensionLogs)
	capture.mu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 extension log entry, got %d", count)
	}
}

func TestHandleExtensionLogs_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	req := httptest.NewRequest("POST", "/extension-logs", strings.NewReader("{invalid json"))
	w := httptest.NewRecorder()

	capture.HandleExtensionLogs(w, req)

	if w.Code == http.StatusOK {
		t.Error("Should reject malformed JSON")
	}
}

func TestHandleExtensionLogs_RejectsNonPOST(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	req := httptest.NewRequest("GET", "/extension-logs", nil)
	w := httptest.NewRecorder()

	capture.HandleExtensionLogs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleExtensionLogs_StoresTimestamp(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := map[string]interface{}{
		"logs": []map[string]interface{}{
			{
				"level":    "info",
				"message":  "Test message",
				"source":   "background",
				"category": "LIFECYCLE",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	before := time.Now()
	capture.HandleExtensionLogs(w, req)
	after := time.Now()

	capture.mu.RLock()
	entry := capture.extensionLogs[0]
	capture.mu.RUnlock()

	if entry.Timestamp.Before(before) || entry.Timestamp.After(after) {
		t.Error("Timestamp should be set to current time when not provided")
	}
}

func TestHandleExtensionLogs_PreservesProvidedTimestamp(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	providedTime := time.Now().Add(-5 * time.Minute)
	payload := map[string]interface{}{
		"logs": []map[string]interface{}{
			{
				"timestamp": providedTime.Format(time.RFC3339),
				"level":     "debug",
				"message":   "Old log entry",
				"source":    "content",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleExtensionLogs(w, req)

	capture.mu.RLock()
	entry := capture.extensionLogs[0]
	capture.mu.RUnlock()

	// Allow 1 second tolerance
	if entry.Timestamp.Sub(providedTime).Abs() > time.Second {
		t.Errorf("Expected timestamp %v, got %v", providedTime, entry.Timestamp)
	}
}

func TestHandleExtensionLogs_ReturnsCorrectResponse(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := map[string]interface{}{
		"logs": []map[string]interface{}{
			{
				"level":   "debug",
				"message": "Test 1",
				"source":  "background",
			},
			{
				"level":   "info",
				"message": "Test 2",
				"source":  "content",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleExtensionLogs(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}

	logsStored, ok := response["logs_stored"].(float64)
	if !ok || int(logsStored) != 2 {
		t.Errorf("Expected logs_stored=2, got %v", response["logs_stored"])
	}
}

func TestHandleExtensionLogs_RejectsOversizedPayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Create a payload larger than maxPostBodySize (5MB)
	largeMessage := strings.Repeat("x", 6*1024*1024)
	payload := map[string]interface{}{
		"logs": []map[string]interface{}{
			{
				"level":   "debug",
				"message": largeMessage,
				"source":  "background",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleExtensionLogs(w, req)

	if w.Code == http.StatusOK {
		t.Error("Should reject oversized payload")
	}
}

// ============================================
// Ring Buffer Tests
// ============================================

func TestExtensionLogs_RingBufferEviction(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Add maxExtensionLogs + 10 entries (should keep only last maxExtensionLogs)
	entriesToAdd := maxExtensionLogs + 10

	for i := 1; i <= entriesToAdd; i++ {
		payload := map[string]interface{}{
			"logs": []map[string]interface{}{
				{
					"level":   "debug",
					"message": "Log entry " + string(rune('0'+i%10)),
					"source":  "background",
				},
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
		w := httptest.NewRecorder()

		capture.HandleExtensionLogs(w, req)
	}

	capture.mu.RLock()
	count := len(capture.extensionLogs)
	capture.mu.RUnlock()

	if count != maxExtensionLogs {
		t.Errorf("Expected %d entries (capacity), got %d", maxExtensionLogs, count)
	}
}

func TestExtensionLogs_MultipleEntriesInSinglePayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := map[string]interface{}{
		"logs": []map[string]interface{}{
			{
				"level":    "debug",
				"message":  "First log",
				"source":   "background",
				"category": "CONNECTION",
			},
			{
				"level":    "info",
				"message":  "Second log",
				"source":   "content",
				"category": "CAPTURE",
			},
			{
				"level":    "error",
				"message":  "Third log",
				"source":   "inject",
				"category": "ERROR",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleExtensionLogs(w, req)

	capture.mu.RLock()
	count := len(capture.extensionLogs)
	capture.mu.RUnlock()

	if count != 3 {
		t.Errorf("Expected 3 entries, got %d", count)
	}
}

func TestExtensionLogs_PreallocatedBuffer(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Verify buffer was pre-allocated
	if capture.extensionLogs == nil {
		t.Error("Extension logs buffer should be pre-allocated, not nil")
	}

	if cap(capture.extensionLogs) != maxExtensionLogs {
		t.Errorf("Expected pre-allocated capacity %d, got %d", maxExtensionLogs, cap(capture.extensionLogs))
	}
}

// ============================================
// MCP Tool Tests
// ============================================

func TestToolGetExtensionLogs_EmptyBuffer(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolGetExtensionLogs(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	text := content.Content[0].Text
	if !strings.Contains(text, "No extension logs found") {
		t.Errorf("Expected 'No extension logs found' message, got: %s", text)
	}
}

func TestToolGetExtensionLogs_PopulatedBuffer(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add some extension logs
	capture.mu.Lock()
	capture.extensionLogs = []ExtensionLog{
		{
			Timestamp: time.Now(),
			Level:     "debug",
			Message:   "Starting heartbeat",
			Source:    "background",
			Category:  "CONNECTION",
			Data: map[string]interface{}{
				"serverUrl": "http://localhost:7890",
			},
		},
		{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Posted settings",
			Source:    "background",
			Category:  "CONNECTION",
		},
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolGetExtensionLogs(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	text := content.Content[0].Text
	if !strings.Contains(text, "2 extension log entries") {
		t.Errorf("Expected '2 extension log entries' in summary, got: %s", text)
	}
	if !strings.Contains(text, "Starting heartbeat") {
		t.Error("Expected log message in output")
	}
	if !strings.Contains(text, "CONNECTION") {
		t.Error("Expected category in output")
	}
}

func TestToolGetExtensionLogs_LimitParameter(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add 5 extension logs
	capture.mu.Lock()
	for i := 1; i <= 5; i++ {
		capture.extensionLogs = append(capture.extensionLogs, ExtensionLog{
			Timestamp: time.Now(),
			Level:     "debug",
			Message:   "Log entry",
			Source:    "background",
		})
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"limit": 2}`)
	resp := mcp.toolHandler.toolGetExtensionLogs(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	text := content.Content[0].Text
	if !strings.Contains(text, "2 extension log entries") {
		t.Errorf("Expected '2 extension log entries' with limit applied, got: %s", text)
	}
}

// ============================================
// Data Counts Tests
// ============================================

func TestComputeDataCounts_IncludesExtensionLogs(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add some extension logs
	capture.mu.Lock()
	capture.extensionLogs = []ExtensionLog{
		{Timestamp: time.Now(), Level: "debug", Message: "Test 1", Source: "background"},
		{Timestamp: time.Now(), Level: "info", Message: "Test 2", Source: "content"},
		{Timestamp: time.Now(), Level: "error", Message: "Test 3", Source: "inject"},
	}
	capture.mu.Unlock()

	_, _, extensionLogsCount, _, _, _, _, _, _, _ := mcp.toolHandler.computeDataCounts()

	if extensionLogsCount != 3 {
		t.Errorf("Expected extensionLogsCount=3, got %d", extensionLogsCount)
	}
}

// ============================================
// Concurrent Access Tests
// ============================================

func TestExtensionLogs_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Write 100 entries concurrently
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(index int) {
			payload := map[string]interface{}{
				"logs": []map[string]interface{}{
					{
						"level":   "debug",
						"message": "Concurrent log",
						"source":  "background",
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
			w := httptest.NewRecorder()

			capture.HandleExtensionLogs(w, req)
			done <- true
		}(i)
	}

	// Wait for all writes to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	capture.mu.RLock()
	count := len(capture.extensionLogs)
	capture.mu.RUnlock()

	if count != 100 {
		t.Errorf("Expected 100 entries, got %d", count)
	}
}
