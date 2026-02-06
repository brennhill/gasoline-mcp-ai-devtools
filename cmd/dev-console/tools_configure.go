// tools_configure.go — MCP configure tool dispatcher and handlers.
// Handles all configure actions: store, load, noise_rule, dismiss, clear, etc.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/queries"
)

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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"), withHint("Valid values: store, load, noise_rule, dismiss, clear, capture, record_event, query_dom, diff_sessions, validate_api, audit_log, health, streaming"))}
	}

	var resp JSONRPCResponse
	switch params.Action {
	case "store":
		resp = h.toolConfigureStore(req, args)
	case "load":
		resp = h.toolLoadSessionContext(req, args)
	case "noise_rule":
		resp = h.toolConfigureNoiseRule(req, args)
	case "clear":
		resp = h.toolConfigureClear(req, args)
	case "query_dom":
		resp = h.toolQueryDOM(req, args)
	case "diff_sessions":
		resp = h.toolDiffSessionsWrapper(req, args)
	case "validate_api":
		resp = h.toolValidateAPI(req, args)
	case "audit_log":
		resp = h.toolGetAuditLog(req, args)
	case "health":
		resp = h.toolGetHealth(req)
	case "streaming":
		resp = h.toolConfigureStreamingWrapper(req, args)
	case "test_boundary_start":
		resp = h.toolConfigureTestBoundaryStart(req, args)
	case "test_boundary_end":
		resp = h.toolConfigureTestBoundaryEnd(req, args)
	case "recording_start":
		resp = h.toolConfigureRecordingStart(req, args)
	case "recording_stop":
		resp = h.toolConfigureRecordingStop(req, args)
	case "playback":
		resp = h.toolConfigurePlayback(req, args)
	case "log_diff":
		resp = h.toolConfigureLogDiff(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown configure action: "+params.Action, "Use a valid action from the 'action' enum", withParam("action"))}
	}
	return resp
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
	rawMap["action"] = compositeArgs.NoiseAction
	if rawMap["action"] == "" {
		rawMap["action"] = "list"
	}
	// Error impossible: rawMap contains only primitive types and strings from input
	rewrittenArgs, _ := json.Marshal(rawMap)

	return h.toolConfigureNoise(req, rewrittenArgs)
}

