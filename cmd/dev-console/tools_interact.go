// Purpose: Implements interact tool handlers and browser action orchestration.
// Why: Preserves deterministic browser action execution across agent workflows.
// Docs: docs/features/feature/interact-explore/index.md

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
			"switch_tab":                h.handleBrowserActionSwitchTab,
			"close_tab":                 h.handleBrowserActionCloseTab,
			"screenshot":                h.handleScreenshotAlias,
			"subtitle":                  h.handleSubtitle,
			"list_interactive":          h.handleListInteractive,
			"record_start":              h.handleRecordStart,
			"record_stop":               h.handleRecordStop,
			"upload":                    h.handleUpload,
			"draw_mode_start":           h.handleDrawModeStart,
			"hardware_click":            h.handleHardwareClick,
			"activate_tab":              h.handleActivateTab,
			"get_readable":              h.handleGetReadable,
			"get_markdown":              h.handleGetMarkdown,
			"navigate_and_wait_for":     h.handleNavigateAndWaitFor,
			"fill_form_and_submit":      h.handleFillFormAndSubmit,
			"fill_form":                 h.handleFillForm,
			"run_a11y_and_export_sarif": h.handleRunA11yAndExportSARIF,
			"explore_page":              h.handleExplorePage,
			"wait_for_stable":          h.handleWaitForStable,
			"auto_dismiss_overlays":    h.handleAutoDismissOverlays,
			"batch":                    h.handleBatch,
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
	usedAliasParam := ""
	// Check for conflict before falling back: if both what and action are set but
	// disagree, that is a caller error — report it before applying the alias fallback.
	if what != "" && params.Action != "" && params.Action != what {
		return whatAliasConflictResponse(req, "action", what, params.Action, h.getValidInteractActions())
	}
	if what == "" {
		what = params.Action
		if what != "" {
			usedAliasParam = "action"
		}
	}

	if what == "" {
		validActions := h.getValidInteractActions()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"Required dispatch parameter is missing: provide 'what' (or deprecated alias 'action')",
			"Add 'what' (preferred) or 'action' and call again",
			withParam("what"),
			withHint("Valid values: "+validActions),
		)}
	}

	if _, err := parseEvidenceMode(args); err != nil {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Invalid 'evidence' value",
			"Use evidence='off' (default), 'on_mutation', or 'always'",
			withParam("evidence"),
		)}
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	}

	// Extract optional composable params (work on any action)
	var composableParams struct {
		Subtitle          *string `json:"subtitle"`
		IncludeScreenshot bool    `json:"include_screenshot"`
		AutoDismiss       bool    `json:"auto_dismiss"`
		WaitForStable     bool    `json:"wait_for_stable"`
		StabilityMs       int     `json:"stability_ms,omitempty"`
	}
	lenientUnmarshal(args, &composableParams)

	resp := h.dispatchInteractAction(req, args, what)

	// If a composable subtitle was provided on a non-subtitle action, queue it.
	// Only queue if the primary action didn't fail (avoid subtitle on error).
	if composableParams.Subtitle != nil && what != "subtitle" && resp.Error == nil {
		h.queueComposableSubtitle(req, *composableParams.Subtitle)
	}

	// If auto_dismiss was requested on navigate and succeeded, queue auto_dismiss_overlays.
	if composableParams.AutoDismiss && what == "navigate" && resp.Error == nil && !isResponseError(resp) {
		h.queueComposableAutoDismiss(req)
	}

	// If wait_for_stable was requested on navigate/click and succeeded, queue wait_for_stable.
	if composableParams.WaitForStable && (what == "navigate" || what == "click") && resp.Error == nil && !isResponseError(resp) {
		h.queueComposableWaitForStable(req, composableParams.StabilityMs)
	}

	// If include_screenshot was requested and the action succeeded, capture a screenshot
	// and append it as an inline image content block.
	if composableParams.IncludeScreenshot && resp.Error == nil && !isResponseError(resp) {
		resp = h.appendScreenshotToResponse(resp, req)
	}

	resp = appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
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

// isResponseError checks if an MCP response contains an error result.
func isResponseError(resp JSONRPCResponse) bool {
	if resp.Result == nil {
		return false
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return false
	}
	return result.IsError
}

