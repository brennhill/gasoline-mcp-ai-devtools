package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// toolConfigureStore Tests (composite store path)
// ============================================

func TestToolConfigureStoreNilStore(t *testing.T) {
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
			"matchSpec": {"messageRegex": "test.*noise"}
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
		"rules": [{"category": "console", "classification": "known", "matchSpec": {"messageRegex": "removable"}}]
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
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	// First add a rule
	addArgs := json.RawMessage(`{
		"action": "add",
		"rules": [{"category": "console", "classification": "known", "matchSpec": {"messageRegex": "test-removal"}}]
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
