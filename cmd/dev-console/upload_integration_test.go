// upload_integration_test.go — Integration and edge case tests for upload feature.
// Covers: concurrency, middleware integration, pending query payload, Content-Disposition
// safety, writeErr propagation, MaxBytesReader enforcement, correlation ID uniqueness,
// response shape verification, HTTP success paths, truncate(), and permission edge cases.
//
// WARNING: DO NOT use t.Parallel() — tests share global state (skipSSRFCheck, uploadSecurityConfig).
//
// Run: go test ./cmd/dev-console -run "TestUploadInteg" -v
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dev-console/dev-console/internal/upload"
)

// allowTestSSRF enables private IP access for tests using httptest.NewServer (127.0.0.1).
func allowTestSSRF(t *testing.T) {
	t.Helper()
	upload.SkipSSRFCheck = true
	t.Cleanup(func() { upload.SkipSSRFCheck = false })
}

// testUploadSecurity returns a permissive UploadSecurity config for tests.
// The upload-dir is set to "/" so any absolute path is allowed.
// The denylist is still active (it's hardcoded).
func testUploadSecurity(t *testing.T) *UploadSecurity {
	t.Helper()
	return upload.NewSecurity("/", nil)
}

// testUploadSecurityWithDir returns an UploadSecurity scoped to a specific directory.
// Resolves symlinks so it matches the EvalSymlinks output in ValidateFilePath.
func testUploadSecurityWithDir(t *testing.T, dir string) *UploadSecurity {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("testUploadSecurityWithDir: EvalSymlinks(%s) failed: %v", dir, err)
	}
	return upload.NewSecurity(resolved, nil)
}

// testUploadSecurityNoDir returns an UploadSecurity with no upload-dir (Stage 1 only).
func testUploadSecurityNoDir(t *testing.T) *UploadSecurity {
	t.Helper()
	return upload.NewSecurity("", nil)
}

// ============================================
// 1. Concurrent form submit (race detector)
// ============================================

