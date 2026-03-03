// Purpose: Observe response decoration helpers (disconnect warnings + alert blocks).
// Why: Keeps post-dispatch response shaping reusable and independent from mode parsing/dispatch.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import "encoding/json"

// prependDisconnectWarning adds a warning to the first content block when the extension is disconnected.
func (h *ToolHandler) prependDisconnectWarning(resp JSONRPCResponse) JSONRPCResponse {
	warning := "⚠ Extension is not connected — results may be stale or empty. Ensure the Gasoline extension shows 'Connected' and a tab is tracked.\n\n"
	return prependWarningToResponse(resp, warning)
}

// appendAlertsToResponse adds an alerts content block to an existing MCP response.
func (h *ToolHandler) appendAlertsToResponse(resp JSONRPCResponse, alerts []Alert) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	alertText := formatAlertsBlock(alerts)
	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: alertText,
	})

	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types.
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}
