// pilot.go — AI Web Pilot feature handlers.
// Implements highlight_element, manage_state, execute_javascript, browser_action.
// All features require human opt-in via extension popup.
// Phase 2: Forwards commands to extension via pending queries.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrPilotDisabled returned when toggle is off
var ErrPilotDisabled = errors.New("ai_web_pilot_disabled: enable 'AI Web Pilot' in extension popup")

// PilotStatusResponse describes the current AI Web Pilot toggle state.
// Includes freshness information to help clients understand data reliability.
type PilotStatusResponse struct {
	Enabled            bool   `json:"enabled"`           // Whether AI Web Pilot is enabled
	Source             string `json:"source"`            // "extension_poll", "stale", or "never_connected"
	ExtensionConnected bool   `json:"extension_connected"` // true if poll < 3 seconds ago
	LastUpdate         string `json:"last_update,omitempty"` // RFC3339 timestamp of last poll
	LastPollAgo        string `json:"last_poll_ago,omitempty"` // Human-readable: "2.3s"
}

// PilotHighlightParams for highlight_element tool
type PilotHighlightParams struct {
	Selector   string `json:"selector"`
	DurationMs int    `json:"duration_ms"`
	TabID      int    `json:"tab_id"` // Target tab ID (0 = active tab)
}

// PilotManageStateParams for manage_state tool
type PilotManageStateParams struct {
	Action       string `json:"action"`
	SnapshotName string `json:"snapshot_name"`
	IncludeUrl   *bool  `json:"include_url,omitempty"`
	TabID        int    `json:"tab_id"` // Target tab ID (0 = active tab)
}

// PilotExecuteJSParams for execute_javascript tool
type PilotExecuteJSParams struct {
	Script    string `json:"script"`
	TimeoutMs int    `json:"timeout_ms"`
	TabID     int    `json:"tab_id"` // Target tab ID (0 = active tab)
}

// BrowserActionParams for browser_action tool
type BrowserActionParams struct {
	Action string `json:"action"` // open, refresh, navigate, back, forward
	URL    string `json:"url"`    // for navigate and open actions
	TabID  int    `json:"tab_id"` // Target tab ID (0 = active tab, not applicable for open)
}

// PluginReadiness indicates the current state of the browser extension and AI Web Pilot toggle.
type PluginReadiness int

const (
	PluginOnPilotEnabled  PluginReadiness = iota // Plugin connected, AI Web Pilot enabled - commands will work
	PluginOff                                     // Plugin not connected or stale - commands will fail
	PluginOnPilotDisabled                         // Plugin connected, AI Web Pilot disabled - commands rejected
)

// PluginReadinessCheck contains the result of checking if plugin is ready for commands.
type PluginReadinessCheck struct {
	State        PluginReadiness    // Current plugin state
	ErrorResp    *JSONRPCResponse   // If non-nil, return this error immediately (reject command)
	Warning      string             // If non-empty, include this warning when sending command
	ShouldAccept bool               // True if command should be attempted (even with warning)
}