// isResponseQueued checks if an MCP response is a "queued" async response.
func isResponseQueued(resp JSONRPCResponse) bool {
	if resp.Result == nil {
		return false
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return false
	}
	if len(result.Content) == 0 {
		return false
	}
	// Check the text content for the queued status JSON.
	// The text may be "summary\n{json}" or just "{json}".
	for _, c := range result.Content {
		if c.Type == "text" && len(c.Text) > 0 {
			text := c.Text
			// If text has a summary line before JSON, extract the JSON part
			if idx := strings.Index(text, "\n{"); idx >= 0 {
				text = text[idx+1:]
			}
			var data map[string]any
			if err := json.Unmarshal([]byte(text), &data); err == nil {
				if status, ok := data["status"].(string); ok && status == "queued" {
					return true
				}
			}
		}
	}
	return false
}

// appendScreenshotToResponse captures a screenshot and appends it as an inline
// image content block to the response. If screenshot capture fails, the original
// response is returned unchanged (best-effort).
func (h *ToolHandler) appendScreenshotToResponse(resp JSONRPCResponse, req JSONRPCRequest) JSONRPCResponse {
	screenshotReq := JSONRPCRequest{JSONRPC: "2.0", ID: req.ID}
	screenshotResp := observe.GetScreenshot(h, screenshotReq, nil)

	// Extract the image content block from the screenshot response
	var screenshotResult MCPToolResult
	if err := json.Unmarshal(screenshotResp.Result, &screenshotResult); err != nil {
		return resp // best-effort: return original response
	}

	// Find the image content block and append it to the original response
	for _, block := range screenshotResult.Content {
		if block.Type == "image" && block.Data != "" {
			var result MCPToolResult
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				return resp
			}
			result.Content = append(result.Content, block)
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return resp
			}
			resp.Result = json.RawMessage(resultJSON)
			break
		}
	}

	return resp
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
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
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
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}
	if resp, blocked := h.requireCSPClear(req, params.World); blocked {
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
	resolvedURL, err := h.resolveNavigateURL(params.URL)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			err.Error(),
			"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
			withParam("url"),
		)}
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("nav")
	h.armEvidenceForCommand(correlationID, "navigate", args, req.ClientID)

	h.stashPerfSnapshot(correlationID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "navigate"
	// Ensure required URL is present even if caller used alias forms.
	actionParams["url"] = resolvedURL
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("navigate", resolvedURL, map[string]any{
		"target_url":    resolvedURL,
		"requested_url": params.URL,
	})

	resp := h.MaybeWaitForCommand(req, correlationID, args, "Navigate queued")

	// If include_content is requested and navigate succeeded, enrich with page content
	if params.IncludeContent {
		resp = h.enrichNavigateResponse(resp, req, params.TabID)
	}

	// Include blocked_actions/blocked_reason when CSP restricts — omit entirely
	// when CSP is clear to avoid wasting tokens on normal pages. (#262)
	resp = h.injectCSPBlockedActions(resp)

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
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
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
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
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
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
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
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	resolvedURL := params.URL
	if params.URL != "" {
		rewriteURL, err := h.resolveNavigateURL(params.URL)
		if err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				err.Error(),
				"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
				withParam("url"),
			)}
		}
		resolvedURL = rewriteURL
	}

	correlationID := newCorrelationID("newtab")
	h.armEvidenceForCommand(correlationID, "new_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "new_tab"
	if resolvedURL != "" {
		actionParams["url"] = resolvedURL
	}
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("new_tab", resolvedURL, map[string]any{
		"target_url":    resolvedURL,
		"requested_url": params.URL,
	})

	return h.MaybeWaitForCommand(req, correlationID, args, "New tab queued")
}

