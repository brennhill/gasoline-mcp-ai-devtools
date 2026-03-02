// Purpose: Implements top-level interact entrypoint parsing and composable behavior wiring.
// Why: Separates request-shape validation/composition from action dispatch and jitter plumbing.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"time"
)

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

	if composableParams.Subtitle != nil && what != "subtitle" && resp.Error == nil {
		h.queueComposableSubtitle(req, *composableParams.Subtitle)
	}

	hasComposableSideEffects := false
	if composableParams.AutoDismiss && what == "navigate" && !isErrorResponse(resp) {
		h.queueComposableAutoDismiss(req)
		hasComposableSideEffects = true
	}
	if composableParams.WaitForStable && (what == "navigate" || what == "click") && !isErrorResponse(resp) {
		h.queueComposableWaitForStable(req, composableParams.StabilityMs)
		hasComposableSideEffects = true
	}
	if composableParams.ActionDiff && !isErrorResponse(resp) {
		h.queueComposableActionDiff(req)
		hasComposableSideEffects = true
	}

	if hasComposableSideEffects && composableParams.IncludeScreenshot {
		time.Sleep(300 * time.Millisecond)
	}
	if composableParams.IncludeScreenshot && !isErrorResponse(resp) {
		resp = h.appendScreenshotToResponse(resp, req)
	}
	if composableParams.IncludeInteractive && !isErrorResponse(resp) {
		resp = h.appendInteractiveToResponse(resp, req)
	}

	resp = appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	return resp
}
