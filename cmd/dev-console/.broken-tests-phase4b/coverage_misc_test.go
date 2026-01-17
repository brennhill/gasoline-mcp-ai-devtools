package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// rate_limit.go: RecordEvents — window expiry path
// ============================================

func TestRecordEvents_WindowExpiry(t *testing.T) {
	t.Parallel()
	c := setupTestCapture(t)

	// Record some events in the first window
	c.RecordEvents(500)

	// Manually expire the window by shifting rateWindowStart back
	c.mu.Lock()
	c.rateWindowStart = time.Now().Add(-2 * time.Second)
	c.mu.Unlock()

	// Record more events — this should trigger tickRateWindow and reset the window
	c.RecordEvents(100)

	c.mu.RLock()
	count := c.windowEventCount
	c.mu.RUnlock()

	// After window expiry and new recording, count should be 100 (the new events only)
	if count != 100 {
		t.Errorf("expected windowEventCount=100 after window reset, got %d", count)
	}
}

func TestRecordEvents_WindowExpiryOverThreshold(t *testing.T) {
	t.Parallel()
	c := setupTestCapture(t)

	// Push the window over the threshold before expiry
	c.RecordEvents(1500)

	// Expire the window
	c.mu.Lock()
	c.rateWindowStart = time.Now().Add(-2 * time.Second)
	c.mu.Unlock()

	// Record new events — tickRateWindow should increment rateLimitStreak
	c.RecordEvents(50)

	c.mu.RLock()
	streak := c.rateLimitStreak
	count := c.windowEventCount
	c.mu.RUnlock()

	if streak != 1 {
		t.Errorf("expected rateLimitStreak=1 after window over threshold expired, got %d", streak)
	}
	if count != 50 {
		t.Errorf("expected windowEventCount=50 in new window, got %d", count)
	}
}

// ============================================
// rate_limit.go: HandleHealth — method not allowed path
// ============================================

func TestHandleHealth_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	c := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/health", nil)
	rec := httptest.NewRecorder()
	c.HandleHealth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST to health, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "Method not allowed" {
		t.Errorf("expected error='Method not allowed', got '%s'", resp["error"])
	}
}

func TestHandleHealth_CircuitClosed(t *testing.T) {
	t.Parallel()
	c := setupTestCapture(t)

	// Circuit is closed by default; verify the response
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	c.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.CircuitOpen {
		t.Error("expected circuit_open=false when circuit is closed")
	}
	if resp.OpenedAt != "" {
		t.Error("expected opened_at to be empty when circuit is closed")
	}
}

// ============================================
// helpers.go: extractURLPath — edge cases
// ============================================

func TestExtractURLPath_JustHost(t *testing.T) {
	t.Parallel()
	result := extractURLPath("http://example.com")
	if result != "/" {
		t.Errorf("expected '/' for host-only URL, got '%s'", result)
	}
}

func TestExtractURLPath_EmptyPath(t *testing.T) {
	t.Parallel()
	result := extractURLPath("http://example.com?query=1")
	if result != "/" {
		t.Errorf("expected '/' for URL with query but no path, got '%s'", result)
	}
}

func TestExtractURLPath_WithPath(t *testing.T) {
	t.Parallel()
	result := extractURLPath("http://example.com/api/v1/users")
	if result != "/api/v1/users" {
		t.Errorf("expected '/api/v1/users', got '%s'", result)
	}
}

func TestExtractURLPath_EmptyString(t *testing.T) {
	t.Parallel()
	// Empty string parses but has empty path
	result := extractURLPath("")
	if result != "/" {
		t.Errorf("expected '/' for empty string, got '%s'", result)
	}
}

func TestExtractURLPath_JustPath(t *testing.T) {
	t.Parallel()
	result := extractURLPath("/foo/bar")
	if result != "/foo/bar" {
		t.Errorf("expected '/foo/bar', got '%s'", result)
	}
}

// ============================================
// helpers.go: removeFromSlice — not found case
// ============================================

