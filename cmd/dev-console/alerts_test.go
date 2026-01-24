package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================
// Push-Based Alerts Tests
// ============================================

// TestObserveNoAlertsOnFreshServer verifies that observe responses have
// a single content block when no alerts exist.
func TestObserveNoAlertsOnFreshServer(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content block (no alerts), got %d", len(result.Content))
	}
}

// TestAlertAppearsAfterObserve verifies that an alert generated between
// observe calls is included in the next observe response.
func TestAlertAppearsAfterObserve(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Add an alert to the tool handler's alert buffer
	th := mcp.toolHandler
	th.addAlert(Alert{
		Severity:  "warning",
		Category:  "regression",
		Title:     "LCP regression detected",
		Detail:    "LCP increased from 1.2s to 2.4s (+100%)",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "performance_monitor",
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) < 2 {
		t.Fatalf("Expected 2 content blocks (result + alerts), got %d", len(result.Content))
	}

	alertBlock := result.Content[1].Text
	if !strings.Contains(alertBlock, "ALERTS") {
		t.Errorf("Second content block should contain ALERTS header, got: %s", alertBlock)
	}
	if !strings.Contains(alertBlock, "LCP regression detected") {
		t.Errorf("Alert block should contain alert title, got: %s", alertBlock)
	}
}

// TestAlertsDrainedAfterObserve verifies that alerts are cleared after
// being returned in an observe response.
func TestAlertsDrainedAfterObserve(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	th := mcp.toolHandler
	th.addAlert(Alert{
		Severity:  "info",
		Category:  "noise",
		Title:     "New noise pattern detected",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "noise_detector",
	})

	// First observe: should include alert
	resp1 := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	var result1 MCPToolResult
	json.Unmarshal(resp1.Result, &result1)
	if len(result1.Content) < 2 {
		t.Fatalf("First observe should have alerts, got %d blocks", len(result1.Content))
	}

	// Second observe: alerts should be drained
	resp2 := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})

	var result2 MCPToolResult
	json.Unmarshal(resp2.Result, &result2)
	if len(result2.Content) != 1 {
		t.Errorf("Second observe should have no alerts (drained), got %d blocks", len(result2.Content))
	}
}

// TestAlertBufferCap verifies that the alert buffer is capped at 50 entries
// with FIFO eviction.
func TestAlertBufferCap(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	th := mcp.toolHandler

	// Add 55 alerts — only the newest 50 should remain
	for i := 0; i < 55; i++ {
		th.addAlert(Alert{
			Severity:  "info",
			Category:  "threshold",
			Title:     "Alert " + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Source:    "test",
		})
	}

	th.alertMu.Lock()
	count := len(th.alerts)
	th.alertMu.Unlock()

	if count != 50 {
		t.Errorf("Alert buffer should be capped at 50, got %d", count)
	}
}

// TestAlertPriorityOrdering verifies that alerts are sorted by severity
// (error > warning > info) then by timestamp (newest first).
func TestAlertPriorityOrdering(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	th := mcp.toolHandler
	now := time.Now().UTC()

	th.addAlert(Alert{Severity: "info", Category: "noise", Title: "Info alert", Timestamp: now.Format(time.RFC3339), Source: "test"})
	th.addAlert(Alert{Severity: "error", Category: "ci", Title: "Error alert", Timestamp: now.Add(time.Second).Format(time.RFC3339), Source: "test"})
	th.addAlert(Alert{Severity: "warning", Category: "threshold", Title: "Warning alert", Timestamp: now.Add(2 * time.Second).Format(time.RFC3339), Source: "test"})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) < 2 {
		t.Fatalf("Expected alerts block, got %d blocks", len(result.Content))
	}

	// Parse the alerts JSON from the block
	alerts := extractAlertsFromBlock(t, result.Content[1].Text)

	if len(alerts) < 3 {
		t.Fatalf("Expected 3 alerts, got %d", len(alerts))
	}

	// First alert should be the error (highest severity)
	if alerts[0].Severity != "error" {
		t.Errorf("First alert should be error severity, got %s", alerts[0].Severity)
	}
	if alerts[1].Severity != "warning" {
		t.Errorf("Second alert should be warning severity, got %s", alerts[1].Severity)
	}
	if alerts[2].Severity != "info" {
		t.Errorf("Third alert should be info severity, got %s", alerts[2].Severity)
	}
}

