package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMain wraps all tests with goroutine leak detection.
// If any test leaks goroutines, the test suite fails.
func TestMain(m *testing.M) {
	// Baseline goroutine count before tests
	baseline := runtime.NumGoroutine()

	code := m.Run()

	// Allow goroutines to wind down
	time.Sleep(200 * time.Millisecond)

	final := runtime.NumGoroutine()
	leaked := final - baseline

	// Threshold accounts for Go testing framework goroutines (~1 per test case).
	// This catches goroutine bombs (spawning in loops) but not framework internals.
	// With ~40 test cases, normal is ~40-80 framework goroutines.
	const leakThreshold = 150
	if leaked > leakThreshold {
		fmt.Fprintf(os.Stderr, "FAIL: %d goroutine(s) leaked (baseline=%d, final=%d, threshold=%d)\n",
			leaked, baseline, final, leakThreshold)
		buf := make([]byte, 1024*1024)
		n := runtime.Stack(buf, true)
		fmt.Fprintf(os.Stderr, "Goroutine dump:\n%s\n", buf[:n])
		os.Exit(1)
	}

	os.Exit(code)
}

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	server, err := NewServer(logFile, 10)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return server, logFile
}

func TestNewServer(t *testing.T) {
	server, logFile := setupTestServer(t)

	if server.logFile != logFile {
		t.Errorf("Expected logFile %s, got %s", logFile, server.logFile)
	}

	if server.maxEntries != 10 {
		t.Errorf("Expected maxEntries 10, got %d", server.maxEntries)
	}

	if len(server.entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(server.entries))
	}
}

func TestAddEntries(t *testing.T) {
	server, _ := setupTestServer(t)

	entries := []LogEntry{
		{"level": "error", "msg": "test1"},
		{"level": "warn", "msg": "test2"},
	}

	received := server.addEntries(entries)

	if received != 2 {
		t.Errorf("Expected 2 received, got %d", received)
	}

	if server.getEntryCount() != 2 {
		t.Errorf("Expected 2 entries, got %d", server.getEntryCount())
	}
}

func TestLogRotation(t *testing.T) {
	server, _ := setupTestServer(t)

	// Add 15 entries (max is 10)
	entries := make([]LogEntry, 15)
	for i := 0; i < 15; i++ {
		entries[i] = LogEntry{"index": i}
	}

	server.addEntries(entries)

	if server.getEntryCount() != 10 {
		t.Errorf("Expected 10 entries after rotation, got %d", server.getEntryCount())
	}

	// First entry should be index 5 (first 5 removed)
	server.mu.RLock()
	firstEntry := server.entries[0]
	server.mu.RUnlock()

	if firstEntry["index"].(int) != 5 {
		t.Errorf("Expected first entry index 5, got %v", firstEntry["index"])
	}
}

func TestClearEntries(t *testing.T) {
	server, _ := setupTestServer(t)

	server.addEntries([]LogEntry{{"msg": "test"}})

	if server.getEntryCount() != 1 {
		t.Fatalf("Expected 1 entry before clear")
	}

	server.clearEntries()

	if server.getEntryCount() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", server.getEntryCount())
	}
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	// Create server and add entries
	server1, _ := NewServer(logFile, 100)
	server1.addEntries([]LogEntry{
		{"level": "error", "msg": "test1"},
		{"level": "warn", "msg": "test2"},
	})

	// Create new server with same file - should load entries
	server2, _ := NewServer(logFile, 100)

	if server2.getEntryCount() != 2 {
		t.Errorf("Expected 2 entries loaded from file, got %d", server2.getEntryCount())
	}
}

func TestHealthEndpoint(t *testing.T) {
	server, logFile := setupTestServer(t)

	// Setup HTTP handler
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"status":     "ok",
			"entries":    server.getEntryCount(),
			"maxEntries": server.maxEntries,
			"logFile":    server.logFile,
		})
	})

	req := httptest.NewRequest("GET", "/health", http.NoBody)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}

	if resp["logFile"] != logFile {
		t.Errorf("Expected logFile %s, got %v", logFile, resp["logFile"])
	}
}

func TestPostLogsEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Entries []LogEntry `json:"entries"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		if body.Entries == nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing entries array"})
			return
		}

		received := server.addEntries(body.Entries)
		jsonResponse(w, http.StatusOK, map[string]int{"received": received})
	})

	body := `{"entries":[{"level":"error","msg":"test1"},{"level":"warn","msg":"test2"}]}`
	req := httptest.NewRequest("POST", "/logs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]int
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["received"] != 2 {
		t.Errorf("Expected received 2, got %d", resp["received"])
	}

	if server.getEntryCount() != 2 {
		t.Errorf("Expected 2 entries in server, got %d", server.getEntryCount())
	}
}

func TestPostLogsInvalidJSON(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Entries []LogEntry `json:"entries"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]int{"received": 0})
	})

	req := httptest.NewRequest("POST", "/logs", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestPostLogsMissingEntries(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Entries []LogEntry `json:"entries"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		if body.Entries == nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing entries array"})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]int{"received": 0})
	})

	req := httptest.NewRequest("POST", "/logs", bytes.NewBufferString(`{"foo":"bar"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestDeleteLogsEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	server.addEntries([]LogEntry{{"msg": "test"}})

	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.clearEntries()
		jsonResponse(w, http.StatusOK, map[string]bool{"cleared": true})
	})

	req := httptest.NewRequest("DELETE", "/logs", http.NoBody)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if server.getEntryCount() != 0 {
		t.Errorf("Expected 0 entries after delete, got %d", server.getEntryCount())
	}
}

func TestCORSHeaders(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/health", http.NoBody)
	req.Header.Set("Origin", "chrome-extension://abc123")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS header '*', got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestOPTIONSPreflight(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/logs", nil)
	req.Header.Set("Origin", "chrome-extension://abc123")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status 204 for OPTIONS, got %d", rec.Code)
	}
}

func TestLogFileWritten(t *testing.T) {
	server, logFile := setupTestServer(t)

	server.addEntries([]LogEntry{
		{"ts": "2024-01-22T10:00:00Z", "level": "error", "msg": "test"},
	})

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Expected log file to have content")
	}

	// Verify it's valid JSONL
	var entry LogEntry
	if err := json.Unmarshal(bytes.TrimSpace(content), &entry); err != nil {
		t.Errorf("Log file content is not valid JSON: %v", err)
	}

	if entry["level"] != "error" {
		t.Errorf("Expected level 'error', got %v", entry["level"])
	}
}

func TestEmptyEntriesArray(t *testing.T) {
	server, _ := setupTestServer(t)

	received := server.addEntries([]LogEntry{})

	if received != 0 {
		t.Errorf("Expected 0 received for empty array, got %d", received)
	}
}

// ============================================
// MCP Protocol Tests
// ============================================

