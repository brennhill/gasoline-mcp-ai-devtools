// testgen_heal.go — MCP glue layer for test healing: analyze, repair, and batch.
// Pure logic lives in internal/testgen; this file owns the MCP dispatch.
package main

import (
	"encoding/json"
	"os"
	"strings"
)

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

	if errResp, ok := validateHealParams(req, params); ok {
		return errResp
	}

	projectDir, _ := os.Getwd()

	result, errResp, ok := h.dispatchHealAction(req, params, projectDir)
	if ok {
		return errResp
	}

	summary := formatHealSummary(params, result)
	data := map[string]any{"result": result}
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, data),
	}
	return appendWarningsToResponse(resp, warnings)
}

var validHealActions = map[string]bool{"analyze": true, "repair": true, "batch": true}

func validateHealParams(req JSONRPCRequest, params TestHealRequest) (JSONRPCResponse, bool) {
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
		}, true
	}
	if !validHealActions[params.Action] {
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
		}, true
	}
	return JSONRPCResponse{}, false
}

func (h *ToolHandler) dispatchHealAction(req JSONRPCRequest, params TestHealRequest, projectDir string) (any, JSONRPCResponse, bool) {
	switch params.Action {
	case "analyze":
		return h.handleHealAnalyze(req, params, projectDir)
	case "repair":
		return h.handleHealRepair(req, params, projectDir)
	case "batch":
		return h.handleHealBatch(req, params, projectDir)
	}
	return nil, JSONRPCResponse{}, false
}

func (h *ToolHandler) handleHealAnalyze(req JSONRPCRequest, params TestHealRequest, projectDir string) (any, JSONRPCResponse, bool) {
	if params.TestFile == "" {
		return nil, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'test_file' is missing for analyze action",
				"Add the 'test_file' parameter and call again",
				withParam("test_file"),
			),
		}, true
	}
	selectors, err := h.analyzeTestFile(params, projectDir)
	if err != nil {
		return nil, mapAnalyzeError(req, params, err), true
	}
	result := map[string]any{
		"broken_selectors": selectors,
		"count":            len(selectors),
	}
	return result, JSONRPCResponse{}, false
}

func mapAnalyzeError(req JSONRPCRequest, params TestHealRequest, err error) JSONRPCResponse {
	errMsg := err.Error()
	if strings.Contains(errMsg, ErrTestFileNotFound) {
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
	if strings.Contains(errMsg, ErrPathNotAllowed) {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrPathNotAllowed,
				errMsg,
				"Use a path within the project directory",
			),
		}
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpStructuredError(ErrInternal, "Failed to analyze test file: "+errMsg, "Check that the test file path is valid and readable"),
	}
}

func (h *ToolHandler) handleHealRepair(req JSONRPCRequest, params TestHealRequest, projectDir string) (any, JSONRPCResponse, bool) {
	if len(params.BrokenSelectors) == 0 {
		return nil, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'broken_selectors' is missing for repair action",
				"Add the 'broken_selectors' parameter and call again",
				withParam("broken_selectors"),
			),
		}, true
	}
	healResult, err := h.repairSelectors(params, projectDir)
	if err != nil {
		return nil, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInternal, "Failed to repair selectors: "+err.Error(), "Check the broken_selectors input and retry"),
		}, true
	}
	return healResult, JSONRPCResponse{}, false
}

func (h *ToolHandler) handleHealBatch(req JSONRPCRequest, params TestHealRequest, projectDir string) (any, JSONRPCResponse, bool) {
	if params.TestDir == "" {
		return nil, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'test_dir' is missing for batch action",
				"Add the 'test_dir' parameter and call again",
				withParam("test_dir"),
			),
		}, true
	}
	batchResult, err := h.healTestBatch(params, projectDir)
	if err != nil {
		return nil, mapBatchError(req, err), true
	}
	return batchResult, JSONRPCResponse{}, false
}

func mapBatchError(req JSONRPCRequest, err error) JSONRPCResponse {
	errMsg := err.Error()
	if strings.Contains(errMsg, ErrPathNotAllowed) {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrPathNotAllowed,
				errMsg,
				"Use a path within the project directory",
			),
		}
	}
	if strings.Contains(errMsg, ErrBatchTooLarge) {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrBatchTooLarge,
				errMsg,
				"Reduce the number or size of test files",
			),
		}
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpStructuredError(ErrInternal, "Failed to heal test batch: "+errMsg, "Check the test_dir path and file permissions, then retry"),
	}
}