// TestAlertDeduplication verifies that repeated identical regressions
// are collapsed into a single alert with a count field.
func TestAlertDeduplication(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	th := mcp.toolHandler
	now := time.Now().UTC()

	// Add the same regression 3 times
	for i := 0; i < 3; i++ {
		th.addAlert(Alert{
			Severity:  "warning",
			Category:  "regression",
			Title:     "LCP regression on /home",
			Detail:    "LCP 2.4s vs baseline 1.2s",
			Timestamp: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Source:    "performance_monitor",
		})
	}

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	alerts := extractAlertsFromBlock(t, result.Content[1].Text)

	// Should be deduplicated to 1 alert with count
	if len(alerts) != 1 {
		t.Fatalf("Expected 1 deduplicated alert, got %d", len(alerts))
	}
	if alerts[0].Count != 3 {
		t.Errorf("Deduplicated alert should have count=3, got %d", alerts[0].Count)
	}
}

// TestAlertSummaryPrefix verifies that when more than 3 alerts exist,
// the alerts block starts with a summary line.
func TestAlertSummaryPrefix(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	th := mcp.toolHandler
	now := time.Now().UTC()

	th.addAlert(Alert{Severity: "error", Category: "ci", Title: "CI failed", Timestamp: now.Format(time.RFC3339), Source: "test"})
	th.addAlert(Alert{Severity: "warning", Category: "threshold", Title: "Memory pressure", Timestamp: now.Add(time.Second).Format(time.RFC3339), Source: "test"})
	th.addAlert(Alert{Severity: "info", Category: "noise", Title: "Noise detected", Timestamp: now.Add(2 * time.Second).Format(time.RFC3339), Source: "test"})
	th.addAlert(Alert{Severity: "warning", Category: "regression", Title: "LCP regression", Timestamp: now.Add(3 * time.Second).Format(time.RFC3339), Source: "test"})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	alertBlock := result.Content[1].Text
	// Should start with summary like "--- ALERTS (4) ---\n4 alerts: 1 regression, ..."
	if !strings.Contains(alertBlock, "4 alerts:") {
		t.Errorf("Expected summary prefix with '4 alerts:', got: %s", alertBlock[:min(100, len(alertBlock))])
	}
}

