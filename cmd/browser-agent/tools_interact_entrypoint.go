// Purpose: Implements top-level interact entrypoint parsing and composable behavior wiring.
// Why: Separates request-shape validation/composition from action dispatch and jitter plumbing.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"time"
)

const (
	// composableSideEffectDelay is the pause before screenshot capture when composable
	// side effects (auto_dismiss, wait_for_stable, action_diff) are in flight.
	composableSideEffectDelay = 300 * time.Millisecond
)

// toolInteract dispatches interact requests based on the 'what' parameter.
// Uses the unified toolRegistry/dispatchTool infrastructure for mode resolution
// and alias handling, with composable side effects applied post-dispatch.
func (h *ToolHandler) toolInteract(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse composable params before dispatch (PostDispatch doesn't receive args).
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

	// Build the registry with lazily-populated handlers and valid modes.
	reg := interactRegistry
	reg.Handlers = getInteractHandlers()
	reg.Resolution.ValidModes = getValidInteractActions()

	// Resolve mode early for composable side-effect checks.
	// We need the resolved 'what' value but dispatchTool handles it internally.
	// Parse it here for composable logic, then let dispatchTool handle the full dispatch.
	what := resolveWhatForComposable(args, interactAliasParams)

	resp := h.dispatchTool(req, args, reg)

	// Apply composable side effects (these need the resolved 'what' and original args).
	if composableParams.Subtitle != nil && what != "subtitle" && resp.Error == nil {
		h.interactAction().QueueComposableSubtitle(req, *composableParams.Subtitle)
	}

	hasComposableSideEffects := false
	if composableParams.AutoDismiss && what == "navigate" && !isErrorResponse(resp) {
		h.interactAction().QueueComposableAutoDismiss(req)
		hasComposableSideEffects = true
	}
	if composableParams.WaitForStable && (what == "navigate" || what == "click") && !isErrorResponse(resp) {
		h.interactAction().QueueComposableWaitForStable(req, composableParams.StabilityMs)
		hasComposableSideEffects = true
	}
	if composableParams.ActionDiff && !isErrorResponse(resp) {
		h.interactAction().QueueComposableActionDiff(req)
		hasComposableSideEffects = true
	}

	// Composable side effects (auto_dismiss, wait_for_stable, action_diff) are fire-and-forget
	// queries that the extension processes asynchronously. When a screenshot is also requested,
	// we need a brief delay to let the extension finish processing before capture.
	if hasComposableSideEffects && composableParams.IncludeScreenshot {
		time.Sleep(composableSideEffectDelay)
	}
	if composableParams.IncludeScreenshot && !isErrorResponse(resp) {
		resp = h.interactAction().AppendScreenshotToResponse(resp, req)
	}
	if composableParams.IncludeInteractive && !isErrorResponse(resp) {
		resp = h.interactAction().AppendInteractiveToResponse(resp, req)
	}

	return resp
}

// resolveWhatForComposable extracts the resolved 'what' value from args for composable
// side-effect logic. This is a lightweight parse — the full resolution with conflict
// detection happens inside dispatchTool.
func resolveWhatForComposable(args json.RawMessage, aliasDefs []modeAlias) string {
	if len(args) == 0 {
		return ""
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return ""
	}
	// Try canonical 'what' first.
	if v, ok := raw["what"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil && s != "" {
			return s
		}
	}
	// Fall back to alias params.
	for _, ad := range aliasDefs {
		if v, ok := raw[ad.JSONField]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// mergeAsyncAlias rewrites {"async":true} → {"background":true} in raw JSON args.
// If "background" is already set, the explicit value takes precedence.
func mergeAsyncAlias(args json.RawMessage) json.RawMessage {
	if len(args) == 0 {
		return args
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return args
	}
	asyncVal, hasAsync := raw["async"]
	_, hasBackground := raw["background"]
	if !hasAsync || hasBackground {
		return args
	}
	raw["background"] = asyncVal
	delete(raw, "async")
	merged, err := json.Marshal(raw)
	if err != nil {
		return args
	}
	return merged
}
