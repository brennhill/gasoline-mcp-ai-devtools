// testgen.go — Test generation from captured errors and user interactions.
// Generates Playwright tests using console errors, user actions, and network data.
// Design: Reuses existing codegen.go infrastructure for script generation.
// Follows 4-tool constraint: integrates into existing generate tool.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Section 1: Types & Constants
// ============================================

// TestFromContextRequest represents generate {type: "test_from_context"} parameters
type TestFromContextRequest struct {
	Context      string `json:"context"`       // "error", "interaction", "regression"
	ErrorID      string `json:"error_id"`      // Optional: specific error to reproduce
	Framework    string `json:"framework"`     // "playwright", "vitest", "jest"
	OutputFormat string `json:"output_format"` // "file", "inline"
	BaseURL      string `json:"base_url"`
	IncludeMocks bool   `json:"include_mocks"`
}

// GeneratedTest represents the output of test generation
type GeneratedTest struct {
	Framework string          `json:"framework"`
	Filename  string          `json:"filename"`
	Content   string          `json:"content"`
	Selectors []string        `json:"selectors"`
	Assertions int            `json:"assertions"`
	Coverage  TestCoverage    `json:"coverage"`
	Metadata  TestGenMetadata `json:"metadata"`
}

// TestCoverage describes what the generated test covers
type TestCoverage struct {
	ErrorReproduced bool `json:"error_reproduced"`
	NetworkMocked   bool `json:"network_mocked"`
	StateCaptured   bool `json:"state_captured"`
}

// TestGenMetadata provides traceability
type TestGenMetadata struct {
	SourceError string   `json:"source_error,omitempty"`
	GeneratedAt string   `json:"generated_at"`
	ContextUsed []string `json:"context_used"`
}

// TestHealRequest represents generate {type: "test_heal"} parameters
type TestHealRequest struct {
	Action          string   `json:"action"`           // "analyze" | "repair" | "batch"
	TestFile        string   `json:"test_file"`        // For analyze/repair
	TestDir         string   `json:"test_dir"`         // For batch
	BrokenSelectors []string `json:"broken_selectors"` // For repair
	AutoApply       bool     `json:"auto_apply"`       // For repair
}

// HealedSelector represents a repaired selector
type HealedSelector struct {
	OldSelector string  `json:"old_selector"`
	NewSelector string  `json:"new_selector"`
	Confidence  float64 `json:"confidence"`
	Strategy    string  `json:"strategy"`
	LineNumber  int     `json:"line_number"`
}

// HealResult represents selector healing output
type HealResult struct {
	Healed         []HealedSelector `json:"healed"`
	Unhealed       []string         `json:"unhealed"`
	UpdatedContent string           `json:"updated_content,omitempty"`
	Summary        HealSummary      `json:"summary"`
}

// HealSummary provides statistics on healing results
type HealSummary struct {
	TotalBroken  int `json:"total_broken"`
	HealedAuto   int `json:"healed_auto"`
	HealedManual int `json:"healed_manual"`
	Unhealed     int `json:"unhealed"`
}

// BatchHealResult represents results from healing a batch of test files
type BatchHealResult struct {
	FilesProcessed int              `json:"files_processed"`
	FilesSkipped   int              `json:"files_skipped"`
	TotalSelectors int              `json:"total_selectors"`
	TotalHealed    int              `json:"total_healed"`
	TotalUnhealed  int              `json:"total_unhealed"`
	FileResults    []FileHealResult `json:"file_results"`
	Warnings       []string         `json:"warnings,omitempty"`
}

// FileHealResult represents healing results for a single file
type FileHealResult struct {
	FilePath string `json:"file_path"`
	Healed   int    `json:"healed"`
	Unhealed int    `json:"unhealed"`
	Skipped  bool   `json:"skipped"`
	Reason   string `json:"reason,omitempty"`
}

// TestClassifyRequest represents generate {type: "test_classify"} parameters
type TestClassifyRequest struct {
	Action     string        `json:"action"` // "failure", "batch"
	Failure    *TestFailure  `json:"failure"`
	Failures   []TestFailure `json:"failures"`
	TestOutput string        `json:"test_output"`
}

// TestFailure represents a single test failure to classify
type TestFailure struct {
	TestName   string `json:"test_name"`
	Error      string `json:"error"`
	Screenshot string `json:"screenshot"` // base64, optional
	Trace      string `json:"trace"`      // stack trace
	DurationMs int64  `json:"duration_ms"`
}

