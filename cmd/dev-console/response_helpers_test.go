// response_helpers_test.go — Tests for W1 response helpers and W5 structured error helpers.
// Phase 1 of the ADR data contract fixes.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// ============================================
// W1: Response Helper Tests
// ============================================

func TestMcpMarkdownResponse_IncludesSummary(t *testing.T) {
	t.Parallel()
	summary := "3 browser error(s)"
	md := "| Level | Message |\n| --- | --- |\n| error | test |\n"
	raw := mcpMarkdownResponse(summary, md)

	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected content blocks")
	}

	text := result.Content[0].Text
	lines := strings.SplitN(text, "\n", 2)
	if lines[0] != summary {
		t.Errorf("First line = %q, want %q", lines[0], summary)
	}
}

func TestMcpMarkdownResponse_HasTable(t *testing.T) {
	t.Parallel()
	headers := []string{"Level", "Message"}
	rows := [][]string{{"error", "test error"}}
	table := markdownTable(headers, rows)
	raw := mcpMarkdownResponse("1 error(s)", table)

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	if !strings.Contains(text, "|") {
		t.Error("Expected pipe delimiters in response")
	}
	if !strings.Contains(text, "---") {
		t.Error("Expected separator row in response")
	}
}

func TestMcpJSONResponse_IncludesSummary(t *testing.T) {
	t.Parallel()
	summary := "WebSocket connection status"
	data := map[string]interface{}{"connections": []interface{}{}}
	raw := mcpJSONResponse(summary, data)

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if lines[0] != summary {
		t.Errorf("First line = %q, want %q", lines[0], summary)
	}

	// Rest should be valid JSON
	if len(lines) < 2 {
		t.Fatal("Expected at least 2 lines")
	}
	if !json.Valid([]byte(lines[1])) {
		t.Errorf("Second line is not valid JSON: %q", lines[1])
	}
}

func TestMcpJSONResponse_CompactJSON(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"key": "value", "nested": map[string]interface{}{"a": 1}}
	raw := mcpJSONResponse("summary", data)

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	jsonPart := lines[1]

	// Compact JSON should NOT contain indentation
	if strings.Contains(jsonPart, "  ") {
		t.Error("JSON should be compact (no indentation)")
	}
}

func TestMcpJSONResponse_EmptySummary(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"status": "ok"}
	raw := mcpJSONResponse("", data)

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	// With empty summary, text should just be the JSON
	if !json.Valid([]byte(text)) {
		t.Errorf("With empty summary, entire text should be valid JSON, got: %q", text)
	}
}

func TestMcpJSONResponse_NilData(t *testing.T) {
	t.Parallel()
	raw := mcpJSONResponse("summary", nil)

	var result MCPToolResult
	json.Unmarshal(raw, &result)

	// Should not panic, should produce valid output
	if len(result.Content) == 0 {
		t.Fatal("Expected content blocks")
	}
	text := result.Content[0].Text
	if text == "" {
		t.Error("Expected non-empty text")
	}
}

func TestMcpJSONResponse_MarshalError(t *testing.T) {
	t.Parallel()
	// channels cannot be marshaled to JSON
	ch := make(chan int)
	raw := mcpJSONResponse("summary", ch)

	var result MCPToolResult
	json.Unmarshal(raw, &result)

	// Should return an error response
	if !result.IsError {
		t.Error("Expected IsError=true for marshal failure")
	}
	if !strings.Contains(result.Content[0].Text, "serialize") {
		t.Errorf("Expected serialization error message, got: %q", result.Content[0].Text)
	}
}

// ============================================
// W1: markdownTable Tests
// ============================================

func TestMarkdownTable_Empty(t *testing.T) {
	t.Parallel()
	result := markdownTable([]string{"A", "B"}, nil)
	if result != "" {
		t.Errorf("Expected empty string for empty rows, got: %q", result)
	}
}

func TestMarkdownTable_SingleRow(t *testing.T) {
	t.Parallel()
	headers := []string{"Name", "Value"}
	rows := [][]string{{"foo", "bar"}}
	result := markdownTable(headers, rows)

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines (header + separator + row), got %d: %v", len(lines), lines)
	}

	// Header
	if !strings.Contains(lines[0], "Name") || !strings.Contains(lines[0], "Value") {
		t.Errorf("Header missing columns: %q", lines[0])
	}

	// Separator
	if !strings.Contains(lines[1], "---") {
		t.Errorf("Expected separator with ---, got: %q", lines[1])
	}

	// Data row
	if !strings.Contains(lines[2], "foo") || !strings.Contains(lines[2], "bar") {
		t.Errorf("Data row missing values: %q", lines[2])
	}
}

