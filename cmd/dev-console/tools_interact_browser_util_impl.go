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

// handleScreenshotAliasImpl provides backward compatibility for clients that call
// interact({action:"screenshot"}). The canonical API remains observe({what:"screenshot"}).
func (h *interactActionHandler) handleScreenshotAliasImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return observe.GetScreenshot(h.parent, req, args)
}

func (h *interactActionHandler) handleSubtitleImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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
	h.parent.armEvidenceForCommand(correlationID, "subtitle", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "subtitle",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.parent.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	queuedMsg := "Subtitle set"
	if *params.Text == "" {
		queuedMsg = "Subtitle cleared"
	}

	return h.parent.MaybeWaitForCommand(req, correlationID, args, queuedMsg)
}