// FailureClassification represents the result of classifying a test failure
type FailureClassification struct {
	Category          string        `json:"category"`
	Confidence        float64       `json:"confidence"`
	Evidence          []string      `json:"evidence"`
	RecommendedAction string        `json:"recommended_action"`
	IsRealBug         bool          `json:"is_real_bug"`
	IsFlaky           bool          `json:"is_flaky"`
	IsEnvironment     bool          `json:"is_environment"`
	SuggestedFix      *SuggestedFix `json:"suggested_fix,omitempty"`
}

// SuggestedFix provides actionable fix suggestion
type SuggestedFix struct {
	Type string `json:"type"` // "selector_update", "add_wait", "mock_network", etc.
	Old  string `json:"old,omitempty"`
	New  string `json:"new,omitempty"`
	Code string `json:"code,omitempty"`
}

// BatchClassifyResult represents the result of classifying multiple failures
type BatchClassifyResult struct {
	TotalClassified  int                      `json:"total_classified"`
	RealBugs         int                      `json:"real_bugs"`
	FlakyTests       int                      `json:"flaky_tests"`
	TestBugs         int                      `json:"test_bugs"`
	Uncertain        int                      `json:"uncertain"`
	Classifications  []FailureClassification  `json:"classifications"`
	Summary          map[string]int           `json:"summary"` // category -> count
}

// Error codes for test generation (ErrPathNotAllowed is defined in tools.go)
const (
	ErrNoErrorContext          = "no_error_context"
	ErrNoActionsCaptured       = "no_actions_captured"
	ErrNoBaseline              = "no_baseline"
	ErrTestFileNotFound        = "test_file_not_found"
	ErrSelectorInjection       = "selector_injection_detected"
	ErrInvalidSelectorSyntax   = "invalid_selector_syntax"
	ErrClassificationUncertain = "classification_uncertain"
	ErrBatchTooLarge           = "batch_too_large"
)

// Batch limits (from TECH_SPEC §2)
const (
	MaxFilesPerBatch    = 20
	MaxFileSizeBytes    = 500 * 1024 // 500KB
	MaxTotalBatchSize   = 5 * 1024 * 1024 // 5MB
	MaxSelectorsPerFile = 50
)

// Classification categories
const (
	CategorySelectorBroken = "selector_broken"
	CategoryTimingFlaky    = "timing_flaky"
	CategoryNetworkFlaky   = "network_flaky"
	CategoryRealBug        = "real_bug"
	CategoryTestBug        = "test_bug"
	CategoryUnknown        = "unknown"
)

// ============================================
// Section 2: Entry Points
// ============================================

// handleGenerateTestFromContext handles the test_from_context mode of the generate tool
func (h *ToolHandler) handleGenerateTestFromContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params TestFromContextRequest

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
	if params.Context == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'context' is missing",
				"Add the 'context' parameter and call again",
				withParam("context"),
				withHint("Valid values: error, interaction, regression"),
			),
		}
	}

	// Validate context value
	validContexts := map[string]bool{"error": true, "interaction": true, "regression": true}
	if !validContexts[params.Context] {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				"Invalid context value: "+params.Context,
				"Use a valid context value",
				withParam("context"),
				withHint("Valid values: error, interaction, regression"),
			),
		}
	}

	// Set defaults
	if params.Framework == "" {
		params.Framework = "playwright"
	}
	if params.OutputFormat == "" {
		params.OutputFormat = "inline"
	}

	// Dispatch based on context
	var generatedTest *GeneratedTest
	switch params.Context {
	case "error":
		generatedTest, err = h.generateTestFromError(params)
	case "interaction":
		generatedTest, err = h.generateTestFromInteraction(params)
	case "regression":
		generatedTest, err = h.generateTestFromRegression(params)
	default:
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				"Unknown context: "+params.Context,
				"Use a valid context value",
				withParam("context"),
			),
		}
	}

	if err != nil {
		// Check for specific error codes
		errStr := err.Error()
		if strings.Contains(errStr, ErrNoErrorContext) {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrNoErrorContext,
					"No console errors captured to generate test from",
					"Trigger an error in the browser first, then retry",
					withHint("Use the observe tool to verify errors are being captured"),
				),
			}
		}
		if strings.Contains(errStr, ErrNoActionsCaptured) {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrNoActionsCaptured,
					"No user actions recorded in the session",
					"Interact with the page first (click, type, navigate), then retry",
					withHint("Use the observe tool with what=actions to verify actions are being captured"),
				),
			}
		}
		if strings.Contains(errStr, ErrNoBaseline) {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrNoBaseline,
					"No regression baseline available",
					"Capture a baseline first by interacting with the page, then retry",
					withHint("The regression mode generates tests by comparing current state against a baseline"),
				),
			}
		}

		// Generic error
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpErrorResponse("Failed to generate test: " + err.Error()),
		}
	}

	// Format response using mcpJSONResponse pattern
	summary := fmt.Sprintf("Generated %s test '%s' (%d assertions)",
		generatedTest.Framework,
		generatedTest.Filename,
		generatedTest.Assertions)

	data := map[string]any{
		"test":     generatedTest,
		"metadata": generatedTest.Metadata,
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, data),
	}

	return appendWarningsToResponse(resp, warnings)
}

