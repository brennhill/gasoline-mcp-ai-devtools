package main

import "github.com/dev-console/dev-console/internal/schema"

// ToolsList returns all MCP tool definitions.
func (h *ToolHandler) ToolsList() []MCPTool {
	return schema.AllTools()
}
