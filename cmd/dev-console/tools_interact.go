// tools_interact.go — MCP interact tool dispatcher and handlers.
// Handles all browser interaction actions: navigate, execute_js, highlight, state management, etc.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/queries"
)

// recordAIAction records an AI-driven action to the enhanced actions buffer.
// This allows distinguishing AI actions from human actions in observe(actions).
func (h *ToolHandler) recordAIAction(actionType string, url string, details map[string]any) {
	action := capture.EnhancedAction{
		Type:      actionType,
		Timestamp: time.Now().UnixMilli(),
		URL:       url,
		Source:    "ai",
	}
	// Add optional details as selectors (reusing the selectors field for metadata)
	if len(details) > 0 {
		action.Selectors = details
	}
	h.capture.AddEnhancedActions([]capture.EnhancedAction{action})
}

// toolInteract dispatches interact requests based on the 'action' parameter.
func (h *ToolHandler) toolInteract(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Action string `json:"action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Action == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"), withHint("Valid values: highlight, subtitle, execute_js, navigate, refresh, back, forward, new_tab, click, type, select, check, get_text, get_value, get_attribute, set_attribute, focus, scroll_to, wait_for, key_press, list_interactive, save_state, load_state, list_states, delete_state"))}
	}

	// Extract optional subtitle param (composable: works on any action)
	var composableSubtitle struct {
		Subtitle *string `json:"subtitle"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &composableSubtitle)
	}

	var resp JSONRPCResponse
	switch params.Action {
	case "highlight":
		resp = h.handlePilotHighlight(req, args)
	case "save_state":
		resp = h.handlePilotManageStateSave(req, args)
	case "load_state":
		resp = h.handlePilotManageStateLoad(req, args)
	case "list_states":
		resp = h.handlePilotManageStateList(req, args)
	case "delete_state":
		resp = h.handlePilotManageStateDelete(req, args)
	case "execute_js":
		resp = h.handlePilotExecuteJS(req, args)
	case "navigate":
		resp = h.handleBrowserActionNavigate(req, args)
	case "refresh":
		resp = h.handleBrowserActionRefresh(req, args)
	case "back":
		resp = h.handleBrowserActionBack(req, args)
	case "forward":
		resp = h.handleBrowserActionForward(req, args)
	case "new_tab":
		resp = h.handleBrowserActionNewTab(req, args)
	case "subtitle":
		resp = h.handleSubtitle(req, args)
	case "click", "type", "select", "check", "get_text", "get_value",
		"get_attribute", "set_attribute", "focus", "scroll_to", "wait_for", "key_press":
		resp = h.handleDOMPrimitive(req, args, params.Action)
	case "list_interactive":
		resp = h.handleListInteractive(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown interact action: "+params.Action, "Use a valid action from the 'action' enum", withParam("action"))}
	}

	// If a composable subtitle was provided on a non-subtitle action, queue it
	if composableSubtitle.Subtitle != nil && params.Action != "subtitle" {
		subtitleArgs, _ := json.Marshal(map[string]string{"text": *composableSubtitle.Subtitle})
		subtitleQuery := queries.PendingQuery{
			Type:          "subtitle",
			Params:        subtitleArgs,
			CorrelationID: fmt.Sprintf("subtitle_%d_%d", time.Now().UnixNano(), rand.Int63()),
		}
		h.capture.CreatePendingQueryWithTimeout(subtitleQuery, queries.AsyncCommandTimeout, req.ClientID)
	}

	return resp
}

// ============================================
// Interact sub-handlers (Pilot and Browser Actions)
// ============================================

