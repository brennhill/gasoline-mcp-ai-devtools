// Purpose: Builds and manipulates MCP image and text content blocks within tool responses.
// Why: Separates content-block construction from JSON marshaling and size clamping.
package mcp

import (
	"encoding/json"
	"strings"
)

// ImageContentBlock creates an MCP image content block with base64-encoded data.
// mimeType should be "image/png" or "image/jpeg".
func ImageContentBlock(base64Data, mimeType string) MCPContentBlock {
	return MCPContentBlock{
		Type:     "image",
		Data:     base64Data,
		MimeType: mimeType,
	}
}

// AppendImageToResponse adds an image content block to an existing MCP response.
// If the response cannot be parsed, it is returned unchanged.
func AppendImageToResponse(resp JSONRPCResponse, base64Data, mimeType string) JSONRPCResponse {
	if base64Data == "" {
		return resp
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	result.Content = append(result.Content, ImageContentBlock(base64Data, mimeType))
	// Error impossible: simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// AppendWarningsToResponse adds a warnings content block to an MCP response if there are any.
func AppendWarningsToResponse(resp JSONRPCResponse, warnings []string) JSONRPCResponse {
	if len(warnings) == 0 {
		return resp
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	warningText := "_warnings: " + strings.Join(warnings, "; ")
	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: warningText,
	})
	// Error impossible: simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// MutateToolResult unmarshals the response result into MCPToolResult, applies the
// mutation function, and remarshals. Returns the original response unchanged if
// unmarshal or remarshal fails.
func MutateToolResult(resp JSONRPCResponse, fn func(*MCPToolResult)) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	fn(&result)
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// PrependWarningToResponse prepends a warning string to the first text block of an MCP response.
func PrependWarningToResponse(resp JSONRPCResponse, warning string) JSONRPCResponse {
	return MutateToolResult(resp, func(r *MCPToolResult) {
		if len(r.Content) > 0 && r.Content[0].Type == "text" {
			r.Content[0].Text = warning + r.Content[0].Text
		} else {
			r.Content = append([]MCPContentBlock{{Type: "text", Text: warning}}, r.Content...)
		}
	})
}

// AppendWarningsToToolResult mutates a parsed MCP tool result in-place by adding a
// warning content block. It returns true if warnings were appended.
func AppendWarningsToToolResult(result *MCPToolResult, warnings []string) bool {
	if result == nil || len(warnings) == 0 {
		return false
	}
	warningText := "_warnings: " + strings.Join(warnings, "; ")
	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: warningText,
	})
	return true
}
