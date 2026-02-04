// api_contract_test.go — API contract tests for Extension ↔ Server communication.
// These tests verify that HTTP endpoints accept the correct methods and payloads.
//
// ARCHITECTURAL INVARIANT: These tests MUST pass. If they fail, the extension
// cannot communicate with the server and the product is broken.
//
// Modification of these tests requires principal review.
package capture

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ============================================
// Contract: POST Endpoints (Extension → Server)
// ============================================

func TestAPIContract_NetworkBodies_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"bodies": []map[string]any{
			{"url": "https://api.example.com/users", "method": "GET", "status": 200},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleNetworkBodies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /network-bodies should return 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}
}

func TestAPIContract_NetworkBodies_AcceptsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/network-bodies", nil)
	w := httptest.NewRecorder()

	c.HandleNetworkBodies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /network-bodies should return 200, got %d", w.Code)
	}
}

func TestAPIContract_EnhancedActions_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"actions": []map[string]any{
			{"type": "click", "timestamp": time.Now().UnixMilli(), "url": "https://example.com"},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleEnhancedActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /enhanced-actions should return 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}
}

func TestAPIContract_EnhancedActions_AcceptsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/enhanced-actions", nil)
	w := httptest.NewRecorder()

	c.HandleEnhancedActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /enhanced-actions should return 200, got %d", w.Code)
	}
}

func TestAPIContract_PerformanceSnapshots_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"snapshots": []map[string]any{
			{"url": "https://example.com", "timestamp": time.Now().Format(time.RFC3339)},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/performance-snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandlePerformanceSnapshots(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /performance-snapshots should return 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}
}

func TestAPIContract_PerformanceSnapshots_AcceptsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/performance-snapshots", nil)
	w := httptest.NewRecorder()

	c.HandlePerformanceSnapshots(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /performance-snapshots should return 200, got %d", w.Code)
	}
}

func TestAPIContract_ExtensionLogs_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"logs": []map[string]any{
			{"level": "debug", "message": "test", "source": "background"},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/extension-logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleExtensionLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /extension-logs should return 200, got %d", w.Code)
	}
}

func TestAPIContract_ExtensionLogs_RejectsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/extension-logs", nil)
	w := httptest.NewRecorder()

	c.HandleExtensionLogs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /extension-logs should return 405, got %d", w.Code)
	}
}

func TestAPIContract_ExtensionStatus_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"type":             "status",
		"tracking_enabled": true,
		"tracked_tab_id":   123,
		"tracked_tab_url":  "https://example.com",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/extension-status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleExtensionStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /api/extension-status should return 200, got %d", w.Code)
	}
}

func TestAPIContract_ExtensionStatus_AcceptsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/api/extension-status", nil)
	w := httptest.NewRecorder()

	c.HandleExtensionStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/extension-status should return 200, got %d", w.Code)
	}
}

func TestAPIContract_NetworkWaterfall_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"entries":  []map[string]any{{"url": "https://example.com/script.js"}},
		"page_url": "https://example.com",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleNetworkWaterfall(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /network-waterfall should return 200, got %d", w.Code)
	}
}

func TestAPIContract_NetworkWaterfall_AcceptsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/network-waterfall", nil)
	w := httptest.NewRecorder()

	c.HandleNetworkWaterfall(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /network-waterfall should return 200, got %d", w.Code)
	}
}

func TestAPIContract_WebSocketEvents_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"events": []map[string]any{
			{"type": "ws_message", "url": "wss://example.com/socket", "data": "test"},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleWebSocketEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /websocket-events should return 200, got %d", w.Code)
	}
}

func TestAPIContract_WebSocketEvents_AcceptsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/websocket-events", nil)
	w := httptest.NewRecorder()

	c.HandleWebSocketEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /websocket-events should return 200, got %d", w.Code)
	}
}

// ============================================
// Contract: Query Result Endpoints (Extension → Server)
// ============================================

func TestAPIContract_DOMResult_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"id":     "q-123",
		"result": map[string]any{"elements": []any{}},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/dom-result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleDOMResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /dom-result should return 200, got %d", w.Code)
	}
}

func TestAPIContract_DOMResult_RejectsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/dom-result", nil)
	w := httptest.NewRecorder()

	c.HandleDOMResult(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /dom-result should return 405, got %d", w.Code)
	}
}

