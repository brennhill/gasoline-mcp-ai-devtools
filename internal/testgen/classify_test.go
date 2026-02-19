// classify_test.go — Tests for test failure classification.
package testgen

import (
	"strings"
	"testing"
)

func TestMatchClassificationPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errMsg  string
		wantCat string
		minConf float64
	}{
		{"selector timeout with selector", `Timeout waiting for selector ".btn-submit"`, CategorySelectorBroken, 0.9},
		{"selector timeout without selector", "Timeout waiting for selector to appear", CategoryTimingFlaky, 0.8},
		{"network error", "net::ERR_CONNECTION_REFUSED at http://localhost:3000", CategoryNetworkFlaky, 0.85},
		{"network keyword", "Network request failed", CategoryNetworkFlaky, 0.85},
		{"assertion failure toBe", `Expected "hello" toBe "world"`, CategoryRealBug, 0.7},
		{"assertion failure to be", "Expected 5 to be 10", CategoryRealBug, 0.7},
		{"assertion failure toEqual", "Expected object toEqual {foo: bar}", CategoryRealBug, 0.7},
		{"element detached", "Element is not attached to DOM", CategoryTimingFlaky, 0.8},
		{"element not attached", "not attached to DOM after interaction", CategoryTimingFlaky, 0.8},
		{"element outside viewport", "Element is outside viewport", CategoryTestBug, 0.75},
		{"element not visible", "Element is not visible", CategoryTestBug, 0.75},
		{"unknown pattern", "some random error we don't recognize", CategoryUnknown, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, conf, evidence := MatchClassificationPattern(tt.errMsg)
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

	t.Run("selector broken", func(t *testing.T) {
		c := ClassifyFailure(&TestFailure{TestName: "login test", Error: `Timeout waiting for selector "#login-btn"`})
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
		c := ClassifyFailure(&TestFailure{Error: "Element is not attached to DOM"})
		if c.Category != CategoryTimingFlaky {
			t.Fatalf("category = %q, want %q", c.Category, CategoryTimingFlaky)
		}
		if !c.IsFlaky {
			t.Fatal("expected IsFlaky=true")
		}
	})

	t.Run("network flaky", func(t *testing.T) {
		c := ClassifyFailure(&TestFailure{Error: "net::ERR_CONNECTION_REFUSED"})
		if c.Category != CategoryNetworkFlaky {
			t.Fatalf("category = %q, want %q", c.Category, CategoryNetworkFlaky)
		}
		if !c.IsEnvironment {
			t.Fatal("expected IsEnvironment=true")
		}
	})

	t.Run("real bug", func(t *testing.T) {
		c := ClassifyFailure(&TestFailure{Error: `Expected "hello" to be "world"`})
		if c.Category != CategoryRealBug {
			t.Fatalf("category = %q, want %q", c.Category, CategoryRealBug)
		}
		if !c.IsRealBug {
			t.Fatal("expected IsRealBug=true")
		}
	})

	t.Run("test bug viewport", func(t *testing.T) {
		c := ClassifyFailure(&TestFailure{Error: "Element is outside viewport"})
		if c.Category != CategoryTestBug {
			t.Fatalf("category = %q, want %q", c.Category, CategoryTestBug)
		}
		if c.SuggestedFix == nil || c.SuggestedFix.Type != "scroll_to_element" {
			t.Fatalf("expected scroll_to_element fix, got %+v", c.SuggestedFix)
		}
	})

	t.Run("unknown gets manual_review", func(t *testing.T) {
		c := ClassifyFailure(&TestFailure{Error: "weird error nobody recognizes"})
		if c.RecommendedAction != "manual_review" {
			t.Fatalf("action = %q, want manual_review", c.RecommendedAction)
		}
	})
}

func TestClassifyFailureBatch(t *testing.T) {
	t.Parallel()

	failures := []TestFailure{
		{TestName: "t1", Error: `Timeout waiting for selector "#btn"`},
		{TestName: "t2", Error: "net::ERR_CONNECTION_REFUSED"},
		{TestName: "t3", Error: `Expected "a" to be "b"`},
		{TestName: "t4", Error: "Element is outside viewport"},
		{TestName: "t5", Error: "unknown error xyz"},
	}

	result := ClassifyFailureBatch(failures)
	if result.TotalClassified != 5 {
		t.Fatalf("TotalClassified = %d, want 5", result.TotalClassified)
	}
	if result.RealBugs != 1 {
		t.Fatalf("RealBugs = %d, want 1", result.RealBugs)
	}
	if result.FlakyTests != 1 {
		t.Fatalf("FlakyTests = %d, want 1", result.FlakyTests)
	}
	if result.TestBugs != 1 {
		t.Fatalf("TestBugs = %d, want 1", result.TestBugs)
	}
	if result.Uncertain != 1 {
		t.Fatalf("Uncertain = %d, want 1", result.Uncertain)
	}
}

func TestGenerateSuggestedFix(t *testing.T) {
	t.Parallel()

	t.Run("selector broken with selector", func(t *testing.T) {
		fix := GenerateSuggestedFix(CategorySelectorBroken, `Timeout waiting for selector "#btn"`)
		if fix == nil || fix.Type != "selector_update" {
			t.Fatalf("expected selector_update, got %+v", fix)
		}
		if fix.Old != "#btn" {
			t.Fatalf("expected Old=#btn, got %q", fix.Old)
		}
	})

	t.Run("timing flaky", func(t *testing.T) {
		fix := GenerateSuggestedFix(CategoryTimingFlaky, "timeout")
		if fix == nil || fix.Type != "add_wait" {
			t.Fatalf("expected add_wait, got %+v", fix)
		}
		if !strings.Contains(fix.Code, "waitForSelector") {
			t.Fatalf("expected waitForSelector in code, got %q", fix.Code)
		}
	})

	t.Run("network flaky", func(t *testing.T) {
		fix := GenerateSuggestedFix(CategoryNetworkFlaky, "net::ERR")
		if fix == nil || fix.Type != "mock_network" {
			t.Fatalf("expected mock_network, got %+v", fix)
		}
	})

	t.Run("test bug viewport", func(t *testing.T) {
		fix := GenerateSuggestedFix(CategoryTestBug, "outside viewport")
		if fix == nil || fix.Type != "scroll_to_element" {
			t.Fatalf("expected scroll_to_element, got %+v", fix)
		}
	})

	t.Run("unknown returns nil", func(t *testing.T) {
		if fix := GenerateSuggestedFix(CategoryUnknown, "anything"); fix != nil {
			t.Fatalf("expected nil for unknown, got %+v", fix)
		}
	})

	t.Run("real bug returns nil", func(t *testing.T) {
		if fix := GenerateSuggestedFix(CategoryRealBug, "assertion failed"); fix != nil {
			t.Fatalf("expected nil for real bug, got %+v", fix)
		}
	})
}
