// schema.go — MCP tool schema assembler.
// Pure data — returns MCPTool structs with zero runtime dependencies.
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
