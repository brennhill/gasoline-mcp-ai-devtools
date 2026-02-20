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
