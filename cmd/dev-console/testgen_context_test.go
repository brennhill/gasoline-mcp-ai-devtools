// testgen_context_test.go — Tests for testgen.go context generation functions at 0% coverage.
// Covers: generateTestFromInteraction, generateTestFromRegression.
package main

import (
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Tests for generateTestFromInteraction
// ============================================

func TestGenerateTestFromInteraction_NoActions(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	_, err := h.generateTestFromInteraction(TestFromContextRequest{
		Framework: "playwright",
	})
	if err == nil {
		t.Fatal("generateTestFromInteraction should fail with no actions")
	}
	if !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %q, want to contain %q", err.Error(), ErrNoActionsCaptured)
	}
}

func TestGenerateTestFromInteraction_Basic(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#btn", "testId": "submit"}, URL: "https://app.example.com"},
		{Type: "input", Selectors: map[string]any{"target": "#email"}, Value: "user@test.com"},
	})

	result, err := h.generateTestFromInteraction(TestFromContextRequest{
		Framework: "playwright",
		BaseURL:   "https://app.example.com",
	})
	if err != nil {
		t.Fatalf("generateTestFromInteraction error = %v", err)
	}

	if result.Framework != "playwright" {
		t.Fatalf("Framework = %q, want playwright", result.Framework)
	}
	if result.Coverage.ErrorReproduced {
		t.Fatal("ErrorReproduced should be false for interaction test")
	}
	if !result.Coverage.StateCaptured {
		t.Fatal("StateCaptured should be true when actions exist")
	}
	if result.Coverage.NetworkMocked {
		t.Fatal("NetworkMocked should be false when IncludeMocks is false")
	}
	if !strings.Contains(result.Content, "await page.click('#btn')") {
		t.Fatalf("generated script should contain click action;\nContent:\n%s", result.Content)
	}
	if len(result.Metadata.ContextUsed) != 1 || result.Metadata.ContextUsed[0] != "actions" {
		t.Fatalf("ContextUsed = %v, want [actions]", result.Metadata.ContextUsed)
	}
	if result.Metadata.GeneratedAt == "" {
		t.Fatal("GeneratedAt should not be empty")
	}
	if result.Metadata.SourceError != "" {
		t.Fatalf("SourceError should be empty for interaction test; got %q", result.Metadata.SourceError)
	}
	if result.Filename == "" {
		t.Fatal("Filename should not be empty")
	}
	if !strings.HasSuffix(result.Filename, ".spec.ts") {
		t.Fatalf("Filename = %q, want .spec.ts suffix", result.Filename)
	}
}

func TestGenerateTestFromInteraction_WithMocks(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#btn"}},
	})
	h.capture.AddNetworkBodiesForTest([]capture.NetworkBody{
		{Method: "GET", URL: "/api/data", Status: 200},
		{Method: "POST", URL: "/api/submit", Status: 201},
		{Method: "GET", URL: "/api/noop", Status: 0}, // status 0 should not count
	})

	result, err := h.generateTestFromInteraction(TestFromContextRequest{
		Framework:    "playwright",
		IncludeMocks: true,
	})
	if err != nil {
		t.Fatalf("generateTestFromInteraction(mocks) error = %v", err)
	}

	if !result.Coverage.NetworkMocked {
		t.Fatal("NetworkMocked should be true when IncludeMocks is true")
	}
	// Assertion count should include network assertions (2 with status > 0)
	if result.Assertions < 2 {
		t.Fatalf("Assertions = %d, want >= 2 (at least 2 network assertions)", result.Assertions)
	}
	if len(result.Metadata.ContextUsed) != 2 {
		t.Fatalf("ContextUsed len = %d, want 2 (actions, network)", len(result.Metadata.ContextUsed))
	}
	hasActions := false
	hasNetwork := false
	for _, ctx := range result.Metadata.ContextUsed {
		if ctx == "actions" {
			hasActions = true
		}
		if ctx == "network" {
			hasNetwork = true
		}
	}
	if !hasActions || !hasNetwork {
		t.Fatalf("ContextUsed = %v, want [actions, network]", result.Metadata.ContextUsed)
	}
}

func TestGenerateTestFromInteraction_VitestFramework(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", URL: "https://example.com"},
	})

	result, err := h.generateTestFromInteraction(TestFromContextRequest{
		Framework: "vitest",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Framework != "vitest" {
		t.Fatalf("Framework = %q, want vitest", result.Framework)
	}
	if !strings.HasSuffix(result.Filename, ".test.ts") {
		t.Fatalf("Filename = %q, want .test.ts suffix for vitest", result.Filename)
	}
}

func TestGenerateTestFromInteraction_Selectors(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"testId": "login-btn", "id": "loginBtn"}},
	})

	result, err := h.generateTestFromInteraction(TestFromContextRequest{
		Framework: "playwright",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result.Selectors) == 0 {
		t.Fatal("Selectors should not be empty")
	}
}

