// upload_os_dismiss_test.go — Tests for handleOSAutomationDismiss HTTP handler.
//
// WARNING: DO NOT use t.Parallel() — tests share global state (Server instance).
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// ============================================
// handleOSAutomationDismiss
// ============================================

func TestHandleOSAutomationDismiss_MethodNotAllowed_GET(t *testing.T) {
	server := newDismissTestServer(t)

	req := httptest.NewRequest("GET", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	server.handleOSAutomationDismiss(w, req, true)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", resp.StatusCode)
	}

	// Verify Allow header is set
	allow := resp.Header.Get("Allow")
	if allow != "POST" {
		t.Errorf("expected Allow header 'POST', got %q", allow)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data["error"] != "Method not allowed" {
		t.Errorf("expected error 'Method not allowed', got %v", data["error"])
	}
}

func TestHandleOSAutomationDismiss_MethodNotAllowed_PUT(t *testing.T) {
	server := newDismissTestServer(t)

	req := httptest.NewRequest("PUT", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	server.handleOSAutomationDismiss(w, req, true)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", resp.StatusCode)
	}
}

func TestHandleOSAutomationDismiss_MethodNotAllowed_DELETE(t *testing.T) {
	server := newDismissTestServer(t)

	req := httptest.NewRequest("DELETE", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	server.handleOSAutomationDismiss(w, req, true)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", resp.StatusCode)
	}
}

func TestHandleOSAutomationDismiss_Disabled(t *testing.T) {
	server := newDismissTestServer(t)

	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	server.handleOSAutomationDismiss(w, req, false)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}

	var data UploadStageResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data.Success {
		t.Error("expected success=false when OS automation is disabled")
	}
	if data.Stage != 4 {
		t.Errorf("expected stage 4, got %d", data.Stage)
	}
	if data.Error != "OS automation is disabled." {
		t.Errorf("expected error message about disabled, got %q", data.Error)
	}
}

func TestHandleOSAutomationDismiss_Enabled_POST(t *testing.T) {
	server := newDismissTestServer(t)

	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	server.handleOSAutomationDismiss(w, req, true)

	resp := w.Result()
	defer resp.Body.Close()

	// When enabled, the handler calls dismissFileDialogInternal() which runs
	// a platform command (osascript/xdotool/powershell). In CI it may succeed
	// or fail depending on the environment. We verify:
	// 1. Response is valid JSON
	// 2. Status code is 200 (success) or 500 (command failed)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 200 or 500, got %d", resp.StatusCode)
	}

	var data UploadStageResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure is consistent regardless of success/failure
	if resp.StatusCode == http.StatusOK {
		if !data.Success {
			t.Error("expected success=true for status 200")
		}
	} else {
		if data.Success {
			t.Error("expected success=false for status 500")
		}
		if data.Error == "" {
			t.Error("expected non-empty error message for failed dismiss")
		}
	}
}

// newDismissTestServer creates a minimal Server for testing handleOSAutomationDismiss.
func newDismissTestServer(t *testing.T) *Server {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "test-dismiss.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	return server
}
