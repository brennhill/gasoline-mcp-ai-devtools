// visual_diff_test.go — Tests for visual baseline/diff handler functions.
package analyze

import (
	"encoding/json"
	"testing"
)

func TestSaveVisualBaseline_MissingNameError(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{})
	_, err := ParseVisualBaselineArgs(args)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestSaveVisualBaseline_ValidName(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{"name": "homepage"})
	parsed, err := ParseVisualBaselineArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Name != "homepage" {
		t.Errorf("expected name 'homepage', got %q", parsed.Name)
	}
}

func TestGetVisualDiff_MissingBaselineError(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{})
	_, err := ParseVisualDiffArgs(args)
	if err == nil {
		t.Fatal("expected error for missing baseline")
	}
}

func TestGetVisualDiff_ValidArgs(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{"baseline": "homepage", "threshold": 50})
	parsed, err := ParseVisualDiffArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Baseline != "homepage" {
		t.Errorf("expected baseline 'homepage', got %q", parsed.Baseline)
	}
	if parsed.Threshold != 50 {
		t.Errorf("expected threshold 50, got %d", parsed.Threshold)
	}
}

func TestGetVisualDiff_DefaultThreshold(t *testing.T) {
	t.Parallel()
	args, _ := json.Marshal(map[string]any{"baseline": "homepage"})
	parsed, err := ParseVisualDiffArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Threshold != 30 {
		t.Errorf("expected default threshold 30, got %d", parsed.Threshold)
	}
}

func TestVisualDiffVerdict(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pct     float64
		verdict string
	}{
		{0, "identical"},
		{0.5, "minor_changes"},
		{4.9, "minor_changes"},
		{5.0, "major_changes"},
		{24.9, "major_changes"},
		{25.0, "completely_different"},
		{100, "completely_different"},
	}
	for _, tc := range tests {
		v := DiffVerdict(tc.pct)
		if v != tc.verdict {
			t.Errorf("DiffVerdict(%.1f) = %q, want %q", tc.pct, v, tc.verdict)
		}
	}
}
