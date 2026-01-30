package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// Unit Tests: checkLogQuality
// ============================================

func TestCheckLogQuality_AllFieldsPresent(t *testing.T) {
	t.Parallel()
	entries := []LogEntry{
		{"ts": "2024-01-01T00:00:00Z", "message": "hello", "source": "app.js:1"},
		{"ts": "2024-01-01T00:00:01Z", "message": "world", "source": "app.js:2"},
	}
	result := checkLogQuality(entries)
	if result != "" {
		t.Errorf("Expected empty string for clean entries, got %q", result)
	}
}

func TestCheckLogQuality_MissingTimestamp(t *testing.T) {
	t.Parallel()
	entries := []LogEntry{
		{"message": "no timestamp", "source": "app.js:1"},
	}
	result := checkLogQuality(entries)
	if result == "" {
		t.Fatal("Expected warning for missing ts, got empty string")
	}
	if !strings.Contains(result, "missing 'ts'") {
		t.Errorf("Expected warning to mention missing 'ts', got %q", result)
	}
}

func TestCheckLogQuality_MissingMessage(t *testing.T) {
	t.Parallel()
	entries := []LogEntry{
		{"ts": "2024-01-01T00:00:00Z", "source": "app.js:1"},
	}
	result := checkLogQuality(entries)
	if result == "" {
		t.Fatal("Expected warning for missing message, got empty string")
	}
	if !strings.Contains(result, "missing 'message'") {
		t.Errorf("Expected warning to mention missing 'message', got %q", result)
	}
}

func TestCheckLogQuality_MissingSource(t *testing.T) {
	t.Parallel()
	entries := []LogEntry{
		{"ts": "2024-01-01T00:00:00Z", "message": "test"},
	}
	result := checkLogQuality(entries)
	if result == "" {
		t.Fatal("Expected warning for missing source, got empty string")
	}
	if !strings.Contains(result, "missing 'source'") {
		t.Errorf("Expected warning to mention missing 'source', got %q", result)
	}
}

func TestCheckLogQuality_AllThreeMissing_SingleEntry(t *testing.T) {
	t.Parallel()
	entries := []LogEntry{
		{"level": "error"},
	}
	result := checkLogQuality(entries)
	if result == "" {
		t.Fatal("Expected warning for entry missing all fields, got empty string")
	}
	if !strings.Contains(result, "1/1 entries") {
		t.Errorf("Expected warning to say '1/1 entries', got %q", result)
	}
	if !strings.Contains(result, "missing 'ts'") {
		t.Errorf("Expected warning to mention missing 'ts', got %q", result)
	}
	if !strings.Contains(result, "missing 'message'") {
		t.Errorf("Expected warning to mention missing 'message', got %q", result)
	}
	if !strings.Contains(result, "missing 'source'") {
		t.Errorf("Expected warning to mention missing 'source', got %q", result)
	}
}

func TestCheckLogQuality_MultipleEntries_Mixed(t *testing.T) {
	t.Parallel()
	entries := []LogEntry{
		{"ts": "2024-01-01T00:00:00Z", "message": "ok", "source": "app.js:1"},
		{"message": "no ts", "source": "app.js:2"},
		{"ts": "2024-01-01T00:00:02Z"},
	}
	result := checkLogQuality(entries)
	if result == "" {
		t.Fatal("Expected warning for mixed entries, got empty string")
	}
	if !strings.Contains(result, "2/3 entries") {
		t.Errorf("Expected warning to say '2/3 entries', got %q", result)
	}
}

func TestCheckLogQuality_EmptySlice(t *testing.T) {
	t.Parallel()
	result := checkLogQuality([]LogEntry{})
	if result != "" {
		t.Errorf("Expected empty string for empty slice, got %q", result)
	}
}

// ============================================
// Integration Tests: observe errors/logs with quality warnings
// ============================================

func TestToolGetBrowserErrors_CleanData_NoWarning(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "test error", "ts": "2024-01-01T00:00:00Z", "source": "app.js:1"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	text := extractMCPText(t, resp)
	if strings.Contains(text, "WARNING") {
		t.Errorf("Expected no WARNING for clean data, got:\n%s", text)
	}
}

func TestToolGetBrowserErrors_MissingSource_ShowsWarning(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "test error", "ts": "2024-01-01T00:00:00Z"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "WARNING") {
		t.Errorf("Expected WARNING for missing source, got:\n%s", text)
	}
}

// TestToolGetBrowserErrors_LimitParameter verifies that the limit parameter
// correctly restricts the number of errors returned (BUG-002).
func TestToolGetBrowserErrors_LimitParameter(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	// Add 5 errors
	server.addEntries([]LogEntry{
		{"level": "error", "message": "error 1", "ts": "2024-01-01T00:00:01Z", "source": "app.js:1"},
		{"level": "error", "message": "error 2", "ts": "2024-01-01T00:00:02Z", "source": "app.js:2"},
		{"level": "error", "message": "error 3", "ts": "2024-01-01T00:00:03Z", "source": "app.js:3"},
		{"level": "error", "message": "error 4", "ts": "2024-01-01T00:00:04Z", "source": "app.js:4"},
		{"level": "error", "message": "error 5", "ts": "2024-01-01T00:00:05Z", "source": "app.js:5"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors","limit":2}}`),
	})

	text := extractMCPText(t, resp)

	// Should show only 2 errors
	if !strings.Contains(text, "2 browser error(s)") {
		t.Errorf("Expected '2 browser error(s)' in summary, got:\n%s", text)
	}

	// Count table rows (exclude header/separator)
	lines := strings.Split(text, "\n")
	dataRows := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") && !strings.Contains(line, "Level") {
			dataRows++
		}
	}
	if dataRows != 2 {
		t.Errorf("Expected 2 data rows in table, got %d", dataRows)
	}
}

func TestToolGetBrowserLogs_Warning_DoesNotBreakMarkdown(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	server.addEntries([]LogEntry{
		{"level": "info", "message": "test log"},
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	text := extractMCPText(t, resp)
	// Should contain WARNING since ts and source are missing
	if !strings.Contains(text, "WARNING") {
		t.Errorf("Expected WARNING for missing fields, got:\n%s", text)
	}
	// JSON response should still be present after the warning
	if !strings.Contains(text, `"logs"`) {
		t.Errorf("Expected JSON 'logs' field to still be present, got:\n%s", text)
	}
	if !strings.Contains(text, `"level":"info"`) && !strings.Contains(text, `"level": "info"`) {
		t.Errorf("Expected log entry with level 'info' to still be present, got:\n%s", text)
	}
}
