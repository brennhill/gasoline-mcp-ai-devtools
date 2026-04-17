// tools_generate_adapter.go — Bridges the toolgenerate package to the main ToolHandler.
// Purpose: Provides the generateDeps accessor that satisfies toolgenerate.Deps.
// Why: Keeps the toolgenerate package decoupled from the main package's god object.

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"

// generateDeps returns the ToolHandler as a toolgenerate.Deps.
// *ToolHandler satisfies the toolgenerate.Deps interface directly.
func (h *ToolHandler) generateDeps() toolgenerate.Deps {
	return h
}
