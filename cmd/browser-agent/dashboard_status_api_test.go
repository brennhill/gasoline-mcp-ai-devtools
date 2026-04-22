// dashboard_status_api_test.go — Pins the GET /api/status response shape
// against the OpenAPI spec's DashboardStatus schema (cmd/browser-agent/openapi.json).
// Without this test, a refactor of handleStatusAPI that drops a documented
// field would only be caught by the CI Schemathesis job — `go test` alone
// would be green. This test closes that gap.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleStatusAPI_PinsResponseShape(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := newCaptureWithRegistry(t)

	handler := handleStatusAPI(srv, cap, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Top-level fields declared by DashboardStatus (openapi.json).
	// Spec updates must add new fields here to keep this test a faithful mirror.
	required := []string{
		"version",
		"uptime_seconds",
		"pid",
		"platform",
		"extension_connected",
		"pilot_enabled",
		"buffers",
		"recent_commands",
		"listen_port",
		"terminal",
	}
	for _, field := range required {
		if _, ok := resp[field]; !ok {
			t.Errorf("response missing required field %q (declared in openapi.json DashboardStatus)", field)
		}
	}

	buffers, ok := resp["buffers"].(map[string]any)
	if !ok {
		t.Fatalf("buffers is not an object, got %T", resp["buffers"])
	}
	bufferFields := []string{
		"console_entries", "console_capacity",
		"network_entries", "network_capacity",
		"websocket_entries", "websocket_capacity",
		"action_entries", "action_capacity",
	}
	for _, field := range bufferFields {
		if _, ok := buffers[field]; !ok {
			t.Errorf("buffers missing required field %q", field)
		}
	}

	terminal, ok := resp["terminal"].(map[string]any)
	if !ok {
		t.Fatalf("terminal is not an object, got %T", resp["terminal"])
	}
	for _, field := range []string{"port", "running", "sessions"} {
		if _, ok := terminal[field]; !ok {
			t.Errorf("terminal missing required field %q", field)
		}
	}

	// recent_commands is nullable per spec (`type: [array, null]`).
	// Accept either null or a JSON array.
	rc := resp["recent_commands"]
	if rc != nil {
		if _, ok := rc.([]any); !ok {
			t.Errorf("recent_commands is neither null nor array, got %T", rc)
		}
	}
}
