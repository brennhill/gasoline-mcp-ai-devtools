// tools_interact_upload_test.go — Tests for file upload feature (4-stage escalation).
//
// WARNING: DO NOT use t.Parallel() — tests share global state (skipSSRFCheck, uploadSecurityConfig).
//
// Tests verify:
// 1. Security gating (--enable-upload-automation flag required)
// 2. Parameter validation for upload action
// 3. Stage 1: File read (base64 for small files)
// 4. Stage 2: File dialog injection
// 5. Stage 3: Form submission with streaming
// 6. Stage 4: OS automation injection
// 7. MIME type detection
// 8. Progress tracking tiers
// 9. Error escalation and edge cases
//
// Run: go test ./cmd/dev-console -run "TestUpload" -v
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Security: Upload automation must be explicitly enabled
// ============================================

func TestUpload_DisabledByDefault(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"upload","selector":"#Filedata","file_path":"/tmp/test.mp4"}`)
	if !ok {
		t.Fatal("upload should return result even when disabled")
	}

	if !result.IsError {
		t.Error("upload MUST return isError when upload automation is disabled")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "upload") || !strings.Contains(text, "disabled") {
			t.Errorf("error should mention upload automation is disabled\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestUpload_EnabledWithFlag(t *testing.T) {
	env := newUploadTestEnv(t)

	// Create a small test file
	testFile := createTestFile(t, "test.txt", "hello world")

	result, ok := env.callInteract(t, `{"action":"upload","selector":"#Filedata","file_path":"`+testFile+`"}`)
	if !ok {
		t.Fatal("upload with enabled flag should return result")
	}

	if result.IsError {
		t.Fatalf("upload with enabled flag should succeed, got error: %s", result.Content[0].Text)
	}

	// Verify response has required fields
	data := parseResponseJSON(t, result)
	assertObjectShape(t, "upload enabled", data, []fieldSpec{
		required("status", "string"),
		required("correlation_id", "string"),
		required("file_name", "string"),
		required("file_size", "number"),
		required("mime_type", "string"),
		required("progress_tier", "string"),
		required("message", "string"),
	})
}

// ============================================
// Parameter validation
// ============================================

func TestUpload_MissingFilePath(t *testing.T) {
	env := newUploadTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"upload","selector":"#Filedata"}`)
	if !ok {
		t.Fatal("upload without file_path should return result")
	}

	if !result.IsError {
		t.Error("upload without file_path MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "file_path") {
			t.Errorf("error should mention file_path parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestUpload_MissingSelectorAndEndpoint(t *testing.T) {
	env := newUploadTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"upload","file_path":"/tmp/test.mp4"}`)
	if !ok {
		t.Fatal("upload without selector should return result")
	}

	if !result.IsError {
		t.Error("upload without selector or apiEndpoint MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "selector") {
			t.Errorf("error should mention selector parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestUpload_FileNotFound(t *testing.T) {
	env := newUploadTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"upload","selector":"#Filedata","file_path":"/nonexistent/path/video.mp4"}`)
	if !ok {
		t.Fatal("upload with nonexistent file should return result")
	}

	if !result.IsError {
		t.Error("upload with nonexistent file MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "not found") && !strings.Contains(text, "no such file") {
			t.Errorf("error should mention file not found\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestUpload_FilePermissionDenied(t *testing.T) {
	// Create a file with no read permissions
	dir := t.TempDir()
	testFile := filepath.Join(dir, "noperm.mp4")
	if err := os.WriteFile(testFile, []byte("data"), 0o000); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(testFile, 0o644) })

	// Permission denied is caught at file read time (Stage 1), not at upload queue time.
	// The interact upload action only validates the file exists (os.Stat succeeds
	// even on 0o000 files since stat only needs directory traversal).
	// Test the Stage 1 handler directly:
	env := newUploadTestEnv(t)
	req := FileReadRequest{FilePath: testFile}
	resp := env.handleFileRead(t, req)

	if resp.Success {
		t.Error("file read with unreadable file should fail")
	}

	if resp.Error == "" {
		t.Error("should have error message for unreadable file")
	}

	if !strings.Contains(strings.ToLower(resp.Error), "permission") {
		t.Errorf("error should mention permission denied\nGot: %s", resp.Error)
	}
}

// ============================================
// Stage 1: File read (POST /api/file/read)
// ============================================

func TestUpload_FileRead_SmallFile(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "small.txt", "hello world")

	req := FileReadRequest{FilePath: testFile}
	resp := env.handleFileRead(t, req)

	if !resp.Success {
		t.Fatalf("file read should succeed for small file, got error: %s", resp.Error)
	}

	if resp.FileName != "small.txt" {
		t.Errorf("expected filename 'small.txt', got '%s'", resp.FileName)
	}

	if resp.FileSize != 11 {
		t.Errorf("expected file size 11, got %d", resp.FileSize)
	}

	if resp.MimeType == "" {
		t.Error("MIME type should be detected")
	}

	if resp.DataBase64 == "" {
		t.Error("small file should include base64 data")
	}
}