func TestMCPInitialize(t *testing.T) {
	server, _ := setupTestServer(t)
	mcp := NewMCPHandler(server)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	}

	resp := mcp.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Capabilities struct {
			Tools map[string]interface{} `json:"tools"`
		} `json:"capabilities"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version 2024-11-05, got %s", result.ProtocolVersion)
	}

	if result.ServerInfo.Name != "gasoline" {
		t.Errorf("Expected server name 'gasoline', got %s", result.ServerInfo.Name)
	}
}

func TestMCPToolsList(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize first
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := mcp.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Should have composite tools: observe, configure
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["observe"] {
		t.Error("Expected tool 'observe' in tools list")
	}

	if !toolNames["configure"] {
		t.Error("Expected tool 'configure' in tools list")
	}
}

func TestMCPGetBrowserErrors(t *testing.T) {
	server, _ := setupTestServer(t)

	// Add some log entries with different levels
	server.addEntries([]LogEntry{
		{"ts": "2024-01-22T10:00:00Z", "level": "error", "type": "exception", "message": "Test error 1"},
		{"ts": "2024-01-22T10:00:01Z", "level": "warn", "type": "console", "message": "Test warning"},
		{"ts": "2024-01-22T10:00:02Z", "level": "error", "type": "network", "message": "Test error 2", "status": 500},
		{"ts": "2024-01-22T10:00:03Z", "level": "info", "type": "console", "message": "Test info"},
	})

	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call observe tool with what:"errors"
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	}

	resp := mcp.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content item")
	}

	// Parse the text content - should only contain errors
	var entries []LogEntry
	if err := json.Unmarshal([]byte(result.Content[0].Text), &entries); err != nil {
		t.Fatalf("Failed to parse entries from content: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 error entries, got %d", len(entries))
	}

	for _, entry := range entries {
		if entry["level"] != "error" {
			t.Errorf("Expected only error level entries, got %v", entry["level"])
		}
	}
}

func TestMCPGetBrowserLogs(t *testing.T) {
	server, _ := setupTestServer(t)

	// Add some log entries
	server.addEntries([]LogEntry{
		{"ts": "2024-01-22T10:00:00Z", "level": "error", "message": "Test error"},
		{"ts": "2024-01-22T10:00:01Z", "level": "warn", "message": "Test warning"},
		{"ts": "2024-01-22T10:00:02Z", "level": "info", "message": "Test info"},
	})

	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call observe tool with what:"logs" (returns all logs)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	}

	resp := mcp.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Parse the text content - should contain all logs
	var entries []LogEntry
	if err := json.Unmarshal([]byte(result.Content[0].Text), &entries); err != nil {
		t.Fatalf("Failed to parse entries from content: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries (all logs), got %d", len(entries))
	}
}

func TestMCPGetBrowserLogsWithLimit(t *testing.T) {
	server, _ := setupTestServer(t)

	// Add many log entries
	for i := 0; i < 20; i++ {
		server.addEntries([]LogEntry{
			{"ts": "2024-01-22T10:00:00Z", "level": "error", "message": "Test error", "index": i},
		})
	}

	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call observe tool with what:"logs" and limit
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs","limit":5}}`),
	}

	resp := mcp.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	var entries []LogEntry
	if err := json.Unmarshal([]byte(result.Content[0].Text), &entries); err != nil {
		t.Fatalf("Failed to parse entries from content: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("Expected 5 entries with limit, got %d", len(entries))
	}
}

func TestMCPClearBrowserLogs(t *testing.T) {
	server, _ := setupTestServer(t)

	// Add some log entries
	server.addEntries([]LogEntry{
		{"ts": "2024-01-22T10:00:00Z", "level": "error", "message": "Test error"},
	})

	if server.getEntryCount() != 1 {
		t.Fatalf("Expected 1 entry before clear")
	}

	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call configure tool with action:"clear"
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"configure","arguments":{"action":"clear"}}`),
	}

	resp := mcp.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	if server.getEntryCount() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", server.getEntryCount())
	}

	// Verify response content
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content item")
	}

	if result.Content[0].Text != "Browser logs cleared successfully" {
		t.Errorf("Expected success message, got: %s", result.Content[0].Text)
	}
}

func TestMCPUnknownTool(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call unknown tool
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"unknown_tool","arguments":{}}`),
	}

	resp := mcp.HandleRequest(req)

	if resp.Error == nil {
		t.Fatal("Expected error for unknown tool")
	}

	if resp.Error.Code != -32601 { // Method not found
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	server, _ := setupTestServer(t)
	mcp := NewMCPHandler(server)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call unknown method
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "unknown/method",
	}

	resp := mcp.HandleRequest(req)

	if resp.Error == nil {
		t.Fatal("Expected error for unknown method")
	}

	if resp.Error.Code != -32601 { // Method not found
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPGetBrowserErrorsEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call observe with what:"errors" with no entries
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	}

	resp := mcp.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Should return message about no errors
	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content item")
	}

	if result.Content[0].Text != "No browser errors found" {
		// Or it could be an empty array - either is acceptable
		var entries []LogEntry
		if err := json.Unmarshal([]byte(result.Content[0].Text), &entries); err == nil {
			if len(entries) != 0 {
				t.Errorf("Expected 0 entries or 'No browser errors found' message")
			}
		}
	}
}

func TestScreenshotEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Small valid JPEG-like data (just testing the flow, not actual image validity)
	jpegData := strings.Repeat("A", 1000) // base64 content

	body := map[string]string{
		"dataUrl":   "data:image/jpeg;base64," + jpegData,
		"url":       "https://example.com/page",
		"errorId":   "err_123_abc",
		"errorType": "console",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/screenshots", bytes.NewReader(bodyJSON))
	w := httptest.NewRecorder()

	server.handleScreenshot(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	filename := result["filename"]
	if filename == "" {
		t.Fatal("Expected filename in response")
	}

	// Verify file was saved
	savedPath := filepath.Join(tmpDir, filename)
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Fatalf("Screenshot file not saved at %s", savedPath)
	}

	// Verify filename follows convention: [website]-[timestamp]-[errortype]-[errorid].jpg
	if !strings.HasPrefix(filename, "example.com-") {
		t.Errorf("Filename should start with hostname, got: %s", filename)
	}
	if !strings.HasSuffix(filename, ".jpg") {
		t.Errorf("Filename should end with .jpg, got: %s", filename)
	}
	if !strings.Contains(filename, "console") {
		t.Errorf("Filename should contain error type, got: %s", filename)
	}
	if !strings.Contains(filename, "err_123_abc") {
		t.Errorf("Filename should contain error ID, got: %s", filename)
	}
}

func TestScreenshotEndpointInvalidMethod(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("GET", "/screenshots", nil)
	w := httptest.NewRecorder()

	server.handleScreenshot(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Result().StatusCode)
	}
}

func TestScreenshotEndpointMissingDataUrl(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	body := map[string]string{"url": "https://example.com"}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/screenshots", bytes.NewReader(bodyJSON))
	w := httptest.NewRecorder()

	server.handleScreenshot(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Result().StatusCode)
	}
}

func TestLoadEntriesLargeEntry(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	// Create a log entry larger than bufio.Scanner's default 64KB buffer
	largeData := strings.Repeat("x", 100*1024) // 100KB
	entry := LogEntry{"level": "error", "type": "console", "screenshot": largeData}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal large entry: %v", err)
	}

	// Write the large entry plus a normal entry to the log file
	normalEntry := LogEntry{"level": "warn", "msg": "normal"}
	normalData, _ := json.Marshal(normalEntry)

	content := string(data) + "\n" + string(normalData) + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write log file: %v", err)
	}

	// NewServer should load both entries without error
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed on large entry: %v", err)
	}

	if len(server.entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(server.entries))
	}
}

// ============================================
// Contract Validation Tests
// ============================================

func TestValidateLogEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry LogEntry
		valid bool
	}{
		{"valid error", LogEntry{"level": "error", "msg": "test"}, true},
		{"valid warn", LogEntry{"level": "warn", "msg": "test"}, true},
		{"valid info", LogEntry{"level": "info", "msg": "test"}, true},
		{"valid debug", LogEntry{"level": "debug", "msg": "test"}, true},
		{"valid log", LogEntry{"level": "log", "msg": "test"}, true},
		{"missing level", LogEntry{"msg": "no level"}, false},
		{"empty level", LogEntry{"level": "", "msg": "test"}, false},
		{"invalid level", LogEntry{"level": "critical", "msg": "test"}, false},
		{"level not string", LogEntry{"level": 42, "msg": "test"}, false},
		{"empty entry", LogEntry{}, false},
		{"nil level", LogEntry{"level": nil, "msg": "test"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateLogEntry(tt.entry)
			if got != tt.valid {
				t.Errorf("validateLogEntry(%v) = %v, want %v", tt.entry, got, tt.valid)
			}
		})
	}
}

func TestValidateLogEntrySize(t *testing.T) {
	// Entry under limit
	smallEntry := LogEntry{"level": "error", "msg": "small"}
	if !validateLogEntry(smallEntry) {
		t.Error("Small entry should be valid")
	}

	// Entry over 1MB limit
	largeEntry := LogEntry{"level": "error", "msg": strings.Repeat("x", maxEntrySize+1)}
	if validateLogEntry(largeEntry) {
		t.Error("Entry over 1MB should be invalid")
	}
}

func TestValidateLogEntries(t *testing.T) {
	entries := []LogEntry{
		{"level": "error", "msg": "valid1"},
		{"level": "invalid", "msg": "bad level"},
		{"level": "warn", "msg": "valid2"},
		{"msg": "no level"},
		{"level": "info", "msg": "valid3"},
	}

	valid, rejected := validateLogEntries(entries)

	if len(valid) != 3 {
		t.Errorf("Expected 3 valid entries, got %d", len(valid))
	}
	if rejected != 2 {
		t.Errorf("Expected 2 rejected entries, got %d", rejected)
	}

	// Verify correct entries were kept
	for _, entry := range valid {
		level := entry["level"].(string)
		if !validLogLevels[level] {
			t.Errorf("Invalid entry slipped through: %v", entry)
		}
	}
}

