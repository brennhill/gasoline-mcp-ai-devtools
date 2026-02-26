// Purpose: Implements configure tool handlers for policy, profiles, and session controls.
// Why: Keeps runtime/session configuration changes explicit and auditable from a single tool surface.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/capture"
	cfg "github.com/dev-console/dev-console/internal/tools/configure"
	"github.com/dev-console/dev-console/internal/util"
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
	if what == "" {
		what = params.Action
	}

	if what == "" {
		validActions := getValidConfigureActions()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validActions))}
	}

	handler, ok := configureHandlers[what]
	if !ok {
		validActions := getValidConfigureActions()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown configure action: "+what, "Use a valid action from the 'what' enum", withParam("what"), withHint("Valid values: "+validActions))}
	}

	return handler(h, req, args)
}

func (h *ToolHandler) toolConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var compositeArgs struct {
		StoreAction string          `json:"store_action"`
		Action      string          `json:"action"`
		Namespace   string          `json:"namespace"`
		Key         string          `json:"key"`
		Data        json.RawMessage `json:"data"`
		Value       json.RawMessage `json:"value"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &compositeArgs); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	action := compositeArgs.StoreAction
	if action == "" && isStoreAction(compositeArgs.Action) {
		action = compositeArgs.Action
	}
	if action == "" {
		action = "list"
	}

	namespace := compositeArgs.Namespace
	if namespace == "" {
		namespace = defaultStoreNamespace
	}

	data := compositeArgs.Data
	if len(data) == 0 && len(compositeArgs.Value) > 0 {
		data = compositeArgs.Value
	}

	// Ensure session store is initialized
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// Convert to SessionStoreArgs
	storeArgs := ai.SessionStoreArgs{
		Action:    action,
		Namespace: namespace,
		Key:       compositeArgs.Key,
		Data:      data,
	}

	result, err := h.sessionStoreImpl.HandleSessionStore(storeArgs)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, err.Error(), "Fix the request parameters and try again")}
	}

	// Parse result back to map for response
	var responseData map[string]any
	if err := json.Unmarshal(result, &responseData); err != nil {
		responseData = map[string]any{"raw": string(result)}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Store operation complete", responseData)}
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
	if h.server == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Server not initialized", "Internal error — do not retry")}
	}

	var params struct {
		TelemetryMode string `json:"telemetry_mode"`
	}
	lenientUnmarshal(args, &params)

	if params.TelemetryMode == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Telemetry mode", map[string]any{
			"status":         "ok",
			"telemetry_mode": h.server.getTelemetryMode(),
		})}
	}

	mode, ok := normalizeTelemetryMode(params.TelemetryMode)
	if !ok {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Invalid telemetry_mode: "+params.TelemetryMode,
			"Use telemetry_mode: off, auto, or full",
			withParam("telemetry_mode"),
		)}
	}

	h.server.setTelemetryMode(mode)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Telemetry mode updated", map[string]any{
		"status":         "ok",
		"telemetry_mode": mode,
	})}
}

func (h *ToolHandler) toolConfigureSecurityMode(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.capture == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrNotInitialized,
			"Capture subsystem not initialized",
			"Internal error — do not retry",
		)}
	}

	var params struct {
		Mode    string `json:"mode"`
		Confirm bool   `json:"confirm"`
	}
	lenientUnmarshal(args, &params)

	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if mode == "" {
		current, productionParity, rewrites := h.capture.GetSecurityMode()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security mode", map[string]any{
			"status":                    "ok",
			"security_mode":             current,
			"production_parity":         productionParity,
			"insecure_rewrites_applied": rewrites,
			"requires_confirmation_for_insecure_mode": true,
		})}
	}

	switch mode {
	case capture.SecurityModeNormal:
		h.capture.SetSecurityMode(capture.SecurityModeNormal, nil)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             capture.SecurityModeNormal,
			"production_parity":         true,
			"insecure_rewrites_applied": []string{},
		})}
	case capture.SecurityModeInsecureProxy:
		if !params.Confirm {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"security_mode=insecure_proxy requires explicit confirmation",
				"Retry with confirm=true to acknowledge altered-environment debugging mode",
				withParam("confirm"),
			)}
		}
		rewrites := []string{"csp_headers"}
		h.capture.SetSecurityMode(capture.SecurityModeInsecureProxy, rewrites)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             capture.SecurityModeInsecureProxy,
			"production_parity":         false,
			"insecure_rewrites_applied": rewrites,
			"warning":                   "Altered environment active. Findings are not production-parity evidence.",
		})}
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Invalid security mode: "+params.Mode,
			"Use mode: normal or insecure_proxy",
			withParam("mode"),
		)}
	}
}

// toolConfigureRestart handles restart requests that reach the daemon.
// Sends self-SIGTERM so the bridge auto-respawns a fresh daemon.
// This covers the case where the daemon is responsive but needs a clean restart.
func (h *ToolHandler) toolConfigureRestart(req JSONRPCRequest) JSONRPCResponse {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Daemon restarting", map[string]any{
		"status":    "ok",
		"restarted": true,
		"message":   "Daemon shutting down — bridge will respawn automatically",
	})}

	// Send SIGTERM to self after a brief delay so the response is sent first.
	util.SafeGo(func() {
		time.Sleep(100 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	})

	return resp
}

func (h *ToolHandler) toolLoadSessionContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// If session store is initialized, use it
	if h.sessionStoreImpl != nil {
		ctx := h.sessionStoreImpl.LoadSessionContext()
		responseData := map[string]any{
			"status":        "ok",
			"project_id":    ctx.ProjectID,
			"session_count": ctx.SessionCount,
			"baselines":     ctx.Baselines,
			"error_history": ctx.ErrorHistory,
		}
		if ctx.NoiseConfig != nil {
			responseData["noise_config"] = ctx.NoiseConfig
		}
		if ctx.APISchema != nil {
			responseData["api_schema"] = ctx.APISchema
		}
		if ctx.Performance != nil {
			responseData["performance"] = ctx.Performance
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session context loaded", responseData)}
	}

	// Session store not initialized — return error, matching store behavior
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
}

// toolConfigureClear handles buffer-specific clearing with optional buffer parameter.
// Supported buffer values: "all", "network", "websocket", "actions", "logs"
func (h *ToolHandler) toolConfigureClear(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Buffer string `json:"buffer"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	buffer := params.Buffer
	if buffer == "" {
		buffer = "all"
	}

	cleared, ok := h.clearBuffer(buffer)
	if !ok {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Unknown buffer: "+buffer, "Use a valid buffer value", withParam("buffer"), withHint("all, network, websocket, actions, logs"))}
	}

	responseData := map[string]any{"status": "ok", "buffer": buffer, "cleared": cleared}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Buffer cleared", responseData)}
}

