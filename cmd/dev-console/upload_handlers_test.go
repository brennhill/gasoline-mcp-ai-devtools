// upload_handlers_test.go — HTTP endpoint and happy-path tests for file upload.
// Tests the HTTP layer (status codes, content types, disabled/enabled gating)
// and verifies MCP handler response contracts that the unit tests miss.
//
// WARNING: DO NOT use t.Parallel() — tests share global state (skipSSRFCheck, uploadSecurityConfig).
//
// Run: go test ./cmd/dev-console -run "TestUploadHandler" -v
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// MCP handler happy-path response contract
// ============================================

func TestUploadHandler_MCPHappyPath_ResponseContract(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "contract-test.txt", "contract test content")

	result, ok := env.callInteract(t, fmt.Sprintf(
		`{"action":"upload","selector":"#Filedata","file_path":"%s"}`, testFile))
	if !ok {
		t.Fatal("upload should return result")
	}

	if result.IsError {
		t.Fatalf("upload happy path should not be an error: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)

	assertObjectShape(t, "upload happy path", data, []fieldSpec{
		required("status", "string"),
		required("correlation_id", "string"),
		required("file_name", "string"),
		required("file_size", "number"),
		required("mime_type", "string"),
		required("progress_tier", "string"),
		required("message", "string"),
	})

	// Verify correlation_id prefix
	corrID, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corrID, "upload_") {
		t.Errorf("correlation_id should start with 'upload_', got %q", corrID)
	}

	// Verify file metadata matches actual file
	if fn, _ := data["file_name"].(string); fn != "contract-test.txt" {
		t.Errorf("file_name should be 'contract-test.txt', got %q", fn)
	}
	if fs, _ := data["file_size"].(float64); int64(fs) != 21 {
		t.Errorf("file_size should be 21, got %v", data["file_size"])
	}
	if mt, _ := data["mime_type"].(string); mt != "text/plain" {
		t.Errorf("mime_type should be 'text/plain', got %q", mt)
	}
}

// ============================================
// Base64 content decode verification
// ============================================

func TestUploadHandler_FileRead_Base64Roundtrip(t *testing.T) {
	env := newUploadTestEnv(t)
	// Use bytes that aren't valid UTF-8 to ensure binary fidelity
	content := []byte{0x00, 0x01, 0xFF, 0xFE, 0x89, 0x50, 0x4E, 0x47}
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.bin")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	resp := env.handleFileRead(t, FileReadRequest{FilePath: path})
	if !resp.Success {
		t.Fatalf("file read failed: %s", resp.Error)
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.DataBase64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if len(decoded) != len(content) {
		t.Fatalf("decoded length %d != original %d", len(decoded), len(content))
	}
	for i := range content {
		if decoded[i] != content[i] {
			t.Errorf("byte %d: decoded 0x%02x != original 0x%02x", i, decoded[i], content[i])
		}
	}
}

// ============================================
// Stage 3: Form submit with httptest server
// ============================================

func TestUploadHandler_FormSubmit_WithTestServer(t *testing.T) {
	skipSSRFCheck = true
	t.Cleanup(func() { skipSSRFCheck = false })
	testFile := createTestFile(t, "upload.txt", "file content for form submit")

	var (
		receivedFileName  string
		receivedFileData  string
		receivedCSRF      string
		receivedCustom    string
		receivedCookie    string
		receivedInputName string
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookie = r.Header.Get("Cookie")

		ct := r.Header.Get("Content-Type")
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			http.Error(w, "bad content type", 400)
			return
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			data, _ := io.ReadAll(part)
			name := part.FormName()
			switch {
			case part.FileName() != "":
				receivedFileName = part.FileName()
				receivedFileData = string(data)
				receivedInputName = name
			case name == "csrf_token":
				receivedCSRF = string(data)
			default:
				receivedCustom = string(data)
			}
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "Filedata",
		FilePath:      testFile,
		CSRFToken:     "tok123",
		Fields:        map[string]string{"title": "My Upload"},
		Cookies:       "session=abc",
	}, sec)

	if !resp.Success {
		t.Fatalf("form submit should succeed, got error: %s", resp.Error)
	}
	if resp.Stage != 3 {
		t.Errorf("expected stage 3, got %d", resp.Stage)
	}
	if resp.DurationMs < 0 {
		t.Error("duration_ms should be >= 0")
	}

	// Verify server received correct data
	if receivedFileName != "upload.txt" {
		t.Errorf("server received filename %q, want 'upload.txt'", receivedFileName)
	}
	if receivedFileData != "file content for form submit" {
		t.Errorf("server received file data %q, want original content", receivedFileData)
	}
	if receivedInputName != "Filedata" {
		t.Errorf("server received input name %q, want 'Filedata'", receivedInputName)
	}
	if receivedCSRF != "tok123" {
		t.Errorf("server received CSRF %q, want 'tok123'", receivedCSRF)
	}
	if receivedCustom != "My Upload" {
		t.Errorf("server received custom field %q, want 'My Upload'", receivedCustom)
	}
	if receivedCookie != "session=abc" {
		t.Errorf("server received cookie %q, want 'session=abc'", receivedCookie)
	}
}

