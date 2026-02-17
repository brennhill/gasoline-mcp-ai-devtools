// Purpose: Owns types.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// types.go — MCP JSON-RPC protocol types.
// Contains the core JSON-RPC 2.0 request/response/error types used for MCP communication.
package main

import (
	"bytes"
	"encoding/json"
)

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"` // camelCase: JSON-RPC 2.0 spec standard
	// any: JSON-RPC 2.0 spec allows ID to be string, number, or null
	ID              any             `json:"id"`
	Method          string          `json:"method"`
	Params          json.RawMessage `json:"params,omitempty"`
	ClientID        string          `json:"-"` // per-request client ID for multi-client isolation (not serialized)
	idPresent       bool            `json:"-"`
	idExplicitNull  bool            `json:"-"`
	idInvalidFormat bool            `json:"-"`
}

// UnmarshalJSON captures whether id was present and whether it was explicitly null.
func (r *JSONRPCRequest) UnmarshalJSON(data []byte) error {
	type rawRequest struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	var raw rawRequest
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}

	r.JSONRPC = raw.JSONRPC
	r.Method = raw.Method
	r.Params = raw.Params
	r.ClientID = ""
	r.ID = nil
	_, r.idPresent = object["id"]
	r.idExplicitNull = false
	r.idInvalidFormat = false

	rawID, ok := object["id"]
	if !ok {
		return nil
	}

	trimmedID := bytes.TrimSpace(rawID)
	if bytes.Equal(trimmedID, []byte("null")) {
		r.idExplicitNull = true
		return nil
	}

	var parsedID any
	if err := json.Unmarshal(trimmedID, &parsedID); err != nil {
		return err
	}
	switch parsedID.(type) {
	case string, float64:
		r.ID = parsedID
	default:
		r.idInvalidFormat = true
	}
	return nil
}

func (r JSONRPCRequest) hasID() bool {
	return r.idPresent || r.ID != nil
}

func (r JSONRPCRequest) hasInvalidID() bool {
	return r.idExplicitNull || r.idInvalidFormat
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"` // camelCase: JSON-RPC 2.0 spec standard
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
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"` // SPEC:MCP — camelCase required by MCP protocol
	// Note: _meta removed - not in MCP spec, caused schema validation errors in Cursor
}
