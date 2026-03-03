// Purpose: Dispatches configure tool modes (health, clear, store, streaming, restart, doctor, security_mode, etc.) to sub-handlers.
// Why: Acts as the top-level router for all session/runtime configuration actions under the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
)

type configureModeParams struct {
	What   string `json:"what"`
	Action string `json:"action"`
}

func parseConfigureModeParams(args json.RawMessage) (configureModeParams, error) {
	params := configureModeParams{}
	if len(args) == 0 {
		return params, nil
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return configureModeParams{}, err
	}
	return params, nil
}

func resolveConfigureAction(req JSONRPCRequest, params configureModeParams) (what string, usedAliasParam string, errResp *JSONRPCResponse) {
	what = params.What
	if what != "" && params.Action != "" && params.Action != what {
		if _, isTopLevelConfigureAction := configureHandlers[params.Action]; isTopLevelConfigureAction {
			resp := whatAliasConflictResponse(req, "action", what, params.Action, getValidConfigureActions())
			return "", "", &resp
		}
	}
	if what == "" {
		what = params.Action
		if what != "" {
			usedAliasParam = "action"
		}
	}
	if what == "" {
		validActions := getValidConfigureActions()
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validActions))}
		return "", usedAliasParam, &resp
	}
	return what, usedAliasParam, nil
}

// toolConfigure dispatches configure requests based on the 'what' parameter.
func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	params, err := parseConfigureModeParams(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	what, usedAliasParam, errResp := resolveConfigureAction(req, params)
	if errResp != nil {
		return *errResp
	}

	handler, ok := configureHandlers[what]
	if !ok {
		validActions := getValidConfigureActions()
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown configure action: "+what, "Use a valid action from the 'what' enum", withParam("what"), withHint("Valid values: "+validActions))}
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	}

	resp := handler(h, req, args)
	return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
}

func (h *ToolHandler) toolConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureStoreImpl(req, args)
}

func isStoreAction(action string) bool {
	switch action {
	case "save", "load", "list", "delete", "stats":
		return true
	default:
		return false
	}
}

func (h *ToolHandler) toolConfigureTelemetry(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureTelemetryImpl(req, args)
}

func (h *ToolHandler) toolConfigureSecurityMode(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureSecurityModeImpl(req, args)
}

// toolConfigureRestart handles restart requests that reach the daemon.
// Sends self-SIGTERM so the bridge auto-respawns a fresh daemon.
// This covers the case where the daemon is responsive but needs a clean restart.
func (h *ToolHandler) toolConfigureRestart(req JSONRPCRequest) JSONRPCResponse {
	return h.configureRestartImpl(req)
}

func (h *ToolHandler) toolLoadSessionContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureLoadSessionContextImpl(req, args)
}

// toolConfigureClear handles buffer-specific clearing with optional buffer parameter.
// Supported buffer values: "all", "network", "websocket", "actions", "logs"
func (h *ToolHandler) toolConfigureClear(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureClearImpl(req, args)
}

// clearBuffer performs the actual buffer clearing and returns what was cleared.
// Returns (cleared, true) on success, or (nil, false) for an unknown buffer name.
func (h *ToolHandler) clearBuffer(buffer string) (any, bool) {
	return h.clearConfiguredBuffer(buffer)
}

// toolConfigureStreamingWrapper repackages streaming_action -> action for toolConfigureStreaming.
func (h *ToolHandler) toolConfigureStreamingWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureStreamingWrapperImpl(req, args)
}

func (h *ToolHandler) toolConfigureTestBoundaryStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureTestBoundaryStartImpl(req, args)
}

func (h *ToolHandler) toolConfigureTestBoundaryEnd(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureTestBoundaryEndImpl(req, args)
}

// toolConfigureDescribeCapabilities returns machine-readable tool metadata derived from ToolsList().
// Supports filtering by tool name and mode to reduce payload size.
// When summary=true, returns only tool name → { description, dispatch_param, modes }.
func (h *ToolHandler) toolConfigureDescribeCapabilities(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureDescribeCapabilitiesImpl(req, args)
}
