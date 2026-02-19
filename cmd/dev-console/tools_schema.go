// tools_schema.go — MCP tool schema assembler.
// Delegates to internal/schema for all tool definitions.
package main

import "github.com/dev-console/dev-console/internal/schema"

// ToolsList returns all MCP tool definitions.
func (h *ToolHandler) ToolsList() []MCPTool {
	return schema.AllTools()
}
