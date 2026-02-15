// testgen_find_error_test.go — Unit tests for findTargetError and generateTestFromError.
package main

import (
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Tests for findTargetError
// ============================================

func TestFindTargetError_EmptyEntries(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	entry, id, ts := h.findTargetError("")
	if entry != nil {
		t.Fatalf("expected nil entry, got %v", entry)
	}
	if id != "" {
		t.Fatalf("expected empty id, got %q", id)
	}
	if ts != 0 {
		t.Fatalf("expected 0 timestamp, got %d", ts)
	}
}

func TestFindTargetError_NoErrorLevel(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "info", "message": "just info"},
		{"level": "warn", "message": "a warning"},
	}
	h.server.mu.Unlock()

	entry, id, ts := h.findTargetError("")
	if entry != nil {
		t.Fatalf("expected nil entry for no-error entries, got %v", entry)
	}
	if id != "" || ts != 0 {
		t.Fatalf("expected empty results, got id=%q ts=%d", id, ts)
	}
}

func TestFindTargetError_ReturnsLastError(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "first error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
		{"level": "info", "message": "info between"},
		{"level": "error", "message": "second error", "error_id": "e2", "ts": "2024-01-01T00:00:02Z"},
	}
	h.server.mu.Unlock()

	entry, id, _ := h.findTargetError("")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	msg, _ := entry["message"].(string)
	if msg != "second error" {
		t.Fatalf("message = %q, want %q", msg, "second error")
	}
	if id != "e2" {
		t.Fatalf("id = %q, want %q", id, "e2")
	}
}

func TestFindTargetError_SpecificErrorID(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "first error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
		{"level": "error", "message": "second error", "error_id": "e2", "ts": "2024-01-01T00:00:02Z"},
		{"level": "error", "message": "third error", "error_id": "e3", "ts": "2024-01-01T00:00:03Z"},
	}
	h.server.mu.Unlock()

	entry, id, _ := h.findTargetError("e1")
	if entry == nil {
		t.Fatal("expected non-nil entry for specific errorID")
	}
	msg, _ := entry["message"].(string)
	if msg != "first error" {
		t.Fatalf("message = %q, want %q", msg, "first error")
	}
	if id != "e1" {
		t.Fatalf("id = %q, want %q", id, "e1")
	}
}

func TestFindTargetError_SpecificErrorIDNotFound(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "an error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
	}
	h.server.mu.Unlock()

	entry, id, ts := h.findTargetError("nonexistent")
	if entry != nil {
		t.Fatalf("expected nil for nonexistent errorID, got %v", entry)
	}
	if id != "" || ts != 0 {
		t.Fatalf("expected empty results, got id=%q ts=%d", id, ts)
	}
}

func TestFindTargetError_OneError(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "only error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
	}
	h.server.mu.Unlock()

	entry, id, ts := h.findTargetError("")
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

// ============================================
// Tests for generateTestFromError
// ============================================

func TestGenerateTestFromError_NoErrors(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	_, err := h.generateTestFromError(TestFromContextRequest{Framework: "playwright"})
	if err == nil {
		t.Fatal("expected error for no error context")
	}
	if !strings.Contains(err.Error(), ErrNoErrorContext) {
		t.Fatalf("error = %q, want to contain %q", err.Error(), ErrNoErrorContext)
	}
}

func TestGenerateTestFromError_ErrorButNoActions(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "test error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
	}
	h.server.mu.Unlock()

	_, err := h.generateTestFromError(TestFromContextRequest{Framework: "playwright"})
	if err == nil {
		t.Fatal("expected error when no actions captured")
	}
	if !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %q, want to contain %q", err.Error(), ErrNoActionsCaptured)
	}
}

func TestGenerateTestFromError_Success(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	errorTime := time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC)

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "click failed", "error_id": "e1", "ts": errorTime.Format(time.RFC3339)},
	}
	h.server.mu.Unlock()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{
			Type:      "click",
			Selectors: map[string]any{"target": "#submit-btn"},
			URL:       "https://app.example.com",
			Timestamp: errorTime.Add(-2 * time.Second).UnixMilli(),
		},
		{
			Type:      "input",
			Selectors: map[string]any{"target": "#email"},
			Value:     "user@test.com",
			URL:       "https://app.example.com",
			Timestamp: errorTime.Add(-1 * time.Second).UnixMilli(),
		},
	})

	result, err := h.generateTestFromError(TestFromContextRequest{
		Framework: "playwright",
		BaseURL:   "https://app.example.com",
	})
	if err != nil {
		t.Fatalf("generateTestFromError error = %v", err)
	}
	if result.Framework != "playwright" {
		t.Fatalf("Framework = %q, want playwright", result.Framework)
	}
	if result.Content == "" {
		t.Fatal("expected non-empty script content")
	}
	if result.Metadata.SourceError != "e1" {
		t.Fatalf("SourceError = %q, want %q", result.Metadata.SourceError, "e1")
	}
	if !result.Coverage.ErrorReproduced {
		t.Fatal("expected ErrorReproduced to be true")
	}
}
