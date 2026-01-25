// pilot.go — AI Web Pilot feature handlers.
// Implements highlight_element, manage_state, execute_javascript.
// All features require human opt-in via extension popup.
// Phase 2: Forwards commands to extension via pending queries.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
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
	IncludeUrl   *bool  `json:"include_url,omitempty"`
}

// PilotExecuteJSParams for execute_javascript tool
type PilotExecuteJSParams struct {
	Script    string `json:"script"`
	TimeoutMs int    `json:"timeout_ms"`
}

// handlePilotHighlight handles the highlight_element tool call.
// Forwards highlight command to the browser extension via pending query mechanism.
// The extension checks the AI Web Pilot toggle and executes if enabled.
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

	// Set default duration if not specified
	if params.DurationMs <= 0 {
		params.DurationMs = 5000
	}

	// Create pending query to send to extension
	queryParams, _ := json.Marshal(map[string]interface{}{
		"selector":    params.Selector,
		"duration_ms": params.DurationMs,
	})

	id := h.capture.CreatePendingQuery(PendingQuery{
		Type:   "highlight",
		Params: queryParams,
	})

	// Wait for extension to execute and return result
	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout)
	if err != nil {
		// Timeout or no extension connected — likely pilot not enabled
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse(ErrPilotDisabled.Error()),
		}
	}

	// Parse the result to check for success/error
	var highlightResult struct {
		Success  bool   `json:"success"`
		Error    string `json:"error,omitempty"`
		Selector string `json:"selector,omitempty"`
		Bounds   struct {
			X      float64 `json:"x"`
			Y      float64 `json:"y"`
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		} `json:"bounds,omitempty"`
	}

	if err := json.Unmarshal(result, &highlightResult); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Invalid response from extension"),
		}
	}

	// Check for pilot disabled error from extension
	if highlightResult.Error == "ai_web_pilot_disabled" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse(ErrPilotDisabled.Error()),
		}
	}

	// Return the result as JSON
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpTextResponse(string(result)),
	}
}

// handlePilotManageState handles the manage_state tool call.
// Forwards state management commands to extension via pending queries.
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

	// Validate snapshot_name for actions that require it
	if (params.Action == "save" || params.Action == "load" || params.Action == "delete") && params.SnapshotName == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse(fmt.Sprintf("snapshot_name required for %s action", params.Action)),
		}
	}

	// Build query params
	includeUrl := true
	if params.IncludeUrl != nil {
		includeUrl = *params.IncludeUrl
	}

	queryParams := map[string]any{
		"action": params.Action,
	}
	if params.SnapshotName != "" {
		queryParams["name"] = params.SnapshotName
	}
	if params.Action == "load" {
		queryParams["include_url"] = includeUrl
	}

	// Determine query type based on action
	queryType := "state_" + params.Action

	// Send pilot command via pending query mechanism
	queryParamsJSON, _ := json.Marshal(queryParams)
	queryID := h.capture.CreatePendingQueryWithTimeout(PendingQuery{
		Type:   queryType,
		Params: queryParamsJSON,
	}, 10*time.Second)

	// Wait for result from extension
	result, err := h.capture.WaitForResult(queryID, 10*time.Second)
	if err != nil {
		// Check if timeout due to pilot being disabled
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse(ErrPilotDisabled.Error()),
		}
	}

	// Parse result
	var stateResult map[string]any
	if err := json.Unmarshal(result, &stateResult); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Failed to parse state result"),
		}
	}

	// Check for error in result
	if errMsg, ok := stateResult["error"].(string); ok && errMsg != "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse(errMsg),
		}
	}

	// Return success result
	resultJSON, _ := json.MarshalIndent(stateResult, "", "  ")
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpTextResponse(string(resultJSON)),
	}
}

// handlePilotExecuteJS handles the execute_javascript tool call.
// Phase 1: Returns not enabled error until toggle and handlers are implemented.
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

	// Phase 1: Always return not enabled error
	// Phase 2 will check extension toggle and execute in sandboxed context
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpErrorResponse(ErrPilotDisabled.Error()),
	}
}
