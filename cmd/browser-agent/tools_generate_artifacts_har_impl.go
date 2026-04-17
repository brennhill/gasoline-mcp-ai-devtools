// Purpose: Thin adapter for generate(har) — delegates to toolgenerate sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
)

func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleExportHAR(h.generateDeps(), req, args)
}
