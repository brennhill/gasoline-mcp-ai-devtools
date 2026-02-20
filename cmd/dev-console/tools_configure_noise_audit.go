// tools_configure_noise_audit.go — Configure handlers for noise rules, session diffs, and audit log.
package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/audit"
	cfg "github.com/dev-console/dev-console/internal/tools/configure"
)

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
