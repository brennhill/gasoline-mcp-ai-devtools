// Purpose: Tests for test-generation error discovery from context.
// Docs: docs/features/feature/test-generation/index.md

// testgen_find_error_test.go — Tests for findTargetError edge cases not covered
// by internal/testgen/generate_test.go.
package main

import (
	"testing"
)

// ============================================
// Tests for findTargetError
// ============================================

func TestFindTargetError_OneError(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "only error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
	}
	h.server.mu.Unlock()

	entry, id, ts := h.testGen().findTargetError("")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if id != "e1" {
		t.Fatalf("id = %q, want %q", id, "e1")
	}
	if ts == 0 {
		t.Fatal("expected non-zero timestamp")
	}
}
