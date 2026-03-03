// Purpose: Handles configure noise-rule CRUD and auto-detection operations.
// Why: Keeps noise filtering behavior isolated from session diff and audit-log actions.
// Docs: docs/features/feature/noise-filtering/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/noise"
	cfg "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/configure"
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
	rules := make([]noise.NoiseRule, len(args.Rules))
	for i, r := range args.Rules {
		rules[i] = noise.NoiseRule{
			Category:       r.Category,
			Classification: r.Classification,
			MatchSpec: noise.NoiseMatchSpec{
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
	consoleEntries := make([]noise.LogEntry, len(h.server.entries))
	for i, e := range h.server.entries {
		consoleEntries[i] = noise.LogEntry(e)
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
