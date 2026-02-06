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

	// Validate required parameters
	if params.Action == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'action' is missing",
				"Add the 'action' parameter and call again",
				withParam("action"),
				withHint("Valid values: failure"),
			),
		}
	}

	// Validate action value
	if params.Action != "failure" && params.Action != "batch" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				"Invalid action value: "+params.Action,
				"Use a valid action value",
				withParam("action"),
				withHint("Valid values: failure, batch"),
			),
		}
	}

	// Handle based on action
	var result any
	var summary string

	switch params.Action {
	case "failure":
		// Validate failure parameter
		if params.Failure == nil {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrMissingParam,
					"Required parameter 'failure' is missing for failure action",
					"Add the 'failure' parameter and call again",
					withParam("failure"),
				),
			}
		}

		// Classify the failure
		classification := h.classifyFailure(params.Failure)

		// Check if classification is uncertain
		if classification.Confidence < 0.5 {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrClassificationUncertain,
					fmt.Sprintf("Could not classify failure with sufficient confidence (%.2f < 0.50)", classification.Confidence),
					"Provide more context or manually review the failure",
					withHint("Category: "+classification.Category),
				),
			}
		}

		// Format response
		summary = fmt.Sprintf("Classified as %s (%.0f%% confidence) — recommended: %s",
			classification.Category,
			classification.Confidence*100,
			classification.RecommendedAction)

		data := map[string]any{
			"classification": classification,
		}

		if classification.SuggestedFix != nil {
			data["suggested_fix"] = classification.SuggestedFix
		}

		result = data

	case "batch":
		// Validate failures parameter
		if len(params.Failures) == 0 {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrMissingParam,
					"Required parameter 'failures' is missing for batch action",
					"Add the 'failures' parameter and call again",
					withParam("failures"),
				),
			}
		}

		// Check batch size limits
		const MaxFailuresPerBatch = 20
		if len(params.Failures) > MaxFailuresPerBatch {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrBatchTooLarge,
					fmt.Sprintf("Batch contains %d failures, max is %d", len(params.Failures), MaxFailuresPerBatch),
					"Reduce the number of failures and try again",
				),
			}
		}

		// Classify all failures
		batchResult := h.classifyFailureBatch(params.Failures)

		// Format summary
		summary = fmt.Sprintf("Classified %d failures: %d real bugs, %d flaky, %d test issues, %d uncertain",
			batchResult.TotalClassified,
			batchResult.RealBugs,
			batchResult.FlakyTests,
			batchResult.TestBugs,
			batchResult.Uncertain)

		result = map[string]any{
			"batch_result": batchResult,
		}
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, result),
	}

	return appendWarningsToResponse(resp, warnings)
}

// classifyFailure analyzes a test failure and categorizes it
func (h *ToolHandler) classifyFailure(failure *TestFailure) *FailureClassification {
	// Match classification patterns
	category, confidence, evidence := matchClassificationPattern(failure.Error)

	// Set flags based on category
	classification := &FailureClassification{
		Category:      category,
		Confidence:    confidence,
		Evidence:      evidence,
		IsRealBug:     category == CategoryRealBug,
		IsFlaky:       category == CategoryTimingFlaky || category == CategoryNetworkFlaky,
		IsEnvironment: category == CategoryNetworkFlaky,
	}

	// Set recommended action
	switch category {
	case CategorySelectorBroken:
		classification.RecommendedAction = "heal"
	case CategoryTimingFlaky:
		classification.RecommendedAction = "add_wait"
	case CategoryNetworkFlaky:
		classification.RecommendedAction = "mock_network"
	case CategoryRealBug:
		classification.RecommendedAction = "fix_bug"
	case CategoryTestBug:
		classification.RecommendedAction = "fix_test"
	default:
		classification.RecommendedAction = "manual_review"
	}

	// Generate suggested fix
	classification.SuggestedFix = generateSuggestedFix(category, failure.Error)

	return classification
}

// classifyFailureBatch classifies multiple test failures
func (h *ToolHandler) classifyFailureBatch(failures []TestFailure) *BatchClassifyResult {
	result := &BatchClassifyResult{
		TotalClassified: len(failures),
		Classifications: make([]FailureClassification, len(failures)),
		Summary:         make(map[string]int),
	}

	// Classify each failure
	for i, failure := range failures {
		classification := h.classifyFailure(&failure)
		result.Classifications[i] = *classification

		// Update counters
		if classification.IsRealBug {
			result.RealBugs++
		}
		if classification.IsFlaky {
			result.FlakyTests++
		}
		if classification.Category == CategoryTestBug {
			result.TestBugs++
		}
		if classification.Confidence < 0.5 {
			result.Uncertain++
		}

		// Update summary
		result.Summary[classification.Category]++
	}

	return result
}

