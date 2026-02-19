// boundaries_test.go — Tests for test boundary start/end pure functions.
package configure

import (
	"encoding/json"
	"strings"
	"testing"
)

// parseResultText extracts the text content from an MCP response for assertions.
func parseResultText(t *testing.T, result json.RawMessage) string {
	t.Helper()
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(parsed.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	return parsed.Content[0].Text
}

func isErrorResult(t *testing.T, result json.RawMessage) bool {
	t.Helper()
	var parsed struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	return parsed.IsError
}

// ============================================
// ParseTestBoundaryStart tests
// ============================================

func TestParseTestBoundaryStart_Valid(t *testing.T) {
	t.Parallel()

	result, errResp := ParseTestBoundaryStart(1, json.RawMessage(`{"test_id":"test-123","label":"My Test"}`))
	if errResp != nil {
		t.Fatalf("unexpected error response: %v", errResp)
	}
	if result.TestID != "test-123" {
		t.Errorf("TestID = %q, want %q", result.TestID, "test-123")
	}
	if result.Label != "My Test" {
		t.Errorf("Label = %q, want %q", result.Label, "My Test")
	}
}

func TestParseTestBoundaryStart_DefaultLabel(t *testing.T) {
	t.Parallel()

	result, errResp := ParseTestBoundaryStart(1, json.RawMessage(`{"test_id":"abc"}`))
	if errResp != nil {
		t.Fatalf("unexpected error response: %v", errResp)
	}
	if !strings.Contains(result.Label, "abc") {
		t.Errorf("default label should contain test_id, got: %q", result.Label)
	}
}

func TestParseTestBoundaryStart_MissingTestID(t *testing.T) {
	t.Parallel()

	_, errResp := ParseTestBoundaryStart(1, json.RawMessage(`{}`))
	if errResp == nil {
		t.Fatal("expected error response for missing test_id")
	}
	if !isErrorResult(t, errResp.Result) {
		t.Fatal("should be isError:true")
	}
	text := parseResultText(t, errResp.Result)
	if !strings.Contains(text, "test_id") {
		t.Errorf("error should mention test_id, got: %s", text)
	}
}

func TestParseTestBoundaryStart_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, errResp := ParseTestBoundaryStart(1, json.RawMessage(`{bad}`))
	if errResp == nil {
		t.Fatal("expected error response for invalid JSON")
	}
	if !isErrorResult(t, errResp.Result) {
		t.Fatal("should be isError:true")
	}
	text := parseResultText(t, errResp.Result)
	if !strings.Contains(text, "invalid_json") {
		t.Errorf("error code should be invalid_json, got: %s", text)
	}
}

func TestParseTestBoundaryStart_NilArgs(t *testing.T) {
	t.Parallel()

	_, errResp := ParseTestBoundaryStart(1, nil)
	if errResp == nil {
		t.Fatal("expected error response for nil args (no test_id)")
	}
}

func TestBuildTestBoundaryStartResponse(t *testing.T) {
	t.Parallel()

	result := &TestBoundaryStartResult{TestID: "test-1", Label: "Label-1"}
	resp := BuildTestBoundaryStartResponse(1, result)

	text := parseResultText(t, resp.Result)
	if !strings.Contains(text, "test-1") {
		t.Errorf("response should contain test_id, got: %s", text)
	}
	if !strings.Contains(text, "Label-1") {
		t.Errorf("response should contain label, got: %s", text)
	}
}

// ============================================
// ParseTestBoundaryEnd tests
// ============================================

func TestParseTestBoundaryEnd_Valid(t *testing.T) {
	t.Parallel()

	result, errResp := ParseTestBoundaryEnd(1, json.RawMessage(`{"test_id":"test-456"}`))
	if errResp != nil {
		t.Fatalf("unexpected error response: %v", errResp)
	}
	if result.TestID != "test-456" {
		t.Errorf("TestID = %q, want %q", result.TestID, "test-456")
	}
}

func TestParseTestBoundaryEnd_MissingTestID(t *testing.T) {
	t.Parallel()

	_, errResp := ParseTestBoundaryEnd(1, json.RawMessage(`{}`))
	if errResp == nil {
		t.Fatal("expected error response for missing test_id")
	}
	text := parseResultText(t, errResp.Result)
	if !strings.Contains(text, "test_id") {
		t.Errorf("error should mention test_id, got: %s", text)
	}
}

func TestParseTestBoundaryEnd_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, errResp := ParseTestBoundaryEnd(1, json.RawMessage(`{bad}`))
	if errResp == nil {
		t.Fatal("expected error response for invalid JSON")
	}
}

func TestBuildTestBoundaryEndResponse(t *testing.T) {
	t.Parallel()

	result := &TestBoundaryEndResult{TestID: "test-789"}
	resp := BuildTestBoundaryEndResponse(1, result)

	text := parseResultText(t, resp.Result)
	if !strings.Contains(text, "test-789") {
		t.Errorf("response should contain test_id, got: %s", text)
	}
	if !strings.Contains(text, "was_active") {
		t.Errorf("response should contain was_active, got: %s", text)
	}
}
