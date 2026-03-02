// Purpose: Dispatches observe tool modes and coordinates alias resolution, validation, and response augmentation.
// Why: Keeps observe entrypoint behavior explicit while mode registry and response helpers live in focused companion files.
// Docs: docs/features/feature/observe/index.md

package main

import (
	"encoding/json"
)

// observeModeParams captures accepted observe mode selectors.
// `mode`/`action` are deprecated aliases for `what` and remain for compatibility.
type observeModeParams struct {
	What   string `json:"what"`
	Mode   string `json:"mode"`
	Action string `json:"action"`
}

func parseObserveModeParams(args json.RawMessage) (observeModeParams, error) {
	params := observeModeParams{}
	if len(args) == 0 {
		return params, nil
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return observeModeParams{}, err
	}
	return params, nil
}

func resolveObserveMode(req JSONRPCRequest, params observeModeParams) (what string, usedAliasParam string, errResp *JSONRPCResponse) {
	what = params.What
	if what != "" && params.Mode != "" && params.Mode != what {
		resp := whatAliasConflictResponse(req, "mode", what, params.Mode, getValidObserveModes())
		return "", "", &resp
	}
	if what != "" && params.Action != "" && params.Action != what {
		resp := whatAliasConflictResponse(req, "action", what, params.Action, getValidObserveModes())
		return "", "", &resp
	}
	if what == "" {
		if params.Mode != "" {
			what = params.Mode
			usedAliasParam = "mode"
		} else if params.Action != "" {
			what = params.Action
			usedAliasParam = "action"
		}
	}
	if what == "" {
		validModes := getValidObserveModes()
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validModes))}
		return "", usedAliasParam, &resp
	}
	if alias, ok := observeAliases[what]; ok {
		what = alias
	}
	return what, usedAliasParam, nil
}

// toolObserve dispatches observe requests based on the 'what' parameter.
func (h *ToolHandler) toolObserve(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	params, err := parseObserveModeParams(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	what, usedAliasParam, errResp := resolveObserveMode(req, params)
	if errResp != nil {
		return *errResp
	}

	handler, ok := observeHandlers[what]
	if !ok {
		validModes := getValidObserveModes()
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown observe mode: "+what, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes))}
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	}

	args = h.maybeInjectSummary(args)

	resp := handler(h, req, args)

	// Warn when extension is disconnected (except for server-side modes that don't need it)
	if !h.capture.IsExtensionConnected() && !serverSideObserveModes[what] {
		resp = h.prependDisconnectWarning(resp)
	}

	// Piggyback alerts: append as second content block if any pending
	alerts := h.drainAlerts()
	if len(alerts) > 0 {
		resp = h.appendAlertsToResponse(resp, alerts)
	}

	resp = appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	return resp
}
