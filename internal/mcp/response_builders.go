// response_builders.go — Convenience constructors for JSONRPCResponse.
package mcp

// Succeed wraps a JSONResponse result in a JSONRPCResponse for req.
func Succeed(req JSONRPCRequest, summary string, data any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: JSONResponse(summary, data)}
}
