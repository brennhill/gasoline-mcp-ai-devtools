// tools_interact.go — MCP interact tool dispatcher and handlers.
// Docs: docs/features/feature/interact-explore/index.md
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
	act "github.com/dev-console/dev-console/internal/tools/interact"
	"github.com/dev-console/dev-console/internal/tools/observe"
)

// interactHandler is the function signature for interact action handlers.
type interactHandler func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// interactDispatch returns the dispatch map for interact actions.
// Initialized once via sync.Once; safe for concurrent use.
func (h *ToolHandler) interactDispatch() map[string]interactHandler {
	h.interactOnce.Do(func() {
		h.interactHandlers = map[string]interactHandler{
			"highlight":                 h.handlePilotHighlight,
			"save_state":                h.handlePilotManageStateSave,
			"state_save":                h.handlePilotManageStateSave, // backward-compatible alias
			"load_state":                h.handlePilotManageStateLoad,
			"state_load":                h.handlePilotManageStateLoad, // backward-compatible alias
			"list_states":               h.handlePilotManageStateList,
			"state_list":                h.handlePilotManageStateList, // backward-compatible alias
			"delete_state":              h.handlePilotManageStateDelete,
			"state_delete":              h.handlePilotManageStateDelete, // backward-compatible alias
			"set_storage":               h.handleSetStorage,
			"delete_storage":            h.handleDeleteStorage,
			"clear_storage":             h.handleClearStorage,
			"set_cookie":                h.handleSetCookie,
			"delete_cookie":             h.handleDeleteCookie,
			"execute_js":                h.handlePilotExecuteJS,
			"navigate":                  h.handleBrowserActionNavigate,
			"refresh":                   h.handleBrowserActionRefresh,
			"back":                      h.handleBrowserActionBack,
			"forward":                   h.handleBrowserActionForward,
			"new_tab":                   h.handleBrowserActionNewTab,
			"screenshot":                h.handleScreenshotAlias,
			"subtitle":                  h.handleSubtitle,
			"list_interactive":          h.handleListInteractive,
			"record_start":              h.handleRecordStart,
			"record_stop":               h.handleRecordStop,
			"upload":                    h.handleUpload,
			"draw_mode_start":           h.handleDrawModeStart,
			"get_readable":              h.handleGetReadable,
			"get_markdown":              h.handleGetMarkdown,
			"navigate_and_wait_for":     h.handleNavigateAndWaitFor,
			"fill_form_and_submit":      h.handleFillFormAndSubmit,
			"fill_form":                 h.handleFillForm,
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

// domPrimitiveActions delegates to the interact package.
var domPrimitiveActions = act.DOMPrimitiveActions

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

// domActionToReproType delegates to the interact package.
var domActionToReproType = act.DOMActionToReproType

// parseSelectorForReproduction delegates to the interact package.
var parseSelectorForReproduction = act.ParseSelectorForReproduction

// toolInteract dispatches interact requests based on the 'what' parameter.
func (h *ToolHandler) toolInteract(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	what := params.What
	if what == "" {
		what = params.Action
	}

	if what == "" {
		validActions := h.getValidInteractActions()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validActions))}
	}

	if _, err := parseEvidenceMode(args); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Invalid 'evidence' value",
			"Use evidence='off' (default), 'on_mutation', or 'always'",
			withParam("evidence"),
		)}
	}

	// Extract optional subtitle param (composable: works on any action)
	var composableSubtitle struct {
		Subtitle *string `json:"subtitle"`
	}
	lenientUnmarshal(args, &composableSubtitle)

	resp := h.dispatchInteractAction(req, args, what)

	// If a composable subtitle was provided on a non-subtitle action, queue it.
	// Only queue if the primary action didn't fail (avoid subtitle on error).
	if composableSubtitle.Subtitle != nil && what != "subtitle" && resp.Error == nil {
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
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown interact action: "+action, "Use a valid action from the 'what' enum", withParam("what"))}
}

// handleScreenshotAlias provides backward compatibility for clients that call
// interact({action:"screenshot"}). The canonical API remains observe({what:"screenshot"}).
func (h *ToolHandler) handleScreenshotAlias(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return observe.GetScreenshot(h, req, args)
}

