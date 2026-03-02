// Purpose: Implements generate(test) artifact assembly.
// Why: Keeps test script generation isolated from other generate artifact formats.

package main

import (
	"encoding/json"
	"fmt"
	"time"

	gen "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/generate"
)

func (h *ToolHandler) generateTestImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params gen.TestGenParams
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.TestName == "" {
		params.TestName = "generated test"
	}

	allActions := h.capture.GetAllEnhancedActions()
	actions := gen.FilterLastN(allActions, params.LastN)
	script := gen.GenerateTestScript(actions, params)

	result := map[string]any{
		"script":       script,
		"test_name":    params.TestName,
		"action_count": len(actions),
		"metadata": map[string]any{
			"generated_at":      time.Now().Format(time.RFC3339),
			"actions_available": len(allActions),
			"actions_included":  len(actions),
			"assert_network":    params.AssertNetwork,
			"assert_no_errors":  params.AssertNoErrors,
		},
	}

	if len(actions) == 0 {
		result["reason"] = "no_actions_captured"
		result["hint"] = "Navigate and interact with the browser first, then call generate(test) again."
	}

	summary := fmt.Sprintf("Playwright test '%s' (%d actions)", params.TestName, len(actions))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, result)}
}
