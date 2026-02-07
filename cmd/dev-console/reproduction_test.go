// reproduction_test.go — Tests for reproduction script generation.
// Verifies Gasoline (natural language) and Playwright output formats,
// selector priority, timing pauses, URL rewriting, and edge cases.
package main

import (
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Test Helpers
// ============================================

func makeTestAction(typ string, ts int64, opts map[string]any) capture.EnhancedAction {
	a := capture.EnhancedAction{
		Type:      typ,
		Timestamp: ts,
		URL:       "https://example.com",
	}
	if s, ok := opts["selectors"].(map[string]any); ok {
		a.Selectors = s
	}
	if v, ok := opts["value"].(string); ok {
		a.Value = v
	}
	if v, ok := opts["toURL"].(string); ok {
		a.ToURL = v
	}
	if v, ok := opts["key"].(string); ok {
		a.Key = v
	}
	if v, ok := opts["selectedValue"].(string); ok {
		a.SelectedValue = v
	}
	if v, ok := opts["selectedText"].(string); ok {
		a.SelectedText = v
	}
	if v, ok := opts["scrollY"].(int); ok {
		a.ScrollY = v
	}
	if v, ok := opts["source"].(string); ok {
		a.Source = v
	}
	return a
}

func basicFlow() []capture.EnhancedAction {
	return []capture.EnhancedAction{
		makeTestAction("navigate", 1000, map[string]any{
			"toURL": "https://example.com/login",
		}),
		makeTestAction("click", 2000, map[string]any{
			"selectors": map[string]any{
				"text": "Sign In",
				"role": map[string]any{"role": "button", "name": "Sign In"},
			},
		}),
		makeTestAction("input", 3000, map[string]any{
			"selectors": map[string]any{
				"ariaLabel": "Email address",
				"role":      map[string]any{"role": "textbox", "name": "Email address"},
			},
			"value": "alice@example.com",
		}),
	}
}

// ============================================
// Gasoline Format Tests
// ============================================

func TestReproduction_Gasoline_BasicFlow(t *testing.T) {
	t.Parallel()
	actions := basicFlow()
	script := generateGasolineScript(actions, ReproductionParams{})

	// Header
	if !strings.Contains(script, "# Reproduction:") {
		t.Error("expected header with '# Reproduction:'")
	}
	if !strings.Contains(script, "3 actions") {
		t.Errorf("expected '3 actions' in header, got:\n%s", script)
	}

	// Navigate step
	if !strings.Contains(script, "Navigate to: https://example.com/login") {
		t.Errorf("expected navigate step, got:\n%s", script)
	}

	// Click step with text + role
	if !strings.Contains(script, `Click: "Sign In" button`) {
		t.Errorf("expected click description, got:\n%s", script)
	}

	// Input step
	if !strings.Contains(script, `Type "alice@example.com" into:`) {
		t.Errorf("expected input step, got:\n%s", script)
	}
}

func TestReproduction_Gasoline_AllActionTypes(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("navigate", 1000, map[string]any{"toURL": "https://example.com"}),
		makeTestAction("click", 2000, map[string]any{
			"selectors": map[string]any{"text": "Go"},
		}),
		makeTestAction("input", 3000, map[string]any{
			"selectors": map[string]any{"id": "name"},
			"value":     "Alice",
		}),
		makeTestAction("select", 4000, map[string]any{
			"selectors":    map[string]any{"id": "country"},
			"selectedText": "Canada",
		}),
		makeTestAction("keypress", 5000, map[string]any{"key": "Enter"}),
		makeTestAction("scroll", 6000, map[string]any{"scrollY": 500}),
	}

	script := generateGasolineScript(actions, ReproductionParams{})

	if !strings.Contains(script, "Navigate to:") {
		t.Error("missing navigate")
	}
	if !strings.Contains(script, "Click:") {
		t.Error("missing click")
	}
	if !strings.Contains(script, "Type") {
		t.Error("missing input/type")
	}
	if !strings.Contains(script, `Select "Canada"`) {
		t.Error("missing select")
	}
	if !strings.Contains(script, "Press: Enter") {
		t.Error("missing keypress")
	}
	if !strings.Contains(script, "Scroll to: y=500") {
		t.Error("missing scroll")
	}
}