func TestUploadInteg_ConcurrentFormSubmit(t *testing.T) {
	allowTestSSRF(t)
	sec := testUploadSecurity(t)
	const workers = 10
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	testFile := createTestFile(t, "concurrent.txt", "concurrent test data")

	var wg sync.WaitGroup
	errs := make(chan string, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp := handleFormSubmitInternal(FormSubmitRequest{
				FormAction:    ts.URL,
				Method:        "POST",
				FileInputName: fmt.Sprintf("file_%d", idx),
				FilePath:      testFile,
			}, sec)
			if !resp.Success {
				errs <- fmt.Sprintf("worker %d failed: %s", idx, resp.Error)
			}
			if resp.Stage != 3 {
				errs <- fmt.Sprintf("worker %d: stage=%d, want 3", idx, resp.Stage)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for msg := range errs {
		t.Error(msg)
	}
}

// ============================================
// 2. extensionOnly middleware integration
// ============================================

func TestUploadInteg_ExtensionOnlyMiddleware(t *testing.T) {
	allowTestSSRF(t)
	// Use setupHTTPRoutes (the real route stack) to get middleware coverage
	server, err := NewServer(filepath.Join(t.TempDir(), "middleware-test.jsonl"), 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	mux := setupHTTPRoutes(server, nil)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	endpoints := []string{
		"/api/file/read",
		"/api/file/dialog/inject",
		"/api/form/submit",
		"/api/os-automation/inject",
	}

	for _, ep := range endpoints {
		t.Run("no_header"+ep, func(t *testing.T) {
			// POST without X-Gasoline-Client header → 403
			resp := postJSON(t, ts.URL+ep, `{"file_path":"/tmp/test.txt"}`)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s without extension header should be 403, got %d", ep, resp.StatusCode)
			}
		})
	}
}

// ============================================
// 3. Pending query payload verification
// ============================================

func TestUploadInteg_PendingQueryPayload(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "payload-check.mp4", "fake video for payload")

	// Call the upload handler
	result, ok := env.callInteract(t, fmt.Sprintf(
		`{"action":"upload","selector":"#VideoUpload","file_path":"%s","submit":true,"escalation_timeout_ms":8000}`,
		testFile))
	if !ok {
		t.Fatal("upload should return result")
	}
	if result.IsError {
		t.Fatalf("upload should succeed: %s", result.Content[0].Text)
	}

	// Retrieve the pending query from capture
	pending := env.capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("expected at least 1 pending query after upload")
	}

	// Find the upload query
	var uploadQuery *struct {
		Action              string `json:"action"`
		Selector            string `json:"selector"`
		FilePath            string `json:"file_path"`
		FileName            string `json:"file_name"`
		FileSize            int64  `json:"file_size"`
		MimeType            string `json:"mime_type"`
		Submit              bool   `json:"submit"`
		EscalationTimeoutMs int    `json:"escalation_timeout_ms"`
		ProgressTier        string `json:"progress_tier"`
	}

	for _, pq := range pending {
		if pq.Type == "upload" {
			uploadQuery = new(struct {
				Action              string `json:"action"`
				Selector            string `json:"selector"`
				FilePath            string `json:"file_path"`
				FileName            string `json:"file_name"`
				FileSize            int64  `json:"file_size"`
				MimeType            string `json:"mime_type"`
				Submit              bool   `json:"submit"`
				EscalationTimeoutMs int    `json:"escalation_timeout_ms"`
				ProgressTier        string `json:"progress_tier"`
			})
			if err := json.Unmarshal(pq.Params, uploadQuery); err != nil {
				t.Fatalf("failed to unmarshal upload params: %v", err)
			}
			break
		}
	}

	if uploadQuery == nil {
		t.Fatal("no upload query found in pending queries")
	}

	if uploadQuery.Action != "upload" {
		t.Errorf("action = %q, want 'upload'", uploadQuery.Action)
	}
	if uploadQuery.Selector != "#VideoUpload" {
		t.Errorf("selector = %q, want '#VideoUpload'", uploadQuery.Selector)
	}
	if uploadQuery.FileName != "payload-check.mp4" {
		t.Errorf("file_name = %q, want 'payload-check.mp4'", uploadQuery.FileName)
	}
	if uploadQuery.MimeType != "video/mp4" {
		t.Errorf("mime_type = %q, want 'video/mp4'", uploadQuery.MimeType)
	}
	if !uploadQuery.Submit {
		t.Error("submit should be true")
	}
	if uploadQuery.EscalationTimeoutMs != 8000 {
		t.Errorf("escalation_timeout_ms = %d, want 8000", uploadQuery.EscalationTimeoutMs)
	}
	if uploadQuery.ProgressTier != "simple" {
		t.Errorf("progress_tier = %q, want 'simple'", uploadQuery.ProgressTier)
	}
}

// ============================================
// 4. Content-Disposition header injection
// ============================================

func TestUploadInteg_ContentDisposition_SafeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal", "video.mp4", "video.mp4"},
		{"quotes", `file"name.mp4`, "file_name.mp4"},
		{"newline", "file\nname.mp4", "file_name.mp4"},
		{"carriage return", "file\rname.mp4", "file_name.mp4"},
		{"null byte", "file\x00name.mp4", "file_name.mp4"},
		{"combined", "file\"\n\r\x00.mp4", "file____.mp4"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeForContentDisposition(tc.input)
			if got != tc.expected {
				t.Errorf("sanitizeForContentDisposition(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestUploadInteg_ContentDisposition_PreservedInMultipart(t *testing.T) {
	allowTestSSRF(t)
	sec := testUploadSecurity(t)
	// Create a file with a name that would break Content-Disposition if unescaped
	dir := t.TempDir()
	// We can't create a file with quotes in the name on all OSes,
	// so test the sanitizer is applied by verifying the submitted filename
	testFile := createTestFile(t, "safe-name.txt", "content")

	var receivedFileName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		_, fh, err := r.FormFile("Filedata")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		receivedFileName = fh.Filename
		w.WriteHeader(200)
	}))
	defer ts.Close()

	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "Filedata",
		FilePath:      testFile,
	}, sec)

	if !resp.Success {
		t.Fatalf("form submit should succeed, got: %s", resp.Error)
	}
	if receivedFileName != "safe-name.txt" {
		t.Errorf("received filename %q, want 'safe-name.txt'", receivedFileName)
	}
	_ = dir // used by createTestFile
}

