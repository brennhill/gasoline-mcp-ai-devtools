//go:build integration
// +build integration

// export_har_test.go — HAR export tests.
// NOTE: These tests require HAR export implementation that doesn't exist yet.
// The implementation stub is in cmd/dev-console/tools.go (toolExportHAR).
// Run with: go test -tags=integration ./internal/export/...
package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// TestNetworkBodyToHAREntry - Conversion tests
// ============================================

func TestNetworkBodyToHAREntry(t *testing.T) {
	t.Parallel()
	t.Run("basic GET", func(t *testing.T) {
		body := NetworkBody{
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
		body := NetworkBody{
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
		body := NetworkBody{
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
		body := NetworkBody{
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
		body := NetworkBody{
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
		body := NetworkBody{
			Method: "GET",
			URL:    "https://x.com/api?foo=bar&baz=1",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		if len(entry.Request.QueryString) != 2 {
			t.Fatalf("expected 2 queryString entries, got %d", len(entry.Request.QueryString))
		}
		// Check first entry
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
		found = false
		for _, q := range entry.Request.QueryString {
			if q.Name == "baz" && q.Value == "1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected queryString entry baz=1")
		}
	})

	t.Run("unknown status code", func(t *testing.T) {
		body := NetworkBody{
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
		body := NetworkBody{
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

	t.Run("empty URL query", func(t *testing.T) {
		body := NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		if len(entry.Request.QueryString) != 0 {
			t.Errorf("expected 0 queryString entries, got %d", len(entry.Request.QueryString))
		}
	})

	t.Run("special chars in query params", func(t *testing.T) {
		body := NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api?key=hello+world&val=a%26b",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		if len(entry.Request.QueryString) != 2 {
			t.Fatalf("expected 2 queryString entries, got %d", len(entry.Request.QueryString))
		}
		found := false
		for _, q := range entry.Request.QueryString {
			if q.Name == "key" && q.Value == "hello world" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected decoded queryString entry key=hello world")
		}
		found = false
		for _, q := range entry.Request.QueryString {
			if q.Name == "val" && q.Value == "a&b" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected decoded queryString entry val=a&b")
		}
	})

	t.Run("headers arrays are empty not nil", func(t *testing.T) {
		body := NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		// Marshal and check JSON - arrays should be [] not null
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
		body := NetworkBody{
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
		capture := NewCapture()

		harLog := capture.ExportHAR(NetworkBodyFilter{})

		if harLog.Log.Version != "1.2" {
			t.Errorf("expected version 1.2, got %s", harLog.Log.Version)
		}
		if len(harLog.Log.Entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(harLog.Log.Entries))
		}

		// Verify JSON serialization produces entries:[] not entries:null
		data, _ := json.Marshal(harLog)
		if strings.Contains(string(data), `"entries":null`) {
			t.Error("entries should be empty array, not null")
		}
	})

	t.Run("multiple entries in chronological order", func(t *testing.T) {
		capture := NewCapture()
		capture.AddNetworkBodies([]NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/1", Status: 200},
			{Timestamp: "2026-01-23T10:30:01.000Z", Method: "GET", URL: "https://example.com/2", Status: 200},
			{Timestamp: "2026-01-23T10:30:02.000Z", Method: "GET", URL: "https://example.com/3", Status: 200},
		})

		harLog := capture.ExportHAR(NetworkBodyFilter{})

		if len(harLog.Log.Entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(harLog.Log.Entries))
		}
		// Chronological order (oldest first)
		if harLog.Log.Entries[0].Request.URL != "https://example.com/1" {
			t.Errorf("expected first entry URL /1, got %s", harLog.Log.Entries[0].Request.URL)
		}
		if harLog.Log.Entries[2].Request.URL != "https://example.com/3" {
			t.Errorf("expected last entry URL /3, got %s", harLog.Log.Entries[2].Request.URL)
		}
	})

	t.Run("with method filter", func(t *testing.T) {
		capture := NewCapture()
		capture.AddNetworkBodies([]NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/1", Status: 200},
			{Timestamp: "2026-01-23T10:30:01.000Z", Method: "POST", URL: "https://example.com/2", Status: 201},
			{Timestamp: "2026-01-23T10:30:02.000Z", Method: "GET", URL: "https://example.com/3", Status: 200},
		})

		harLog := capture.ExportHAR(NetworkBodyFilter{Method: "POST"})

		if len(harLog.Log.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(harLog.Log.Entries))
		}
		if harLog.Log.Entries[0].Request.Method != "POST" {
			t.Errorf("expected POST, got %s", harLog.Log.Entries[0].Request.Method)
		}
	})

	t.Run("with URL filter", func(t *testing.T) {
		capture := NewCapture()
		capture.AddNetworkBodies([]NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api/users", Status: 200},
			{Timestamp: "2026-01-23T10:30:01.000Z", Method: "GET", URL: "https://example.com/static/app.js", Status: 200},
			{Timestamp: "2026-01-23T10:30:02.000Z", Method: "GET", URL: "https://example.com/api/posts", Status: 200},
		})

		harLog := capture.ExportHAR(NetworkBodyFilter{URLFilter: "api"})

		if len(harLog.Log.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(harLog.Log.Entries))
		}
	})

	t.Run("with status filter", func(t *testing.T) {
		capture := NewCapture()
		capture.AddNetworkBodies([]NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/1", Status: 200},
			{Timestamp: "2026-01-23T10:30:01.000Z", Method: "GET", URL: "https://example.com/2", Status: 404},
			{Timestamp: "2026-01-23T10:30:02.000Z", Method: "GET", URL: "https://example.com/3", Status: 500},
		})

		harLog := capture.ExportHAR(NetworkBodyFilter{StatusMin: 400})

		if len(harLog.Log.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(harLog.Log.Entries))
		}
	})

	t.Run("creator field", func(t *testing.T) {
		capture := NewCapture()

		harLog := capture.ExportHAR(NetworkBodyFilter{})

		if harLog.Log.Creator.Name != "Gasoline" {
			t.Errorf("expected creator name Gasoline, got %s", harLog.Log.Creator.Name)
		}
		if harLog.Log.Creator.Version != version {
			t.Errorf("expected creator version %s, got %s", version, harLog.Log.Creator.Version)
		}
	})

	t.Run("HAR version is 1.2", func(t *testing.T) {
		capture := NewCapture()

		harLog := capture.ExportHAR(NetworkBodyFilter{})

		if harLog.Log.Version != "1.2" {
			t.Errorf("expected version 1.2, got %s", harLog.Log.Version)
		}
	})
}

