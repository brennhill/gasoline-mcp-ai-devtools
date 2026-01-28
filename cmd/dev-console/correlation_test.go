package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// W8: Correlation ID Tests
// ============================================

// --- Screenshot endpoint with correlation_id ---

func TestScreenshot_FilenameWithCorrelationID(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	jpegData := strings.Repeat("A", 1000)
	body := map[string]string{
		"dataUrl":       "data:image/jpeg;base64," + jpegData,
		"url":           "https://example.com/page",
		"correlationId": "err-42",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/screenshots", strings.NewReader(string(bodyJSON)))
	w := httptest.NewRecorder()
	server.handleScreenshot(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	filename := result["filename"]
	if !strings.Contains(filename, "err-42") {
		t.Errorf("Expected filename to contain 'err-42', got: %s", filename)
	}
	if !strings.HasSuffix(filename, ".jpg") {
		t.Errorf("Expected .jpg suffix, got: %s", filename)
	}
}

func TestScreenshot_FilenameWithoutCorrelationID(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	jpegData := strings.Repeat("A", 1000)
	body := map[string]string{
		"dataUrl": "data:image/jpeg;base64," + jpegData,
		"url":     "https://example.com/page",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/screenshots", strings.NewReader(string(bodyJSON)))
	w := httptest.NewRecorder()
	server.handleScreenshot(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	filename := result["filename"]
	// Without correlationId, filename should be hostname-timestamp.jpg (no trailing dash)
	if !strings.HasPrefix(filename, "example.com-") {
		t.Errorf("Expected filename to start with 'example.com-', got: %s", filename)
	}
	if !strings.HasSuffix(filename, ".jpg") {
		t.Errorf("Expected .jpg suffix, got: %s", filename)
	}
	// Should NOT have trailing dash before .jpg
	if strings.Contains(filename, "-.jpg") {
		t.Errorf("Expected no trailing dash before .jpg, got: %s", filename)
	}
}

func TestScreenshot_CorrelationIDSanitized(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	jpegData := strings.Repeat("A", 1000)
	body := map[string]string{
		"dataUrl":       "data:image/jpeg;base64," + jpegData,
		"url":           "https://example.com/page",
		"correlationId": "foo/bar:baz",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/screenshots", strings.NewReader(string(bodyJSON)))
	w := httptest.NewRecorder()
	server.handleScreenshot(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	filename := result["filename"]
	// Special chars should be sanitized (replaced with underscore)
	if strings.Contains(filename, "/") || strings.Contains(filename, ":") {
		t.Errorf("Expected special chars sanitized in filename, got: %s", filename)
	}
	// The sanitized correlation ID should still be present
	if !strings.Contains(filename, "foo_bar_baz") {
		t.Errorf("Expected sanitized correlation ID 'foo_bar_baz' in filename, got: %s", filename)
	}
}

func TestScreenshot_CorrelationIDEchoedInResponse(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	jpegData := strings.Repeat("A", 1000)
	body := map[string]string{
		"dataUrl":       "data:image/jpeg;base64," + jpegData,
		"url":           "https://example.com/page",
		"correlationId": "err-42",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/screenshots", strings.NewReader(string(bodyJSON)))
	w := httptest.NewRecorder()
	server.handleScreenshot(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["correlation_id"] != "err-42" {
		t.Errorf("Expected correlation_id 'err-42' in response, got: %q", result["correlation_id"])
	}
}

// --- Schema: correlation_id in interact tool ---

func TestInteract_CorrelationIDInSchema(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	initMCP(t, mcp)

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result MCPToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	// Find the interact tool
	var interactTool *MCPTool
	for i, tool := range result.Tools {
		if tool.Name == "interact" {
			interactTool = &result.Tools[i]
			break
		}
	}
	if interactTool == nil {
		t.Fatal("interact tool not found in tools list")
	}

	// Check that correlation_id exists in properties
	props, ok := interactTool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("interact tool missing properties in inputSchema")
	}

	corrProp, ok := props["correlation_id"]
	if !ok {
		t.Fatal("interact tool missing correlation_id property")
	}

	corrPropMap, ok := corrProp.(map[string]interface{})
	if !ok {
		t.Fatal("correlation_id property is not a map")
	}

	if corrPropMap["type"] != "string" {
		t.Errorf("Expected correlation_id type 'string', got: %v", corrPropMap["type"])
	}

	if corrPropMap["description"] == nil || corrPropMap["description"] == "" {
		t.Error("Expected correlation_id to have a description")
	}
}
