// tools_interact.go — MCP interact tool dispatcher and response utilities.

package main

import (
	"encoding/json"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/tools/observe"
)

// randIntn returns a random int in [0, n). Uses math/rand/v2 which auto-seeds.
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	return rand.IntN(n)
}

// interactHandler is the function signature for interact action handlers.
type interactHandler func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// interactDispatch returns the dispatch map for interact actions.
// Initialized once via sync.Once; safe for concurrent use.
func (h *ToolHandler) interactDispatch() map[string]interactHandler {
	h.interactOnce.Do(func() {
		h.interactHandlers = map[string]interactHandler{
			"highlight":                 h.handleHighlight,
			"save_state":                h.handleStateSave,
			"state_save":                h.handleStateSave, // backward-compatible alias
			"load_state":                h.handleStateLoad,
			"state_load":                h.handleStateLoad, // backward-compatible alias
			"list_states":               h.handleStateList,
			"state_list":                h.handleStateList, // backward-compatible alias
			"delete_state":              h.handleStateDelete,
			"state_delete":              h.handleStateDelete, // backward-compatible alias
			"set_storage":               h.handleSetStorage,
			"delete_storage":            h.handleDeleteStorage,
			"clear_storage":             h.handleClearStorage,
			"set_cookie":                h.handleSetCookie,
			"delete_cookie":             h.handleDeleteCookie,
			"execute_js":                h.handleExecuteJS,
			"navigate":                  h.handleBrowserActionNavigate,
			"refresh":                   h.handleBrowserActionRefresh,
			"back":                      h.handleBrowserActionBack,
			"forward":                   h.handleBrowserActionForward,
			"new_tab":                   h.handleBrowserActionNewTab,
			"switch_tab":               h.handleBrowserActionSwitchTab,
			"close_tab":                h.handleBrowserActionCloseTab,
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
			"clipboard_read":           h.handleClipboardRead,
			"clipboard_write":          h.handleClipboardWrite,
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
		Subtitle           *string `json:"subtitle"`
		IncludeScreenshot  bool    `json:"include_screenshot"`
		IncludeInteractive bool    `json:"include_interactive"`
		AutoDismiss        bool    `json:"auto_dismiss"`
		WaitForStable      bool    `json:"wait_for_stable"`
		StabilityMs        int     `json:"stability_ms,omitempty"`
		ActionDiff         bool    `json:"action_diff"`
	}
	lenientUnmarshal(args, &composableParams)

	resp := h.dispatchInteractAction(req, args, what)

	// If a composable subtitle was provided on a non-subtitle action, queue it.
	// Only queue if the primary action didn't fail (avoid subtitle on error).
	if composableParams.Subtitle != nil && what != "subtitle" && resp.Error == nil {
		h.queueComposableSubtitle(req, *composableParams.Subtitle)
	}

	// Queue composable side-effects in correct order: dismiss overlays → wait for stable → action_diff.
	// These are processed by the extension before the screenshot is captured.
	hasComposableSideEffects := false

	// If auto_dismiss was requested on navigate and succeeded, queue auto_dismiss_overlays.
	if composableParams.AutoDismiss && what == "navigate" && !isErrorResponse(resp) {
		h.queueComposableAutoDismiss(req)
		hasComposableSideEffects = true
	}

	// If wait_for_stable was requested on navigate/click and succeeded, queue wait_for_stable.
	if composableParams.WaitForStable && (what == "navigate" || what == "click") && !isErrorResponse(resp) {
		h.queueComposableWaitForStable(req, composableParams.StabilityMs)
		hasComposableSideEffects = true
	}

	// If action_diff was requested and the action succeeded, queue mutation capture (#343).
	if composableParams.ActionDiff && !isErrorResponse(resp) {
		h.queueComposableActionDiff(req)
		hasComposableSideEffects = true
	}

	// Wait briefly for composable side-effects to complete before capturing screenshot.
	// This ensures screenshots show the page AFTER overlays are dismissed and DOM stabilizes (#9.3.4).
	// 300ms is a pragmatic heuristic: most overlay dismissals and DOM mutations settle within
	// 100-200ms, and the composable commands (auto_dismiss, wait_for_stable, action_diff) run
	// asynchronously in the extension. If the delay is insufficient, the screenshot captures
	// a pre-effect state — this degrades gracefully (stale screenshot) rather than failing.
	if hasComposableSideEffects && composableParams.IncludeScreenshot {
		time.Sleep(300 * time.Millisecond)
	}

	// If include_screenshot was requested and the action succeeded, capture a screenshot
	// and append it as an inline image content block.
	if composableParams.IncludeScreenshot && !isErrorResponse(resp) {
		resp = h.appendScreenshotToResponse(resp, req)
	}

	// If include_interactive was requested and the action succeeded, run list_interactive
	// and merge the results into the response text.
	if composableParams.IncludeInteractive && !isErrorResponse(resp) {
		resp = h.appendInteractiveToResponse(resp, req)
	}

	resp = appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	return resp
}

// readOnlyInteractActions lists actions that should NOT have jitter applied.
var readOnlyInteractActions = map[string]bool{
	"list_interactive":          true,
	"get_text":                  true,
	"get_value":                 true,
	"get_attribute":             true,
	"query":                     true,
	"screenshot":                true,
	"list_states":               true,
	"state_list":                true,
	"get_readable":              true,
	"get_markdown":              true,
	"explore_page":              true,
	"run_a11y_and_export_sarif": true,
	"wait_for":                  true,
	"wait_for_stable":           true,
	"auto_dismiss_overlays":     true,
	"batch":                     true,
	"highlight":                 true,
	"subtitle":                  true,
	"clipboard_read":            true,
}

// applyJitter sleeps for a random duration up to maxMs if jitter is configured.
// Returns the actual jitter applied in milliseconds.
func (h *ToolHandler) applyJitter(action string) int {
	if readOnlyInteractActions[action] {
		return 0
	}
	h.jitterMu.RLock()
	maxMs := h.actionJitterMaxMs
	h.jitterMu.RUnlock()
	if maxMs <= 0 {
		return 0
	}
	jitterMs := randIntn(maxMs)
	if jitterMs > 0 {
		time.Sleep(time.Duration(jitterMs) * time.Millisecond)
	}
	return jitterMs
}

// dispatchInteractAction routes an action to the correct handler using
// the dispatch map for named actions and the DOM primitive set for DOM actions.
func (h *ToolHandler) dispatchInteractAction(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	h.applyJitter(action)
	if handler, ok := h.interactDispatch()[action]; ok {
		return handler(req, args)
	}
	if domPrimitiveActions[action] {
		return h.handleDOMPrimitive(req, args, action)
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown interact action: "+action, "Use a valid action from the 'what' enum", withParam("what"))}
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

// appendInteractiveToResponse runs list_interactive and appends the results
// as an additional text content block. Best-effort: returns original on failure.
func (h *ToolHandler) appendInteractiveToResponse(resp JSONRPCResponse, req JSONRPCRequest) JSONRPCResponse {
	listReq := JSONRPCRequest{JSONRPC: "2.0", ID: req.ID, ClientID: req.ClientID}
	listArgs, _ := json.Marshal(map[string]any{"what": "list_interactive", "visible_only": true})
	listResp := h.handleListInteractive(listReq, listArgs)

	var listResult MCPToolResult
	if err := json.Unmarshal(listResp.Result, &listResult); err != nil || listResult.IsError {
		return resp
	}

	// Append the list_interactive text content to the primary response
	for _, block := range listResult.Content {
		if block.Type == "text" && block.Text != "" {
			var result MCPToolResult
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				return resp
			}
			result.Content = append(result.Content, MCPContentBlock{
				Type: "text",
				Text: "\n--- Interactive Elements ---\n" + block.Text,
			})
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

// Element indexing functions moved to tools_interact_elements.go
