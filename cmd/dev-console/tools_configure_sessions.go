// Purpose: Handles configure session-diff wrappers and session manager delegation.
// Why: Separates session diff concerns from noise-rule and audit-log configure actions.
// Docs: docs/features/feature/noise-filtering/index.md

package main

import (
	"encoding/json"

	cfg "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/configure"
)

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
