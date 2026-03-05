// Purpose: Implements CLI-side JSON-RPC transport to the local MCP endpoint.
// Why: Isolates request/response wire handling from CLI orchestration and daemon startup concerns.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// callTool builds a JSON-RPC tools/call request, POSTs to /mcp, and parses the response.
func callTool(baseURL, toolName string, mcpArgs map[string]any, timeoutMs int) (*MCPToolResult, error) {
	body, err := buildToolCallBody(toolName, mcpArgs)
	if err != nil {
		return nil, err
	}

	respBody, err := postToolCall(baseURL+"/mcp", body, timeoutMs)
	if err != nil {
		return nil, err
	}

	return parseToolCallResponse(respBody)
}

// buildToolCallBody creates the JSON-RPC request body for a tools/call.
func buildToolCallBody(toolName string, mcpArgs map[string]any) ([]byte, error) {
	params := map[string]any{"name": toolName, "arguments": mcpArgs}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	rpcReq := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      "cli-1",
		Method:  "tools/call",
		Params:  paramsJSON,
	}
	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	return body, nil
}

// postToolCall sends a JSON-RPC request to the MCP endpoint and returns the raw response body.
func postToolCall(endpoint string, body []byte, timeoutMs int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq) // #nosec G704 -- endpoint comes from ensureDaemon() and is localhost-only
	if err != nil {
		return nil, fmt.Errorf("server connection error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxPostBodySize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// parseToolCallResponse parses a JSON-RPC response into an MCPToolResult.
func parseToolCallResponse(respBody []byte) (*MCPToolResult, error) {
	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("server error (%d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var toolResult MCPToolResult
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}
	return &toolResult, nil
}
