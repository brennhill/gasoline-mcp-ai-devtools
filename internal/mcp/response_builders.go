// response_builders.go — Convenience constructors for JSONRPCResponse.
package mcp

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

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

// SucceedRaw builds a success JSONRPCResponse with a pre-built Result payload.
func SucceedRaw(req JSONRPCRequest, result json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: result}
}

// FailJSON builds an error JSONRPCResponse with a JSON data payload (isError=true).
func FailJSON(req JSONRPCRequest, summary string, data any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: JSONErrorResponse(summary, data)}
}

// ParseArgs unmarshals JSON args into dest. Returns (resp, true) if parsing failed.
func ParseArgs(req JSONRPCRequest, args json.RawMessage, dest any) (JSONRPCResponse, bool) {
	if err := json.Unmarshal(args, dest); err != nil {
		return Fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"), true
	}
	return JSONRPCResponse{}, false
}

// RequireString validates that a string parameter is non-empty.
// Returns (resp, true) if validation failed.
func RequireString(req JSONRPCRequest, value, paramName, hint string) (JSONRPCResponse, bool) {
	if value != "" {
		return JSONRPCResponse{}, false
	}
	return Fail(req, ErrMissingParam,
		"Required parameter '"+paramName+"' is missing",
		hint,
		WithParam(paramName)), true
}

// BuildQueryParams marshals a string-keyed map into JSON for query dispatch.
// Falls back to `{}` on marshal failure.
func BuildQueryParams(fields map[string]any) json.RawMessage {
	return SafeMarshal(fields, "{}")
}

// NewCorrelationID generates a unique correlation ID with the given prefix.
func NewCorrelationID(prefix string) string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), time.Now().UnixNano())
	}
	n := int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 | int64(b[3])<<24 |
		int64(b[4])<<32 | int64(b[5])<<40 | int64(b[6])<<48 | int64(b[7]&0x7f)<<56
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), n)
}