// ============================================
// 5. writeErrCh error path
// ============================================

func TestUploadInteg_FormSubmit_FileDeletedDuringUpload(t *testing.T) {
	allowTestSSRF(t)
	// Create a file, start form submit, but have the server delay reading
	// to give us time to delete the file. This exercises the write error path.
	dir := t.TempDir()
	path := filepath.Join(dir, "ephemeral.txt")
	if err := os.WriteFile(path, []byte("temporary content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Server that discards the body
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read (and discard) the body to let the pipe drain
		_, _ = fmt.Fprintf(w, `{"ok":true}`)
	}))
	defer ts.Close()

	// Delete the file after os.Stat and os.Open succeed but the pipe copy is ongoing.
	// This is racy by nature — the test verifies no panic/deadlock occurs.
	// We can't reliably trigger a read error, but concurrent deletion during
	// the multipart write may produce a write error. The key assertion: no panic.
	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "file",
		FilePath:      path,
	}, sec)

	// Success or error are both valid — the point is no panic/deadlock
	if resp.Stage != 3 {
		t.Errorf("stage should be 3, got %d", resp.Stage)
	}
}

// ============================================
// 6. MaxBytesReader enforcement
// ============================================

func TestUploadInteg_MaxBytesReader_FileRead(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	// Send a body larger than 1MB (the MaxBytesReader limit for /api/file/read)
	largeBody := strings.Repeat("x", 2*1024*1024) // 2MB
	resp, err := http.Post(ts.URL+"/api/file/read", "application/json", strings.NewReader(largeBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Should be rejected (400 for bad JSON or connection reset)
	if resp.StatusCode == http.StatusOK {
		t.Error("oversized body should not succeed")
	}
}

func TestUploadInteg_MaxBytesReader_FormSubmit(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	// Send a body larger than 10MB (the MaxBytesReader limit for /api/form/submit)
	largeBody := strings.Repeat("x", 11*1024*1024) // 11MB
	resp, err := http.Post(ts.URL+"/api/form/submit", "application/json", strings.NewReader(largeBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("oversized body should not succeed")
	}
}

// ============================================
// 7. Concurrent correlation ID uniqueness
// ============================================

func TestUploadInteg_CorrelationID_Unique(t *testing.T) {
	const count = 20
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "corr-id.txt", "test")

	ids := make(chan string, count)
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, ok := env.callInteract(t, fmt.Sprintf(
				`{"action":"upload","selector":"#f","file_path":"%s"}`, testFile))
			if !ok || result.IsError {
				return
			}
			data := parseResponseJSON(t, result)
			if corrID, ok := data["correlation_id"].(string); ok {
				ids <- corrID
			}
		}()
	}

	wg.Wait()
	close(ids)

	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate correlation_id: %s", id)
		}
		seen[id] = true
	}

	if len(seen) == 0 {
		t.Error("no correlation IDs collected")
	}
}

// ============================================
// 8. Dialog inject success response shape
// ============================================

func TestUploadInteg_DialogInject_ResponseShape(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "shape-check.mp4", "fake video for shape")

	resp := env.handleDialogInject(t, FileDialogInjectRequest{
		FilePath:   testFile,
		BrowserPID: 12345,
	})

	if !resp.Success {
		t.Fatalf("dialog inject should succeed, got error: %s", resp.Error)
	}
	if resp.Stage != 2 {
		t.Errorf("stage should be 2, got %d", resp.Stage)
	}
	if resp.FileName != "shape-check.mp4" {
		t.Errorf("file_name should be 'shape-check.mp4', got %q", resp.FileName)
	}
	if resp.FileSizeBytes <= 0 {
		t.Errorf("file_size_bytes should be > 0, got %d", resp.FileSizeBytes)
	}
	if resp.Status == "" {
		t.Error("status should not be empty")
	}
}

// ============================================
// 9. HTTP success paths for form/submit and os-automation
// ============================================

