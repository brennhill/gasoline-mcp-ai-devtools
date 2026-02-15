// testgen_generate_test.go — Tests for testgen.go pure/helper functions at 0% coverage.
// Covers: getActionsInTimeWindow, deriveInteractionTestName, countNetworkAssertions,
// collectErrorMessages, buildRegressionAssertions, insertAssertionsBeforeClose.
package main

import (
	"strings"
	"sync"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
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
		MCPHandler: &MCPHandler{server: srv},
		capture:    cap,
	}
}

// ============================================
// Tests for getActionsInTimeWindow
// ============================================

func TestGetActionsInTimeWindow_NoActions(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	_, err := h.getActionsInTimeWindow(1000, 500)
	if err == nil {
		t.Fatal("getActionsInTimeWindow should return error when no actions captured")
	}
	if !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %q, want to contain %q", err.Error(), ErrNoActionsCaptured)
	}
}

func TestGetActionsInTimeWindow_NoneInWindow(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Timestamp: 1000},
		{Type: "input", Timestamp: 2000},
	})

	_, err := h.getActionsInTimeWindow(10000, 500)
	if err == nil {
		t.Fatal("getActionsInTimeWindow should return error when no actions in window")
	}
	if !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %q, want to contain %q", err.Error(), ErrNoActionsCaptured)
	}
}

func TestGetActionsInTimeWindow_AllInWindow(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Timestamp: 900},
		{Type: "input", Timestamp: 1000},
		{Type: "navigate", Timestamp: 1100},
	})

	result, err := h.getActionsInTimeWindow(1000, 500)
	if err != nil {
		t.Fatalf("getActionsInTimeWindow error = %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("got %d actions, want 3", len(result))
	}
}

func TestGetActionsInTimeWindow_PartialMatch(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Timestamp: 100},     // outside window (too early)
		{Type: "input", Timestamp: 900},     // inside window (diff = -100)
		{Type: "navigate", Timestamp: 1000}, // inside window (diff = 0)
		{Type: "scroll", Timestamp: 1400},   // inside window (diff = 400)
		{Type: "wait", Timestamp: 2000},     // outside window (diff = 1000)
	})

	result, err := h.getActionsInTimeWindow(1000, 500)
	if err != nil {
		t.Fatalf("getActionsInTimeWindow error = %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("got %d actions, want 3 (input, navigate, scroll)", len(result))
	}
	if result[0].Type != "input" {
		t.Fatalf("result[0].Type = %q, want input", result[0].Type)
	}
	if result[1].Type != "navigate" {
		t.Fatalf("result[1].Type = %q, want navigate", result[1].Type)
	}
	if result[2].Type != "scroll" {
		t.Fatalf("result[2].Type = %q, want scroll", result[2].Type)
	}
}

func TestGetActionsInTimeWindow_ExactBoundary(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Actions exactly at window boundaries should be included
	h.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Timestamp: 500},  // center - window = exactly at lower bound
		{Type: "input", Timestamp: 1500}, // center + window = exactly at upper bound
	})

	result, err := h.getActionsInTimeWindow(1000, 500)
	if err != nil {
		t.Fatalf("getActionsInTimeWindow error = %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d actions, want 2 (both at exact boundary)", len(result))
	}
}

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

	result, err := h.getActionsInTimeWindow(1000, 500)
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
// Tests for deriveInteractionTestName
// ============================================

func TestDeriveInteractionTestName_EmptyActions(t *testing.T) {
	t.Parallel()

	name := deriveInteractionTestName(nil)
	if name != "user-interaction" {
		t.Fatalf("deriveInteractionTestName(nil) = %q, want user-interaction", name)
	}

	name = deriveInteractionTestName([]capture.EnhancedAction{})
	if name != "user-interaction" {
		t.Fatalf("deriveInteractionTestName([]) = %q, want user-interaction", name)
	}
}

