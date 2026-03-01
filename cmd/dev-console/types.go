// Purpose: Re-exports core MCP wire types (JSONRPCRequest, JSONRPCResponse, MCPTool) as package-level aliases.
// Why: Allows cmd/dev-console code to reference MCP types by short names without direct internal/mcp imports.

package main

import (
	"github.com/dev-console/dev-console/internal/mcp"
)

// Type aliases — all callers in package main continue to use these names unchanged.
type JSONRPCRequest = mcp.JSONRPCRequest
type JSONRPCResponse = mcp.JSONRPCResponse
type JSONRPCError = mcp.JSONRPCError
type MCPTool = mcp.MCPTool