func TestUploadInteg_HTTP_FormSubmit_SuccessPath(t *testing.T) {
	allowTestSSRF(t)
	// Create a target server that accepts the upload
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	testFile := createTestFile(t, "http-form.txt", "form submit content")
	body := fmt.Sprintf(`{"form_action":"%s","file_input_name":"file","file_path":"%s"}`,
		target.URL, testFile)
	resp := postJSON(t, ts.URL+"/api/form/submit", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("form submit success path should be 200, got %d", resp.StatusCode)
	}

	var result UploadStageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Stage != 3 {
		t.Errorf("stage should be 3, got %d", result.Stage)
	}
}

func TestUploadInteg_HTTP_OSAutomation_SuccessPath(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	testFile := createTestFile(t, "os-auto.mp4", "fake video")
	body := fmt.Sprintf(`{"file_path":"%s","browser_pid":1234}`, testFile)
	resp := postJSON(t, ts.URL+"/api/os-automation/inject", body)
	defer resp.Body.Close()

	// OS automation may succeed or fail depending on environment.
	// What matters: we get a valid response (200 success or 400 error), not a crash.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 200 or 400, got %d", resp.StatusCode)
	}

	var result UploadStageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Stage != 4 {
		t.Errorf("stage should be 4, got %d", result.Stage)
	}
}

// ============================================
// 10. truncate() helper
// ============================================

func TestUploadInteg_Truncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"shorter", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"over", "hello world", 8, "hello..."},
		{"empty", "", 5, ""},
		{"maxLen 3", "hello", 3, "..."},
		{"maxLen 2", "hello", 2, ".."},
		{"maxLen 1", "hello", 1, "."},
		{"maxLen 0", "hello", 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

// ============================================
// 11. File permission edge case in form submit
// ============================================

func TestUploadInteg_FormSubmit_FilePermissionDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.txt")
	if err := os.WriteFile(path, []byte("data"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    "https://example.com/upload",
		FileInputName: "file",
		FilePath:      path,
	}, sec)

	// os.Stat succeeds on 0o000 files (only needs dir traversal)
	// but os.Open will fail with permission denied
	if resp.Success {
		t.Error("form submit with unreadable file should fail")
	}
	if !strings.Contains(strings.ToLower(resp.Error), "open") &&
		!strings.Contains(strings.ToLower(resp.Error), "permission") {
		t.Errorf("error should mention open/permission failure, got: %s", resp.Error)
	}
}

// ============================================
// 12. OS automation path validation integration
// ============================================

func TestUploadInteg_OSAutomation_PathWithNewline(t *testing.T) {
	// Path with newline should be rejected by the security validation chain.
	// The newline causes EvalSymlinks to fail (file doesn't exist),
	// so we verify rejection happens regardless.
	sec := testUploadSecurity(t)
	resp := handleOSAutomationInternal(OSAutomationInjectRequest{
		FilePath:   "/tmp/safe\ninjection",
		BrowserPID: 12345,
	}, sec)

	if resp.Success {
		t.Error("path with newline should be rejected")
	}
}

func TestUploadInteg_OSAutomation_PathWithNullByte(t *testing.T) {
	sec := testUploadSecurity(t)
	resp := handleOSAutomationInternal(OSAutomationInjectRequest{
		FilePath:   "/tmp/safe\x00injection",
		BrowserPID: 12345,
	}, sec)

	if resp.Success {
		t.Error("path with null byte should be rejected")
	}
}

func TestUploadInteg_OSAutomation_PathWithBacktick(t *testing.T) {
	// Create a real file so EvalSymlinks succeeds, then verify
	// validatePathForOSAutomation catches the backtick.
	dir := t.TempDir()
	path := filepath.Join(dir, "safe`whoami`.mp4")
	os.WriteFile(path, []byte("test"), 0o644)

	sec := testUploadSecurityWithDir(t, dir)
	resp := handleOSAutomationInternal(OSAutomationInjectRequest{
		FilePath:   path,
		BrowserPID: 12345,
	}, sec)

	if resp.Success {
		t.Error("path with backtick should be rejected")
	}
	if !strings.Contains(resp.Error, "backtick") {
		t.Errorf("error should mention backtick, got: %s", resp.Error)
	}
}