func TestUpload_FileRead_MimeDetection(t *testing.T) {
	env := newUploadTestEnv(t)

	tests := []struct {
		filename string
		content  string
		expected string
	}{
		{"test.mp4", "fake video", "video/mp4"},
		{"test.jpg", "fake image", "image/jpeg"},
		{"test.png", "fake image", "image/png"},
		{"test.pdf", "fake pdf", "application/pdf"},
		{"test.txt", "hello", "text/plain"},
		{"test.html", "<html>", "text/html"},
		{"test.json", "{}", "application/json"},
		{"test.csv", "a,b,c", "text/csv"},
		{"test.zip", "fake zip", "application/zip"},
		{"test.unknown", "data", "application/octet-stream"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			testFile := createTestFile(t, tc.filename, tc.content)
			req := FileReadRequest{FilePath: testFile}
			resp := env.handleFileRead(t, req)

			if !resp.Success {
				t.Fatalf("file read should succeed, got error: %s", resp.Error)
			}

			if resp.MimeType != tc.expected {
				t.Errorf("expected MIME type '%s' for %s, got '%s'", tc.expected, tc.filename, resp.MimeType)
			}
		})
	}
}

func TestUpload_FileRead_NotFound(t *testing.T) {
	env := newUploadTestEnv(t)

	req := FileReadRequest{FilePath: "/nonexistent/file.txt"}
	resp := env.handleFileRead(t, req)

	if resp.Success {
		t.Error("file read for nonexistent file should fail")
	}

	if resp.Error == "" {
		t.Error("should have error message for nonexistent file")
	}
}

func TestUpload_FileRead_EmptyPath(t *testing.T) {
	env := newUploadTestEnv(t)

	req := FileReadRequest{FilePath: ""}
	resp := env.handleFileRead(t, req)

	if resp.Success {
		t.Error("file read with empty path should fail")
	}
}

// ============================================
// Stage 2: File dialog injection
// ============================================

func TestUpload_DialogInject_ValidRequest(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "video.mp4", "fake video content")

	req := FileDialogInjectRequest{
		FilePath:   testFile,
		BrowserPID: 12345,
	}
	resp := env.handleDialogInject(t, req)

	// Should return queued status (actual dialog injection is async)
	if !resp.Success {
		t.Fatalf("dialog inject should succeed (queue), got error: %s", resp.Error)
	}

	if resp.Status == "" {
		t.Error("should have status message")
	}
}

func TestUpload_DialogInject_FileNotFound(t *testing.T) {
	env := newUploadTestEnv(t)

	req := FileDialogInjectRequest{
		FilePath:   "/nonexistent/video.mp4",
		BrowserPID: 12345,
	}
	resp := env.handleDialogInject(t, req)

	if resp.Success {
		t.Error("dialog inject with nonexistent file should fail")
	}
}