func TestValidateLogEntriesAllValid(t *testing.T) {
	entries := []LogEntry{
		{"level": "error", "msg": "e1"},
		{"level": "warn", "msg": "w1"},
	}

	valid, rejected := validateLogEntries(entries)
	if len(valid) != 2 {
		t.Errorf("Expected 2 valid, got %d", len(valid))
	}
	if rejected != 0 {
		t.Errorf("Expected 0 rejected, got %d", rejected)
	}
}

func TestValidateLogEntriesAllInvalid(t *testing.T) {
	entries := []LogEntry{
		{"msg": "no level"},
		{"level": "unknown"},
	}

	valid, rejected := validateLogEntries(entries)
	if len(valid) != 0 {
		t.Errorf("Expected 0 valid, got %d", len(valid))
	}
	if rejected != 2 {
		t.Errorf("Expected 2 rejected, got %d", rejected)
	}
}

func TestValidateLogEntriesEmpty(t *testing.T) {
	valid, rejected := validateLogEntries([]LogEntry{})
	if len(valid) != 0 {
		t.Errorf("Expected 0 valid, got %d", len(valid))
	}
	if rejected != 0 {
		t.Errorf("Expected 0 rejected, got %d", rejected)
	}
}

// ============================================
// Fuzz Tests
// ============================================

// FuzzPostLogs fuzzes the POST /logs endpoint with arbitrary JSON payloads.
// The server must never panic regardless of input.
func FuzzPostLogs(f *testing.F) {
	// Seed corpus
	f.Add([]byte(`{"entries":[{"level":"error","msg":"test"}]}`))
	f.Add([]byte(`{"entries":[]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"entries":null}`))
	f.Add([]byte(`not json at all`))
	f.Add([]byte(`{"entries":[{"level":"error","msg":"` + strings.Repeat("x", 10000) + `"}]}`))
	f.Add([]byte{0x00, 0xff, 0xfe})

	f.Fuzz(func(t *testing.T, data []byte) {
		server, _ := setupTestServer(t)

		handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Entries []LogEntry `json:"entries"`
			}

			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
				return
			}

			if body.Entries == nil {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing entries array"})
				return
			}

			server.addEntries(body.Entries)
			jsonResponse(w, http.StatusOK, map[string]int{"received": len(body.Entries)})
		})

		req := httptest.NewRequest("POST", "/logs", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Must not panic
		handler(rec, req)

		// Response must be valid HTTP
		if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
			t.Errorf("Unexpected status code: %d", rec.Code)
		}
	})
}

// FuzzMCPRequest fuzzes the MCP JSON-RPC handler with arbitrary payloads.
func FuzzMCPRequest(f *testing.F) {
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`))
	f.Add([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`))
	f.Add([]byte(`{"jsonrpc":"2.0","id":3,"method":"unknown"}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{"jsonrpc":"2.0"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		server, _ := setupTestServer(t)
		mcp := NewMCPHandler(server)

		// Initialize first
		mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
			Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
		})

		var req JSONRPCRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return // Skip unparseable inputs for this fuzz target
		}

		// Must not panic
		mcp.HandleRequest(req)
	})
}

// FuzzScreenshotEndpoint fuzzes the screenshot upload handler.
func FuzzScreenshotEndpoint(f *testing.F) {
	f.Add([]byte(`{"dataUrl":"data:image/jpeg;base64,AAAA","url":"https://example.com","errorId":"err1","errorType":"console"}`))
	f.Add([]byte(`{"dataUrl":"","url":""}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test-logs.jsonl")
		server, err := NewServer(logFile, 100)
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}

		req := httptest.NewRequest("POST", "/screenshots", bytes.NewReader(data))
		w := httptest.NewRecorder()

		// Must not panic
		server.handleScreenshot(w, req)
	})
}

