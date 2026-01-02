package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	req := httptest.NewRequest("GET", "/health", nil)
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

	req := httptest.NewRequest("DELETE", "/logs", nil)
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

	req := httptest.NewRequest("GET", "/health", nil)
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
	mcp := NewMCPHandler(server)

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

	// Should have at least get_browser_errors and clear_browser_logs
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["get_browser_errors"] {
		t.Error("Expected tool 'get_browser_errors' in tools list")
	}

	if !toolNames["clear_browser_logs"] {
		t.Error("Expected tool 'clear_browser_logs' in tools list")
	}

	if !toolNames["get_browser_logs"] {
		t.Error("Expected tool 'get_browser_logs' in tools list")
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

	mcp := NewMCPHandler(server)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call get_browser_errors tool
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_browser_errors","arguments":{}}`),
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

	mcp := NewMCPHandler(server)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call get_browser_logs tool (returns all logs)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_browser_logs","arguments":{}}`),
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

	mcp := NewMCPHandler(server)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call get_browser_logs with limit
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_browser_logs","arguments":{"limit":5}}`),
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

	mcp := NewMCPHandler(server)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call clear_browser_logs tool
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"clear_browser_logs","arguments":{}}`),
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
	mcp := NewMCPHandler(server)

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
	mcp := NewMCPHandler(server)

	// Initialize
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call get_browser_errors with no entries
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_browser_errors","arguments":{}}`),
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
		"dataUrl":  "data:image/jpeg;base64," + jpegData,
		"url":      "https://example.com/page",
		"errorId":  "err_123_abc",
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
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
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