func TestUpload_DialogInject_MissingPID(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "video.mp4", "fake video content")

	req := FileDialogInjectRequest{
		FilePath:   testFile,
		BrowserPID: 0,
	}
	resp := env.handleDialogInject(t, req)

	if resp.Success {
		t.Error("dialog inject without browser PID should fail")
	}
}

// ============================================
// Stage 3: Form submission
// ============================================

func TestUpload_FormSubmit_ValidRequest(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "video.mp4", "fake video content")

	req := FormSubmitRequest{
		FormAction:    "https://example.com/upload.php",
		Method:        "POST",
		Fields:        map[string]string{"title": "Test Video"},
		FileInputName: "Filedata",
		FilePath:      testFile,
		CSRFToken:     "abc123",
		Cookies:       "session=xyz",
	}
	resp := env.handleFormSubmit(t, req)

	// Remote server will reject (4xx/5xx) or network will fail.
	// What matters: no validation error (missing params, bad JSON).
	if resp.Success {
		t.Error("form submit to external host should not succeed")
	}
	if resp.Stage != 3 {
		t.Errorf("stage should be 3, got %d", resp.Stage)
	}
	// Must be a transport or server error, not a validation error
	if strings.Contains(strings.ToLower(resp.Error), "missing") ||
		strings.Contains(strings.ToLower(resp.Error), "invalid") {
		t.Errorf("should not be a validation error, got: %s", resp.Error)
	}
}

func TestUpload_FormSubmit_MissingFormAction(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "video.mp4", "fake video content")

	req := FormSubmitRequest{
		Method:        "POST",
		FileInputName: "Filedata",
		FilePath:      testFile,
	}
	resp := env.handleFormSubmit(t, req)

	if resp.Success {
		t.Error("form submit without form_action should fail")
	}
}

func TestUpload_FormSubmit_MissingFilePath(t *testing.T) {
	env := newUploadTestEnv(t)

	req := FormSubmitRequest{
		FormAction:    "https://example.com/upload.php",
		Method:        "POST",
		FileInputName: "Filedata",
	}
	resp := env.handleFormSubmit(t, req)

	if resp.Success {
		t.Error("form submit without file_path should fail")
	}
}

func TestUpload_FormSubmit_MissingFileInputName(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "video.mp4", "fake video content")

	req := FormSubmitRequest{
		FormAction: "https://example.com/upload.php",
		Method:     "POST",
		FilePath:   testFile,
	}
	resp := env.handleFormSubmit(t, req)

	if resp.Success {
		t.Error("form submit without file_input_name should fail")
	}
}

func TestUpload_FormSubmit_FileNotFound(t *testing.T) {
	env := newUploadTestEnv(t)

	req := FormSubmitRequest{
		FormAction:    "https://example.com/upload.php",
		Method:        "POST",
		FileInputName: "Filedata",
		FilePath:      "/nonexistent/video.mp4",
	}
	resp := env.handleFormSubmit(t, req)

	if resp.Success {
		t.Error("form submit with nonexistent file should fail")
	}
}

func TestUpload_FormSubmit_DefaultMethod(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "video.mp4", "fake video content")

	req := FormSubmitRequest{
		FormAction:    "https://example.com/upload.php",
		FileInputName: "Filedata",
		FilePath:      testFile,
		// Method is empty - should default to POST
	}
	resp := env.handleFormSubmit(t, req)

	// Remote server will reject or network will fail.
	// This proves the empty method was accepted and defaulted to POST.
	if resp.Stage != 3 {
		t.Errorf("stage should be 3, got %d", resp.Stage)
	}
	// Must NOT fail on validation - empty method defaults to POST
	if strings.Contains(strings.ToLower(resp.Error), "method") ||
		strings.Contains(strings.ToLower(resp.Error), "missing") {
		t.Errorf("empty method should default to POST, got validation error: %s", resp.Error)
	}
}

// ============================================
// Stage 4: OS automation
// ============================================

