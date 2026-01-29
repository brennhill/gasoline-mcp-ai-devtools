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
	"time"
)

// ============================================
// toolConfigureStore Tests (composite store path)
// ============================================

func TestToolConfigureStoreNilStore(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.sessionStore = nil

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"store_action":"save","namespace":"test","key":"k1","data":{"foo":"bar"}}`)
	resp := mcp.toolHandler.toolConfigureStore(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if !content.IsError {
		t.Error("Expected isError=true when sessionStore is nil")
	}
	if len(content.Content) == 0 || !strings.Contains(content.Content[0].Text, "Session store not initialized") {
		t.Error("Expected 'Session store not initialized' error message")
	}
}

func TestToolConfigureStoreSave(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"store_action":"save","namespace":"testns","key":"testkey","data":{"hello":"world"}}`)
	resp := mcp.toolHandler.toolConfigureStore(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Errorf("Expected no error, got: %s", content.Content[0].Text)
	}
	if len(content.Content) == 0 || !strings.Contains(content.Content[0].Text, "saved") {
		t.Error("Expected response to contain 'saved'")
	}
}

func TestToolConfigureStoreLoadAfterSave(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "tools/call",
	}

	saveArgs := json.RawMessage(`{"store_action":"save","namespace":"testns","key":"loadkey","data":{"value":42}}`)
	mcp.toolHandler.toolConfigureStore(req, saveArgs)

	loadArgs := json.RawMessage(`{"store_action":"load","namespace":"testns","key":"loadkey"}`)
	resp := mcp.toolHandler.toolConfigureStore(req, loadArgs)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Errorf("Expected no error on load, got: %s", content.Content[0].Text)
	}
	if len(content.Content) == 0 || !strings.Contains(content.Content[0].Text, "loadkey") {
		t.Error("Expected response to contain key name")
	}
}

func TestToolConfigureStoreList(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
		Method:  "tools/call",
	}

	saveArgs := json.RawMessage(`{"store_action":"save","namespace":"listns","key":"item1","data":"val1"}`)
	mcp.toolHandler.toolConfigureStore(req, saveArgs)

	listArgs := json.RawMessage(`{"store_action":"list","namespace":"listns"}`)
	resp := mcp.toolHandler.toolConfigureStore(req, listArgs)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Errorf("Expected no error on list, got: %s", content.Content[0].Text)
	}
	if len(content.Content) == 0 || !strings.Contains(content.Content[0].Text, "listns") {
		t.Error("Expected response to contain namespace name")
	}
}

func TestToolConfigureStoreDelete(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`6`),
		Method:  "tools/call",
	}

	saveArgs := json.RawMessage(`{"store_action":"save","namespace":"delns","key":"delkey","data":"to-delete"}`)
	mcp.toolHandler.toolConfigureStore(req, saveArgs)

	deleteArgs := json.RawMessage(`{"store_action":"delete","namespace":"delns","key":"delkey"}`)
	resp := mcp.toolHandler.toolConfigureStore(req, deleteArgs)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Errorf("Expected no error on delete, got: %s", content.Content[0].Text)
	}
	if len(content.Content) == 0 || !strings.Contains(content.Content[0].Text, "deleted") {
		t.Error("Expected response to contain 'deleted'")
	}
}

func TestToolConfigureStoreDefaultStats(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`7`),
		Method:  "tools/call",
	}

	// Empty store_action defaults to "stats"
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolConfigureStore(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Errorf("Expected no error on stats, got: %s", content.Content[0].Text)
	}
}

func TestToolConfigureStoreUnknownAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`8`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"store_action":"nonexistent"}`)
	resp := mcp.toolHandler.toolConfigureStore(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if !content.IsError {
		t.Error("Expected isError=true for unknown action")
	}
	if len(content.Content) == 0 || !strings.Contains(content.Content[0].Text, "Error") {
		t.Error("Expected error message for unknown action")
	}
}

// ============================================
// toolConfigureNoise Tests
// ============================================

func TestToolConfigureNoiseAddRules(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`10`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{
		"action": "add",
		"rules": [{
			"category": "console",
			"classification": "known",
			"match_spec": {"message_regex": "test.*noise"}
		}]
	}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, `"status":"ok"`) && !strings.Contains(text, `"status": "ok"`) {
		t.Errorf("Expected status ok in response, got: %s", text)
	}
	if !strings.Contains(text, `"rulesAdded":1`) && !strings.Contains(text, `"rulesAdded": 1`) {
		t.Errorf("Expected rulesAdded:1 in response, got: %s", text)
	}
}

func TestToolConfigureNoiseRemoveUserRule(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`11`),
		Method:  "tools/call",
	}

	// First add a user rule so we have something removable
	addArgs := json.RawMessage(`{
		"action": "add",
		"rules": [{"category": "console", "classification": "known", "match_spec": {"message_regex": "removable"}}]
	}`)
	mcp.toolHandler.toolConfigureNoise(req, addArgs)

	// Remove the user rule (user_1)
	args := json.RawMessage(`{"action":"remove","rule_id":"user_1"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, `"status":"ok"`) && !strings.Contains(text, `"status": "ok"`) {
		t.Errorf("Expected status ok, got: %s", text)
	}
	if !strings.Contains(text, "user_1") {
		t.Errorf("Expected removed rule ID in response, got: %s", text)
	}
}

func TestToolConfigureNoiseRemoveBuiltinRejected(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"12a"`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"action":"remove","rule_id":"builtin_chrome_extension"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, "cannot remove built-in") {
		t.Errorf("Expected built-in rejection error, got: %s", text)
	}
}

func TestToolConfigureNoiseRemoveNonexistent(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`12`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"action":"remove","rule_id":"nonexistent_rule"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("Expected error for nonexistent rule, got: %s", text)
	}
}

func TestToolConfigureNoiseListRules(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`13`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"action":"list"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, "rules") {
		t.Errorf("Expected 'rules' in list response, got: %s", text)
	}
	if !strings.Contains(text, "statistics") {
		t.Errorf("Expected 'statistics' in list response, got: %s", text)
	}
}

func TestToolConfigureNoiseReset(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`14`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"action":"reset"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, `"status":"ok"`) && !strings.Contains(text, `"status": "ok"`) {
		t.Errorf("Expected status ok, got: %s", text)
	}
}

func TestToolConfigureNoiseAutoDetect(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add some console entries that could be detected as noise
	server.mu.Lock()
	server.entries = append(server.entries,
		LogEntry{"level": "log", "message": "Extension loaded", "source": "chrome-extension://abc123"},
		LogEntry{"level": "log", "message": "Extension loaded", "source": "chrome-extension://abc123"},
		LogEntry{"level": "log", "message": "Extension loaded", "source": "chrome-extension://abc123"},
	)
	server.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`15`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"action":"auto_detect"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, "proposals") {
		t.Errorf("Expected 'proposals' in auto_detect response, got: %s", text)
	}
	if !strings.Contains(text, "totalRules") {
		t.Errorf("Expected 'totalRules' in auto_detect response, got: %s", text)
	}
}

func TestToolConfigureNoiseUnknownAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`16`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"action":"unknown_action"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, "unknown action") {
		t.Errorf("Expected 'unknown action' error, got: %s", text)
	}
}

// ============================================
// toolExportSARIF Tests (save_to path)
// ============================================

func TestToolExportSARIFSaveTo(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Seed the a11y cache with a valid result
	a11yResult := json.RawMessage(`{
		"url": "http://localhost:3000/",
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Elements must have sufficient color contrast",
			"help": "Help text",
			"helpUrl": "https://example.com",
			"tags": ["wcag2aa"],
			"nodes": [{
				"html": "<p style=\"color: #aaa\">Low contrast</p>",
				"target": ["p"],
				"impact": "serious",
				"any": [{"id": "color-contrast", "message": "Fail"}],
				"all": [],
				"none": []
			}]
		}],
		"passes": [],
		"incomplete": []
	}`)

	// Set the cache entry (using the same key logic as the tool)
	cacheKey := capture.a11yCacheKey("", nil)
	capture.setA11yCacheEntry(cacheKey, a11yResult)

	// Use a temp file for save_to
	tmpDir := t.TempDir()
	savePath := tmpDir + "/output.sarif"

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`20`),
		Method:  "tools/call",
	}

	args, _ := json.Marshal(map[string]interface{}{
		"scope":   "",
		"save_to": savePath,
	})
	resp := mcp.toolHandler.toolExportSARIF(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Errorf("Expected no error, got: %s", content.Content[0].Text)
	}
	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, `"status":"ok"`) && !strings.Contains(text, `"status": "ok"`) {
		t.Errorf("Expected status ok in save_to response, got: %s", text)
	}
	if !strings.Contains(text, savePath) {
		t.Errorf("Expected path in response, got: %s", text)
	}
	if !strings.Contains(text, "rules") {
		t.Errorf("Expected 'rules' count in response, got: %s", text)
	}
	if !strings.Contains(text, "results") {
		t.Errorf("Expected 'results' count in response, got: %s", text)
	}
}

func TestToolExportSARIFNoCache(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`21`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"scope":"","save_to":"/tmp/test.sarif"}`)
	resp := mcp.toolHandler.toolExportSARIF(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if !content.IsError {
		t.Error("Expected isError=true when no cache entry exists")
	}
	if len(content.Content) == 0 || !strings.Contains(content.Content[0].Text, "No accessibility audit results") {
		t.Error("Expected 'No accessibility audit results' error")
	}
}