func TestRemoveFromSlice_NotFound(t *testing.T) {
	t.Parallel()
	slice := []string{"a", "b", "c"}
	result := removeFromSlice(slice, "d")
	if len(result) != 3 {
		t.Errorf("expected length 3 when item not found, got %d", len(result))
	}
	for i, v := range []string{"a", "b", "c"} {
		if result[i] != v {
			t.Errorf("expected result[%d]='%s', got '%s'", i, v, result[i])
		}
	}
}

func TestRemoveFromSlice_EmptySlice(t *testing.T) {
	t.Parallel()
	result := removeFromSlice([]string{}, "anything")
	if len(result) != 0 {
		t.Errorf("expected empty slice, got length %d", len(result))
	}
}

func TestRemoveFromSlice_Found(t *testing.T) {
	t.Parallel()
	slice := []string{"a", "b", "c"}
	result := removeFromSlice(slice, "b")
	if len(result) != 2 {
		t.Errorf("expected length 2, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "c" {
		t.Errorf("expected [a, c], got %v", result)
	}
}

// ============================================
// main.go: getEntries — basic functionality
// ============================================

func TestGetEntries_Empty(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	entries := server.getEntries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries on fresh server, got %d", len(entries))
	}
}

func TestGetEntries_ReturnsCopy(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	// Add some entries
	server.addEntries([]LogEntry{
		{"level": "error", "message": "test1"},
		{"level": "warn", "message": "test2"},
	})

	entries := server.getEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Replacing elements in the returned slice should not affect the server
	entries[0] = LogEntry{"level": "info", "message": "replaced"}
	original := server.getEntries()
	if msg, _ := original[0]["message"].(string); msg != "test1" {
		t.Error("getEntries should return a slice copy; replacing elements should not affect the server")
	}

	// Length changes to the copy should not affect the server
	_ = append(entries, LogEntry{"level": "debug", "message": "extra"})
	if server.getEntryCount() != 2 {
		t.Error("appending to copy should not change server entry count")
	}
}

// ============================================
// main.go: NewServer — file doesn't exist (happy path)
// ============================================

func TestNewServer_NonExistentFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "subdir", "logs.jsonl")

	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer should succeed when file doesn't exist: %v", err)
	}
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	// The directory should have been created
	if _, err := os.Stat(filepath.Dir(logFile)); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

func TestNewServer_InvalidDirectory(t *testing.T) {
	t.Parallel()
	// Use /dev/null/impossible which cannot be a directory
	_, err := NewServer("/dev/null/impossible/logs.jsonl", 100)
	if err == nil {
		t.Error("expected error when directory cannot be created")
	}
}

// ============================================
// main.go: saveEntries — writes and reads back
// ============================================

func TestSaveEntries_WritesCorrectly(t *testing.T) {
	t.Parallel()
	server, logFile := setupTestServer(t)

	server.mu.Lock()
	server.entries = []LogEntry{
		{"level": "error", "message": "saved1"},
		{"level": "info", "message": "saved2"},
	}
	server.mu.Unlock()

	server.mu.Lock()
	err := server.saveEntries()
	server.mu.Unlock()
	if err != nil {
		t.Fatalf("saveEntries failed: %v", err)
	}

	// Read the file back and verify
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "saved1") || !strings.Contains(content, "saved2") {
		t.Error("expected saved entries in log file")
	}
}

func TestSaveEntries_InvalidPath(t *testing.T) {
	t.Parallel()
	server := &Server{
		logFile:    "/dev/null/impossible/file.jsonl",
		maxEntries: 10,
		entries:    []LogEntry{{"level": "error", "message": "test"}},
	}

	err := server.saveEntries()
	if err == nil {
		t.Error("expected error writing to invalid path")
	}
}

// ============================================
// main.go: sanitizeForFilename — character cases
// ============================================

