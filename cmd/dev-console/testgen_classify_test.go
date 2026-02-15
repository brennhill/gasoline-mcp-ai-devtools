// testgen_classify_test.go — Unit tests for test failure classification.
package main

import (
	"strings"
	"testing"
)

func TestMatchClassificationPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		errorMsg   string
		wantCat    string
		minConf    float64
	}{
		{
			name:     "selector timeout with selector",
			errorMsg: `Timeout waiting for selector ".btn-submit"`,
			wantCat:  CategorySelectorBroken,
			minConf:  0.9,
		},
		{
			name:     "selector timeout without selector",
			errorMsg: "Timeout waiting for selector to appear",
			wantCat:  CategoryTimingFlaky,
			minConf:  0.8,
		},
		{
			name:     "network error",
			errorMsg: "net::ERR_CONNECTION_REFUSED at http://localhost:3000",
			wantCat:  CategoryNetworkFlaky,
			minConf:  0.85,
		},
		{
			name:     "network keyword",
			errorMsg: "Network request failed",
			wantCat:  CategoryNetworkFlaky,
			minConf:  0.85,
		},
		{
			name:     "assertion failure toBe",
			errorMsg: `Expected "hello" toBe "world"`,
			wantCat:  CategoryRealBug,
			minConf:  0.7,
		},
		{
			name:     "assertion failure to be",
			errorMsg: "Expected 5 to be 10",
			wantCat:  CategoryRealBug,
			minConf:  0.7,
		},
		{
			name:     "assertion failure toEqual",
			errorMsg: "Expected object toEqual {foo: bar}",
			wantCat:  CategoryRealBug,
			minConf:  0.7,
		},
		{
			name:     "element detached",
			errorMsg: "Element is not attached to DOM",
			wantCat:  CategoryTimingFlaky,
			minConf:  0.8,
		},
		{
			name:     "element not attached",
			errorMsg: "not attached to DOM after interaction",
			wantCat:  CategoryTimingFlaky,
			minConf:  0.8,
		},
		{
			name:     "element outside viewport",
			errorMsg: "Element is outside viewport",
			wantCat:  CategoryTestBug,
			minConf:  0.75,
		},
		{
			name:     "element not visible",
			errorMsg: "Element is not visible",
			wantCat:  CategoryTestBug,
			minConf:  0.75,
		},
		{
			name:     "unknown pattern",
			errorMsg: "some random error we don't recognize",
			wantCat:  CategoryUnknown,
			minConf:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, conf, evidence := matchClassificationPattern(tt.errorMsg)
			if cat != tt.wantCat {
				t.Errorf("category = %q, want %q", cat, tt.wantCat)
			}
			if conf < tt.minConf {
				t.Errorf("confidence = %.2f, want >= %.2f", conf, tt.minConf)
			}
			if len(evidence) == 0 {
				t.Error("expected non-empty evidence")
			}
		})
	}
}

func TestClassifyFailure(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}

	t.Run("selector broken", func(t *testing.T) {
		f := &TestFailure{
			TestName: "login test",
			Error:    `Timeout waiting for selector "#login-btn"`,
		}
		c := h.classifyFailure(f)
		if c.Category != CategorySelectorBroken {
			t.Fatalf("category = %q, want %q", c.Category, CategorySelectorBroken)
		}
		if c.RecommendedAction != "heal" {
			t.Fatalf("action = %q, want heal", c.RecommendedAction)
		}
		if c.SuggestedFix == nil || c.SuggestedFix.Type != "selector_update" {
			t.Fatalf("expected selector_update fix, got %+v", c.SuggestedFix)
		}
	})

	t.Run("timing flaky", func(t *testing.T) {
		f := &TestFailure{Error: "Element is not attached to DOM"}
		c := h.classifyFailure(f)
		if c.Category != CategoryTimingFlaky {
			t.Fatalf("category = %q, want %q", c.Category, CategoryTimingFlaky)
		}
		if c.RecommendedAction != "add_wait" {
			t.Fatalf("action = %q, want add_wait", c.RecommendedAction)
		}
		if !c.IsFlaky {
			t.Fatal("expected IsFlaky=true")
		}
	})

	t.Run("network flaky", func(t *testing.T) {
		f := &TestFailure{Error: "net::ERR_CONNECTION_REFUSED"}
		c := h.classifyFailure(f)
		if c.Category != CategoryNetworkFlaky {
			t.Fatalf("category = %q, want %q", c.Category, CategoryNetworkFlaky)
		}
		if c.RecommendedAction != "mock_network" {
			t.Fatalf("action = %q, want mock_network", c.RecommendedAction)
		}
		if !c.IsEnvironment {
			t.Fatal("expected IsEnvironment=true")
		}
	})

	t.Run("real bug", func(t *testing.T) {
		f := &TestFailure{Error: `Expected "hello" to be "world"`}
		c := h.classifyFailure(f)
		if c.Category != CategoryRealBug {
			t.Fatalf("category = %q, want %q", c.Category, CategoryRealBug)
		}
		if c.RecommendedAction != "fix_bug" {
			t.Fatalf("action = %q, want fix_bug", c.RecommendedAction)
		}
		if !c.IsRealBug {
			t.Fatal("expected IsRealBug=true")
		}
	})

	t.Run("test bug viewport", func(t *testing.T) {
		f := &TestFailure{Error: "Element is outside viewport"}
		c := h.classifyFailure(f)
		if c.Category != CategoryTestBug {
			t.Fatalf("category = %q, want %q", c.Category, CategoryTestBug)
		}
		if c.RecommendedAction != "fix_test" {
			t.Fatalf("action = %q, want fix_test", c.RecommendedAction)
		}
		if c.SuggestedFix == nil || c.SuggestedFix.Type != "scroll_to_element" {
			t.Fatalf("expected scroll_to_element fix, got %+v", c.SuggestedFix)
		}
	})

	t.Run("unknown gets manual_review", func(t *testing.T) {
		f := &TestFailure{Error: "weird error nobody recognizes"}
		c := h.classifyFailure(f)
		if c.Category != CategoryUnknown {
			t.Fatalf("category = %q, want %q", c.Category, CategoryUnknown)
		}
		if c.RecommendedAction != "manual_review" {
			t.Fatalf("action = %q, want manual_review", c.RecommendedAction)
		}
	})
}

