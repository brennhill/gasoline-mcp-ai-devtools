// export_har_test.go — HAR export unit tests.
// Tests conversion logic, query string parsing, path safety, and filtering.
package export

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// TestNetworkBodyToHAREntry - Conversion tests
// ============================================

func TestNetworkBodyToHAREntry(t *testing.T) {
	t.Parallel()
	t.Run("basic GET", func(t *testing.T) {
		body := types.NetworkBody{
			Timestamp: "2026-01-23T10:30:00.000Z",
			Method:    "GET",
			URL:       "https://example.com/api/users",
			Status:    200,
			Duration:  142,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Request.Method != "GET" {
			t.Errorf("expected method GET, got %s", entry.Request.Method)
		}
		if entry.Request.URL != "https://example.com/api/users" {
			t.Errorf("expected URL https://example.com/api/users, got %s", entry.Request.URL)
		}
		if entry.Response.Status != 200 {
			t.Errorf("expected status 200, got %d", entry.Response.Status)
		}
		if entry.Response.StatusText != "OK" {
			t.Errorf("expected statusText OK, got %s", entry.Response.StatusText)
		}
		if entry.Time != 142 {
			t.Errorf("expected time 142, got %d", entry.Time)
		}
		if entry.StartedDateTime != "2026-01-23T10:30:00.000Z" {
			t.Errorf("expected startedDateTime 2026-01-23T10:30:00.000Z, got %s", entry.StartedDateTime)
		}
		if entry.Request.HTTPVersion != "HTTP/1.1" {
			t.Errorf("expected httpVersion HTTP/1.1, got %s", entry.Request.HTTPVersion)
		}
		if entry.Response.HTTPVersion != "HTTP/1.1" {
			t.Errorf("expected response httpVersion HTTP/1.1, got %s", entry.Response.HTTPVersion)
		}
		if entry.Request.HeadersSize != -1 {
			t.Errorf("expected headersSize -1, got %d", entry.Request.HeadersSize)
		}
		if entry.Response.HeadersSize != -1 {
			t.Errorf("expected response headersSize -1, got %d", entry.Response.HeadersSize)
		}
	})

	t.Run("POST with body", func(t *testing.T) {
		body := types.NetworkBody{
			Timestamp:   "2026-01-23T10:30:00.000Z",
			Method:      "POST",
			URL:         "https://example.com/api/users",
			Status:      201,
			Duration:    100,
			RequestBody: `{"name": "Alice"}`,
			ContentType: "application/json",
		}

		entry := networkBodyToHAREntry(body)

		if entry.Request.PostData == nil {
			t.Fatal("expected postData to be present")
		}
		if entry.Request.PostData.MimeType != "application/json" {
			t.Errorf("expected mimeType application/json, got %s", entry.Request.PostData.MimeType)
		}
		if entry.Request.PostData.Text != `{"name": "Alice"}` {
			t.Errorf("expected text to match request body, got %s", entry.Request.PostData.Text)
		}
		if entry.Request.BodySize != len(`{"name": "Alice"}`) {
			t.Errorf("expected bodySize %d, got %d", len(`{"name": "Alice"}`), entry.Request.BodySize)
		}
	})

	t.Run("no request body", func(t *testing.T) {
		body := types.NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api/users",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Request.PostData != nil {
			t.Error("expected postData to be nil for GET with no body")
		}
	})

	t.Run("truncated request", func(t *testing.T) {
		body := types.NetworkBody{
			Method:           "POST",
			URL:              "https://example.com/api",
			Status:           200,
			RequestBody:      "truncated...",
			ContentType:      "text/plain",
			RequestTruncated: true,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Request.Comment != "Body truncated at 8KB by Gasoline" {
			t.Errorf("expected truncation comment, got %q", entry.Request.Comment)
		}
	})

	t.Run("truncated response", func(t *testing.T) {
		body := types.NetworkBody{
			Method:            "GET",
			URL:               "https://example.com/api",
			Status:            200,
			ResponseBody:      "truncated...",
			ResponseTruncated: true,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Response.Comment != "Body truncated at 16KB by Gasoline" {
			t.Errorf("expected truncation comment, got %q", entry.Response.Comment)
		}
	})

	t.Run("query string parsing", func(t *testing.T) {
		body := types.NetworkBody{
			Method: "GET",
			URL:    "https://x.com/api?foo=bar&baz=1",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		if len(entry.Request.QueryString) != 2 {
			t.Fatalf("expected 2 queryString entries, got %d", len(entry.Request.QueryString))
		}
		found := false
		for _, q := range entry.Request.QueryString {
			if q.Name == "foo" && q.Value == "bar" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected queryString entry foo=bar")
		}
	})

	t.Run("unknown status code", func(t *testing.T) {
		body := types.NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api",
			Status: 999,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Response.StatusText != "" {
			t.Errorf("expected empty statusText for unknown code, got %q", entry.Response.StatusText)
		}
	})

	t.Run("duration maps to timings.wait", func(t *testing.T) {
		body := types.NetworkBody{
			Method:   "GET",
			URL:      "https://example.com/api",
			Status:   200,
			Duration: 250,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Timings.Wait != 250 {
			t.Errorf("expected timings.wait 250, got %d", entry.Timings.Wait)
		}
		if entry.Timings.Send != -1 {
			t.Errorf("expected timings.send -1, got %d", entry.Timings.Send)
		}
		if entry.Timings.Receive != -1 {
			t.Errorf("expected timings.receive -1, got %d", entry.Timings.Receive)
		}
	})

	t.Run("headers arrays are empty not nil", func(t *testing.T) {
		body := types.NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if strings.Contains(s, `"headers":null`) {
			t.Error("headers should be empty array, not null")
		}
		if strings.Contains(s, `"queryString":null`) {
			t.Error("queryString should be empty array, not null")
		}
	})

	t.Run("response body in content", func(t *testing.T) {
		body := types.NetworkBody{
			Method:       "GET",
			URL:          "https://example.com/api",
			Status:       200,
			ResponseBody: `{"id": 42}`,
			ContentType:  "application/json",
		}

		entry := networkBodyToHAREntry(body)

		if entry.Response.Content.Text != `{"id": 42}` {
			t.Errorf("expected response content text, got %s", entry.Response.Content.Text)
		}
		if entry.Response.Content.MimeType != "application/json" {
			t.Errorf("expected mimeType application/json, got %s", entry.Response.Content.MimeType)
		}
		if entry.Response.Content.Size != len(`{"id": 42}`) {
			t.Errorf("expected content size %d, got %d", len(`{"id": 42}`), entry.Response.Content.Size)
		}
		if entry.Response.BodySize != len(`{"id": 42}`) {
			t.Errorf("expected bodySize %d, got %d", len(`{"id": 42}`), entry.Response.BodySize)
		}
	})
}

// ============================================
// TestExportHAR - Full export tests
// ============================================

func TestExportHAR(t *testing.T) {
	t.Parallel()
	t.Run("empty - no network bodies", func(t *testing.T) {
		harLog := ExportHAR(nil, types.NetworkBodyFilter{}, "test")

		if harLog.Log.Version != "1.2" {
			t.Errorf("expected version 1.2, got %s", harLog.Log.Version)
		}
		if len(harLog.Log.Entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(harLog.Log.Entries))
		}

		data, _ := json.Marshal(harLog)
		if strings.Contains(string(data), `"entries":null`) {
			t.Error("entries should be empty array, not null")
		}
	})

	t.Run("multiple entries in chronological order", func(t *testing.T) {
		bodies := []types.NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/1", Status: 200},
			{Timestamp: "2026-01-23T10:30:01.000Z", Method: "GET", URL: "https://example.com/2", Status: 200},
			{Timestamp: "2026-01-23T10:30:02.000Z", Method: "GET", URL: "https://example.com/3", Status: 200},
		}

		harLog := ExportHAR(bodies, types.NetworkBodyFilter{}, "test")

		if len(harLog.Log.Entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(harLog.Log.Entries))
		}
		if harLog.Log.Entries[0].Request.URL != "https://example.com/1" {
			t.Errorf("expected first entry URL /1, got %s", harLog.Log.Entries[0].Request.URL)
		}
		if harLog.Log.Entries[2].Request.URL != "https://example.com/3" {
			t.Errorf("expected last entry URL /3, got %s", harLog.Log.Entries[2].Request.URL)
		}
	})

	t.Run("with method filter", func(t *testing.T) {
		bodies := []types.NetworkBody{
			{Method: "GET", URL: "https://example.com/1", Status: 200},
			{Method: "POST", URL: "https://example.com/2", Status: 201},
			{Method: "GET", URL: "https://example.com/3", Status: 200},
		}

		harLog := ExportHAR(bodies, types.NetworkBodyFilter{Method: "POST"}, "test")

		if len(harLog.Log.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(harLog.Log.Entries))
		}
		if harLog.Log.Entries[0].Request.Method != "POST" {
			t.Errorf("expected POST, got %s", harLog.Log.Entries[0].Request.Method)
		}
	})

	t.Run("with URL filter", func(t *testing.T) {
		bodies := []types.NetworkBody{
			{Method: "GET", URL: "https://example.com/api/users", Status: 200},
			{Method: "GET", URL: "https://example.com/static/app.js", Status: 200},
			{Method: "GET", URL: "https://example.com/api/posts", Status: 200},
		}

		harLog := ExportHAR(bodies, types.NetworkBodyFilter{URLFilter: "api"}, "test")

		if len(harLog.Log.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(harLog.Log.Entries))
		}
	})

	t.Run("with status filter", func(t *testing.T) {
		bodies := []types.NetworkBody{
			{Method: "GET", URL: "https://example.com/1", Status: 200},
			{Method: "GET", URL: "https://example.com/2", Status: 404},
			{Method: "GET", URL: "https://example.com/3", Status: 500},
		}

		harLog := ExportHAR(bodies, types.NetworkBodyFilter{StatusMin: 400}, "test")

		if len(harLog.Log.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(harLog.Log.Entries))
		}
	})

	t.Run("creator field", func(t *testing.T) {
		harLog := ExportHAR(nil, types.NetworkBodyFilter{}, "1.2.3")

		if harLog.Log.Creator.Name != "Gasoline" {
			t.Errorf("expected creator name Gasoline, got %s", harLog.Log.Creator.Name)
		}
		if harLog.Log.Creator.Version != "1.2.3" {
			t.Errorf("expected creator version 1.2.3, got %s", harLog.Log.Creator.Version)
		}
	})
}