func TestUpload_OSAutomation_ValidRequest(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "video.mp4", "fake video content")

	req := OSAutomationInjectRequest{
		FilePath:   testFile,
		BrowserPID: 12345,
	}
	resp := env.handleOSAutomation(t, req)

	// OS automation attempts platform-specific execution.
	// In CI/test environments it may succeed or fail depending on OS capabilities.
	if resp.Stage != 4 {
		t.Errorf("stage should be 4, got %d", resp.Stage)
	}
	if resp.FileName == "" && resp.Success {
		t.Error("successful OS automation should include file_name")
	}
	// Whether success or failure, file_name should be in the response if success=true
	if resp.Success && resp.FileName != "video.mp4" {
		t.Errorf("expected file_name 'video.mp4', got %q", resp.FileName)
	}
}

func TestUpload_OSAutomation_FileNotFound(t *testing.T) {
	env := newUploadTestEnv(t)

	req := OSAutomationInjectRequest{
		FilePath:   "/nonexistent/video.mp4",
		BrowserPID: 12345,
	}
	resp := env.handleOSAutomation(t, req)

	if resp.Success {
		t.Error("OS automation with nonexistent file should fail")
	}
}

func TestUpload_OSAutomation_MissingBrowserPID(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "video.mp4", "fake video content")

	req := OSAutomationInjectRequest{
		FilePath: testFile,
	}
	resp := env.handleOSAutomation(t, req)

	if resp.Success {
		t.Error("OS automation without browser PID should fail")
	}
}

// ============================================
// Progress tracking tiers
// ============================================

func TestUpload_ProgressTier_Small(t *testing.T) {
	tier := getProgressTier(50 * 1024 * 1024) // 50MB
	if tier != ProgressTierSimple {
		t.Errorf("50MB file should use simple progress tier, got %s", tier)
	}
}

func TestUpload_ProgressTier_Medium(t *testing.T) {
	tier := getProgressTier(500 * 1024 * 1024) // 500MB
	if tier != ProgressTierPeriodic {
		t.Errorf("500MB file should use periodic progress tier, got %s", tier)
	}
}

func TestUpload_ProgressTier_Large(t *testing.T) {
	tier := getProgressTier(3 * 1024 * 1024 * 1024) // 3GB
	if tier != ProgressTierDetailed {
		t.Errorf("3GB file should use detailed progress tier, got %s", tier)
	}
}

func TestUpload_ProgressTier_Boundaries(t *testing.T) {
	// Exactly 100MB
	tier := getProgressTier(100 * 1024 * 1024)
	if tier != ProgressTierPeriodic {
		t.Errorf("exactly 100MB should use periodic tier, got %s", tier)
	}

	// Exactly 2GB
	tier = getProgressTier(2 * 1024 * 1024 * 1024)
	if tier != ProgressTierDetailed {
		t.Errorf("exactly 2GB should use detailed tier, got %s", tier)
	}

	// Just under 100MB
	tier = getProgressTier(99 * 1024 * 1024)
	if tier != ProgressTierSimple {
		t.Errorf("99MB should use simple tier, got %s", tier)
	}
}

// ============================================
// MIME type detection
// ============================================

func TestUpload_MimeType_Detection(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"video.mp4", "video/mp4"},
		{"video.webm", "video/webm"},
		{"video.avi", "video/x-msvideo"},
		{"video.mov", "video/quicktime"},
		{"video.mkv", "video/x-matroska"},
		{"image.jpg", "image/jpeg"},
		{"image.jpeg", "image/jpeg"},
		{"image.png", "image/png"},
		{"image.gif", "image/gif"},
		{"image.webp", "image/webp"},
		{"image.svg", "image/svg+xml"},
		{"doc.pdf", "application/pdf"},
		{"doc.txt", "text/plain"},
		{"doc.html", "text/html"},
		{"doc.css", "text/css"},
		{"doc.js", "application/javascript"},
		{"data.json", "application/json"},
		{"data.xml", "application/xml"},
		{"data.csv", "text/csv"},
		{"archive.zip", "application/zip"},
		{"archive.tar.gz", "application/gzip"},
		{"unknown.xyz123", "application/octet-stream"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := detectMimeType(tc.filename)
			if got != tc.expected {
				t.Errorf("detectMimeType(%q) = %q, want %q", tc.filename, got, tc.expected)
			}
		})
	}
}

