// Purpose: Tests for form validation analysis.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/analyze-tool/index.md

// forms_test.go — Tests for form discovery handler argument parsing.
package analyze

import (
	"encoding/json"
	"testing"
)

func TestParseFormsArgs_NoSelector(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{})
	parsed, err := ParseFormsArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Selector != "" {
		t.Errorf("expected empty selector, got %q", parsed.Selector)
	}
}

func TestParseFormsArgs_SelectorFilters(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{"selector": "form#register"})
	parsed, err := ParseFormsArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Selector != "form#register" {
		t.Errorf("expected selector 'form#register', got %q", parsed.Selector)
	}
}

func TestParseFormValidationArgs_ValidMode(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{"selector": "form#login"})
	parsed, err := ParseFormValidationArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Selector != "form#login" {
		t.Errorf("expected selector 'form#login', got %q", parsed.Selector)
	}
}

func TestParseFormsArgs_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := ParseFormsArgs(json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseDataTableArgs_WithSelectorAndLimits(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{
		"selector": "table.report",
		"max_rows": 25,
		"max_cols": 12,
		"tab_id":   7,
	})
	parsed, err := ParseDataTableArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Selector != "table.report" {
		t.Errorf("expected selector 'table.report', got %q", parsed.Selector)
	}
	if parsed.MaxRows != 25 {
		t.Errorf("expected max_rows 25, got %d", parsed.MaxRows)
	}
	if parsed.MaxCols != 12 {
		t.Errorf("expected max_cols 12, got %d", parsed.MaxCols)
	}
	if parsed.TabID != 7 {
		t.Errorf("expected tab_id 7, got %d", parsed.TabID)
	}
}

func TestParseDataTableArgs_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := ParseDataTableArgs(json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
