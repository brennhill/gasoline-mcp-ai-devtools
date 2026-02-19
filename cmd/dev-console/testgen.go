// testgen.go — Test generation MCP glue layer.
// Pure logic lives in internal/testgen; this file provides type aliases,
// the ToolHandler-to-DataProvider adapter, and MCP entry points.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/testgen"
)

// ============================================
// Type Aliases
// ============================================

type TestFromContextRequest = testgen.TestFromContextRequest
type GeneratedTest = testgen.GeneratedTest
type TestCoverage = testgen.TestCoverage
type TestGenMetadata = testgen.TestGenMetadata
type TestHealRequest = testgen.TestHealRequest
type HealedSelector = testgen.HealedSelector
type HealResult = testgen.HealResult
type HealSummary = testgen.HealSummary
type BatchHealResult = testgen.BatchHealResult
type FileHealResult = testgen.FileHealResult
type TestClassifyRequest = testgen.TestClassifyRequest
type TestFailure = testgen.TestFailure
type FailureClassification = testgen.FailureClassification
type SuggestedFix = testgen.SuggestedFix
type BatchClassifyResult = testgen.BatchClassifyResult

// Constant aliases
const (
	ErrNoErrorContext          = testgen.ErrNoErrorContext
	ErrNoActionsCaptured       = testgen.ErrNoActionsCaptured
	ErrNoBaseline              = testgen.ErrNoBaseline
	ErrTestFileNotFound        = testgen.ErrTestFileNotFound
	ErrSelectorInjection       = testgen.ErrSelectorInjection
	ErrInvalidSelectorSyntax   = testgen.ErrInvalidSelectorSyntax
	ErrClassificationUncertain = testgen.ErrClassificationUncertain
	ErrBatchTooLarge           = testgen.ErrBatchTooLarge
)

const (
	MaxFilesPerBatch    = testgen.MaxFilesPerBatch
	MaxFileSizeBytes    = testgen.MaxFileSizeBytes
	MaxTotalBatchSize   = testgen.MaxTotalBatchSize
	MaxSelectorsPerFile = testgen.MaxSelectorsPerFile
)

const (
	CategorySelectorBroken = testgen.CategorySelectorBroken
	CategoryTimingFlaky    = testgen.CategoryTimingFlaky
	CategoryNetworkFlaky   = testgen.CategoryNetworkFlaky
	CategoryRealBug        = testgen.CategoryRealBug
	CategoryTestBug        = testgen.CategoryTestBug
	CategoryUnknown        = testgen.CategoryUnknown
)

const maxFailuresPerBatch = testgen.MaxFailuresPerBatch

// Function aliases for pure helpers
var (
	generateErrorID              = testgen.GenerateErrorID
	generateTestFilename         = testgen.GenerateTestFilename
	extractSelectorsFromActions  = testgen.ExtractSelectorsFromActions
	normalizeTimestamp           = testgen.NormalizeTimestamp
	targetSelector               = testgen.TargetSelector
	playwrightActionLine         = testgen.PlaywrightActionLine
	generatePlaywrightScript     = testgen.GeneratePlaywrightScript
	deriveInteractionTestName    = testgen.DeriveInteractionTestName
	buildRegressionAssertions    = testgen.BuildRegressionAssertions
	insertAssertionsBeforeClose  = testgen.InsertAssertionsBeforeClose
	matchClassificationPattern   = testgen.MatchClassificationPattern
	generateSuggestedFix         = testgen.GenerateSuggestedFix
	validateTestFilePath         = testgen.ValidateTestFilePath
	resolveTestPath              = testgen.ResolveTestPath
	containsDangerousPattern     = testgen.ContainsDangerousPattern
	validateSelector             = testgen.ValidateSelector
	extractSelectorsFromTestFile = testgen.ExtractSelectorsFromTestFile
	findTestFiles                = testgen.FindTestFiles
	formatHealSummary            = testgen.FormatHealSummary
	classifyHealedSelector       = testgen.ClassifyHealedSelector
	isTestFile                   = testgen.IsTestFile
)