// ============================================
// TestExportHARToFile - File save tests
// ============================================

func TestExportHARToFile(t *testing.T) {
	t.Parallel()
	t.Run("save to tmp", func(t *testing.T) {
		bodies := []types.NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/1", Status: 200},
			{Timestamp: "2026-01-23T10:30:01.000Z", Method: "POST", URL: "https://example.com/2", Status: 201},
		}

		tmpFile := "/tmp/gasoline-test-har-export.har"
		defer os.Remove(tmpFile)

		result, err := ExportHARToFile(bodies, types.NetworkBodyFilter{}, "test", tmpFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.SavedTo != tmpFile {
			t.Errorf("expected saved_to %s, got %s", tmpFile, result.SavedTo)
		}
		if result.EntriesCount != 2 {
			t.Errorf("expected entries_count 2, got %d", result.EntriesCount)
		}
		if result.FileSizeBytes <= 0 {
			t.Errorf("expected positive file_size_bytes, got %d", result.FileSizeBytes)
		}

		data, err := os.ReadFile(tmpFile) // nosemgrep: go_filesystem_rule-fileread — test reads output
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		var harLog HARLog
		if err := json.Unmarshal(data, &harLog); err != nil {
			t.Fatalf("file content is not valid HAR JSON: %v", err)
		}
		if len(harLog.Log.Entries) != 2 {
			t.Errorf("expected 2 entries in file, got %d", len(harLog.Log.Entries))
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		_, err := ExportHARToFile(nil, types.NetworkBodyFilter{}, "test", "../../etc/passwd")
		if err == nil {
			t.Error("expected error for path traversal")
		}
	})

	t.Run("absolute path outside tmp rejected", func(t *testing.T) {
		_, err := ExportHARToFile(nil, types.NetworkBodyFilter{}, "test", "/etc/hosts")
		if err == nil {
			t.Error("expected error for absolute path outside tmp")
		}
	})

	t.Run("nonexistent parent dir", func(t *testing.T) {
		_, err := ExportHARToFile(
			[]types.NetworkBody{{Method: "GET", URL: "https://example.com", Status: 200}},
			types.NetworkBodyFilter{}, "test",
			"/tmp/gasoline-test-nonexist/deep/nested/file.har",
		)
		if err == nil {
			t.Error("expected error for nonexistent parent dir")
		}
	})
}

// ============================================
// TestIsPathSafe - Path validation tests
// ============================================

func TestIsPathSafe(t *testing.T) {
	t.Parallel()
	// Use runtime temp dir to avoid sandbox TMPDIR mismatch
	runtimeTmpDir := os.TempDir()

	tests := []struct {
		name string
		path string
		safe bool
	}{
		{"tmp absolute path", "/tmp/test.har", true},
		{"os tempdir", runtimeTmpDir + "/test.har", true},
		{"relative simple", "output.har", true},
		{"relative subdir", "reports/output.har", true},
		{"traversal", "../../etc/passwd", false},
		{"absolute outside tmp", "/etc/hosts", false},
		{"absolute user dir", "/home/user/file.har", false},
		{"dot-dot in middle", "foo/../../../etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathSafe(tt.path)
			if got != tt.safe {
				t.Errorf("isPathSafe(%q) = %v, want %v", tt.path, got, tt.safe)
			}
		})
	}
}

// ============================================
// TestParseQueryString - Query string parsing
// ============================================

func TestParseQueryString(t *testing.T) {
	t.Parallel()
	t.Run("basic params", func(t *testing.T) {
		result := parseQueryString("https://example.com/api?foo=bar&baz=1")
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
	})

	t.Run("empty query", func(t *testing.T) {
		result := parseQueryString("https://example.com/api")
		if len(result) != 0 {
			t.Errorf("expected 0 entries, got %d", len(result))
		}
	})

	t.Run("encoded values", func(t *testing.T) {
		result := parseQueryString("https://example.com/api?key=hello+world&val=a%26b")
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
		found := false
		for _, q := range result {
			if q.Name == "key" && q.Value == "hello world" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected decoded queryString entry key=hello world")
		}
	})

	t.Run("invalid URL returns empty", func(t *testing.T) {
		result := parseQueryString("://not-a-url")
		if len(result) != 0 {
			t.Errorf("expected 0 entries for invalid URL, got %d", len(result))
		}
	})
}
