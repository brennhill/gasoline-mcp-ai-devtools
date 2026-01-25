// pilot.go — AI Web Pilot feature handlers.
// Implements highlight_element, manage_state, execute_javascript.
// All features require human opt-in via extension popup.
// Phase 1: Stubs only — returns "not enabled" until Phase 2 agents implement handlers.
package main

import (
	"encoding/json"
	"errors"
)

// ErrPilotDisabled returned when toggle is off
var ErrPilotDisabled = errors.New("ai_web_pilot_disabled: enable 'AI Web Pilot' in extension popup")

// PilotHighlightParams for highlight_element tool
type PilotHighlightParams struct {
	Selector   string `json:"selector"`
	DurationMs int    `json:"duration_ms"`
}

// PilotManageStateParams for manage_state tool
type PilotManageStateParams struct {
	Action       string `json:"action"`
	SnapshotName string `json:"snapshot_name"`
}

// PilotExecuteJSParams for execute_javascript tool
type PilotExecuteJSParams struct {
	Script    string `json:"script"`
	TimeoutMs int    `json:"timeout_ms"`
}

// handlePilotHighlight handles the highlight_element tool call.
// Phase 1: Returns not enabled error until toggle and handlers are implemented.
func (h *ToolHandler) handlePilotHighlight(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params PilotHighlightParams
	_ = json.Unmarshal(args, &params)

	// Validate required parameter
	if params.Selector == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Required parameter 'selector' is missing"),
		}
	}

	// Phase 1: Always return not enabled error
	// Phase 2 will check extension toggle and forward to content script
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpErrorResponse(ErrPilotDisabled.Error()),
	}
}

// handlePilotManageState handles the manage_state tool call.
// Phase 1: Returns not enabled error until toggle and handlers are implemented.
func (h *ToolHandler) handlePilotManageState(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params PilotManageStateParams
	_ = json.Unmarshal(args, &params)

	// Validate action parameter
	validActions := map[string]bool{"save": true, "load": true, "list": true, "delete": true}
	if params.Action == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Required parameter 'action' is missing"),
		}
	}

	if !validActions[params.Action] {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Invalid action: must be save, load, list, or delete"),
		}
	}

	// Phase 1: Always return not enabled error
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpErrorResponse(ErrPilotDisabled.Error()),
	}
}

// handlePilotExecuteJS handles the execute_javascript tool call.
// Creates a pending query for the extension to pick up and execute.
func (h *ToolHandler) handlePilotExecuteJS(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params PilotExecuteJSParams
	_ = json.Unmarshal(args, &params)

	// Validate required parameter
	if params.Script == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Required parameter 'script' is missing"),
		}
	}

	// Set default timeout
	timeoutMs := params.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}

	// Create a pending query for the extension to execute
	queryParams := map[string]interface{}{
		"script":     params.Script,
		"timeout_ms": timeoutMs,
	}
	paramsJSON, _ := json.Marshal(queryParams)

	id := h.capture.CreatePendingQuery(PendingQuery{
		Type:   "execute",
		Params: paramsJSON,
	})

	// Wait for the result from the extension
	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Timeout waiting for script execution. Is the browser extension connected and AI Web Pilot enabled?"),
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpTextResponse(string(result)),
	}
}
