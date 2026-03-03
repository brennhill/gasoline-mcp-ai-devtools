// Purpose: Tests for test-generation script output.
// Docs: docs/features/feature/test-generation/index.md

// testgen_generate_test.go — Tests for testgen.go pure/helper functions.
// Only tests that cover behavior not already tested in internal/testgen/*_test.go.
package main

import (
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// newTestToolHandler creates a minimal ToolHandler for unit tests.
// It sets up a real Capture instance and a Server with empty entries.
func newTestToolHandler() *ToolHandler {
	cap := capture.NewCapture()
	srv := &Server{
		entries: make([]LogEntry, 0),
		mu:      sync.RWMutex{},
	}
	return &ToolHandler{
		MCPHandler:           &MCPHandler{server: srv},
		capture:              cap,
		elementIndexRegistry: newElementIndexRegistry(),
	}
}

// ============================================
// Tests for getActionsInTimeWindow
// ============================================

func TestGetActionsInTimeWindow_PreservesActionFields(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{
			Type:      "click",
			Timestamp: 1000,
			URL:       "https://example.com",
			Value:     "submit",
			Selectors: map[string]any{"target": "#btn"},
		},
	})

	result, err := h.testGen().getActionsInTimeWindow(1000, 500)
	if err != nil {
		t.Fatalf("getActionsInTimeWindow error = %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("got %d actions, want 1", len(result))
	}
	if result[0].URL != "https://example.com" {
		t.Fatalf("URL = %q, want https://example.com", result[0].URL)
	}
	if result[0].Value != "submit" {
		t.Fatalf("Value = %q, want submit", result[0].Value)
	}
	if result[0].Selectors["target"] != "#btn" {
		t.Fatalf("Selectors[target] = %v, want #btn", result[0].Selectors["target"])
	}
}

// ============================================
// Tests for countNetworkAssertions
// ============================================

func TestCountNetworkAssertions_AllZeroStatus(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddNetworkBodiesForTest([]capture.NetworkBody{
		{Method: "GET", URL: "/api/a", Status: 0},
		{Method: "GET", URL: "/api/b", Status: 0},
	})

	count := h.testGen().countNetworkAssertions()
	if count != 0 {
		t.Fatalf("countNetworkAssertions(all zero) = %d, want 0", count)
	}
}

// ============================================
// Tests for collectErrorMessages
// ============================================

func TestCollectErrorMessages_NoErrors(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "info", "message": "all good"},
		{"level": "warn", "message": "minor issue"},
	}
	h.server.mu.Unlock()

	msgs := h.testGen().collectErrorMessages(5)
	if len(msgs) != 0 {
		t.Fatalf("collectErrorMessages(no errors) len = %d, want 0", len(msgs))
	}
}

// ============================================
// Tests for buildRegressionAssertions
// ============================================

func TestBuildRegressionAssertions_ErrorsWithNetwork(t *testing.T) {
	t.Parallel()

	errors := []string{"some error"}
	bodies := []capture.NetworkBody{
		{Method: "GET", URL: "/api/data", Status: 200},
	}
	assertions, count := buildRegressionAssertions(errors, bodies)

	// 0 for baseline with errors + 1 network assertion
	if count != 1 {
		t.Fatalf("assertionCount = %d, want 1", count)
	}
	joined := strings.Join(assertions, "\n")
	if !strings.Contains(joined, "Baseline had 1 console errors") {
		t.Fatal("expected baseline error comment")
	}
	if !strings.Contains(joined, "Assert GET /api/data returns 200") {
		t.Fatal("expected network assertion")
	}
}

// ============================================
// Tests for insertAssertionsBeforeClose
// ============================================

func TestInsertAssertionsBeforeClose_EmptyAssertions(t *testing.T) {
	t.Parallel()

	script := "test('test', () => {\n});\n"
	result := insertAssertionsBeforeClose(script, nil)

	if !strings.Contains(result, "});") {
		t.Fatal("result should still contain closing brace")
	}
}

func TestInsertAssertionsBeforeClose_MultipleClosingBraces(t *testing.T) {
	t.Parallel()

	script := "test('outer', () => {\n  test('inner', () => {\n  });\n});\n"
	assertions := []string{"  // final assertion"}

	result := insertAssertionsBeforeClose(script, assertions)

	lastClose := strings.LastIndex(result, "});")
	assertIdx := strings.LastIndex(result, "// final assertion")
	if assertIdx > lastClose {
		t.Fatal("assertion should appear before the last });")
	}
}
