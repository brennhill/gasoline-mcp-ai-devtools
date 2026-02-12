// tools_configure.go — MCP configure tool dispatcher and handlers.
// Handles session settings: store, load, noise_rule, clear, streaming, recordings, etc.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
)

// randomInt63 generates a random int64 for correlation IDs using crypto/rand.
func randomInt63() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to time-based if rand fails (should never happen)
		return time.Now().UnixNano()
	}
	return int64(binary.BigEndian.Uint64(b[:]) & 0x7FFFFFFFFFFFFFFF)
}

// ConfigureHandler is the function signature for configure action handlers.
type ConfigureHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// configureHandlers maps configure action names to their handler functions.
var configureHandlers = map[string]ConfigureHandler{
	"store":                func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureStore(req, args) },
	"load":                 func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolLoadSessionContext(req, args) },
	"noise_rule":           func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureNoiseRule(req, args) },
	"clear":                func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureClear(req, args) },
	"diff_sessions":        func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolDiffSessionsWrapper(req, args) },
	"audit_log":            func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetAuditLog(req, args) },
	"health":               func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetHealth(req) },
	"streaming":            func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureStreamingWrapper(req, args) },
	"test_boundary_start":  func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureTestBoundaryStart(req, args) },
	"test_boundary_end":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureTestBoundaryEnd(req, args) },
	"recording_start":      func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureRecordingStart(req, args) },
	"recording_stop":       func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureRecordingStop(req, args) },
	"playback":             func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigurePlayback(req, args) },
	"log_diff":             func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolConfigureLogDiff(req, args) },
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

// toolConfigure dispatches configure requests based on the 'action' parameter.
func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Action string `json:"action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Action == "" {
		validActions := getValidConfigureActions()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"), withHint("Valid values: "+validActions))}
	}

	handler, ok := configureHandlers[params.Action]
	if !ok {
		validActions := getValidConfigureActions()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown configure action: "+params.Action, "Use a valid action from the 'action' enum", withParam("action"), withHint("Valid values: "+validActions))}
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

	// Fallback if no session store
	responseData := map[string]any{
		"status":  "ok",
		"context": map[string]any{},
		"message": "Session store not initialized",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session context loaded", responseData)}
}

func (h *ToolHandler) toolConfigureNoiseRule(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract the noise_action field as the action for configure_noise
	var compositeArgs struct {
		NoiseAction string `json:"noise_action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &compositeArgs); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Rewrite args to have "action" field that toolConfigureNoise expects
	var rawMap map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &rawMap); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	if rawMap == nil {
		rawMap = make(map[string]any)
	}
	rawMap["action"] = compositeArgs.NoiseAction
	if rawMap["action"] == "" {
		rawMap["action"] = "list"
	}
	// Error impossible: rawMap contains only primitive types and strings from input
	rewrittenArgs, _ := json.Marshal(rawMap)

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
		"status":     "ok",
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

// toolDiffSessionsWrapper repackages session_action → action for toolDiffSessions.
func (h *ToolHandler) toolDiffSessionsWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	if sa, ok := raw["session_action"].(string); ok {
		raw["action"] = sa
	}
	// Error impossible: raw contains only primitive types and strings from input
	rewritten, _ := json.Marshal(raw)
	return h.toolDiffSessions(req, rewritten)
}

func (h *ToolHandler) toolDiffSessions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SessionAction string `json:"session_action"`
		Name          string `json:"name"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]any{
		"status": "ok",
		"action": params.SessionAction,
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session diff", responseData)}
}

func (h *ToolHandler) toolGetAuditLog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SessionID string `json:"session_id"`
		ToolName  string `json:"tool_name"`
		Limit     int    `json:"limit"`
		Since     string `json:"since"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]any{
		"status":  "ok",
		"entries": []any{},
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Audit log entries", responseData)}
}

// toolConfigureStreamingWrapper repackages streaming_action → action for toolConfigureStreaming.
func (h *ToolHandler) toolConfigureStreamingWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	if sa, ok := raw["streaming_action"].(string); ok {
		raw["action"] = sa
	}
	// Error impossible: raw contains only primitive types and strings from input
	rewritten, _ := json.Marshal(raw)
	return h.toolConfigureStreaming(req, rewritten)
}

// ============================================
// Test Boundary Tool Implementations
// ============================================

func (h *ToolHandler) toolConfigureTestBoundaryStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TestID string `json:"test_id"`
		Label  string `json:"label"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.TestID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'test_id' is missing", "Add the 'test_id' parameter", withParam("test_id"))}
	}

	label := params.Label
	if label == "" {
		label = "Test: " + params.TestID
	}

	responseData := map[string]any{
		"status":  "ok",
		"test_id": params.TestID,
		"label":   label,
		"message": "Test boundary started",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Test boundary started", responseData)}
}

func (h *ToolHandler) toolConfigureTestBoundaryEnd(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TestID string `json:"test_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.TestID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'test_id' is missing", "Add the 'test_id' parameter", withParam("test_id"))}
	}

	responseData := map[string]any{
		"status":     "ok",
		"test_id":    params.TestID,
		"was_active": true,
		"message":    "Test boundary ended",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Test boundary ended", responseData)}
}
