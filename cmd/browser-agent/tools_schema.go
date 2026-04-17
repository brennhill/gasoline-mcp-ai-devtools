// Purpose: Delegates MCP tools/list to internal/schema.AllTools() for tool definition discovery.
// Why: Keeps tool JSON schema definitions in a dedicated internal package while exposing them through ToolHandler.

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"

// ToolsList returns all MCP tool definitions.
func (h *ToolHandler) ToolsList() []MCPTool {
	return schema.AllTools()
}
