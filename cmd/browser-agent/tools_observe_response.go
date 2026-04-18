// Purpose: Observe response decoration helpers — delegates to cmd/browser-agent/internal/toolobserve.
// Why: Keeps backwards compatibility for any in-package callers while logic lives in the extracted package.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
)

// appendAlertsToResponse adds an alerts content block to an existing MCP response.
func (h *ToolHandler) appendAlertsToResponse(resp JSONRPCResponse, alerts []Alert) JSONRPCResponse {
	return toolobserve.AppendAlertsToResponse(resp, alerts)
}