func TestDeriveInteractionTestName_WithURL(t *testing.T) {
	t.Parallel()

	actions := []capture.EnhancedAction{
		{Type: "click", URL: "https://app.example.com/login"},
	}
	name := deriveInteractionTestName(actions)
	if name != "https://app.example.com/login" {
		t.Fatalf("deriveInteractionTestName(URL) = %q, want https://app.example.com/login", name)
	}
}

func TestDeriveInteractionTestName_WithTypeNoURL(t *testing.T) {
	t.Parallel()

	actions := []capture.EnhancedAction{
		{Type: "click", URL: ""},
	}
	name := deriveInteractionTestName(actions)
	if name != "click-flow" {
		t.Fatalf("deriveInteractionTestName(Type) = %q, want click-flow", name)
	}
}

func TestDeriveInteractionTestName_NoURLNoType(t *testing.T) {
	t.Parallel()

	actions := []capture.EnhancedAction{
		{URL: "", Type: ""},
	}
	name := deriveInteractionTestName(actions)
	if name != "user-interaction" {
		t.Fatalf("deriveInteractionTestName(empty) = %q, want user-interaction", name)
	}
}

func TestDeriveInteractionTestName_URLTakesPrecedence(t *testing.T) {
	t.Parallel()

	actions := []capture.EnhancedAction{
		{Type: "navigate", URL: "https://example.com"},
	}
	name := deriveInteractionTestName(actions)
	if name != "https://example.com" {
		t.Fatalf("URL should take precedence over type; got %q", name)
	}
}

// ============================================
// Tests for countNetworkAssertions
// ============================================

func TestCountNetworkAssertions_Empty(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	count := h.countNetworkAssertions()
	if count != 0 {
		t.Fatalf("countNetworkAssertions(empty) = %d, want 0", count)
	}
}

func TestCountNetworkAssertions_WithBodies(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddNetworkBodiesForTest([]capture.NetworkBody{
		{Method: "GET", URL: "/api/users", Status: 200},
		{Method: "POST", URL: "/api/login", Status: 201},
		{Method: "GET", URL: "/api/data", Status: 0}, // status 0 should not count
	})

	count := h.countNetworkAssertions()
	if count != 2 {
		t.Fatalf("countNetworkAssertions = %d, want 2 (skip status 0)", count)
	}
}

func TestCountNetworkAssertions_AllZeroStatus(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddNetworkBodiesForTest([]capture.NetworkBody{
		{Method: "GET", URL: "/api/a", Status: 0},
		{Method: "GET", URL: "/api/b", Status: 0},
	})

	count := h.countNetworkAssertions()
	if count != 0 {
		t.Fatalf("countNetworkAssertions(all zero) = %d, want 0", count)
	}
}

func TestCountNetworkAssertions_NegativeStatusIgnored(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.AddNetworkBodiesForTest([]capture.NetworkBody{
		{Method: "GET", URL: "/api/a", Status: -1},
	})

	count := h.countNetworkAssertions()
	if count != 0 {
		t.Fatalf("countNetworkAssertions(negative) = %d, want 0", count)
	}
}

// ============================================
// Tests for collectErrorMessages
// ============================================

func TestCollectErrorMessages_NoEntries(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	msgs := h.collectErrorMessages(5)
	if len(msgs) != 0 {
		t.Fatalf("collectErrorMessages(empty) len = %d, want 0", len(msgs))
	}
}

func TestCollectErrorMessages_NoErrors(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "info", "message": "all good"},
		{"level": "warn", "message": "minor issue"},
	}
	h.server.mu.Unlock()

	msgs := h.collectErrorMessages(5)
	if len(msgs) != 0 {
		t.Fatalf("collectErrorMessages(no errors) len = %d, want 0", len(msgs))
	}
}

