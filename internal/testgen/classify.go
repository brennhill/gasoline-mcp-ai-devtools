// classify.go — Test failure classification and suggested fixes.
package testgen

import (
	"fmt"
	"regexp"
	"strings"
)

// categoryActions maps failure categories to their recommended action.
var categoryActions = map[string]string{
	CategorySelectorBroken: "heal",
	CategoryTimingFlaky:    "add_wait",
	CategoryNetworkFlaky:   "mock_network",
	CategoryRealBug:        "fix_bug",
	CategoryTestBug:        "fix_test",
}

// ClassifyFailure analyzes a test failure and categorizes it.
func ClassifyFailure(failure *TestFailure) *FailureClassification {
	category, confidence, evidence := MatchClassificationPattern(failure.Error)

	action := categoryActions[category]
	if action == "" {
		action = "manual_review"
	}

	return &FailureClassification{
		Category:          category,
		Confidence:        confidence,
		Evidence:          evidence,
		IsRealBug:         category == CategoryRealBug,
		IsFlaky:           category == CategoryTimingFlaky || category == CategoryNetworkFlaky,
		IsEnvironment:     category == CategoryNetworkFlaky,
		RecommendedAction: action,
		SuggestedFix:      GenerateSuggestedFix(category, failure.Error),
	}
}

// ClassifyFailureBatch classifies multiple test failures.
func ClassifyFailureBatch(failures []TestFailure) *BatchClassifyResult {
	result := &BatchClassifyResult{
		TotalClassified: len(failures),
		Classifications: make([]FailureClassification, len(failures)),
		Summary:         make(map[string]int),
	}

	for i, failure := range failures {
		classification := ClassifyFailure(&failure)
		result.Classifications[i] = *classification
		result.Summary[classification.Category]++
		AccumulateBatchCounters(result, classification)
	}

	return result
}

// AccumulateBatchCounters updates batch result counters from a classification.
func AccumulateBatchCounters(result *BatchClassifyResult, c *FailureClassification) {
	if c.IsRealBug {
		result.RealBugs++
	}
	if c.IsFlaky {
		result.FlakyTests++
	}
	if c.Category == CategoryTestBug {
		result.TestBugs++
	}
	if c.Confidence < 0.5 {
		result.Uncertain++
	}
}

// classificationRule defines a single error pattern matcher.
type classificationRule struct {
	match      func(string) bool
	category   string
	confidence float64
	evidence   func(string) []string
}

var selectorExtractPattern = regexp.MustCompile(`selector\s+["']([^"']+)["']`)

// classificationRules is evaluated in order; the first match wins.
var classificationRules = []classificationRule{
	{
		match: func(msg string) bool {
			if !strings.Contains(msg, "waiting for selector") {
				return false
			}
			return selectorExtractPattern.MatchString(msg)
		},
		category:   CategorySelectorBroken,
		confidence: 0.9,
		evidence: func(msg string) []string {
			matches := selectorExtractPattern.FindStringSubmatch(msg)
			return []string{
				fmt.Sprintf("Selector '%s' not found in current DOM", matches[1]),
				"Error pattern matches 'Timeout waiting for selector'",
			}
		},
	},
	{
		match: func(msg string) bool {
			return strings.Contains(msg, "waiting for selector")
		},
		category:   CategoryTimingFlaky,
		confidence: 0.8,
		evidence: func(_ string) []string {
			return []string{"Timeout waiting for selector", "Element might exist but timing is inconsistent"}
		},
	},
	{
		match: func(msg string) bool {
			return strings.Contains(msg, "net::ERR_") || strings.Contains(msg, "Network")
		},
		category:   CategoryNetworkFlaky,
		confidence: 0.85,
		evidence: func(msg string) []string {
			return []string{"Network error detected: " + msg, "Likely network connectivity or service availability issue"}
		},
	},
	{
		match: func(msg string) bool {
			return strings.Contains(msg, "Expected") &&
				(strings.Contains(msg, "to be") || strings.Contains(msg, "toBe") || strings.Contains(msg, "toEqual"))
		},
		category:   CategoryRealBug,
		confidence: 0.7,
		evidence: func(_ string) []string {
			return []string{"Assertion failure detected", "Actual value differs from expected value"}
		},
	},
	{
		match: func(msg string) bool {
			return strings.Contains(msg, "not attached to DOM") || strings.Contains(msg, "Element is not attached")
		},
		category:   CategoryTimingFlaky,
		confidence: 0.8,
		evidence: func(_ string) []string {
			return []string{"Element detached from DOM during test", "Timing issue - element removed before interaction"}
		},
	},
	{
		match: func(msg string) bool {
			return strings.Contains(msg, "outside viewport") || strings.Contains(msg, "not visible")
		},
		category:   CategoryTestBug,
		confidence: 0.75,
		evidence: func(_ string) []string {
			return []string{"Element is outside viewport", "Test needs to scroll or resize viewport"}
		},
	},
}

// MatchClassificationPattern matches error patterns and returns category, confidence, and evidence.
func MatchClassificationPattern(errorMsg string) (string, float64, []string) {
	for _, rule := range classificationRules {
		if rule.match(errorMsg) {
			return rule.category, rule.confidence, rule.evidence(errorMsg)
		}
	}
	return CategoryUnknown, 0.3, []string{"Error pattern not recognized", "Error message: " + errorMsg}
}

// suggestedFixGenerators maps categories to fix-generation functions.
var suggestedFixGenerators = map[string]func(string) *SuggestedFix{
	CategorySelectorBroken: suggestedFixSelector,
	CategoryTimingFlaky: func(_ string) *SuggestedFix {
		return &SuggestedFix{Type: "add_wait", Code: "await page.waitForSelector('...', { state: 'visible' })"}
	},
	CategoryNetworkFlaky: func(_ string) *SuggestedFix {
		return &SuggestedFix{Type: "mock_network", Code: "await page.route('**/api/**', route => route.fulfill({ ... }))"}
	},
	CategoryTestBug: suggestedFixTestBug,
}

func suggestedFixSelector(errorMsg string) *SuggestedFix {
	matches := selectorExtractPattern.FindStringSubmatch(errorMsg)
	if len(matches) > 1 {
		return &SuggestedFix{Type: "selector_update", Old: matches[1], Code: "Use test_heal to find replacement selector"}
	}
	return &SuggestedFix{Type: "selector_update", Code: "Use test_heal to find replacement selector"}
}

func suggestedFixTestBug(errorMsg string) *SuggestedFix {
	if strings.Contains(errorMsg, "viewport") {
		return &SuggestedFix{Type: "scroll_to_element", Code: "await element.scrollIntoViewIfNeeded()"}
	}
	return nil
}

// GenerateSuggestedFix creates a suggested fix based on category and error message.
func GenerateSuggestedFix(category string, errorMsg string) *SuggestedFix {
	gen := suggestedFixGenerators[category]
	if gen == nil {
		return nil
	}
	return gen(errorMsg)
}
