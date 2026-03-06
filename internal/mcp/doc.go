// Purpose: Package mcp — MCP protocol types, structured errors, validation, and response helpers.
// Why: Gives all tools consistent protocol handling and machine-readable error semantics.
// Docs: docs/features/feature/query-service/index.md

/*
Package mcp defines the core MCP (Model Context Protocol) types, structured error
handling, parameter validation, and response formatting used across all five tools.

Key types:
  - JSONRPCRequest: incoming JSON-RPC 2.0 request with client ID isolation.
  - MCPToolResult: tool call result with content blocks and error flag.
  - MCPTool: tool schema definition (name, description, input schema).
  - ToolError: structured error with code, message, retry hint, and diagnostic context.

Key functions:
  - SafeMarshal: defensive JSON marshaling with fallback.
  - GetJSONFieldNames: extracts known JSON field names from struct tags for validation.
  - NewToolError: creates a structured error with diagnostic hints.
*/
package mcp