func (h *ToolHandler) toolConfigureNoise(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
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
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Ensure noise config is initialized
	if h.noiseConfig == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Noise configuration not initialized", "Internal error — do not retry")}
	}

	var responseData any

	switch arguments.Action {
	case "add":
		// Convert arguments to ai.NoiseRule slice
		rules := make([]ai.NoiseRule, len(arguments.Rules))
		for i, r := range arguments.Rules {
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
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, err.Error(), "Fix the rule pattern and try again")}
		}
		allRules := h.noiseConfig.ListRules()
		responseData = map[string]any{
			"status":     "ok",
			"rulesAdded": len(arguments.Rules),
			"totalRules": len(allRules),
		}

	case "remove":
		if arguments.RuleID == "" {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'rule_id' is missing", "Add the 'rule_id' parameter", withParam("rule_id"))}
		}
		if err := h.noiseConfig.RemoveRule(arguments.RuleID); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, err.Error(), "Use a valid rule ID from list action")}
		}
		responseData = map[string]any{
			"status":  "ok",
			"removed": arguments.RuleID,
		}

	case "list":
		rules := h.noiseConfig.ListRules()
		stats := h.noiseConfig.GetStatistics()
		responseData = map[string]any{
			"rules": rules,
			"statistics": map[string]any{
				"total_filtered":  stats.TotalFiltered,
				"per_rule":        stats.PerRule,
				"last_signal_at":  stats.LastSignalAt,
				"last_noise_at":   stats.LastNoiseAt,
			},
		}

	case "reset":
		h.noiseConfig.Reset()
		allRules := h.noiseConfig.ListRules()
		responseData = map[string]any{
			"status":     "ok",
			"totalRules": len(allRules),
			"message":    "Reset to built-in rules only",
		}

	case "auto_detect":
		// Get current buffer contents for analysis
		h.server.mu.RLock()
		consoleEntries := make([]ai.LogEntry, len(h.server.entries))
		for i, e := range h.server.entries {
			consoleEntries[i] = ai.LogEntry(e)
		}
		h.server.mu.RUnlock()

		networkBodies := h.capture.GetNetworkBodies()
		wsEvents := h.capture.GetAllWebSocketEvents()

		proposals := h.noiseConfig.AutoDetect(consoleEntries, networkBodies, wsEvents)
		allRules := h.noiseConfig.ListRules()

		responseData = map[string]any{
			"proposals":       proposals,
			"totalRules":      len(allRules),
			"proposals_count": len(proposals),
			"message":         "High-confidence proposals (>= 0.9) were auto-applied",
		}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown noise action: "+arguments.Action, "Use a valid action: add, remove, list, reset, auto_detect", withParam("noise_action"))}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Noise configuration updated", responseData)}
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

	// Default to "all" if no buffer specified
	buffer := params.Buffer
	if buffer == "" {
		buffer = "all"
	}

	responseData := map[string]any{
		"status": "ok",
		"buffer": buffer,
	}

	switch buffer {
	case "all":
		// Clear capture buffers
		h.capture.ClearAll()
		// Clear console logs
		h.server.clearEntries()
		// Clear extension logs
		extLogCount := h.capture.ClearExtensionLogs()
		responseData["cleared"] = "all buffers"
		responseData["extension_logs_cleared"] = extLogCount

	case "network":
		counts := h.capture.ClearNetworkBuffers()
		responseData["cleared"] = map[string]int{
			"waterfall": counts.NetworkWaterfall,
			"bodies":    counts.NetworkBodies,
		}

	case "websocket":
		counts := h.capture.ClearWebSocketBuffers()
		responseData["cleared"] = map[string]int{
			"events":      counts.WebSocketEvents,
			"connections": counts.WebSocketStatus,
		}

	case "actions":
		counts := h.capture.ClearActionBuffer()
		responseData["cleared"] = map[string]int{
			"actions": counts.Actions,
		}

	case "logs":
		// Get count before clearing
		logCount := h.server.getEntryCount()
		h.server.clearEntries()
		responseData["cleared"] = map[string]int{
			"logs": logCount,
		}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Unknown buffer: "+buffer, "Use a valid buffer value", withParam("buffer"), withHint("all, network, websocket, actions, logs"))}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Buffer cleared", responseData)}
}

func (h *ToolHandler) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Selector string `json:"selector"`
		TabID    int    `json:"tab_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Selector == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'selector' is missing", "Add the 'selector' parameter with a CSS selector and call again", withParam("selector"))}
	}

	// Generate correlation ID for tracking
	correlationID := fmt.Sprintf("dom_%d_%d", time.Now().UnixNano(), rand.Int63())

	// Create pending query for DOM query
	query := queries.PendingQuery{
		Type:          "dom",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("DOM query queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"selector":       params.Selector,
		"hint":           "Use observe({what:'command_result', correlation_id:'" + correlationID + "'}) to check the result",
	})}
}

// toolDiffSessionsWrapper repackages session_action → action for toolDiffSessions.
func (h *ToolHandler) toolDiffSessionsWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
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

func (h *ToolHandler) toolValidateAPI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation       string   `json:"operation"`
		URLFilter       string   `json:"url"`
		IgnoreEndpoints []string `json:"ignore_endpoints"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	switch params.Operation {
	case "analyze":
		responseData := map[string]any{
			"status":     "ok",
			"operation":  "analyze",
			"violations": []any{},
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", responseData)}

	case "report":
		responseData := map[string]any{
			"status":    "ok",
			"operation": "report",
			"endpoints": []any{},
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", responseData)}

	case "clear":
		clearResult := map[string]any{
			"action": "cleared",
			"status": "ok",
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", clearResult)}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "operation parameter must be 'analyze', 'report', or 'clear'", "Use a valid value for 'operation'", withParam("operation"), withHint("analyze, report, or clear"))}
	}
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