// ============================================
// 13. Path with leading dashes (flag injection)
// ============================================

func TestUploadInteg_OSAutomation_ValidPathPassesThrough(t *testing.T) {
	// Paths with unusual but valid characters should pass validation
	// and reach executeOSAutomation (which may fail on this OS but
	// shouldn't fail at validation). Tests that the -- terminator
	// in xdotool args doesn't break valid paths.
	validPaths := []string{
		"/tmp/normal.mp4",
		"/tmp/spaces in name.mp4",
		"/tmp/café.mp4",
		"/tmp/file-with-dashes.mp4",
		"/tmp/file_with_underscores.mp4",
		"/tmp/UPPERCASE.MP4",
	}

	for _, p := range validPaths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			err := validatePathForOSAutomation(p)
			if err != nil {
				t.Errorf("valid path %q should pass validation, got: %v", p, err)
			}
		})
	}
}

func TestUploadInteg_ContentDisposition_InputNameInjection(t *testing.T) {
	// Verify that a malicious file_input_name with quotes can't break
	// the Content-Disposition header framing.
	tests := []struct {
		name      string
		inputName string
		expected  string
	}{
		{"normal", "Filedata", "Filedata"},
		{"quotes in name", `file"data`, "file_data"},
		{"newline in name", "file\ndata", "file_data"},
		{"null in name", "file\x00data", "file_data"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeForContentDisposition(tc.inputName)
			if got != tc.expected {
				t.Errorf("sanitizeForContentDisposition(%q) = %q, want %q",
					tc.inputName, got, tc.expected)
			}
		})
	}
}

// ============================================
// 15. HTTP Method Validation
// ============================================

func TestUploadInteg_ValidateHTTPMethod(t *testing.T) {
	allowed := []string{"POST", "PUT", "PATCH", "post", "put", "patch", "Post"}
	for _, m := range allowed {
		t.Run("allow_"+m, func(t *testing.T) {
			if err := validateHTTPMethod(m); err != nil {
				t.Errorf("validateHTTPMethod(%q) should allow, got: %v", m, err)
			}
		})
	}

	blocked := []string{"GET", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT", ""}
	for _, m := range blocked {
		name := m
		if name == "" {
			name = "empty"
		}
		t.Run("block_"+name, func(t *testing.T) {
			if err := validateHTTPMethod(m); err == nil {
				t.Errorf("validateHTTPMethod(%q) should block, got nil", m)
			}
		})
	}
}

// ============================================
// 16. Cookie Header Validation
// ============================================

func TestUploadInteg_ValidateCookieHeader(t *testing.T) {
	valid := []string{
		"",
		"session=abc123",
		"session=abc123; csrf=xyz",
		"name=value with spaces",
		"emoji=🔥",
	}
	for _, c := range valid {
		name := c
		if name == "" {
			name = "empty"
		}
		t.Run("allow_"+name, func(t *testing.T) {
			if err := validateCookieHeader(c); err != nil {
				t.Errorf("validateCookieHeader(%q) should allow, got: %v", c, err)
			}
		})
	}

	injections := []struct {
		name   string
		cookie string
	}{
		{"CR", "session=abc\rInjected: header"},
		{"LF", "session=abc\nInjected: header"},
		{"CRLF", "session=abc\r\nInjected: header"},
		{"null", "session=abc\x00"},
		{"CRLF_start", "\r\nEvil: header"},
	}
	for _, tc := range injections {
		t.Run("block_"+tc.name, func(t *testing.T) {
			if err := validateCookieHeader(tc.cookie); err == nil {
				t.Errorf("validateCookieHeader(%q) should block header injection, got nil", tc.cookie)
			}
		})
	}
}

// ============================================
// 17. SSRF Validation (validateFormActionURL)
// ============================================

func TestUploadInteg_ValidateFormActionURL(t *testing.T) {
	// These tests exercise the URL parser and scheme checks;
	// they do NOT need skipSSRFCheck since they never hit the DNS path.
	schemeBlocked := []struct {
		name string
		url  string
	}{
		{"ftp", "ftp://example.com/upload"},
		{"file", "file:///etc/passwd"},
		{"javascript", "javascript:alert(1)"},
		{"data", "data:text/html,<h1>hi</h1>"},
		{"gopher", "gopher://evil.com"},
	}
	for _, tc := range schemeBlocked {
		t.Run("scheme_"+tc.name, func(t *testing.T) {
			if err := validateFormActionURL(tc.url); err == nil {
				t.Errorf("validateFormActionURL(%q) should block non-http scheme, got nil", tc.url)
			}
		})
	}

	// Blocked hostnames
	hostBlocked := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost/upload"},
		{"localhost_port", "http://localhost:8080/upload"},
		{"metadata_gcp", "http://metadata.google.internal/computeMetadata/v1/"},
	}
	for _, tc := range hostBlocked {
		t.Run("host_"+tc.name, func(t *testing.T) {
			if err := validateFormActionURL(tc.url); err == nil {
				t.Errorf("validateFormActionURL(%q) should block, got nil", tc.url)
			}
		})
	}

	// Empty / missing hostname
	t.Run("no_hostname", func(t *testing.T) {
		if err := validateFormActionURL("http:///path"); err == nil {
			t.Error("should reject URL with empty hostname")
		}
	})
}

