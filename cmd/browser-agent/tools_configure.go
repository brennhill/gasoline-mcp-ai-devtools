// Purpose: Dispatches configure tool modes (health, clear, store, streaming, restart, doctor, security_mode, etc.) to sub-handlers.
// Why: Acts as the top-level router for all session/runtime configuration actions under the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
)

// configureAliasParams defines the deprecated alias parameters for the configure tool.
// "mode" is included for parity with observe and analyze. Both "mode" and "action" have
// ConflictFn and FallbackFn gates because these fields also serve as sub-parameters
// (e.g. security_mode uses "mode" as a field, playback uses "action" as a sub-action).
// Conflicts and fallbacks are only triggered when the value is a known top-level configure mode.
var configureAliasParams = []modeAlias{
	{JSONField: "mode", ConflictFn: func(v string) bool {
		_, ok := configureHandlers[v]
		return ok
	}, FallbackFn: func(v string) bool {
		_, ok := configureHandlers[v]
		return ok
	}, DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	{JSONField: "action", ConflictFn: func(v string) bool {
		_, ok := configureHandlers[v]
		return ok
	}, DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// configureRegistry is the tool registry for configure dispatch.
var configureRegistry = toolRegistry{
	Handlers:  configureHandlers,
	AliasDefs: configureAliasParams,
	Resolution: modeResolution{
		ToolName:   "configure",
		ValidModes: "", // populated lazily
	},
}

// toolConfigure dispatches configure requests based on the 'what' parameter.
func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	reg := configureRegistry
	reg.Resolution.ValidModes = getValidConfigureActions()
	return h.dispatchTool(req, args, reg)
}

func isStoreAction(action string) bool {
	switch action {
	case "save", "load", "list", "delete", "stats":
		return true
	default:
		return false
	}
}
