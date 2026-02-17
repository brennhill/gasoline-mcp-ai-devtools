// Purpose: Implements interact tool handlers and browser action orchestration.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact.go — MCP interact tool dispatcher and handlers.
// Handles all browser interaction actions: navigate, execute_js, highlight, state management, etc.
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/queries"
)

// interactHandler is the function signature for interact action handlers.
type interactHandler func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// interactDispatch returns the dispatch map for interact actions.
// Initialized once via sync.Once; safe for concurrent use.
func (h *ToolHandler) interactDispatch() map[string]interactHandler {
	h.interactOnce.Do(func() {
		h.interactHandlers = map[string]interactHandler{
			"highlight":        h.handlePilotHighlight,
			"save_state":       h.handlePilotManageStateSave,
			"load_state":       h.handlePilotManageStateLoad,
			"list_states":      h.handlePilotManageStateList,
			"delete_state":     h.handlePilotManageStateDelete,
			"execute_js":       h.handlePilotExecuteJS,
			"navigate":         h.handleBrowserActionNavigate,
			"refresh":          h.handleBrowserActionRefresh,
			"back":             h.handleBrowserActionBack,
			"forward":          h.handleBrowserActionForward,
			"new_tab":          h.handleBrowserActionNewTab,
			"screenshot":       h.handleScreenshotAlias,
			"subtitle":         h.handleSubtitle,
			"list_interactive": h.handleListInteractive,
			"record_start":     h.handleRecordStart,
			"record_stop":      h.handleRecordStop,
			"upload":           h.handleUpload,
			"draw_mode_start":  h.handleDrawModeStart,
			"get_readable":            h.handleGetReadable,
			"get_markdown":            h.handleGetMarkdown,
			"navigate_and_wait_for":    h.handleNavigateAndWaitFor,
			"fill_form_and_submit":     h.handleFillFormAndSubmit,
			"run_a11y_and_export_sarif": h.handleRunA11yAndExportSARIF,
		}
	})
	return h.interactHandlers
}

