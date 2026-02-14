// export_coverage_test.go — Targeted coverage tests for uncovered export paths.
// Covers: ExportHARMergedToFile, httpStatusText branches, computeWaterfallTimings edge cases,
// matchesWaterfallFilter, matchesHARFilter, and SARIF path validation.
package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// httpStatusText — Cover all status code branches
// ============================================

func TestHttpStatusText_AllCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code int
		want string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{0, ""},
		{418, ""},
		{999, ""},
	}

	for _, tc := range cases {
		got := httpStatusText(tc.code)
		if got != tc.want {
			t.Errorf("httpStatusText(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

// ============================================
// computeWaterfallTimings — Edge cases
// ============================================

func TestComputeWaterfallTimings_ZeroFetchStart(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{
		Duration:    100.0,
		StartTime:   0,
		FetchStart:  0, // zero => early return
		ResponseEnd: 100.0,
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != -1 {
		t.Errorf("send = %d, want -1", send)
	}
	if wait != 100 {
		t.Errorf("wait = %d, want 100", wait)
	}
	if receive != -1 {
		t.Errorf("receive = %d, want -1", receive)
	}
}

func TestComputeWaterfallTimings_ZeroResponseEnd(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{
		Duration:    50.0,
		StartTime:   10.0,
		FetchStart:  15.0,
		ResponseEnd: 0, // zero => early return
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != -1 {
		t.Errorf("send = %d, want -1", send)
	}
	if wait != 50 {
		t.Errorf("wait = %d, want 50", wait)
	}
	if receive != -1 {
		t.Errorf("receive = %d, want -1", receive)
	}
}

func TestComputeWaterfallTimings_NegativeSendClampsToZero(t *testing.T) {
	t.Parallel()

	// FetchStart < StartTime => sendF would be negative, clamped to 0
	wf := types.NetworkWaterfallEntry{
		Duration:    100.0,
		StartTime:   20.0,
		FetchStart:  10.0, // less than StartTime
		ResponseEnd: 120.0,
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != 0 {
		t.Errorf("send = %d, want 0 (clamped from negative)", send)
	}
	if wait < 0 {
		t.Errorf("wait = %d, want non-negative", wait)
	}
	if receive != 0 {
		t.Errorf("receive = %d, want 0", receive)
	}
}

func TestComputeWaterfallTimings_NegativeRemainClampsToZero(t *testing.T) {
	t.Parallel()

	// ResponseEnd < FetchStart => remainF would be negative, clamped to 0
	wf := types.NetworkWaterfallEntry{
		Duration:    100.0,
		StartTime:   10.0,
		FetchStart:  50.0,
		ResponseEnd: 30.0, // less than FetchStart
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != 40 {
		t.Errorf("send = %d, want 40", send)
	}
	if wait != 0 {
		t.Errorf("wait = %d, want 0 (clamped from negative)", wait)
	}
	if receive != 0 {
		t.Errorf("receive = %d, want 0", receive)
	}
}

func TestComputeWaterfallTimings_ValidValues(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{
		Duration:    100.0,
		StartTime:   100.0,
		FetchStart:  110.0,
		ResponseEnd: 200.0,
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != 10 {
		t.Errorf("send = %d, want 10", send)
	}
	if wait != 90 {
		t.Errorf("wait = %d, want 90", wait)
	}
	if receive != 0 {
		t.Errorf("receive = %d, want 0", receive)
	}
}

// ============================================
// matchesWaterfallFilter — Method filter
// ============================================

func TestMatchesWaterfallFilter_MethodFilter(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{URL: "https://example.com/api"}

	// Waterfall entries are implicitly GET
	if !matchesWaterfallFilter(wf, types.NetworkBodyFilter{Method: "GET"}) {
		t.Error("expected GET filter to match waterfall entry")
	}
	if !matchesWaterfallFilter(wf, types.NetworkBodyFilter{Method: "get"}) {
		t.Error("expected case-insensitive GET match")
	}
	if matchesWaterfallFilter(wf, types.NetworkBodyFilter{Method: "POST"}) {
		t.Error("expected POST filter to reject waterfall entry")
	}
	if matchesWaterfallFilter(wf, types.NetworkBodyFilter{Method: "PUT"}) {
		t.Error("expected PUT filter to reject waterfall entry")
	}
}

func TestMatchesWaterfallFilter_URLFilter(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{URL: "https://example.com/API/users"}

	if !matchesWaterfallFilter(wf, types.NetworkBodyFilter{URLFilter: "api"}) {
		t.Error("URL filter 'api' should match (case-insensitive)")
	}
	if matchesWaterfallFilter(wf, types.NetworkBodyFilter{URLFilter: "posts"}) {
		t.Error("URL filter 'posts' should not match")
	}
}

func TestMatchesWaterfallFilter_NoFilter(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{URL: "https://example.com/anything"}
	if !matchesWaterfallFilter(wf, types.NetworkBodyFilter{}) {
		t.Error("empty filter should match everything")
	}
}

// ============================================
// matchesHARFilter — StatusMax branch
// ============================================

func TestMatchesHARFilter_StatusMax(t *testing.T) {
	t.Parallel()

	body := types.NetworkBody{Method: "GET", URL: "https://example.com", Status: 500}

	if matchesHARFilter(body, types.NetworkBodyFilter{StatusMax: 399}) {
		t.Error("status 500 should not pass StatusMax=399 filter")
	}
	if !matchesHARFilter(body, types.NetworkBodyFilter{StatusMax: 500}) {
		t.Error("status 500 should pass StatusMax=500 filter")
	}
	if !matchesHARFilter(body, types.NetworkBodyFilter{StatusMax: 599}) {
		t.Error("status 500 should pass StatusMax=599 filter")
	}
}

func TestMatchesHARFilter_StatusMinAndMax(t *testing.T) {
	t.Parallel()

	body := types.NetworkBody{Method: "GET", URL: "https://example.com", Status: 404}

	if !matchesHARFilter(body, types.NetworkBodyFilter{StatusMin: 400, StatusMax: 499}) {
		t.Error("status 404 should pass 400-499 range")
	}
	if matchesHARFilter(body, types.NetworkBodyFilter{StatusMin: 500, StatusMax: 599}) {
		t.Error("status 404 should not pass 500-599 range")
	}
}

// ============================================
// ExportHARMergedToFile — Full file export with merged data
// ============================================

func TestExportHARMergedToFile_Success(t *testing.T) {
	t.Parallel()

	bodies := []types.NetworkBody{
		{
			Timestamp:    "2026-01-23T10:30:00.000Z",
			Method:       "GET",
			URL:          "https://api.example.com/data",
			Status:       200,
			ResponseBody: `{"ok":true}`,
			ContentType:  "application/json",
			Duration:     142,
		},
	}
	waterfall := []types.NetworkWaterfallEntry{
		{
			URL:             "https://cdn.example.com/app.js",
			Duration:        80.0,
			StartTime:       100.0,
			FetchStart:      105.0,
			ResponseEnd:     180.0,
			DecodedBodySize: 8192,
			EncodedBodySize: 4096,
			Timestamp:       time.Now(),
		},
	}

	tmpFile := "/tmp/gasoline-test-har-merged-export.har"
	defer os.Remove(tmpFile)

	result, err := ExportHARMergedToFile(bodies, waterfall, types.NetworkBodyFilter{}, "6.0.3", tmpFile)
	if err != nil {
		t.Fatalf("ExportHARMergedToFile() error = %v", err)
	}

	if result.SavedTo != tmpFile {
		t.Errorf("saved_to = %q, want %q", result.SavedTo, tmpFile)
	}
	if result.EntriesCount != 2 {
		t.Errorf("entries_count = %d, want 2", result.EntriesCount)
	}
	if result.FileSizeBytes <= 0 {
		t.Errorf("file_size_bytes = %d, want positive", result.FileSizeBytes)
	}

	// Verify file content is valid HAR JSON
	data, err := os.ReadFile(tmpFile) // nosemgrep: go_filesystem_rule-fileread -- test reads output
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var harLog HARLog
	if err := json.Unmarshal(data, &harLog); err != nil {
		t.Fatalf("file is not valid HAR JSON: %v", err)
	}
	if harLog.Log.Version != "1.2" {
		t.Errorf("version = %q, want 1.2", harLog.Log.Version)
	}
	if len(harLog.Log.Entries) != 2 {
		t.Errorf("entries = %d, want 2", len(harLog.Log.Entries))
	}
}

func TestExportHARMergedToFile_PathTraversal(t *testing.T) {
	t.Parallel()

	_, err := ExportHARMergedToFile(nil, nil, types.NetworkBodyFilter{}, "test", "../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "unsafe path") {
		t.Errorf("expected 'unsafe path' error, got: %v", err)
	}
}

func TestExportHARMergedToFile_UnsafeAbsolutePath(t *testing.T) {
	t.Parallel()

	_, err := ExportHARMergedToFile(nil, nil, types.NetworkBodyFilter{}, "test", "/etc/hosts")
	if err == nil {
		t.Error("expected error for absolute path outside tmp")
	}
}

func TestExportHARMergedToFile_NonexistentDir(t *testing.T) {
	t.Parallel()

	_, err := ExportHARMergedToFile(
		[]types.NetworkBody{{Method: "GET", URL: "https://example.com", Status: 200}},
		nil, types.NetworkBodyFilter{}, "test",
		"/tmp/gasoline-nonexist-merged/deep/file.har",
	)
	if err == nil {
		t.Error("expected error for nonexistent parent dir")
	}
}

func TestExportHARMergedToFile_EmptyData(t *testing.T) {
	t.Parallel()

	tmpFile := "/tmp/gasoline-test-har-merged-empty.har"
	defer os.Remove(tmpFile)

	result, err := ExportHARMergedToFile(nil, nil, types.NetworkBodyFilter{}, "test", tmpFile)
	if err != nil {
		t.Fatalf("ExportHARMergedToFile() error = %v", err)
	}
	if result.EntriesCount != 0 {
		t.Errorf("entries_count = %d, want 0", result.EntriesCount)
	}
}

// ============================================
// ExportHARToFile — Additional edge cases
// ============================================

func TestExportHARToFile_PrivateTmpPath(t *testing.T) {
	t.Parallel()

	// On macOS /tmp is a symlink to /private/tmp
	bodies := []types.NetworkBody{
		{Method: "GET", URL: "https://example.com", Status: 200, Timestamp: "2026-01-01T00:00:00Z"},
	}

	tmpFile := "/private/tmp/gasoline-test-har-private.har"
	defer os.Remove(tmpFile)

	result, err := ExportHARToFile(bodies, types.NetworkBodyFilter{}, "test", tmpFile)
	if err != nil {
		t.Fatalf("ExportHARToFile() error = %v", err)
	}
	if result.EntriesCount != 1 {
		t.Errorf("entries_count = %d, want 1", result.EntriesCount)
	}
}

// ============================================
// isPathSafe — Additional branches
// ============================================

func TestIsPathSafe_PrivateTmpPrefix(t *testing.T) {
	t.Parallel()

	if !isPathSafe("/private/tmp/test.har") {
		t.Error("expected /private/tmp/ path to be safe")
	}
}

// ============================================
// SARIF: isPathUnderResolvedDir — EvalSymlinks error
// ============================================

func TestIsPathUnderResolvedDir_NonexistentDir(t *testing.T) {
	t.Parallel()

	// Pass a non-existent directory — EvalSymlinks should fail
	result := isPathUnderResolvedDir("/tmp/some/file.sarif", "/nonexistent/dir/that/does/not/exist")
	if result {
		t.Error("expected false for non-existent directory")
	}
}

// ============================================
// SARIF: validateSARIFSavePath — Additional paths
// ============================================

func TestValidateSARIFSavePath_TempDirPath(t *testing.T) {
	t.Parallel()

	tmpDir := os.TempDir()
	resolvedTmp, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Skipf("cannot resolve temp dir: %v", err)
	}

	testPath := filepath.Join(resolvedTmp, "gasoline-test", "output.sarif")
	err = validateSARIFSavePath(testPath, testPath)
	if err != nil {
		t.Errorf("validateSARIFSavePath under temp dir should succeed, got: %v", err)
	}
}

// ============================================
// SARIF: ensureRule with no WCAG tags
// ============================================

func TestEnsureRule_NoWCAGTags(t *testing.T) {
	t.Parallel()

	run := &SARIFRun{
		Tool: SARIFTool{
			Driver: SARIFDriver{Rules: []SARIFRule{}},
		},
		Results: []SARIFResult{},
	}
	indices := make(map[string]int)

	// Violation with no WCAG tags
	v := axeViolation{
		ID:          "test-rule",
		Description: "Test description",
		Help:        "Test help",
		Tags:        []string{"cat.aria", "TTv5"},
	}

	idx := ensureRule(run, indices, v)
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}

	rule := run.Tool.Driver.Rules[0]
	if rule.Properties != nil {
		t.Error("expected nil Properties when no WCAG tags")
	}
}

// ============================================
// SARIF: nodeToResult with empty target
// ============================================

func TestNodeToResult_EmptyTarget(t *testing.T) {
	t.Parallel()

	v := axeViolation{ID: "test", Help: "Help text"}
	node := axeNode{HTML: "<div></div>", Target: []string{}}

	result := nodeToResult(v, node, 0, "error")
	if result.Locations[0].PhysicalLocation.ArtifactLocation.URI != "" {
		t.Errorf("expected empty URI for empty target, got %q",
			result.Locations[0].PhysicalLocation.ArtifactLocation.URI)
	}
}

// ============================================
// ExportHAR JSON field naming validation
// ============================================

func TestExportHAR_JSONFieldNames(t *testing.T) {
	t.Parallel()

	bodies := []types.NetworkBody{
		{
			Timestamp:    "2026-01-01T00:00:00Z",
			Method:       "POST",
			URL:          "https://example.com/api?q=test",
			Status:       201,
			Duration:     50,
			RequestBody:  `{"name":"Alice"}`,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
			ResponseHeaders: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	harLog := ExportHAR(bodies, types.NetworkBodyFilter{}, "6.0.3")
	data, err := json.Marshal(harLog)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	s := string(data)

	// Verify SPEC:HAR camelCase fields are present
	requiredFields := []string{
		`"startedDateTime"`, `"httpVersion"`, `"queryString"`,
		`"headersSize"`, `"bodySize"`, `"statusText"`, `"mimeType"`,
		`"postData"`,
	}
	for _, field := range requiredFields {
		if !strings.Contains(s, field) {
			t.Errorf("expected HAR field %s in JSON output", field)
		}
	}
}

// ============================================
// HARExportResult JSON field names (snake_case)
// ============================================

func TestHARExportResult_JSONFields(t *testing.T) {
	t.Parallel()

	result := HARExportResult{
		SavedTo:       "/tmp/test.har",
		EntriesCount:  5,
		FileSizeBytes: 1234,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	s := string(data)

	// Our own result type must use snake_case
	if !strings.Contains(s, `"saved_to"`) {
		t.Error("expected snake_case field 'saved_to'")
	}
	if !strings.Contains(s, `"entries_count"`) {
		t.Error("expected snake_case field 'entries_count'")
	}
	if !strings.Contains(s, `"file_size_bytes"`) {
		t.Error("expected snake_case field 'file_size_bytes'")
	}
}
