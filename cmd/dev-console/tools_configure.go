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
