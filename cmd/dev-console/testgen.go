// Purpose: Generates Playwright test scripts from captured browser actions and context (error, interaction, regression).
// Why: Converts runtime telemetry into executable test artifacts for reproduction and regression coverage.
// Docs: docs/features/feature/test-generation/index.md

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/testgen"
)

// ============================================
// MCP Entry Point: test_from_context
// ============================================

// testGenContextDispatch maps context values to their generator functions.
var testGenContextDispatch = map[string]func(h *testGenHandler, params TestFromContextRequest) (*GeneratedTest, error){
	"error":       (*testGenHandler).generateTestFromError,
	"interaction": (*testGenHandler).generateTestFromInteraction,
	"regression":  (*testGenHandler).generateTestFromRegression,
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

func (h *testGenHandler) handleGenerateTestFromContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params TestFromContextRequest

	warnings, err := unmarshalWithWarnings(args, &params)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	warnings = filterGenerateDispatchWarnings(warnings)

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

	resp := succeed(req, summary, data)

	return appendWarningsToResponse(resp, warnings)
}

func validateTestFromContextParams(reqID any, params TestFromContextRequest) (JSONRPCResponse, bool) {
	if params.Context == "" {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
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
			JSONRPC: JSONRPCVersion,
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
				JSONRPC: JSONRPCVersion,
				ID:      reqID,
				Result:  mcpStructuredError(m.code, m.message, m.retry, withHint(m.hint)),
			}
		}
	}
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      reqID,
		Result:  mcpStructuredError(ErrInternal, "Failed to generate test: "+err.Error(), "Check the input parameters and ensure captured data is available, then retry"),
	}
}
