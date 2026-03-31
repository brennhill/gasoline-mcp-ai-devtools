// Purpose: Thin adapter for generate(sarif) — delegates to toolgenerate sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
)

func (h *ToolHandler) toolExportSARIF(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleExportSARIF(h.generateDeps(), req, args)
}
