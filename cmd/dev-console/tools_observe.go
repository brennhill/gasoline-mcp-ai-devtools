// Purpose: Dispatches observe tool modes and coordinates alias resolution, validation, and response augmentation.
// Why: Keeps observe entrypoint behavior explicit while mode registry and response helpers live in focused companion files.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
)

// observeAliasParams references the shared default mode/action aliases.
var observeAliasParams = defaultModeActionAliases

// observeRegistry is the tool registry for observe dispatch.
var observeRegistry = toolRegistry{
	Handlers:  observeHandlers,
	AliasDefs: observeAliasParams,
	Resolution: modeResolution{
		ToolName:     "observe",
		ValidModes:   "", // populated lazily
		ValueAliases: observeValueAliases,
	},
	PreDispatch: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, _ string) (json.RawMessage, *JSONRPCResponse) {
		return h.maybeInjectSummary(args), nil
	},
	PostDispatch: func(h *ToolHandler, req JSONRPCRequest, resp JSONRPCResponse, what string) JSONRPCResponse {
		// Warn when extension is disconnected (except for server-side modes that don't need it)
		if !h.capture.IsExtensionConnected() && !serverSideObserveModes[what] {
			resp = h.prependDisconnectWarning(resp)
		}
		// Piggyback alerts: append as second content block if any pending
		if alerts := h.drainAlerts(); len(alerts) > 0 {
			resp = h.appendAlertsToResponse(resp, alerts)
		}
		return resp
	},
}

// toolObserve dispatches observe requests based on the 'what' parameter.
func (h *ToolHandler) toolObserve(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	reg := observeRegistry
	reg.Resolution.ValidModes = getValidObserveModes()
	return h.dispatchTool(req, args, reg)
}
