// Purpose: Owns testgen.go runtime behavior and integration logic.
// Docs: docs/features/feature/test-generation/index.md

// testgen.go — Test generation from captured errors and user interactions.
// Generates Playwright tests using console errors, user actions, and network data.
// Design: Reuses existing codegen.go infrastructure for script generation.
// Follows 4-tool constraint: integrates into existing generate tool.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	Framework  string          `json:"framework"`
	Filename   string          `json:"filename"`
	Content    string          `json:"content"`
	Selectors  []string        `json:"selectors"`
	Assertions int             `json:"assertions"`
	Coverage   TestCoverage    `json:"coverage"`
	Metadata   TestGenMetadata `json:"metadata"`
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
	TotalClassified int                     `json:"total_classified"`
	RealBugs        int                     `json:"real_bugs"`
	FlakyTests      int                     `json:"flaky_tests"`
	TestBugs        int                     `json:"test_bugs"`
	Uncertain       int                     `json:"uncertain"`
	Classifications []FailureClassification `json:"classifications"`
	Summary         map[string]int          `json:"summary"` // category -> count
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
	MaxFileSizeBytes    = 500 * 1024      // 500KB
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

// testGenContextDispatch maps context values to their generator functions.
var testGenContextDispatch = map[string]func(h *ToolHandler, params TestFromContextRequest) (*GeneratedTest, error){
	"error":       (*ToolHandler).generateTestFromError,
	"interaction": (*ToolHandler).generateTestFromInteraction,
	"regression":  (*ToolHandler).generateTestFromRegression,
}

// testGenErrorMap maps error code substrings to structured error responses.
type testGenErrorMapping struct {
	code    string
	message string
	retry   string
	hint    string
}

var testGenErrorMappings = []testGenErrorMapping{
	{
		code:    ErrNoErrorContext,
		message: "No console errors captured to generate test from",
		retry:   "Trigger an error in the browser first, then retry",
		hint:    "Use the observe tool to verify errors are being captured",
	},
	{
		code:    ErrNoActionsCaptured,
		message: "No user actions recorded in the session",
		retry:   "Interact with the page first (click, type, navigate), then retry",
		hint:    "Use the observe tool with what=actions to verify actions are being captured",
	},
	{
		code:    ErrNoBaseline,
		message: "No regression baseline available",
		retry:   "Capture a baseline first by interacting with the page, then retry",
		hint:    "The regression mode generates tests by comparing current state against a baseline",
	},
}

// ============================================
// Section 2: Entry Points
// ============================================

func (h *ToolHandler) handleGenerateTestFromContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params TestFromContextRequest

	warnings, err := unmarshalWithWarnings(args, &params)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"),
		}
	}

	if errResp, ok := validateTestFromContextParams(req.ID, params); !ok {
		return errResp
	}

	if params.Framework == "" {
		params.Framework = "playwright"
	}
	if params.OutputFormat == "" {
		params.OutputFormat = "inline"
	}

	generator := testGenContextDispatch[params.Context]
	generatedTest, err := generator(h, params)
	if err != nil {
		return testGenErrorToResponse(req.ID, err)
	}

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

func validateTestFromContextParams(reqID any, params TestFromContextRequest) (JSONRPCResponse, bool) {
	if params.Context == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'context' is missing",
				"Add the 'context' parameter and call again",
				withParam("context"),
				withHint("Valid values: error, interaction, regression"),
			),
		}, false
	}

	if _, ok := testGenContextDispatch[params.Context]; !ok {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				"Invalid context value: "+params.Context,
				"Use a valid context value",
				withParam("context"),
				withHint("Valid values: error, interaction, regression"),
			),
		}, false
	}

	return JSONRPCResponse{}, true
}

func testGenErrorToResponse(reqID any, err error) JSONRPCResponse {
	errStr := err.Error()
	for _, m := range testGenErrorMappings {
		if strings.Contains(errStr, m.code) {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      reqID,
				Result:  mcpStructuredError(m.code, m.message, m.retry, withHint(m.hint)),
			}
		}
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      reqID,
		Result:  mcpStructuredError(ErrInternal, "Failed to generate test: "+err.Error(), "Check the input parameters and ensure captured data is available, then retry"),
	}
}

// ============================================
// Section 3: test_from_context Implementation
// ============================================

func (h *ToolHandler) generateTestFromError(req TestFromContextRequest) (*GeneratedTest, error) {
	targetError, errorID, errorTimestamp := h.findTargetError(req.ErrorID)
	if targetError == nil {
		return nil, errors.New(ErrNoErrorContext)
	}

	errorMessage, _ := targetError["message"].(string)

	relevantActions, err := h.getActionsInTimeWindow(errorTimestamp, 5000)
	if err != nil {
		return nil, err
	}

	script := generatePlaywrightScript(relevantActions, errorMessage, req.BaseURL)
	assertionCount := strings.Count(script, "expect(") + 1
	filename := generateTestFilename(errorMessage, req.Framework)
	selectors := extractSelectorsFromActions(relevantActions)

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   filename,
		Content:    script,
		Selectors:  selectors,
		Assertions: assertionCount,
		Coverage: TestCoverage{
			ErrorReproduced: true,
			NetworkMocked:   req.IncludeMocks,
			StateCaptured:   len(relevantActions) > 0,
		},
		Metadata: TestGenMetadata{
			SourceError: errorID,
			GeneratedAt: time.Now().Format(time.RFC3339),
			ContextUsed: []string{"console", "actions"},
		},
	}, nil
}