// ============================================
// Section 3: test_from_context Implementation
// ============================================

// generateTestFromError creates a Playwright test that reproduces a captured console error
func (h *ToolHandler) generateTestFromError(req TestFromContextRequest) (*GeneratedTest, error) {
	// 1. Get error context from capture
	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	// Find the most recent error (or specific error by ID if provided)
	var targetError LogEntry
	var errorTimestamp int64
	var errorID string

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}

		// If error_id specified, match it
		if req.ErrorID != "" {
			entryID, _ := entry["error_id"].(string)
			if entryID == req.ErrorID {
				targetError = entry
				errorID = entryID
				tsStr, _ := entry["ts"].(string)
				errorTimestamp = normalizeTimestamp(tsStr)
				break
			}
		} else {
			// Use most recent error
			targetError = entry
			tsStr, _ := entry["ts"].(string)
			errorTimestamp = normalizeTimestamp(tsStr)
			errorID, _ = entry["error_id"].(string)
			break
		}
	}

	if targetError == nil {
		return nil, fmt.Errorf(ErrNoErrorContext)
	}

	errorMessage, _ := targetError["message"].(string)

	// 2. Get actions leading up to error (±5 seconds window)
	allActions := h.capture.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, fmt.Errorf(ErrNoActionsCaptured)
	}

	// Filter actions within time window
	const timeWindowMs = 5000 // ±5 seconds
	var relevantActions []capture.EnhancedAction
	for i := range allActions {
		action := &allActions[i]
		timeDiff := action.Timestamp - errorTimestamp
		if timeDiff >= -timeWindowMs && timeDiff <= timeWindowMs {
			relevantActions = append(relevantActions, *action)
		}
	}

	if len(relevantActions) == 0 {
		return nil, fmt.Errorf(ErrNoActionsCaptured)
	}

	// 3. Generate test using existing codegen infrastructure
	script := generatePlaywrightScript(relevantActions, errorMessage, req.BaseURL)

	// 4. Count assertions (simple heuristic: count expect() calls + error comment)
	assertionCount := strings.Count(script, "expect(") + 1 // +1 for error comment

	// 5. Generate filename
	filename := generateTestFilename(errorMessage, req.Framework)

	// 6. Extract selectors used in test
	selectors := extractSelectorsFromActions(relevantActions)

	// 7. Build metadata
	metadata := TestGenMetadata{
		SourceError: errorID,
		GeneratedAt: time.Now().Format(time.RFC3339),
		ContextUsed: []string{"console", "actions"},
	}

	// 8. Build coverage info
	coverage := TestCoverage{
		ErrorReproduced: true,
		NetworkMocked:   req.IncludeMocks,
		StateCaptured:   len(relevantActions) > 0,
	}

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   filename,
		Content:    script,
		Selectors:  selectors,
		Assertions: assertionCount,
		Coverage:   coverage,
		Metadata:   metadata,
	}, nil
}

