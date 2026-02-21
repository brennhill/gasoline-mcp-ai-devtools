// mcp_stdout.go — centralized stdout emitters for MCP/JSON-RPC wrapper paths.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/bridge"
)

// writeMCPPayload is the only stdout emitter used by MCP wrapper responses.
func writeMCPPayload(payload []byte, framing bridge.StdioFraming) {
	normalized := normalizeMCPPayload(payload)
	out := activeMCPTransportWriter()
	mcpStdoutMu.Lock()
	defer mcpStdoutMu.Unlock()
	if framing == bridge.StdioFramingContentLength {
		_, _ = fmt.Fprintf(out, "Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(normalized), normalized)
	} else {
		// Wrapper invariant: line-framed MCP output is always exactly one JSON
		// payload plus exactly one newline.
		_, _ = out.Write(normalized)
		_, _ = out.Write([]byte("\n"))
	}
	flushStdout()
}

// normalizeMCPPayload trims outer whitespace and guarantees a valid JSON payload.
func normalizeMCPPayload(payload []byte) []byte {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) > 0 && json.Valid(trimmed) {
		return trimmed
	}

	stderrf("[gasoline-bridge] ERROR: stdout invariant violation: invalid JSON payload (len=%d)\n", len(payload))
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil,
		Error: &JSONRPCError{
			Code:    -32603,
			Message: "Wrapper emitted invalid JSON payload",
		},
	}
	// Error impossible: simple struct with no circular refs or unsupported types.
	respJSON, _ := json.Marshal(errResp)
	return respJSON
}

// sendStartupError sends a JSON-RPC error response before exiting.
// This ensures the parent process (IDE) receives a proper error instead of empty response.
func sendStartupError(message string) {
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      "startup",
		Error: &JSONRPCError{
			Code:    -32603,
			Message: message,
		},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	writeMCPPayload(respJSON, bridge.StdioFramingLine)
	syncStdoutBestEffort()
	time.Sleep(100 * time.Millisecond) // Allow OS to flush pipe to parent
}