// clearBuffer performs the actual buffer clearing and returns what was cleared.
// Returns (cleared, true) on success, or (nil, false) for an unknown buffer name.
func (h *ToolHandler) clearBuffer(buffer string) (any, bool) {
	switch buffer {
	case "all":
		h.capture.ClearAll()
		h.server.clearEntries()
		return map[string]any{"buffers": "all", "extension_logs_cleared": h.capture.ClearExtensionLogs()}, true
	case "network":
		counts := h.capture.ClearNetworkBuffers()
		return map[string]int{"waterfall": counts.NetworkWaterfall, "bodies": counts.NetworkBodies}, true
	case "websocket":
		counts := h.capture.ClearWebSocketBuffers()
		return map[string]int{"events": counts.WebSocketEvents, "connections": counts.WebSocketStatus}, true
	case "actions":
		counts := h.capture.ClearActionBuffer()
		return map[string]int{"actions": counts.Actions}, true
	case "logs":
		logCount := h.server.getEntryCount()
		h.server.clearEntries()
		return map[string]int{"logs": logCount}, true
	default:
		return nil, false
	}
}

// toolConfigureStreamingWrapper repackages streaming_action -> action for toolConfigureStreaming.
func (h *ToolHandler) toolConfigureStreamingWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	rewritten, err := cfg.RewriteStreamingArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	return h.toolConfigureStreaming(req, rewritten)
}