// ============================================
// HTTP endpoint tests
// ============================================

// newUploadHTTPServer creates a test HTTP server with the 4 upload routes registered.
// osAutomationEnabled controls Stage 4 gating; Stages 1-3 are always available.
func newUploadHTTPServer(t *testing.T, osAutomationEnabled bool) (*httptest.Server, *Server) {
	t.Helper()
	// Allow private IPs in tests (httptest.NewServer uses 127.0.0.1)
	skipSSRFCheck = true
	t.Cleanup(func() { skipSSRFCheck = false })
	// Set permissive upload security for HTTP handler tests
	prev := uploadSecurityConfig
	uploadSecurityConfig = &UploadSecurity{uploadDir: "/"}
	t.Cleanup(func() { uploadSecurityConfig = prev })

	server, err := NewServer(filepath.Join(t.TempDir(), "upload-http.jsonl"), 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/file/read", func(w http.ResponseWriter, r *http.Request) {
		server.handleFileRead(w, r)
	})
	mux.HandleFunc("/api/file/dialog/inject", func(w http.ResponseWriter, r *http.Request) {
		server.handleFileDialogInject(w, r)
	})
	mux.HandleFunc("/api/form/submit", func(w http.ResponseWriter, r *http.Request) {
		server.handleFormSubmit(w, r)
	})
	mux.HandleFunc("/api/os-automation/inject", func(w http.ResponseWriter, r *http.Request) {
		server.handleOSAutomation(w, r, osAutomationEnabled)
	})

	return httptest.NewServer(mux), server
}

// --- /api/file/read ---

func TestUploadHandler_HTTP_FileRead_MethodNotAllowed(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/file/read")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/file/read should be 405, got %d", resp.StatusCode)
	}
}

func TestUploadHandler_HTTP_FileRead_Success(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	testFile := createTestFile(t, "http-read.txt", "http test content")
	resp := postJSON(t, ts.URL+"/api/file/read",
		fmt.Sprintf(`{"file_path":"%s"}`, testFile))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/file/read with valid file should be 200, got %d", resp.StatusCode)
	}

	var body FileReadResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if !body.Success {
		t.Errorf("expected success, got error: %s", body.Error)
	}
	if body.FileName != "http-read.txt" {
		t.Errorf("file_name = %q, want 'http-read.txt'", body.FileName)
	}
	if body.FileSize != 17 {
		t.Errorf("file_size = %d, want 17", body.FileSize)
	}
	if body.DataBase64 == "" {
		t.Error("data_base64 should not be empty for small file")
	}
}

func TestUploadHandler_HTTP_FileRead_NotFound(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/file/read", `{"file_path":"/nonexistent/file.txt"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("file not found should be 404, got %d", resp.StatusCode)
	}
}

func TestUploadHandler_HTTP_FileRead_InvalidJSON(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/file/read", `{not valid json}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid JSON should be 400, got %d", resp.StatusCode)
	}
}

// --- /api/file/dialog/inject ---

func TestUploadHandler_HTTP_DialogInject_Success(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	testFile := createTestFile(t, "dialog.mp4", "fake video")
	resp := postJSON(t, ts.URL+"/api/file/dialog/inject",
		fmt.Sprintf(`{"file_path":"%s","browser_pid":1234}`, testFile))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("valid dialog inject should be 200, got %d", resp.StatusCode)
	}
}

