// Purpose: Validate response_test.go behavior and guard against regressions.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/observe/index.md

// response_test.go — Tests for response formatting and size clamping.
package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// buildRawToolResult creates an MCPToolResult JSON without clamping.
func buildRawToolResult(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
	}
	raw, _ := json.Marshal(result)
	return json.RawMessage(raw)
}

func TestClampResponseSize_UnderLimit(t *testing.T) {
	t.Parallel()
	result := buildRawToolResult("hello world")
	clamped := ClampResponseSize(result)
	if string(clamped) != string(result) {
		t.Errorf("expected no change for small response")
	}
}

func TestClampResponseSize_OverLimit(t *testing.T) {
	t.Parallel()
	bigText := strings.Repeat("x", MaxResponseBytes+1000)
	result := buildRawToolResult(bigText)
	clamped := ClampResponseSize(result)

	if len(clamped) >= len(result) {
		t.Errorf("expected clamped to be smaller: clamped=%d, original=%d", len(clamped), len(result))
	}

	if !strings.Contains(string(clamped), "[truncated") {
		t.Error("expected truncation note in clamped response")
	}
}

func TestClampResponseSize_PreservesStructure(t *testing.T) {
	t.Parallel()
	bigText := strings.Repeat("a", MaxResponseBytes+5000)
	result := buildRawToolResult(bigText)
	clamped := ClampResponseSize(result)

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("clamped response should be valid JSON: %v", err)
	}
	if len(toolResult.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if toolResult.Content[0].Type != "text" {
		t.Errorf("expected type=text, got %s", toolResult.Content[0].Type)
	}
}

func TestClampResponseSize_JSONPayload(t *testing.T) {
	t.Parallel()
	bigJSON := `{"key":"` + strings.Repeat("z", MaxResponseBytes+1000) + `"}`
	result := buildRawToolResult(bigJSON)
	clamped := ClampResponseSize(result)

	if len(clamped) >= len(result) {
		t.Errorf("expected clamped JSON response to be smaller")
	}
}

func TestClampResponseSize_ErrorResponse(t *testing.T) {
	t.Parallel()
	bigText := strings.Repeat("e", MaxResponseBytes+1000)
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: bigText}},
		IsError: true,
	}
	raw, _ := json.Marshal(result)
	clamped := ClampResponseSize(json.RawMessage(raw))

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("clamped error response should be valid JSON: %v", err)
	}
	if len(clamped) >= len(raw) {
		t.Error("expected error response to also be clamped when oversized")
	}
}

func TestClampResponseSize_PaginationHint(t *testing.T) {
	t.Parallel()
	bigText := strings.Repeat("p", MaxResponseBytes+5000)
	result := buildRawToolResult(bigText)
	clamped := ClampResponseSize(result)

	if !strings.Contains(string(clamped), "pagination") {
		t.Error("expected pagination hint in truncation note")
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"ab", 3, "ab"},
		{"abcd", 3, "..."},
	}
	for _, tt := range tests {
		got := Truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestAppendWarningsToToolResult(t *testing.T) {
	t.Parallel()
	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "ok"}},
	}

	changed := AppendWarningsToToolResult(result, []string{"unknown parameter 'x' (ignored)"})
	if !changed {
		t.Fatal("expected warnings to be appended")
	}
	if len(result.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(result.Content))
	}
	if !strings.Contains(result.Content[1].Text, "_warnings:") {
		t.Fatalf("warning block text missing prefix: %q", result.Content[1].Text)
	}
}

func TestAppendWarningsToToolResult_NoOp(t *testing.T) {
	t.Parallel()
	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "ok"}},
	}

	if AppendWarningsToToolResult(result, nil) {
		t.Fatal("expected false for nil warnings")
	}
	if AppendWarningsToToolResult(nil, []string{"warn"}) {
		t.Fatal("expected false for nil result")
	}
}