func (h *ToolHandler) toolConfigureTestBoundaryStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, errResp := cfg.ParseTestBoundaryStart(req.ID, args)
	if errResp != nil {
		return *errResp
	}

	// Track the active boundary
	h.activeBoundariesMu.Lock()
	if h.activeBoundaries == nil {
		h.activeBoundaries = make(map[string]time.Time)
	}
	h.activeBoundaries[result.TestID] = time.Now()
	h.activeBoundariesMu.Unlock()

	return cfg.BuildTestBoundaryStartResponse(req.ID, result)
}

func (h *ToolHandler) toolConfigureTestBoundaryEnd(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, errResp := cfg.ParseTestBoundaryEnd(req.ID, args)
	if errResp != nil {
		return *errResp
	}

	// Check if this boundary was actually started
	h.activeBoundariesMu.Lock()
	_, wasActive := h.activeBoundaries[result.TestID]
	if wasActive {
		delete(h.activeBoundaries, result.TestID)
	}
	h.activeBoundariesMu.Unlock()

	if !wasActive {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"No active test boundary for test_id '"+result.TestID+"'",
			"Call configure({what: 'test_boundary_start', test_id: '"+result.TestID+"'}) first",
			withParam("test_id"),
		)}
	}

	return cfg.BuildTestBoundaryEndResponse(req.ID, result, wasActive)
}

// handleDescribeCapabilities returns machine-readable tool metadata derived from ToolsList().
// Supports filtering by tool name and mode to reduce payload size.
// When summary=true, returns only tool name → { description, dispatch_param, modes }.
func (h *ToolHandler) handleDescribeCapabilities(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Summary bool   `json:"summary"`
		Tool    string `json:"tool"`
		Mode    string `json:"mode"`
	}
	lenientUnmarshal(args, &params)

	tools := h.ToolsList()

	// mode without tool is an error
	if params.Mode != "" && params.Tool == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"'mode' requires 'tool' to be set",
			"Add the 'tool' parameter to filter by mode",
			withParam("tool"),
		)}
	}

	// Filter by tool + mode
	if params.Tool != "" {
		toolCap, ok := cfg.BuildCapabilitiesForTool(tools, params.Tool)
		if !ok {
			validNames := make([]string, 0, len(tools))
			for _, t := range tools {
				validNames = append(validNames, t.Name)
			}
			sort.Strings(validNames)
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"Unknown tool: "+params.Tool,
				"Use a valid tool name",
				withParam("tool"),
				withHint("Valid tools: "+strings.Join(validNames, ", ")),
			)}
		}

		if params.Mode != "" {
			modeCap, ok := cfg.FilterToolByMode(toolCap, params.Tool, params.Mode)
			if !ok {
				modes, _ := toolCap["modes"].([]string)
				if modes == nil {
					if modesAny, ok := toolCap["modes"].([]any); ok {
						for _, m := range modesAny {
							if s, ok := m.(string); ok {
								modes = append(modes, s)
							}
						}
					}
				}
				return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
					ErrInvalidParam,
					"Unknown mode '"+params.Mode+"' for tool '"+params.Tool+"'",
					"Use a valid mode for this tool",
					withParam("mode"),
					withHint("Valid modes: "+strings.Join(modes, ", ")),
				)}
			}
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Capabilities", modeCap)}
		}

		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Capabilities", map[string]any{
			"version":          version,
			"protocol_version": "2024-11-05",
			"tools":            map[string]any{params.Tool: toolCap},
		})}
	}

	// Full or summary response (no filters)
	var toolsMap map[string]any
	if params.Summary {
		toolsMap = cfg.BuildCapabilitiesSummary(tools)
	} else {
		toolsMap = cfg.BuildCapabilitiesMap(tools)
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Capabilities", map[string]any{
		"version":          version,
		"protocol_version": "2024-11-05",
		"tools":            toolsMap,
		"deprecated":       []string{},
	})}
}
