// Purpose: Bridge transport error and tool-error envelope helpers.

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/dev-console/dev-console/internal/bridge"
)

type bridgeToolErrorOptions struct {
	ErrorCode     string
	Subsystem     string
	Reason        string
	Retryable     bool
	RetryAfterMs  int
	FallbackUsed  bool
	CorrelationID string
	Detail        string
}

// handleDaemonNotReady sends appropriate error responses when the daemon is not available.
func handleDaemonNotReady(req JSONRPCRequest, status string, signal func(), framing bridge.StdioFraming) {
	switch status {
	case "method_not_found":
		sendBridgeError(req.ID, -32601, "Method not found: "+req.Method, framing)
	case "starting":
		sendToolErrorWithOptions(req.ID, "Server is starting up. Please retry this tool call in 2 seconds.", framing, bridgeToolErrorOptions{
			ErrorCode:    "daemon_starting",
			Subsystem:    "bridge_startup",
			Reason:       "daemon_starting",
			Retryable:    true,
			RetryAfterMs: 2000,
		})
	default:
		sendToolErrorWithOptions(req.ID, status, framing, bridgeToolErrorOptions{
			ErrorCode:    "daemon_not_ready",
			Subsystem:    "bridge_startup",
			Reason:       "daemon_not_ready",
			Retryable:    true,
			RetryAfterMs: 2000,
		})
	}
	signal()
}

// sendBridgeParseError sends a JSON-RPC parse error (id must be null per spec).
func sendBridgeParseError(_ []byte, err error, framing bridge.StdioFraming) {
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil, // JSON-RPC: parse errors must have null id
		Error:   &JSONRPCError{Code: -32700, Message: "Parse error: " + err.Error()},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	writeMCPPayload(respJSON, framing)
}

// sendBridgeError sends a JSON-RPC error response to stdout.
func sendBridgeError(id any, code int, message string, framing bridge.StdioFraming) {
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	writeMCPPayload(respJSON, framing)
}

// sendToolError sends a tool result with isError: true (soft error, not protocol error).
// This tells the LLM the tool ran but returned an error, allowing it to retry.
func sendToolError(id any, message string, framing bridge.StdioFraming) {
	sendToolErrorWithOptions(id, message, framing, bridgeToolErrorOptions{})
}

func sendToolErrorWithOptions(id any, message string, framing bridge.StdioFraming, opts bridgeToolErrorOptions) {
	errorCode := opts.ErrorCode
	if errorCode == "" {
		errorCode = "bridge_tool_error"
	}
	subsystem := opts.Subsystem
	if subsystem == "" {
		subsystem = "bridge"
	}
	reason := opts.Reason
	if reason == "" {
		reason = "tool_error"
	}
	correlationID := opts.CorrelationID
	if correlationID == "" {
		correlationID = bridgeRequestIDString(id)
	}

	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": message},
		},
		"isError":        true,
		"status":         "error",
		"error_code":     errorCode,
		"subsystem":      subsystem,
		"reason":         reason,
		"retryable":      opts.Retryable,
		"fallback_used":  opts.FallbackUsed,
		"correlation_id": correlationID,
	}
	if opts.Retryable && opts.RetryAfterMs > 0 {
		result["retry_after_ms"] = opts.RetryAfterMs
	}
	if opts.Detail != "" {
		result["detail"] = opts.Detail
	}
	// Error impossible: map contains only primitive types and nested maps
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	writeMCPPayload(respJSON, framing)
}

func bridgeRequestIDString(id any) string {
	switch v := id.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.RawMessage:
		if len(v) == 0 {
			return ""
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			return s
		}
		var n float64
		if err := json.Unmarshal(v, &n); err == nil {
			return strconv.FormatFloat(n, 'f', -1, 64)
		}
		return strings.TrimSpace(string(v))
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}
