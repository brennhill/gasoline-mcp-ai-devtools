// tools_observe_unit_test.go — Unit tests for observe tool helpers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

func TestAppendAlertsToResponse(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	server, err := NewServer("/tmp/test-observe-alerts.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.toolHandler.(*ToolHandler)

	t.Run("appends alert block", func(t *testing.T) {
		// Build a valid MCP response with one content block
		original := MCPToolResult{
			Content: []MCPContentBlock{
				{Type: "text", Text: "original data"},
			},
		}
		resultJSON, _ := json.Marshal(original)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(resultJSON),
		}

		alerts := []Alert{
			{Severity: "error", Category: "regression", Title: "Something broke"},
		}

		got := h.appendAlertsToResponse(resp, alerts)

		var result MCPToolResult
		if err := json.Unmarshal(got.Result, &result); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}

		if len(result.Content) != 2 {
			t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
		}
		if result.Content[0].Text != "original data" {
			t.Fatalf("original content modified: %q", result.Content[0].Text)
		}
		if !strings.Contains(result.Content[1].Text, "Something broke") {
			t.Fatalf("alert block should contain alert title, got: %q", result.Content[1].Text)
		}
	})

	t.Run("empty alerts still appends block", func(t *testing.T) {
		original := MCPToolResult{
			Content: []MCPContentBlock{
				{Type: "text", Text: "data"},
			},
		}
		resultJSON, _ := json.Marshal(original)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(resultJSON),
		}

		got := h.appendAlertsToResponse(resp, []Alert{})

		var result MCPToolResult
		if err := json.Unmarshal(got.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// Should still have 2 blocks (original + alert summary)
		if len(result.Content) != 2 {
			t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
		}
	})

	t.Run("malformed result returns unchanged", func(t *testing.T) {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`"not an object"`),
		}

		got := h.appendAlertsToResponse(resp, []Alert{{Title: "alert"}})
		// Should return the original response unchanged
		if string(got.Result) != `"not an object"` {
			t.Fatalf("malformed result should be returned unchanged, got: %s", got.Result)
		}
	})
}

// ============================================
// CR-9: parseTimestampBestEffort handles non-RFC3339 timestamps
// ============================================

func TestCR9_ParseTimestampBestEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		ok    bool // expect non-zero time
	}{
		{"RFC3339", "2026-02-18T10:00:00Z", true},
		{"RFC3339 with offset", "2026-02-18T10:00:00+01:00", true},
		{"RFC3339Nano", "2026-02-18T10:00:00.123456789Z", true},
		{"RFC3339 millis", "2026-02-18T10:00:00.123Z", true},
		{"empty string", "", false},
		{"garbage", "not-a-timestamp", false},
		{"unix epoch string", "1708252800", false}, // not supported, should return zero
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTimestampBestEffort(tt.input)
			if tt.ok && result.IsZero() {
				t.Errorf("parseTimestampBestEffort(%q) = zero, want non-zero", tt.input)
			}
			if !tt.ok && !result.IsZero() {
				t.Errorf("parseTimestampBestEffort(%q) = %v, want zero", tt.input, result)
			}
		})
	}
}