// TestCIWebhookValidRequest verifies that POST /ci-result with a valid body
// generates a CI alert.
func TestCIWebhookValidRequest(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	th := mcp.toolHandler

	body := `{
		"status": "failure",
		"source": "github-actions",
		"ref": "main",
		"commit": "abc123",
		"summary": "12 tests passed, 2 failed",
		"failures": [{"name": "test_login", "message": "Expected 200, got 401"}],
		"url": "https://github.com/org/repo/actions/runs/123",
		"duration_ms": 45000
	}`

	req := httptest.NewRequest("POST", "/ci-result", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	th.handleCIWebhook(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(respBody), `"ok":true`) {
		t.Errorf("Expected ok:true response, got: %s", string(respBody))
	}

	// Verify alert was generated
	th.alertMu.Lock()
	alertCount := len(th.alerts)
	th.alertMu.Unlock()

	if alertCount != 1 {
		t.Fatalf("Expected 1 alert from CI webhook, got %d", alertCount)
	}

	th.alertMu.Lock()
	alert := th.alerts[0]
	th.alertMu.Unlock()

	if alert.Category != "ci" {
		t.Errorf("Expected category 'ci', got %s", alert.Category)
	}
	if alert.Severity != "error" {
		t.Errorf("CI failure should generate error severity, got %s", alert.Severity)
	}
}

// TestCIWebhookInvalidBody verifies that invalid JSON returns 400 and no alert.
func TestCIWebhookInvalidBody(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	th := mcp.toolHandler

	req := httptest.NewRequest("POST", "/ci-result", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	th.handleCIWebhook(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid body, got %d", resp.StatusCode)
	}

	th.alertMu.Lock()
	alertCount := len(th.alerts)
	th.alertMu.Unlock()

	if alertCount != 0 {
		t.Errorf("No alert should be generated for invalid body, got %d", alertCount)
	}
}

// TestCIWebhookIdempotent verifies that posting the same commit+status twice
// updates rather than duplicates.
func TestCIWebhookIdempotent(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	th := mcp.toolHandler

	body := `{"status":"success","source":"github-actions","commit":"def456","summary":"all passed"}`

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/ci-result", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		th.handleCIWebhook(w, req)
	}

	th.alertMu.Lock()
	alertCount := len(th.alerts)
	th.alertMu.Unlock()

	// Should have 1 alert (updated, not duplicated)
	if alertCount != 1 {
		t.Errorf("Expected 1 alert (idempotent), got %d", alertCount)
	}
}

// TestCIWebhookSuccessSeverity verifies that successful CI results
// generate info-level alerts, not errors.
func TestCIWebhookSuccessSeverity(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	th := mcp.toolHandler

	body := `{"status":"success","source":"github-actions","commit":"ghi789","summary":"all 50 tests passed"}`
	req := httptest.NewRequest("POST", "/ci-result", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	th.handleCIWebhook(w, req)

	th.alertMu.Lock()
	alert := th.alerts[0]
	th.alertMu.Unlock()

	if alert.Severity != "info" {
		t.Errorf("Successful CI should be info severity, got %s", alert.Severity)
	}
}

// TestCIResultsCapped verifies that only the most recent 10 CI results are kept.
func TestCIResultsCapped(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	th := mcp.toolHandler

	for i := 0; i < 15; i++ {
		body := `{"status":"success","source":"github-actions","commit":"commit` + string(rune('a'+i)) + `","summary":"passed"}`
		req := httptest.NewRequest("POST", "/ci-result", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		th.handleCIWebhook(w, req)
	}

	th.alertMu.Lock()
	ciCount := len(th.ciResults)
	th.alertMu.Unlock()

	if ciCount > 10 {
		t.Errorf("CI results should be capped at 10, got %d", ciCount)
	}
}

// TestAlertCorrelation verifies that a performance regression and error spike
// within the same 5-second window are grouped into a compound alert.
func TestAlertCorrelation(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	th := mcp.toolHandler
	now := time.Now().UTC()

	// Add regression and anomaly within 5-second window
	th.addAlert(Alert{
		Severity:  "warning",
		Category:  "regression",
		Title:     "LCP regression",
		Detail:    "LCP 2.4s vs 1.2s",
		Timestamp: now.Format(time.RFC3339),
		Source:    "performance_monitor",
	})
	th.addAlert(Alert{
		Severity:  "warning",
		Category:  "anomaly",
		Title:     "Error rate spike",
		Detail:    "3x increase in errors",
		Timestamp: now.Add(2 * time.Second).Format(time.RFC3339),
		Source:    "anomaly_detector",
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	alerts := extractAlertsFromBlock(t, result.Content[1].Text)

	// Should be correlated into 1 compound alert
	if len(alerts) != 1 {
		t.Fatalf("Expected 1 correlated alert, got %d", len(alerts))
	}
	if !strings.Contains(alerts[0].Detail, "LCP") || !strings.Contains(alerts[0].Detail, "3x increase") {
		t.Errorf("Compound alert should contain both details, got: %s", alerts[0].Detail)
	}
}

// TestAnomalyDetectionErrorSpike verifies that error frequency >3x rolling average
// triggers an anomaly alert.
func TestAnomalyDetectionErrorSpike(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	th := mcp.toolHandler

	// Establish a baseline: add errors at a steady rate (simulating 60s of history)
	now := time.Now().UTC()
	for i := 0; i < 6; i++ {
		th.recordErrorForAnomaly(now.Add(time.Duration(-60+i*10) * time.Second))
	}

	// Now spike: add >3x the average in a 10-second window
	for i := 0; i < 20; i++ {
		th.recordErrorForAnomaly(now)
	}

	th.alertMu.Lock()
	var anomalyFound bool
	for _, alert := range th.alerts {
		if alert.Category == "anomaly" {
			anomalyFound = true
			break
		}
	}
	th.alertMu.Unlock()

	if !anomalyFound {
		t.Error("Expected anomaly alert from error spike, none found")
	}
}

// TestAlertOnlyOnObserve verifies that alerts are ONLY appended to observe
// tool responses, not analyze/generate/configure.
func TestAlertOnlyOnObserve(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	th := mcp.toolHandler
	th.addAlert(Alert{
		Severity:  "info",
		Category:  "ci",
		Title:     "CI passed",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "ci_webhook",
	})

	// Call analyze — should NOT include alerts
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"clear"}}`),
	})

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) != 1 {
		t.Errorf("Non-observe tools should not have alerts appended, got %d blocks", len(result.Content))
	}

	// Alert should still be in buffer (not drained by non-observe call)
	th.alertMu.Lock()
	count := len(th.alerts)
	th.alertMu.Unlock()
	if count != 1 {
		t.Errorf("Alert should still be in buffer after non-observe call, got %d", count)
	}
}

// TestCIWebhookBodySizeLimit verifies that request bodies over 1MB are rejected.
func TestCIWebhookBodySizeLimit(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	th := mcp.toolHandler

	// Create a body larger than 1MB
	largeBody := strings.Repeat("x", 1024*1024+1)
	req := httptest.NewRequest("POST", "/ci-result", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	th.handleCIWebhook(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for oversized body, got %d", resp.StatusCode)
	}
}

// ============================================
// Alert Test Helpers
// ============================================

func extractAlertsFromBlock(t *testing.T, block string) []Alert {
	t.Helper()
	// Format: "--- ALERTS (N) ---\n[optional summary]\n[JSON array]"
	// Find the JSON array in the block
	idx := strings.Index(block, "[")
	if idx == -1 {
		t.Fatalf("No JSON array found in alerts block: %s", block)
	}
	var alerts []Alert
	if err := json.Unmarshal([]byte(block[idx:]), &alerts); err != nil {
		t.Fatalf("Failed to parse alerts JSON: %v\nBlock: %s", err, block)
	}
	return alerts
}