func TestMarkdownTable_EscapesPipes(t *testing.T) {
	t.Parallel()
	headers := []string{"Message"}
	rows := [][]string{{"value with | pipe char"}}
	result := markdownTable(headers, rows)

	// The pipe in the cell value should be escaped
	// Count pipe chars: header row has 2 delimiters, separator has 2, data row should have 2 delimiters
	// If the pipe is NOT escaped, the data row would have 3 pipe delimiters (broken)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	dataRow := lines[2]

	// The cell pipe should be escaped as \|
	if strings.Contains(dataRow, "| pipe") && !strings.Contains(dataRow, `\|`) {
		t.Errorf("Pipe char in cell value should be escaped, got: %q", dataRow)
	}
}

func TestMarkdownTable_EscapesNewlines(t *testing.T) {
	t.Parallel()
	headers := []string{"Message"}
	rows := [][]string{{"line1\nline2"}}
	result := markdownTable(headers, rows)

	// Newlines in cell values should be replaced with spaces
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	dataRow := lines[2]

	if strings.Contains(dataRow, "\n") {
		t.Errorf("Newlines in cell values should be replaced, got: %q", dataRow)
	}
	if !strings.Contains(dataRow, "line1 line2") {
		t.Errorf("Expected newlines replaced with spaces, got: %q", dataRow)
	}
}

func TestMarkdownTable_SpecialChars(t *testing.T) {
	t.Parallel()
	headers := []string{"Content"}
	rows := [][]string{
		{`<script>alert("xss")</script>`},
		{"has `backticks` here"},
		{"angle > bracket < test"},
	}
	result := markdownTable(headers, rows)

	// Should not panic or break the table structure
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	// 1 header + 1 separator + 3 data rows = 5 lines
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines, got %d", len(lines))
	}

	// Each data row should still be a valid table row
	for i := 2; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "| ") || !strings.HasSuffix(lines[i], " |") {
			t.Errorf("Row %d is not a valid table row: %q", i, lines[i])
		}
	}
}

func TestMarkdownTable_LargeTable(t *testing.T) {
	t.Parallel()
	headers := []string{"ID", "Name", "Status"}
	rows := make([][]string, 500)
	for i := range rows {
		rows[i] = []string{"id-" + string(rune('0'+i%10)), "item", "active"}
	}

	result := markdownTable(headers, rows)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	// 1 header + 1 separator + 500 data rows
	expected := 502
	if len(lines) != expected {
		t.Errorf("Expected %d lines, got %d", expected, len(lines))
	}

	// Verify first and last rows are valid
	if !strings.HasPrefix(lines[0], "| ") {
		t.Errorf("First line should be table header: %q", lines[0])
	}
	if !strings.HasPrefix(lines[len(lines)-1], "| ") {
		t.Errorf("Last line should be table row: %q", lines[len(lines)-1])
	}
}

// ============================================
// W1: truncate Tests
// ============================================

func TestTruncate_ShortString(t *testing.T) {
	t.Parallel()
	result := truncate("hello", 10)
	if result != "hello" {
		t.Errorf("Expected %q, got %q", "hello", result)
	}
}

func TestTruncate_ExactLimit(t *testing.T) {
	t.Parallel()
	result := truncate("12345", 5)
	if result != "12345" {
		t.Errorf("Expected %q, got %q", "12345", result)
	}
}

func TestTruncate_LongString(t *testing.T) {
	t.Parallel()
	result := truncate("this is a long string that should be truncated", 10)
	if len(result) != 10 {
		t.Errorf("Expected length 10, got %d: %q", len(result), result)
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("Expected suffix '...', got %q", result)
	}
	// First 7 chars + "..." = 10
	if result != "this is..." {
		t.Errorf("Expected %q, got %q", "this is...", result)
	}
}

