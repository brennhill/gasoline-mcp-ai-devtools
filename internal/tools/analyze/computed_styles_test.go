// computed_styles_test.go — Tests for computed styles handler argument parsing.
package analyze

import (
	"encoding/json"
	"testing"
)

func TestParseComputedStylesArgs_MissingSelectorError(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{})
	_, err := ParseComputedStylesArgs(args)
	if err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestParseComputedStylesArgs_ValidSelector(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{"selector": ".subline"})
	parsed, err := ParseComputedStylesArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Selector != ".subline" {
		t.Errorf("expected selector '.subline', got %q", parsed.Selector)
	}
}

func TestParseComputedStylesArgs_PropertiesFilter(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{
		"selector":   "body",
		"properties": []string{"color", "font-size"},
	})
	parsed, err := ParseComputedStylesArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(parsed.Properties))
	}
}

func TestParseComputedStylesArgs_FrameParam(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{
		"selector": "body",
		"frame":    "iframe.content",
	})
	parsed, err := ParseComputedStylesArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Frame != "iframe.content" {
		t.Errorf("expected frame 'iframe.content', got %q", parsed.Frame)
	}
}