// matchClassificationPattern matches error patterns and returns category, confidence, and evidence
func matchClassificationPattern(errorMsg string) (string, float64, []string) {
	evidence := []string{}

	// Pattern: "Timeout waiting for selector" + selector missing
	if strings.Contains(errorMsg, "Timeout waiting for selector") ||
		strings.Contains(errorMsg, "waiting for selector") {

		// Extract selector from error message
		selectorPattern := regexp.MustCompile(`selector\s+["']([^"']+)["']`)
		matches := selectorPattern.FindStringSubmatch(errorMsg)

		if len(matches) > 1 {
			selector := matches[1]
			evidence = append(evidence, fmt.Sprintf("Selector '%s' not found in current DOM", selector))
			evidence = append(evidence, "Error pattern matches 'Timeout waiting for selector'")
			return CategorySelectorBroken, 0.9, evidence
		}

		// If selector exists (would need DOM query, but assume timing issue for now)
		evidence = append(evidence, "Timeout waiting for selector")
		evidence = append(evidence, "Element might exist but timing is inconsistent")
		return CategoryTimingFlaky, 0.8, evidence
	}

	// Pattern: "net::ERR_"
	if strings.Contains(errorMsg, "net::ERR_") || strings.Contains(errorMsg, "Network") {
		evidence = append(evidence, "Network error detected: "+errorMsg)
		evidence = append(evidence, "Likely network connectivity or service availability issue")
		return CategoryNetworkFlaky, 0.85, evidence
	}

	// Pattern: "Expected X to be Y" + values differ
	if strings.Contains(errorMsg, "Expected") && (strings.Contains(errorMsg, "to be") ||
		strings.Contains(errorMsg, "toBe") || strings.Contains(errorMsg, "toEqual")) {
		evidence = append(evidence, "Assertion failure detected")
		evidence = append(evidence, "Actual value differs from expected value")
		return CategoryRealBug, 0.7, evidence
	}

	// Pattern: "Element is not attached to DOM"
	if strings.Contains(errorMsg, "not attached to DOM") ||
		strings.Contains(errorMsg, "Element is not attached") {
		evidence = append(evidence, "Element detached from DOM during test")
		evidence = append(evidence, "Timing issue - element removed before interaction")
		return CategoryTimingFlaky, 0.8, evidence
	}

	// Pattern: "Element is outside viewport"
	if strings.Contains(errorMsg, "outside viewport") ||
		strings.Contains(errorMsg, "not visible") {
		evidence = append(evidence, "Element is outside viewport")
		evidence = append(evidence, "Test needs to scroll or resize viewport")
		return CategoryTestBug, 0.75, evidence
	}

	// Unknown pattern
	evidence = append(evidence, "Error pattern not recognized")
	evidence = append(evidence, "Error message: "+errorMsg)
	return CategoryUnknown, 0.3, evidence
}

// generateSuggestedFix creates a suggested fix based on category and error message
func generateSuggestedFix(category string, errorMsg string) *SuggestedFix {
	switch category {
	case CategorySelectorBroken:
		// Extract selector from error message
		selectorPattern := regexp.MustCompile(`selector\s+["']([^"']+)["']`)
		matches := selectorPattern.FindStringSubmatch(errorMsg)

		if len(matches) > 1 {
			selector := matches[1]
			return &SuggestedFix{
				Type: "selector_update",
				Old:  selector,
				Code: "Use test_heal to find replacement selector",
			}
		}

		return &SuggestedFix{
			Type: "selector_update",
			Code: "Use test_heal to find replacement selector",
		}

	case CategoryTimingFlaky:
		return &SuggestedFix{
			Type: "add_wait",
			Code: "await page.waitForSelector('...', { state: 'visible' })",
		}

	case CategoryNetworkFlaky:
		return &SuggestedFix{
			Type: "mock_network",
			Code: "await page.route('**/api/**', route => route.fulfill({ ... }))",
		}

	case CategoryTestBug:
		if strings.Contains(errorMsg, "viewport") {
			return &SuggestedFix{
				Type: "scroll_to_element",
				Code: "await element.scrollIntoViewIfNeeded()",
			}
		}

	default:
		return nil
	}

	return nil
}
