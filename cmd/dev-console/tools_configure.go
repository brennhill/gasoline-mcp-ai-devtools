// Purpose: Owns tools_configure.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// tools_configure.go — MCP configure tool dispatcher and handlers.
// Handles session settings: store, load, noise_rule, clear, streaming, recordings, etc.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/audit"
	cfg "github.com/dev-console/dev-console/internal/tools/configure"
	"github.com/dev-console/dev-console/internal/util"
)

// ConfigureHandler is the function signature for configure action handlers.
type ConfigureHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

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

// ============================================
// Configure sub-handlers
// ============================================

func (h *ToolHandler) toolConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var compositeArgs struct {
		StoreAction string          `json:"store_action"`
		Namespace   string          `json:"namespace"`
		Key         string          `json:"key"`
		Data        json.RawMessage `json:"data"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &compositeArgs); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	action := compositeArgs.StoreAction
	if action == "" {
		action = "list"
	}

	// Ensure session store is initialized
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// Convert to SessionStoreArgs
	storeArgs := ai.SessionStoreArgs{
		Action:    action,
		Namespace: compositeArgs.Namespace,
		Key:       compositeArgs.Key,
		Data:      compositeArgs.Data,
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

func (h *ToolHandler) toolConfigureNoiseRule(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	rewrittenArgs, err := cfg.RewriteNoiseRuleArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	return h.toolConfigureNoise(req, rewrittenArgs)
}

// noiseRuleArgs holds the parsed parameters for noise configuration.
type noiseRuleArgs struct {
	Action string `json:"action"`
	Rules  []struct {
		Category       string `json:"category"`
		Classification string `json:"classification"`
		MatchSpec      struct {
			MessageRegex string `json:"message_regex"`
			SourceRegex  string `json:"source_regex"`
			URLRegex     string `json:"url_regex"`
			Method       string `json:"method"`
			StatusMin    int    `json:"status_min"`
			StatusMax    int    `json:"status_max"`
			Level        string `json:"level"`
		} `json:"match_spec"`
	} `json:"rules"`
	RuleID string `json:"rule_id"`
}

func (h *ToolHandler) toolConfigureNoise(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments noiseRuleArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if h.noiseConfig == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Noise configuration not initialized", "Internal error — do not retry")}
	}

	responseData, errResp := h.dispatchNoiseAction(req, arguments)
	if errResp != nil {
		return *errResp
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Noise configuration updated", responseData)}
}

func (h *ToolHandler) dispatchNoiseAction(req JSONRPCRequest, args noiseRuleArgs) (any, *JSONRPCResponse) {
	switch args.Action {
	case "add":
		return h.noiseActionAdd(req, args)
	case "remove":
		return h.noiseActionRemove(req, args)
	case "list":
		return h.noiseActionList(), nil
	case "reset":
		return h.noiseActionReset(), nil
	case "auto_detect":
		return h.noiseActionAutoDetect(), nil
	default:
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown noise action: "+args.Action, "Use a valid action: add, remove, list, reset, auto_detect", withParam("noise_action"))}
		return nil, &resp
	}
}

func (h *ToolHandler) noiseActionAdd(req JSONRPCRequest, args noiseRuleArgs) (any, *JSONRPCResponse) {
	rules := make([]ai.NoiseRule, len(args.Rules))
	for i, r := range args.Rules {
		rules[i] = ai.NoiseRule{
			Category:       r.Category,
			Classification: r.Classification,
			MatchSpec: ai.NoiseMatchSpec{
				MessageRegex: r.MatchSpec.MessageRegex,
				SourceRegex:  r.MatchSpec.SourceRegex,
				URLRegex:     r.MatchSpec.URLRegex,
				Method:       r.MatchSpec.Method,
				StatusMin:    r.MatchSpec.StatusMin,
				StatusMax:    r.MatchSpec.StatusMax,
				Level:        r.MatchSpec.Level,
			},
		}
	}
	if err := h.noiseConfig.AddRules(rules); err != nil {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, err.Error(), "Fix the rule pattern and try again")}
		return nil, &resp
	}
	return map[string]any{
		"status":      "ok",
		"rules_added": len(args.Rules),
		"total_rules": len(h.noiseConfig.ListRules()),
	}, nil
}

func (h *ToolHandler) noiseActionRemove(req JSONRPCRequest, args noiseRuleArgs) (any, *JSONRPCResponse) {
	if args.RuleID == "" {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'rule_id' is missing", "Add the 'rule_id' parameter", withParam("rule_id"))}
		return nil, &resp
	}
	if err := h.noiseConfig.RemoveRule(args.RuleID); err != nil {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, err.Error(), "Use a valid rule ID from list action")}
		return nil, &resp
	}
	return map[string]any{"status": "ok", "removed": args.RuleID}, nil
}

func (h *ToolHandler) noiseActionList() any {
	rules := h.noiseConfig.ListRules()
	stats := h.noiseConfig.GetStatistics()
	return map[string]any{
		"rules": rules,
		"statistics": map[string]any{
			"total_filtered": stats.TotalFiltered,
			"per_rule":       stats.PerRule,
			"last_signal_at": stats.LastSignalAt,
			"last_noise_at":  stats.LastNoiseAt,
		},
	}
}

func (h *ToolHandler) noiseActionReset() any {
	h.noiseConfig.Reset()
	return map[string]any{
		"status":      "ok",
		"total_rules": len(h.noiseConfig.ListRules()),
		"message":     "Reset to built-in rules only",
	}
}

func (h *ToolHandler) noiseActionAutoDetect() any {
	h.server.mu.RLock()
	consoleEntries := make([]ai.LogEntry, len(h.server.entries))
	for i, e := range h.server.entries {
		consoleEntries[i] = ai.LogEntry(e)
	}
	h.server.mu.RUnlock()

	networkBodies := h.capture.GetNetworkBodies()
	wsEvents := h.capture.GetAllWebSocketEvents()

	proposals := h.noiseConfig.AutoDetect(consoleEntries, networkBodies, wsEvents)
	return map[string]any{
		"proposals":       proposals,
		"total_rules":     len(h.noiseConfig.ListRules()),
		"proposals_count": len(proposals),
		"message":         "High-confidence proposals (>= 0.9) were auto-applied",
	}
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

// toolDiffSessionsWrapper repackages verif_session_action -> action for toolDiffSessions.
func (h *ToolHandler) toolDiffSessionsWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	rewritten, err := cfg.RewriteDiffSessionsArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	return h.toolDiffSessions(req, rewritten)
}

func (h *ToolHandler) toolDiffSessions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionManager == nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrNotInitialized, "Session manager not initialized", "Internal error — do not retry"),
		}
	}

	result, err := h.sessionManager.HandleTool(args)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidParam, err.Error(), "Fix request parameters and retry"),
		}
	}

	responseData := map[string]any{"status": "ok"}
	if m, ok := result.(map[string]any); ok {
		for k, v := range m {
			responseData[k] = v
		}
	} else {
		responseData["result"] = result
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session diff", responseData)}
}

func (h *ToolHandler) toolGetAuditLog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.auditTrail == nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrNotInitialized, "Audit trail not initialized", "Internal error — do not retry"),
		}
	}

	var params struct {
		Operation      string `json:"operation"`
		AuditSessionID string `json:"audit_session_id"`
		ToolName       string `json:"tool_name"`
		Limit          int    `json:"limit"`
		Since          string `json:"since"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	operation := strings.ToLower(strings.TrimSpace(params.Operation))
	if operation == "" {
		operation = "report"
	}
	if operation != "analyze" && operation != "report" && operation != "clear" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidParam, "Invalid audit_log operation: "+params.Operation, "Use operation: analyze, report, or clear", withParam("operation")),
		}
	}
	if operation == "clear" {
		cleared := h.auditTrail.Clear()
		h.auditMu.Lock()
		h.auditSessionMap = make(map[string]string)
		h.auditMu.Unlock()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Audit log cleared", map[string]any{
			"status":    "ok",
			"operation": "clear",
			"cleared":   cleared,
		})}
	}

	filter := audit.AuditFilter{
		AuditSessionID: params.AuditSessionID,
		ToolName:       params.ToolName,
		Limit:          params.Limit,
	}
	if params.Since != "" {
		since, err := time.Parse(time.RFC3339, params.Since)
		if err != nil {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpStructuredError(ErrInvalidParam, "Invalid 'since' timestamp: "+err.Error(), "Use RFC3339 format, for example 2026-02-17T15:04:05Z", withParam("since")),
			}
		}
		filter.Since = &since
	}

	entries := h.auditTrail.Query(filter)
	if operation == "analyze" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Audit log analysis", map[string]any{
			"status":    "ok",
			"operation": "analyze",
			"summary":   cfg.SummarizeAuditEntries(entries),
		})}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Audit log entries", map[string]any{
		"status":    "ok",
		"operation": "report",
		"entries":   entries,
		"count":     len(entries),
	})}
}

// toolConfigureStreamingWrapper repackages streaming_action -> action for toolConfigureStreaming.
func (h *ToolHandler) toolConfigureStreamingWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	rewritten, err := cfg.RewriteStreamingArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	return h.toolConfigureStreaming(req, rewritten)
}

// ============================================
// Test Boundary Tool Implementations
// ============================================

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
func (h *ToolHandler) handleDescribeCapabilities(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	tools := h.ToolsList()
	toolsMap := cfg.BuildCapabilitiesMap(tools)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Capabilities", map[string]any{
		"version":          version,
		"protocol_version": "2024-11-05",
		"tools":            toolsMap,
		"deprecated":       []string{},
	})}
}