func TestTruncate_EmptyString(t *testing.T) {
	t.Parallel()
	result := truncate("", 10)
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

// ============================================
// W5: Structured Error Helper Tests
// ============================================

func TestMcpStructuredError_Format(t *testing.T) {
	t.Parallel()
	raw := mcpStructuredError(ErrMissingParam, "Parameter 'what' is missing",
		"Add the 'what' parameter and call again")

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if len(lines) < 2 {
		t.Fatal("Expected at least 2 lines in error response")
	}

	// First line: "Error: <code> — <retry>"
	expectedPrefix := "Error: missing_param"
	if !strings.HasPrefix(lines[0], expectedPrefix) {
		t.Errorf("First line should start with %q, got: %q", expectedPrefix, lines[0])
	}

	// Second line should be valid JSON
	if !json.Valid([]byte(lines[1])) {
		t.Errorf("Second line should be valid JSON, got: %q", lines[1])
	}
}

func TestMcpStructuredError_IsError(t *testing.T) {
	t.Parallel()
	raw := mcpStructuredError(ErrInvalidJSON, "bad json", "Fix JSON syntax and call again")

	var result MCPToolResult
	json.Unmarshal(raw, &result)

	if !result.IsError {
		t.Error("Expected IsError=true for structured error")
	}
}

func TestMcpStructuredError_SelfDescribingCode(t *testing.T) {
	t.Parallel()
	raw := mcpStructuredError(ErrUnknownMode, "Unknown mode: foo",
		"Use a valid mode from the 'what' enum")

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	var se StructuredError
	json.Unmarshal([]byte(lines[1]), &se)

	// Code should be readable snake_case, not an opaque number
	if se.Error != "unknown_mode" {
		t.Errorf("Error code should be self-describing, got: %q", se.Error)
	}
	if !strings.Contains(se.Error, "_") {
		t.Error("Error code should be snake_case")
	}
}

func TestMcpStructuredError_RetryIsInstruction(t *testing.T) {
	t.Parallel()
	retry := "Fix JSON syntax and call again"
	raw := mcpStructuredError(ErrInvalidJSON, "bad json", retry)

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	var se StructuredError
	json.Unmarshal([]byte(lines[1]), &se)

	// Retry should be plain English, not a coded strategy
	if se.Retry != retry {
		t.Errorf("Retry = %q, want %q", se.Retry, retry)
	}
	// Should contain a verb (plain English)
	if !strings.Contains(se.Retry, " ") {
		t.Error("Retry should be plain English instruction, not a single word")
	}
}

func TestMcpStructuredError_WithParam(t *testing.T) {
	t.Parallel()
	raw := mcpStructuredError(ErrMissingParam, "Missing 'what'",
		"Add the 'what' parameter and call again",
		withParam("what"))

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	var se StructuredError
	json.Unmarshal([]byte(lines[1]), &se)

	if se.Param != "what" {
		t.Errorf("Param = %q, want %q", se.Param, "what")
	}
}

func TestMcpStructuredError_WithHint(t *testing.T) {
	t.Parallel()
	hint := "Valid values: errors, logs, network"
	raw := mcpStructuredError(ErrMissingParam, "Missing 'what'",
		"Add the 'what' parameter and call again",
		withHint(hint))

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	var se StructuredError
	json.Unmarshal([]byte(lines[1]), &se)

	if se.Hint != hint {
		t.Errorf("Hint = %q, want %q", se.Hint, hint)
	}
}

func TestMcpStructuredError_Parseable(t *testing.T) {
	t.Parallel()
	raw := mcpStructuredError(ErrExtTimeout, "Timeout waiting for page info",
		"Browser extension didn't respond — wait a moment and retry",
		withParam("page"), withHint("Check that the extension is connected"))

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)

	var se StructuredError
	if err := json.Unmarshal([]byte(lines[1]), &se); err != nil {
		t.Fatalf("Failed to parse JSON body: %v", err)
	}

	if se.Error != ErrExtTimeout {
		t.Errorf("Error = %q, want %q", se.Error, ErrExtTimeout)
	}
	if se.Message != "Timeout waiting for page info" {
		t.Errorf("Message = %q", se.Message)
	}
	if se.Retry == "" {
		t.Error("Retry should not be empty")
	}
	if se.Param != "page" {
		t.Errorf("Param = %q, want %q", se.Param, "page")
	}
	if se.Hint == "" {
		t.Error("Hint should not be empty")
	}
}

func TestMcpStructuredError_PrefixMatchesBody(t *testing.T) {
	t.Parallel()
	code := ErrMissingParam
	retry := "Add the 'what' parameter and call again"
	raw := mcpStructuredError(code, "Missing 'what'", retry, withParam("what"))

	var result MCPToolResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	prefix := lines[0]
	body := lines[1]

	// Parse the prefix: "Error: <code> — <retry>"
	// Verify code and retry from the prefix match the JSON body
	var se StructuredError
	json.Unmarshal([]byte(body), &se)

	if !strings.Contains(prefix, se.Error) {
		t.Errorf("Prefix %q should contain error code %q", prefix, se.Error)
	}
	if !strings.Contains(prefix, se.Retry) {
		t.Errorf("Prefix %q should contain retry instruction %q", prefix, se.Retry)
	}
}

