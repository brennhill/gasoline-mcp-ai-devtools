// Purpose: Handles composable side-effect parameters (auto_dismiss, wait_for_stable, action_diff) that attach to any interact action.
// Why: Enables agents to combine overlay dismissal, stability waits, and diff capture with primary actions in a single call.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// handleWaitForStable is the named handler for the standalone wait_for_stable action.
// It injects default stability_ms and timeout_ms if not provided, then delegates
// to the standard DOM primitive dispatch.
func (h *ToolHandler) handleWaitForStable(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StabilityMs int `json:"stability_ms,omitempty"`
		TimeoutMs   int `json:"timeout_ms,omitempty"`
		TabID       int `json:"tab_id,omitempty"`
	}
	lenientUnmarshal(args, &params)

	// Apply defaults
	if params.StabilityMs <= 0 {
		params.StabilityMs = 500
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 5000
	}

	// Rewrite args with defaults injected
	var rawArgs map[string]any
	if err := json.Unmarshal(args, &rawArgs); err != nil {
		rawArgs = make(map[string]any)
	}
	rawArgs["stability_ms"] = params.StabilityMs
	rawArgs["timeout_ms"] = params.TimeoutMs
	enrichedArgs, _ := json.Marshal(rawArgs)

	return h.interactAction().handleDOMPrimitive(req, enrichedArgs, "wait_for_stable")
}

// handleAutoDismissOverlays is the named handler for the standalone auto_dismiss_overlays action.
// It delegates to the DOM primitive dispatch, which runs consent framework selectors
// followed by the existing dismiss_top_overlay multi-strategy approach on the extension side.
func (h *ToolHandler) handleAutoDismissOverlays(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleDOMPrimitive(req, args, "auto_dismiss_overlays")
}

// queueComposableAutoDismiss queues an auto_dismiss_overlays command as a side effect.
// Used when auto_dismiss=true is passed as a composable param on navigate.
func (h *ToolHandler) queueComposableAutoDismiss(req JSONRPCRequest) {
	dismissArgs, _ := json.Marshal(map[string]string{"action": "auto_dismiss_overlays"})
	correlationID := newCorrelationID("dom_auto_dismiss_overlays")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        dismissArgs,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)
}

// queueComposableActionDiff queues an action_diff command as a side effect.
// Used when action_diff=true is passed as a composable param on any mutating action.
// The extension instruments a MutationObserver after the main action, captures mutations,
// and returns a structured summary of what changed (overlays, toasts, form errors, etc.).
func (h *ToolHandler) queueComposableActionDiff(req JSONRPCRequest) {
	diffArgs, _ := json.Marshal(map[string]any{
		"action":     "action_diff",
		"timeout_ms": 3000,
	})
	correlationID := newCorrelationID("dom_action_diff")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        diffArgs,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)
}

// queueComposableWaitForStable queues a wait_for_stable command as a side effect.
// Used when wait_for_stable=true is passed as a composable param on navigate or click.
func (h *ToolHandler) queueComposableWaitForStable(req JSONRPCRequest, stabilityMs int) {
	if stabilityMs <= 0 {
		stabilityMs = 500
	}
	timeoutMs := 5000

	stableArgs, _ := json.Marshal(map[string]any{
		"action":       "wait_for_stable",
		"stability_ms": stabilityMs,
		"timeout_ms":   timeoutMs,
	})
	correlationID := newCorrelationID("dom_wait_for_stable")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        stableArgs,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)
}

// queueComposableSubtitle queues a subtitle command as a side effect of another action.
func (h *ToolHandler) queueComposableSubtitle(req JSONRPCRequest, text string) {
	subtitleArgs, _ := json.Marshal(map[string]string{"text": text})
	subtitleQuery := queries.PendingQuery{
		Type:          "subtitle",
		Params:        subtitleArgs,
		CorrelationID: newCorrelationID("subtitle"),
	}
	h.capture.CreatePendingQueryWithTimeout(subtitleQuery, queries.AsyncCommandTimeout, req.ClientID)
}
