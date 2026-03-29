// Purpose: Observe response decoration helpers (disconnect warnings + alert blocks).
// Why: Keeps post-dispatch response shaping reusable and independent from mode parsing/dispatch.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

// prependDisconnectWarning adds a warning to the first content block when the extension is disconnected.
func (h *ToolHandler) prependDisconnectWarning(resp JSONRPCResponse) JSONRPCResponse {
	warning := "⚠ Extension is not connected — results may be stale or empty. Ensure the Kaboom extension shows 'Connected' and a tab is tracked.\n\n"
	return prependWarningToResponse(resp, warning)
}

// appendAlertsToResponse adds an alerts content block to an existing MCP response.
func (h *ToolHandler) appendAlertsToResponse(resp JSONRPCResponse, alerts []Alert) JSONRPCResponse {
	return mutateToolResult(resp, func(r *MCPToolResult) {
		r.Content = append(r.Content, MCPContentBlock{
			Type: "text",
			Text: formatAlertsBlock(alerts),
		})
	})
}