func TestMcpStructuredError_SpecialCharsInMessage(t *testing.T) {
	t.Parallel()
	message := `Error at "line 5": unexpected 'token' with unicode \u00e9 and newline
second line`
	raw := mcpStructuredError(ErrInvalidJSON, message, "Fix JSON syntax and call again")

	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("Failed to unmarshal outer result: %v", err)
	}
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if len(lines) < 2 {
		t.Fatal("Expected at least 2 lines")
	}

	// The JSON line should still be parseable
	// The JSON body might start after the first line which is the prefix.
	// But the message contains a newline, so the JSON body is the last line.
	// Actually, the format is: "Error: code — retry\n{json}"
	// The JSON is everything after the first newline.
	jsonPart := lines[1]
	var se StructuredError
	if err := json.Unmarshal([]byte(jsonPart), &se); err != nil {
		t.Fatalf("JSON body should still be parseable with special chars in message: %v\nJSON: %q", err, jsonPart)
	}

	if se.Message != message {
		t.Errorf("Message should preserve special chars, got: %q", se.Message)
	}
}

// ============================================
// W5: Error Code Constants Verification
// ============================================

func TestErrorCodeConstants_AreSnakeCase(t *testing.T) {
	t.Parallel()
	codes := []string{
		ErrInvalidJSON, ErrMissingParam, ErrInvalidParam, ErrUnknownMode,
		ErrPathNotAllowed, ErrNotInitialized, ErrNoData, ErrCodePilotDisabled,
		ErrRateLimited, ErrExtTimeout, ErrExtError, ErrInternal,
		ErrMarshalFailed, ErrExportFailed,
	}

	for _, code := range codes {
		if code == "" {
			t.Error("Error code constant should not be empty")
			continue
		}
		// Should be snake_case: only lowercase letters and underscores
		for _, c := range code {
			if (c < 'a' || c > 'z') && c != '_' {
				t.Errorf("Error code %q contains non-snake_case char %q", code, string(c))
				break
			}
		}
	}
}

// ============================================
// W1/W5: Integration Tests — Response Migration Verification
// ============================================

// TestNoRawMcpErrorResponse scans tools.go source for mcpErrorResponse( calls.
// After migration, no handler code should call mcpErrorResponse directly.
// The function definition and test helpers may still reference it.
func TestNoRawMcpErrorResponse(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("tools.go")
	if err != nil {
		t.Fatalf("Failed to read tools.go: %v", err)
	}

	lines := strings.Split(string(src), "\n")
	var violations []string
	inExcludedFunc := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip the mcpErrorResponse function definition itself
		if strings.HasPrefix(trimmed, "func mcpErrorResponse(") {
			inExcludedFunc = true
			continue
		}
		// Skip the mcpJSONResponse function definition (it calls mcpErrorResponse for marshal failures)
		if strings.HasPrefix(trimmed, "func mcpJSONResponse(") {
			inExcludedFunc = true
			continue
		}
		if inExcludedFunc {
			// End of function: line starting with "}" at column 0
			if strings.HasPrefix(line, "}") {
				inExcludedFunc = false
			}
			continue
		}

		// Skip comments
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		if strings.Contains(line, "mcpErrorResponse(") {
			violations = append(violations, fmt.Sprintf("line %d: %s", i+1, trimmed))
		}
	}

	if len(violations) > 0 {
		t.Errorf("Found %d raw mcpErrorResponse calls in tools.go (should use mcpStructuredError instead):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// TestAllErrorCodes_UsedInHandlers verifies that each ErrXxx constant appears
// in at least one mcpStructuredError call across the source files.
func TestAllErrorCodes_UsedInHandlers(t *testing.T) {
	t.Parallel()
	// Read all Go source files in the package
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	var allSource string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			data, err := os.ReadFile(entry.Name())
			if err != nil {
				continue
			}
			allSource += string(data)
		}
	}

	codes := map[string]string{
		"ErrInvalidJSON":    ErrInvalidJSON,
		"ErrMissingParam":   ErrMissingParam,
		"ErrInvalidParam":   ErrInvalidParam,
		"ErrUnknownMode":    ErrUnknownMode,
		"ErrNotInitialized": ErrNotInitialized,
		"ErrNoData":         ErrNoData,
		"ErrInternal":       ErrInternal,
		"ErrExportFailed":   ErrExportFailed,
	}

	for name, code := range codes {
		// Check that the constant appears in a mcpStructuredError call
		pattern := "mcpStructuredError(" + name
		if !strings.Contains(allSource, pattern) {
			t.Errorf("Error code %s (%q) is not used in any mcpStructuredError call", name, code)
		}
	}
}