func (h *ToolHandler) handlePilotHighlight(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Selector   string `json:"selector"`
		DurationMs int    `json:"duration_ms,omitempty"`
		TabID      int    `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.Selector == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'selector' is missing", "Add the 'selector' parameter", withParam("selector"))}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	// Queue highlight command for extension
	correlationID := fmt.Sprintf("highlight_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "highlight",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action
	h.recordAIAction("highlight", "", map[string]any{"selector": params.Selector})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Highlight queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Highlight command queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to check status.",
	})}
}

const stateNamespace = "saved_states"

func (h *ToolHandler) handlePilotManageStateSave(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.SnapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter", withParam("snapshot_name"))}
	}

	// Ensure session store is initialized
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// Capture current state from the tracked tab
	_, tabID, tabURL := h.capture.GetTrackingStatus()
	tabTitle := h.capture.GetTrackedTabTitle()

	stateData := map[string]any{
		"url":        tabURL,
		"title":      tabTitle,
		"tab_id":     tabID,
		"saved_at":   time.Now().Format(time.RFC3339),
	}

	// Serialize and save
	data, err := json.Marshal(stateData)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to serialize state: "+err.Error(), "Internal error — do not retry")}
	}

	if err := h.sessionStoreImpl.Save(stateNamespace, params.SnapshotName, data); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to save state: "+err.Error(), "Internal error — check storage")}
	}

	// Record AI action
	h.recordAIAction("save_state", tabURL, map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State saved", map[string]any{
		"status":        "saved",
		"snapshot_name": params.SnapshotName,
		"state": map[string]any{
			"url":   tabURL,
			"title": tabTitle,
		},
	})}
}

func (h *ToolHandler) handlePilotManageStateLoad(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
		IncludeURL   bool   `json:"include_url,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.SnapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter", withParam("snapshot_name"))}
	}

	// Ensure session store is initialized
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// Load state data
	data, err := h.sessionStoreImpl.Load(stateNamespace, params.SnapshotName)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "State not found: "+params.SnapshotName, "Use list_states to see available states")}
	}

	var stateData map[string]any
	if err := json.Unmarshal(data, &stateData); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to parse state data", "Internal error — state may be corrupted")}
	}

	// If include_url is true and pilot is enabled, navigate to the saved URL
	if params.IncludeURL {
		if savedURL, ok := stateData["url"].(string); ok && savedURL != "" {
			if h.capture.IsPilotEnabled() {
				// Queue navigation command
				correlationID := fmt.Sprintf("nav_%d_%d", time.Now().UnixNano(), rand.Int63())
				navArgs, _ := json.Marshal(map[string]any{"action": "navigate", "url": savedURL})
				query := queries.PendingQuery{
					Type:          "browser_action",
					Params:        navArgs,
					CorrelationID: correlationID,
				}
				h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)
				stateData["navigation_queued"] = true
				stateData["correlation_id"] = correlationID
			}
		}
	}

	// Record AI action
	h.recordAIAction("load_state", "", map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State loaded", map[string]any{
		"status":        "loaded",
		"snapshot_name": params.SnapshotName,
		"state":         stateData,
	})}
}

func (h *ToolHandler) handlePilotManageStateList(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Ensure session store is initialized
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// List all saved states
	keys, err := h.sessionStoreImpl.List(stateNamespace)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to list states: "+err.Error(), "Internal error — do not retry")}
	}

	// Build state list with metadata
	states := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		stateEntry := map[string]any{
			"name": key,
		}
		// Try to load and extract metadata
		if data, err := h.sessionStoreImpl.Load(stateNamespace, key); err == nil {
			var stateData map[string]any
			if json.Unmarshal(data, &stateData) == nil {
				if url, ok := stateData["url"].(string); ok {
					stateEntry["url"] = url
				}
				if title, ok := stateData["title"].(string); ok {
					stateEntry["title"] = title
				}
				if savedAt, ok := stateData["saved_at"].(string); ok {
					stateEntry["saved_at"] = savedAt
				}
			}
		}
		states = append(states, stateEntry)
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("States listed", map[string]any{
		"states": states,
		"count":  len(states),
	})}
}

func (h *ToolHandler) handlePilotManageStateDelete(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.SnapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter", withParam("snapshot_name"))}
	}

	// Ensure session store is initialized
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// Delete the state
	if err := h.sessionStoreImpl.Delete(stateNamespace, params.SnapshotName); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "State not found: "+params.SnapshotName, "Use list_states to see available states")}
	}

	// Record AI action
	h.recordAIAction("delete_state", "", map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State deleted", map[string]any{
		"status":        "deleted",
		"snapshot_name": params.SnapshotName,
	})}
}

