// response_builders.go — Convenience constructors for JSONRPCResponse.
package mcp

import "encoding/json"

// Succeed wraps a JSONResponse result in a JSONRPCResponse for req.
func Succeed(req JSONRPCRequest, summary string, data any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: JSONResponse(summary, data)}
}

// SucceedText wraps a TextResponse result in a JSONRPCResponse for req.
func SucceedText(req JSONRPCRequest, text string) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: TextResponse(text)}
}

// Fail builds an error JSONRPCResponse with a structured error payload (isError=true).
func Fail(req JSONRPCRequest, code, message, recovery string, opts ...func(*StructuredError)) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: StructuredErrorResponse(code, message, recovery, opts...)}
}

// ParseArgs unmarshals JSON args into dest. Returns (resp, true) if parsing failed.
func ParseArgs(req JSONRPCRequest, args json.RawMessage, dest any) (JSONRPCResponse, bool) {
	if err := json.Unmarshal(args, dest); err != nil {
		return Fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"), true
	}
	return JSONRPCResponse{}, false
}