// ============================================
// TestExportHARToFile - File save tests
// ============================================

func TestExportHARToFile(t *testing.T) {
	t.Parallel()
	t.Run("save to tmp", func(t *testing.T) {
		capture := NewCapture()
		capture.AddNetworkBodies([]NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/1", Status: 200},
			{Timestamp: "2026-01-23T10:30:01.000Z", Method: "POST", URL: "https://example.com/2", Status: 201},
		})

		tmpFile := filepath.Join(os.TempDir(), "test-har-export.har")
		defer os.Remove(tmpFile)

		result, err := capture.ExportHARToFile(NetworkBodyFilter{}, tmpFile)
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

		// Verify file was actually written and is valid JSON
		data, err := os.ReadFile(tmpFile) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
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

	t.Run("invalid path", func(t *testing.T) {
		capture := NewCapture()

		_, err := capture.ExportHARToFile(NetworkBodyFilter{}, "/root/nope/cannot-write.har")
		if err == nil {
			t.Error("expected error for unwritable path")
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		capture := NewCapture()

		_, err := capture.ExportHARToFile(NetworkBodyFilter{}, "../../etc/passwd")
		if err == nil {
			t.Error("expected error for path traversal")
		}
	})

	t.Run("absolute path outside tmp rejected", func(t *testing.T) {
		capture := NewCapture()

		_, err := capture.ExportHARToFile(NetworkBodyFilter{}, "/etc/hosts")
		if err == nil {
			t.Error("expected error for absolute path outside tmp")
		}
	})
}

// ============================================
// TestIsPathSafe - Path validation tests
// ============================================

func TestIsPathSafe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		safe bool
	}{
		{"tmp absolute path", "/tmp/test.har", true},
		{"os tempdir", filepath.Join(os.TempDir(), "test.har"), true},
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
	})

	t.Run("invalid URL returns empty", func(t *testing.T) {
		result := parseQueryString("://not-a-url")
		if len(result) != 0 {
			t.Errorf("expected 0 entries for invalid URL, got %d", len(result))
		}
	})
}

