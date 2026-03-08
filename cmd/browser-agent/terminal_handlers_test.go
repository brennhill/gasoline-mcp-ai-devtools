// terminal_handlers_test.go — Tests for terminal HTTP handlers: auth, session lifecycle, static serving.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pty"
)

func TestNewTerminalFrameWriter_SerializesConcurrentWrites(t *testing.T) {
	t.Parallel()

	const frameCount = 64

	var wire bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(&wire), bufio.NewWriter(&wire))
	writeFrame := newTerminalFrameWriter(rw)

	var wg sync.WaitGroup
	for i := 0; i < frameCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload := []byte(fmt.Sprintf("msg-%02d", i))
			if err := writeFrame(0x1, payload); err != nil {
				t.Errorf("writeFrame(%d): %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, frameCount)
	reader := bytes.NewReader(wire.Bytes())
	for {
		_, _, payload, err := wsReadFrame(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("wsReadFrame failed: %v", err)
		}
		seen[string(payload)] = true
	}

	if len(seen) != frameCount {
		t.Fatalf("decoded frames = %d, want %d", len(seen), frameCount)
	}
	for i := 0; i < frameCount; i++ {
		expected := fmt.Sprintf("msg-%02d", i)
		if !seen[expected] {
			t.Fatalf("missing payload %q", expected)
		}
	}
}

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
		"cmd":  "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	relays := newTerminalRelayMap()
	handleTerminalStart(rec, req, nil, mgr, nil, relays)

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
		"cmd":  "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})

	relays := newTerminalRelayMap()

	// First start.
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handleTerminalStart(rec, req, nil, mgr, nil, relays)
	if rec.Code != http.StatusOK {
		t.Fatalf("first start: expected 200, got %d", rec.Code)
	}

	// Second start with same ID.
	req = httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handleTerminalStart(rec, req, nil, mgr, nil, relays)
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

	relays := newTerminalRelayMap()
	handleTerminalStart(rec, req, nil, mgr, nil, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTerminalStop_DestroysSession(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	// Start a session first.
	startBody, _ := json.Marshal(map[string]any{
		"cmd":  "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})
	relays := newTerminalRelayMap()
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(startBody))
	rec := httptest.NewRecorder()
	handleTerminalStart(rec, req, nil, mgr, nil, relays)

	// Stop it.
	stopBody, _ := json.Marshal(map[string]any{"id": "default"})
	req = httptest.NewRequest("POST", "/terminal/stop", bytes.NewReader(stopBody))
	rec = httptest.NewRecorder()
	handleTerminalStop(rec, req, mgr, relays)

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
	relays := newTerminalRelayMap()
	handleTerminalStop(rec, req, mgr, relays)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleTerminalConfig_ListsSessions(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	// Start a session.
	_, err := mgr.Start(pty.StartConfig{
		ID:   "test",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/config", nil)
	rec := httptest.NewRecorder()
	relays := newTerminalRelayMap()
	handleTerminalConfig(rec, req, mgr, relays)

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

	// Validate the new rich session objects.
	sessions, ok := resp["sessions"].([]any)
	if !ok {
		t.Fatalf("expected sessions array, got %T", resp["sessions"])
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	sess := sessions[0].(map[string]any)
	if sess["id"] != "test" {
		t.Fatalf("expected id 'test', got %v", sess["id"])
	}
	if sess["alive"] != true {
		t.Fatalf("expected alive=true, got %v", sess["alive"])
	}
	if sess["pid"] == nil {
		t.Fatal("expected pid in session info")
	}
	if _, hasAlt := sess["alt_screen"]; !hasAlt {
		t.Fatal("expected alt_screen field in session info")
	}
}

func TestHandleTerminalValidate_ValidToken(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	result, err := mgr.Start(pty.StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/validate?token="+result.Token, nil)
	rec := httptest.NewRecorder()
	handleTerminalValidate(rec, req, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp["valid"] {
		t.Fatal("expected valid=true for live session")
	}
}

func TestHandleTerminalValidate_InvalidToken(t *testing.T) {
	mgr := pty.NewManager()

	req := httptest.NewRequest("GET", "/terminal/validate?token=bogus", nil)
	rec := httptest.NewRecorder()
	handleTerminalValidate(rec, req, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["valid"] {
		t.Fatal("expected valid=false for bogus token")
	}
}

func TestHandleTerminalValidate_EmptyToken(t *testing.T) {
	mgr := pty.NewManager()

	req := httptest.NewRequest("GET", "/terminal/validate", nil)
	rec := httptest.NewRecorder()
	handleTerminalValidate(rec, req, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["valid"] {
		t.Fatal("expected valid=false for empty token")
	}
}

func TestHandleTerminalValidate_StaleToken(t *testing.T) {
	mgr := pty.NewManager()

	// Start and immediately stop to create a stale token.
	result, err := mgr.Start(pty.StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	token := result.Token
	if err := mgr.Stop("default"); err != nil {
		t.Fatalf("stop: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/validate?token="+token, nil)
	rec := httptest.NewRecorder()
	handleTerminalValidate(rec, req, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["valid"] {
		t.Fatal("expected valid=false for stale token (session stopped)")
	}
}

func TestHandleTerminalWS_MissingToken(t *testing.T) {
	mgr := pty.NewManager()

	req := httptest.NewRequest("GET", "/terminal/ws", nil)
	rec := httptest.NewRecorder()
	relays := newTerminalRelayMap()
	handleTerminalWS(rec, req, mgr, relays)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleTerminalWS_InvalidToken(t *testing.T) {
	mgr := pty.NewManager()

	req := httptest.NewRequest("GET", "/terminal/ws?token=bogus", nil)
	rec := httptest.NewRecorder()
	relays := newTerminalRelayMap()
	handleTerminalWS(rec, req, mgr, relays)

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
	relays := newTerminalRelayMap()
	handleTerminalWS(rec, req, mgr, relays)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleTerminalUpload_Success(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	_, err := mgr.Start(pty.StartConfig{
		ID:   "upload-test",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	sess, _ := mgr.Get("upload-test")
	relays := newTerminalRelayMap()
	relays.getOrCreate("upload-test", sess, t.TempDir())

	imgData := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 10)
	req := httptest.NewRequest("POST", "/terminal/upload?session_id=upload-test&filename=test.png", bytes.NewReader(imgData))
	req.Header.Set("Content-Type", "image/png")
	rec := httptest.NewRecorder()
	handleTerminalUpload(rec, req, mgr, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["path"] == nil || resp["path"] == "" {
		t.Fatal("expected non-empty path in response")
	}
}

func TestHandleTerminalUpload_InvalidContentType(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	_, err := mgr.Start(pty.StartConfig{
		ID:   "upload-bad",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	sess, _ := mgr.Get("upload-bad")
	relays := newTerminalRelayMap()
	relays.getOrCreate("upload-bad", sess, t.TempDir())

	req := httptest.NewRequest("POST", "/terminal/upload?session_id=upload-bad&filename=test.txt", bytes.NewReader([]byte("not an image")))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handleTerminalUpload(rec, req, mgr, relays)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTerminalUpload_SessionNotFound(t *testing.T) {
	mgr := pty.NewManager()
	relays := newTerminalRelayMap()

	req := httptest.NewRequest("POST", "/terminal/upload?session_id=nonexistent", bytes.NewReader([]byte("data")))
	req.Header.Set("Content-Type", "image/png")
	rec := httptest.NewRecorder()
	handleTerminalUpload(rec, req, mgr, relays)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
