// Purpose: Tests for interact entrypoint quiet aliases.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"testing"
)

func TestMergeAsyncAlias_RewritesAsyncToBackground(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{"what":"click","async":true}`)
	result := mergeAsyncAlias(input)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := parsed["async"]; ok {
		t.Error("async should be removed from args")
	}
	if bg, ok := parsed["background"]; !ok {
		t.Error("background should be set")
	} else if bg != true {
		t.Errorf("background = %v, want true", bg)
	}
}

func TestMergeAsyncAlias_BackgroundTakesPrecedence(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{"what":"click","async":true,"background":false}`)
	result := mergeAsyncAlias(input)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if bg, ok := parsed["background"]; !ok {
		t.Error("background should be preserved")
	} else if bg != false {
		t.Errorf("background = %v, want false (explicit takes precedence)", bg)
	}
}

func TestMergeAsyncAlias_NoAsyncNoChange(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{"what":"click","selector":"#btn"}`)
	result := mergeAsyncAlias(input)

	var orig, parsed map[string]any
	if err := json.Unmarshal(input, &orig); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if len(orig) != len(parsed) {
		t.Errorf("args changed unexpectedly: orig=%v result=%v", orig, parsed)
	}
}