func TestAPIContract_A11yResult_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"id":     "q-123",
		"result": map[string]any{"violations": []any{}},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/a11y-result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleA11yResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /a11y-result should return 200, got %d", w.Code)
	}
}

func TestAPIContract_ExecuteResult_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"id":     "q-123",
		"result": map[string]any{"success": true},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/execute-result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleExecuteResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /execute-result should return 200, got %d", w.Code)
	}
}

func TestAPIContract_HighlightResult_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"id":     "q-123",
		"result": map[string]any{"highlighted": 3},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/highlight-result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleHighlightResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /highlight-result should return 200, got %d", w.Code)
	}
}

func TestAPIContract_StateResult_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"id":     "q-123",
		"result": map[string]any{"success": true},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/state-result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleStateResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /state-result should return 200, got %d", w.Code)
	}
}

// ============================================
// Contract: GET-only Endpoints (Extension ← Server)
// ============================================

func TestAPIContract_PendingQueries_GETOnly(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// GET should work
	req := httptest.NewRequest("GET", "/pending-queries", nil)
	w := httptest.NewRecorder()
	c.HandlePendingQueries(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /pending-queries should return 200, got %d", w.Code)
	}

	// POST should fail
	req = httptest.NewRequest("POST", "/pending-queries", nil)
	w = httptest.NewRecorder()
	c.HandlePendingQueries(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /pending-queries should return 405, got %d", w.Code)
	}
}

func TestAPIContract_PilotStatus_GETOnly(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// GET should work
	req := httptest.NewRequest("GET", "/pilot-status", nil)
	w := httptest.NewRecorder()
	c.HandlePilotStatus(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /pilot-status should return 200, got %d", w.Code)
	}

	// POST should fail
	req = httptest.NewRequest("POST", "/pilot-status", nil)
	w = httptest.NewRecorder()
	c.HandlePilotStatus(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /pilot-status should return 405, got %d", w.Code)
	}
}

// ============================================
// Contract: Data Flow Verification
// ============================================

func TestAPIContract_EnhancedActions_POSTThenGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// POST some actions
	payload := map[string]any{
		"actions": []map[string]any{
			{"type": "click", "timestamp": time.Now().UnixMilli(), "url": "https://example.com"},
			{"type": "input", "timestamp": time.Now().UnixMilli(), "url": "https://example.com"},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c.HandleEnhancedActions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d", w.Code)
	}

	// GET should return them
	req = httptest.NewRequest("GET", "/enhanced-actions", nil)
	w = httptest.NewRecorder()
	c.HandleEnhancedActions(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	count, ok := resp["count"].(float64)
	if !ok || count != 2 {
		t.Errorf("Expected 2 actions after POST, got %v", resp["count"])
	}
}

func TestAPIContract_PerformanceSnapshots_POSTThenGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// POST some snapshots
	payload := map[string]any{
		"snapshots": []map[string]any{
			{"url": "https://example.com/page1", "timestamp": time.Now().Format(time.RFC3339)},
			{"url": "https://example.com/page2", "timestamp": time.Now().Format(time.RFC3339)},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/performance-snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c.HandlePerformanceSnapshots(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d", w.Code)
	}

	// GET should return them
	req = httptest.NewRequest("GET", "/performance-snapshots", nil)
	w = httptest.NewRecorder()
	c.HandlePerformanceSnapshots(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	count, ok := resp["count"].(float64)
	if !ok || count != 2 {
		t.Errorf("Expected 2 snapshots after POST, got %v", resp["count"])
	}
}

func TestAPIContract_NetworkBodies_POSTThenGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// POST some bodies
	payload := map[string]any{
		"bodies": []map[string]any{
			{"url": "https://api.example.com/users", "method": "GET", "status": 200},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c.HandleNetworkBodies(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d", w.Code)
	}

	// GET should return them
	req = httptest.NewRequest("GET", "/network-bodies", nil)
	w = httptest.NewRecorder()
	c.HandleNetworkBodies(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	count, ok := resp["count"].(float64)
	if !ok || count != 1 {
		t.Errorf("Expected 1 body after POST, got %v", resp["count"])
	}
}