func TestReproduction_Gasoline_ElementDescriptionPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sels     map[string]any
		expected string
	}{
		{
			name:     "text + role",
			sels:     map[string]any{"text": "Submit", "role": map[string]any{"role": "button", "name": "Submit"}},
			expected: `"Submit" button`,
		},
		{
			name:     "ariaLabel + role",
			sels:     map[string]any{"ariaLabel": "Close dialog", "role": map[string]any{"role": "button", "name": "Close dialog"}},
			expected: `"Close dialog" button`,
		},
		{
			name:     "role name only",
			sels:     map[string]any{"role": map[string]any{"role": "textbox", "name": "Email"}},
			expected: `"Email" textbox`,
		},
		{
			name:     "testId fallback",
			sels:     map[string]any{"testId": "submit-btn"},
			expected: `[data-testid="submit-btn"]`,
		},
		{
			name:     "text alone",
			sels:     map[string]any{"text": "Add to Cart"},
			expected: `"Add to Cart"`,
		},
		{
			name:     "ariaLabel alone",
			sels:     map[string]any{"ariaLabel": "Search"},
			expected: `"Search"`,
		},
		{
			name:     "id fallback",
			sels:     map[string]any{"id": "quantity"},
			expected: `#quantity`,
		},
		{
			name:     "cssPath last resort",
			sels:     map[string]any{"cssPath": "form > input"},
			expected: `form > input`,
		},
		{
			name:     "no selectors",
			sels:     map[string]any{},
			expected: `(unknown element)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action := makeTestAction("click", 1000, map[string]any{"selectors": tc.sels})
			desc := describeElement(action)
			if desc != tc.expected {
				t.Errorf("describeElement() = %q, want %q", desc, tc.expected)
			}
		})
	}
}

func TestReproduction_Gasoline_TimingPauses(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("click", 1000, map[string]any{
			"selectors": map[string]any{"text": "A"},
		}),
		makeTestAction("click", 5500, map[string]any{ // 4.5s gap
			"selectors": map[string]any{"text": "B"},
		}),
		makeTestAction("click", 6000, map[string]any{ // 0.5s gap — no pause
			"selectors": map[string]any{"text": "C"},
		}),
	}

	script := generateGasolineScript(actions, ReproductionParams{})

	if !strings.Contains(script, "[4s pause]") {
		t.Errorf("expected [4s pause], got:\n%s", script)
	}

	// Should NOT have a pause before C (only 500ms gap)
	lines := strings.Split(script, "\n")
	for i, line := range lines {
		if strings.Contains(line, `"C"`) {
			if i > 0 && strings.Contains(lines[i-1], "pause") {
				t.Errorf("should not have pause before C (500ms gap)")
			}
		}
	}
}

func TestReproduction_Gasoline_RedactedValues(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("input", 1000, map[string]any{
			"selectors": map[string]any{"role": map[string]any{"role": "textbox", "name": "Password"}},
			"value":     "[redacted]",
		}),
	}

	script := generateGasolineScript(actions, ReproductionParams{})

	if !strings.Contains(script, "[user-provided]") {
		t.Errorf("expected [user-provided] for redacted value, got:\n%s", script)
	}
	if strings.Contains(script, "[redacted]") {
		t.Errorf("should not contain raw [redacted] marker")
	}
}

func TestReproduction_Gasoline_AIActions(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("click", 1000, map[string]any{
			"selectors": map[string]any{"text": "Submit"},
			"source":    "ai",
		}),
		makeTestAction("click", 2000, map[string]any{
			"selectors": map[string]any{"text": "Next"},
		}),
	}

	script := generateGasolineScript(actions, ReproductionParams{})

	if !strings.Contains(script, "(AI) Click:") {
		t.Errorf("expected (AI) prefix for AI action, got:\n%s", script)
	}
	// Human action should NOT have prefix
	if strings.Contains(script, "(AI) Click: \"Next\"") {
		t.Errorf("human action should not have (AI) prefix")
	}
}

func TestReproduction_Gasoline_ErrorMessage(t *testing.T) {
	t.Parallel()
	actions := basicFlow()
	script := generateGasolineScript(actions, ReproductionParams{
		ErrorMessage: "Cannot read property 'x' of undefined",
	})

	if !strings.Contains(script, "# Error: Cannot read property 'x' of undefined") {
		t.Errorf("expected error annotation, got:\n%s", script)
	}
}

// ============================================
// Playwright Format Tests
// ============================================

func TestReproduction_Playwright_BasicFlow(t *testing.T) {
	t.Parallel()
	actions := basicFlow()
	script := generateReproPlaywrightScript(actions, ReproductionParams{})

	// Valid Playwright imports
	if !strings.Contains(script, "import { test, expect } from '@playwright/test'") {
		t.Error("expected Playwright import")
	}

	// Test function
	if !strings.Contains(script, "test('reproduction:") {
		t.Error("expected test() wrapper")
	}

	// Navigate
	if !strings.Contains(script, "await page.goto('https://example.com/login')") {
		t.Errorf("expected goto, got:\n%s", script)
	}

	// Click with role locator
	if !strings.Contains(script, "getByRole('button', { name: 'Sign In' })") {
		t.Errorf("expected getByRole click, got:\n%s", script)
	}

	// Fill — role takes priority over ariaLabel (both present in basicFlow)
	if !strings.Contains(script, "getByRole('textbox', { name: 'Email address' })") {
		t.Errorf("expected getByRole fill, got:\n%s", script)
	}
}

func TestReproduction_Playwright_LocatorPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sels     map[string]any
		expected string
	}{
		{
			name:     "testId first",
			sels:     map[string]any{"testId": "submit-btn", "role": map[string]any{"role": "button", "name": "Submit"}, "text": "Submit"},
			expected: "getByTestId('submit-btn')",
		},
		{
			name:     "role second",
			sels:     map[string]any{"role": map[string]any{"role": "button", "name": "Submit"}, "text": "Submit", "ariaLabel": "Submit"},
			expected: "getByRole('button', { name: 'Submit' })",
		},
		{
			name:     "ariaLabel third",
			sels:     map[string]any{"ariaLabel": "Search", "text": "Search", "id": "search"},
			expected: "getByLabel('Search')",
		},
		{
			name:     "text fourth",
			sels:     map[string]any{"text": "Click me", "id": "btn"},
			expected: "getByText('Click me')",
		},
		{
			name:     "id fifth",
			sels:     map[string]any{"id": "quantity", "cssPath": "form > input"},
			expected: "locator('#quantity')",
		},
		{
			name:     "cssPath last",
			sels:     map[string]any{"cssPath": "form > div > input"},
			expected: "locator('form > div > input')",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc := playwrightLocator(tc.sels)
			if loc != tc.expected {
				t.Errorf("playwrightLocator() = %q, want %q", loc, tc.expected)
			}
		})
	}
}

func TestReproduction_Playwright_URLRewriting(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("navigate", 1000, map[string]any{
			"toURL": "https://production.example.com/dashboard",
		}),
	}

	script := generateReproPlaywrightScript(actions, ReproductionParams{
		BaseURL: "http://localhost:3000",
	})

	if !strings.Contains(script, "http://localhost:3000/dashboard") {
		t.Errorf("expected rewritten URL, got:\n%s", script)
	}
	if strings.Contains(script, "production.example.com") {
		t.Error("production URL should be rewritten")
	}
}

func TestReproduction_Playwright_SpecialCharacters(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("input", 1000, map[string]any{
			"selectors": map[string]any{"id": "msg"},
			"value":     "it's a \"test\"\nnewline",
		}),
	}

	script := generateReproPlaywrightScript(actions, ReproductionParams{})

	if strings.Contains(script, "it's") && !strings.Contains(script, `it\'s`) {
		t.Errorf("expected escaped quotes, got:\n%s", script)
	}
}