// ============================================
// DataProvider Adapter
// ============================================

// toolHandlerDataProvider adapts *ToolHandler to testgen.DataProvider.
type toolHandlerDataProvider struct {
	h *ToolHandler
}

func (a *toolHandlerDataProvider) GetLogEntries() []map[string]any {
	a.h.server.mu.RLock()
	entries := make([]LogEntry, len(a.h.server.entries))
	copy(entries, a.h.server.entries)
	a.h.server.mu.RUnlock()
	return entries
}

func (a *toolHandlerDataProvider) GetAllEnhancedActions() []capture.EnhancedAction {
	return a.h.capture.GetAllEnhancedActions()
}

func (a *toolHandlerDataProvider) GetNetworkBodies() []capture.NetworkBody {
	return a.h.capture.GetNetworkBodies()
}

// dataProvider returns a testgen.DataProvider backed by this ToolHandler.
func (h *ToolHandler) dataProvider() testgen.DataProvider {
	return &toolHandlerDataProvider{h: h}
}

// ============================================
// ToolHandler Method Wrappers
// ============================================

func (h *ToolHandler) findTargetError(errorID string) (LogEntry, string, int64) {
	return testgen.FindTargetError(h.dataProvider(), errorID)
}

func (h *ToolHandler) getActionsInTimeWindow(centerTimestamp int64, windowMs int64) ([]capture.EnhancedAction, error) {
	return testgen.GetActionsInTimeWindow(h.dataProvider(), centerTimestamp, windowMs)
}

func (h *ToolHandler) countNetworkAssertions() int {
	return testgen.CountNetworkAssertions(h.dataProvider())
}

func (h *ToolHandler) collectErrorMessages(limit int) []string {
	return testgen.CollectErrorMessages(h.dataProvider(), limit)
}

func (h *ToolHandler) generateTestFromError(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromError(h.dataProvider(), req)
}

func (h *ToolHandler) generateTestFromInteraction(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromInteraction(h.dataProvider(), req)
}

func (h *ToolHandler) generateTestFromRegression(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromRegression(h.dataProvider(), req)
}

func (h *ToolHandler) analyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	return testgen.AnalyzeTestFile(req, projectDir)
}

func (h *ToolHandler) repairSelectors(req TestHealRequest, _ string) (*HealResult, error) {
	return testgen.RepairSelectors(req)
}

func (h *ToolHandler) healSelector(oldSelector string) (*HealedSelector, error) {
	return testgen.HealSelector(oldSelector)
}

func (h *ToolHandler) healTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	return testgen.HealTestBatch(req, projectDir)
}

func (h *ToolHandler) classifyFailure(failure *TestFailure) *FailureClassification {
	return testgen.ClassifyFailure(failure)
}

func (h *ToolHandler) classifyFailureBatch(failures []TestFailure) *BatchClassifyResult {
	return testgen.ClassifyFailureBatch(failures)
}

// ============================================
// MCP Entry Point: test_from_context
// ============================================

// testGenContextDispatch maps context values to their generator functions.
var testGenContextDispatch = map[string]func(h *ToolHandler, params TestFromContextRequest) (*GeneratedTest, error){
	"error":       (*ToolHandler).generateTestFromError,
	"interaction": (*ToolHandler).generateTestFromInteraction,
	"regression":  (*ToolHandler).generateTestFromRegression,
}

// testGenErrorMapping type for MCP error responses.
type testGenErrorMapping struct {
	code    string
	message string
	retry   string
	hint    string
}

var testGenErrorMappings []testGenErrorMapping

func init() {
	for _, m := range testgen.ErrorMappings {
		testGenErrorMappings = append(testGenErrorMappings, testGenErrorMapping{
			code: m.Code, message: m.Message, retry: m.Retry, hint: m.Hint,
		})
	}
}

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
