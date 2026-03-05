// Purpose: Dispatches configure tool modes (health, clear, store, streaming, restart, doctor, security_mode, etc.) to sub-handlers.
// Why: Acts as the top-level router for all session/runtime configuration actions under the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
)

// configureAliasParams defines the deprecated alias parameters for the configure tool.
// ConflictFn gates conflict detection: only flag a what/action conflict when the action value
// is a known top-level configure mode (since "action" also serves as a sub-action field).
// FallbackFn is nil so any action value is accepted as a mode fallback when what is absent.
var configureAliasParams = []modeAlias{
	{JSONField: "action", ConflictFn: func(v string) bool {
		_, ok := configureHandlers[v]
		return ok
	}},
}

// toolConfigure dispatches configure requests based on the 'what' parameter.
func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	what, usedAliasParam, errResp := resolveToolMode(req, args, configureAliasParams, modeResolution{
		ToolName:   "configure",
		ValidModes: getValidConfigureActions(),
	})
	if errResp != nil {
		return *errResp
	}

	handler, ok := configureHandlers[what]
	if !ok {
		validActions := getValidConfigureActions()
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown configure action: "+what, "Use a valid action from the 'what' enum", withParam("what"), withHint("Valid values: "+validActions), describeCapabilitiesRecovery("configure"))}
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	}

	resp := handler(h, req, args)
	return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
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

// toolConfigureClear handles buffer-specific clearing with optional buffer parameter.
// Supported buffer values: "all", "network", "websocket", "actions", "logs"
func (h *ToolHandler) toolConfigureClear(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureClearImpl(req, args)
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
// When summary=true, returns only tool name -> { description, dispatch_param, modes }.
func (h *ToolHandler) toolConfigureDescribeCapabilities(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureDescribeCapabilitiesImpl(req, args)
}
