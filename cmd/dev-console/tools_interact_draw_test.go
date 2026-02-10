// tools_interact_draw_test.go — Tests for draw_mode_start interact handler.
package main

import (
	"encoding/json"
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
	if !strings.Contains(text, "disabled") && !strings.Contains(text, "Pilot") {
		t.Errorf("expected pilot disabled error, got %q", text)
	}
}

func TestHandleDrawModeStart_Success(t *testing.T) {
	h := createTestToolHandler(t)

	// Enable pilot
	h.capture.SetPilotEnabled(true)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := h.handleDrawModeStart(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "queued") && !strings.Contains(text, "correlation_id") {
		t.Errorf("expected queued response with correlation_id, got %q", text)
	}
}