// checkPilotReady verifies that AI Web Pilot is enabled and extension is connected.
// Returns a PluginReadinessCheck indicating whether to accept/reject the command.
// Uses most optimistic view: checks both polling and settings channels, attempts command even if stale.
func (h *ToolHandler) checkPilotReady(req JSONRPCRequest) PluginReadinessCheck {
	h.capture.mu.RLock()
	pilotEnabled := h.capture.pilotEnabled
	lastPollAt := h.capture.lastPollAt
	pilotUpdatedAt := h.capture.pilotUpdatedAt
	h.capture.mu.RUnlock()

	now := time.Now()
	pollAge := now.Sub(lastPollAt)
	settingsAge := now.Sub(pilotUpdatedAt)

	// Case 1: Extension never connected (no polling, no settings POST)
	if lastPollAt.IsZero() && pilotUpdatedAt.IsZero() {
		return PluginReadinessCheck{
			State:        PluginOff,
			ShouldAccept: false,
			ErrorResp: &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrExtTimeout,
					"Extension not connected",
					"The browser extension has never connected to the server. Please verify that:\n"+
						"1. The Gasoline extension is installed and enabled in the browser\n"+
						"2. The browser with the extension is running\n"+
						"3. The extension's server URL is configured correctly\n"+
						"4. Check the extension popup to verify connection status",
				),
			},
		}
	}

	// Case 2: Polling is recent (<5s) - extension actively listening for commands
	if pollAge < 5*time.Second {
		if !pilotEnabled {
			return PluginReadinessCheck{
				State:        PluginOnPilotDisabled,
				ShouldAccept: false,
				ErrorResp: &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrCodePilotDisabled,
						"AI Web Pilot is disabled",
						"The extension is connected but AI Web Pilot is turned OFF. "+
							"Open the Gasoline extension popup and enable 'AI Control', then retry this command.",
					),
				},
			}
		}
		// Extension polling actively and pilot enabled - ready to send commands
		return PluginReadinessCheck{
			State:        PluginOnPilotEnabled,
			ShouldAccept: true,
		}
	}

	// Case 3: Settings POST is recent (<5s) but polling is stale
	// Extension is alive but not polling - accept command with warning
	if settingsAge < 5*time.Second {
		if !pilotEnabled {
			return PluginReadinessCheck{
				State:        PluginOnPilotDisabled,
				ShouldAccept: false,
				ErrorResp: &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrCodePilotDisabled,
						"AI Web Pilot is disabled",
						"The extension is connected but AI Web Pilot is turned OFF. "+
							"Open the Gasoline extension popup and enable 'AI Control', then retry this command.",
					),
				},
			}
		}
		// Extension alive (posting settings) but polling is slow - warn LLM but try anyway
		return PluginReadinessCheck{
			State:        PluginOnPilotEnabled,
			ShouldAccept: true,
			Warning: fmt.Sprintf(
				"WARNING: Extension polling is stale (last poll %.1fs ago, but settings POST is active). "+
					"The plugin may be experiencing delays. If this command fails, inform the user that the browser "+
					"extension may need attention or the page may need to be refreshed.",
				pollAge.Seconds(),
			),
		}
	}

	// Case 4: Both polling and settings are stale (>5s) - attempt command but warn about staleness
	// Most optimistic: try anyway, let timeout handling deal with failure
	lastConnection := lastPollAt
	if pilotUpdatedAt.After(lastPollAt) {
		lastConnection = pilotUpdatedAt
	}
	staleAge := now.Sub(lastConnection)

	var staleMessage string
	if lastConnection.IsZero() {
		staleMessage = "Last connection from browser plugin was: never (server may have just started)"
	} else {
		staleMessage = fmt.Sprintf(
			"Last connection from browser plugin was at %s (%.1f seconds ago)",
			lastConnection.Format(time.RFC3339),
			staleAge.Seconds(),
		)
	}

	return PluginReadinessCheck{
		State:        PluginOnPilotEnabled, // Optimistic: assume it will work
		ShouldAccept: true,
		Warning: fmt.Sprintf(
			"WARNING: Extension connection is stale. %s\n"+
				"The command will be attempted, but may fail. If it fails, inform the user to:\n"+
				"1. Check that the browser with Gasoline extension is still running\n"+
				"2. Reload the browser extension if the connection doesn't resume\n"+
				"3. Refresh the web page to re-establish connection\n"+
				"4. Verify the extension popup shows 'Connected'",
			staleMessage,
		),
	}
}