func FuzzNetworkBodies(f *testing.F) {
	f.Add([]byte(`{"bodies":[{"url":"https://api.example.com/users","method":"GET","status":200,"responseBody":"{}"}]}`))
	f.Add([]byte(`{"bodies":[]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{"bodies":[{"url":"","method":"","status":0}]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		capture := NewCapture()
		req := httptest.NewRequest("POST", "/network-bodies", bytes.NewReader(data))
		w := httptest.NewRecorder()
		capture.HandleNetworkBodies(w, req)
	})
}

func FuzzWebSocketEvents(f *testing.F) {
	f.Add([]byte(`{"events":[{"id":"ws-1","event":"open","url":"wss://example.com/ws"}]}`))
	f.Add([]byte(`{"events":[{"id":"ws-1","event":"message","direction":"incoming","data":"hello"}]}`))
	f.Add([]byte(`{"events":[]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		capture := NewCapture()
		req := httptest.NewRequest("POST", "/websocket-events", bytes.NewReader(data))
		w := httptest.NewRecorder()
		capture.HandleWebSocketEvents(w, req)
	})
}

func FuzzEnhancedActions(f *testing.F) {
	f.Add([]byte(`{"actions":[{"type":"click","selector":"#btn","timestamp":"2024-01-01T00:00:00Z"}]}`))
	f.Add([]byte(`{"actions":[]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{"actions":[{"type":"input","inputType":"password","value":"secret"}]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		capture := NewCapture()
		req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewReader(data))
		w := httptest.NewRecorder()
		capture.HandleEnhancedActions(w, req)
	})
}

func FuzzValidateLogEntry(f *testing.F) {
	f.Add(`{"level":"error","msg":"test"}`)
	f.Add(`{"level":"warn","msg":"` + strings.Repeat("a", 600000) + `"}`)
	f.Add(`{"level":"invalid"}`)
	f.Add(`{}`)
	f.Add(`{"level":"error"}`)

	f.Fuzz(func(t *testing.T, data string) {
		var entry LogEntry
		if json.Unmarshal([]byte(data), &entry) != nil {
			return
		}
		validateLogEntry(entry)
	})
}

// ============================================
// Benchmarks
// ============================================

func BenchmarkAddEntries(b *testing.B) {
	tmpDir := b.TempDir()
	logFile := filepath.Join(tmpDir, "bench-logs.jsonl")
	server, _ := NewServer(logFile, 1000)

	entries := []LogEntry{
		{"level": "error", "msg": "benchmark entry", "ts": "2024-01-01T00:00:00Z"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.addEntries(entries)
	}
}

func BenchmarkAddEntriesBatch(b *testing.B) {
	tmpDir := b.TempDir()
	logFile := filepath.Join(tmpDir, "bench-logs.jsonl")
	server, _ := NewServer(logFile, 10000)

	entries := make([]LogEntry, 100)
	for i := range entries {
		entries[i] = LogEntry{"level": "error", "msg": fmt.Sprintf("entry %d", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.addEntries(entries)
		if server.getEntryCount() > 5000 {
			server.clearEntries()
		}
	}
}

func BenchmarkLogRotation(b *testing.B) {
	tmpDir := b.TempDir()
	logFile := filepath.Join(tmpDir, "bench-logs.jsonl")
	server, _ := NewServer(logFile, 100) // Small max to trigger rotation

	entries := make([]LogEntry, 50)
	for i := range entries {
		entries[i] = LogEntry{"level": "error", "msg": fmt.Sprintf("rotate %d", i)}
	}

	// Fill to capacity
	server.addEntries(entries)
	server.addEntries(entries)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.addEntries(entries) // Triggers rotation each time
	}
}

func BenchmarkMCPGetBrowserErrors(b *testing.B) {
	tmpDir := b.TempDir()
	logFile := filepath.Join(tmpDir, "bench-logs.jsonl")
	server, _ := NewServer(logFile, 1000)

	// Add mix of entries
	entries := make([]LogEntry, 500)
	for i := range entries {
		level := "info"
		if i%5 == 0 {
			level = "error"
		}
		entries[i] = LogEntry{"level": level, "msg": fmt.Sprintf("msg %d", i), "ts": "2024-01-01T00:00:00Z"}
	}
	server.addEntries(entries)

	capture := NewCapture()
	mcp := NewToolHandler(server, capture)
	b.Cleanup(func() {
		if mcp.toolHandler != nil && mcp.toolHandler.sessionStore != nil {
			mcp.toolHandler.sessionStore.Shutdown()
		}
	})
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mcp.HandleRequest(req)
	}
}

func BenchmarkMCPGetBrowserLogs(b *testing.B) {
	tmpDir := b.TempDir()
	logFile := filepath.Join(tmpDir, "bench-logs.jsonl")
	server, _ := NewServer(logFile, 1000)

	entries := make([]LogEntry, 500)
	for i := range entries {
		entries[i] = LogEntry{"level": "info", "msg": fmt.Sprintf("msg %d", i)}
	}
	server.addEntries(entries)

	capture := NewCapture()
	mcp := NewToolHandler(server, capture)
	b.Cleanup(func() {
		if mcp.toolHandler != nil && mcp.toolHandler.sessionStore != nil {
			mcp.toolHandler.sessionStore.Shutdown()
		}
	})
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mcp.HandleRequest(req)
	}
}

func BenchmarkPostLogsHTTP(b *testing.B) {
	tmpDir := b.TempDir()
	logFile := filepath.Join(tmpDir, "bench-logs.jsonl")
	server, _ := NewServer(logFile, 10000)

	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Entries []LogEntry `json:"entries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		server.addEntries(body.Entries)
		jsonResponse(w, http.StatusOK, map[string]int{"received": len(body.Entries)})
	})

	payload := []byte(`{"entries":[{"level":"error","msg":"bench1"},{"level":"warn","msg":"bench2"},{"level":"info","msg":"bench3"}]}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/logs", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if i%1000 == 0 {
			server.clearEntries()
		}
	}
}

// ============================================
// Golden File / Snapshot Tests
// ============================================

// updateGoldenFiles controls whether to update golden files.
// Run with: go test -run TestMCP.*Golden -update-golden
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden.json")
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	path := goldenPath(name)

	if updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("Failed to create testdata dir: %v", err)
		}
		// Pretty-print for readable diffs
		var buf bytes.Buffer
		json.Indent(&buf, got, "", "  ")
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("Failed to write golden file: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Golden file %s not found. Run with UPDATE_GOLDEN=1 to create it.", path)
	}

	// Normalize both for comparison
	var gotNorm, expNorm bytes.Buffer
	json.Indent(&gotNorm, got, "", "  ")
	json.Indent(&expNorm, expected, "", "  ")

	if gotNorm.String() != expNorm.String() {
		t.Errorf("Response does not match golden file %s.\nGot:\n%s\nExpected:\n%s", path, gotNorm.String(), expNorm.String())
	}
}

func TestMCPInitializeGolden(t *testing.T) {
	server, _ := setupTestServer(t)
	mcp := NewMCPHandler(server)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	}

	resp := mcp.HandleRequest(req)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	assertGolden(t, "mcp-initialize", resp.Result)
}

func TestMCPToolsListGolden(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Initialize first
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := mcp.HandleRequest(req)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	assertGolden(t, "mcp-tools-list", resp.Result)
}

func TestMCPGetBrowserErrorsGolden(t *testing.T) {
	server, _ := setupTestServer(t)

	// Add deterministic entries for snapshot
	server.addEntries([]LogEntry{
		{"ts": "2024-01-22T10:00:00Z", "level": "error", "type": "exception", "message": "ReferenceError: foo is not defined"},
		{"ts": "2024-01-22T10:00:01Z", "level": "info", "type": "console", "message": "Normal log"},
		{"ts": "2024-01-22T10:00:02Z", "level": "error", "type": "network", "message": "GET /api/data 500", "status": float64(500)},
	})

	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	}

	resp := mcp.HandleRequest(req)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	assertGolden(t, "mcp-get-browser-errors", resp.Result)
}

// ============================================
// Coverage Gap Tests
// ============================================

// TestHandleRequest_Initialized tests the "initialized" method handler
func TestHandleRequest_Initialized(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialized",
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error for 'initialized', got: %v", resp.Error)
	}
	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %v", resp.ID)
	}
	// Result should be an empty object
	if string(resp.Result) != "{}" {
		t.Errorf("Expected empty object result, got: %s", string(resp.Result))
	}
}

// TestHandleToolsCall_NilToolHandler tests tools/call with nil toolHandler
func TestHandleToolsCall_NilToolHandler(t *testing.T) {
	server, _ := setupTestServer(t)

	// Create MCPHandler directly with nil toolHandler (no NewToolHandler)
	mcp := NewMCPHandler(server)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{}}`),
	})

	if resp.Error == nil {
		t.Fatal("Expected error for nil toolHandler")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Unknown tool") {
		t.Errorf("Expected 'Unknown tool' in error, got: %s", resp.Error.Message)
	}
}