func TestGenerateTestFromInteraction_NoMocksContextUsed(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#btn"}},
	})

	result, err := h.generateTestFromInteraction(TestFromContextRequest{
		Framework:    "playwright",
		IncludeMocks: false,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result.Metadata.ContextUsed) != 1 {
		t.Fatalf("ContextUsed len = %d, want 1 (only actions)", len(result.Metadata.ContextUsed))
	}
	if result.Metadata.ContextUsed[0] != "actions" {
		t.Fatalf("ContextUsed[0] = %q, want actions", result.Metadata.ContextUsed[0])
	}
}

// ============================================
// Tests for generateTestFromRegression
// ============================================

func TestGenerateTestFromRegression_NoActions(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	_, err := h.generateTestFromRegression(TestFromContextRequest{
		Framework: "playwright",
	})
	if err == nil {
		t.Fatal("generateTestFromRegression should fail with no actions")
	}
	if !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %q, want to contain %q", err.Error(), ErrNoActionsCaptured)
	}
}

func TestGenerateTestFromRegression_CleanBaseline(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#submit"}},
		{Type: "navigate", ToURL: "https://app.example.com/dashboard"},
	})

	result, err := h.generateTestFromRegression(TestFromContextRequest{
		Framework: "playwright",
		BaseURL:   "https://app.example.com",
	})
	if err != nil {
		t.Fatalf("generateTestFromRegression error = %v", err)
	}

	if result.Framework != "playwright" {
		t.Fatalf("Framework = %q, want playwright", result.Framework)
	}
	if result.Filename != "regression-test.spec.ts" {
		t.Fatalf("Filename = %q, want regression-test.spec.ts", result.Filename)
	}
	if !result.Coverage.StateCaptured {
		t.Fatal("StateCaptured should be true")
	}
	if result.Coverage.ErrorReproduced {
		t.Fatal("ErrorReproduced should be false for regression test")
	}
	if result.Assertions < 1 {
		t.Fatalf("Assertions = %d, want >= 1 (clean baseline assertion)", result.Assertions)
	}
	if !strings.Contains(result.Content, "expect(consoleErrors).toHaveLength(0)") {
		t.Fatalf("expected clean baseline assertion in content;\nContent:\n%s", result.Content)
	}

	// Check metadata context
	expectedContexts := map[string]bool{
		"actions":     false,
		"console":     false,
		"network":     false,
		"performance": false,
	}
	for _, ctx := range result.Metadata.ContextUsed {
		expectedContexts[ctx] = true
	}
	for ctx, found := range expectedContexts {
		if !found {
			t.Fatalf("ContextUsed missing %q; got %v", ctx, result.Metadata.ContextUsed)
		}
	}
}

func TestGenerateTestFromRegression_WithErrorsAndNetwork(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#btn"}},
	})
	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "TypeError: undefined is not a function"},
		{"level": "error", "message": "ReferenceError: x is not defined"},
	}
	h.server.mu.Unlock()
	h.capture.AddNetworkBodiesForTest([]capture.NetworkBody{
		{Method: "GET", URL: "/api/users", Status: 200},
	})

	result, err := h.generateTestFromRegression(TestFromContextRequest{
		Framework: "playwright",
	})
	if err != nil {
		t.Fatalf("generateTestFromRegression error = %v", err)
	}

	if !strings.Contains(result.Content, "Baseline had 2 console errors") {
		t.Fatalf("expected baseline error comment in content;\nContent:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "Assert GET /api/users returns 200") {
		t.Fatalf("expected network assertion in content;\nContent:\n%s", result.Content)
	}
}

func TestGenerateTestFromRegression_WithMocks(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#x"}},
	})

	result, err := h.generateTestFromRegression(TestFromContextRequest{
		Framework:    "playwright",
		IncludeMocks: true,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !result.Coverage.NetworkMocked {
		t.Fatal("NetworkMocked should be true when IncludeMocks is true")
	}
}

func TestGenerateTestFromRegression_JestFramework(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#a"}},
	})

	result, err := h.generateTestFromRegression(TestFromContextRequest{
		Framework: "jest",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Framework != "jest" {
		t.Fatalf("Framework = %q, want jest", result.Framework)
	}
	if !strings.HasSuffix(result.Filename, ".test.ts") {
		t.Fatalf("Filename = %q, want .test.ts suffix for jest", result.Filename)
	}
}

func TestGenerateTestFromRegression_SelectorsExtracted(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"testId": "save-btn", "id": "saveBtn"}},
	})

	result, err := h.generateTestFromRegression(TestFromContextRequest{
		Framework: "playwright",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result.Selectors) == 0 {
		t.Fatal("Selectors should not be empty for regression test")
	}
}

func TestGenerateTestFromRegression_ContentHasActions(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#go"}},
		{Type: "input", Selectors: map[string]any{"target": "#name"}, Value: "test"},
	})

	result, err := h.generateTestFromRegression(TestFromContextRequest{
		Framework: "playwright",
		BaseURL:   "https://example.com",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(result.Content, "await page.click('#go')") {
		t.Fatalf("content should contain click action;\nContent:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "await page.fill('#name', 'test')") {
		t.Fatalf("content should contain fill action;\nContent:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "await page.goto('https://example.com')") {
		t.Fatalf("content should contain goto baseURL;\nContent:\n%s", result.Content)
	}
}