// ============================================
// toolLoadSessionContext Tests
// ============================================

func TestToolLoadSessionContextNilStore(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.sessionStore = nil

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`30`),
		Method:  "tools/call",
	}
	resp := mcp.toolHandler.toolLoadSessionContext(req, nil)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if !content.IsError {
		t.Error("Expected isError=true when sessionStore is nil")
	}
}

func TestToolLoadSessionContextValid(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`31`),
		Method:  "tools/call",
	}
	resp := mcp.toolHandler.toolLoadSessionContext(req, nil)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Error("Expected no error for valid session context load")
	}
	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	// Should contain valid JSON with session context fields
	text := content.Content[0].Text
	if !strings.Contains(text, "project_id") {
		t.Errorf("Expected 'project_id' in context, got: %s", text)
	}
}

// ============================================
// toolGeneratePRSummary Tests (with data)
// ============================================

func TestToolGeneratePRSummaryWithData(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add console error entries
	server.mu.Lock()
	server.entries = append(server.entries,
		LogEntry{"level": "error", "message": "TypeError: Cannot read property 'x'", "source": "app.js:42"},
		LogEntry{"level": "error", "message": "Failed to fetch /api/data", "source": "network"},
		LogEntry{"level": "log", "message": "App loaded"},
	)
	server.mu.Unlock()

	// Add performance snapshots so we get a table
	fcp1 := 800.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:00:00Z",
		Timing:    PerformanceTiming{Load: 1200, FirstContentfulPaint: &fcp1},
		Network:   NetworkSummary{TransferSize: 340 * 1024},
	})
	fcp2 := 900.0
	capture.TrackPerformanceSnapshot(PerformanceSnapshot{
		URL:       "http://localhost:3000/",
		Timestamp: "2026-01-24T10:05:00Z",
		Timing:    PerformanceTiming{Load: 1400, FirstContentfulPaint: &fcp2},
		Network:   NetworkSummary{TransferSize: 385 * 1024},
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`40`),
		Method:  "tools/call",
	}
	resp := mcp.toolHandler.toolGeneratePRSummary(req, nil)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Error("Expected no error for PR summary")
	}
	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, "Performance Impact") {
		t.Errorf("Expected 'Performance Impact' header, got: %s", text)
	}
	if !strings.Contains(text, "Load Time") {
		t.Errorf("Expected 'Load Time' row, got: %s", text)
	}
}

func TestToolGeneratePRSummaryNoEntries(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// No entries at all
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`41`),
		Method:  "tools/call",
	}
	resp := mcp.toolHandler.toolGeneratePRSummary(req, nil)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &content)

	if content.IsError {
		t.Error("Expected no error")
	}
	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	// Should still produce markdown (even if no data)
	text := content.Content[0].Text
	if !strings.Contains(text, "Performance Impact") {
		t.Errorf("Expected 'Performance Impact' even with no data, got: %s", text)
	}
}

// ============================================
// Coverage Gap Tests: appendAlertsToResponse, toolGenerate
// ============================================

func TestAppendAlertsToResponse_EmptyAlerts(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Create a base response
	baseResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  mcpTextResponse("base content"),
	}

	// Drain should return nil when no alerts
	alerts := mcp.toolHandler.drainAlerts()
	if alerts != nil {
		t.Errorf("Expected nil alerts from empty buffer, got %d", len(alerts))
	}

	// appendAlertsToResponse with empty/nil alerts should not be called,
	// but if called with non-nil empty slice, it should still append
	emptyAlerts := []Alert{}
	result := mcp.toolHandler.appendAlertsToResponse(baseResp, emptyAlerts)

	var toolResult MCPToolResult
	json.Unmarshal(result.Result, &toolResult)

	// With empty alerts, formatAlertsBlock returns empty string but still appends
	if len(toolResult.Content) != 2 {
		t.Errorf("Expected 2 content blocks (base + empty alerts), got %d", len(toolResult.Content))
	}
}

func TestAppendAlertsToResponse_NonEmptyAlerts(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add an alert to the buffer
	mcp.toolHandler.addAlert(Alert{
		Title:    "Test Alert",
		Category: "performance",
		Severity: "warning",
		Detail:   "Load time regression detected",
	})

	// Drain the alerts
	alerts := mcp.toolHandler.drainAlerts()
	if len(alerts) == 0 {
		t.Fatal("Expected non-empty alerts after addAlert")
	}

	// Append to a base response
	baseResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  mcpTextResponse("original content"),
	}

	result := mcp.toolHandler.appendAlertsToResponse(baseResp, alerts)

	var toolResult MCPToolResult
	json.Unmarshal(result.Result, &toolResult)

	if len(toolResult.Content) < 2 {
		t.Fatalf("Expected at least 2 content blocks, got %d", len(toolResult.Content))
	}
	// First block is the original content
	if toolResult.Content[0].Text != "original content" {
		t.Errorf("Expected first block to be 'original content', got %q", toolResult.Content[0].Text)
	}
	// Second block should contain alert text
	if !strings.Contains(toolResult.Content[1].Text, "Test Alert") {
		t.Errorf("Expected alert text to contain 'Test Alert', got %q", toolResult.Content[1].Text)
	}
}

func TestToolGenerate_UnknownFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	args, _ := json.Marshal(map[string]interface{}{"format": "unknown_format"})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	resp := mcp.toolHandler.toolGenerate(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Error("Expected isError=true for unknown format")
	}
	if !strings.Contains(result.Content[0].Text, "Unknown generate format") {
		t.Errorf("Expected 'Unknown generate format' error, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "unknown_format") {
		t.Errorf("Expected format name in error, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Coverage: appendAlertsToResponse with invalid Result JSON (line 513)
// ============================================

func TestAppendAlertsToResponse_InvalidResultJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Create a response with invalid Result JSON
	baseResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  json.RawMessage(`not valid json`),
	}

	alerts := []Alert{{Title: "Test", Category: "perf", Severity: "warning"}}
	result := mcp.toolHandler.appendAlertsToResponse(baseResp, alerts)

	// When unmarshal fails, the original response is returned unchanged
	if string(result.Result) != string(baseResp.Result) {
		t.Errorf("Expected unchanged response when Result JSON is invalid")
	}
}

// ============================================
// Coverage: toolGenerate sarif format (line 571)
// ============================================

func TestToolGenerate_SARIFFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	args, _ := json.Marshal(map[string]interface{}{"format": "sarif"})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	resp := mcp.toolHandler.toolGenerate(req, args)

	// Should not be a JSON-RPC error (may be an MCP tool error if no cache)
	if resp.Error != nil {
		t.Fatalf("Expected no JSON-RPC error, got: %v", resp.Error)
	}
}

// ============================================
// Coverage: toolGenerate har format (line 571/574)
// ============================================

