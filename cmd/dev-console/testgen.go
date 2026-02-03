// testgen.go — Test generation, healing, and classification
// Generates Playwright tests from captured errors and user interactions.
// Design: Reuses existing codegen.go infrastructure for script generation.
// Follows 4-tool constraint: integrates into existing generate tool.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// ============================================
// Section 4: test_heal Implementation
// ============================================

// validateTestFilePath ensures path is within project directory
// Uses the same pattern as validatePathInDir from ai_persistence.go
func validateTestFilePath(path string, projectDir string) error {
	if path == "" {
		return fmt.Errorf("test file path is required")
	}

	// Reject path traversal sequences
	if strings.Contains(path, "..") {
		return fmt.Errorf(ErrPathNotAllowed + ": path contains '..'")
	}

	// Construct full path
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		fullPath = filepath.Join(projectDir, path)
	}

	// Clean and validate
	cleanPath := filepath.Clean(fullPath)
	cleanProject := filepath.Clean(projectDir)

	// Ensure resolved path is within project directory
	if !strings.HasPrefix(cleanPath, cleanProject) {
		return fmt.Errorf(ErrPathNotAllowed + ": path escapes project directory")
	}

	return nil
}

// validateSelector ensures selector is safe for DOM query
func validateSelector(selector string) error {
	// Max length
	if len(selector) > 1000 {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector exceeds 1000 characters")
	}

	// Empty selector
	if selector == "" {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector is empty")
	}

	// Disallow script injection patterns
	dangerous := []string{"javascript:", "<script", "onerror=", "onload="}
	lowerSelector := strings.ToLower(selector)
	for _, pattern := range dangerous {
		if strings.Contains(lowerSelector, pattern) {
			return fmt.Errorf(ErrSelectorInjection + ": selector contains dangerous pattern: " + pattern)
		}
	}

	// Basic CSS selector syntax check (must start with valid chars)
	// Valid selectors start with: letter, #, ., [, or *
	if len(selector) > 0 {
		firstChar := rune(selector[0])
		isValid := (firstChar >= 'a' && firstChar <= 'z') ||
			(firstChar >= 'A' && firstChar <= 'Z') ||
			firstChar == '#' ||
			firstChar == '.' ||
			firstChar == '[' ||
			firstChar == '*'
		if !isValid {
			return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector must start with valid CSS selector character")
		}
	}

	return nil
}

// handleGenerateTestHeal handles the test_heal mode of the generate tool
func (h *ToolHandler) handleGenerateTestHeal(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params TestHealRequest

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
				withHint("Valid values: analyze, repair"),
			),
		}
	}

	// Validate action value
	validActions := map[string]bool{"analyze": true, "repair": true, "batch": true}
	if !validActions[params.Action] {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				"Invalid action value: "+params.Action,
				"Use a valid action value",
				withParam("action"),
				withHint("Valid values: analyze, repair, batch"),
			),
		}
	}

	// Get project directory
	projectDir, _ := os.Getwd()

	// Dispatch based on action
	var result any
	switch params.Action {
	case "analyze":
		if params.TestFile == "" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrMissingParam,
					"Required parameter 'test_file' is missing for analyze action",
					"Add the 'test_file' parameter and call again",
					withParam("test_file"),
				),
			}
		}
		selectors, err := h.analyzeTestFile(params, projectDir)
		if err != nil {
			if strings.Contains(err.Error(), ErrTestFileNotFound) {
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrTestFileNotFound,
						"Test file not found: "+params.TestFile,
						"Check the file path and try again",
					),
				}
			}
			if strings.Contains(err.Error(), ErrPathNotAllowed) {
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrPathNotAllowed,
						err.Error(),
						"Use a path within the project directory",
					),
				}
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpErrorResponse("Failed to analyze test file: " + err.Error()),
			}
		}
		result = map[string]any{
			"broken_selectors": selectors,
			"count":            len(selectors),
		}

	case "repair":
		if len(params.BrokenSelectors) == 0 {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrMissingParam,
					"Required parameter 'broken_selectors' is missing for repair action",
					"Add the 'broken_selectors' parameter and call again",
					withParam("broken_selectors"),
				),
			}
		}

		healResult, err := h.repairSelectors(params, projectDir)
		if err != nil {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpErrorResponse("Failed to repair selectors: " + err.Error()),
			}
		}
		result = healResult

	case "batch":
		if params.TestDir == "" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrMissingParam,
					"Required parameter 'test_dir' is missing for batch action",
					"Add the 'test_dir' parameter and call again",
					withParam("test_dir"),
				),
			}
		}

		batchResult, err := h.healTestBatch(params, projectDir)
		if err != nil {
			if strings.Contains(err.Error(), ErrPathNotAllowed) {
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrPathNotAllowed,
						err.Error(),
						"Use a path within the project directory",
					),
				}
			}
			if strings.Contains(err.Error(), ErrBatchTooLarge) {
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrBatchTooLarge,
						err.Error(),
						"Reduce the number or size of test files",
					),
				}
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpErrorResponse("Failed to heal test batch: " + err.Error()),
			}
		}
		result = batchResult
	}

	// Format response
	var summary string
	if params.Action == "analyze" {
		count := result.(map[string]any)["count"].(int)
		summary = fmt.Sprintf("Found %d selectors in %s", count, params.TestFile)
	} else if params.Action == "repair" {
		healResult := result.(*HealResult)
		summary = fmt.Sprintf("Healed %d/%d selectors (%d unhealed, %d auto-applied)",
			len(healResult.Healed),
			healResult.Summary.TotalBroken,
			healResult.Summary.Unhealed,
			healResult.Summary.HealedAuto)
	} else if params.Action == "batch" {
		batchResult := result.(*BatchHealResult)
		summary = fmt.Sprintf("Healed %d/%d selectors across %d files (%d files skipped, %d selectors unhealed)",
			batchResult.TotalHealed,
			batchResult.TotalSelectors,
			batchResult.FilesProcessed,
			batchResult.FilesSkipped,
			batchResult.TotalUnhealed)
	}

	data := map[string]any{
		"result": result,
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, data),
	}

	return appendWarningsToResponse(resp, warnings)
}

