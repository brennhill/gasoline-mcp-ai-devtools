// types.go — MCP JSON-RPC protocol types (aliases to internal/mcp).
package main

import (
	"github.com/dev-console/dev-console/internal/mcp"
)

// Type aliases — all callers in package main continue to use these names unchanged.
type JSONRPCRequest = mcp.JSONRPCRequest
type JSONRPCResponse = mcp.JSONRPCResponse
type JSONRPCError = mcp.JSONRPCError
type MCPTool = mcp.MCPTool
