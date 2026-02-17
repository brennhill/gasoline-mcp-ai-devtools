// Purpose: Owns testgen_classify.go runtime behavior and integration logic.
// Docs: docs/features/feature/test-generation/index.md

// testgen_classify.go — Test failure classification and suggested fixes.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// handleGenerateTestClassify handles the test_classify mode of the generate tool
func (h *ToolHandler) handleGenerateTestClassify(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params TestClassifyRequest

	warnings, err := unmarshalWithWarnings(args, &params)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidJSON,
				"Invalid JSON arguments: "+err.Error(),
				"Fix JSON syntax and call again",
			),
		}
	}

	if errResp, ok := validateClassifyParams(req.ID, params); !ok {
		return errResp
	}

	result, summary, errResp := h.dispatchClassifyAction(req.ID, params)
	if errResp != nil {
		return *errResp
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, result),
	}
	return appendWarningsToResponse(resp, warnings)
}

func (h *ToolHandler) dispatchClassifyAction(reqID any, params TestClassifyRequest) (any, string, *JSONRPCResponse) {
	switch params.Action {
	case "failure":
		result, summary, errResp, ok := h.classifySingleFailure(reqID, params)
		if !ok {
			return nil, "", &errResp
		}
		return result, summary, nil
	case "batch":
		result, summary, errResp, ok := h.classifyBatchFailures(reqID, params)
		if !ok {
			return nil, "", &errResp
		}
		return result, summary, nil
	}
	return nil, "", nil
}

var validClassifyActions = map[string]bool{"failure": true, "batch": true}

func validateClassifyParams(reqID any, params TestClassifyRequest) (JSONRPCResponse, bool) {
	if params.Action == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'action' is missing",
				"Add the 'action' parameter and call again",
				withParam("action"),
				withHint("Valid values: failure"),
			),
		}, false
	}
	if !validClassifyActions[params.Action] {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				"Invalid action value: "+params.Action,
				"Use a valid action value",
				withParam("action"),
				withHint("Valid values: failure, batch"),
			),
		}, false
	}
	return JSONRPCResponse{}, true
}

func (h *ToolHandler) classifySingleFailure(reqID any, params TestClassifyRequest) (any, string, JSONRPCResponse, bool) {
	if params.Failure == nil {
		return nil, "", JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'failure' is missing for failure action",
				"Add the 'failure' parameter and call again",
				withParam("failure"),
			),
		}, false
	}

	classification := h.classifyFailure(params.Failure)

	if classification.Confidence < 0.5 {
		return nil, "", JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcpStructuredError(
				ErrClassificationUncertain,
				fmt.Sprintf("Could not classify failure with sufficient confidence (%.2f < 0.50)", classification.Confidence),
				"Provide more context or manually review the failure",
				withHint("Category: "+classification.Category),
			),
		}, false
	}

	summary := fmt.Sprintf("Classified as %s (%.0f%% confidence) — recommended: %s",
		classification.Category,
		classification.Confidence*100,
		classification.RecommendedAction)

	data := map[string]any{"classification": classification}
	if classification.SuggestedFix != nil {
		data["suggested_fix"] = classification.SuggestedFix
	}
	return data, summary, JSONRPCResponse{}, true
}

const maxFailuresPerBatch = 20

func (h *ToolHandler) classifyBatchFailures(reqID any, params TestClassifyRequest) (any, string, JSONRPCResponse, bool) {
	if len(params.Failures) == 0 {
		return nil, "", JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'failures' is missing for batch action",
				"Add the 'failures' parameter and call again",
				withParam("failures"),
			),
		}, false
	}

	if len(params.Failures) > maxFailuresPerBatch {
		return nil, "", JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcpStructuredError(
				ErrBatchTooLarge,
				fmt.Sprintf("Batch contains %d failures, max is %d", len(params.Failures), maxFailuresPerBatch),
				"Reduce the number of failures and try again",
			),
		}, false
	}

	batchResult := h.classifyFailureBatch(params.Failures)

	summary := fmt.Sprintf("Classified %d failures: %d real bugs, %d flaky, %d test issues, %d uncertain",
		batchResult.TotalClassified,
		batchResult.RealBugs,
		batchResult.FlakyTests,
		batchResult.TestBugs,
		batchResult.Uncertain)

	result := map[string]any{"batch_result": batchResult}
	return result, summary, JSONRPCResponse{}, true
}

// categoryActions maps failure categories to their recommended action.
var categoryActions = map[string]string{
	CategorySelectorBroken: "heal",
	CategoryTimingFlaky:    "add_wait",
	CategoryNetworkFlaky:   "mock_network",
	CategoryRealBug:        "fix_bug",
	CategoryTestBug:        "fix_test",
}

// classifyFailure analyzes a test failure and categorizes it
func (h *ToolHandler) classifyFailure(failure *TestFailure) *FailureClassification {
	category, confidence, evidence := matchClassificationPattern(failure.Error)

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
		SuggestedFix:      generateSuggestedFix(category, failure.Error),
	}
}

// classifyFailureBatch classifies multiple test failures
func (h *ToolHandler) classifyFailureBatch(failures []TestFailure) *BatchClassifyResult {
	result := &BatchClassifyResult{
		TotalClassified: len(failures),
		Classifications: make([]FailureClassification, len(failures)),
		Summary:         make(map[string]int),
	}

	for i, failure := range failures {
		classification := h.classifyFailure(&failure)
		result.Classifications[i] = *classification
		result.Summary[classification.Category]++
		accumulateBatchCounters(result, classification)
	}

	return result
}

func accumulateBatchCounters(result *BatchClassifyResult, c *FailureClassification) {
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

// matchClassificationPattern matches error patterns and returns category, confidence, and evidence
func matchClassificationPattern(errorMsg string) (string, float64, []string) {
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

// generateSuggestedFix creates a suggested fix based on category and error message
func generateSuggestedFix(category string, errorMsg string) *SuggestedFix {
	gen := suggestedFixGenerators[category]
	if gen == nil {
		return nil
	}
	return gen(errorMsg)
}