func TestSanitizeForFilename_SpecialChars(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "hello_world"},
		{"file/path:name", "file_path_name"},
		{"normal-file.txt", "normal-file.txt"},
		{"a@b#c$d%e", "a_b_c_d_e"},
		{"UPPERCASE", "UPPERCASE"},
		{"123.456", "123.456"},
		{"with spaces and (parens)", "with_spaces_and__parens_"},
	}

	for _, tt := range tests {
		result := sanitizeForFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeForFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeForFilename_Truncation(t *testing.T) {
	t.Parallel()
	long := "abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz"
	result := sanitizeForFilename(long)
	if len(result) > 50 {
		t.Errorf("expected max length 50, got %d", len(result))
	}
}

// ============================================
// main.go: handleScreenshot — missing fields, method check
// ============================================

func TestHandleScreenshot_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/screenshot", nil)
	rec := httptest.NewRecorder()
	server.handleScreenshot(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleScreenshot_InvalidBase64(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	body := `{"data_url":"data:image/jpeg;base64,!!!invalid!!!", "url":"http://example.com"}`
	req := httptest.NewRequest("POST", "/screenshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.handleScreenshot(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid base64, got %d", rec.Code)
	}
}

func TestHandleScreenshot_MissingDataUrl(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	body := `{"url":"http://example.com"}`
	req := httptest.NewRequest("POST", "/screenshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.handleScreenshot(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing dataUrl, got %d", rec.Code)
	}
}

func TestHandleScreenshot_InvalidDataUrlFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	body := `{"data_url":"not-a-data-url"}`
	req := httptest.NewRequest("POST", "/screenshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.handleScreenshot(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid dataUrl format, got %d", rec.Code)
	}
}

func TestHandleScreenshot_Success(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	// Minimal valid base64 data (not a real JPEG but valid base64)
	body := `{"data_url":"data:image/jpeg;base64,dGVzdGRhdGE=", "url":"http://example.com/page", "correlation_id":"TypeError-err-123"}`
	req := httptest.NewRequest("POST", "/screenshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.handleScreenshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["filename"] == "" {
		t.Error("expected non-empty filename")
	}
	if resp["path"] == "" {
		t.Error("expected non-empty path")
	}
	// Verify file exists
	if _, err := os.Stat(resp["path"]); os.IsNotExist(err) {
		t.Errorf("expected screenshot file to exist at %s", resp["path"])
	}
}

func TestHandleScreenshot_NoURL(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	// No URL means hostname defaults to "unknown"
	body := `{"data_url":"data:image/jpeg;base64,dGVzdGRhdGE="}`
	req := httptest.NewRequest("POST", "/screenshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.handleScreenshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["filename"], "unknown") {
		t.Errorf("expected 'unknown' in filename when no URL provided, got %s", resp["filename"])
	}
}

func TestHandleScreenshot_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	body := `{not json}`
	req := httptest.NewRequest("POST", "/screenshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.handleScreenshot(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

// ============================================
// main.go: clearEntries
// ============================================

func TestClearEntries_RemovesAll(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "test1"},
		{"level": "info", "message": "test2"},
	})

	if server.getEntryCount() != 2 {
		t.Fatalf("expected 2 entries before clear, got %d", server.getEntryCount())
	}

	server.clearEntries()

	if server.getEntryCount() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", server.getEntryCount())
	}
}

func TestClearEntries_PersistsToFile(t *testing.T) {
	t.Parallel()
	server, logFile := setupTestServer(t)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "test1"},
	})

	server.clearEntries()

	// File should be empty or minimal
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) > 0 {
		t.Errorf("expected empty file after clear, got %d bytes", len(data))
	}
}

// ============================================
// main.go: jsonResponse — various status codes and data
// ============================================

func TestJsonResponse_Success(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	jsonResponse(rec, http.StatusOK, map[string]string{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %s", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", resp["status"])
	}
}