// queueComposableSubtitle queues a subtitle command as a side effect of another action.
func (h *ToolHandler) queueComposableSubtitle(req JSONRPCRequest, text string) {
	// Error impossible: map contains only string values
	subtitleArgs, _ := json.Marshal(map[string]string{"text": text})
	subtitleQuery := queries.PendingQuery{
		Type:          "subtitle",
		Params:        subtitleArgs,
		CorrelationID: newCorrelationID("subtitle"),
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

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	// Queue highlight command for extension
	correlationID := newCorrelationID("highlight")
	h.armEvidenceForCommand(correlationID, "highlight", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "highlight",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Record AI action
	h.recordAIAction("highlight", "", map[string]any{"selector": params.Selector})

	return h.MaybeWaitForCommand(req, correlationID, args, "Highlight queued")
}

// validWorldValues delegates to the interact package.
var validWorldValues = act.ValidWorldValues

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

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("exec")
	h.armEvidenceForCommand(correlationID, "execute_js", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("execute_js", "", map[string]any{"script_preview": truncateToLen(params.Script, 100)})

	return h.MaybeWaitForCommand(req, correlationID, args, "Command queued")
}

// truncateToLen delegates to the interact package.
var truncateToLen = act.TruncateToLen

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

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("nav")
	h.armEvidenceForCommand(correlationID, "navigate", args, req.ClientID)

	h.stashPerfSnapshot(correlationID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "navigate"
	// Ensure required URL is present even if caller used alias forms.
	actionParams["url"] = params.URL
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("navigate", params.URL, map[string]any{"target_url": params.URL})

	resp := h.MaybeWaitForCommand(req, correlationID, args, "Navigate queued")

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

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("refresh")
	h.armEvidenceForCommand(correlationID, "refresh", args, req.ClientID)

	h.stashPerfSnapshot(correlationID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"refresh"}`),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("refresh", "", nil)

	return h.MaybeWaitForCommand(req, correlationID, args, "Refresh queued")
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
	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("back")
	h.armEvidenceForCommand(correlationID, "back", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"back"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("back", "", nil)

	return h.MaybeWaitForCommand(req, correlationID, args, "Back queued")
}

func (h *ToolHandler) handleBrowserActionForward(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("forward")
	h.armEvidenceForCommand(correlationID, "forward", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"forward"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("forward", "", nil)

	return h.MaybeWaitForCommand(req, correlationID, args, "Forward queued")
}

func (h *ToolHandler) handleBrowserActionNewTab(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("newtab")
	h.armEvidenceForCommand(correlationID, "new_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "new_tab"
	if params.URL != "" {
		actionParams["url"] = params.URL
	}
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("new_tab", params.URL, map[string]any{"target_url": params.URL})

	return h.MaybeWaitForCommand(req, correlationID, args, "New tab queued")
}

// ============================================
// DOM Primitives: Pre-compiled browser interactions
// These use chrome.scripting.executeScript with func parameter
// to bypass CSP restrictions on pages like Gmail.
// ============================================

// domActionRequiredParams delegates to the interact package.
var domActionRequiredParams = act.DOMActionRequiredParams

// normalizeDOMActionArgs rewrites interact args so extension-facing dom_action
// payloads always carry canonical "action", while preserving user-facing "what".
func normalizeDOMActionArgs(args json.RawMessage, action string) json.RawMessage {
	var payload map[string]any
	if err := json.Unmarshal(args, &payload); err != nil || payload == nil {
		payload = map[string]any{}
	}
	payload["action"] = action
	if _, hasScopeRect := payload["scope_rect"]; !hasScopeRect {
		if annotationRect, hasAnnotationRect := payload["annotation_rect"]; hasAnnotationRect {
			payload["scope_rect"] = annotationRect
		}
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return args
	}
	return normalized
}

func (h *ToolHandler) handleDOMPrimitive(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	var params struct {
		Selector      string `json:"selector"`
		ScopeSelector string `json:"scope_selector,omitempty"`
		ElementID     string `json:"element_id,omitempty"`
		Index         *int   `json:"index,omitempty"`
		Text          string `json:"text,omitempty"`
		Value         string `json:"value,omitempty"`
		Clear         bool   `json:"clear,omitempty"`
		Checked       *bool  `json:"checked,omitempty"`
		Name          string `json:"name,omitempty"`
		TimeoutMs     int    `json:"timeout_ms,omitempty"`
		TabID         int    `json:"tab_id,omitempty"`
		Analyze       bool   `json:"analyze,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	// Resolve index to selector if index is provided and selector is empty
	if params.Index != nil && params.Selector == "" && params.ElementID == "" {
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

	selectorOptionalActions := map[string]bool{
		"open_composer":          true,
		"submit_active_composer": true,
		"confirm_top_dialog":     true,
		"dismiss_top_overlay":    true,
	}
	if params.Selector == "" && params.ElementID == "" && !selectorOptionalActions[action] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"Required parameter 'selector', 'element_id', or 'index' is missing",
			"Add 'selector' (CSS or semantic selector), or use 'element_id'/'index' from list_interactive results.",
			withParam("selector"),
		)}
	}

	if errResp, failed := validateDOMActionParams(req, action, params.Text, params.Value, params.Name); failed {
		return errResp
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	args = normalizeDOMActionArgs(args, action)

	correlationID := newCorrelationID("dom_" + action)
	h.armEvidenceForCommand(correlationID, action, args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordDOMPrimitiveAction(action, params.Selector, params.Text, params.Value)

	return h.MaybeWaitForCommand(req, correlationID, args, action+" queued")
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
	switch rule.Field {
	case "text":
		paramValue = text
	case "value":
		paramValue = value
	case "name":
		paramValue = name
	}
	if paramValue == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, rule.Message, rule.Retry, withParam(rule.Field))}, true
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

	correlationID := newCorrelationID("subtitle")
	h.armEvidenceForCommand(correlationID, "subtitle", args, req.ClientID)

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

	return h.MaybeWaitForCommand(req, correlationID, args, queuedMsg)
}

// Element indexing functions moved to tools_interact_elements.go
