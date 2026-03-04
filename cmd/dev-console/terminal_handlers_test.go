// terminal_handlers_test.go — Tests for terminal HTTP handlers: auth, session lifecycle, static serving.

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pty"
)

func TestHandleTerminalPage_ServesHTML(t *testing.T) {
	req := httptest.NewRequest("GET", "/terminal", nil)
	rec := httptest.NewRecorder()
	handleTerminalPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %s", ct)
	}
	body := rec.Body.String()
	if len(body) < 100 {
		t.Fatal("expected substantial HTML content")
	}
}

func TestHandleTerminalPage_RejectsNonGET(t *testing.T) {
	req := httptest.NewRequest("POST", "/terminal", nil)
	rec := httptest.NewRecorder()
	handleTerminalPage(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleTerminalStart_CreatesSession(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	body, _ := json.Marshal(map[string]any{
		"cmd": "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handleTerminalStart(rec, req, mgr, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["session_id"] != "default" {
		t.Fatalf("expected session_id 'default', got: %v", resp["session_id"])
	}
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected non-empty token")
	}
	if resp["pid"] == nil {
		t.Fatal("expected pid in response")
	}
}

func TestHandleTerminalStart_DuplicateReturnsConflict(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	body, _ := json.Marshal(map[string]any{
		"cmd": "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})

	// First start.
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handleTerminalStart(rec, req, mgr, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("first start: expected 200, got %d", rec.Code)
	}

	// Second start with same ID.
	req = httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handleTerminalStart(rec, req, mgr, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate start: expected 409, got %d", rec.Code)
	}
}

func TestHandleTerminalStart_DefaultsToShell(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handleTerminalStart(rec, req, mgr, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTerminalStop_DestroysSession(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	// Start a session first.
	startBody, _ := json.Marshal(map[string]any{
		"cmd": "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(startBody))
	rec := httptest.NewRecorder()
	handleTerminalStart(rec, req, mgr, nil)

	// Stop it.
	stopBody, _ := json.Marshal(map[string]any{"id": "default"})
	req = httptest.NewRequest("POST", "/terminal/stop", bytes.NewReader(stopBody))
	rec = httptest.NewRecorder()
	handleTerminalStop(rec, req, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify session is gone.
	if mgr.Count() != 0 {
		t.Fatal("expected 0 sessions after stop")
	}
}

func TestHandleTerminalStop_NotFound(t *testing.T) {
	mgr := pty.NewManager()

	body, _ := json.Marshal(map[string]any{"id": "nonexistent"})
	req := httptest.NewRequest("POST", "/terminal/stop", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handleTerminalStop(rec, req, mgr)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleTerminalConfig_ListsSessions(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	// Start a session.
	_, err := mgr.Start(pty.StartConfig{
		ID:  "test",
		Cmd: "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/config", nil)
	rec := httptest.NewRecorder()
	handleTerminalConfig(rec, req, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	count := resp["count"].(float64)
	if count != 1 {
		t.Fatalf("expected count 1, got %v", count)
	}
}

func TestHandleTerminalWS_MissingToken(t *testing.T) {
	mgr := pty.NewManager()

	req := httptest.NewRequest("GET", "/terminal/ws", nil)
	rec := httptest.NewRecorder()
	handleTerminalWS(rec, req, mgr)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleTerminalWS_InvalidToken(t *testing.T) {
	mgr := pty.NewManager()

	req := httptest.NewRequest("GET", "/terminal/ws?token=bogus", nil)
	rec := httptest.NewRecorder()
	handleTerminalWS(rec, req, mgr)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleTerminalWS_NoUpgradeHeader(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	// Start a session to get a valid token.
	result, err := mgr.Start(pty.StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/ws?token="+result.Token, nil)
	rec := httptest.NewRecorder()
	handleTerminalWS(rec, req, mgr)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