func TestJsonResponse_ErrorStatus(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	jsonResponse(rec, http.StatusBadRequest, map[string]string{"error": "bad"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestJsonResponse_NilData(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	jsonResponse(rec, http.StatusOK, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	// nil encodes as "null\n"
	if rec.Body.String() != "null\n" {
		t.Errorf("expected 'null\\n', got %q", rec.Body.String())
	}
}

func TestJsonResponse_ComplexData(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	data := map[string]interface{}{
		"count": 42,
		"items": []string{"a", "b"},
	}
	jsonResponse(rec, http.StatusCreated, data)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

// ============================================
// alerts.go: canCorrelate — time parse failure path
// ============================================

func TestCanCorrelate_InvalidTimestamps(t *testing.T) {
	t.Parallel()
	a := Alert{Category: "regression", Timestamp: "not-a-time"}
	b := Alert{Category: "anomaly", Timestamp: "also-not-a-time"}

	if canCorrelate(a, b) {
		t.Error("expected canCorrelate to return false when timestamps are unparseable")
	}
}

func TestCanCorrelate_OneInvalidTimestamp(t *testing.T) {
	t.Parallel()
	a := Alert{Category: "regression", Timestamp: time.Now().Format(time.RFC3339)}
	b := Alert{Category: "anomaly", Timestamp: "invalid"}

	if canCorrelate(a, b) {
		t.Error("expected canCorrelate to return false when one timestamp is invalid")
	}
}

func TestCanCorrelate_FirstTimestampInvalid(t *testing.T) {
	t.Parallel()
	a := Alert{Category: "regression", Timestamp: "invalid"}
	b := Alert{Category: "anomaly", Timestamp: time.Now().Format(time.RFC3339)}

	if canCorrelate(a, b) {
		t.Error("expected canCorrelate to return false when first timestamp is invalid")
	}
}

func TestCanCorrelate_SameCategory(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := Alert{Category: "regression", Timestamp: now.Format(time.RFC3339)}
	b := Alert{Category: "regression", Timestamp: now.Format(time.RFC3339)}

	if canCorrelate(a, b) {
		t.Error("expected canCorrelate to return false for same category")
	}
}

func TestCanCorrelate_WithinWindow(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := Alert{Category: "regression", Timestamp: now.Format(time.RFC3339)}
	b := Alert{Category: "anomaly", Timestamp: now.Add(2 * time.Second).Format(time.RFC3339)}

	if !canCorrelate(a, b) {
		t.Error("expected canCorrelate to return true for regression+anomaly within 5s")
	}
}

func TestCanCorrelate_OutsideWindow(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := Alert{Category: "regression", Timestamp: now.Format(time.RFC3339)}
	b := Alert{Category: "anomaly", Timestamp: now.Add(10 * time.Second).Format(time.RFC3339)}

	if canCorrelate(a, b) {
		t.Error("expected canCorrelate to return false for alerts >5s apart")
	}
}

func TestCanCorrelate_ReversedCategories(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := Alert{Category: "anomaly", Timestamp: now.Format(time.RFC3339)}
	b := Alert{Category: "regression", Timestamp: now.Add(1 * time.Second).Format(time.RFC3339)}

	if !canCorrelate(a, b) {
		t.Error("expected canCorrelate to return true for anomaly+regression pair")
	}
}

func TestCanCorrelate_NegativeTimeDifference(t *testing.T) {
	t.Parallel()
	now := time.Now()
	// b is BEFORE a (negative diff, should still work with absolute value)
	a := Alert{Category: "regression", Timestamp: now.Add(3 * time.Second).Format(time.RFC3339)}
	b := Alert{Category: "anomaly", Timestamp: now.Format(time.RFC3339)}

	if !canCorrelate(a, b) {
		t.Error("expected canCorrelate to handle negative time difference (within window)")
	}
}

// ============================================
// alerts.go: mergeAlerts — b.Severity > a.Severity path
// ============================================

func TestMergeAlerts_BHigherSeverity(t *testing.T) {
	t.Parallel()
	a := Alert{
		Severity:  "info",
		Category:  "regression",
		Title:     "Alert A",
		Detail:    "Detail A",
		Timestamp: "2026-01-01T00:00:00Z",
		Source:    "test",
	}
	b := Alert{
		Severity:  "error",
		Category:  "anomaly",
		Title:     "Alert B",
		Detail:    "Detail B",
		Timestamp: "2026-01-01T00:00:01Z",
		Source:    "test2",
	}

	merged := mergeAlerts(a, b)

	if merged.Severity != "error" {
		t.Errorf("expected severity='error' (from b), got '%s'", merged.Severity)
	}
	if merged.Timestamp != "2026-01-01T00:00:01Z" {
		t.Errorf("expected latest timestamp from b, got '%s'", merged.Timestamp)
	}
	if merged.Category != "regression" {
		t.Errorf("expected category='regression', got '%s'", merged.Category)
	}
	if !strings.Contains(merged.Title, "Alert A") || !strings.Contains(merged.Title, "Alert B") {
		t.Errorf("expected merged title to contain both alert titles, got '%s'", merged.Title)
	}
	if !strings.Contains(merged.Detail, "Detail A") || !strings.Contains(merged.Detail, "Detail B") {
		t.Errorf("expected merged detail to contain both details, got '%s'", merged.Detail)
	}
	if merged.Source != "test" {
		t.Errorf("expected source from a, got '%s'", merged.Source)
	}
}

func TestMergeAlerts_AHigherSeverity(t *testing.T) {
	t.Parallel()
	a := Alert{
		Severity:  "warning",
		Category:  "regression",
		Title:     "Alert A",
		Detail:    "Detail A",
		Timestamp: "2026-01-01T00:00:02Z",
		Source:    "test",
	}
	b := Alert{
		Severity:  "info",
		Category:  "anomaly",
		Title:     "Alert B",
		Detail:    "Detail B",
		Timestamp: "2026-01-01T00:00:00Z",
		Source:    "test2",
	}

	merged := mergeAlerts(a, b)

	if merged.Severity != "warning" {
		t.Errorf("expected severity='warning' (from a), got '%s'", merged.Severity)
	}
	if merged.Timestamp != "2026-01-01T00:00:02Z" {
		t.Errorf("expected latest timestamp from a, got '%s'", merged.Timestamp)
	}
}

func TestMergeAlerts_EqualSeverity(t *testing.T) {
	t.Parallel()
	a := Alert{
		Severity:  "warning",
		Category:  "regression",
		Title:     "Alert A",
		Detail:    "Detail A",
		Timestamp: "2026-01-01T00:00:00Z",
		Source:    "test",
	}
	b := Alert{
		Severity:  "warning",
		Category:  "anomaly",
		Title:     "Alert B",
		Detail:    "Detail B",
		Timestamp: "2026-01-01T00:00:01Z",
		Source:    "test2",
	}

	merged := mergeAlerts(a, b)

	// When equal, a.Severity is used (severityRank(b) is NOT > severityRank(a))
	if merged.Severity != "warning" {
		t.Errorf("expected severity='warning', got '%s'", merged.Severity)
	}
}

// ============================================
// alerts.go: severityRank — default case
// ============================================

func TestSeverityRank_Default(t *testing.T) {
	t.Parallel()
	if severityRank("unknown") != 0 {
		t.Errorf("expected rank 0 for unknown severity, got %d", severityRank("unknown"))
	}
	if severityRank("") != 0 {
		t.Errorf("expected rank 0 for empty severity, got %d", severityRank(""))
	}
	if severityRank("critical") != 0 {
		t.Errorf("expected rank 0 for 'critical' (not a valid level), got %d", severityRank("critical"))
	}
}

func TestSeverityRank_AllLevels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		severity string
		rank     int
	}{
		{"error", 3},
		{"warning", 2},
		{"info", 1},
		{"", 0},
		{"other", 0},
		{"UPPERCASE_ERROR", 0},
	}
	for _, tt := range tests {
		if got := severityRank(tt.severity); got != tt.rank {
			t.Errorf("severityRank(%q) = %d, want %d", tt.severity, got, tt.rank)
		}
	}
}

// ============================================
// export_sarif.go: saveSARIFToFile — error paths
// ============================================

func TestSaveSARIFToFile_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "test-output.sarif")

	log := &SARIFLog{
		Schema:  "https://example.com/schema",
		Version: "2.1.0",
		Runs:    []SARIFRun{},
	}

	err := saveSARIFToFile(log, outPath)
	if err != nil {
		t.Fatalf("saveSARIFToFile failed: %v", err)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	var parsed SARIFLog
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", parsed.Version)
	}
}

func TestSaveSARIFToFile_PathNotAllowed(t *testing.T) {
	t.Parallel()
	log := &SARIFLog{
		Schema:  "https://example.com/schema",
		Version: "2.1.0",
		Runs:    []SARIFRun{},
	}

	// Path outside allowed directories (not /tmp, not cwd)
	err := saveSARIFToFile(log, "/usr/local/nope/output.sarif")
	if err == nil {
		t.Error("expected error for path outside allowed directories")
	}
	if !strings.Contains(err.Error(), "must be under") {
		t.Errorf("expected path restriction error, got: %v", err)
	}
}

func TestSaveSARIFToFile_TmpPath(t *testing.T) {
	t.Parallel()
	log := &SARIFLog{
		Schema:  "https://example.com/schema",
		Version: "2.1.0",
		Runs:    []SARIFRun{},
	}

	outPath := filepath.Join(os.TempDir(), "sarif-test-tmp-output.sarif")
	defer os.Remove(outPath)

	err := saveSARIFToFile(log, outPath)
	if err != nil {
		t.Fatalf("saveSARIFToFile to /tmp should succeed: %v", err)
	}
}

// ============================================
// export_har.go: ExportHARToFile — error paths
// ============================================

func TestExportHARToFile_Success(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()
	capture.AddNetworkBodies([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:00:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200, ResponseBody: "ok"},
	})

	tmpFile := filepath.Join(t.TempDir(), "test-export.har")
	result, err := capture.ExportHARToFile(types.NetworkBodyFilter{}, tmpFile)
	if err != nil {
		t.Fatalf("ExportHARToFile failed: %v", err)
	}

	if result.SavedTo != tmpFile {
		t.Errorf("expected saved_to=%s, got %s", tmpFile, result.SavedTo)
	}
	if result.EntriesCount != 1 {
		t.Errorf("expected 1 entry, got %d", result.EntriesCount)
	}
	if result.FileSizeBytes == 0 {
		t.Error("expected non-zero file size")
	}

	// Verify file content
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	var harLog HARLog
	if err := json.Unmarshal(data, &harLog); err != nil {
		t.Fatalf("file is not valid HAR JSON: %v", err)
	}
}

