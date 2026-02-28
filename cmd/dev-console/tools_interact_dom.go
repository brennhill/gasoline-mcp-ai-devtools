// tools_interact_dom.go — DOM primitive action handlers and hardware click.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/queries"
	act "github.com/dev-console/dev-console/internal/tools/interact"
)

// domPrimitiveActions delegates to the interact package.
var domPrimitiveActions = act.DOMPrimitiveActions

// domActionRequiredParams delegates to the interact package.
var domActionRequiredParams = act.DOMActionRequiredParams

// domActionToReproType delegates to the interact package.
var domActionToReproType = act.DOMActionToReproType

// parseSelectorForReproduction delegates to the interact package.
var parseSelectorForReproduction = act.ParseSelectorForReproduction

// normalizeDOMActionArgs rewrites interact args so extension-facing dom_action
// payloads always carry canonical "action", while preserving user-facing "what".
func normalizeDOMActionArgs(args json.RawMessage, action string) json.RawMessage {
	var payload map[string]any
	if err := json.Unmarshal(args, &payload); err != nil || payload == nil {
		payload = map[string]any{}
	}
	payload["action"] = action
	if _, hasScopeRect := payload["scope_rect"]; !hasScopeRect {
		if annotationRect, hasAnnotationRect := payload["annotation_rect"]; hasAnnotationRect {
			payload["scope_rect"] = annotationRect
		}
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return args
	}
	return normalized
}