func (h *ToolHandler) handleBrowserActionSwitchTab(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID      int   `json:"tab_id,omitempty"`
		TabIndex   *int  `json:"tab_index,omitempty"`
		SetTracked *bool `json:"set_tracked,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	if params.TabID <= 0 && params.TabIndex == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"switch_tab requires tab_id or tab_index",
			"Provide tab_id from observe(what='tabs') or tab_index from your tab list ordering.",
			withParam("tab_id"),
			withHint("Alternative: provide tab_index"),
		)}
	}
	if params.TabIndex != nil && *params.TabIndex < 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"tab_index must be >= 0",
			"Provide a non-negative tab_index (0-based).",
			withParam("tab_index"),
		)}
	}

	// Default set_tracked to true so subsequent commands target the new tab.
	setTracked := params.SetTracked == nil || *params.SetTracked

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	// No requireTabTracking gate: switch_tab IS how you establish tracking
	// for an existing tab. The handler calls applySwitchTabTracking on success.

	correlationID := newCorrelationID("switchtab")
	h.armEvidenceForCommand(correlationID, "switch_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "switch_tab"
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("switch_tab", "", map[string]any{
		"tab_id":    params.TabID,
		"tab_index": params.TabIndex,
	})

	resp := h.MaybeWaitForCommand(req, correlationID, args, "Switch tab queued")

	// After the command completes, update tracked tab state so subsequent
	// commands target the newly activated tab. See issue #271.
	// NOTE: In async mode (sync=false), tracking update is deferred to
	// extension-side persistTrackedTab via the next /sync heartbeat.
	// Server-side update only occurs in sync mode because MaybeWaitForCommand
	// returns immediately when sync=false, so GetCommandResult has no result yet.
	if setTracked {
		h.applySwitchTabTracking(correlationID)
	}

	return resp
}

func (h *ToolHandler) handleActivateTab(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("activate")
	h.armEvidenceForCommand(correlationID, "activate_tab", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"activate_tab"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("activate_tab", "", nil)

	return h.MaybeWaitForCommand(req, correlationID, args, "Activate tab queued")
}

func (h *ToolHandler) handleBrowserActionCloseTab(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	// NOTE: close_tab is gated even with explicit tab_id.
	// Future: allow bypass when tab_id is explicitly provided.
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("closetab")
	h.armEvidenceForCommand(correlationID, "close_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "close_tab"
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("close_tab", "", map[string]any{
		"tab_id": params.TabID,
	})

	return h.MaybeWaitForCommand(req, correlationID, args, "Close tab queued")
}

func (h *ToolHandler) resolveNavigateURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	const insecurePrefix = "gasoline-insecure://"
	if !strings.HasPrefix(strings.ToLower(trimmed), insecurePrefix) {
		return trimmed, nil
	}
	if h.capture == nil {
		return "", fmt.Errorf("gasoline-insecure URL is unavailable because capture is not initialized")
	}

	mode, _, _ := h.capture.GetSecurityMode()
	if mode != capture.SecurityModeInsecureProxy {
		return "", fmt.Errorf("gasoline-insecure URL requires security_mode=insecure_proxy")
	}

	target := strings.TrimSpace(trimmed[len(insecurePrefix):])
	if target == "" {
		return "", fmt.Errorf("gasoline-insecure URL is missing target URL")
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("invalid gasoline-insecure target URL: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("gasoline-insecure target URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("gasoline-insecure target URL must include host")
	}

	port := defaultPort
	if h.server != nil {
		port = h.server.getListenPort()
	}
	return fmt.Sprintf("http://127.0.0.1:%d/insecure-proxy?target=%s", port, url.QueryEscape(target)), nil
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
		Selector      string   `json:"selector"`
		ScopeSelector string   `json:"scope_selector,omitempty"`
		ElementID     string   `json:"element_id,omitempty"`
		Index         *int     `json:"index,omitempty"`
		IndexGen      string   `json:"index_generation,omitempty"`
		Text          string   `json:"text,omitempty"`
		Value         string   `json:"value,omitempty"`
		Clear         bool     `json:"clear,omitempty"`
		Checked       *bool    `json:"checked,omitempty"`
		Name          string   `json:"name,omitempty"`
		TimeoutMs     int      `json:"timeout_ms,omitempty"`
		TabID         int      `json:"tab_id,omitempty"`
		Analyze       bool     `json:"analyze,omitempty"`
		X             *float64 `json:"x,omitempty"`
		Y             *float64 `json:"y,omitempty"`
		URLContains   string   `json:"url_contains,omitempty"`
		Absent        bool     `json:"absent,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	// If x/y coordinates provided on a click action, escalate to CDP for hardware-level click
	if action == "click" && params.X != nil && params.Y != nil {
		return h.handleCDPClick(req, args, action, *params.X, *params.Y, params.TabID)
	}

	// Resolve index to selector if index is provided and selector is empty
	if params.Index != nil && params.Selector == "" && params.ElementID == "" {
		sel, ok, stale, latestGeneration := h.resolveIndexToSelector(req.ClientID, params.TabID, *params.Index, params.IndexGen)
		if stale {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				formatIndexGenerationConflict(params.IndexGen, latestGeneration),
				"Re-run interact with what='list_interactive' for the current page context, then retry with the returned index_generation.",
				withParam("index_generation"),
				withParam("index"),
			)}
		}
		if !ok {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				fmt.Sprintf("Element index %d not found for tab_id=%d. Call list_interactive first to refresh the element index for this tab/client scope.", *params.Index, params.TabID),
				"Call interact with what='list_interactive' first (same tab/client scope), then use the returned index.",
				withParam("index"),
				withParam("tab_id"),
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
		"auto_dismiss_overlays":  true,
		"wait_for_stable":        true,
		"key_press":              true,
		"wait_for":               true,
	}
	if params.Selector == "" && params.ElementID == "" && !selectorOptionalActions[action] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"Required parameter 'selector', 'element_id', or 'index' is missing",
			"Add 'selector' (CSS or semantic selector), or use 'element_id'/'index' from list_interactive results.",
			withParam("selector"),
		)}
	}

	// wait_for: require at least one condition and reject incompatible combinations
	if action == "wait_for" {
		hasSelector := params.Selector != "" || params.ElementID != ""
		hasText := params.Text != ""
		hasURL := params.URLContains != ""
		condCount := 0
		if hasSelector || params.Absent {
			condCount++
		}
		if hasText {
			condCount++
		}
		if hasURL {
			condCount++
		}
		if condCount == 0 {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrMissingParam,
				"wait_for requires at least one condition: selector, text, or url_contains",
				"Provide 'selector' (wait for element), 'text' (wait for text), or 'url_contains' (wait for URL change).",
				withParam("selector"),
			)}
		}
		if condCount > 1 {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"wait_for conditions are mutually exclusive: use only one of selector, text, or url_contains",
				"Choose a single wait condition per call.",
			)}
		}
		if params.Absent && !hasSelector {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrMissingParam,
				"wait_for with absent requires a selector",
				"Provide 'selector' to specify which element to wait to disappear.",
				withParam("selector"),
			)}
		}
	}

	if errResp, failed := validateDOMActionParams(req, action, params.Text, params.Value, params.Name); failed {
		return errResp
	}

	contextOpts := []func(*StructuredError){withAction(action)}
	if params.Selector != "" {
		contextOpts = append(contextOpts, withSelector(params.Selector))
	}
	if resp, blocked := h.requirePilot(req, contextOpts...); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req, contextOpts...); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req, contextOpts...); blocked {
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

// handleHardwareClick dispatches a coordinate-based click via CDP Input.dispatchMouseEvent.
// This gives LLMs an explicit "I see coordinates in a screenshot, click there" path.
func (h *ToolHandler) handleHardwareClick(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		X     *float64 `json:"x"`
		Y     *float64 `json:"y"`
		TabID int      `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.X == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'x' is missing", "Add the 'x' coordinate (pixels from left)", withParam("x"))}
	}
	if params.Y == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'y' is missing", "Add the 'y' coordinate (pixels from top)", withParam("y"))}
	}

	return h.handleCDPClick(req, args, "hardware_click", *params.X, *params.Y, params.TabID)
}

// handleCDPClick creates a cdp_action query for a hardware-level click at coordinates.
func (h *ToolHandler) handleCDPClick(req JSONRPCRequest, args json.RawMessage, action string, x, y float64, tabID int) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req, withAction(action)); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req, withAction(action)); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req, withAction(action)); blocked {
		return resp
	}

	correlationID := newCorrelationID("cdp_click")
	h.armEvidenceForCommand(correlationID, action, args, req.ClientID)

	cdpParams, _ := json.Marshal(map[string]any{
		"action": "click",
		"x":      x,
		"y":      y,
	})

	query := queries.PendingQuery{
		Type:          "cdp_action",
		Params:        cdpParams,
		TabID:         tabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction(action, "", map[string]any{"x": x, "y": y, "method": "cdp"})

	return h.MaybeWaitForCommand(req, correlationID, args, action+" queued")
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