// analyzeTestFile reads a test file and extracts all selectors used in it
func (h *ToolHandler) analyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	// Validate path
	if err := validateTestFilePath(req.TestFile, projectDir); err != nil {
		return nil, err
	}

	// Construct full path
	var fullPath string
	if filepath.IsAbs(req.TestFile) {
		fullPath = req.TestFile
	} else {
		fullPath = filepath.Join(projectDir, req.TestFile)
	}

	// Read file
	content, err := os.ReadFile(fullPath) // #nosec G304 -- path validated above
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(ErrTestFileNotFound + ": " + req.TestFile)
		}
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	// Extract selectors using regex patterns
	selectors := extractSelectorsFromTestFile(string(content))

	return selectors, nil
}

// extractSelectorsFromTestFile uses regex to find selector patterns in test code
func extractSelectorsFromTestFile(content string) []string {
	var selectors []string
	seen := make(map[string]bool)

	// Patterns to match common Playwright selector methods
	patterns := []string{
		`getByTestId\(['"]([^'"]+)['"]\)`,
		`locator\(['"]([^'"]+)['"]\)`,
		`getByRole\(['"]([^'"]+)['"]\)`,
		`getByLabel\(['"]([^'"]+)['"]\)`,
		`getByText\(['"]([^'"]+)['"]\)`,
		`querySelector\(['"]([^'"]+)['"]\)`,
		`querySelectorAll\(['"]([^'"]+)['"]\)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				selector := match[1]
				if !seen[selector] {
					selectors = append(selectors, selector)
					seen[selector] = true
				}
			}
		}
	}

	return selectors
}

// repairSelectors attempts to heal broken selectors
func (h *ToolHandler) repairSelectors(req TestHealRequest, projectDir string) (*HealResult, error) {
	result := &HealResult{
		Healed:   make([]HealedSelector, 0),
		Unhealed: make([]string, 0),
		Summary: HealSummary{
			TotalBroken: len(req.BrokenSelectors),
		},
	}

	// Validate all selectors first
	for _, selector := range req.BrokenSelectors {
		if err := validateSelector(selector); err != nil {
			// If selector is invalid, mark as unhealed
			result.Unhealed = append(result.Unhealed, selector)
			continue
		}

		// Try to heal the selector
		healed, err := h.healSelector(selector)
		if err != nil || healed == nil {
			result.Unhealed = append(result.Unhealed, selector)
			continue
		}

		result.Healed = append(result.Healed, *healed)

		// Track auto-apply if confidence is high enough
		if req.AutoApply && healed.Confidence >= 0.9 {
			result.Summary.HealedAuto++
		} else if healed.Confidence >= 0.5 {
			result.Summary.HealedManual++
		}
	}

	result.Summary.Unhealed = len(result.Unhealed)

	return result, nil
}

// healSelector attempts to find a replacement for a broken selector
// For Phase 2, this is a simplified implementation that uses basic heuristics
// In a full implementation, this would query the DOM via the extension
func (h *ToolHandler) healSelector(oldSelector string) (*HealedSelector, error) {
	// For now, return a mock implementation
	// In production, this would:
	// 1. Query the DOM for similar elements
	// 2. Score candidates using healing strategies
	// 3. Return the best match

	// Simplified heuristic: try to infer a better selector
	var newSelector string
	var confidence float64
	var strategy string

	// If selector looks like an ID, try to suggest data-testid
	if strings.HasPrefix(oldSelector, "#") {
		idValue := strings.TrimPrefix(oldSelector, "#")
		newSelector = fmt.Sprintf("[data-testid='%s']", idValue)
		confidence = 0.6
		strategy = "testid_match"
	} else if strings.HasPrefix(oldSelector, ".") {
		// Class selector - low confidence
		newSelector = oldSelector
		confidence = 0.3
		strategy = "structural_match"
	} else {
		// Unknown selector type
		return nil, fmt.Errorf("cannot heal selector: %s", oldSelector)
	}

	return &HealedSelector{
		OldSelector: oldSelector,
		NewSelector: newSelector,
		Confidence:  confidence,
		Strategy:    strategy,
		LineNumber:  0, // Would need file parsing to determine
	}, nil
}

// healTestBatch processes multiple test files in a directory
func (h *ToolHandler) healTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	// 1. Validate test_dir parameter
	if err := validateTestFilePath(req.TestDir, projectDir); err != nil {
		return nil, err
	}

	// Construct full path
	var fullPath string
	if filepath.IsAbs(req.TestDir) {
		fullPath = req.TestDir
	} else {
		fullPath = filepath.Join(projectDir, req.TestDir)
	}

	// Check if directory exists
	dirInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(ErrTestFileNotFound + ": directory not found: " + req.TestDir)
		}
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}

	if !dirInfo.IsDir() {
		return nil, fmt.Errorf(ErrInvalidParam + ": test_dir must be a directory")
	}

	// 2. Scan directory for test files
	testFiles, err := findTestFiles(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	// Initialize result
	result := &BatchHealResult{
		FilesProcessed: 0,
		FilesSkipped:   0,
		TotalSelectors: 0,
		TotalHealed:    0,
		TotalUnhealed:  0,
		FileResults:    make([]FileHealResult, 0),
		Warnings:       make([]string, 0),
	}

	// 3. Enforce batch limits
	if len(testFiles) > MaxFilesPerBatch {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Batch limited to %d files (found %d)", MaxFilesPerBatch, len(testFiles)))
		testFiles = testFiles[:MaxFilesPerBatch]
	}

	var totalBatchSize int64

	// 4. For each file: extract selectors, attempt healing, track results
	for _, filePath := range testFiles {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: 0,
				Skipped:  true,
				Reason:   "Failed to read file info",
			})
			result.FilesSkipped++
			continue
		}

		// Check file size limit
		if fileInfo.Size() > MaxFileSizeBytes {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: 0,
				Skipped:  true,
				Reason:   fmt.Sprintf("File size exceeds %dKB limit", MaxFileSizeBytes/1024),
			})
			result.FilesSkipped++
			continue
		}

		// Check total batch size limit
		if totalBatchSize+fileInfo.Size() > MaxTotalBatchSize {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: 0,
				Skipped:  true,
				Reason:   fmt.Sprintf("Total batch size would exceed %dMB limit", MaxTotalBatchSize/(1024*1024)),
			})
			result.FilesSkipped++
			continue
		}

		totalBatchSize += fileInfo.Size()

		// Extract selectors from file
		fileReq := TestHealRequest{
			Action:   "analyze",
			TestFile: filePath,
		}
		selectors, err := h.analyzeTestFile(fileReq, projectDir)
		if err != nil {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: 0,
				Skipped:  true,
				Reason:   "Failed to analyze file: " + err.Error(),
			})
			result.FilesSkipped++
			continue
		}

		// Limit selectors per file
		if len(selectors) > MaxSelectorsPerFile {
			result.Warnings = append(result.Warnings, fmt.Sprintf("File %s has %d selectors, limited to %d", filepath.Base(filePath), len(selectors), MaxSelectorsPerFile))
			selectors = selectors[:MaxSelectorsPerFile]
		}

		// Attempt to heal selectors
		repairReq := TestHealRequest{
			Action:          "repair",
			BrokenSelectors: selectors,
			AutoApply:       false, // Never auto-apply in batch mode
		}

		healResult, err := h.repairSelectors(repairReq, projectDir)
		if err != nil {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: len(selectors),
				Skipped:  true,
				Reason:   "Failed to repair selectors: " + err.Error(),
			})
			result.FilesSkipped++
			continue
		}

		// Track results for this file
		fileResult := FileHealResult{
			FilePath: filePath,
			Healed:   len(healResult.Healed),
			Unhealed: len(healResult.Unhealed),
			Skipped:  false,
		}
		result.FileResults = append(result.FileResults, fileResult)

		// Update totals
		result.FilesProcessed++
		result.TotalSelectors += len(selectors)
		result.TotalHealed += len(healResult.Healed)
		result.TotalUnhealed += len(healResult.Unhealed)
	}

	return result, nil
}

// findTestFiles recursively finds test files in a directory
func findTestFiles(dir string) ([]string, error) {
	var testFiles []string
	testPatterns := []string{".spec.ts", ".test.ts", ".spec.js", ".test.js"}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip node_modules and other common non-test directories
			if info.Name() == "node_modules" || info.Name() == ".git" || info.Name() == "dist" || info.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches test patterns
		for _, pattern := range testPatterns {
			if strings.HasSuffix(path, pattern) {
				testFiles = append(testFiles, path)
				break
			}
		}

		return nil
	})

	return testFiles, err
}

// ============================================
// Section 5: test_classify Implementation
// ============================================

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
		Category:   category,
		Confidence: confidence,
		Evidence:   evidence,
		IsRealBug:  category == CategoryRealBug,
		IsFlaky:    category == CategoryTimingFlaky || category == CategoryNetworkFlaky,
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
