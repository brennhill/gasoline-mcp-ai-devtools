package main

import (
	"encoding/json"
	"testing"
)

func TestArgHasKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args string
		key  string
		want bool
	}{
		{"present string", `{"summary":true,"limit":10}`, "summary", true},
		{"present false", `{"summary":false}`, "summary", true},
		{"absent", `{"limit":10}`, "summary", false},
		{"empty object", `{}`, "summary", false},
		{"null args", `null`, "summary", false},
		{"full key present", `{"full":true}`, "full", true},
		{"full key absent", `{"summary":true}`, "full", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := argHasKey(json.RawMessage(tt.args), tt.key)
			if got != tt.want {
				t.Errorf("argHasKey(%s, %q) = %v, want %v", tt.args, tt.key, got, tt.want)
			}
		})
	}
}

func TestMaybeInjectSummary_NoPreference(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}
	// No preference set (summaryPrefReady=false, summaryPrefValue=false)
	args := json.RawMessage(`{"what":"errors","limit":10}`)
	result := h.maybeInjectSummary(args)

	// Should return args unchanged
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parsed["summary"]; ok {
		t.Error("expected no summary key when preference not set")
	}
}

func TestMaybeInjectSummary_PreferenceSet(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}
	h.summaryPrefReady = true
	h.summaryPrefValue = true

	args := json.RawMessage(`{"what":"errors","limit":10}`)
	result := h.maybeInjectSummary(args)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	summary, ok := parsed["summary"]
	if !ok {
		t.Fatal("expected summary key to be injected")
	}
	if summary != true {
		t.Errorf("expected summary=true, got %v", summary)
	}
}

func TestMaybeInjectSummary_ExplicitSummaryFalse(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}
	h.summaryPrefReady = true
	h.summaryPrefValue = true

	args := json.RawMessage(`{"what":"errors","summary":false}`)
	result := h.maybeInjectSummary(args)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// summary key was already present, so it should NOT be overridden
	summary, ok := parsed["summary"]
	if !ok {
		t.Fatal("expected summary key to still be present")
	}
	if summary != false {
		t.Errorf("expected summary=false (explicit override), got %v", summary)
	}
}

func TestMaybeInjectSummary_ExplicitFullTrue(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}
	h.summaryPrefReady = true
	h.summaryPrefValue = true

	args := json.RawMessage(`{"what":"errors","full":true}`)
	result := h.maybeInjectSummary(args)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// "full" is present, so summary should NOT be injected
	if _, ok := parsed["summary"]; ok {
		t.Error("expected no summary key when full=true is present")
	}
}

func TestInvalidateSummaryPref(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}
	h.summaryPrefReady = true
	h.summaryPrefValue = true

	h.invalidateSummaryPref()

	if h.summaryPrefReady {
		t.Error("expected summaryPrefReady to be false after invalidation")
	}
}

func TestMaybeInjectSummary_NilArgs(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}
	h.summaryPrefReady = true
	h.summaryPrefValue = true

	// nil args should get summary injected
	result := h.maybeInjectSummary(nil)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parsed["summary"]; !ok {
		t.Error("expected summary key to be injected into nil args")
	}
}

func TestMaybeInjectSummary_EmptyArgs(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}
	h.summaryPrefReady = true
	h.summaryPrefValue = true

	result := h.maybeInjectSummary(json.RawMessage(`{}`))

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	summary, ok := parsed["summary"]
	if !ok {
		t.Fatal("expected summary key to be injected into empty args")
	}
	if summary != true {
		t.Errorf("expected summary=true, got %v", summary)
	}
}