func TestClassifyFailureBatch(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}

	failures := []TestFailure{
		{TestName: "t1", Error: `Timeout waiting for selector "#btn"`},
		{TestName: "t2", Error: "net::ERR_CONNECTION_REFUSED"},
		{TestName: "t3", Error: `Expected "a" to be "b"`},
		{TestName: "t4", Error: "Element is outside viewport"},
		{TestName: "t5", Error: "unknown error xyz"},
	}

	result := h.classifyFailureBatch(failures)

	if result.TotalClassified != 5 {
		t.Fatalf("TotalClassified = %d, want 5", result.TotalClassified)
	}
	if result.RealBugs != 1 {
		t.Fatalf("RealBugs = %d, want 1", result.RealBugs)
	}
	if result.FlakyTests != 1 {
		t.Fatalf("FlakyTests = %d, want 1 (network flaky)", result.FlakyTests)
	}
	if result.TestBugs != 1 {
		t.Fatalf("TestBugs = %d, want 1", result.TestBugs)
	}
	if result.Uncertain != 1 {
		t.Fatalf("Uncertain = %d, want 1 (unknown < 0.5)", result.Uncertain)
	}
	if len(result.Summary) == 0 {
		t.Fatal("expected non-empty Summary map")
	}
}

func TestGenerateSuggestedFix(t *testing.T) {
	t.Parallel()

	t.Run("selector broken with selector", func(t *testing.T) {
		fix := generateSuggestedFix(CategorySelectorBroken, `Timeout waiting for selector "#btn"`)
		if fix == nil || fix.Type != "selector_update" {
			t.Fatalf("expected selector_update, got %+v", fix)
		}
		if fix.Old != "#btn" {
			t.Fatalf("expected Old=#btn, got %q", fix.Old)
		}
	})

	t.Run("selector broken without selector", func(t *testing.T) {
		fix := generateSuggestedFix(CategorySelectorBroken, "generic selector error")
		if fix == nil || fix.Type != "selector_update" {
			t.Fatalf("expected selector_update, got %+v", fix)
		}
	})

	t.Run("timing flaky", func(t *testing.T) {
		fix := generateSuggestedFix(CategoryTimingFlaky, "timeout")
		if fix == nil || fix.Type != "add_wait" {
			t.Fatalf("expected add_wait, got %+v", fix)
		}
		if !strings.Contains(fix.Code, "waitForSelector") {
			t.Fatalf("expected waitForSelector in code, got %q", fix.Code)
		}
	})

	t.Run("network flaky", func(t *testing.T) {
		fix := generateSuggestedFix(CategoryNetworkFlaky, "net::ERR")
		if fix == nil || fix.Type != "mock_network" {
			t.Fatalf("expected mock_network, got %+v", fix)
		}
	})

	t.Run("test bug viewport", func(t *testing.T) {
		fix := generateSuggestedFix(CategoryTestBug, "outside viewport")
		if fix == nil || fix.Type != "scroll_to_element" {
			t.Fatalf("expected scroll_to_element, got %+v", fix)
		}
	})

	t.Run("test bug non-viewport", func(t *testing.T) {
		fix := generateSuggestedFix(CategoryTestBug, "something else")
		if fix != nil {
			t.Fatalf("expected nil fix for non-viewport test bug, got %+v", fix)
		}
	})

	t.Run("unknown returns nil", func(t *testing.T) {
		fix := generateSuggestedFix(CategoryUnknown, "anything")
		if fix != nil {
			t.Fatalf("expected nil for unknown, got %+v", fix)
		}
	})

	t.Run("real bug returns nil", func(t *testing.T) {
		fix := generateSuggestedFix(CategoryRealBug, "assertion failed")
		if fix != nil {
			t.Fatalf("expected nil for real bug, got %+v", fix)
		}
	})
}