// TestHandleToolsCall_UnknownTool tests tools/call with an unrecognized tool name
func TestHandleToolsCall_UnknownTool(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"nonexistent_tool_xyz","arguments":{}}`),
	})

	if resp.Error == nil {
		t.Fatal("Expected error for unknown tool")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Unknown tool") {
		t.Errorf("Expected 'Unknown tool' in error, got: %s", resp.Error.Message)
	}
}

// TestLoadEntries_MalformedJSON tests that malformed JSON lines are skipped
func TestLoadEntries_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "malformed.jsonl")

	// Write a file with mixed valid/invalid JSON lines
	content := `{"level":"error","msg":"valid entry 1"}
this is not json
{"level":"info","msg":"valid entry 2"}
{broken json here
{"level":"warn","msg":"valid entry 3"}
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Should have loaded only the 3 valid entries, skipping malformed lines
	if server.getEntryCount() != 3 {
		t.Errorf("Expected 3 valid entries, got %d", server.getEntryCount())
	}
}

// TestLoadEntries_EmptyLines tests that empty lines are skipped
func TestLoadEntries_EmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "empty-lines.jsonl")

	content := `{"level":"error","msg":"entry 1"}

   
{"level":"info","msg":"entry 2"}

`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server.getEntryCount() != 2 {
		t.Errorf("Expected 2 entries (empty lines skipped), got %d", server.getEntryCount())
	}
}

