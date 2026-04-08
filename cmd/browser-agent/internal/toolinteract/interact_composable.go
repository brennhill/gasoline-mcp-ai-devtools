// Purpose: Handles composable side-effect parameters (auto_dismiss, wait_for_stable, action_diff) that attach to any interact action.
// Why: Enables agents to combine overlay dismissal, stability waits, and diff capture with primary actions in a single call.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// handleWaitForStable is the named handler for the standalone wait_for_stable action.
// It injects default stability_ms and timeout_ms if not provided, then delegates
// to the standard DOM primitive dispatch.
func (h *InteractActionHandler) HandleWaitForStable(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		StabilityMs int `json:"stability_ms,omitempty"`
		TimeoutMs   int `json:"timeout_ms,omitempty"`
		TabID       int `json:"tab_id,omitempty"`
	}
	mcp.LenientUnmarshal(args, &params)

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

	return h.HandleDOMPrimitive(req, enrichedArgs, "wait_for_stable")
}

// handleAutoDismissOverlays is the named handler for the standalone auto_dismiss_overlays action.
// It delegates to the DOM primitive dispatch, which runs consent framework selectors
// followed by the existing dismiss_top_overlay multi-strategy approach on the extension side.
func (h *InteractActionHandler) HandleAutoDismissOverlays(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.HandleDOMPrimitive(req, args, "auto_dismiss_overlays")
}

// queueComposableAutoDismiss queues an auto_dismiss_overlays command as a side effect.
// Used when auto_dismiss=true is passed as a composable param on navigate.
func (h *InteractActionHandler) QueueComposableAutoDismiss(req mcp.JSONRPCRequest) {
	dismissArgs := mcp.BuildQueryParams(map[string]any{"action": "auto_dismiss_overlays"})
	correlationID := mcp.NewCorrelationID("dom_auto_dismiss_overlays")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        dismissArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return
	}
}

// queueComposableActionDiff queues an action_diff command as a side effect.
// Used when action_diff=true is passed as a composable param on any mutating action.
// The extension instruments a MutationObserver after the main action, captures mutations,
// and returns a structured summary of what changed (overlays, toasts, form errors, etc.).
func (h *InteractActionHandler) QueueComposableActionDiff(req mcp.JSONRPCRequest) {
	diffArgs := mcp.BuildQueryParams(map[string]any{
		"action":     "action_diff",
		"timeout_ms": 3000,
	})
	correlationID := mcp.NewCorrelationID("dom_action_diff")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        diffArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return
	}
}

// queueComposableWaitForStable queues a wait_for_stable command as a side effect.
// Used when wait_for_stable=true is passed as a composable param on navigate or click.
func (h *InteractActionHandler) QueueComposableWaitForStable(req mcp.JSONRPCRequest, stabilityMs int) {
	if stabilityMs <= 0 {
		stabilityMs = 500
	}
	timeoutMs := 5000

	stableArgs := mcp.BuildQueryParams(map[string]any{
		"action":       "wait_for_stable",
		"stability_ms": stabilityMs,
		"timeout_ms":   timeoutMs,
	})
	correlationID := mcp.NewCorrelationID("dom_wait_for_stable")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        stableArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return
	}
}

// queueComposableSubtitle queues a subtitle command as a side effect of another action.
func (h *InteractActionHandler) QueueComposableSubtitle(req mcp.JSONRPCRequest, text string) {
	subtitleArgs := mcp.BuildQueryParams(map[string]any{"text": text})
	subtitleQuery := queries.PendingQuery{
		Type:          "subtitle",
		Params:        subtitleArgs,
		CorrelationID: mcp.NewCorrelationID("subtitle"),
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, subtitleQuery, queries.AsyncCommandTimeout); blocked {
		return
	}
}