// handlePilotHighlight handles the highlight_element tool call.
// Forwards highlight command to the browser extension via pending query mechanism.
// The extension checks the AI Web Pilot toggle and executes if enabled.
func (h *ToolHandler) handlePilotHighlight(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params PilotHighlightParams
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"),
		}
	}

	// Validate required parameter
	if params.Selector == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrMissingParam, "Required parameter 'selector' is missing", "Add the 'selector' parameter and call again", withParam("selector")),
		}
	}

	// Set default duration if not specified
	if params.DurationMs <= 0 {
		params.DurationMs = 5000
	}

	// Check if plugin is ready to receive commands
	readiness := h.checkPilotReady(req)
	if !readiness.ShouldAccept {
		// Plugin not ready - return error immediately without sending command
		return *readiness.ErrorResp
	}
	// If readiness.Warning is set, we'll include it in the response later

	// Create pending query to send to extension
	// Error impossible: map[string]interface{} with primitive values is always serializable
	queryParams, _ := json.Marshal(map[string]interface{}{
		"selector":    params.Selector,
		"duration_ms": params.DurationMs,
	})

	id := h.capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "highlight",
		Params: queryParams,
		TabID:  params.TabID,
	}, h.capture.queryTimeout, req.ClientID)

	// Wait for extension to execute and return result
	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout, req.ClientID)
	if err != nil {
		// Timeout - don't assume disabled, report accurately
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrExtTimeout, "Timeout waiting for extension response", "Browser extension didn't respond — wait a moment and retry", withHint("Check that the browser extension is connected and a page is focused")),
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
			Result:  mcpStructuredError(ErrExtError, "Invalid response from extension", "Extension returned invalid data — wait a moment and retry"),
		}
	}

	if highlightResult.Error == "ai_web_pilot_disabled" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is not enabled", "Enable AI Web Pilot in the browser extension, then retry"),
		}
	}

	// Return the result as JSON, including warning if connection was questionable
	summary := "Highlight result"
	if readiness.Warning != "" {
		summary = readiness.Warning + "\n\n" + summary
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, json.RawMessage(result)),
	}
}

// handlePilotManageState handles the manage_state tool call.
// Forwards state management commands to extension via pending queries.
func (h *ToolHandler) handlePilotManageState(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params PilotManageStateParams
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"),
		}
	}

	// Validate action parameter
	validActions := map[string]bool{"capture": true, "save": true, "load": true, "list": true, "delete": true}
	if params.Action == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action")),
		}
	}

	if !validActions[params.Action] {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidParam, "Invalid action: must be capture, save, load, list, or delete", "Use a valid action value"),
		}
	}

	// Validate snapshot_name for actions that require it (capture and list don't need it)
	if (params.Action == "save" || params.Action == "load" || params.Action == "delete") && params.SnapshotName == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrMissingParam, fmt.Sprintf("snapshot_name required for %s action", params.Action), "Add the 'snapshot_name' parameter and call again", withParam("snapshot_name")),
		}
	}

	// Check if plugin is ready to receive commands
	readiness := h.checkPilotReady(req)
	if !readiness.ShouldAccept {
		// Plugin not ready - return error immediately without sending command
		return *readiness.ErrorResp
	}
	// If readiness.Warning is set, we'll include it in the response later

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
	// Error impossible: map[string]interface{} with primitive values is always serializable
	queryParamsJSON, _ := json.Marshal(queryParams)
	queryID := h.capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   queryType,
		Params: queryParamsJSON,
		TabID:  params.TabID,
	}, h.capture.queryTimeout, req.ClientID)

	// Wait for result from extension
	result, err := h.capture.WaitForResult(queryID, h.capture.queryTimeout, req.ClientID)
	if err != nil {
		// Timeout - don't assume disabled, report accurately
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrExtTimeout, "Timeout waiting for extension response", "Browser extension didn't respond — wait a moment and retry", withHint("Check that the browser extension is connected and a page is focused")),
		}
	}

	// Parse result
	var stateResult map[string]any
	if err := json.Unmarshal(result, &stateResult); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrExtError, "Invalid response from extension", "Extension returned invalid data — wait a moment and retry"),
		}
	}

	// Check for error in result
	if errMsg, ok := stateResult["error"].(string); ok && errMsg != "" {
		// Check for pilot disabled error specifically
		if errMsg == "ai_web_pilot_disabled" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is not enabled", "Enable AI Web Pilot in the browser extension, then retry"),
			}
		}
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrExtError, errMsg, "Extension returned invalid data — wait a moment and retry"),
		}
	}

	// Return success result, including warning if connection was questionable
	summary := "State operation"
	if readiness.Warning != "" {
		summary = readiness.Warning + "\n\n" + summary
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, stateResult),
	}
}