func (h *ToolHandler) handlePilotExecuteJS(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		Script    string `json:"script"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
		World     string `json:"world,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.Script == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'script' is missing", "Add the 'script' parameter and call again", withParam("script"))}
	}

	// Validate world parameter: auto (default), main, isolated
	if params.World == "" {
		params.World = "auto"
	}
	if params.World != "auto" && params.World != "main" && params.World != "isolated" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'auto' (default, tries main then isolated), 'main' (page JS access), or 'isolated' (bypasses CSP, DOM only)", withParam("world"))}
	}

	// Check if pilot is enabled
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", withHint("Click the extension icon and toggle 'AI Web Pilot' on"))}
	}

	// Generate correlation ID for async tracking
	correlationID := fmt.Sprintf("exec_%d_%d", time.Now().UnixNano(), rand.Int63())

	// Queue command for extension to pick up (use long timeout for async commands)
	query := queries.PendingQuery{
		Type:          "execute",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action (truncate script preview)
	scriptPreview := params.Script
	if len(scriptPreview) > 100 {
		scriptPreview = scriptPreview[:100] + "..."
	}
	h.recordAIAction("execute_js", "", map[string]any{"script_preview": scriptPreview})

	// Return immediately with "queued" status
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Command queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Command queued for execution. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})}
}

func (h *ToolHandler) handleBrowserActionNavigate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		URL   string `json:"url"`
		TabID int    `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.URL == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'url' is missing", "Add the 'url' parameter and call again", withParam("url"))}
	}

	// Check if pilot is enabled
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	// Generate correlation ID
	correlationID := fmt.Sprintf("nav_%d_%d", time.Now().UnixNano(), rand.Int63())

	// Queue command
	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action
	h.recordAIAction("navigate", params.URL, map[string]any{"target_url": params.URL})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Navigate queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Navigation queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})}
}

func (h *ToolHandler) handleBrowserActionRefresh(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("refresh_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"refresh"}`),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action
	h.recordAIAction("refresh", "", nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Refresh queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
	})}
}

func (h *ToolHandler) handleBrowserActionBack(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("back_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"back"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action
	h.recordAIAction("back", "", nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Back queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
	})}
}

func (h *ToolHandler) handleBrowserActionForward(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("forward_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"forward"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action
	h.recordAIAction("forward", "", nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Forward queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
	})}
}

func (h *ToolHandler) handleBrowserActionNewTab(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("newtab_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action
	h.recordAIAction("new_tab", params.URL, map[string]any{"target_url": params.URL})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("New tab queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
	})}
}

// ============================================
// DOM Primitives: Pre-compiled browser interactions
// These use chrome.scripting.executeScript with func parameter
// to bypass CSP restrictions on pages like Gmail.
// ============================================

func (h *ToolHandler) handleDOMPrimitive(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	var params struct {
		Selector  string `json:"selector"`
		Text      string `json:"text,omitempty"`
		Value     string `json:"value,omitempty"`
		Clear     bool   `json:"clear,omitempty"`
		Checked   *bool  `json:"checked,omitempty"`
		Name      string `json:"name,omitempty"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	// All DOM primitives require selector
	if params.Selector == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'selector' is missing", "Add the 'selector' parameter. Supports CSS selectors or semantic: text=Submit, role=button, placeholder=Email, label=Name, aria-label=Close", withParam("selector"))}
	}

	// Action-specific required param validation
	switch action {
	case "type":
		if params.Text == "" {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'text' is missing for type action", "Add the 'text' parameter with the text to type", withParam("text"))}
		}
	case "select":
		if params.Value == "" {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'value' is missing for select action", "Add the 'value' parameter with the option value to select", withParam("value"))}
		}
	case "get_attribute", "set_attribute":
		if params.Name == "" {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'name' is missing for "+action+" action", "Add the 'name' parameter with the attribute name", withParam("name"))}
		}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("dom_%s_%d_%d", action, time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("dom_"+action, "", map[string]any{"selector": params.Selector})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(action+" queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "DOM action queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to check status.",
	})}
}

func (h *ToolHandler) handleSubtitle(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Text *string `json:"text"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.Text == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'text' is missing for subtitle action", "Add the 'text' parameter with subtitle text, or empty string to clear", withParam("text"))}
	}

	correlationID := fmt.Sprintf("subtitle_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "subtitle",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	if *params.Text == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Subtitle cleared", map[string]any{
			"status":  "queued",
			"subtitle": "cleared",
		})}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Subtitle set", map[string]any{
		"status":  "queued",
		"subtitle": *params.Text,
	})}
}

func (h *ToolHandler) handleListInteractive(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("dom_list_%d_%d", time.Now().UnixNano(), rand.Int63())

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("dom_list_interactive", "", nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("list_interactive queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Discovery queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get interactive elements.",
	})}
}
