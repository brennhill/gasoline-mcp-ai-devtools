// Purpose: Package-level declaration for the schema package that houses all tool schema definitions.

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
