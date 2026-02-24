// Purpose: Validate tools_interact_draw_test.go behavior and guard against regressions.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_draw_test.go — Tests for draw_mode_start interact handler.
package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleDrawModeStart_PilotDisabled(t *testing.T) {
	h := createTestToolHandler(t)

	// Pilot is disabled by default in test handler
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := h.handleDrawModeStart(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "disabled") || !strings.Contains(text, "Pilot") {
		t.Errorf("expected pilot disabled error mentioning both 'disabled' and 'Pilot', got %q", text)
	}
}

func TestHandleDrawModeStart_Success(t *testing.T) {
	h := createTestToolHandler(t)

	// Enable pilot
	h.capture.SetPilotEnabled(true)
	syncReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	syncReq.Header.Set("X-Gasoline-Client", "test-client")
	h.capture.HandleSync(httptest.NewRecorder(), syncReq)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := h.handleDrawModeStart(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "queued") || !strings.Contains(text, "correlation_id") {
		t.Errorf("expected queued response with both 'queued' and 'correlation_id', got %q", text)
	}
}

func TestHandleDrawModeStart_WithSession(t *testing.T) {
	h := createTestToolHandler(t)
	h.capture.SetPilotEnabled(true)
	syncReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	syncReq.Header.Set("X-Gasoline-Client", "test-client")
	h.capture.HandleSync(httptest.NewRecorder(), syncReq)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"annot_session":"my-review"}`)

	resp := h.handleDrawModeStart(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "queued") {
		t.Errorf("expected queued response, got %q", text)
	}
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("expected correlation_id in response, got %q", text)
	}
}

func TestGetAnnotationDetail_MalformedJSON(t *testing.T) {
	h := createTestToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{not valid json`)

	resp := h.toolGetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "Invalid JSON") {
		t.Errorf("expected Invalid JSON error, got %q", text)
	}
}