// getValidInteractActions returns a sorted, comma-separated list of valid interact actions.
func (h *ToolHandler) getValidInteractActions() string {
	actions := make(map[string]bool)
	for action := range h.interactDispatch() {
		actions[action] = true
	}
	for action := range domPrimitiveActions {
		actions[action] = true
	}
	sorted := make([]string, 0, len(actions))
	for a := range actions {
		sorted = append(sorted, a)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

// domPrimitiveActions is the set of actions routed to handleDOMPrimitive.
var domPrimitiveActions = map[string]bool{
	"click": true, "type": true, "select": true, "check": true,
	"get_text": true, "get_value": true, "get_attribute": true,
	"set_attribute": true, "focus": true, "scroll_to": true,
	"wait_for": true, "key_press": true, "paste": true,
}

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

// recordAIEnhancedAction records a fully populated AI-driven action.
// Used by DOM primitives to store action data in reproduction-compatible format.
func (h *ToolHandler) recordAIEnhancedAction(action capture.EnhancedAction) {
	action.Timestamp = time.Now().UnixMilli()
	action.Source = "ai"
	h.capture.AddEnhancedActions([]capture.EnhancedAction{action})
}

// domActionToReproType maps interact DOM action names to reproduction-compatible types.
// Actions not in this map are recorded as-is (with "dom_" prefix for audit trail).
var domActionToReproType = map[string]string{
	"click":     "click",
	"type":      "input",
	"select":    "select",
	"check":     "click",
	"key_press": "keypress",
	"scroll_to": "scroll_element",
	"focus":     "focus",
}

// parseSelectorForReproduction converts an interact-tool selector string into
// a selectors map compatible with the reproduction formatter.
// Handles semantic selectors (text=Submit, role=button) and CSS selectors.
func parseSelectorForReproduction(selector string) map[string]any {
	selectors := map[string]any{}
	if idx := strings.Index(selector, "="); idx > 0 {
		prefix := selector[:idx]
		value := selector[idx+1:]
		switch prefix {
		case "text":
			selectors["text"] = value
		case "role":
			selectors["role"] = map[string]any{"role": value}
		case "label", "aria-label":
			selectors["ariaLabel"] = value
		case "placeholder":
			selectors["ariaLabel"] = value
		default:
			selectors["cssPath"] = selector
		}
	} else {
		// Plain CSS selector — detect #id vs general CSS
		if strings.HasPrefix(selector, "#") && !strings.ContainsAny(selector[1:], " >.+~[]:#") {
			selectors["id"] = selector[1:]
		} else {
			selectors["cssPath"] = selector
		}
	}
	return selectors
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
		validActions := h.getValidInteractActions()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"), withHint("Valid values: "+validActions))}
	}

	// Extract optional subtitle param (composable: works on any action)
	var composableSubtitle struct {
		Subtitle *string `json:"subtitle"`
	}
	lenientUnmarshal(args, &composableSubtitle)

	resp := h.dispatchInteractAction(req, args, params.Action)

	// If a composable subtitle was provided on a non-subtitle action, queue it.
	// Only queue if the primary action didn't fail (avoid subtitle on error).
	if composableSubtitle.Subtitle != nil && params.Action != "subtitle" && resp.Error == nil {
		h.queueComposableSubtitle(req, *composableSubtitle.Subtitle)
	}

	return resp
}

// dispatchInteractAction routes an action to the correct handler using
// the dispatch map for named actions and the DOM primitive set for DOM actions.
func (h *ToolHandler) dispatchInteractAction(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	if handler, ok := h.interactDispatch()[action]; ok {
		return handler(req, args)
	}
	if domPrimitiveActions[action] {
		return h.handleDOMPrimitive(req, args, action)
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown interact action: "+action, "Use a valid action from the 'action' enum", withParam("action"))}
}

// handleScreenshotAlias provides backward compatibility for clients that call
// interact({action:"screenshot"}). The canonical API remains observe({what:"screenshot"}).
func (h *ToolHandler) handleScreenshotAlias(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.toolGetScreenshot(req, args)
}

// queueComposableSubtitle queues a subtitle command as a side effect of another action.
func (h *ToolHandler) queueComposableSubtitle(req JSONRPCRequest, text string) {
	// Error impossible: map contains only string values
	subtitleArgs, _ := json.Marshal(map[string]string{"text": text})
	subtitleQuery := queries.PendingQuery{
		Type:          "subtitle",
		Params:        subtitleArgs,
		CorrelationID: fmt.Sprintf("subtitle_%d_%d", time.Now().UnixNano(), randomInt63()),
	}
	h.capture.CreatePendingQueryWithTimeout(subtitleQuery, queries.AsyncCommandTimeout, req.ClientID)
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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	// Queue highlight command for extension
	correlationID := fmt.Sprintf("highlight_%d_%d", time.Now().UnixNano(), randomInt63())

	query := queries.PendingQuery{
		Type:          "highlight",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action
	h.recordAIAction("highlight", "", map[string]any{"selector": params.Selector})

	return h.maybeWaitForCommand(req, correlationID, args, "Highlight queued")
}

// validWorldValues is the set of accepted values for the execute_js 'world' parameter.
var validWorldValues = map[string]bool{
	"auto": true, "main": true, "isolated": true,
}

func (h *ToolHandler) handlePilotExecuteJS(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

	if params.World == "" {
		params.World = "auto"
	}
	if !validWorldValues[params.World] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'auto' (default, tries main then isolated), 'main' (page JS access), or 'isolated' (bypasses CSP, DOM only)", withParam("world"))}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("exec_%d_%d", time.Now().UnixNano(), randomInt63())

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("execute_js", "", map[string]any{"script_preview": truncateToLen(params.Script, 100)})

	return h.maybeWaitForCommand(req, correlationID, args, "Command queued")
}

// truncatePreview returns s unchanged if shorter than maxLen, otherwise truncates with "...".
func truncateToLen(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (h *ToolHandler) handleBrowserActionNavigate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL            string `json:"url"`
		TabID          int    `json:"tab_id,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.URL == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'url' is missing", "Add the 'url' parameter and call again", withParam("url"))}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("nav_%d_%d", time.Now().UnixNano(), randomInt63())

	h.stashPerfSnapshot(correlationID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("navigate", params.URL, map[string]any{"target_url": params.URL})

	resp := h.maybeWaitForCommand(req, correlationID, args, "Navigate queued")

	// If include_content is requested and navigate succeeded, enrich with page content
	if params.IncludeContent {
		resp = h.enrichNavigateResponse(resp, req, params.TabID)
	}

	return resp
}

// enrichNavigateResponse moved to tools_interact_content.go

func (h *ToolHandler) handleBrowserActionRefresh(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("refresh_%d_%d", time.Now().UnixNano(), randomInt63())

	h.stashPerfSnapshot(correlationID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"refresh"}`),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("refresh", "", nil)

	return h.maybeWaitForCommand(req, correlationID, args, "Refresh queued")
}

// stashPerfSnapshot saves the current performance snapshot as a "before" baseline
// for perf_diff computation, keyed by correlation ID.
func (h *ToolHandler) stashPerfSnapshot(correlationID string) {
	_, _, trackedURL := h.capture.GetTrackingStatus()
	u, err := url.Parse(trackedURL)
	if err != nil || u.Path == "" {
		return
	}
	if snap, ok := h.capture.GetPerformanceSnapshotByURL(u.Path); ok {
		h.capture.StoreBeforeSnapshot(correlationID, snap)
	}
}

func (h *ToolHandler) handleBrowserActionBack(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("back_%d_%d", time.Now().UnixNano(), randomInt63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"back"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("back", "", nil)

	return h.maybeWaitForCommand(req, correlationID, args, "Back queued")
}

func (h *ToolHandler) handleBrowserActionForward(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("forward_%d_%d", time.Now().UnixNano(), randomInt63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"forward"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("forward", "", nil)

	return h.maybeWaitForCommand(req, correlationID, args, "Forward queued")
}

func (h *ToolHandler) handleBrowserActionNewTab(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("newtab_%d_%d", time.Now().UnixNano(), randomInt63())

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("new_tab", params.URL, map[string]any{"target_url": params.URL})

	return h.maybeWaitForCommand(req, correlationID, args, "New tab queued")
}

// ============================================
// DOM Primitives: Pre-compiled browser interactions
// These use chrome.scripting.executeScript with func parameter
// to bypass CSP restrictions on pages like Gmail.
// ============================================

// domActionRequiredParams maps DOM actions to their required parameter name and error guidance.
var domActionRequiredParams = map[string]struct {
	field   string
	message string
	retry   string
}{
	"type":          {"text", "Required parameter 'text' is missing for type action", "Add the 'text' parameter with the text to type"},
	"paste":         {"text", "Required parameter 'text' is missing for paste action", "Add the 'text' parameter with the text to paste"},
	"select":        {"value", "Required parameter 'value' is missing for select action", "Add the 'value' parameter with the option value to select"},
	"get_attribute": {"name", "Required parameter 'name' is missing for get_attribute action", "Add the 'name' parameter with the attribute name"},
	"set_attribute": {"name", "Required parameter 'name' is missing for set_attribute action", "Add the 'name' parameter with the attribute name"},
}

func (h *ToolHandler) handleDOMPrimitive(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	var params struct {
		Selector  string `json:"selector"`
		Index     *int   `json:"index,omitempty"`
		Text      string `json:"text,omitempty"`
		Value     string `json:"value,omitempty"`
		Clear     bool   `json:"clear,omitempty"`
		Checked   *bool  `json:"checked,omitempty"`
		Name      string `json:"name,omitempty"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
		Analyze   bool   `json:"analyze,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	// Resolve index to selector if index is provided and selector is empty
	if params.Index != nil && params.Selector == "" {
		sel, ok := h.resolveIndexToSelector(*params.Index)
		if !ok {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				fmt.Sprintf("Element index %d not found. Call list_interactive first to refresh the element index.", *params.Index),
				"Call interact with action='list_interactive' first, then use the index from the results.",
				withParam("index"),
			)}
		}
		params.Selector = sel
		// Rewrite args to include the resolved selector
		var rawArgs map[string]json.RawMessage
		if json.Unmarshal(args, &rawArgs) == nil {
			selectorJSON, _ := json.Marshal(sel)
			rawArgs["selector"] = selectorJSON
			args, _ = json.Marshal(rawArgs)
		}
	}

	if params.Selector == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'selector' (or 'index') is missing", "Add the 'selector' parameter. Supports CSS selectors or semantic: text=Submit, role=button, placeholder=Email, label=Name, aria-label=Close. Or use 'index' from list_interactive results.", withParam("selector"))}
	}

	if errResp, failed := validateDOMActionParams(req, action, params.Text, params.Value, params.Name); failed {
		return errResp
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("dom_%s_%d_%d", action, time.Now().UnixNano(), randomInt63())

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordDOMPrimitiveAction(action, params.Selector, params.Text, params.Value)

	return h.maybeWaitForCommand(req, correlationID, args, action+" queued")
}

// recordDOMPrimitiveAction records a DOM primitive action with reproduction-compatible
// type and field mapping. Falls back to "dom_<action>" for actions without a mapping.
func (h *ToolHandler) recordDOMPrimitiveAction(action, selector, text, value string) {
	reproType, ok := domActionToReproType[action]
	if !ok {
		// Unmapped actions (get_text, get_value, etc.) — keep dom_ prefix for audit trail
		h.recordAIAction("dom_"+action, "", map[string]any{"selector": selector})
		return
	}

	selectors := parseSelectorForReproduction(selector)
	ea := capture.EnhancedAction{
		Type:      reproType,
		Selectors: selectors,
	}

	// Populate type-specific fields
	switch action {
	case "type":
		ea.Value = text
	case "key_press":
		ea.Key = text
	case "select":
		ea.SelectedValue = value
	}

	h.recordAIEnhancedAction(ea)
}

// validateDOMActionParams checks action-specific required parameters.
// Returns (response, true) if validation failed, or (zero, false) if valid.
func validateDOMActionParams(req JSONRPCRequest, action, text, value, name string) (JSONRPCResponse, bool) {
	rule, ok := domActionRequiredParams[action]
	if !ok {
		return JSONRPCResponse{}, false
	}
	var paramValue string
	switch rule.field {
	case "text":
		paramValue = text
	case "value":
		paramValue = value
	case "name":
		paramValue = name
	}
	if paramValue == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, rule.message, rule.retry, withParam(rule.field))}, true
	}
	return JSONRPCResponse{}, false
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

	correlationID := fmt.Sprintf("subtitle_%d_%d", time.Now().UnixNano(), randomInt63())

	query := queries.PendingQuery{
		Type:          "subtitle",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	queuedMsg := "Subtitle set"
	if *params.Text == "" {
		queuedMsg = "Subtitle cleared"
	}

	return h.maybeWaitForCommand(req, correlationID, args, queuedMsg)
}

// Element indexing functions moved to tools_interact_elements.go
