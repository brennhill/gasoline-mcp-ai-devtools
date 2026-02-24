// Purpose: Defines JSON schema contracts for tool arguments and responses.
// Why: Keeps tool interfaces strict and synchronized across server, extension, and clients.
// Docs: docs/features/feature/analyze-tool/index.md
// Docs: docs/features/feature/interact-explore/index.md
// Docs: docs/features/feature/observe/index.md
// Docs: docs/features/feature/config-profiles/index.md
// Docs: docs/features/feature/test-generation/index.md

package schema

import "github.com/dev-console/dev-console/internal/mcp"

// AllTools returns all MCP tool definitions.
func AllTools() []mcp.MCPTool {
	return []mcp.MCPTool{
		ObserveToolSchema(),
		AnalyzeToolSchema(),
		GenerateToolSchema(),
		ConfigureToolSchema(),
		InteractToolSchema(),
	}
}
