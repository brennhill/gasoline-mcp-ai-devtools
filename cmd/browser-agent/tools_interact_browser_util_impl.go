// Purpose: Implements shared browser-action utilities and aliases.
// Why: Consolidates helper/alias behavior separate from main navigation handlers.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
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
	const insecurePrefix = "kaboom-insecure://"
	if !strings.HasPrefix(strings.ToLower(trimmed), insecurePrefix) {
		return trimmed, nil
	}
	if h.parent.capture == nil {
		return "", fmt.Errorf("resolve insecure URL: capture not initialized. Initialize capture before using insecure mode")
	}

	mode, _, _ := h.parent.capture.GetSecurityMode()
	if mode != capture.SecurityModeInsecureProxy {
		return "", fmt.Errorf("resolve insecure URL: requires security_mode=insecure_proxy. Set security mode before navigating")
	}

	target := strings.TrimSpace(trimmed[len(insecurePrefix):])
	if target == "" {
		return "", fmt.Errorf("resolve insecure URL: target URL is empty. Provide a URL after the kaboom-insecure:// prefix")
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("invalid kaboom-insecure target URL: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("kaboom-insecure target URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("kaboom-insecure target URL must include host")
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
// Uses commandBuilder to eliminate repeated boilerplate.
func (h *interactActionHandler) queueBrowserAction(req JSONRPCRequest, args json.RawMessage, opts browserActionOpts) JSONRPCResponse {
	actionParams := opts.params
	if actionParams == nil {
		actionParams = buildQueryParams(map[string]any{"action": opts.action})
	}

	recordAction := opts.recordAction
	if recordAction == "" {
		recordAction = opts.action
	}

	cmd := h.newCommand(opts.action).
		correlationPrefix(opts.correlationPfx).
		reason(opts.action).
		queryType("browser_action").
		queryParams(actionParams).
		tabID(opts.tabID).
		recordAction(recordAction, opts.recordURL, opts.recordExtra).
		queuedMessage(opts.queuedMsg)

	if opts.skipTabGuard {
		cmd.guards(h.parent.requirePilot, h.parent.requireExtension)
	} else {
		cmd.guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking)
	}

	return cmd.execute(req, args)
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

	queuedMsg := "Subtitle set"
	if *params.Text == "" {
		queuedMsg = "Subtitle cleared"
	}

	return h.newCommand("subtitle").
		correlationPrefix("subtitle").
		reason("subtitle").
		queryType("subtitle").
		queryParams(args).
		queuedMessage(queuedMsg).
		execute(req, args)
}