func TestExportHARToFile_UnsafePath(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	// Path traversal should be rejected
	_, err := capture.ExportHARToFile(types.NetworkBodyFilter{}, "../../etc/passwd")
	if err == nil {
		t.Error("expected error for unsafe path with traversal")
	}
}

func TestExportHARToFile_AbsoluteUnsafePath(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	_, err := capture.ExportHARToFile(types.NetworkBodyFilter{}, "/etc/hosts")
	if err == nil {
		t.Error("expected error for absolute path outside /tmp")
	}
}

func TestExportHARToFile_EmptyCapture(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	tmpFile := filepath.Join(t.TempDir(), "empty-export.har")
	result, err := capture.ExportHARToFile(types.NetworkBodyFilter{}, tmpFile)
	if err != nil {
		t.Fatalf("ExportHARToFile failed: %v", err)
	}
	if result.EntriesCount != 0 {
		t.Errorf("expected 0 entries, got %d", result.EntriesCount)
	}
}

// ============================================
// export_har.go: toolExportHAR — save_to path
// ============================================

func TestToolExportHAR_SaveTo_UnsafePath(t *testing.T) {
	t.Parallel()
	server := &Server{
		entries: make([]LogEntry, 0),
	}
	capture := capture.NewCapture()
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server),
		capture:    capture,
	}

	args, _ := json.Marshal(map[string]interface{}{
		"save_to": "/usr/local/nope/evil.har",
	})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	// The result should contain an error message via mcpErrorResponse
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "not allowed") {
		t.Errorf("expected error about path not allowed, got: %v", result)
	}
}

