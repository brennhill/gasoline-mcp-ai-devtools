// Purpose: Owns protocol.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// protocol.go — MCP JSON-RPC protocol foundation types.
// Contains the core JSON-RPC 2.0 request/response/error types used for MCP communication.
// Zero dependencies - foundational layer used by all packages.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
// SPEC:MCP — Fields in this file use camelCase where required by the MCP protocol spec.
package types

import "encoding/json"

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"` // SPEC:MCP — JSON-RPC 2.0 spec
	// any: JSON-RPC 2.0 spec allows ID to be string, number, or null
	ID       any             `json:"id"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params,omitempty"`
	ClientID string          `json:"-"` // per-request client ID for multi-client isolation (not serialized)
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"` // SPEC:MCP — JSON-RPC 2.0 spec
	// any: JSON-RPC 2.0 spec allows ID to be string, number, or null (must match request)
	ID     any             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPTool represents a tool in the MCP protocol
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// any: JSON Schema is inherently dynamic with nested objects/arrays of varying types
	InputSchema map[string]any `json:"inputSchema"` // SPEC:MCP — camelCase required by MCP protocol
	// any: MCP _meta field contains arbitrary tool metadata (data counts, hints, etc.)
	Meta map[string]any `json:"_meta,omitempty"`
}