// handlePilotExecuteJS handles the execute_javascript tool call.
// Creates a pending query for the extension to pick up and execute.
func (h *ToolHandler) handlePilotExecuteJS(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params PilotExecuteJSParams
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"),
		}
	}

	// Validate required parameter
	if params.Script == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrMissingParam, "Required parameter 'script' is missing", "Add the 'script' parameter and call again", withParam("script")),
		}
	}

	// Set default timeout
	timeoutMs := params.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}

	// Check if plugin is ready to receive commands
	readiness := h.checkPilotReady(req)
	if !readiness.ShouldAccept {
		// Plugin not ready - return error immediately without sending command
		return *readiness.ErrorResp
	}
	// If readiness.Warning is set, we'll include it in the response later

	// ASYNC COMMAND EXECUTION (v6.0.0)
	// Generate correlation_id for async command tracking
	correlationID := generateCorrelationID()

	// Create initial CommandResult with pending status
	h.capture.SetCommandResult(CommandResult{
		CorrelationID: correlationID,
		Status:        "pending",
		CreatedAt:     time.Now(),
	})

	// Create a pending query for the extension to execute
	queryParams := map[string]interface{}{
		"script":     params.Script,
		"timeout_ms": timeoutMs,
	}
	// Error impossible: map[string]interface{} with primitive values is always serializable
	paramsJSON, _ := json.Marshal(queryParams)

	h.capture.CreatePendingQueryWithClient(PendingQuery{
		Type:          "execute",
		Params:        paramsJSON,
		TabID:         params.TabID,
		CorrelationID: correlationID, // Pass correlation_id to extension
	}, h.capture.queryTimeout, req.ClientID)

	// Return immediately with correlation_id (server never blocks)
	summary := "Command queued for execution"
	if readiness.Warning != "" {
		summary = readiness.Warning + "\n\n" + summary
	}

	responseData := map[string]interface{}{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        fmt.Sprintf("Command queued. Extension will execute in 1-2s. IMPORTANT: Poll for result every 2 seconds using this EXACT command:\n\nobserve({what: 'command_result', correlation_id: '%s'})\n\nKeep polling until status changes from 'pending' to 'complete' or 'timeout'. Do NOT proceed until you receive the result.", correlationID),
	}
	responseJSON, _ := json.Marshal(responseData)

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, json.RawMessage(responseJSON)),
	}
}

