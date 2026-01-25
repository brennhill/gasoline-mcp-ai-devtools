// pilot.go — AI Web Pilot feature handlers.
// Implements highlight_element, manage_state, execute_javascript, browser_action.
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

// BrowserActionParams for browser_action tool
type BrowserActionParams struct {
	Action string `json:"action"` // refresh, navigate, back, forward
	URL    string `json:"url"`    // for navigate action
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
		// Timeout - don't assume disabled, report accurately
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Timeout waiting for extension response. Check that the extension is connected and the page is focused."),
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
		// Timeout - don't assume disabled, report accurately
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Timeout waiting for extension response. Check that the extension is connected and the page is focused."),
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
		// Check for pilot disabled error specifically
		if errMsg == "ai_web_pilot_disabled" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpErrorResponse(ErrPilotDisabled.Error()),
			}
		}
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
		// Timeout - don't assume disabled, report accurately
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Timeout waiting for extension response. Check that the extension is connected and the page is focused."),
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpTextResponse(string(result)),
	}
}

// handleBrowserAction handles the browser_action tool call.
// Forwards browser navigation commands to extension via pending queries.
func (h *ToolHandler) handleBrowserAction(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params BrowserActionParams
	_ = json.Unmarshal(args, &params)

	// Validate action parameter
	validActions := map[string]bool{"refresh": true, "navigate": true, "back": true, "forward": true}
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
			Result:  mcpErrorResponse("Invalid action: must be refresh, navigate, back, or forward"),
		}
	}

	// Validate URL for navigate action
	if params.Action == "navigate" && params.URL == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("URL required for navigate action"),
		}
	}

	// Build query params
	queryParams := map[string]interface{}{
		"action": params.Action,
	}
	if params.URL != "" {
		queryParams["url"] = params.URL
	}

	// Send browser action command via pending query mechanism
	queryParamsJSON, _ := json.Marshal(queryParams)
	queryID := h.capture.CreatePendingQueryWithTimeout(PendingQuery{
		Type:   "browser_action",
		Params: queryParamsJSON,
	}, 10*time.Second)

	// Wait for result from extension
	result, err := h.capture.WaitForResult(queryID, 10*time.Second)
	if err != nil {
		// Timeout - don't assume disabled, report accurately
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Timeout waiting for extension response. Check that the extension is connected and the page is focused."),
		}
	}

	// Parse result
	var actionResult map[string]interface{}
	if err := json.Unmarshal(result, &actionResult); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse("Failed to parse browser action result"),
		}
	}

	// Check for error in result
	if errMsg, ok := actionResult["error"].(string); ok && errMsg != "" {
		// Check for pilot disabled error specifically
		if errMsg == "ai_web_pilot_disabled" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpErrorResponse(ErrPilotDisabled.Error()),
			}
		}
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpErrorResponse(errMsg),
		}
	}

	// Return success result
	resultJSON, _ := json.MarshalIndent(actionResult, "", "  ")
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpTextResponse(string(resultJSON)),
	}
}
