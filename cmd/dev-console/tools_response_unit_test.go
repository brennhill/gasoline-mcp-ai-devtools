package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func parseToolResultRaw(t *testing.T, raw json.RawMessage) MCPToolResult {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal MCPToolResult: %v; raw=%s", err, string(raw))
	}
	return result
}

func TestSafeMarshal(t *testing.T) {
	t.Parallel()

	ok := safeMarshal(map[string]any{"k": "v"}, `{"fallback":true}`)
	var parsed map[string]any
	if err := json.Unmarshal(ok, &parsed); err != nil {
		t.Fatalf("safeMarshal valid output parse error: %v", err)
	}
	if parsed["k"] != "v" {
		t.Fatalf("safeMarshal output = %+v, want k=v", parsed)
	}

	// Unsupported type triggers fallback path.
	bad := safeMarshal(map[string]any{"ch": make(chan int)}, `{"fallback":true}`)
	if string(bad) != `{"fallback":true}` {
		t.Fatalf("safeMarshal fallback = %s, want fallback JSON", string(bad))
	}
}

func TestLenientUnmarshal(t *testing.T) {
	t.Parallel()

	var params struct {
		Name string `json:"name"`
	}
	lenientUnmarshal(json.RawMessage(`{"name":"ok"}`), &params)
	if params.Name != "ok" {
		t.Fatalf("lenientUnmarshal valid parse => Name=%q, want ok", params.Name)
	}

	params.Name = "unchanged"
	lenientUnmarshal(json.RawMessage(`{invalid}`), &params)
	if params.Name != "unchanged" {
		t.Fatalf("lenientUnmarshal invalid should leave previous values unchanged; got %q", params.Name)
	}

	lenientUnmarshal(nil, &params) // should be no-op
}

func TestMCPResponseHelpers(t *testing.T) {
	t.Parallel()

	textResult := parseToolResultRaw(t, mcpTextResponse("hello"))
	if textResult.IsError {
		t.Fatal("mcpTextResponse should not set IsError")
	}
	if len(textResult.Content) != 1 || textResult.Content[0].Text != "hello" {
		t.Fatalf("unexpected mcpTextResponse content: %+v", textResult.Content)
	}

	errResult := parseToolResultRaw(t, mcpErrorResponse("boom"))
	if !errResult.IsError {
		t.Fatal("mcpErrorResponse should set IsError")
	}
	if len(errResult.Content) != 1 || errResult.Content[0].Text != "boom" {
		t.Fatalf("unexpected mcpErrorResponse content: %+v", errResult.Content)
	}

	jsonErr := parseToolResultRaw(t, mcpJSONErrorResponse("Summary", map[string]any{"ok": false}))
	if !jsonErr.IsError {
		t.Fatal("mcpJSONErrorResponse should set IsError")
	}
	if !strings.Contains(jsonErr.Content[0].Text, "Summary\n") {
		t.Fatalf("mcpJSONErrorResponse should prefix summary line: %q", jsonErr.Content[0].Text)
	}
	if !strings.Contains(jsonErr.Content[0].Text, `"ok":false`) {
		t.Fatalf("mcpJSONErrorResponse should include JSON body: %q", jsonErr.Content[0].Text)
	}

	jsonOK := parseToolResultRaw(t, mcpJSONResponse("Summary", map[string]any{"count": 2}))
	if jsonOK.IsError {
		t.Fatal("mcpJSONResponse should not set IsError")
	}
	if !strings.Contains(jsonOK.Content[0].Text, "Summary\n") ||
		!strings.Contains(jsonOK.Content[0].Text, `"count":2`) {
		t.Fatalf("mcpJSONResponse output unexpected: %q", jsonOK.Content[0].Text)
	}

	markdown := parseToolResultRaw(t, mcpMarkdownResponse("Rows", "| a | b |"))
	if markdown.IsError {
		t.Fatal("mcpMarkdownResponse should not set IsError")
	}
	if !strings.Contains(markdown.Content[0].Text, "Rows\n\n| a | b |") {
		t.Fatalf("mcpMarkdownResponse output unexpected: %q", markdown.Content[0].Text)
	}
}

