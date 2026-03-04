// upload_test_helpers_test.go — Shared test helpers for upload tests.
// Why: Provides uploadTestEnv and createTestFile used by upload_handlers_test.go,
// upload_handlers_edge_test.go, and upload_integration_test.go.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/upload"
)

type uploadTestEnv struct {
	*interactTestEnv
}

// newUploadTestEnv creates a test environment with upload automation enabled.
func newUploadTestEnv(t *testing.T) *uploadTestEnv {
	t.Helper()
	upload.SetSkipSSRFCheck(true)
	t.Cleanup(func() { upload.SetSkipSSRFCheck(false) })

	server, err := NewServer(filepath.Join(t.TempDir(), "test-upload.jsonl"), 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	cap := capture.NewCapture()
	mockConnectedTrackedTab(t, cap)
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)

	handler.uploadSecurity = upload.NewSecurity("/", nil)

	return &uploadTestEnv{
		interactTestEnv: &interactTestEnv{handler: handler, server: server, capture: cap},
	}
}

// createTestFile creates a temp file with given content and returns its path.
func createTestFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return path
}

// handleFileRead directly calls the file read handler for unit testing.
func (e *uploadTestEnv) handleFileRead(t *testing.T, req FileReadRequest) FileReadResponse {
	t.Helper()
	return handleFileReadInternal(req, e.handler.uploadSecurity, false)
}

// handleDialogInject directly calls the dialog inject handler for unit testing.
func (e *uploadTestEnv) handleDialogInject(t *testing.T, req FileDialogInjectRequest) UploadStageResponse {
	t.Helper()
	return handleDialogInjectInternal(req, e.handler.uploadSecurity)
}

// handleFormSubmit directly calls the form submit handler for unit testing.
func (e *uploadTestEnv) handleFormSubmit(t *testing.T, req FormSubmitRequest) UploadStageResponse {
	t.Helper()
	return e.handler.handleFormSubmitInternal(req)
}

// handleOSAutomation directly calls the OS automation handler for unit testing.
func (e *uploadTestEnv) handleOSAutomation(t *testing.T, req OSAutomationInjectRequest) UploadStageResponse {
	t.Helper()
	return e.handler.handleOSAutomationInternal(req)
}
