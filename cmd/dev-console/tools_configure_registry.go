// Purpose: Central registry for configure actions and valid-action metadata.
// Why: Keeps dispatch table updates isolated from argument parsing and wrapper logic.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
)

const defaultStoreNamespace = "session"

// configureHandlers maps configure action names to their handler functions.
var configureHandlers = map[string]ModeHandler{
	// Sub-handler delegates (require closures — configureSession() accessor)
	"store": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().toolConfigureStore(req, args)
	},
	"load": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().toolLoadSessionContext(req, args)
	},
	"diff_sessions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().toolDiffSessionsWrapper(req, args)
	},
	// Args-less handlers (require closures — different receiver signature)
	"health": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolGetHealth(req)
	},
	"restart": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolConfigureRestart(req)
	},
	"doctor": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolDoctor(req)
	},
	// Direct method delegates
	"noise_rule":            method((*ToolHandler).toolConfigureNoiseRule),
	"clear":                 method((*ToolHandler).toolConfigureClear),
	"audit_log":             method((*ToolHandler).toolGetAuditLog),
	"streaming":             method((*ToolHandler).toolConfigureStreaming),
	"test_boundary_start":   method((*ToolHandler).toolConfigureTestBoundaryStart),
	"test_boundary_end":     method((*ToolHandler).toolConfigureTestBoundaryEnd),
	"event_recording_start": method((*ToolHandler).toolConfigureEventRecordingStart),
	"event_recording_stop":  method((*ToolHandler).toolConfigureEventRecordingStop),
	"playback":              method((*ToolHandler).toolConfigurePlayback),
	"log_diff":              method((*ToolHandler).toolConfigureLogDiff),
	"telemetry":             method((*ToolHandler).toolConfigureTelemetry),
	"describe_capabilities": method((*ToolHandler).toolConfigureDescribeCapabilities),
	"tutorial":              method((*ToolHandler).toolConfigureTutorial),
	"examples":              method((*ToolHandler).toolConfigureTutorial),
	"save_sequence":         method((*ToolHandler).toolConfigureSaveSequence),
	"get_sequence":          method((*ToolHandler).toolConfigureGetSequence),
	"list_sequences":        method((*ToolHandler).toolConfigureListSequences),
	"delete_sequence":       method((*ToolHandler).toolConfigureDeleteSequence),
	"replay_sequence":       method((*ToolHandler).toolConfigureReplaySequence),
	"security_mode":         method((*ToolHandler).toolConfigureSecurityMode),
	"network_recording":     method((*ToolHandler).toolConfigureNetworkRecording),
	"action_jitter":         method((*ToolHandler).toolConfigureActionJitter),
	"report_issue":          method((*ToolHandler).toolConfigureReportIssue),
}

// getValidConfigureActions returns a sorted, comma-separated list of valid configure actions.
func getValidConfigureActions() string { return sortedMapKeys(configureHandlers) }
