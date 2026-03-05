// Purpose: Implements shared browser-action utilities and aliases.
// Why: Consolidates helper/alias behavior separate from main navigation handlers.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/observe"
)

// stashPerfSnapshotImpl saves the current performance snapshot as a "before" baseline
// for perf_diff computation, keyed by correlation ID.
func (h *interactActionHandler) stashPerfSnapshotImpl(correlationID string) {
	_, _, trackedURL := h.parent.capture.GetTrackingStatus()
	u, err := url.Parse(trackedURL)
	if err != nil || u.Path == "" {
		return
	}
	if snap, ok := h.parent.capture.GetPerformanceSnapshotByURL(u.Path); ok {
		h.parent.capture.StoreBeforeSnapshot(correlationID, snap)
	}
}

func (h *interactActionHandler) resolveNavigateURLImpl(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	const insecurePrefix = "gasoline-insecure://"
	if !strings.HasPrefix(strings.ToLower(trimmed), insecurePrefix) {
		return trimmed, nil
	}
	if h.parent.capture == nil {
		return "", fmt.Errorf("gasoline-insecure URL is unavailable because capture is not initialized")
	}

	mode, _, _ := h.parent.capture.GetSecurityMode()
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
	if h.parent.server != nil {
		port = h.parent.server.getListenPort()
	}
	return fmt.Sprintf("http://127.0.0.1:%d/insecure-proxy?target=%s", port, url.QueryEscape(target)), nil
}

// browserActionOpts configures the queueBrowserAction helper.
type browserActionOpts struct {
	action         string         // Action name (e.g. "back", "forward", "activate_tab")
	correlationPfx string         // Correlation ID prefix (e.g. "back", "forward")
	params         json.RawMessage // Serialized action params; nil uses `{"action":"<action>"}`
	tabID          int            // Tab ID for the pending query (0 = default)
	skipTabGuard   bool           // If true, skip requireTabTracking guard
	queuedMsg      string         // Queued message for MaybeWaitForCommand
	recordAction   string         // Action type for recordAIAction (defaults to action)
	recordURL      string         // URL for recordAIAction
	recordExtra    map[string]any // Extra details for recordAIAction
}

// queueBrowserAction is the shared helper for simple browser actions that follow
// the guard → correlate → arm evidence → enqueue → record → wait pattern.
// Eliminates the repeated 15+ line boilerplate in back, forward, refresh, activate_tab, close_tab, etc.
func (h *interactActionHandler) queueBrowserAction(req JSONRPCRequest, args json.RawMessage, opts browserActionOpts) JSONRPCResponse {
	if resp, blocked := checkGuards(req, h.parent.requirePilot, h.parent.requireExtension); blocked {
		return resp
	}
	if !opts.skipTabGuard {
		if resp, blocked := h.parent.requireTabTracking(req); blocked {
			return resp
		}
	}

	correlationID := newCorrelationID(opts.correlationPfx)
	h.armEvidenceForCommand(correlationID, opts.action, args, req.ClientID)

	actionParams := opts.params
	if actionParams == nil {
		actionParams, _ = json.Marshal(map[string]string{"action": opts.action})
	}

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionParams,
		TabID:         opts.tabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	recordAction := opts.recordAction
	if recordAction == "" {
		recordAction = opts.action
	}
	h.parent.recordAIAction(recordAction, opts.recordURL, opts.recordExtra)

	return h.parent.MaybeWaitForCommand(req, correlationID, args, opts.queuedMsg)
}

// handleScreenshotAliasImpl provides backward compatibility for clients that call
// interact({action:"screenshot"}). The canonical API remains observe({what:"screenshot"}).
func (h *interactActionHandler) handleScreenshotAliasImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return observe.GetScreenshot(h.parent, req, args)
}

func (h *interactActionHandler) handleSubtitleImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Text *string `json:"text"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if params.Text == nil {
		return fail(req, ErrMissingParam, "Required parameter 'text' is missing for subtitle action", "Add the 'text' parameter with subtitle text, or empty string to clear", withParam("text"))
	}

	correlationID := newCorrelationID("subtitle")
	h.armEvidenceForCommand(correlationID, "subtitle", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "subtitle",
		Params:        args,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	queuedMsg := "Subtitle set"
	if *params.Text == "" {
		queuedMsg = "Subtitle cleared"
	}

	return h.parent.MaybeWaitForCommand(req, correlationID, args, queuedMsg)
}
