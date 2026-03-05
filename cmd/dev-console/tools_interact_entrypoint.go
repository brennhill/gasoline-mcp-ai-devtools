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
func (h *ToolHandler) toolInteract(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	what := params.What
	usedAliasParam := ""
	if what != "" && params.Action != "" && params.Action != what {
		return whatAliasConflictResponse(req, "action", what, params.Action, h.interactAction().getValidInteractActions())
	}
	if what == "" {
		what = params.Action
		if what != "" {
			usedAliasParam = "action"
		}
	}

	if what == "" {
		validActions := h.interactAction().getValidInteractActions()
		return fail(req, ErrMissingParam,
			"Required dispatch parameter is missing: provide 'what' (or deprecated alias 'action')",
			"Add 'what' (preferred) or 'action' and call again",
			withParam("what"),
			withHint("Valid values: "+validActions))
	}

	if _, err := parseEvidenceMode(args); err != nil {
		resp := fail(req, ErrInvalidParam,
			"Invalid 'evidence' value",
			"Use evidence='off' (default), 'on_mutation', or 'always'",
			withParam("evidence"))
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	}

	// Quiet alias: async → background.
	args = mergeAsyncAlias(args)

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

	resp := h.interactAction().dispatchInteractAction(req, args, what)

	if composableParams.Subtitle != nil && what != "subtitle" && resp.Error == nil {
		h.interactAction().queueComposableSubtitle(req, *composableParams.Subtitle)
	}

	hasComposableSideEffects := false
	if composableParams.AutoDismiss && what == "navigate" && !isErrorResponse(resp) {
		h.interactAction().queueComposableAutoDismiss(req)
		hasComposableSideEffects = true
	}
	if composableParams.WaitForStable && (what == "navigate" || what == "click") && !isErrorResponse(resp) {
		h.interactAction().queueComposableWaitForStable(req, composableParams.StabilityMs)
		hasComposableSideEffects = true
	}
	if composableParams.ActionDiff && !isErrorResponse(resp) {
		h.interactAction().queueComposableActionDiff(req)
		hasComposableSideEffects = true
	}

	// Composable side effects (auto_dismiss, wait_for_stable, action_diff) are fire-and-forget
	// queries that the extension processes asynchronously. When a screenshot is also requested,
	// we need a brief delay to let the extension finish processing before capture.
	if hasComposableSideEffects && composableParams.IncludeScreenshot {
		time.Sleep(composableSideEffectDelay)
	}
	if composableParams.IncludeScreenshot && !isErrorResponse(resp) {
		resp = h.interactAction().appendScreenshotToResponse(resp, req)
	}
	if composableParams.IncludeInteractive && !isErrorResponse(resp) {
		resp = h.interactAction().appendInteractiveToResponse(resp, req)
	}

	resp = appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	return resp
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
