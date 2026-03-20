// Purpose: Provides defensive JSON marshal/unmarshal helpers with fallback values for MCP responses.
// Why: Isolates JSON safety wrappers from content building and markdown formatting.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
)

// SafeMarshal performs defensive JSON marshaling with a fallback value.
func SafeMarshal(v any, fallback string) json.RawMessage {
	resultJSON, err := json.Marshal(v)
	if err != nil {
		// This should never happen with simple structs, but handle it defensively
		fmt.Fprintf(os.Stderr, "[gasoline] JSON marshal error: %v\n", err)
		return json.RawMessage(fallback)
	}
	return json.RawMessage(resultJSON)
}

// LenientUnmarshal parses optional JSON params, logging failures to stderr for debugging.
// Behavior is deliberately lenient: malformed optional params are logged but not rejected,
// allowing callers to fall through to defaults.
func LenientUnmarshal(args json.RawMessage, v any) {
	if len(args) == 0 {
		return
	}
	if err := json.Unmarshal(args, v); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] optional param parse: %v (args: %.100s)\n", err, string(args))
	}
}

// TextResponse constructs an MCP tool result containing a single text content block.
func TextResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}]}`)
}

// errorResponse constructs an MCP tool error result containing a single text content block.
func errorResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
		IsError: true,
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

func marshalSummaryData(summary string, data any) (string, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if summary == "" {
		return string(dataJSON), nil
	}
	return summary + "\n" + string(dataJSON), nil
}

func jsonResultWithSummary(summary string, data any, isError bool) json.RawMessage {
	text, err := marshalSummaryData(summary, data)
	if err != nil {
		return errorResponse("Failed to serialize response: " + err.Error())
	}
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
		IsError: isError,
	}
	if isError {
		return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}]}`)
}

// JSONErrorResponse constructs an MCP tool error result with a summary line
// followed by compact JSON. Sets IsError: true so LLMs recognize the failure.
func JSONErrorResponse(summary string, data any) json.RawMessage {
	return jsonResultWithSummary(summary, data, true)
}

// JSONResponse constructs an MCP tool result with a summary line prefix
// followed by compact JSON. Use for nested, irregular, or highly variable data.
func JSONResponse(summary string, data any) json.RawMessage {
	return jsonResultWithSummary(summary, data, false)
}
