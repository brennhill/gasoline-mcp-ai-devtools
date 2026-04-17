// Purpose: Thin adapter for generate(csp) and generate(sri) — delegates to toolgenerate sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
)

func (h *ToolHandler) toolGenerateCSP(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleGenerateCSP(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolGenerateSRI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleGenerateSRI(h.generateDeps(), req, args)
}