// --- /api/form/submit ---

func TestUploadHandler_HTTP_FormSubmit_MissingRequired(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	// Missing form_action
	resp := postJSON(t, ts.URL+"/api/form/submit",
		`{"file_input_name":"f","file_path":"/tmp/x"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing form_action should be 400, got %d", resp.StatusCode)
	}
}

// --- /api/os-automation/inject ---

func TestUploadHandler_HTTP_OSAutomation_Disabled(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, false)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/os-automation/inject",
		`{"file_path":"/tmp/test.mp4","browser_pid":1234}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("OS automation when disabled should be 403, got %d", resp.StatusCode)
	}

	var body UploadStageResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if !strings.Contains(body.Error, "enable-os-upload-automation") {
		t.Errorf("error should mention --enable-os-upload-automation, got: %s", body.Error)
	}
}

// ============================================
// Directory path rejection
// ============================================

func TestUploadHandler_FileRead_DirectoryRejected(t *testing.T) {
	env := newUploadTestEnv(t)
	dir := t.TempDir()

	resp := env.handleFileRead(t, FileReadRequest{FilePath: dir})

	if resp.Success {
		t.Error("reading a directory should fail")
	}
	if !strings.Contains(strings.ToLower(resp.Error), "directory") {
		t.Errorf("error should mention directory, got: %s", resp.Error)
	}
}

// ============================================
// Script injection prevention
// ============================================

func TestUploadHandler_AppleScriptSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{"normal path", "/Users/test/file.mp4", "/Users/test/file.mp4", ""},
		{"double quotes", `/tmp/file"name.mp4`, `\"`, ""},
		{"backslash", `/tmp/file\name.mp4`, `\\`, ""},
		// Injection: quotes are escaped so AppleScript sees literal text, not a command break.
		// The unescaped input would break out of the keystroke string in AppleScript.
		// After sanitization, all " become \" so the string stays intact.
		{"injection attempt", `/tmp/"; do shell script "rm -rf /"`, `\"`, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeForAppleScript(tc.input)
			if tc.contains != "" && !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
			// The injection payload should be escaped, not executable
			if tc.excludes != "" && strings.Contains(result, tc.excludes) {
				t.Errorf("expected result NOT to contain %q, got %q", tc.excludes, result)
			}
		})
	}
}

func TestUploadHandler_SendKeysSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal path", `C:\Users\test\file.mp4`, `C:\Users\test\file.mp4`},
		{"plus sign", `C:\test+file.mp4`, `C:\test{+}file.mp4`},
		{"percent sign", `C:\test%file.mp4`, `C:\test{%}file.mp4`},
		{"caret", `C:\test^file.mp4`, `C:\test{^}file.mp4`},
		{"tilde", `C:\test~file.mp4`, `C:\test{~}file.mp4`},
		{"parens", `C:\test(1).mp4`, `C:\test{(}1{)}.mp4`},
		{"braces", `C:\test{1}.mp4`, `C:\test{{}1{}}.mp4`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeForSendKeys(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeForSendKeys(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestUploadHandler_PathValidation_RejectsMetachars(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"normal path", "/Users/test/file.mp4", false},
		{"null byte", "/tmp/file\x00.mp4", true},
		{"newline", "/tmp/file\n.mp4", true},
		{"carriage return", "/tmp/file\r.mp4", true},
		{"backtick", "/tmp/file`.mp4", true},
		{"spaces ok", "/tmp/my file.mp4", false},
		{"unicode ok", "/tmp/café.mp4", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePathForOSAutomation(tc.path)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for path %q, got nil", tc.path)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for path %q: %v", tc.path, err)
			}
		})
	}
}

// ============================================
// Windows double-escape: SendKeys + PowerShell
// ============================================