func TestToolExportHAR_SaveTo_Success(t *testing.T) {
	t.Parallel()
	server := &Server{
		entries: make([]LogEntry, 0),
	}
	capture := capture.NewCapture()
	capture.AddNetworkBodies([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:00:00.000Z", Method: "GET", URL: "https://example.com", Status: 200},
	})
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server),
		capture:    capture,
	}

	tmpFile := filepath.Join(os.TempDir(), "test-tool-save-misc.har")
	defer os.Remove(tmpFile)

	args, _ := json.Marshal(map[string]interface{}{
		"save_to": tmpFile,
	})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Strip summary line before parsing JSON
	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var summary HARExportResult
	if err := json.Unmarshal([]byte(jsonPart), &summary); err != nil {
		t.Fatalf("failed to parse summary: %v", err)
	}
	if summary.EntriesCount != 1 {
		t.Errorf("expected 1 entry, got %d", summary.EntriesCount)
	}
	if summary.SavedTo != tmpFile {
		t.Errorf("expected saved_to=%s, got %s", tmpFile, summary.SavedTo)
	}
}

func TestToolExportHAR_NoSaveTo_ReturnsJSON(t *testing.T) {
	t.Parallel()
	server := &Server{
		entries: make([]LogEntry, 0),
	}
	capture := capture.NewCapture()
	capture.AddNetworkBodies([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:00:00.000Z", Method: "GET", URL: "https://example.com/test", Status: 200},
	})
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server),
		capture:    capture,
	}

	args, _ := json.Marshal(map[string]interface{}{})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Strip summary line before parsing JSON
	text2 := result.Content[0].Text
	jsonPart2 := text2
	if lines := strings.SplitN(text2, "\n", 2); len(lines) == 2 {
		jsonPart2 = lines[1]
	}
	var harLog HARLog
	if err := json.Unmarshal([]byte(jsonPart2), &harLog); err != nil {
		t.Fatalf("expected valid HAR JSON in response, got parse error: %v", err)
	}
	if harLog.Log.Version != "1.2" {
		t.Errorf("expected HAR version 1.2, got %s", harLog.Log.Version)
	}
}

