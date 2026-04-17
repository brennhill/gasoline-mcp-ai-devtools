// Purpose: Thin adapter for generate(test) — delegates to toolgenerate sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
)

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleGenerateTest(h.generateDeps(), req, args)
}