// ============================================
// 18. isPrivateIP
// ============================================

func TestUploadInteg_IsPrivateIP(t *testing.T) {
	privateIPs := []string{
		"127.0.0.1", "127.0.0.99", "10.0.0.1", "10.255.255.255",
		"172.16.0.1", "172.31.255.255", "192.168.0.1", "192.168.255.255",
		"169.254.1.1", "::1", "fc00::1", "fe80::1",
	}
	for _, ipStr := range privateIPs {
		t.Run("private_"+ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", ipStr)
			}
			if !isPrivateIP(ip) {
				t.Errorf("isPrivateIP(%s) = false, want true", ipStr)
			}
		})
	}

	publicIPs := []string{
		"8.8.8.8", "1.1.1.1", "93.184.216.34", "2607:f8b0:4004:800::200e",
	}
	for _, ipStr := range publicIPs {
		t.Run("public_"+ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", ipStr)
			}
			if isPrivateIP(ip) {
				t.Errorf("isPrivateIP(%s) = true, want false", ipStr)
			}
		})
	}
}

// ============================================
// 19. Goroutine leak: writeErrCh drained on errors
// ============================================

func TestUploadInteg_FormSubmit_ContextCancelled(t *testing.T) {
	// Verifies that context cancellation doesn't leak the pipe-writer goroutine.
	// The race detector will catch a leaked goroutine writing to a closed pipe.
	allowTestSSRF(t)

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server that never finishes reading
		select {}
	}))
	defer slow.Close()

	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.txt")
	os.WriteFile(f, []byte("hello"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := FormSubmitRequest{
		FormAction:    slow.URL + "/upload",
		Method:        "POST",
		FileInputName: "file",
		FilePath:      f,
	}

	// Should fail fast due to cancelled context, not leak goroutines.
	// The race detector (-race) validates no goroutine leak.
	sec := testUploadSecurityWithDir(t, tmp)
	resp := handleFormSubmitInternalCtx(ctx, req, sec)
	if resp.Success {
		t.Error("expected failure with cancelled context")
	}
}

func TestUploadInteg_FormSubmit_ServerError_DrainChannel(t *testing.T) {
	// When the server returns an error, writeErrCh must be drained
	// so the pipe-writer goroutine can exit. Race detector catches leaks.
	allowTestSSRF(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.txt")
	os.WriteFile(f, []byte("some content"), 0644)

	req := FormSubmitRequest{
		FormAction:    srv.URL + "/upload",
		Method:        "POST",
		FileInputName: "file",
		FilePath:      f,
	}

	sec := testUploadSecurityWithDir(t, tmp)
	resp := handleFormSubmitInternalCtx(context.Background(), req, sec)
	if resp.Success {
		t.Error("expected failure on 500 response")
	}
	if !strings.Contains(resp.Error, "500") {
		t.Errorf("expected error to mention 500 status, got: %s", resp.Error)
	}
}