// ============================================
// Coverage: rate_limit.go tickRateWindow — below threshold sets lastBelowThresholdAt (line 85)
// ============================================

func TestTickRateWindow_BelowThresholdSetsTime(t *testing.T) {
	t.Parallel()
	c := setupTestCapture(t)

	// Start with zero events in window (below threshold)
	c.mu.Lock()
	c.windowEventCount = 100             // well below rateLimitThreshold (1000)
	c.lastBelowThresholdAt = time.Time{} // not yet set
	c.tickRateWindow()
	belowAt := c.lastBelowThresholdAt
	c.mu.Unlock()

	if belowAt.IsZero() {
		t.Error("expected lastBelowThresholdAt to be set when below threshold")
	}
}

// ============================================
// Coverage: rate_limit.go evaluateCircuit — circuit open, streak > 0 prevents close (line 116)
// ============================================

func TestEvaluateCircuit_StillOverThreshold(t *testing.T) {
	t.Parallel()
	c := setupTestCapture(t)

	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now().Add(-30 * time.Second)
	c.circuitReason = "rate_exceeded"
	c.rateLimitStreak = 2 // still over threshold
	c.evaluateCircuit()
	stillOpen := c.circuitOpen
	c.mu.Unlock()

	if !stillOpen {
		t.Error("expected circuit to remain open when rateLimitStreak > 0")
	}
}

// ============================================
// Coverage: rate_limit.go evaluateCircuit — lastBelowThresholdAt is zero (line 122)
// ============================================

func TestEvaluateCircuit_BelowThresholdAtZero(t *testing.T) {
	t.Parallel()
	c := setupTestCapture(t)

	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now().Add(-30 * time.Second)
	c.circuitReason = "rate_exceeded"
	c.rateLimitStreak = 0
	c.lastBelowThresholdAt = time.Time{} // zero
	c.evaluateCircuit()
	stillOpen := c.circuitOpen
	c.mu.Unlock()

	if !stillOpen {
		t.Error("expected circuit to remain open when lastBelowThresholdAt is zero")
	}
}