func TestCollectErrorMessages_MixedLevels(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "error one"},
		{"level": "info", "message": "info msg"},
		{"level": "error", "message": "error two"},
		{"level": "error", "message": "error three"},
		{"level": "warn", "message": "warning"},
	}
	h.server.mu.Unlock()

	msgs := h.collectErrorMessages(5)
	if len(msgs) != 3 {
		t.Fatalf("collectErrorMessages len = %d, want 3", len(msgs))
	}
	if msgs[0] != "error one" {
		t.Fatalf("msgs[0] = %q, want error one", msgs[0])
	}
	if msgs[1] != "error two" {
		t.Fatalf("msgs[1] = %q, want error two", msgs[1])
	}
	if msgs[2] != "error three" {
		t.Fatalf("msgs[2] = %q, want error three", msgs[2])
	}
}

func TestCollectErrorMessages_RespectsLimit(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "err-1"},
		{"level": "error", "message": "err-2"},
		{"level": "error", "message": "err-3"},
		{"level": "error", "message": "err-4"},
		{"level": "error", "message": "err-5"},
	}
	h.server.mu.Unlock()

	msgs := h.collectErrorMessages(3)
	if len(msgs) != 3 {
		t.Fatalf("collectErrorMessages(limit=3) len = %d, want 3", len(msgs))
	}
	if msgs[2] != "err-3" {
		t.Fatalf("msgs[2] = %q, want err-3", msgs[2])
	}
}

func TestCollectErrorMessages_SkipsEmptyMessages(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": ""},
		{"level": "error", "message": "real error"},
		{"level": "error"},
	}
	h.server.mu.Unlock()

	msgs := h.collectErrorMessages(5)
	if len(msgs) != 1 {
		t.Fatalf("collectErrorMessages len = %d, want 1 (skip empty msgs)", len(msgs))
	}
	if msgs[0] != "real error" {
		t.Fatalf("msgs[0] = %q, want real error", msgs[0])
	}
}

func TestCollectErrorMessages_LimitZero(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.server.mu.Lock()
	h.server.entries = []LogEntry{
		{"level": "error", "message": "should not appear"},
	}
	h.server.mu.Unlock()

	msgs := h.collectErrorMessages(0)
	if len(msgs) != 0 {
		t.Fatalf("collectErrorMessages(limit=0) len = %d, want 0", len(msgs))
	}
}

// ============================================
// Tests for buildRegressionAssertions
// ============================================

func TestBuildRegressionAssertions_NoErrorsNoNetwork(t *testing.T) {
	t.Parallel()

	assertions, count := buildRegressionAssertions(nil, nil)

	if count != 1 {
		t.Fatalf("assertionCount = %d, want 1 (clean baseline assertion)", count)
	}
	joined := strings.Join(assertions, "\n")
	if !strings.Contains(joined, "Assert no console errors") {
		t.Fatal("expected clean baseline assertion comment")
	}
	if !strings.Contains(joined, "expect(consoleErrors).toHaveLength(0)") {
		t.Fatal("expected consoleErrors assertion")
	}
	if !strings.Contains(joined, "TODO: Add performance assertions") {
		t.Fatal("expected performance TODO comment")
	}
}

func TestBuildRegressionAssertions_WithErrors(t *testing.T) {
	t.Parallel()

	errors := []string{"TypeError: undefined", "ReferenceError: foo"}
	assertions, count := buildRegressionAssertions(errors, nil)

	if count != 0 {
		t.Fatalf("assertionCount = %d, want 0 (baseline had errors, no clean assertion)", count)
	}
	joined := strings.Join(assertions, "\n")
	if !strings.Contains(joined, "Baseline had 2 console errors") {
		t.Fatalf("expected baseline error count comment; got:\n%s", joined)
	}
	if !strings.Contains(joined, "TODO: Add assertions to verify errors haven't changed") {
		t.Fatal("expected TODO for error verification")
	}
}