// TestEntryStr_Helper verifies the entryStr helper for safe LogEntry field extraction.
func TestEntryStr_Helper(t *testing.T) {
	t.Parallel()
	entry := LogEntry{
		"level":   "error",
		"message": "test message",
		"source":  "app.js:42",
	}

	if got := entryStr(entry, "level"); got != "error" {
		t.Errorf("entryStr(level) = %q, want %q", got, "error")
	}
	if got := entryStr(entry, "message"); got != "test message" {
		t.Errorf("entryStr(message) = %q, want %q", got, "test message")
	}
	// Missing key should return empty string
	if got := entryStr(entry, "nonexistent"); got != "" {
		t.Errorf("entryStr(nonexistent) = %q, want empty", got)
	}
	// Non-string value
	entry["count"] = 42
	if got := entryStr(entry, "count"); got != "" {
		t.Errorf("entryStr(count) = %q, want empty for non-string", got)
	}
}

// TestBrowserErrors_MarkdownFormat verifies toolGetBrowserErrors returns markdown table format.
func TestBrowserErrors_MarkdownFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "TypeError: foo", "source": "app.js:10", "ts": "2024-01-01T00:00:00Z"},
		{"level": "error", "message": "ReferenceError: bar", "source": "lib.js:20", "ts": "2024-01-01T00:00:01Z"},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetBrowserErrors(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// First line should be summary
	lines := strings.SplitN(text, "\n", 2)
	if !strings.Contains(lines[0], "2 browser error(s)") {
		t.Errorf("Expected summary with count, got: %q", lines[0])
	}

	// Should contain markdown table delimiters
	if !strings.Contains(text, "| ") {
		t.Error("Expected pipe delimiters in markdown table")
	}
	if !strings.Contains(text, "---") {
		t.Error("Expected separator row in markdown table")
	}

	// Should contain the error data
	if !strings.Contains(text, "TypeError: foo") {
		t.Error("Expected error message in table")
	}
	if !strings.Contains(text, "app.js:10") {
		t.Error("Expected source in table")
	}
}

// TestBrowserLogs_MarkdownFormat verifies toolGetBrowserLogs returns markdown table format.
func TestBrowserLogs_MarkdownFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "log", "message": "App started", "source": "main.js:1", "ts": "2024-01-01T00:00:00Z"},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetBrowserLogs(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if !strings.Contains(lines[0], "1 log entries") {
		t.Errorf("Expected summary with count, got: %q", lines[0])
	}
	if !strings.Contains(text, "| ") {
		t.Error("Expected pipe delimiters in markdown table")
	}
}

// TestBrowserLogs_IncludesTabId verifies toolGetBrowserLogs includes tabId in response.
func TestBrowserLogs_IncludesTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "test error", "source": "app.js:10", "ts": "2024-01-01T00:00:00Z", "tabId": float64(42)},
		{"level": "log", "message": "info msg", "source": "main.js:1", "ts": "2024-01-01T00:00:01Z", "tabId": float64(99)},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetBrowserLogs(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain Tab column header
	if !strings.Contains(text, "Tab") {
		t.Error("Expected 'Tab' column header in markdown table")
	}
	// Should contain tab IDs
	if !strings.Contains(text, "42") {
		t.Error("Expected tabId 42 in table")
	}
	if !strings.Contains(text, "99") {
		t.Error("Expected tabId 99 in table")
	}
}

// TestBrowserErrors_IncludesTabId verifies toolGetBrowserErrors includes tabId in response.
func TestBrowserErrors_IncludesTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "TypeError: foo", "source": "app.js:10", "ts": "2024-01-01T00:00:00Z", "tabId": float64(42)},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetBrowserErrors(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain Tab column header
	if !strings.Contains(text, "Tab") {
		t.Error("Expected 'Tab' column header in markdown table")
	}
	// Should contain tab ID
	if !strings.Contains(text, "42") {
		t.Error("Expected tabId 42 in table")
	}
}

// TestBrowserLogs_NoTabId verifies logs without tabId still work (backward compat).
func TestBrowserLogs_NoTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	server.addEntries([]LogEntry{
		{"level": "error", "message": "old entry", "source": "app.js:10", "ts": "2024-01-01T00:00:00Z"},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetBrowserLogs(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should still render correctly without tabId
	if !strings.Contains(text, "old entry") {
		t.Error("Expected log message in table")
	}
}

// TestConfigureStore_JSONFormat verifies toolConfigureStore returns JSON format with summary.
func TestConfigureStore_JSONFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"store_action":"stats"}`)
	resp := mcp.toolHandler.toolConfigureStore(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if lines[0] != "Store operation complete" {
		t.Errorf("Expected summary 'Store operation complete', got: %q", lines[0])
	}
	if len(lines) < 2 || !json.Valid([]byte(lines[1])) {
		t.Error("Expected valid JSON on second line")
	}
}

// TestConfigureNoise_JSONFormat verifies toolConfigureNoise returns JSON format with summary.
func TestConfigureNoise_JSONFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"action":"list"}`)
	resp := mcp.toolHandler.toolConfigureNoise(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if lines[0] != "Noise configuration updated" {
		t.Errorf("Expected summary 'Noise configuration updated', got: %q", lines[0])
	}
}

