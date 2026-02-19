// testgen_classify.go — MCP glue layer for test failure classification.
// Pure logic lives in internal/testgen; this file owns the MCP dispatch.
package main

import (
	"encoding/json"
	"fmt"
)

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
