// tools_interact_browser.go — Browser action handlers for navigate, tabs, history.

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/queries"
	act "github.com/dev-console/dev-console/internal/tools/interact"
	"github.com/dev-console/dev-console/internal/tools/observe"
)

// validWorldValues delegates to the interact package.
var validWorldValues = act.ValidWorldValues

// truncateToLen delegates to the interact package.
var truncateToLen = act.TruncateToLen

func (h *ToolHandler) handleHighlight(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

func (h *ToolHandler) handleExecuteJS(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

// handleScreenshotAlias provides backward compatibility for clients that call
// interact({action:"screenshot"}). The canonical API remains observe({what:"screenshot"}).
func (h *ToolHandler) handleScreenshotAlias(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return observe.GetScreenshot(h, req, args)
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