func TestUploadHandler_WindowsDoubleEscape(t *testing.T) {
	// Verifies the two-layer escaping applied in executeWindowsAutomation:
	// Layer 1: sanitizeForSendKeys escapes SendKeys metacharacters (+^%~(){})
	// Layer 2: strings.ReplaceAll escapes " → `" for PowerShell string literals
	tests := []struct {
		name     string
		input    string
		expected string // after both layers
	}{
		{
			"normal path",
			`C:\Users\test\file.mp4`,
			`C:\Users\test\file.mp4`,
		},
		{
			"path with plus and quotes",
			`C:\test+dir\file"name.mp4`,
			// + → {+} (SendKeys), " → `" (PowerShell)
			"C:\\test{+}dir\\file`\"name.mp4",
		},
		{
			"all SendKeys specials no quotes",
			`C:\a+b^c%d~e(f)g{h}i.mp4`,
			`C:\a{+}b{^}c{%}d{~}e{(}f{)}g{{}h{}}i.mp4`,
		},
		{
			"quotes only",
			`C:\Users\"quoted".mp4`,
			// " → `" (PowerShell)
			"C:\\Users\\`\"quoted`\".mp4",
		},
		{
			"SendKeys specials and quotes combined",
			`C:\test+dir\"file(1)".mp4`,
			// + → {+}, " → `", ( → {(}, ) → {)}
			"C:\\test{+}dir\\`\"file{(}1{)}`\".mp4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Layer 1: SendKeys escaping
			sendKeysEscaped := sanitizeForSendKeys(tc.input)
			// Layer 2: PowerShell quote escaping
			psEscaped := strings.ReplaceAll(sendKeysEscaped, `"`, "`\"")

			if psEscaped != tc.expected {
				t.Errorf("double escape(%q):\n  got:  %q\n  want: %q", tc.input, psEscaped, tc.expected)
			}

			// Invariant: no unescaped " should remain (all " become `")
			if strings.Contains(psEscaped, `"`) && !strings.Contains(psEscaped, "`\"") {
				t.Errorf("found unescaped quote in result: %q", psEscaped)
			}
		})
	}
}

// ============================================
// maxBase64FileSize threshold
// ============================================

func TestUploadHandler_FileRead_LargeFileSkipsBase64(t *testing.T) {
	// Create a sparse file that appears large but doesn't use disk space.
	// We only need the file to be > maxBase64FileSize for the stat check.
	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Seek to just past the threshold and write 1 byte to make the file "large"
	if _, err := f.Seek(maxBase64FileSize+1, 0); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0x42}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	resp := handleFileReadInternal(FileReadRequest{FilePath: path}, testUploadSecurity(t), false)
	if !resp.Success {
		t.Fatalf("file read should succeed for large file, got error: %s", resp.Error)
	}
	if resp.DataBase64 != "" {
		t.Error("file > maxBase64FileSize should NOT include base64 data")
	}
	if resp.FileSize <= maxBase64FileSize {
		t.Errorf("file size should be > %d, got %d", maxBase64FileSize, resp.FileSize)
	}
	if resp.FileName != "large.bin" {
		t.Errorf("file_name should be 'large.bin', got %q", resp.FileName)
	}
}

func TestUploadHandler_FileRead_ExactThresholdIncludesBase64(t *testing.T) {
	// File exactly at the threshold (<=) should include base64
	dir := t.TempDir()
	path := filepath.Join(dir, "exact.bin")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Seek to threshold and write 1 byte (total size = maxBase64FileSize)
	if _, err := f.Seek(maxBase64FileSize-1, 0); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0x42}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	resp := handleFileReadInternal(FileReadRequest{FilePath: path}, testUploadSecurity(t), false)
	if !resp.Success {
		t.Fatalf("file read should succeed, got error: %s", resp.Error)
	}
	// File exactly at 100MB should include base64 (<=)
	if resp.DataBase64 == "" {
		t.Error("file at exactly maxBase64FileSize should include base64 data")
	}
}

// ============================================
// MIME type edge cases
// ============================================

func TestUploadHandler_MimeType_CaseInsensitive(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"FILE.MP4", "video/mp4"},
		{"Image.JPG", "image/jpeg"},
		{"doc.PDF", "application/pdf"},
		{"DATA.JSON", "application/json"},
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

func TestUploadHandler_MimeType_NoExtension(t *testing.T) {
	got := detectMimeType("Makefile")
	if got != "application/octet-stream" {
		t.Errorf("file without extension should be octet-stream, got %q", got)
	}
}

