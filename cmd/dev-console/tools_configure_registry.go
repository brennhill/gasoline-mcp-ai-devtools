// Purpose: Central registry for configure actions and valid-action metadata.
// Why: Keeps dispatch table updates isolated from argument parsing and wrapper logic.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
)

// ConfigureHandler is the function signature for configure action handlers.
type ConfigureHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

const defaultStoreNamespace = "session"

// configureHandlers maps configure action names to their handler functions.
var configureHandlers = map[string]ConfigureHandler{
	"store": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().toolConfigureStore(req, args)
	},
	"load": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().toolLoadSessionContext(req, args)
	},
	"noise_rule": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureNoiseRule(req, args)
	},
	"clear": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureClearImpl(req, args)
	},
	"diff_sessions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().toolDiffSessionsWrapper(req, args)
	},
	"audit_log": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetAuditLog(req, args)
	},
	"health": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetHealth(req)
	},
	"streaming": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureStreamingWrapperImpl(req, args)
	},
	"test_boundary_start": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureTestBoundaryStartImpl(req, args)
	},
	"test_boundary_end": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureTestBoundaryEndImpl(req, args)
	},
	"event_recording_start": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureEventRecordingStart(req, args)
	},
	"event_recording_stop": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureEventRecordingStop(req, args)
	},
	"playback": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigurePlayback(req, args)
	},
	"log_diff": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureLogDiff(req, args)
	},
	"telemetry": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureTelemetryImpl(req, args)
	},
	"describe_capabilities": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureDescribeCapabilitiesImpl(req, args)
	},
	"restart": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureRestartImpl(req)
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
		return h.configureSecurityModeImpl(req, args)
	},
	"network_recording": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureNetworkRecording(req, args)
	},
	"action_jitter": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureActionJitter(req, args)
	},
	"report_issue": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolConfigureReportIssue(req, args)
	},
}

// getValidConfigureActions returns a sorted, comma-separated list of valid configure actions.
func getValidConfigureActions() string { return sortedMapKeys(configureHandlers) }