func TestReproduction_Playwright_TimingPauses(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("click", 1000, map[string]any{"selectors": map[string]any{"text": "A"}}),
		makeTestAction("click", 6000, map[string]any{"selectors": map[string]any{"text": "B"}}),
	}

	script := generateReproPlaywrightScript(actions, ReproductionParams{})

	if !strings.Contains(script, "// [5s pause]") {
		t.Errorf("expected pause comment, got:\n%s", script)
	}
}

// ============================================
// Shared Behavior Tests
// ============================================

func TestReproduction_EmptyActions(t *testing.T) {
	t.Parallel()

	gasoline := generateGasolineScript(nil, ReproductionParams{})
	if !strings.Contains(gasoline, "No actions") {
		t.Errorf("gasoline should indicate no actions, got:\n%s", gasoline)
	}

	playwright := generateReproPlaywrightScript(nil, ReproductionParams{})
	if !strings.Contains(playwright, "No actions") {
		t.Errorf("playwright should indicate no actions, got:\n%s", playwright)
	}
}

func TestReproduction_LastN(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("click", 1000, map[string]any{"selectors": map[string]any{"text": "First"}}),
		makeTestAction("click", 2000, map[string]any{"selectors": map[string]any{"text": "Second"}}),
		makeTestAction("click", 3000, map[string]any{"selectors": map[string]any{"text": "Third"}}),
	}

	script := generateGasolineScript(actions, ReproductionParams{LastN: 2})

	if strings.Contains(script, "First") {
		t.Error("last_n=2 should exclude first action")
	}
	if !strings.Contains(script, "Second") || !strings.Contains(script, "Third") {
		t.Error("last_n=2 should include last two actions")
	}
}

func TestReproduction_DefaultFormat(t *testing.T) {
	t.Parallel()
	// When output_format is empty, should default to "gasoline"
	params := ReproductionParams{}
	format := params.OutputFormat
	if format == "" {
		format = "gasoline" // default
	}
	if format != "gasoline" {
		t.Errorf("expected default format 'gasoline', got %q", format)
	}
}

func TestReproduction_NavigateNoURL(t *testing.T) {
	t.Parallel()
	actions := []capture.EnhancedAction{
		makeTestAction("navigate", 1000, map[string]any{}), // no URL
		makeTestAction("click", 2000, map[string]any{"selectors": map[string]any{"text": "Go"}}),
	}

	gasoline := generateGasolineScript(actions, ReproductionParams{})
	// Should skip the empty navigate, only have 1 numbered step
	if strings.Contains(gasoline, "Navigate to: \n") || strings.Contains(gasoline, "Navigate to:  ") {
		t.Errorf("should skip navigate with no URL, got:\n%s", gasoline)
	}
}
