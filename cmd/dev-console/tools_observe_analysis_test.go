// tools_observe_analysis_test.go — Tests for ensureA11ySummary function.
// Note: executeA11yQuery is tested indirectly (requires live capture + extension).
package main

import (
	"testing"
)

// ============================================
// ensureA11ySummary
// ============================================

func TestEnsureA11ySummary_AddsSummaryWhenMissing(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": []any{
			map[string]any{"id": "color-contrast"},
			map[string]any{"id": "image-alt"},
		},
		"passes": []any{
			map[string]any{"id": "html-has-lang"},
		},
	}

	ensureA11ySummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violation_count"] != 2 {
		t.Errorf("expected violation_count 2, got %v", summary["violation_count"])
	}
	if summary["pass_count"] != 1 {
		t.Errorf("expected pass_count 1, got %v", summary["pass_count"])
	}
}

func TestEnsureA11ySummary_PreservesExistingSummary(t *testing.T) {
	t.Parallel()
	existingSummary := map[string]any{
		"violation_count": 99,
		"pass_count":      88,
		"custom_field":    "preserved",
	}
	auditResult := map[string]any{
		"violations": []any{
			map[string]any{"id": "color-contrast"},
		},
		"passes":  []any{},
		"summary": existingSummary,
	}

	ensureA11ySummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	// Should NOT overwrite existing summary
	if summary["violation_count"] != 99 {
		t.Errorf("expected existing violation_count 99 to be preserved, got %v", summary["violation_count"])
	}
	if summary["pass_count"] != 88 {
		t.Errorf("expected existing pass_count 88 to be preserved, got %v", summary["pass_count"])
	}
	if summary["custom_field"] != "preserved" {
		t.Errorf("expected custom_field to be preserved, got %v", summary["custom_field"])
	}
}

func TestEnsureA11ySummary_NoViolationsNoPassses(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{}

	ensureA11ySummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violation_count"] != 0 {
		t.Errorf("expected violation_count 0, got %v", summary["violation_count"])
	}
	if summary["pass_count"] != 0 {
		t.Errorf("expected pass_count 0, got %v", summary["pass_count"])
	}
}

func TestEnsureA11ySummary_ViolationsNotArray(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": "not an array",
		"passes":     42,
	}

	ensureA11ySummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	// Type assertion fails gracefully, len of nil slice is 0
	if summary["violation_count"] != 0 {
		t.Errorf("expected violation_count 0 for non-array violations, got %v", summary["violation_count"])
	}
	if summary["pass_count"] != 0 {
		t.Errorf("expected pass_count 0 for non-array passes, got %v", summary["pass_count"])
	}
}

func TestEnsureA11ySummary_OnlyViolations(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": []any{
			map[string]any{"id": "v1"},
			map[string]any{"id": "v2"},
			map[string]any{"id": "v3"},
		},
	}

	ensureA11ySummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violation_count"] != 3 {
		t.Errorf("expected violation_count 3, got %v", summary["violation_count"])
	}
	if summary["pass_count"] != 0 {
		t.Errorf("expected pass_count 0 when passes key missing, got %v", summary["pass_count"])
	}
}

func TestEnsureA11ySummary_OnlyPasses(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"passes": []any{
			map[string]any{"id": "p1"},
			map[string]any{"id": "p2"},
		},
	}

	ensureA11ySummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violation_count"] != 0 {
		t.Errorf("expected violation_count 0 when violations key missing, got %v", summary["violation_count"])
	}
	if summary["pass_count"] != 2 {
		t.Errorf("expected pass_count 2, got %v", summary["pass_count"])
	}
}

func TestEnsureA11ySummary_EmptyArrays(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": []any{},
		"passes":     []any{},
	}

	ensureA11ySummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violation_count"] != 0 {
		t.Errorf("expected violation_count 0, got %v", summary["violation_count"])
	}
	if summary["pass_count"] != 0 {
		t.Errorf("expected pass_count 0, got %v", summary["pass_count"])
	}
}

func TestEnsureA11ySummary_NilViolationsKey(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": nil,
		"passes":     nil,
	}

	ensureA11ySummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violation_count"] != 0 {
		t.Errorf("expected violation_count 0 for nil violations, got %v", summary["violation_count"])
	}
	if summary["pass_count"] != 0 {
		t.Errorf("expected pass_count 0 for nil passes, got %v", summary["pass_count"])
	}
}

func TestBuildA11yQueryParams_IncludesFrameWhenProvided(t *testing.T) {
	t.Parallel()

	params := buildA11yQueryParams("#app", []string{"wcag2a"}, "iframe.editor")

	if got := params["scope"]; got != "#app" {
		t.Fatalf("scope = %v, want #app", got)
	}
	if _, ok := params["tags"]; !ok {
		t.Fatal("expected tags in query params")
	}
	if got := params["frame"]; got != "iframe.editor" {
		t.Fatalf("frame = %v, want iframe.editor", got)
	}
}

func TestBuildA11yQueryParams_OmitsFrameWhenNil(t *testing.T) {
	t.Parallel()

	params := buildA11yQueryParams("", nil, nil)

	if _, ok := params["frame"]; ok {
		t.Fatal("frame should be omitted when nil")
	}
	if _, ok := params["scope"]; ok {
		t.Fatal("scope should be omitted when empty")
	}
	if _, ok := params["tags"]; ok {
		t.Fatal("tags should be omitted when empty")
	}
}