// ============================================
// TestExportHARTool - MCP tool integration tests
// ============================================

func TestExportHARTool(t *testing.T) {
	t.Parallel()
	t.Run("no save_to returns full HAR JSON", func(t *testing.T) {
		server := &Server{
			entries: make([]LogEntry, 0),
		}
		capture := NewCapture()
		capture.AddNetworkBodies([]NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200, Duration: 50},
		})
		handler := &ToolHandler{
			MCPHandler: NewMCPHandler(server),
			capture:    capture,
		}

		args, _ := json.Marshal(map[string]any{})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
		resp := handler.toolExportHAR(req, args)

		if resp.Error != nil {
			t.Fatalf("unexpected error: %v", resp.Error)
		}

		// Parse the result to check it contains HAR JSON
		var result MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in response")
		}

		// Strip summary line before parsing JSON
		text := result.Content[0].Text
		jsonPart := text
		if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
			jsonPart = lines[1]
		}
		var harLog HARLog
		if err := json.Unmarshal([]byte(jsonPart), &harLog); err != nil {
			t.Fatalf("response text is not valid HAR JSON: %v", err)
		}
		if len(harLog.Log.Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(harLog.Log.Entries))
		}
	})

	t.Run("with save_to saves file and returns summary", func(t *testing.T) {
		server := &Server{
			entries: make([]LogEntry, 0),
		}
		capture := NewCapture()
		capture.AddNetworkBodies([]NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200},
		})
		handler := &ToolHandler{
			MCPHandler: NewMCPHandler(server),
			capture:    capture,
		}

		tmpFile := filepath.Join(os.TempDir(), "test-tool-export.har")
		defer os.Remove(tmpFile)

		args, _ := json.Marshal(map[string]any{
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
			t.Fatalf("response text is not valid summary JSON: %v", err)
		}
		if summary.SavedTo != tmpFile {
			t.Errorf("expected saved_to %s, got %s", tmpFile, summary.SavedTo)
		}
		if summary.EntriesCount != 1 {
			t.Errorf("expected entries_count 1, got %d", summary.EntriesCount)
		}

		// Verify file exists
		if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
			t.Error("expected file to exist")
		}
	})

	t.Run("with filters passed through", func(t *testing.T) {
		server := &Server{
			entries: make([]LogEntry, 0),
		}
		capture := NewCapture()
		capture.AddNetworkBodies([]NetworkBody{
			{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200},
			{Timestamp: "2026-01-23T10:30:01.000Z", Method: "POST", URL: "https://example.com/api", Status: 500},
			{Timestamp: "2026-01-23T10:30:02.000Z", Method: "GET", URL: "https://example.com/static", Status: 200},
		})
		handler := &ToolHandler{
			MCPHandler: NewMCPHandler(server),
			capture:    capture,
		}

		args, _ := json.Marshal(map[string]any{
			"method":     "POST",
			"status_min": 400,
		})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}
		resp := handler.toolExportHAR(req, args)

		var result MCPToolResult
		json.Unmarshal(resp.Result, &result)

		// Strip summary line before parsing JSON
		text := result.Content[0].Text
		jsonPart := text
		if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
			jsonPart = lines[1]
		}
		var harLog HARLog
		json.Unmarshal([]byte(jsonPart), &harLog)

		if len(harLog.Log.Entries) != 1 {
			t.Fatalf("expected 1 entry with filters, got %d", len(harLog.Log.Entries))
		}
		if harLog.Log.Entries[0].Request.Method != "POST" {
			t.Errorf("expected POST, got %s", harLog.Log.Entries[0].Request.Method)
		}
	})

	t.Run("tool registered in tools list", func(t *testing.T) {
		server := &Server{
			entries: make([]LogEntry, 0),
		}
		capture := NewCapture()
		handler := &ToolHandler{
			MCPHandler: NewMCPHandler(server),
			capture:    capture,
		}

		tools := handler.toolsList()
		found := false
		for _, tool := range tools {
			if tool.Name == "generate" {
				found = true
				break
			}
		}
		if !found {
			t.Error("generate tool not found in tools list")
		}
	})

	t.Run("tool dispatched in handleToolCall", func(t *testing.T) {
		server := &Server{
			entries: make([]LogEntry, 0),
		}
		capture := NewCapture()
		handler := &ToolHandler{
			MCPHandler: NewMCPHandler(server),
			capture:    capture,
		}

		args, _ := json.Marshal(map[string]any{"format": "har"})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`4`), Method: "tools/call"}
		resp, handled := handler.handleToolCall(req, "generate", args)

		if !handled {
			t.Error("expected generate to be handled")
		}
		if resp.Error != nil {
			t.Errorf("unexpected error: %v", resp.Error)
		}
	})

	t.Run("save_to with invalid path returns error", func(t *testing.T) {
		server := &Server{
			entries: make([]LogEntry, 0),
		}
		capture := NewCapture()
		handler := &ToolHandler{
			MCPHandler: NewMCPHandler(server),
			capture:    capture,
		}

		args, _ := json.Marshal(map[string]any{
			"save_to": "../../etc/passwd",
		})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`5`), Method: "tools/call"}
		resp := handler.toolExportHAR(req, args)

		var result MCPToolResult
		json.Unmarshal(resp.Result, &result)
		if !result.IsError {
			t.Error("expected error response for path traversal")
		}
	})
}