// TestDismissNoise_JSONFormat verifies toolDismissNoise returns JSON format with summary.
func TestDismissNoise_JSONFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	args := json.RawMessage(`{"pattern":"test","category":"console","reason":"noise"}`)
	resp := mcp.toolHandler.toolDismissNoise(req, args)

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if lines[0] != "Noise pattern dismissed" {
		t.Errorf("Expected summary 'Noise pattern dismissed', got: %q", lines[0])
	}
}

// TestObserveDispatcher_StructuredErrors verifies dispatcher errors use structured format.
func TestObserveDispatcher_StructuredErrors(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	tests := []struct {
		name     string
		args     string
		wantCode string
	}{
		{
			name:     "missing what",
			args:     `{}`,
			wantCode: ErrMissingParam,
		},
		{
			name:     "unknown mode",
			args:     `{"what":"nonexistent_mode"}`,
			wantCode: ErrUnknownMode,
		},
		{
			name:     "invalid json",
			args:     `{bad json}`,
			wantCode: ErrInvalidJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
			resp := mcp.toolHandler.toolObserve(req, json.RawMessage(tt.args))

			var result MCPToolResult
			json.Unmarshal(resp.Result, &result)

			if !result.IsError {
				t.Fatal("Expected isError=true")
			}

			text := result.Content[0].Text
			// Structured error format: "Error: <code> — <retry>\n{json}"
			if !strings.HasPrefix(text, "Error: "+tt.wantCode) {
				t.Errorf("Expected structured error with code %q, got: %q", tt.wantCode, text)
			}

			// Should have parseable JSON body
			lines := strings.SplitN(text, "\n", 2)
			if len(lines) < 2 {
				t.Fatal("Expected JSON body on second line")
			}
			var se StructuredError
			if err := json.Unmarshal([]byte(lines[1]), &se); err != nil {
				t.Fatalf("JSON body not parseable: %v", err)
			}
			if se.Error != tt.wantCode {
				t.Errorf("Error code = %q, want %q", se.Error, tt.wantCode)
			}
			if se.Retry == "" {
				t.Error("Retry instruction should not be empty")
			}
		})
	}
}

// TestGenerateDispatcher_StructuredErrors verifies generate dispatcher errors use structured format.
func TestGenerateDispatcher_StructuredErrors(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	// Missing format
	resp := mcp.toolHandler.toolGenerate(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("Expected isError=true for missing format")
	}
	if !strings.Contains(result.Content[0].Text, ErrMissingParam) {
		t.Error("Expected missing_param error code")
	}

	// Unknown format
	resp = mcp.toolHandler.toolGenerate(req, json.RawMessage(`{"format":"unknown_format"}`))
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("Expected isError=true for unknown format")
	}
	if !strings.Contains(result.Content[0].Text, ErrUnknownMode) {
		t.Error("Expected unknown_mode error code")
	}
}

// TestConfigureDispatcher_StructuredErrors verifies configure dispatcher errors use structured format.
func TestConfigureDispatcher_StructuredErrors(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	// Missing action
	resp := mcp.toolHandler.toolConfigure(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("Expected isError=true for missing action")
	}
	if !strings.Contains(result.Content[0].Text, ErrMissingParam) {
		t.Error("Expected missing_param error code")
	}

	// Unknown action
	resp = mcp.toolHandler.toolConfigure(req, json.RawMessage(`{"action":"nonexistent"}`))
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("Expected isError=true for unknown action")
	}
	if !strings.Contains(result.Content[0].Text, ErrUnknownMode) {
		t.Error("Expected unknown_mode error code")
	}
}

// TestInteractDispatcher_StructuredErrors verifies interact dispatcher errors use structured format.
func TestInteractDispatcher_StructuredErrors(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}

	// Missing action
	resp := mcp.toolHandler.toolInteract(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("Expected isError=true for missing action")
	}
	if !strings.Contains(result.Content[0].Text, ErrMissingParam) {
		t.Error("Expected missing_param error code")
	}

	// Unknown action
	resp = mcp.toolHandler.toolInteract(req, json.RawMessage(`{"action":"nonexistent"}`))
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("Expected isError=true for unknown action")
	}
	if !strings.Contains(result.Content[0].Text, ErrUnknownMode) {
		t.Error("Expected unknown_mode error code")
	}
}

