// output_test.go — Tests for output formatters (human, JSON, CSV).
package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHumanFormatSuccess(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	result := &Result{
		Success: true,
		Tool:    "interact",
		Action:  "click",
		Data:    map[string]any{"selector": "#btn", "clicked": true},
	}

	h := &HumanFormatter{}
	err := h.Format(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Success") {
		t.Errorf("expected success indicator in output, got: %s", out)
	}
	if !strings.Contains(out, "interact") {
		t.Errorf("expected tool name in output, got: %s", out)
	}
}

func TestHumanFormatError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	result := &Result{
		Success: false,
		Tool:    "interact",
		Action:  "click",
		Error:   "element not found",
	}

	h := &HumanFormatter{}
	err := h.Format(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Error") {
		t.Errorf("expected error indicator in output, got: %s", out)
	}
	if !strings.Contains(out, "element not found") {
		t.Errorf("expected error message in output, got: %s", out)
	}
}

func TestJSONFormatSuccess(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	result := &Result{
		Success: true,
		Tool:    "observe",
		Action:  "logs",
		Data: map[string]any{
			"entries": []map[string]any{
				{"level": "error", "message": "test"},
			},
		},
	}

	f := &JSONFormatter{}
	err := f.Format(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if parsed["success"] != true {
		t.Errorf("expected success=true in JSON, got: %v", parsed["success"])
	}
}

func TestJSONFormatError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	result := &Result{
		Success: false,
		Tool:    "interact",
		Action:  "click",
		Error:   "selector not found",
	}

	f := &JSONFormatter{}
	err := f.Format(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["success"] != false {
		t.Errorf("expected success=false, got: %v", parsed["success"])
	}
	if parsed["error"] != "selector not found" {
		t.Errorf("expected error message, got: %v", parsed["error"])
	}
}

func TestCSVFormatSingleRow(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	result := &Result{
		Success: true,
		Tool:    "interact",
		Action:  "click",
		Data: map[string]any{
			"selector": "#btn",
			"clicked":  true,
		},
	}

	f := &CSVFormatter{}
	err := f.Format(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least header + 1 data row, got %d lines", len(lines))
	}

	// First line should be headers
	if !strings.Contains(lines[0], "success") {
		t.Errorf("expected CSV header with 'success', got: %s", lines[0])
	}
}

func TestCSVFormatMultipleRows(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	results := []*Result{
		{
			Success: true,
			Tool:    "interact",
			Action:  "click",
			Data:    map[string]any{"selector": "#btn1"},
		},
		{
			Success: false,
			Tool:    "interact",
			Action:  "click",
			Error:   "timeout",
			Data:    map[string]any{"selector": "#btn2"},
		},
	}

	f := &CSVFormatter{}
	err := f.FormatMultiple(&buf, results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Header + 2 data rows
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d: %s", len(lines), out)
	}
}

func TestStreamFormatEvents(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	events := []StreamEvent{
		{Type: "start", Status: "connecting"},
		{Type: "progress", Status: "uploading", Percent: 50},
		{Type: "complete", Status: "done", Success: true},
	}

	f := &StreamFormatter{}
	for _, e := range events {
		if err := f.WriteEvent(&buf, &e); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Each line should be valid JSON
	for i, line := range lines {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestGetFormatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format string
		want   string
	}{
		{"human", "*output.HumanFormatter"},
		{"json", "*output.JSONFormatter"},
		{"csv", "*output.CSVFormatter"},
	}

	for _, tt := range tests {
		f := GetFormatter(tt.format)
		if f == nil {
			t.Errorf("GetFormatter(%q) returned nil", tt.format)
		}
	}
}

func TestGetFormatterInvalid(t *testing.T) {
	t.Parallel()

	// Invalid format falls back to human
	f := GetFormatter("xml")
	if f == nil {
		t.Fatal("expected fallback formatter, got nil")
	}
}

func TestHumanFormatTextContent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	result := &Result{
		Success:     true,
		Tool:        "observe",
		Action:      "logs",
		TextContent: "5 log entries found\n[error] test error\n[warn] test warning",
	}

	h := &HumanFormatter{}
	err := h.Format(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "5 log entries found") {
		t.Errorf("expected text content in output, got: %s", out)
	}
}
