// Purpose: Dispatches observe tool modes and coordinates alias resolution, validation, and response augmentation.
// Why: Keeps observe entrypoint behavior explicit while mode registry and response helpers live in focused companion files.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
)

// observeAliasParams defines the deprecated alias parameters for the observe tool.
var observeAliasParams = []modeAlias{
	{JSONField: "mode"},
	{JSONField: "action"},
}

// toolObserve dispatches observe requests based on the 'what' parameter.
func (h *ToolHandler) toolObserve(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	what, usedAliasParam, errResp := resolveToolMode(req, args, observeAliasParams, modeResolution{
		ToolName:   "observe",
		ValidModes: getValidObserveModes(),
		Aliases:    observeAliases,
	})
	if errResp != nil {
		return *errResp
	}

	handler, ok := observeHandlers[what]
	if !ok {
		validModes := getValidObserveModes()
		resp := fail(req, ErrUnknownMode, "Unknown observe mode: "+what, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes), describeCapabilitiesRecovery("observe"))
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
