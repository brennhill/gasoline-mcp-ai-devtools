// Purpose: Hosts MCP entrypoint for get_health requests.
// Why: Keeps transport-layer request/response wiring separate from health state and composition.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

// toolGetHealth is the MCP tool handler for get_health.
// It returns comprehensive server health metrics.
func (h *ToolHandler) toolGetHealth(req JSONRPCRequest) JSONRPCResponse {
	if h.healthMetrics == nil {
		return fail(req, ErrInternal, "Health metrics not initialized", "Internal server error — do not retry")
	}

	response := h.healthMetrics.GetHealth(h.capture, h.server, version)
	return succeed(req, "Server health", response)
}
