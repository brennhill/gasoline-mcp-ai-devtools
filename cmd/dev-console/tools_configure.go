// Purpose: Dispatches configure tool modes (health, clear, store, streaming, restart, doctor, security_mode, etc.) to sub-handlers.
// Why: Acts as the top-level router for all session/runtime configuration actions under the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
	"sort"
	"strings"
)

// ConfigureHandler is the function signature for configure action handlers.
type ConfigureHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

const defaultStoreNamespace = "session"

// configureHandlers maps configure action names to their handler functions.
var configureHandlers = map[string]ConfigureHandler{
	"store": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureStore(req, args)
	},
	"load": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolLoadSessionContext(req, args)
	},
	"noise_rule": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureNoiseRule(req, args)
	},
	"clear": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureClear(req, args)
	},
	"diff_sessions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolDiffSessionsWrapper(req, args)
	},
	"audit_log": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetAuditLog(req, args)
	},
	"health": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetHealth(req)
	},
	"streaming": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureStreamingWrapper(req, args)
	},
	"test_boundary_start": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureTestBoundaryStart(req, args)
	},
	"test_boundary_end": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureTestBoundaryEnd(req, args)
	},
	"recording_start": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureRecordingStart(req, args)
	},
	"recording_stop": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureRecordingStop(req, args)
	},
	"playback": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigurePlayback(req, args)
	},
	"log_diff": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureLogDiff(req, args)
	},
	"telemetry": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureTelemetry(req, args)
	},
	"describe_capabilities": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.handleDescribeCapabilities(req, args)
	},
	"restart": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureRestart(req)
	},
	"doctor": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolDoctor(req)
	},
	"tutorial": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureTutorial(req, args)
	},
	"examples": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureTutorial(req, args)
	},
	"save_sequence": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureSaveSequence(req, args)
	},
	"get_sequence": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureGetSequence(req, args)
	},
	"list_sequences": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureListSequences(req, args)
	},
	"delete_sequence": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureDeleteSequence(req, args)
	},
	"replay_sequence": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureReplaySequence(req, args)
	},
	"security_mode": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureSecurityMode(req, args)
	},
	"network_recording": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureNetworkRecording(req, args)
	},
	"action_jitter": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureActionJitter(req, args)
	},
}

// getValidConfigureActions returns a sorted, comma-separated list of valid configure actions.
func getValidConfigureActions() string {
	actions := make([]string, 0, len(configureHandlers))
	for action := range configureHandlers {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return strings.Join(actions, ", ")
}

// toolConfigure dispatches configure requests based on the 'what' parameter.
func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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
		if _, isTopLevelConfigureAction := configureHandlers[params.Action]; isTopLevelConfigureAction {
			return whatAliasConflictResponse(req, "action", what, params.Action, getValidConfigureActions())
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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validActions))}
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

// handleDescribeCapabilities returns machine-readable tool metadata derived from ToolsList().
// Supports filtering by tool name and mode to reduce payload size.
// When summary=true, returns only tool name → { description, dispatch_param, modes }.
func (h *ToolHandler) handleDescribeCapabilities(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.configureDescribeCapabilitiesImpl(req, args)
}