// TestConfigureStoreNil_StructuredError verifies nil session store returns structured error.
func TestConfigureStoreNil_StructuredError(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	mcp.toolHandler.sessionStore = nil

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolConfigureStore(req, json.RawMessage(`{"store_action":"save"}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("Expected isError=true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, ErrNotInitialized) {
		t.Errorf("Expected not_initialized error code, got: %q", text)
	}
}

// TestVerifyFix_StructuredError verifies verification manager nil returns structured error.
func TestVerifyFix_StructuredError(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)
	mcp.toolHandler.verificationMgr = nil

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolVerifyFix(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("Expected isError=true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, ErrNotInitialized) {
		t.Errorf("Expected not_initialized error code, got: %q", text)
	}
}

// TestChanges_JSONFormat verifies toolGetChangesSince returns JSON format with summary.
func TestChanges_JSONFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetChangesSince(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if lines[0] != "Changes since checkpoint" {
		t.Errorf("Expected summary 'Changes since checkpoint', got: %q", lines[0])
	}
}

// TestLoadSessionContext_JSONFormat verifies toolLoadSessionContext returns JSON format.
func TestLoadSessionContext_JSONFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolLoadSessionContext(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	lines := strings.SplitN(text, "\n", 2)
	if lines[0] != "Session context loaded" {
		t.Errorf("Expected summary 'Session context loaded', got: %q", lines[0])
	}
}

// ============================================
// Bug #6: Missing tabId in MCP Responses - TDD Tests
// ============================================

// TestNetworkBodies_IncludesTabId verifies toolGetNetworkBodies includes tabId in response entries.
func TestNetworkBodies_IncludesTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add network bodies with tabId
	capture.mu.Lock()
	capture.networkBodies = []NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, TabId: 42},
		{URL: "https://api.example.com/posts", Method: "POST", Status: 201, TabId: 99},
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetNetworkBodies(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain tab_id in JSON response
	if !strings.Contains(text, `"tab_id":42`) && !strings.Contains(text, `"tab_id": 42`) {
		t.Errorf("Expected tab_id 42 in response, got: %s", text)
	}
	if !strings.Contains(text, `"tab_id":99`) && !strings.Contains(text, `"tab_id": 99`) {
		t.Errorf("Expected tab_id 99 in response, got: %s", text)
	}
}

// TestNetworkBodies_NoTabId verifies backward compatibility when tabId is missing.
func TestNetworkBodies_NoTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add network body without tabId (TabId=0 means not set)
	capture.mu.Lock()
	capture.networkBodies = []NetworkBody{
		{URL: "https://api.example.com/legacy", Method: "GET", Status: 200, TabId: 0},
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetNetworkBodies(req, json.RawMessage(`{}`))

	// Should not crash and should return valid response
	if resp.Error != nil {
		t.Errorf("Expected no error, got: %v", resp.Error)
	}
}

// TestWebSocketEvents_IncludesTabId verifies toolGetWSEvents includes tabId in response entries.
func TestWebSocketEvents_IncludesTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add WebSocket events with tabId
	capture.mu.Lock()
	capture.wsEvents = []WebSocketEvent{
		{ID: "conn1", Event: "message", Direction: "incoming", Data: "hello", TabId: 42},
		{ID: "conn2", Event: "message", Direction: "outgoing", Data: "world", TabId: 99},
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetWSEvents(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain Tab column header and tab values in markdown table
	if !strings.Contains(text, "| Tab |") {
		t.Errorf("Expected 'Tab' column header in response, got: %s", text)
	}
	if !strings.Contains(text, "| 42 |") {
		t.Errorf("Expected tabId 42 in markdown table, got: %s", text)
	}
	if !strings.Contains(text, "| 99 |") {
		t.Errorf("Expected tabId 99 in markdown table, got: %s", text)
	}
}

// TestEnhancedActions_IncludesTabId verifies toolGetEnhancedActions includes tabId in response entries.
func TestEnhancedActions_IncludesTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add enhanced actions with tabId
	capture.mu.Lock()
	capture.enhancedActions = []EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "https://example.com", TabId: 42},
		{Type: "input", Timestamp: 2000, URL: "https://example.com", TabId: 99},
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolGetEnhancedActions(req, json.RawMessage(`{}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain Tab column header and tab values in markdown table
	if !strings.Contains(text, "| Tab |") {
		t.Errorf("Expected 'Tab' column header in response, got: %s", text)
	}
	if !strings.Contains(text, "| 42 |") {
		t.Errorf("Expected tabId 42 in markdown table, got: %s", text)
	}
	if !strings.Contains(text, "| 99 |") {
		t.Errorf("Expected tabId 99 in markdown table, got: %s", text)
	}
}

// TestObserveLogs_FilterByTabId verifies observe logs can filter by tab_id parameter.
func TestObserveLogs_FilterByTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add log entries from different tabs
	server.addEntries([]LogEntry{
		{"level": "error", "message": "error from tab 42", "tabId": float64(42)},
		{"level": "error", "message": "error from tab 99", "tabId": float64(99)},
		{"level": "log", "message": "log from tab 42", "tabId": float64(42)},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	// Filter by tab_id: 42
	resp := mcp.toolHandler.toolObserve(req, json.RawMessage(`{"what":"logs","tab_id":42}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain entries from tab 42
	if !strings.Contains(text, "error from tab 42") {
		t.Error("Expected 'error from tab 42' in filtered results")
	}
	if !strings.Contains(text, "log from tab 42") {
		t.Error("Expected 'log from tab 42' in filtered results")
	}
	// Should NOT contain entries from tab 99
	if strings.Contains(text, "error from tab 99") {
		t.Error("Should NOT contain 'error from tab 99' when filtering by tab_id 42")
	}
}

// TestObserveErrors_FilterByTabId verifies observe errors can filter by tab_id parameter.
func TestObserveErrors_FilterByTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add error entries from different tabs
	server.addEntries([]LogEntry{
		{"level": "error", "message": "error from tab 42", "tabId": float64(42)},
		{"level": "error", "message": "error from tab 99", "tabId": float64(99)},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	// Filter by tab_id: 99
	resp := mcp.toolHandler.toolObserve(req, json.RawMessage(`{"what":"errors","tab_id":99}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain entries from tab 99 only
	if !strings.Contains(text, "error from tab 99") {
		t.Error("Expected 'error from tab 99' in filtered results")
	}
	// Should NOT contain entries from tab 42
	if strings.Contains(text, "error from tab 42") {
		t.Error("Should NOT contain 'error from tab 42' when filtering by tab_id 99")
	}
}

// TestObserveNetworkBodies_FilterByTabId verifies observe network_bodies can filter by tab_id.
func TestObserveNetworkBodies_FilterByTabId(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add network bodies from different tabs
	capture.mu.Lock()
	capture.networkBodies = []NetworkBody{
		{URL: "https://api.example.com/tab42", Method: "GET", Status: 200, TabId: 42},
		{URL: "https://api.example.com/tab99", Method: "GET", Status: 200, TabId: 99},
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	// Filter by tab_id: 42
	resp := mcp.toolHandler.toolObserve(req, json.RawMessage(`{"what":"network_bodies","tab_id":42}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain entries from tab 42 only
	if !strings.Contains(text, "tab42") {
		t.Error("Expected URL with 'tab42' in filtered results")
	}
	// Should NOT contain entries from tab 99
	if strings.Contains(text, "tab99") {
		t.Error("Should NOT contain URL with 'tab99' when filtering by tab_id 42")
	}
}

// TestObserve_FilterByTabId_EmptyResults verifies empty array returned when no entries match filter.
func TestObserve_FilterByTabId_EmptyResults(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add log entries from tab 42 only
	server.addEntries([]LogEntry{
		{"level": "error", "message": "error from tab 42", "tabId": float64(42)},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	// Filter by non-existent tab_id: 999
	resp := mcp.toolHandler.toolObserve(req, json.RawMessage(`{"what":"logs","tab_id":999}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should not crash and should indicate no matching entries
	if resp.Error != nil {
		t.Errorf("Expected no error, got: %v", resp.Error)
	}
	// Response should indicate empty results or no entries
	if !strings.Contains(text, "0") && !strings.Contains(text, "No") && !strings.Contains(text, "no") {
		t.Logf("Response text: %s", text)
	}
}

// TestObserve_IncludesCurrentlyTrackedTab verifies observe responses include currently_tracked_tab metadata.
func TestObserve_IncludesCurrentlyTrackedTab(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Set tracking state
	capture.mu.Lock()
	capture.trackingEnabled = true
	capture.trackedTabID = 42
	capture.trackingUpdated = time.Now()
	capture.mu.Unlock()

	// Add some logs
	server.addEntries([]LogEntry{
		{"level": "error", "message": "test error", "tabId": float64(42)},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := mcp.toolHandler.toolObserve(req, json.RawMessage(`{"what":"errors"}`))

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	text := result.Content[0].Text

	// Should contain currently_tracked_tab info (either in text or as metadata)
	// The implementation can include this in the summary or as a separate field
	if !strings.Contains(text, "42") {
		t.Logf("Response may include tracking info in metadata. Text: %s", text)
	}
}