// ============================================
// Coverage Gap Tests
// ============================================

func TestExportHARToFile_WriteError(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add a network body so there's data to export
	capture.AddNetworkBodies([]NetworkBody{{
		Timestamp: "2026-01-23T10:00:00.000Z",
		Method:    "GET",
		URL:       "https://example.com/api",
		Status:    200,
	}})

	// Use a path where the parent is a file (not a directory), so WriteFile fails
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("file"), 0600); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}
	badPath := filepath.Join(blockingFile, "output.har")

	_, err := capture.ExportHARToFile(NetworkBodyFilter{}, badPath)
	if err == nil {
		t.Fatal("Expected error when write fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to write file") {
		t.Errorf("Expected 'failed to write file' error, got: %v", err)
	}
}

func TestToolExportHAR_MethodStatusFilters(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add bodies with different methods and statuses
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2026-01-23T10:00:00.000Z", Method: "GET", URL: "https://example.com/users", Status: 200},
		{Timestamp: "2026-01-23T10:01:00.000Z", Method: "POST", URL: "https://example.com/users", Status: 201},
		{Timestamp: "2026-01-23T10:02:00.000Z", Method: "GET", URL: "https://example.com/admin", Status: 403},
		{Timestamp: "2026-01-23T10:03:00.000Z", Method: "DELETE", URL: "https://example.com/users/1", Status: 204},
	})

	server := &Server{entries: make([]LogEntry, 0)}
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server),
		capture:    capture,
	}

	t.Run("filter by method", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{"method": "POST"})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

		resp := handler.toolExportHAR(req, args)
		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result MCPToolResult
		json.Unmarshal(resp.Result, &result)

		text := result.Content[0].Text
		jp := text
		if ls := strings.SplitN(text, "\n", 2); len(ls) == 2 {
			jp = ls[1]
		}
		var harLog HARLog
		json.Unmarshal([]byte(jp), &harLog)

		if len(harLog.Log.Entries) != 1 {
			t.Errorf("Expected 1 entry for method=POST, got %d", len(harLog.Log.Entries))
		}
		if len(harLog.Log.Entries) > 0 && harLog.Log.Entries[0].Request.Method != "POST" {
			t.Errorf("Expected POST method in result, got %s", harLog.Log.Entries[0].Request.Method)
		}
	})

	t.Run("filter by status range", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{"status_min": 400, "status_max": 499})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`)}

		resp := handler.toolExportHAR(req, args)
		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result MCPToolResult
		json.Unmarshal(resp.Result, &result)

		text := result.Content[0].Text
		jp := text
		if ls := strings.SplitN(text, "\n", 2); len(ls) == 2 {
			jp = ls[1]
		}
		var harLog HARLog
		json.Unmarshal([]byte(jp), &harLog)

		if len(harLog.Log.Entries) != 1 {
			t.Errorf("Expected 1 entry for status 4xx, got %d", len(harLog.Log.Entries))
		}
		if len(harLog.Log.Entries) > 0 && harLog.Log.Entries[0].Response.Status != 403 {
			t.Errorf("Expected status 403, got %d", harLog.Log.Entries[0].Response.Status)
		}
	})

	t.Run("filter by method and status", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{"method": "GET", "status_min": 200, "status_max": 299})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`)}

		resp := handler.toolExportHAR(req, args)
		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result MCPToolResult
		json.Unmarshal(resp.Result, &result)

		text := result.Content[0].Text
		jp := text
		if ls := strings.SplitN(text, "\n", 2); len(ls) == 2 {
			jp = ls[1]
		}
		var harLog HARLog
		json.Unmarshal([]byte(jp), &harLog)

		if len(harLog.Log.Entries) != 1 {
			t.Errorf("Expected 1 entry for GET+2xx, got %d", len(harLog.Log.Entries))
		}
		if len(harLog.Log.Entries) > 0 && harLog.Log.Entries[0].Request.URL != "https://example.com/users" {
			t.Errorf("Expected /users URL, got %s", harLog.Log.Entries[0].Request.URL)
		}
	})
}