func (h *ToolHandler) handleDOMPrimitive(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	var params struct {
		Selector      string   `json:"selector"`
		ScopeSelector string   `json:"scope_selector,omitempty"`
		ElementID     string   `json:"element_id,omitempty"`
		Index         *int     `json:"index,omitempty"`
		IndexGen      string   `json:"index_generation,omitempty"`
		Text          string   `json:"text,omitempty"`
		Value         string   `json:"value,omitempty"`
		Clear         bool     `json:"clear,omitempty"`
		Checked       *bool    `json:"checked,omitempty"`
		Name          string   `json:"name,omitempty"`
		TimeoutMs     int      `json:"timeout_ms,omitempty"`
		TabID         int      `json:"tab_id,omitempty"`
		Analyze       bool     `json:"analyze,omitempty"`
		X             *float64 `json:"x,omitempty"`
		Y             *float64 `json:"y,omitempty"`
		URLContains   string   `json:"url_contains,omitempty"`
		Absent        bool     `json:"absent,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	// If x/y coordinates provided on a click action, escalate to CDP for hardware-level click
	if action == "click" && params.X != nil && params.Y != nil {
		return h.handleCDPClick(req, args, action, *params.X, *params.Y, params.TabID)
	}

	// Resolve index to selector if index is provided and selector is empty
	if params.Index != nil && params.Selector == "" && params.ElementID == "" {
		sel, ok, stale, latestGeneration := h.resolveIndexToSelector(req.ClientID, params.TabID, *params.Index, params.IndexGen)
		if stale {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				formatIndexGenerationConflict(params.IndexGen, latestGeneration),
				"Re-run interact with what='list_interactive' for the current page context, then retry with the returned index_generation.",
				withParam("index_generation"),
				withParam("index"),
			)}
		}
		if !ok {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				fmt.Sprintf("Element index %d not found for tab_id=%d. Call list_interactive first to refresh the element index for this tab/client scope.", *params.Index, params.TabID),
				"Call interact with what='list_interactive' first (same tab/client scope), then use the returned index.",
				withParam("index"),
				withParam("tab_id"),
			)}
		}
		params.Selector = sel
		// Rewrite args to include the resolved selector
		var rawArgs map[string]json.RawMessage
		if json.Unmarshal(args, &rawArgs) == nil {
			selectorJSON, _ := json.Marshal(sel)
			rawArgs["selector"] = selectorJSON
			args, _ = json.Marshal(rawArgs)
		}
	}

	selectorOptionalActions := map[string]bool{
		"open_composer":          true,
		"submit_active_composer": true,
		"confirm_top_dialog":     true,
		"dismiss_top_overlay":    true,
		"auto_dismiss_overlays":  true,
		"wait_for_stable":        true,
		"key_press":              true,
		"wait_for":               true,
	}
	if params.Selector == "" && params.ElementID == "" && !selectorOptionalActions[action] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"Required parameter 'selector', 'element_id', or 'index' is missing",
			"Add 'selector' (CSS or semantic selector), or use 'element_id'/'index' from list_interactive results.",
			withParam("selector"),
		)}
	}

	// wait_for: require at least one condition and reject incompatible combinations
	if action == "wait_for" {
		hasSelector := params.Selector != "" || params.ElementID != ""
		hasText := params.Text != ""
		hasURL := params.URLContains != ""
		condCount := 0
		if hasSelector || params.Absent {
			condCount++
		}
		if hasText {
			condCount++
		}
		if hasURL {
			condCount++
		}
		if condCount == 0 {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrMissingParam,
				"wait_for requires at least one condition: selector, text, or url_contains",
				"Provide 'selector' (wait for element), 'text' (wait for text), or 'url_contains' (wait for URL change).",
				withParam("selector"),
			)}
		}
		if condCount > 1 {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"wait_for conditions are mutually exclusive: use only one of selector, text, or url_contains",
				"Choose a single wait condition per call.",
			)}
		}
		if params.Absent && !hasSelector {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrMissingParam,
				"wait_for with absent requires a selector",
				"Provide 'selector' to specify which element to wait to disappear.",
				withParam("selector"),
			)}
		}
	}

	if errResp, failed := validateDOMActionParams(req, action, params.Text, params.Value, params.Name); failed {
		return errResp
	}

	contextOpts := []func(*StructuredError){withAction(action)}
	if params.Selector != "" {
		contextOpts = append(contextOpts, withSelector(params.Selector))
	}
	if resp, blocked := h.requirePilot(req, contextOpts...); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req, contextOpts...); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req, contextOpts...); blocked {
		return resp
	}

	args = normalizeDOMActionArgs(args, action)

	correlationID := newCorrelationID("dom_" + action)
	h.armEvidenceForCommand(correlationID, action, args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordDOMPrimitiveAction(action, params.Selector, params.Text, params.Value)

	return h.MaybeWaitForCommand(req, correlationID, args, action+" queued")
}

// recordDOMPrimitiveAction records a DOM primitive action with reproduction-compatible
// type and field mapping. Falls back to "dom_<action>" for actions without a mapping.
func (h *ToolHandler) recordDOMPrimitiveAction(action, selector, text, value string) {
	reproType, ok := domActionToReproType[action]
	if !ok {
		// Unmapped actions (get_text, get_value, etc.) — keep dom_ prefix for audit trail
		h.recordAIAction("dom_"+action, "", map[string]any{"selector": selector})
		return
	}

	selectors := parseSelectorForReproduction(selector)
	ea := capture.EnhancedAction{
		Type:      reproType,
		Selectors: selectors,
	}

	// Populate type-specific fields
	switch action {
	case "type":
		ea.Value = text
	case "key_press":
		ea.Key = text
	case "select":
		ea.SelectedValue = value
	}

	h.recordAIEnhancedAction(ea)
}

// validateDOMActionParams checks action-specific required parameters.
// Returns (response, true) if validation failed, or (zero, false) if valid.
func validateDOMActionParams(req JSONRPCRequest, action, text, value, name string) (JSONRPCResponse, bool) {
	rule, ok := domActionRequiredParams[action]
	if !ok {
		return JSONRPCResponse{}, false
	}
	var paramValue string
	switch rule.Field {
	case "text":
		paramValue = text
	case "value":
		paramValue = value
	case "name":
		paramValue = name
	}
	if paramValue == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, rule.Message, rule.Retry, withParam(rule.Field))}, true
	}
	return JSONRPCResponse{}, false
}

// handleHardwareClick dispatches a coordinate-based click via CDP Input.dispatchMouseEvent.
// This gives LLMs an explicit "I see coordinates in a screenshot, click there" path.
func (h *ToolHandler) handleHardwareClick(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		X     *float64 `json:"x"`
		Y     *float64 `json:"y"`
		TabID int      `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.X == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'x' is missing", "Add the 'x' coordinate (pixels from left)", withParam("x"))}
	}
	if params.Y == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'y' is missing", "Add the 'y' coordinate (pixels from top)", withParam("y"))}
	}

	return h.handleCDPClick(req, args, "hardware_click", *params.X, *params.Y, params.TabID)
}

// handleCDPClick creates a cdp_action query for a hardware-level click at coordinates.
func (h *ToolHandler) handleCDPClick(req JSONRPCRequest, args json.RawMessage, action string, x, y float64, tabID int) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req, withAction(action)); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req, withAction(action)); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req, withAction(action)); blocked {
		return resp
	}

	correlationID := newCorrelationID("cdp_click")
	h.armEvidenceForCommand(correlationID, action, args, req.ClientID)

	cdpParams, _ := json.Marshal(map[string]any{
		"action": "click",
		"x":      x,
		"y":      y,
	})

	query := queries.PendingQuery{
		Type:          "cdp_action",
		Params:        cdpParams,
		TabID:         tabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction(action, "", map[string]any{"x": x, "y": y, "method": "cdp"})

	return h.MaybeWaitForCommand(req, correlationID, args, action+" queued")
}