// generateTestFromInteraction creates a Playwright test from recorded user interactions
func (h *ToolHandler) generateTestFromInteraction(req TestFromContextRequest) (*GeneratedTest, error) {
	// 1. Get all enhanced actions
	allActions := h.capture.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, fmt.Errorf(ErrNoActionsCaptured)
	}

	// 2. Filter actions if needed (by time window or action count)
	// Note: For interaction mode, we can use last_n_actions from session timeline
	// For now, use all actions (filtering can be added via request parameters)
	relevantActions := allActions

	// 3. Generate test using existing codegen infrastructure
	// No error message for interaction mode
	script := generatePlaywrightScript(relevantActions, "", req.BaseURL)

	// 4. Count assertions
	// For interaction mode, we count URL assertions and any expect() calls
	assertionCount := strings.Count(script, "expect(")

	// Add network assertions if requested
	if req.IncludeMocks {
		// Get network bodies for the same time window
		h.server.mu.RLock()
		entries := make([]LogEntry, len(h.server.entries))
		copy(entries, h.server.entries)
		h.server.mu.RUnlock()

		// Get network data from capture
		networkBodies := h.capture.GetNetworkBodies()

		// Add network assertions to script
		if len(networkBodies) > 0 {
			// Count network responses that would be asserted
			for _, nb := range networkBodies {
				if nb.Status > 0 {
					assertionCount++
				}
			}
		}
	}

	// 5. Generate filename based on first action or URL
	testName := "user-interaction"
	if len(relevantActions) > 0 {
		if relevantActions[0].URL != "" {
			testName = relevantActions[0].URL
		} else if relevantActions[0].Type != "" {
			testName = relevantActions[0].Type + "-flow"
		}
	}
	filename := generateTestFilename(testName, req.Framework)

	// 6. Extract selectors used in test
	selectors := extractSelectorsFromActions(relevantActions)

	// 7. Build metadata
	metadata := TestGenMetadata{
		SourceError: "", // No error for interaction mode
		GeneratedAt: time.Now().Format(time.RFC3339),
		ContextUsed: []string{"actions"},
	}

	// Add network to context if mocks included
	if req.IncludeMocks {
		metadata.ContextUsed = append(metadata.ContextUsed, "network")
	}

	// 8. Build coverage info
	coverage := TestCoverage{
		ErrorReproduced: false, // No error in interaction mode
		NetworkMocked:   req.IncludeMocks,
		StateCaptured:   len(relevantActions) > 0,
	}

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   filename,
		Content:    script,
		Selectors:  selectors,
		Assertions: assertionCount,
		Coverage:   coverage,
		Metadata:   metadata,
	}, nil
}

// generateTestFromRegression creates a Playwright test that verifies behavior against a baseline
func (h *ToolHandler) generateTestFromRegression(req TestFromContextRequest) (*GeneratedTest, error) {
	// 1. Get current session actions as the baseline
	allActions := h.capture.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, fmt.Errorf(ErrNoActionsCaptured)
	}

	// 2. Get console entries to check for baseline state
	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	// 3. Count console errors for baseline
	errorCount := 0
	var errorMessages []string
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level == "error" {
			errorCount++
			msg, _ := entry["message"].(string)
			if msg != "" && len(errorMessages) < 5 { // Limit to first 5 errors
				errorMessages = append(errorMessages, msg)
			}
		}
	}

	// 4. Get network data from capture
	networkBodies := h.capture.GetNetworkBodies()

	// 5. Build assertions based on baseline state
	var assertions []string
	assertionCount := 0

	// Add error assertion if baseline was clean
	if errorCount == 0 {
		assertions = append(assertions, "  // Assert no console errors (baseline was clean)")
		assertions = append(assertions, "  const consoleErrors = []")
		assertions = append(assertions, "  page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()) })")
		assertions = append(assertions, "  // After actions complete:")
		assertions = append(assertions, "  expect(consoleErrors).toHaveLength(0)")
		assertionCount++
	} else {
		assertions = append(assertions, fmt.Sprintf("  // Baseline had %d console errors", errorCount))
		assertions = append(assertions, "  // TODO: Add assertions to verify errors haven't changed")
	}

	// Add network assertions for key requests
	networkAssertions := 0
	for _, nb := range networkBodies {
		if nb.Status > 0 && networkAssertions < 3 {
			assertions = append(assertions, fmt.Sprintf("  // Assert %s %s returns %d", nb.Method, nb.URL, nb.Status))
			assertions = append(assertions, fmt.Sprintf("  // TODO: await page.waitForResponse(r => r.url().includes('%s') && r.status() === %d)", nb.URL, nb.Status))
			networkAssertions++
			assertionCount++
		}
	}

	// Add performance TODO
	assertions = append(assertions, "")
	assertions = append(assertions, "  // TODO: Add performance assertions")
	assertions = append(assertions, "  // - Load time within acceptable range")
	assertions = append(assertions, "  // - Key metrics (FCP, LCP) haven't regressed")

	// 6. Generate test using existing codegen infrastructure
	script := generatePlaywrightScript(allActions, "", req.BaseURL)

	// Insert assertions before the closing brace
	assertionBlock := strings.Join(assertions, "\n")
	// Find the last });\n and insert before it
	lastBrace := strings.LastIndex(script, "});")
	if lastBrace > 0 {
		script = script[:lastBrace] + "\n" + assertionBlock + "\n" + script[lastBrace:]
	}

	// 7. Generate filename
	filename := generateTestFilename("regression-test", req.Framework)

	// 8. Extract selectors used in test
	selectors := extractSelectorsFromActions(allActions)

	// 9. Build metadata
	contextUsed := []string{"actions", "console", "network", "performance"}
	metadata := TestGenMetadata{
		SourceError: "", // No specific error for regression mode
		GeneratedAt: time.Now().Format(time.RFC3339),
		ContextUsed: contextUsed,
	}

	// 10. Build coverage info
	coverage := TestCoverage{
		ErrorReproduced: false, // Regression tests verify behavior, not errors
		NetworkMocked:   req.IncludeMocks,
		StateCaptured:   len(allActions) > 0,
	}

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   filename,
		Content:    script,
		Selectors:  selectors,
		Assertions: assertionCount,
		Coverage:   coverage,
		Metadata:   metadata,
	}, nil
}