// TestValidateLogEntry_OversizedEntry tests rejection of entries exceeding 1MB
func TestValidateLogEntry_OversizedEntry(t *testing.T) {
	// Create an entry with a value larger than 1MB
	largeValue := strings.Repeat("x", maxEntrySize+1)
	entry := LogEntry{
		"level": "error",
		"msg":   largeValue,
	}

	if validateLogEntry(entry) {
		t.Error("Expected oversized entry to be rejected")
	}
}

// TestValidateLogEntry_ExactMaxSize tests an entry at exactly the max size boundary
func TestValidateLogEntry_ExactMaxSize(t *testing.T) {
	// An entry that is just under the limit should pass
	// The overhead of {"level":"error","msg":"..."} is about 22 bytes
	overhead := len(`{"level":"error","msg":""}`)
	value := strings.Repeat("a", maxEntrySize-overhead-1)
	entry := LogEntry{
		"level": "error",
		"msg":   value,
	}

	if !validateLogEntry(entry) {
		t.Error("Expected entry at boundary to be accepted")
	}
}

// TestClearEntries_SaveError tests clearEntries when save fails (read-only file)
func TestClearEntries_SaveError(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "readonly-test.jsonl")

	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Add some entries
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "test"},
	})

	// Make the file read-only to trigger a save error
	if err := os.Chmod(logFile, 0444); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(logFile, 0644) // restore for cleanup
	})

	// clearEntries should not panic even though save fails
	server.clearEntries()

	// Entries should still be cleared in memory
	if server.getEntryCount() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", server.getEntryCount())
	}
}

// TestSaveEntries_WriteError tests saveEntries when the file cannot be written
func TestSaveEntries_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a path inside a non-existent directory to trigger create error
	logFile := filepath.Join(tmpDir, "nonexistent-subdir", "test.jsonl")

	server := &Server{
		logFile:    logFile,
		maxEntries: 100,
		entries: []LogEntry{
			{"level": "error", "msg": "test"},
		},
	}

	err := server.saveEntries()
	if err == nil {
		t.Error("Expected error when directory doesn't exist")
	}
}
