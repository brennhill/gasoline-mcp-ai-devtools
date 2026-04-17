// Purpose: Package-level declaration for the schema package that houses all tool schema definitions.

package schema

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// AllTools returns all MCP tool definitions.
func AllTools() []mcp.MCPTool {
	return []mcp.MCPTool{
		ObserveToolSchema(),
		analyzeToolSchema(),
		generateToolSchema(),
		configureToolSchema(),
		InteractToolSchema(),
	}
}