func TestBuildRegressionAssertions_WithNetworkBodies(t *testing.T) {
	t.Parallel()

	bodies := []capture.NetworkBody{
		{Method: "GET", URL: "/api/users", Status: 200},
		{Method: "POST", URL: "/api/login", Status: 201},
		{Method: "PUT", URL: "/api/update", Status: 204},
	}
	assertions, count := buildRegressionAssertions(nil, bodies)

	// 1 for clean baseline + 3 network assertions
	if count != 4 {
		t.Fatalf("assertionCount = %d, want 4", count)
	}
	joined := strings.Join(assertions, "\n")
	if !strings.Contains(joined, "Assert GET /api/users returns 200") {
		t.Fatal("expected network assertion for GET /api/users")
	}
	if !strings.Contains(joined, "Assert POST /api/login returns 201") {
		t.Fatal("expected network assertion for POST /api/login")
	}
	if !strings.Contains(joined, "Assert PUT /api/update returns 204") {
		t.Fatal("expected network assertion for PUT /api/update")
	}
}

func TestBuildRegressionAssertions_NetworkLimitedToThree(t *testing.T) {
	t.Parallel()

	bodies := []capture.NetworkBody{
		{Method: "GET", URL: "/api/1", Status: 200},
		{Method: "GET", URL: "/api/2", Status: 200},
		{Method: "GET", URL: "/api/3", Status: 200},
		{Method: "GET", URL: "/api/4", Status: 200}, // should be excluded
		{Method: "GET", URL: "/api/5", Status: 200}, // should be excluded
	}
	assertions, count := buildRegressionAssertions(nil, bodies)

	// 1 for clean baseline + 3 network (max 3)
	if count != 4 {
		t.Fatalf("assertionCount = %d, want 4 (max 3 network)", count)
	}
	joined := strings.Join(assertions, "\n")
	if strings.Contains(joined, "/api/4") {
		t.Fatal("should not include 4th network body")
	}
	if strings.Contains(joined, "/api/5") {
		t.Fatal("should not include 5th network body")
	}
}

func TestBuildRegressionAssertions_SkipsZeroStatusNetworkBodies(t *testing.T) {
	t.Parallel()

	bodies := []capture.NetworkBody{
		{Method: "GET", URL: "/api/ok", Status: 200},
		{Method: "GET", URL: "/api/zero", Status: 0},
	}
	assertions, count := buildRegressionAssertions(nil, bodies)

	// 1 for clean baseline + 1 network (status 0 skipped)
	if count != 2 {
		t.Fatalf("assertionCount = %d, want 2 (skip status 0)", count)
	}
	joined := strings.Join(assertions, "\n")
	if strings.Contains(joined, "/api/zero") {
		t.Fatal("should not include network body with status 0")
	}
}

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

func TestInsertAssertionsBeforeClose_Normal(t *testing.T) {
	t.Parallel()

	script := "test('example', async ({ page }) => {\n  await page.click('#btn');\n});\n"
	assertions := []string{"  expect(page).toBeTruthy();", "  expect(errors).toHaveLength(0);"}

	result := insertAssertionsBeforeClose(script, assertions)

	if !strings.Contains(result, "expect(page).toBeTruthy();") {
		t.Fatal("assertion not inserted")
	}
	if !strings.Contains(result, "expect(errors).toHaveLength(0);") {
		t.Fatal("second assertion not inserted")
	}
	assertionIdx := strings.Index(result, "expect(page)")
	closeIdx := strings.LastIndex(result, "});")
	if assertionIdx > closeIdx {
		t.Fatal("assertions should be before closing });")
	}
}

func TestInsertAssertionsBeforeClose_NoClosingBrace(t *testing.T) {
	t.Parallel()

	script := "incomplete script without closing"
	assertions := []string{"  expect(1).toBe(1);"}

	result := insertAssertionsBeforeClose(script, assertions)

	if result != script {
		t.Fatalf("expected unchanged script when no }); found; got:\n%s", result)
	}
}

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