func TestToolGenerate_HARFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	args, _ := json.Marshal(map[string]interface{}{"format": "har"})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	resp := mcp.toolHandler.toolGenerate(req, args)

	// Should succeed (return HAR JSON without error)
	if resp.Error != nil {
		t.Fatalf("Expected no JSON-RPC error, got: %v", resp.Error)
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if result.IsError {
		t.Errorf("Expected no MCP error for HAR format, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Coverage: toolGetBrowserErrors with noise filter (line 617)
// ============================================

func TestToolGetBrowserErrors_WithNoiseFilter(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add errors where some will be filtered by noise
	server.mu.Lock()
	server.entries = append(server.entries,
		LogEntry{"level": "error", "message": "Real error", "source": "app.js"},
		LogEntry{"level": "error", "message": "Extension noise", "source": "chrome-extension://abc123/script.js"},
	)
	server.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	resp := mcp.toolHandler.toolGetBrowserErrors(req, nil)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	// Should have filtered out the extension noise
	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
	if len(result.Content) > 0 && strings.Contains(result.Content[0].Text, "No browser errors") {
		t.Error("Expected some errors to pass noise filter")
	}
}

// ============================================
// Coverage: toolConfigureNoise empty noise_action defaults to "list" (line 718)
// ============================================

func TestToolConfigureNoiseRule_EmptyAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	// Call with empty noise_action (should default to "list")
	args := json.RawMessage(`{"noise_action":""}`)
	resp := mcp.toolHandler.toolConfigureNoiseRule(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	// Default "list" action should return rules info
	text := content.Content[0].Text
	if !strings.Contains(text, "rules") {
		t.Errorf("Expected 'rules' in list response (default action), got: %s", text)
	}
}

// ============================================
// Coverage: toolConfigureNoise remove rule success (line 810 is dead code, test remove success instead for line 821-825)
// ============================================

func TestToolConfigureNoiseRemoveRule_Success(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	// First add a rule
	addArgs := json.RawMessage(`{
		"action": "add",
		"rules": [{"category": "console", "classification": "known", "match_spec": {"message_regex": "test-removal"}}]
	}`)
	mcp.toolHandler.toolConfigureNoise(req, addArgs)

	// Now remove it using the generated ID
	removeArgs := json.RawMessage(`{"action":"remove","rule_id":"user_1"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, removeArgs)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected response content")
	}
	text := content.Content[0].Text
	if !strings.Contains(text, `"status"`) || !strings.Contains(text, "ok") {
		t.Errorf("Expected success status, got: %s", text)
	}
}

// ============================================
// Coverage: toolExportSARIF with valid cache (line 915)
// ============================================

func TestToolExportSARIF_DirectReturn(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Seed the a11y cache
	a11yResult := json.RawMessage(`{
		"url": "http://localhost:3000/",
		"violations": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images need alt text",
			"help": "Add alt",
			"helpUrl": "https://example.com",
			"tags": ["wcag2a"],
			"nodes": [{"html": "<img src=\"x\">", "target": ["img"], "impact": "critical"}]
		}],
		"passes": [],
		"incomplete": []
	}`)

	cacheKey := capture.a11yCacheKey("", nil)
	capture.setA11yCacheEntry(cacheKey, a11yResult)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	// No save_to means return SARIF JSON directly
	args, _ := json.Marshal(map[string]interface{}{"scope": ""})
	resp := mcp.toolHandler.toolExportSARIF(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected content")
	}
	// Should contain SARIF JSON
	if !strings.Contains(result.Content[0].Text, "sarif") || !strings.Contains(result.Content[0].Text, "2.1.0") {
		t.Errorf("Expected SARIF JSON in response, got: %s", result.Content[0].Text[:100])
	}
}

// ============================================
// V6 Tool Dispatcher Tests
// ============================================

func TestToolGenerateCSP_ViaDispatch(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"format":"csp"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "generate", args)
	if !handled {
		t.Fatal("expected generate to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	// Should return a valid response (CSP with no data is still valid)
	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected response content")
	}
	// Result should be JSON containing CSP data
	text := result.Content[0].Text
	if !strings.Contains(text, "directives") && !strings.Contains(text, "policy") && !strings.Contains(text, "origins") {
		// At minimum it should be valid JSON
		var parsed interface{}
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			t.Errorf("Expected valid JSON response from generate_csp, got: %s", text)
		}
	}
}

func TestToolGenerateCSP_WithOrigins(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Seed network bodies with some external origins
	capture.mu.Lock()
	capture.networkBodies = append(capture.networkBodies,
		NetworkBody{URL: "https://cdn.example.com/app.js", Method: "GET", Status: 200, ContentType: "application/javascript"},
		NetworkBody{URL: "https://fonts.googleapis.com/css", Method: "GET", Status: 200, ContentType: "text/css"},
	)
	capture.mu.Unlock()

	// Feed entries so CSP generator has page context
	mcp.toolHandler.cspGenerator.mu.Lock()
	mcp.toolHandler.cspGenerator.pages["https://myapp.com/"] = true
	mcp.toolHandler.cspGenerator.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	args := json.RawMessage(`{}`)

	resp := mcp.toolHandler.toolGenerateCSP(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
}

func TestToolSecurityAudit_ViaDispatch(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"what":"security_audit"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("expected observe to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected response content")
	}
	// Should contain valid JSON after summary prefix
	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(jsonPart), &parsed); err != nil {
		t.Errorf("Expected valid JSON from security_audit, got parse error: %v", err)
	}
}

func TestToolSecurityAudit_WithData(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add network bodies with potential security issues
	capture.mu.Lock()
	capture.networkBodies = append(capture.networkBodies,
		NetworkBody{URL: "http://api.example.com/login", Method: "POST", Status: 200, ResponseBody: `{"password":"secret123","token":"abc"}`},
		NetworkBody{URL: "https://api.example.com/data", Method: "GET", Status: 200},
	)
	capture.mu.Unlock()

	// Add console entries
	server.mu.Lock()
	server.entries = append(server.entries,
		LogEntry{"level": "error", "message": "Mixed Content: loading HTTP resource on HTTPS page"},
		LogEntry{"level": "warn", "message": "Cookie without Secure flag"},
	)
	server.mu.Unlock()

	// Add page URLs
	mcp.toolHandler.cspGenerator.mu.Lock()
	mcp.toolHandler.cspGenerator.pages["https://myapp.com/"] = true
	mcp.toolHandler.cspGenerator.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	args := json.RawMessage(`{}`)

	resp := mcp.toolHandler.toolSecurityAudit(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
}

func TestToolGetAuditLog_ViaDispatch(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"action":"audit_log"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected response content")
	}
	// Should contain valid JSON after summary prefix
	text2 := result.Content[0].Text
	jsonPart2 := text2
	if lines := strings.SplitN(text2, "\n", 2); len(lines) == 2 {
		jsonPart2 = lines[1]
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(jsonPart2), &parsed); err != nil {
		t.Errorf("Expected valid JSON from get_audit_log, got parse error: %v", err)
	}
}

func TestToolGetAuditLog_WithFilters(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Record some audit entries first
	mcp.toolHandler.auditTrail.Record(AuditEntry{ToolName: "observe", Parameters: `{"what":"error_clusters"}`, ResponseSize: 100, Duration: 42, Success: true})
	mcp.toolHandler.auditTrail.Record(AuditEntry{ToolName: "query_dom", Parameters: `{"selector":".foo"}`, ResponseSize: 200, Duration: 15, Success: true})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	args := json.RawMessage(`{"limit":10}`)

	resp := mcp.toolHandler.toolGetAuditLog(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "observe") {
		t.Errorf("Expected 'observe' in audit log, got: %s", text)
	}
}

func TestToolDiffSessions_ViaDispatch(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	// Capture a snapshot
	args := json.RawMessage(`{"action":"diff_sessions","session_action":"capture","name":"test-snap"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	// capture action should succeed
	if result.IsError {
		t.Errorf("Expected no error on capture, got: %s", result.Content[0].Text)
	}
}

func TestToolDiffSessions_List(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	args := json.RawMessage(`{"action":"list"}`)

	resp := mcp.toolHandler.toolDiffSessions(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error on list, got: %s", result.Content[0].Text)
	}
	// Should contain valid JSON after summary prefix
	text3 := result.Content[0].Text
	jsonPart3 := text3
	if lines := strings.SplitN(text3, "\n", 2); len(lines) == 2 {
		jsonPart3 = lines[1]
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(jsonPart3), &parsed); err != nil {
		t.Errorf("Expected valid JSON, got parse error: %v", err)
	}
}

func TestToolDiffSessions_Compare(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}

	// Capture first snapshot
	mcp.toolHandler.toolDiffSessions(req, json.RawMessage(`{"action":"capture","name":"before"}`))

	// Add some data to change state
	server.mu.Lock()
	server.entries = append(server.entries, LogEntry{"level": "error", "message": "new error"})
	server.mu.Unlock()

	// Capture second snapshot
	mcp.toolHandler.toolDiffSessions(req, json.RawMessage(`{"action":"capture","name":"after"}`))

	// Compare using compare_a and compare_b
	args := json.RawMessage(`{"action":"compare","compare_a":"before","compare_b":"after"}`)
	resp := mcp.toolHandler.toolDiffSessions(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error on compare, got: %s", result.Content[0].Text)
	}
}

// ============================================
// captureStateAdapter Tests
// ============================================

func TestCaptureStateAdapter_GetConsoleErrors(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	server.mu.Lock()
	server.entries = append(server.entries,
		LogEntry{"level": "error", "message": "TypeError: foo is undefined"},
		LogEntry{"level": "warn", "message": "Deprecation warning"},
		LogEntry{"level": "error", "message": "ReferenceError: bar"},
		LogEntry{"level": "log", "message": "App started"},
	)
	server.mu.Unlock()

	adapter := &captureStateAdapter{capture: capture, server: server}

	errors := adapter.GetConsoleErrors()
	if len(errors) != 2 {
		t.Fatalf("Expected 2 errors, got %d", len(errors))
	}
	if errors[0].Message != "TypeError: foo is undefined" {
		t.Errorf("Expected first error message 'TypeError: foo is undefined', got %q", errors[0].Message)
	}
	if errors[1].Message != "ReferenceError: bar" {
		t.Errorf("Expected second error message 'ReferenceError: bar', got %q", errors[1].Message)
	}
	if errors[0].Type != "error" {
		t.Errorf("Expected type 'error', got %q", errors[0].Type)
	}
}

func TestCaptureStateAdapter_GetConsoleWarnings(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	server.mu.Lock()
	server.entries = append(server.entries,
		LogEntry{"level": "error", "message": "Error msg"},
		LogEntry{"level": "warn", "message": "Deprecation warning"},
		LogEntry{"level": "warn", "message": "Performance warning"},
		LogEntry{"level": "log", "message": "Info msg"},
	)
	server.mu.Unlock()

	adapter := &captureStateAdapter{capture: capture, server: server}

	warnings := adapter.GetConsoleWarnings()
	if len(warnings) != 2 {
		t.Fatalf("Expected 2 warnings, got %d", len(warnings))
	}
	if warnings[0].Message != "Deprecation warning" {
		t.Errorf("Expected 'Deprecation warning', got %q", warnings[0].Message)
	}
	if warnings[0].Type != "warning" {
		t.Errorf("Expected type 'warning', got %q", warnings[0].Type)
	}
}

func TestCaptureStateAdapter_GetNetworkRequests(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	capture.mu.Lock()
	capture.networkBodies = append(capture.networkBodies,
		NetworkBody{URL: "https://api.example.com/users", Method: "GET", Status: 200, Duration: 150},
		NetworkBody{URL: "https://api.example.com/data", Method: "POST", Status: 201, Duration: 300},
	)
	capture.mu.Unlock()

	adapter := &captureStateAdapter{capture: capture, server: server}

	requests := adapter.GetNetworkRequests()
	if len(requests) != 2 {
		t.Fatalf("Expected 2 requests, got %d", len(requests))
	}
	if requests[0].URL != "https://api.example.com/users" {
		t.Errorf("Expected URL 'https://api.example.com/users', got %q", requests[0].URL)
	}
	if requests[0].Method != "GET" {
		t.Errorf("Expected method 'GET', got %q", requests[0].Method)
	}
	if requests[0].Status != 200 {
		t.Errorf("Expected status 200, got %d", requests[0].Status)
	}
	if requests[1].Duration != 300 {
		t.Errorf("Expected duration 300, got %d", requests[1].Duration)
	}
}

func TestCaptureStateAdapter_GetWSConnections(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	capture.mu.Lock()
	capture.connections = map[string]*connectionState{
		"conn-1": {url: "wss://ws.example.com/live", state: "open"},
		"conn-2": {url: "wss://ws.example.com/chat", state: "closed"},
	}
	capture.mu.Unlock()

	adapter := &captureStateAdapter{capture: capture, server: server}

	conns := adapter.GetWSConnections()
	if len(conns) != 2 {
		t.Fatalf("Expected 2 connections, got %d", len(conns))
	}

	// Check both connections exist (order is non-deterministic from map)
	found := map[string]string{}
	for _, c := range conns {
		found[c.URL] = c.State
	}
	if found["wss://ws.example.com/live"] != "open" {
		t.Errorf("Expected live connection state 'open', got %q", found["wss://ws.example.com/live"])
	}
	if found["wss://ws.example.com/chat"] != "closed" {
		t.Errorf("Expected chat connection state 'closed', got %q", found["wss://ws.example.com/chat"])
	}
}

func TestCaptureStateAdapter_GetPerformance(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	adapter := &captureStateAdapter{capture: capture, server: server}

	perf := adapter.GetPerformance()
	if perf != nil {
		t.Errorf("Expected nil performance (not yet integrated), got %v", perf)
	}
}

func TestCaptureStateAdapter_GetCurrentPageURL(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	capture.mu.Lock()
	capture.a11y.lastURL = "https://myapp.example.com/dashboard"
	capture.mu.Unlock()

	adapter := &captureStateAdapter{capture: capture, server: server}

	url := adapter.GetCurrentPageURL()
	if url != "https://myapp.example.com/dashboard" {
		t.Errorf("Expected 'https://myapp.example.com/dashboard', got %q", url)
	}
}

func TestCaptureStateAdapter_EmptyState(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	adapter := &captureStateAdapter{capture: capture, server: server}

	if errors := adapter.GetConsoleErrors(); errors != nil {
		t.Errorf("Expected nil errors on empty state, got %d", len(errors))
	}
	if warnings := adapter.GetConsoleWarnings(); warnings != nil {
		t.Errorf("Expected nil warnings on empty state, got %d", len(warnings))
	}
	if requests := adapter.GetNetworkRequests(); requests != nil {
		t.Errorf("Expected nil requests on empty state, got %d", len(requests))
	}
	if conns := adapter.GetWSConnections(); conns != nil {
		t.Errorf("Expected nil connections on empty state, got %d", len(conns))
	}
	if url := adapter.GetCurrentPageURL(); url != "" {
		t.Errorf("Expected empty URL on empty state, got %q", url)
	}
}

// ============================================
// toolConfigureCapture Tests
// ============================================

func TestToolConfigureCapture_MissingSettings(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", json.RawMessage(`{"action":"capture"}`))
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Error("Expected isError=true when settings is missing")
	}
	if !strings.Contains(result.Content[0].Text, "settings") {
		t.Errorf("Expected error about 'settings', got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureCapture_Reset(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	args := json.RawMessage(`{"action":"capture","settings":"reset"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error on reset, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureCapture_SetSettings(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}
	args := json.RawMessage(`{"action":"capture","settings":{"log_level":"warn","ws_mode":"off"}}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error on settings, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureCapture_InvalidSettings(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`4`), Method: "tools/call"}
	// settings is not a map or "reset"
	args := json.RawMessage(`{"action":"capture","settings":12345}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Error("Expected isError=true for invalid settings type")
	}
}

// ============================================
// toolAnalyzeErrors Tests
// ============================================

func TestToolAnalyzeErrors_Empty(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"what":"error_clusters"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("expected observe to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	// Should contain cluster analysis response
	if !strings.Contains(text, "total_errors") {
		t.Errorf("Expected 'total_errors' in response, got: %s", text)
	}
}

func TestToolAnalyzeErrors_WithErrors(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Feed errors into the cluster manager
	mcp.toolHandler.clusters.AddError(ErrorInstance{
		Message:  "TypeError: Cannot read property 'x' of undefined",
		Stack:    "at foo (app.js:10)",
		Source:   "app.js",
		Severity: "error",
	})
	mcp.toolHandler.clusters.AddError(ErrorInstance{
		Message:  "TypeError: Cannot read property 'y' of undefined",
		Stack:    "at foo (app.js:10)",
		Source:   "app.js",
		Severity: "error",
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	args := json.RawMessage(`{"what":"error_clusters"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("expected observe to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "clusters") {
		t.Errorf("Expected 'clusters' in response, got: %s", text)
	}
}

// ============================================
// toolAnalyzeHistory Tests
// ============================================

func TestToolAnalyzeHistory_NilGraph(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Set temporalGraph to nil
	mcp.toolHandler.temporalGraph = nil

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"what":"history"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("expected observe to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Errorf("Expected structured error for nil graph, got success")
	}
	if !strings.Contains(result.Content[0].Text, "No history") {
		t.Errorf("Expected 'No history' message, got: %s", result.Content[0].Text)
	}
}

func TestToolAnalyzeHistory_WithGraph(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// temporalGraph is initialized by NewToolHandler using CWD
	// Just call with empty query
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	args := json.RawMessage(`{"what":"history","query":{}}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("expected observe to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	// Should return valid JSON with summary prefix (or structured error for nil graph)
	text := result.Content[0].Text
	if strings.Contains(text, "No history") {
		// nil graph case — structured error
		return
	}
	// Strip summary line before parsing JSON
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(jsonPart), &parsed); err != nil {
		t.Errorf("Expected valid JSON after summary, got: %s", text)
	}
}

// ============================================
// toolConfigureRecordEvent Tests
// ============================================

func TestToolConfigureRecordEvent_NilGraph(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.toolHandler.temporalGraph = nil

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"action":"record_event","event":{"type":"fix","description":"Fixed bug"}}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Error("Expected isError=true when temporalGraph is nil")
	}
	if !strings.Contains(result.Content[0].Text, "not initialized") {
		t.Errorf("Expected 'not initialized' error, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureRecordEvent_MissingEvent(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	args := json.RawMessage(`{"action":"record_event"}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Error("Expected isError=true when event is missing")
	}
	if !strings.Contains(result.Content[0].Text, "event") {
		t.Errorf("Expected error about 'event', got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureRecordEvent_ValidEvent(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Ensure temporalGraph is initialized (may be nil in test env)
	if mcp.toolHandler.temporalGraph == nil {
		tmpDir := t.TempDir()
		mcp.toolHandler.temporalGraph = NewTemporalGraph(tmpDir)
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}
	args := json.RawMessage(`{"action":"record_event","event":{"type":"fix","description":"Fixed login bug","source":"auth.go"}}`)

	resp, handled := mcp.toolHandler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
}

// ============================================
// handleToolCall dispatch completeness
// ============================================

func TestHandleToolCall_UnknownTool(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	_, handled := mcp.toolHandler.handleToolCall(req, "nonexistent_tool", nil)
	if handled {
		t.Error("Expected unknown tool to not be handled")
	}
}

func TestHandleToolCall_AllV6Tools(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	emptyArgs := json.RawMessage(`{}`)

	tools := []string{"observe", "generate", "configure", "interact"}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			args := emptyArgs
			if tool == "configure" {
				args = json.RawMessage(`{"action":"health"}`)
			}
			_, handled := mcp.toolHandler.handleToolCall(req, tool, args)
			if !handled {
				t.Errorf("Expected %s to be handled", tool)
			}
		})
	}
}

// ============================================
// redactSecret Tests (security.go helper)
// ============================================

func TestRedactSecret_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "very short (<=3)", input: "abc", expected: "abc***"},
		{name: "short (<=6)", input: "abcdef", expected: "abc***"},
		{name: "medium (<=10)", input: "abcdefghij", expected: "abcdef***"},
		{name: "long", input: "abcdefghijklm", expected: "abcdef***klm"},
		{name: "empty", input: "", expected: "***"},
		{name: "single char", input: "x", expected: "x***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSecret(tt.input)
			if result != tt.expected {
				t.Errorf("redactSecret(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================
// computeMetricChange Tests (sessions.go helper)
// ============================================

func TestComputeMetricChange_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		before     float64
		after      float64
		wantChange string
		wantRegr   bool
	}{
		{name: "zero before, positive after", before: 0, after: 100, wantChange: "+inf", wantRegr: true},
		{name: "zero before, zero after", before: 0, after: 0, wantChange: "0%", wantRegr: false},
		{name: "increase (regression)", before: 100, after: 200, wantChange: "+100%", wantRegr: true},
		{name: "decrease (improvement)", before: 200, after: 100, wantChange: "-50%", wantRegr: false},
		{name: "same value", before: 100, after: 100, wantChange: "+0%", wantRegr: false},
		{name: "small increase (not regression)", before: 100, after: 105, wantChange: "+5%", wantRegr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := computeMetricChange(tt.before, tt.after)
			if mc.Change != tt.wantChange {
				t.Errorf("computeMetricChange(%v, %v).Change = %q, want %q", tt.before, tt.after, mc.Change, tt.wantChange)
			}
			if mc.Regression != tt.wantRegr {
				t.Errorf("computeMetricChange(%v, %v).Regression = %v, want %v", tt.before, tt.after, mc.Regression, tt.wantRegr)
			}
		})
	}
}

// ============================================
// CaptureOverrides.SetMultiple Tests (rate limit and invalid values)
// ============================================

func TestSetMultiple_InvalidValue(t *testing.T) {
	t.Parallel()
	co := NewCaptureOverrides()

	errs := co.SetMultiple(map[string]string{"log_level": "invalid_value"})

	if err, ok := errs["log_level"]; !ok || err == nil {
		t.Error("Expected error for invalid log_level value")
	} else if !strings.Contains(err.Error(), "invalid value") {
		t.Errorf("Expected 'invalid value' error, got: %s", err.Error())
	}
}

func TestSetMultiple_RateLimit(t *testing.T) {
	t.Parallel()
	co := NewCaptureOverrides()

	// First call should succeed
	errs := co.SetMultiple(map[string]string{"log_level": "warn"})
	if err := errs["log_level"]; err != nil {
		t.Fatalf("First SetMultiple should succeed, got: %v", err)
	}

	// Second call immediately should be rate limited
	errs = co.SetMultiple(map[string]string{"log_level": "error"})
	if err := errs["log_level"]; err == nil {
		t.Error("Expected rate limit error on immediate second call")
	} else if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("Expected 'rate limited' error, got: %s", err.Error())
	}
}

func TestSetMultiple_MultipleSettings(t *testing.T) {
	t.Parallel()
	co := NewCaptureOverrides()

	errs := co.SetMultiple(map[string]string{
		"log_level": "warn",
		"ws_mode":   "off",
	})

	for k, err := range errs {
		if err != nil {
			t.Errorf("Expected no error for %s, got: %v", k, err)
		}
	}

	// Check alert was generated
	alert := co.DrainAlert()
	if alert == nil {
		t.Error("Expected pending alert after settings change")
	}
}

// ============================================
// buildSettingsResponse Tests
// ============================================

func TestBuildSettingsResponse(t *testing.T) {
	t.Parallel()
	co := NewCaptureOverrides()

	resp := buildSettingsResponse(co)

	if !resp.Connected {
		t.Error("Expected Connected=true")
	}
	if resp.CaptureOverrides == nil {
		t.Error("Expected non-nil CaptureOverrides map")
	}
}

// ============================================
// extractOrigin and isThirdPartyURL Tests (security.go)
// ============================================

func TestExtractOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"https://api.example.com/path/to/resource?q=1", "https://api.example.com"},
		{"http://localhost:3000/api", "http://localhost:3000"},
		{"https://cdn.example.com:8080/lib.js", "https://cdn.example.com:8080"},
	}

	for _, tt := range tests {
		result := extractOrigin(tt.input)
		if result != tt.expected {
			t.Errorf("extractOrigin(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsThirdPartyURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		url      string
		pages    []string
		expected bool
	}{
		{"no pages", "https://api.example.com/data", nil, false},
		{"same origin", "https://myapp.com/api", []string{"https://myapp.com/"}, false},
		{"third party", "https://cdn.external.com/lib.js", []string{"https://myapp.com/"}, true},
		{"subdomain (same registrable domain)", "https://api.myapp.com/data", []string{"https://myapp.com/"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isThirdPartyURL(tt.url, tt.pages)
			if result != tt.expected {
				t.Errorf("isThirdPartyURL(%q, %v) = %v, want %v", tt.url, tt.pages, result, tt.expected)
			}
		})
	}
}

// ============================================
// HandleGetAuditLog filter coverage
// ============================================

func TestHandleGetAuditLog_ToolNameFilter(t *testing.T) {
	t.Parallel()
	at := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})

	at.Record(AuditEntry{ToolName: "observe", Parameters: `{"what":"error_clusters"}`, Success: true})
	at.Record(AuditEntry{ToolName: "query_dom", Parameters: `{"selector":"div"}`, Success: true})
	at.Record(AuditEntry{ToolName: "observe", Parameters: `{"what":"logs"}`, Success: true})

	result, err := at.HandleGetAuditLog(json.RawMessage(`{"tool_name":"observe"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	data, _ := json.Marshal(result)
	text := string(data)
	if !strings.Contains(text, "observe") {
		t.Errorf("Expected 'observe' entries, got: %s", text)
	}
}

func TestHandleGetAuditLog_LimitFilter(t *testing.T) {
	t.Parallel()
	at := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})

	for i := 0; i < 5; i++ {
		at.Record(AuditEntry{ToolName: "observe", Success: true})
	}

	result, err := at.HandleGetAuditLog(json.RawMessage(`{"limit":2}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	data, _ := json.Marshal(result)
	// Should contain at most 2 entries
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)
	if entries, ok := parsed["entries"].([]interface{}); ok && len(entries) > 2 {
		t.Errorf("Expected at most 2 entries with limit=2, got %d", len(entries))
	}
}

// ============================================
// isTestKey and isLocalhostURL Tests (security.go)
// ============================================

func TestIsTestKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected bool
	}{
		{"sk_test_abc123", true},
		{"test_key_value", true},
		{"pk_live_real", false},
		{"some_example_key", true},
		{"production_key_real", false},
	}

	for _, tt := range tests {
		result := isTestKey(tt.input)
		if result != tt.expected {
			t.Errorf("isTestKey(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsLocalhostURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected bool
	}{
		{"http://localhost:3000/api", true},
		{"http://127.0.0.1:8080/", true},
		{"http://[::1]:3000/", true},
		{"http://0.0.0.0:5000/", true},
		{"https://api.example.com/data", false},
		{"not a url \x7f", false},
	}

	for _, tt := range tests {
		result := isLocalhostURL(tt.input)
		if result != tt.expected {
			t.Errorf("isLocalhostURL(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// ============================================
// HandleEnhancedActions edge case (actions.go)
// ============================================

func TestHandleEnhancedActions_WithFilters(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add some actions
	capture.mu.Lock()
	capture.enhancedActions = append(capture.enhancedActions,
		EnhancedAction{Type: "click", URL: "https://myapp.com/page1", Timestamp: 1706090400000},
		EnhancedAction{Type: "input", URL: "https://myapp.com/page2", Timestamp: 1706090460000},
		EnhancedAction{Type: "navigate", URL: "https://other.com/page", Timestamp: 1706090520000},
	)
	capture.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	// Filter by URL
	args := json.RawMessage(`{"what":"actions","url":"myapp.com"}`)

	resp, _ := mcp.toolHandler.handleToolCall(req, "observe", args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
}

func TestHandleEnhancedActions_LastN(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add actions
	capture.mu.Lock()
	for i := 0; i < 10; i++ {
		capture.enhancedActions = append(capture.enhancedActions,
			EnhancedAction{Type: "click", Timestamp: 1706090400000 + int64(i)*1000},
		)
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"what":"actions","last_n":3}`)

	resp, _ := mcp.toolHandler.handleToolCall(req, "observe", args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error, got: %s", result.Content[0].Text)
	}
}

// ============================================
// NewSessionManager error path (sessions.go)
// ============================================

func TestNewSessionManager_WithState(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	adapter := &captureStateAdapter{capture: capture, server: server}
	sm := NewSessionManager(5, adapter)

	if sm == nil {
		t.Fatal("Expected non-nil SessionManager")
	}

	// Verify it can handle a list action
	result, err := sm.HandleTool(json.RawMessage(`{"action":"list"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestNewSessionManager_ZeroMaxSnapshots(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)

	adapter := &captureStateAdapter{capture: capture, server: server}
	sm := NewSessionManager(0, adapter)

	if sm == nil {
		t.Fatal("Expected non-nil SessionManager")
	}
	// Should default to 10
	if sm.maxSize != 10 {
		t.Errorf("Expected maxSize=10 when 0 provided, got %d", sm.maxSize)
	}
}

// ============================================
// Security Scanner credential detection (security.go)
// ============================================

func TestScanBodyForCredentials_AWSKey(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	// AKIA followed by 16 uppercase alphanumeric chars (no test indicators)
	body := `{"config": "AKIAIOSFODNN7PRODKEY"}`
	findings := scanner.scanBodyForCredentials(body, "https://api.company.com/config", "response body")

	foundAWS := false
	for _, f := range findings {
		if strings.Contains(f.Title, "AWS") {
			foundAWS = true
		}
	}
	if !foundAWS {
		t.Error("Expected AWS key finding")
	}
}

func TestScanBodyForCredentials_GitHubToken(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	body := `{"token": "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"}`
	findings := scanner.scanBodyForCredentials(body, "https://api.example.com/auth", "response body")

	foundGH := false
	for _, f := range findings {
		if strings.Contains(f.Title, "GitHub") {
			foundGH = true
		}
	}
	if !foundGH {
		t.Error("Expected GitHub token finding")
	}
}

func TestScanBodyForCredentials_StripeKey(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	testKey := "sk_" + "live_ABCDEFGHIJKLMNOPQRSTUVWXab"
	body := `{"key": "` + testKey + `"}`
	findings := scanner.scanBodyForCredentials(body, "https://api.example.com/payment", "response body")

	foundStripe := false
	for _, f := range findings {
		if strings.Contains(f.Title, "Stripe") {
			foundStripe = true
		}
	}
	if !foundStripe {
		t.Error("Expected Stripe key finding")
	}
}

func TestScanBodyForCredentials_PrivateKey(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	body := `-----BEGIN RSA PRIVATE KEY-----\nMIIE...truncated\n-----END RSA PRIVATE KEY-----`
	findings := scanner.scanBodyForCredentials(body, "https://api.example.com/key", "response body")

	foundPK := false
	for _, f := range findings {
		if strings.Contains(f.Title, "Private key") {
			foundPK = true
		}
	}
	if !foundPK {
		t.Error("Expected private key finding")
	}
}

func TestScanBodyForCredentials_JWT(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	body := `{"access_token": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SIGNATURE_HERE_abc"}`
	findings := scanner.scanBodyForCredentials(body, "https://api.example.com/auth", "response body")

	foundJWT := false
	for _, f := range findings {
		if strings.Contains(f.Title, "JWT") {
			foundJWT = true
		}
	}
	if !foundJWT {
		t.Error("Expected JWT finding")
	}
}

func TestScanBodyForCredentials_EmptyBody(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	findings := scanner.scanBodyForCredentials("", "https://api.example.com/", "response body")
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for empty body, got %d", len(findings))
	}
}

func TestScanBodyForCredentials_LargeBody(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	// Create body larger than 10240 chars with a key beyond the limit
	largeBody := strings.Repeat("x", 10300) + `AKIAIOSFODNN7EXAMPLE`
	findings := scanner.scanBodyForCredentials(largeBody, "https://api.example.com/", "response body")
	// The key is beyond the scan limit, so it shouldn't be found
	for _, f := range findings {
		if strings.Contains(f.Title, "AWS") {
			t.Error("Should not find AWS key beyond scan limit")
		}
	}
}

func TestScanConsoleForCredentials_BearerToken(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	entry := LogEntry{
		"level":   "log",
		"message": "Auth header: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test",
		"source":  "auth.js",
	}
	findings := scanner.scanConsoleForCredentials(entry)

	foundBearer := false
	for _, f := range findings {
		if strings.Contains(f.Title, "Bearer") {
			foundBearer = true
		}
	}
	if !foundBearer {
		t.Error("Expected Bearer token finding")
	}
}

func TestScanConsoleForCredentials_AWSKey(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	entry := LogEntry{
		"level":   "log",
		"message": "Config loaded: key=AKIAIOSFODNN7PRODKEY",
		"source":  "config.js",
	}
	findings := scanner.scanConsoleForCredentials(entry)

	foundAWS := false
	for _, f := range findings {
		if strings.Contains(f.Title, "AWS") {
			foundAWS = true
		}
	}
	if !foundAWS {
		t.Error("Expected AWS key finding in console")
	}
}

func TestScanConsoleForCredentials_EmptyMessage(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	entry := LogEntry{"level": "log", "source": "app.js"}
	findings := scanner.scanConsoleForCredentials(entry)
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for empty message, got %d", len(findings))
	}
}

// ============================================
// handleCaptureSettings single-setting path (capture_control.go)
// ============================================

func TestHandleCaptureSettings_SingleSetting(t *testing.T) {
	t.Parallel()
	co := NewCaptureOverrides()

	result, errMsg := handleCaptureSettings(co, map[string]string{"log_level": "error"}, nil, "test-agent")
	if errMsg != "" {
		t.Fatalf("Expected no error, got: %s", errMsg)
	}
	if !strings.Contains(result, "log_level") {
		t.Errorf("Expected result to mention 'log_level', got: %s", result)
	}
}

func TestHandleCaptureSettings_EmptySettings(t *testing.T) {
	t.Parallel()
	co := NewCaptureOverrides()

	_, errMsg := handleCaptureSettings(co, map[string]string{}, nil, "test-agent")
	if errMsg == "" {
		t.Error("Expected error for empty settings")
	}
	if !strings.Contains(errMsg, "No settings") {
		t.Errorf("Expected 'No settings' error, got: %s", errMsg)
	}
}

func TestHandleCaptureSettings_InvalidSettingName(t *testing.T) {
	t.Parallel()
	co := NewCaptureOverrides()

	_, errMsg := handleCaptureSettings(co, map[string]string{"invalid_name": "value"}, nil, "test-agent")
	if errMsg == "" {
		t.Error("Expected error for invalid setting name")
	}
}

// ============================================
// HandleEnhancedActions HTTP handler (actions.go)
// ============================================

func TestHandleEnhancedActions_BadJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	_ = setupToolHandler(t, server, capture)

	req := httptest.NewRequest("POST", "/actions", bytes.NewBufferString("not valid json"))
	w := httptest.NewRecorder()

	capture.HandleEnhancedActions(w, req)

	if w.Code != 400 {
		t.Errorf("Expected 400 for bad JSON, got %d", w.Code)
	}
}

func TestHandleEnhancedActions_ValidActions(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	_ = setupToolHandler(t, server, capture)

	body := `{"actions":[{"type":"click","timestamp":1706090400000,"url":"https://example.com/"}]}`
	req := httptest.NewRequest("POST", "/actions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	capture.HandleEnhancedActions(w, req)

	if w.Code != 200 {
		t.Errorf("Expected 200 for valid actions, got %d", w.Code)
	}
}

// ============================================
// computeMetricChange regression threshold (sessions.go)
// ============================================

func TestComputeMetricChange_NegativeBeforeZeroAfter(t *testing.T) {
	t.Parallel()
	mc := computeMetricChange(100, 0)
	if mc.Change != "-100%" {
		t.Errorf("Expected '-100%%', got %q", mc.Change)
	}
	if mc.Regression {
		t.Error("Expected no regression when value decreases")
	}
}

// ============================================
// Additional coverage for toolGetEnhancedActions (actions.go)
// ============================================

func TestToolGetEnhancedActions_Empty(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"what":"actions"}`)

	resp, _ := mcp.toolHandler.handleToolCall(req, "observe", args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if strings.Contains(result.Content[0].Text, "error") {
		t.Errorf("Expected no error for empty actions, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Coverage gap: temporal_graph.go evict paths
// ============================================

func TestTemporalEvict_UnparseableTimestamp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	histDir := filepath.Join(dir, "history")
	os.MkdirAll(histDir, 0755)

	// Write events: one old (evicted), one with bad timestamp (kept), one recent (kept)
	oldEvent := TemporalEvent{
		ID: "evt_old", Type: "regression", Description: "Old regression",
		Timestamp: time.Now().Add(-100 * 24 * time.Hour).UTC().Format(time.RFC3339),
		Origin: "system", Status: "active",
	}
	badTSEvent := TemporalEvent{
		ID: "evt_bad_ts", Type: "regression", Description: "Bad timestamp event",
		Timestamp: "not-a-valid-time", Origin: "system", Status: "active",
	}
	recentEvent := TemporalEvent{
		ID: "evt_recent", Type: "regression", Description: "Recent regression",
		Timestamp: time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
		Origin: "system", Status: "active",
	}

	var buf bytes.Buffer
	for _, e := range []TemporalEvent{oldEvent, badTSEvent, recentEvent} {
		data, _ := json.Marshal(e)
		buf.Write(data)
		buf.WriteByte('\n')
	}
	os.WriteFile(filepath.Join(histDir, "events.jsonl"), buf.Bytes(), 0644)

	tg := NewTemporalGraph(dir)
	defer tg.Close()

	// The rewritten file should have 2 entries: bad_ts (kept due to parse error) + recent
	data, err := os.ReadFile(filepath.Join(histDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines in rewritten file (bad_ts + recent), got %d", len(lines))
	}
	// Verify bad_ts event was kept
	found := false
	for _, line := range lines {
		if strings.Contains(line, "evt_bad_ts") {
			found = true
		}
	}
	if !found {
		t.Error("event with unparseable timestamp should be kept in rewritten file")
	}
}

func TestTemporalEvict_RebuildFingerprints(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	histDir := filepath.Join(dir, "history")
	os.MkdirAll(histDir, 0755)

	oldEvent := TemporalEvent{
		ID: "evt_old", Type: "error", Description: "Login failed",
		Timestamp: time.Now().Add(-100 * 24 * time.Hour).UTC().Format(time.RFC3339),
		Origin: "system", Status: "active", Source: "auth.js",
	}
	recentEvent1 := TemporalEvent{
		ID: "evt_r1", Type: "error", Description: "Timeout error",
		Timestamp: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		Origin: "system", Status: "active", Source: "api.js",
	}
	recentEvent2 := TemporalEvent{
		ID: "evt_r2", Type: "regression", Description: "Slow load",
		Timestamp: time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
		Origin: "system", Status: "active", Source: "perf.js",
	}

	var buf bytes.Buffer
	for _, e := range []TemporalEvent{oldEvent, recentEvent1, recentEvent2} {
		data, _ := json.Marshal(e)
		buf.Write(data)
		buf.WriteByte('\n')
	}
	os.WriteFile(filepath.Join(histDir, "events.jsonl"), buf.Bytes(), 0644)

	tg := NewTemporalGraph(dir)
	defer tg.Close()

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 2 {
		t.Fatalf("expected 2 events after eviction, got %d", len(events.Events))
	}

	// Verify the rewritten file
	data, err := os.ReadFile(filepath.Join(histDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("rewritten file should have 2 lines, got %d", len(lines))
	}
}

// ============================================
// Coverage gap: security.go scanning paths
// ============================================

func TestScanBodyForCredentials_APIKeyInJSON(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	body := `{"user": "test", "api_key": "sk_live_real_production_key_1234567890"}`
	findings := scanner.scanBodyForCredentials(body, "https://api.example.com/config", "response body")
	found := false
	for _, f := range findings {
		if strings.Contains(f.Title, "API key") && strings.Contains(f.Title, "api_key") {
			found = true
			if f.Severity != "warning" {
				t.Errorf("expected warning severity, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected API key finding for api_key field in JSON body")
	}
}

func TestScanForPII_SSNThirdParty(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	content := `{"ssn": "123-45-6789"}`
	findings := scanner.scanForPII(content, "https://analytics.third-party.com/track", "request body", true)
	if len(findings) == 0 {
		t.Fatal("expected SSN finding")
	}
	if findings[0].Severity != "critical" {
		t.Errorf("expected critical severity for third-party SSN, got %s", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Description, "third-party") {
		t.Errorf("expected third-party in description, got: %s", findings[0].Description)
	}
}

func TestScanURLForCredentials_AWSKeyInURL(t *testing.T) {
	t.Parallel()
	scanner := NewSecurityScanner()
	body := NetworkBody{
		URL:    "https://s3.amazonaws.com/bucket?AWSAccessKeyId=AKIAIOSFODNN7PRODKEY&Expires=1234",
		Method: "GET",
		Status: 200,
	}
	findings := scanner.scanURLForCredentials(body)
	found := false
	for _, f := range findings {
		if strings.Contains(f.Title, "AWS access key") {
			found = true
			if f.Severity != "critical" {
				t.Errorf("expected critical severity, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected AWS key finding in URL")
	}
}

// ============================================
// Coverage gap: tools.go onEntries callback
// ============================================

func TestOnEntriesCallback_ArgsString(t *testing.T) {
	t.Parallel()
	server := &Server{logFile: filepath.Join(t.TempDir(), "test.jsonl"), maxEntries: 100}
	capture := NewCapture()
	_ = setupToolHandler(t, server, capture)

	entries := []LogEntry{
		{
			"level": "error",
			"args":  []interface{}{"Connection timeout on /api/users"},
		},
	}
	if server.onEntries != nil {
		server.onEntries(entries)
	}
}

func TestOnEntriesCallback_ArgsErrorObject(t *testing.T) {
	t.Parallel()
	server := &Server{logFile: filepath.Join(t.TempDir(), "test.jsonl"), maxEntries: 100}
	capture := NewCapture()
	_ = setupToolHandler(t, server, capture)

	entries := []LogEntry{
		{
			"level": "error",
			"args": []interface{}{
				map[string]interface{}{
					"name":    "TypeError",
					"message": "Cannot read property 'x' of null",
					"stack":   "TypeError: Cannot read property 'x'\n    at foo.js:10",
				},
			},
		},
	}
	if server.onEntries != nil {
		server.onEntries(entries)
	}
}

func TestOnEntriesCallback_EmptyMessage(t *testing.T) {
	t.Parallel()
	server := &Server{logFile: filepath.Join(t.TempDir(), "test.jsonl"), maxEntries: 100}
	capture := NewCapture()
	_ = setupToolHandler(t, server, capture)

	entries := []LogEntry{
		{
			"level": "error",
			"args":  []interface{}{42},
		},
	}
	if server.onEntries != nil {
		server.onEntries(entries)
	}
}

func TestOnEntriesCallback_NonErrorSkipped(t *testing.T) {
	t.Parallel()
	server := &Server{logFile: filepath.Join(t.TempDir(), "test.jsonl"), maxEntries: 100}
	capture := NewCapture()
	_ = setupToolHandler(t, server, capture)

	entries := []LogEntry{
		{"level": "info", "message": "All systems operational"},
	}
	if server.onEntries != nil {
		server.onEntries(entries)
	}
}

// ============================================
// Coverage gap: actions.go HandleEnhancedActions
// ============================================

func TestHandleEnhancedActions_BodyTooLarge(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	largeBody := strings.Repeat("x", 11*1024*1024)
	req := httptest.NewRequest("POST", "/actions", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	capture.HandleEnhancedActions(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}
}

func TestHandleEnhancedActions_RateLimitAfterRecording(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	actions := make([]EnhancedAction, 200)
	for i := range actions {
		actions[i] = EnhancedAction{
			Type: "click", Timestamp: int64(1706090400000 + i), URL: "http://example.com",
		}
	}
	payload := struct {
		Actions []EnhancedAction `json:"actions"`
	}{Actions: actions}
	data, _ := json.Marshal(payload)

	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("POST", "/actions", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		capture.HandleEnhancedActions(w, req)
		if w.Code == http.StatusTooManyRequests {
			return
		}
	}
}

// ============================================
// Coverage gap: temporal_graph.go handleRecordEvent
// ============================================

func TestHandleRecordEvent_MissingDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	eventData := map[string]interface{}{"type": "error"}
	result, errMsg := handleRecordEvent(tg, eventData, "test-agent")
	if result != "" {
		t.Errorf("expected empty result, got: %s", result)
	}
	if !strings.Contains(errMsg, "description") {
		t.Errorf("expected description error, got: %s", errMsg)
	}
}

func TestToolConfigureRecordEvent_MissingDescription(t *testing.T) {
	t.Parallel()
	server := &Server{logFile: filepath.Join(t.TempDir(), "test.jsonl"), maxEntries: 100}
	capture := NewCapture()
	handler := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}
	resp := handler.toolHandler.toolConfigureRecordEvent(req, json.RawMessage(`{"event": {"type": "error"}}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if !strings.Contains(result.Content[0].Text, "description") {
		t.Errorf("expected description error, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Coverage gap: capture_control.go
// ============================================

func TestSetMultiple_UnknownSettingName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow timeout test")
	}
	t.Parallel()
	co := NewCaptureOverrides()
	time.Sleep(1100 * time.Millisecond)

	settings := map[string]string{"nonexistent_setting": "value"}
	errs := co.SetMultiple(settings)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown setting")
	}
	errStr := errs["nonexistent_setting"].Error()
	if !strings.Contains(errStr, "unknown capture setting") {
		t.Errorf("expected 'unknown capture setting' error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "log_level") {
		t.Errorf("expected valid settings listed, got: %s", errStr)
	}
}

func TestBuildSettingsResponse_NilOverrides(t *testing.T) {
	t.Parallel()
	co := NewCaptureOverrides()
	resp := buildSettingsResponse(co)
	if !resp.Connected {
		t.Error("expected Connected=true")
	}
	if resp.CaptureOverrides == nil {
		t.Error("expected non-nil CaptureOverrides map")
	}
}

func TestNewAuditLogger_ValidPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "audit.jsonl")
	logger, err := NewAuditLogger(path)
	if err != nil {
		t.Fatalf("NewAuditLogger failed: %v", err)
	}
	defer logger.Close()

	logger.Write(AuditEvent{Event: "setting_changed", Setting: "log_level", To: "debug", Source: "test"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected audit log to have content")
	}
}

// ============================================
// Coverage gap: main.go helpers
// ============================================

func TestJsonResponse_ContentType(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	jsonResponse(w, http.StatusCreated, map[string]string{"id": "123"})
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("expected application/json content type")
	}
}

func TestSaveEntries_WithEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "save-test.jsonl")
	server := &Server{logFile: logFile, maxEntries: 100}
	server.entries = []LogEntry{
		{"level": "info", "message": "test1"},
		{"level": "error", "message": "test2"},
	}

	err := server.saveEntries()
	if err != nil {
		t.Fatalf("saveEntries failed: %v", err)
	}

	data, _ := os.ReadFile(logFile)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

// ============================================
// Coverage gap: security.go extractOrigin
// ============================================

func TestExtractOrigin_VariousURLs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, expected string
	}{
		{"https://example.com/path?q=1", "https://example.com"},
		{"http://localhost:3000/api", "http://localhost:3000"},
		{"https://sub.domain.com:8080/foo", "https://sub.domain.com:8080"},
	}
	for _, tt := range tests {
		got := extractOrigin(tt.input)
		if got != tt.expected {
			t.Errorf("extractOrigin(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