// handleBrowserAction handles the browser_action tool call.
// Forwards browser navigation commands to extension via pending queries.
func (h *ToolHandler) handleBrowserAction(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params BrowserActionParams
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"),
		}
	}

	// Validate action parameter
	validActions := map[string]bool{"open": true, "refresh": true, "navigate": true, "back": true, "forward": true}
	if params.Action == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action")),
		}
	}

	if !validActions[params.Action] {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidParam, "Invalid action: must be open, refresh, navigate, back, or forward", "Use a valid action value"),
		}
	}

	// Validate URL for navigate and open actions
	if (params.Action == "navigate" || params.Action == "open") && params.URL == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrMissingParam, "URL required for "+params.Action+" action", "Add the 'url' parameter and call again", withParam("url")),
		}
	}

	// Check if plugin is ready to receive commands
	readiness := h.checkPilotReady(req)
	if !readiness.ShouldAccept {
		// Plugin not ready - return error immediately without sending command
		return *readiness.ErrorResp
	}
	// If readiness.Warning is set, we'll include it in the response later

	// Build query params
	queryParams := map[string]interface{}{
		"action": params.Action,
	}
	if params.URL != "" {
		queryParams["url"] = params.URL
	}

	// Send browser action command via pending query mechanism
	// Error impossible: map[string]interface{} with primitive values is always serializable
	queryParamsJSON, _ := json.Marshal(queryParams)
	queryID := h.capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "browser_action",
		Params: queryParamsJSON,
		TabID:  params.TabID, // Note: for "open" action, TabID is ignored by extension
	}, h.capture.queryTimeout, req.ClientID)

	// Wait for result from extension
	result, err := h.capture.WaitForResult(queryID, h.capture.queryTimeout, req.ClientID)
	if err != nil {
		// Timeout - don't assume disabled, report accurately
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrExtTimeout, "Timeout waiting for extension response", "Browser extension didn't respond — wait a moment and retry", withHint("Check that the browser extension is connected and a page is focused")),
		}
	}

	// Parse result
	var actionResult map[string]interface{}
	if err := json.Unmarshal(result, &actionResult); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrExtError, "Invalid response from extension", "Extension returned invalid data — wait a moment and retry"),
		}
	}

	// Check for error in result
	if errMsg, ok := actionResult["error"].(string); ok && errMsg != "" {
		// Check for pilot disabled error specifically
		if errMsg == "ai_web_pilot_disabled" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is not enabled", "Enable AI Web Pilot in the browser extension, then retry"),
			}
		}
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrExtError, errMsg, "Extension returned invalid data — wait a moment and retry"),
		}
	}

	// Return success result, including warning if connection was questionable
	summary := "Browser action result"
	if readiness.Warning != "" {
		summary = readiness.Warning + "\n\n" + summary
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, actionResult),
	}
}

// GetPilotStatus returns the current AI Web Pilot toggle state.
// Includes freshness information so clients understand data reliability.
// Uses most optimistic view: checks both polling and settings POST channels.
// Thread-safe: reads state under RLock.
func (c *Capture) GetPilotStatus() PilotStatusResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := PilotStatusResponse{
		Enabled: c.pilotEnabled,
	}

	pollAge := time.Since(c.lastPollAt)
	settingsAge := time.Since(c.pilotUpdatedAt)

	// Determine source and freshness using most optimistic view
	// Check both polling (GET /pending-queries) and settings POST (POST /settings)

	// Case 1: Never connected (neither polling nor settings)
	if c.lastPollAt.IsZero() && c.pilotUpdatedAt.IsZero() {
		status.Source = "never_connected"
		status.ExtensionConnected = false
		return status
	}

	// Case 2: Polling is recent (<5s) - extension actively listening for commands
	if pollAge < 5*time.Second {
		status.Source = "extension_poll"
		status.ExtensionConnected = true
		status.LastUpdate = c.lastPollAt.Format(time.RFC3339)
		status.LastPollAgo = fmt.Sprintf("%.1fs", pollAge.Seconds())
		return status
	}

	// Case 3: Settings POST is recent (<5s) but polling is stale
	// Extension alive but command channel may be slow
	if settingsAge < 5*time.Second {
		status.Source = "settings_heartbeat"
		status.ExtensionConnected = true // Optimistic: extension is alive
		status.LastUpdate = c.pilotUpdatedAt.Format(time.RFC3339)
		status.LastPollAgo = fmt.Sprintf("%.1fs", settingsAge.Seconds())
		return status
	}

	// Case 4: Both stale - use most recent timestamp
	if c.pilotUpdatedAt.After(c.lastPollAt) {
		status.LastUpdate = c.pilotUpdatedAt.Format(time.RFC3339)
		status.LastPollAgo = fmt.Sprintf("%.1fs", settingsAge.Seconds())
	} else if !c.lastPollAt.IsZero() {
		status.LastUpdate = c.lastPollAt.Format(time.RFC3339)
		status.LastPollAgo = fmt.Sprintf("%.1fs", pollAge.Seconds())
	}

	status.Source = "stale"
	status.ExtensionConnected = false

	return status
}

