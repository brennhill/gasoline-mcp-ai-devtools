// Purpose: Configures randomized micro-delays (action jitter) before interact actions for human-like interaction timing.
// Why: Prevents bot-detection by adding configurable random delays that simulate natural user input cadence.

package main

import (
	"encoding/json"
)

// toolConfigureActionJitter handles configure(what="action_jitter").
// Sets randomized micro-delays before interact actions.
func (h *ToolHandler) toolConfigureActionJitter(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		ActionJitterMs *int `json:"action_jitter_ms"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidJSON,
				"Invalid JSON arguments: "+err.Error(),
				"Fix JSON syntax and call again",
			)}
		}
	}

	h.jitterMu.Lock()
	defer h.jitterMu.Unlock()
	if params.ActionJitterMs != nil {
		v := *params.ActionJitterMs
		if v < 0 {
			v = 0
		}
		if v > 5000 {
			v = 5000
		}
		h.actionJitterMaxMs = v
	}
	actionMs := h.actionJitterMaxMs

	result := map[string]any{
		"action_jitter_ms": actionMs,
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Action jitter configured", result)}
}
