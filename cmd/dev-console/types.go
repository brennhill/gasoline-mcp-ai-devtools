// types.go — MCP JSON-RPC protocol types.
// Contains the core JSON-RPC 2.0 request/response/error types used for MCP communication.
package main

import "encoding/json"

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC  string          `json:"jsonrpc"` // camelCase: JSON-RPC 2.0 spec standard
	ID       interface{}     `json:"id"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params,omitempty"`
	ClientID string          `json:"-"` // per-request client ID for multi-client isolation (not serialized)
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"` // camelCase: JSON-RPC 2.0 spec standard
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPTool represents a tool in the MCP protocol
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"` // camelCase: MCP spec standard
	Meta        map[string]interface{} `json:"_meta,omitempty"`
}
