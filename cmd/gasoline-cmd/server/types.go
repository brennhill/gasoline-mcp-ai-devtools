// types.go — JSON-RPC 2.0 protocol types for the client.
// Mirrors the server-side types but kept separate to avoid import cycles.
package server

import "encoding/json"

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPToolResult represents the result of an MCP tool call.
type MCPToolResult struct {
	Content []MCPContentBlock `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}

// MCPContentBlock represents a single content block in an MCP tool result.
type MCPContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