// ============================================
// Edge cases
// ============================================

func TestUpload_InvalidJSON_ReturnsError(t *testing.T) {
	env := newUploadTestEnv(t)

	args := json.RawMessage(`{invalid json here}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolInteract(req, args)

	if resp.Result == nil && resp.Error == nil {
		t.Fatal("invalid JSON should return response, not nil")
	}

	if resp.Result != nil {
		var result MCPToolResult
		_ = json.Unmarshal(resp.Result, &result)
		if !result.IsError {
			t.Error("invalid JSON MUST return isError:true")
		}
	}
}

func TestUpload_RelativePath_Rejected(t *testing.T) {
	env := newUploadTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"upload","selector":"#Filedata","file_path":"../../../etc/passwd"}`)
	if !ok {
		t.Fatal("upload with relative path should return result")
	}

	if !result.IsError {
		t.Error("upload with relative path MUST return isError:true")
	}
}

func TestUpload_NoPanic_AllVariants(t *testing.T) {
	env := newUploadTestEnv(t)

	variants := []string{
		`{"action":"upload","selector":"#f","file_path":"/tmp/x"}`,
		`{"action":"upload","selector":"#f"}`,
		`{"action":"upload","file_path":"/tmp/x"}`,
		`{"action":"upload"}`,
		`{"action":"upload","selector":"#f","file_path":"","submit":true}`,
	}

	for i, v := range variants {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("upload variant %d PANICKED: %v", i, r)
				}
			}()

			args := json.RawMessage(v)
			req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := env.handler.toolInteract(req, args)

			if resp.Result == nil && resp.Error == nil {
				t.Errorf("upload variant %d returned nil response", i)
			}
		})
	}
}

// ============================================
// Test Infrastructure
// ============================================

type uploadTestEnv struct {
	*interactTestEnv
}

// newUploadTestEnv creates a test environment with upload automation enabled
func newUploadTestEnv(t *testing.T) *uploadTestEnv {
	t.Helper()
	// Allow private IPs in tests (httptest.NewServer uses 127.0.0.1)
	skipSSRFCheck = true
	t.Cleanup(func() { skipSSRFCheck = false })

	server, err := NewServer(filepath.Join(t.TempDir(), "test-upload.jsonl"), 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	cap := newTestCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)

	// Enable upload automation for tests with permissive security config
	handler.uploadAutomationEnabled = true
	handler.uploadSecurity = &UploadSecurity{uploadDir: "/"}

	return &uploadTestEnv{
		interactTestEnv: &interactTestEnv{handler: handler, server: server, capture: cap},
	}
}

// createTestFile creates a temp file with given content and returns its path
func createTestFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return path
}

// handleFileRead directly calls the file read handler for unit testing
func (e *uploadTestEnv) handleFileRead(t *testing.T, req FileReadRequest) FileReadResponse {
	t.Helper()
	return e.handler.handleFileReadInternal(req)
}

// handleDialogInject directly calls the dialog inject handler for unit testing
func (e *uploadTestEnv) handleDialogInject(t *testing.T, req FileDialogInjectRequest) UploadStageResponse {
	t.Helper()
	return e.handler.handleDialogInjectInternal(req)
}

// handleFormSubmit directly calls the form submit handler for unit testing
func (e *uploadTestEnv) handleFormSubmit(t *testing.T, req FormSubmitRequest) UploadStageResponse {
	t.Helper()
	return e.handler.handleFormSubmitInternal(req)
}

// handleOSAutomation directly calls the OS automation handler for unit testing
func (e *uploadTestEnv) handleOSAutomation(t *testing.T, req OSAutomationInjectRequest) UploadStageResponse {
	t.Helper()
	return e.handler.handleOSAutomationInternal(req)
}

// newTestCapture creates a capture for testing (reused from existing test infra)
func newTestCapture() *capture.Capture {
	return capture.NewCapture()
}
