package main

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
		data, err := os.ReadFile(tmpFile)
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

		args, _ := json.Marshal(map[string]interface{}{})
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

		// Parse the HAR from the text content
		var harLog HARLog
		if err := json.Unmarshal([]byte(result.Content[0].Text), &harLog); err != nil {
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

		// Parse the summary
		var summary HARExportResult
		if err := json.Unmarshal([]byte(result.Content[0].Text), &summary); err != nil {
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

		args, _ := json.Marshal(map[string]interface{}{
			"method":     "POST",
			"status_min": 400,
		})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}
		resp := handler.toolExportHAR(req, args)

		var result MCPToolResult
		json.Unmarshal(resp.Result, &result)

		var harLog HARLog
		json.Unmarshal([]byte(result.Content[0].Text), &harLog)

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
			if tool.Name == "export_har" {
				found = true
				if tool.Description == "" {
					t.Error("expected non-empty description")
				}
				props, ok := tool.InputSchema["properties"].(map[string]interface{})
				if !ok {
					t.Fatal("expected properties in input schema")
				}
				// Check required properties exist
				for _, prop := range []string{"url", "method", "status_min", "status_max", "save_to"} {
					if _, exists := props[prop]; !exists {
						t.Errorf("expected property %s in schema", prop)
					}
				}
				break
			}
		}
		if !found {
			t.Error("export_har tool not found in tools list")
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

		args, _ := json.Marshal(map[string]interface{}{})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`4`), Method: "tools/call"}
		resp, handled := handler.handleToolCall(req, "export_har", args)

		if !handled {
			t.Error("expected export_har to be handled")
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

		args, _ := json.Marshal(map[string]interface{}{
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