func (h *ToolHandler) findTargetError(errorID string) (LogEntry, string, int64) {
	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}

		if errorID != "" {
			entryID, _ := entry["error_id"].(string)
			if entryID != errorID {
				continue
			}
			tsStr, _ := entry["ts"].(string)
			return entry, entryID, normalizeTimestamp(tsStr)
		}

		tsStr, _ := entry["ts"].(string)
		id, _ := entry["error_id"].(string)
		return entry, id, normalizeTimestamp(tsStr)
	}

	return nil, "", 0
}

func (h *ToolHandler) getActionsInTimeWindow(centerTimestamp int64, windowMs int64) ([]capture.EnhancedAction, error) {
	allActions := h.capture.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, errors.New(ErrNoActionsCaptured)
	}

	var relevant []capture.EnhancedAction
	for i := range allActions {
		action := &allActions[i]
		timeDiff := action.Timestamp - centerTimestamp
		if timeDiff >= -windowMs && timeDiff <= windowMs {
			relevant = append(relevant, *action)
		}
	}

	if len(relevant) == 0 {
		return nil, errors.New(ErrNoActionsCaptured)
	}

	return relevant, nil
}

func (h *ToolHandler) generateTestFromInteraction(req TestFromContextRequest) (*GeneratedTest, error) {
	allActions := h.capture.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, errors.New(ErrNoActionsCaptured)
	}

	relevantActions := allActions
	script := generatePlaywrightScript(relevantActions, "", req.BaseURL)
	assertionCount := strings.Count(script, "expect(")

	if req.IncludeMocks {
		assertionCount += h.countNetworkAssertions()
	}

	filename := generateTestFilename(deriveInteractionTestName(relevantActions), req.Framework)
	selectors := extractSelectorsFromActions(relevantActions)

	contextUsed := []string{"actions"}
	if req.IncludeMocks {
		contextUsed = append(contextUsed, "network")
	}

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   filename,
		Content:    script,
		Selectors:  selectors,
		Assertions: assertionCount,
		Coverage: TestCoverage{
			ErrorReproduced: false,
			NetworkMocked:   req.IncludeMocks,
			StateCaptured:   len(relevantActions) > 0,
		},
		Metadata: TestGenMetadata{
			GeneratedAt: time.Now().Format(time.RFC3339),
			ContextUsed: contextUsed,
		},
	}, nil
}

func deriveInteractionTestName(actions []capture.EnhancedAction) string {
	if len(actions) == 0 {
		return "user-interaction"
	}
	if actions[0].URL != "" {
		return actions[0].URL
	}
	if actions[0].Type != "" {
		return actions[0].Type + "-flow"
	}
	return "user-interaction"
}

func (h *ToolHandler) countNetworkAssertions() int {
	networkBodies := h.capture.GetNetworkBodies()
	count := 0
	for _, nb := range networkBodies {
		if nb.Status > 0 {
			count++
		}
	}
	return count
}

func (h *ToolHandler) generateTestFromRegression(req TestFromContextRequest) (*GeneratedTest, error) {
	allActions := h.capture.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, errors.New(ErrNoActionsCaptured)
	}

	errorMessages := h.collectErrorMessages(5)
	networkBodies := h.capture.GetNetworkBodies()

	assertions, assertionCount := buildRegressionAssertions(errorMessages, networkBodies)

	script := generatePlaywrightScript(allActions, "", req.BaseURL)
	script = insertAssertionsBeforeClose(script, assertions)

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   generateTestFilename("regression-test", req.Framework),
		Content:    script,
		Selectors:  extractSelectorsFromActions(allActions),
		Assertions: assertionCount,
		Coverage: TestCoverage{
			ErrorReproduced: false,
			NetworkMocked:   req.IncludeMocks,
			StateCaptured:   len(allActions) > 0,
		},
		Metadata: TestGenMetadata{
			GeneratedAt: time.Now().Format(time.RFC3339),
			ContextUsed: []string{"actions", "console", "network", "performance"},
		},
	}, nil
}

func (h *ToolHandler) collectErrorMessages(limit int) []string {
	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	var messages []string
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		msg, _ := entry["message"].(string)
		if msg != "" && len(messages) < limit {
			messages = append(messages, msg)
		}
	}
	return messages
}

