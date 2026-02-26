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