func TestMCPJSONErrorResponseMarshalFailure(t *testing.T) {
	t.Parallel()

	result := parseToolResultRaw(t, mcpJSONErrorResponse("x", map[string]any{"ch": make(chan int)}))
	if !result.IsError {
		t.Fatal("marshal failure path should return IsError response")
	}
	if !strings.Contains(result.Content[0].Text, "Failed to serialize response") {
		t.Fatalf("expected serialization error message, got %q", result.Content[0].Text)
	}
}

func TestMarkdownTable(t *testing.T) {
	t.Parallel()

	if got := markdownTable([]string{"A"}, nil); got != "" {
		t.Fatalf("markdownTable with no rows = %q, want empty string", got)
	}

	table := markdownTable(
		[]string{"col1", "col2"},
		[][]string{
			{"line1\nline2", "a|b"},
		},
	)

	if !strings.Contains(table, "| col1 | col2 |") {
		t.Fatalf("missing header row: %q", table)
	}
	if !strings.Contains(table, "line1 line2") {
		t.Fatalf("newline should be replaced with space: %q", table)
	}
	if !strings.Contains(table, `a\|b`) {
		t.Fatalf("pipe should be escaped: %q", table)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	if got := truncate("short", 10); got != "short" {
		t.Fatalf("truncate should keep short strings, got %q", got)
	}
	if got := truncate("abcdef", 6); got != "abcdef" {
		t.Fatalf("truncate exact length should be unchanged, got %q", got)
	}
	if got := truncate("abcdef", 5); got != "ab..." {
		t.Fatalf("truncate(abcdef,5) = %q, want ab...", got)
	}
	if got := truncate("abcdef", 3); got != "..." {
		t.Fatalf("truncate(abcdef,3) = %q, want ...", got)
	}
	if got := truncate("abcdef", 2); got != ".." {
		t.Fatalf("truncate(abcdef,2) = %q, want ..", got)
	}
}

func TestMCPJSONResponse_EmptySummary(t *testing.T) {
	t.Parallel()

	result := parseToolResultRaw(t, mcpJSONResponse("", map[string]any{"count": 1}))
	if result.IsError {
		t.Fatal("should not be error")
	}
	text := result.Content[0].Text
	// With empty summary, text should be just the JSON (no leading newline)
	if text[0] != '{' {
		t.Fatalf("expected JSON to start with '{', got: %q", text)
	}
	if strings.Contains(text, "\n") {
		t.Fatalf("empty summary should not produce newline prefix, got: %q", text)
	}
}

func TestMCPJSONResponse_MarshalFailure(t *testing.T) {
	t.Parallel()

	result := parseToolResultRaw(t, mcpJSONResponse("x", map[string]any{"ch": make(chan int)}))
	if !result.IsError {
		t.Fatal("marshal failure should return error")
	}
	if !strings.Contains(result.Content[0].Text, "Failed to serialize response") {
		t.Fatalf("expected serialization error, got: %q", result.Content[0].Text)
	}
}

func TestMCPJSONErrorResponse_EmptySummary(t *testing.T) {
	t.Parallel()

	result := parseToolResultRaw(t, mcpJSONErrorResponse("", map[string]any{"err": true}))
	if !result.IsError {
		t.Fatal("should be error")
	}
	text := result.Content[0].Text
	if text[0] != '{' {
		t.Fatalf("expected JSON to start with '{', got: %q", text)
	}
}

func TestAppendWarningsToResponse(t *testing.T) {
	t.Parallel()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcpTextResponse("ok"),
	}

	withWarnings := appendWarningsToResponse(resp, []string{"first", "second"})
	parsed := parseToolResultRaw(t, withWarnings.Result)
	if len(parsed.Content) != 2 {
		t.Fatalf("content blocks len = %d, want 2 after warning append", len(parsed.Content))
	}
	if parsed.Content[1].Text != "_warnings: first; second" {
		t.Fatalf("warning block text = %q, want joined warnings", parsed.Content[1].Text)
	}

	unchanged := appendWarningsToResponse(resp, nil)
	if string(unchanged.Result) != string(resp.Result) {
		t.Fatalf("appendWarningsToResponse(nil) should not modify response")
	}

	invalid := JSONRPCResponse{JSONRPC: "2.0", ID: 2, Result: json.RawMessage(`{not-json}`)}
	got := appendWarningsToResponse(invalid, []string{"x"})
	if string(got.Result) != string(invalid.Result) {
		t.Fatalf("invalid result JSON should remain unchanged")
	}
}