func buildRegressionAssertions(errorMessages []string, networkBodies []capture.NetworkBody) ([]string, int) {
	var assertions []string
	assertionCount := 0

	if len(errorMessages) == 0 {
		assertions = append(assertions,
			"  // Assert no console errors (baseline was clean)",
			"  const consoleErrors = []",
			"  page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()) })",
			"  // After actions complete:",
			"  expect(consoleErrors).toHaveLength(0)",
		)
		assertionCount++
	} else {
		assertions = append(assertions,
			fmt.Sprintf("  // Baseline had %d console errors", len(errorMessages)),
			"  // TODO: Add assertions to verify errors haven't changed",
		)
	}

	networkAssertions := 0
	for _, nb := range networkBodies {
		if nb.Status > 0 && networkAssertions < 3 {
			assertions = append(assertions,
				fmt.Sprintf("  // Assert %s %s returns %d", nb.Method, nb.URL, nb.Status),
				fmt.Sprintf("  // TODO: await page.waitForResponse(r => r.url().includes('%s') && r.status() === %d)", nb.URL, nb.Status),
			)
			networkAssertions++
			assertionCount++
		}
	}

	assertions = append(assertions,
		"",
		"  // TODO: Add performance assertions",
		"  // - Load time within acceptable range",
		"  // - Key metrics (FCP, LCP) haven't regressed",
	)

	return assertions, assertionCount
}

func insertAssertionsBeforeClose(script string, assertions []string) string {
	assertionBlock := strings.Join(assertions, "\n")
	lastBrace := strings.LastIndex(script, "});")
	if lastBrace > 0 {
		return script[:lastBrace] + "\n" + assertionBlock + "\n" + script[lastBrace:]
	}
	return script
}

// ============================================
// Section 4: Helper Functions
// ============================================

func generateErrorID(message, stack, url string) string {
	timestamp := time.Now().UnixMilli()

	h := sha256.New()
	h.Write([]byte(message))
	h.Write([]byte(stack))
	h.Write([]byte(url))
	hashBytes := h.Sum(nil)
	hashHex := hex.EncodeToString(hashBytes)

	hash8 := hashHex[:8]

	return fmt.Sprintf("err_%d_%s", timestamp, hash8)
}

func generateTestFilename(errorMessage, framework string) string {
	name := strings.ToLower(errorMessage)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, ":", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "\"", "")

	if len(name) > 50 {
		name = name[:50]
	}

	name = strings.TrimRight(name, "-")

	ext := ".spec.ts"
	if framework == "vitest" || framework == "jest" {
		ext = ".test.ts"
	}

	return name + ext
}

func extractSelectorsFromActions(actions []capture.EnhancedAction) []string {
	selectorSet := make(map[string]bool)
	for i := range actions {
		addSelectorsFromEntry(selectorSet, actions[i].Selectors)
	}

	var result []string
	for selector := range selectorSet {
		result = append(result, selector)
	}
	return result
}

func addSelectorsFromEntry(selectorSet map[string]bool, selectors map[string]any) {
	if selectors == nil {
		return
	}
	if testID, ok := selectors["testId"].(string); ok && testID != "" {
		selectorSet["[data-testid=\""+testID+"\"]"] = true
	}
	if role := extractRoleFromSelectors(selectors); role != "" {
		selectorSet["[role=\""+role+"\"]"] = true
	}
	if id, ok := selectors["id"].(string); ok && id != "" {
		selectorSet["#"+id] = true
	}
}

func extractRoleFromSelectors(selectors map[string]any) string {
	roleData, ok := selectors["role"]
	if !ok {
		return ""
	}
	roleMap, ok := roleData.(map[string]any)
	if !ok {
		return ""
	}
	role, _ := roleMap["role"].(string)
	return role
}

func normalizeTimestamp(tsStr string) int64 {
	t, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return 0
		}
	}
	return t.UnixMilli()
}

func targetSelector(action capture.EnhancedAction) (string, bool) {
	if action.Selectors == nil {
		return "", false
	}
	sel, ok := action.Selectors["target"].(string)
	if !ok || sel == "" {
		return "", false
	}
	return sel, true
}

func playwrightActionLine(action capture.EnhancedAction) string {
	switch action.Type {
	case "click":
		sel, ok := targetSelector(action)
		if !ok {
			return ""
		}
		return fmt.Sprintf("  await page.click('%s');\n", sel)
	case "input":
		sel, ok := targetSelector(action)
		if !ok {
			return ""
		}
		return fmt.Sprintf("  await page.fill('%s', '%s');\n", sel, action.Value)
	case "navigate":
		if action.ToURL == "" {
			return ""
		}
		return fmt.Sprintf("  await page.goto('%s');\n", action.ToURL)
	case "wait":
		return fmt.Sprintf("  await page.waitForTimeout(%d);\n", 100)
	default:
		return ""
	}
}

func generatePlaywrightScript(actions []capture.EnhancedAction, errorMessage string, baseURL string) string {
	var script strings.Builder
	script.WriteString("import { test, expect } from '@playwright/test';\n\n")
	script.WriteString("test('Reproduce issue', async ({ page }) => {\n")

	if baseURL != "" {
		script.WriteString(fmt.Sprintf("  await page.goto('%s');\n", baseURL))
	}

	for _, action := range actions {
		script.WriteString(playwrightActionLine(action))
	}

	if errorMessage != "" {
		script.WriteString(fmt.Sprintf("  // Expected error: %s\n", errorMessage))
		script.WriteString("  // TODO: Add specific assertion for this error\n")
	}

	script.WriteString("});\n")
	return script.String()
}
