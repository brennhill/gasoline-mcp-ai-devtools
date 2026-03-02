package a11ysummary

import "testing"

func TestBuildSummary_IncludesCanonicalAndLegacyKeys(t *testing.T) {
	t.Parallel()
	got := BuildSummary(Counts{
		Violations:   2,
		Passes:       10,
		Incomplete:   3,
		Inapplicable: 4,
	})

	if got["violations"] != 2 || got["violation_count"] != 2 {
		t.Fatalf("violations keys mismatch: %+v", got)
	}
	if got["passes"] != 10 || got["pass_count"] != 10 {
		t.Fatalf("passes keys mismatch: %+v", got)
	}
	if got["incomplete"] != 3 || got["incomplete_count"] != 3 {
		t.Fatalf("incomplete keys mismatch: %+v", got)
	}
	if got["inapplicable"] != 4 || got["inapplicable_count"] != 4 {
		t.Fatalf("inapplicable keys mismatch: %+v", got)
	}
}

func TestEnsureAuditSummary_AddsSummaryWhenMissing(t *testing.T) {
	t.Parallel()
	audit := map[string]any{
		"violations":   []any{1, 2},
		"passes":       []any{1},
		"incomplete":   []any{1, 2, 3},
		"inapplicable": []any{},
	}

	EnsureAuditSummary(audit)
	summary, ok := audit["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary map, got %T", audit["summary"])
	}

	if summary["violations"] != 2 || summary["violation_count"] != 2 {
		t.Fatalf("violations mismatch: %+v", summary)
	}
	if summary["passes"] != 1 || summary["pass_count"] != 1 {
		t.Fatalf("passes mismatch: %+v", summary)
	}
	if summary["incomplete"] != 3 || summary["incomplete_count"] != 3 {
		t.Fatalf("incomplete mismatch: %+v", summary)
	}
	if summary["inapplicable"] != 0 || summary["inapplicable_count"] != 0 {
		t.Fatalf("inapplicable mismatch: %+v", summary)
	}
}

func TestEnsureAuditSummary_NormalizesExistingCanonicalSummary(t *testing.T) {
	t.Parallel()
	audit := map[string]any{
		"violations": []any{1},
		"passes":     []any{1, 2},
		"summary": map[string]any{
			"violations": 9,
			"passes":     8,
			"custom":     "keep",
		},
	}

	EnsureAuditSummary(audit)
	summary := audit["summary"].(map[string]any)

	if summary["violations"] != 9 || summary["violation_count"] != 9 {
		t.Fatalf("violations normalization mismatch: %+v", summary)
	}
	if summary["passes"] != 8 || summary["pass_count"] != 8 {
		t.Fatalf("passes normalization mismatch: %+v", summary)
	}
	if summary["custom"] != "keep" {
		t.Fatalf("custom field should be preserved: %+v", summary)
	}
}

func TestEnsureAuditSummary_NormalizesExistingLegacySummary(t *testing.T) {
	t.Parallel()
	audit := map[string]any{
		"summary": map[string]any{
			"violation_count": 5,
			"pass_count":      6,
		},
	}

	EnsureAuditSummary(audit)
	summary := audit["summary"].(map[string]any)

	if summary["violations"] != 5 || summary["violation_count"] != 5 {
		t.Fatalf("violations normalization mismatch: %+v", summary)
	}
	if summary["passes"] != 6 || summary["pass_count"] != 6 {
		t.Fatalf("passes normalization mismatch: %+v", summary)
	}
}

func TestEnsureAuditSummary_RebuildsInvalidSummaryType(t *testing.T) {
	t.Parallel()
	audit := map[string]any{
		"violations": []any{1, 2, 3},
		"summary":    "bad",
	}

	EnsureAuditSummary(audit)
	summary, ok := audit["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary map, got %T", audit["summary"])
	}
	if summary["violations"] != 3 || summary["violation_count"] != 3 {
		t.Fatalf("rebuild mismatch: %+v", summary)
	}
}