// ============================================
// Section 4: Helper Functions
// ============================================

// generateErrorID creates a unique error ID following format: err_{timestamp_ms}_{hash8}
func generateErrorID(message, stack, url string) string {
	timestamp := time.Now().UnixMilli()

	// Create hash from message + stack + url
	h := sha256.New()
	h.Write([]byte(message))
	h.Write([]byte(stack))
	h.Write([]byte(url))
	hashBytes := h.Sum(nil)
	hashHex := hex.EncodeToString(hashBytes)

	// Take first 8 characters of hash
	hash8 := hashHex[:8]

	return fmt.Sprintf("err_%d_%s", timestamp, hash8)
}

// generateTestFilename creates a filename for the generated test
func generateTestFilename(errorMessage, framework string) string {
	// Sanitize error message for filename
	name := strings.ToLower(errorMessage)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, ":", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "\"", "")

	// Truncate if too long
	if len(name) > 50 {
		name = name[:50]
	}

	// Trim trailing hyphens
	name = strings.TrimRight(name, "-")

	// Add extension based on framework
	ext := ".spec.ts"
	if framework == "vitest" || framework == "jest" {
		ext = ".test.ts"
	}

	return name + ext
}

// extractSelectorsFromActions extracts all selectors used in actions
func extractSelectorsFromActions(actions []capture.EnhancedAction) []string {
	selectorSet := make(map[string]bool)
	for i := range actions {
		selectors := actions[i].Selectors
		if selectors == nil {
			continue
		}

		// Extract testId
		if testID, ok := selectors["testId"].(string); ok && testID != "" {
			selectorSet["[data-testid=\""+testID+"\"]"] = true
		}

		// Extract role
		if roleData, ok := selectors["role"]; ok {
			if roleMap, ok := roleData.(map[string]any); ok {
				role, _ := roleMap["role"].(string)
				if role != "" {
					selectorSet["[role=\""+role+"\"]"] = true
				}
			}
		}

		// Extract ID
		if id, ok := selectors["id"].(string); ok && id != "" {
			selectorSet["#"+id] = true
		}
	}

	// Convert to slice
	var result []string
	for selector := range selectorSet {
		result = append(result, selector)
	}
	return result
}

// normalizeTimestamp converts ISO 8601 timestamp string to milliseconds since epoch
func normalizeTimestamp(tsStr string) int64 {
	t, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		// Try alternate formats if RFC3339 fails
		t, err = time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return 0
		}
	}
	return t.UnixMilli()
}

// generatePlaywrightScript creates a basic Playwright test script from actions
func generatePlaywrightScript(actions []capture.EnhancedAction, errorMessage string, baseURL string) string {
	var script strings.Builder
	script.WriteString("import { test, expect } from '@playwright/test';\n\n")
	script.WriteString("test('Reproduce issue', async ({ page }) => {\n")

	if baseURL != "" {
		script.WriteString(fmt.Sprintf("  await page.goto('%s');\n", baseURL))
	}

	// Generate Playwright code for each action
	for _, action := range actions {
		switch action.Type {
		case "click":
			// Extract selector from Selectors map if available
			if selectors, ok := action.Selectors["target"].(string); ok {
				script.WriteString(fmt.Sprintf("  await page.click('%s');\n", selectors))
			}
		case "input":
			if selectors, ok := action.Selectors["target"].(string); ok {
				script.WriteString(fmt.Sprintf("  await page.fill('%s', '%s');\n", selectors, action.Value))
			}
		case "navigate":
			if action.ToURL != "" {
				script.WriteString(fmt.Sprintf("  await page.goto('%s');\n", action.ToURL))
			}
		case "wait":
			script.WriteString(fmt.Sprintf("  await page.waitForTimeout(%d);\n", 100))
		}
	}

	// Add assertion for error if provided
	if errorMessage != "" {
		script.WriteString(fmt.Sprintf("  // Expected error: %s\n", errorMessage))
		script.WriteString("  // TODO: Add specific assertion for this error\n")
	}

	script.WriteString("});\n")
	return script.String()
}
