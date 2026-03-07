// Purpose: Shared post-processing helpers for MCP tool result envelopes.
// Why: Isolates response parsing/error checks used by HandleToolCall.

package main

import "encoding/json"

func parseToolResultForPostProcessing(raw json.RawMessage) (*MCPToolResult, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false
	}
	return &result, true
}

func isToolResultError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return false
	}
	return result.IsError
}