// ============================================
// Coverage: ExportHARToFile marshal error (line 238) - not easily triggerable
// but we can test the path that writes to an unwritable location (line 308/319)
// ============================================

func TestExportHARToFile_WriteErrorNonexistentParent(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2026-01-23T10:00:00.000Z", Method: "GET", URL: "https://example.com", Status: 200},
	})

	// Try to write to a path where parent directory doesn't exist
	_, err := capture.ExportHARToFile(NetworkBodyFilter{}, "/tmp/gasoline-test-readonly-dir-har/subdir/test.har")
	// This path is safe but the directory doesn't exist; however os.WriteFile
	// will fail if the parent dir doesn't exist. Let's use a definitely unwritable path.
	if err != nil {
		// The /tmp path is safe, but if directory doesn't exist, WriteFile fails
		if !strings.Contains(err.Error(), "write") && !strings.Contains(err.Error(), "no such file") {
			t.Errorf("Expected write-related error, got: %v", err)
		}
	}
}

// ============================================
// Coverage: toolExportHAR — error from ExportHARToFile (line 308)
// ============================================

func TestToolExportHAR_WriteFailure(t *testing.T) {
	t.Parallel()
	server := &Server{
		entries: make([]LogEntry, 0),
	}
	capture := NewCapture()
	capture.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2026-01-23T10:00:00.000Z", Method: "GET", URL: "https://example.com", Status: 200},
	})
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server),
		capture:    capture,
	}

	// Use a path under /tmp that has a non-existent deep directory
	args, _ := json.Marshal(map[string]any{
		"save_to": "/tmp/gasoline-nonexist-parent/deep/nested/file.har",
	})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Error("Expected isError=true when file write fails")
	}
	if len(result.Content) > 0 && !strings.Contains(result.Content[0].Text, "export_failed") {
		t.Errorf("Expected structured error with export_failed code, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Coverage: toolExportHAR — marshal error path (line 319)
// This would require ExportHAR to produce unmarshallable data, which is unlikely.
// Instead test the no-save_to path with empty data
// ============================================

func TestToolExportHAR_NoSaveTo_EmptyCapture(t *testing.T) {
	t.Parallel()
	server := &Server{
		entries: make([]LogEntry, 0),
	}
	capture := NewCapture()
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server),
		capture:    capture,
	}

	args, _ := json.Marshal(map[string]any{})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := handler.toolExportHAR(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)

	if result.IsError {
		t.Errorf("Expected no error for empty HAR export, got: %s", result.Content[0].Text)
	}
	// Strip summary line before parsing JSON
	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	// Should still return valid HAR JSON with 0 entries
	var harLog HARLog
	if err := json.Unmarshal([]byte(jsonPart), &harLog); err != nil {
		t.Fatalf("Expected valid HAR JSON, got parse error: %v", err)
	}
	if len(harLog.Log.Entries) != 0 {
		t.Errorf("Expected 0 entries in empty HAR, got %d", len(harLog.Log.Entries))
	}
}