// HandlePilotStatus is the HTTP endpoint for /pilot-status.
// Returns the current AI Web Pilot toggle state (bypasses MCP layer).
// Always returns 200 OK with JSON (never fails) - source field indicates freshness.
func (c *Capture) HandlePilotStatus(w http.ResponseWriter, r *http.Request) {
	status := c.GetPilotStatus()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// toolObservePilot is the observe handler for "pilot" mode.
// Returns the current AI Web Pilot toggle state via the observe tool.
func (h *ToolHandler) toolObservePilot(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	status := h.capture.GetPilotStatus()
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse("Pilot status", status),
	}
}

// toolObservePolling returns the 50 most recent polling requests (GET /pending-queries and POST /settings).
// INTERNAL ONLY — intentionally NOT exposed via MCP. Polling diagnostics are dev-only
// and would confuse end users. Do NOT add "polling" to the observe tool enum in tools.go.
func (h *ToolHandler) toolObservePolling(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	entries := h.capture.GetPollingLog()

	if len(entries) == 0 {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpTextResponse("No polling activity recorded yet"),
		}
	}

	summary := fmt.Sprintf("Polling activity: %d entries (most recent 50)", len(entries))
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, entries),
	}
}

// ============================================
// Interact Tool Action Handlers (Phase 2: unified tool dispatcher)
// ============================================
// These methods are called by the consolidated interact tool dispatcher.
// They repackage arguments and delegate to existing handlers.

// handlePilotManageStateSave delegates to handlePilotManageState with action="save"
func (h *ToolHandler) handlePilotManageStateSave(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	// "save_state" from interact maps to "capture" (save current page state, no name required).
	// The internal "save" action requires snapshot_name; "capture" does not.
	m["action"] = "capture"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handlePilotManageState(req, repackaged)
}

// handlePilotManageStateLoad delegates to handlePilotManageState with action="load"
func (h *ToolHandler) handlePilotManageStateLoad(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	m["action"] = "load"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handlePilotManageState(req, repackaged)
}

// handlePilotManageStateList delegates to handlePilotManageState with action="list"
func (h *ToolHandler) handlePilotManageStateList(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	m["action"] = "list"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handlePilotManageState(req, repackaged)
}

// handlePilotManageStateDelete delegates to handlePilotManageState with action="delete"
func (h *ToolHandler) handlePilotManageStateDelete(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	m["action"] = "delete"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handlePilotManageState(req, repackaged)
}

// handleBrowserActionNavigate delegates to handleBrowserAction with action="navigate"
func (h *ToolHandler) handleBrowserActionNavigate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	m["action"] = "navigate"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handleBrowserAction(req, repackaged)
}

// handleBrowserActionRefresh delegates to handleBrowserAction with action="refresh"
func (h *ToolHandler) handleBrowserActionRefresh(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	m["action"] = "refresh"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handleBrowserAction(req, repackaged)
}

// handleBrowserActionBack delegates to handleBrowserAction with action="back"
func (h *ToolHandler) handleBrowserActionBack(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	m["action"] = "back"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handleBrowserAction(req, repackaged)
}

// handleBrowserActionForward delegates to handleBrowserAction with action="forward"
func (h *ToolHandler) handleBrowserActionForward(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	m["action"] = "forward"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handleBrowserAction(req, repackaged)
}

// handleBrowserActionNewTab delegates to handleBrowserAction with action="open"
func (h *ToolHandler) handleBrowserActionNewTab(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var m map[string]interface{}
	json.Unmarshal(args, &m)
	m["action"] = "open"
	// Error impossible: map[string]interface{} with primitive values is always serializable
	repackaged, _ := json.Marshal(m)
	return h.handleBrowserAction(req, repackaged)
}