func TestImageContentBlock(t *testing.T) {
	t.Parallel()
	block := ImageContentBlock("dGVzdA==", "image/png")
	if block.Type != "image" {
		t.Errorf("Type = %q, want 'image'", block.Type)
	}
	if block.Data != "dGVzdA==" {
		t.Errorf("Data = %q, want 'dGVzdA=='", block.Data)
	}
	if block.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want 'image/png'", block.MimeType)
	}
	if block.Text != "" {
		t.Errorf("Text should be empty for image blocks, got %q", block.Text)
	}
}

func TestAppendImageToResponse(t *testing.T) {
	t.Parallel()
	textResult := TextResponse("Screenshot captured")
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: textResult}

	resp = AppendImageToResponse(resp, "dGVzdA==", "image/jpeg")

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if len(result.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("first block type = %q, want 'text'", result.Content[0].Type)
	}
	if result.Content[1].Type != "image" {
		t.Errorf("second block type = %q, want 'image'", result.Content[1].Type)
	}
	if result.Content[1].Data != "dGVzdA==" {
		t.Errorf("image data = %q, want 'dGVzdA=='", result.Content[1].Data)
	}
	if result.Content[1].MimeType != "image/jpeg" {
		t.Errorf("image mimeType = %q, want 'image/jpeg'", result.Content[1].MimeType)
	}
}

func TestAppendImageToResponse_EmptyData(t *testing.T) {
	t.Parallel()
	textResult := TextResponse("test")
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: textResult}

	resp = AppendImageToResponse(resp, "", "image/png")

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block (no image appended for empty data), got %d", len(result.Content))
	}
}

func TestClampResponseSize_JSONBoundaryTruncation(t *testing.T) {
	t.Parallel()
	// Build a JSON payload with nested objects that exceeds MaxResponseBytes.
	// After clamping, the result should still be valid JSON (parseable).
	inner := `{"items":[` + strings.Repeat(`{"id":1,"name":"test item with some content"},`, MaxResponseBytes/50) + `{"id":999}]}`
	result := buildRawToolResult(inner)
	clamped := ClampResponseSize(result)

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("clamped JSON-heavy response should still be valid MCPToolResult JSON: %v\nraw: %s", err, string(clamped)[:200])
	}
	if len(toolResult.Content) == 0 {
		t.Fatal("expected at least one content block after clamping")
	}
}

func TestClampResponseSize_PreservesImageBlocks(t *testing.T) {
	t.Parallel()
	// Image blocks should not count toward the byte limit.
	textContent := strings.Repeat("x", MaxResponseBytes-500)
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: textContent},
			{Type: "image", Data: strings.Repeat("A", 50000), MimeType: "image/png"},
		},
	}
	raw, _ := json.Marshal(result)
	clamped := ClampResponseSize(json.RawMessage(raw))

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("response with image block should remain valid: %v", err)
	}
	// Text should NOT be truncated since it's under the limit
	hasImage := false
	for _, block := range toolResult.Content {
		if block.Type == "image" {
			hasImage = true
		}
	}
	if !hasImage {
		t.Error("image content block should be preserved")
	}
}

func TestImageContentBlock_SerializesCorrectly(t *testing.T) {
	t.Parallel()
	block := ImageContentBlock("AAAA", "image/png")
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	s := string(data)
	// Should have "type":"image", "data":"AAAA", "mimeType":"image/png"
	if !strings.Contains(s, `"type":"image"`) {
		t.Errorf("JSON should contain type:image, got: %s", s)
	}
	if !strings.Contains(s, `"data":"AAAA"`) {
		t.Errorf("JSON should contain data:AAAA, got: %s", s)
	}
	if !strings.Contains(s, `"mimeType":"image/png"`) {
		t.Errorf("JSON should contain mimeType:image/png, got: %s", s)
	}
	// Should NOT have "text" field
	if strings.Contains(s, `"text"`) {
		t.Errorf("image block JSON should not contain 'text' field, got: %s", s)
	}
}
