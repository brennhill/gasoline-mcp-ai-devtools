// Purpose: Thin adapter for generate(pr_summary) — delegates to toolgenerate sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
)

func (h *ToolHandler) toolGeneratePRSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandlePRSummary(h.generateDeps(), req, args)
}