func TestUploadHandler_MimeType_DotFile(t *testing.T) {
	got := detectMimeType(".gitignore")
	if got != "application/octet-stream" {
		t.Errorf(".gitignore should be octet-stream, got %q", got)
	}
}

func TestUploadHandler_MimeType_DoubleExtension(t *testing.T) {
	got := detectMimeType("archive.tar.gz")
	if got != "application/gzip" {
		t.Errorf("archive.tar.gz should detect .gz, got %q", got)
	}
}

// ============================================
// Progress tier boundary at 0
// ============================================

func TestUploadHandler_ProgressTier_ZeroBytes(t *testing.T) {
	tier := getProgressTier(0)
	if tier != ProgressTierSimple {
		t.Errorf("0 bytes should use simple tier, got %s", tier)
	}
}

// ============================================
// Relative path rejection in form submit & dialog
// ============================================

func TestUploadHandler_FormSubmit_RelativePathRejected(t *testing.T) {
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    "https://example.com/upload",
		FileInputName: "file",
		FilePath:      "../../../etc/passwd",
	}, testUploadSecurity(t))

	if resp.Success {
		t.Error("form submit with relative path should fail")
	}
	if !strings.Contains(resp.Error, "absolute path") {
		t.Errorf("error should mention absolute path, got: %s", resp.Error)
	}
}

func TestUploadHandler_DialogInject_RelativePathRejected(t *testing.T) {
	resp := handleDialogInjectInternal(FileDialogInjectRequest{
		FilePath:   "../../../etc/passwd",
		BrowserPID: 1234,
	}, testUploadSecurity(t))

	if resp.Success {
		t.Error("dialog inject with relative path should fail")
	}
	if !strings.Contains(resp.Error, "absolute path") {
		t.Errorf("error should mention absolute path, got: %s", resp.Error)
	}
}

func TestUploadHandler_FileRead_RelativePathRejected(t *testing.T) {
	resp := handleFileReadInternal(FileReadRequest{
		FilePath: "../../../etc/passwd",
	}, testUploadSecurity(t), false)

	if resp.Success {
		t.Error("file read with relative path should fail")
	}
	if !strings.Contains(resp.Error, "absolute path") {
		t.Errorf("error should mention absolute path, got: %s", resp.Error)
	}
}

// ============================================
// Form submit HTTP error status codes
// ============================================

func TestUploadHandler_FormSubmit_HTTP401Response(t *testing.T) {
	skipSSRFCheck = true
	t.Cleanup(func() { skipSSRFCheck = false })
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	testFile := createTestFile(t, "auth.txt", "test content")
	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "file",
		FilePath:      testFile,
	}, sec)

	if resp.Success {
		t.Error("401 response should not be success")
	}
	if !strings.Contains(resp.Error, "401") {
		t.Errorf("error should mention 401, got: %s", resp.Error)
	}
}

func TestUploadHandler_FormSubmit_HTTP403Response(t *testing.T) {
	skipSSRFCheck = true
	t.Cleanup(func() { skipSSRFCheck = false })
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer ts.Close()

	testFile := createTestFile(t, "csrf.txt", "test content")
	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "file",
		FilePath:      testFile,
	}, sec)

	if resp.Success {
		t.Error("403 response should not be success")
	}
	if !strings.Contains(resp.Error, "403") {
		t.Errorf("error should mention 403, got: %s", resp.Error)
	}
}

func TestUploadHandler_FormSubmit_HTTP422Response(t *testing.T) {
	skipSSRFCheck = true
	t.Cleanup(func() { skipSSRFCheck = false })
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
	}))
	defer ts.Close()

	testFile := createTestFile(t, "validation.txt", "test content")
	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "file",
		FilePath:      testFile,
	}, sec)

	if resp.Success {
		t.Error("422 response should not be success")
	}
	if !strings.Contains(resp.Error, "422") {
		t.Errorf("error should mention 422, got: %s", resp.Error)
	}
}

// ============================================
// Helpers
// ============================================

// postJSON sends a POST request with JSON body to the given URL.
func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	return resp
}
