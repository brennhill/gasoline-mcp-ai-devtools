// Purpose: Dispatches DOM primitive actions (click, type, select, check, focus, scroll, hover, key_press) and hardware click to the extension.
// Why: Maps each low-level DOM interaction to a pending query with selector/element resolution and timeout handling.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
	act "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/interact"
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

func (h *interactActionHandler) handleDOMPrimitive(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	params, err := parseDOMPrimitiveParams(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	// If x/y coordinates provided on a click action, escalate to CDP for hardware-level click
	if action == "click" && params.X != nil && params.Y != nil {
		return h.parent.handleCDPClick(req, args, action, *params.X, *params.Y, params.TabID)
	}

	var failed bool
	var errResp JSONRPCResponse
	args, errResp, failed = h.resolveDOMSelectorFromIndex(req, args, &params)
	if failed {
		return errResp
	}

	if errResp, failed := validateDOMSelectorRequirement(req, action, params); failed {
		return errResp
	}

	if errResp, failed := validateWaitForConditions(req, action, params); failed {
		return errResp
	}

	if errResp, failed := validateDOMActionParams(req, action, params.Text, params.Value, params.Name); failed {
		return errResp
	}

	contextOpts := domActionContextOptions(action, params.Selector)
	if resp, blocked := h.parent.requirePilot(req, contextOpts...); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req, contextOpts...); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req, contextOpts...); blocked {
		return resp
	}

	args = normalizeDOMActionArgs(args, action)

	correlationID := newCorrelationID("dom_" + action)
	h.parent.armEvidenceForCommand(correlationID, action, args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.parent.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.parent.recordDOMPrimitiveAction(action, params.Selector, params.Text, params.Value)

	return h.parent.MaybeWaitForCommand(req, correlationID, args, action+" queued")
}
