package main

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
)

func safeMarshal(v any, fallback string) json.RawMessage {
	return mcp.SafeMarshal(v, fallback)
}

func lenientUnmarshal(args json.RawMessage, v any) {
	mcp.LenientUnmarshal(args, v)
}

func mcpTextResponse(text string) json.RawMessage {
	return mcp.TextResponse(text)
}

func mcpErrorResponse(text string) json.RawMessage {
	return mcp.ErrorResponse(text)
}

// ResponseFormat tags each response for documentation and testing.
type ResponseFormat string

const (
	FormatMarkdown ResponseFormat = "markdown"
	FormatJSON     ResponseFormat = "json"
)

func mcpMarkdownResponse(summary string, markdown string) json.RawMessage {
	return mcp.MarkdownResponse(summary, markdown)
}

func mcpJSONErrorResponse(summary string, data any) json.RawMessage {
	return mcp.JSONErrorResponse(summary, data)
}

func mcpJSONResponse(summary string, data any) json.RawMessage {
	return mcp.JSONResponse(summary, data)
}

func markdownTable(headers []string, rows [][]string) string {
	return mcp.MarkdownTable(headers, rows)
}

func truncate(s string, maxLen int) string {
	return mcp.Truncate(s, maxLen)
}

func appendWarningsToResponse(resp JSONRPCResponse, warnings []string) JSONRPCResponse {
	return mcp.AppendWarningsToResponse(resp, warnings)
}

// injectCSPBlockedActions adds blocked_actions and blocked_reason to a JSON
// response when the current page CSP restricts script execution. When CSP is
// clear the response is returned unchanged (zero token cost). (#262)
func (h *ToolHandler) injectCSPBlockedActions(resp JSONRPCResponse) JSONRPCResponse {
	restricted, level := h.capture.GetCSPStatus()
	if !restricted {
		return resp
	}
	actions, reason := capture.CSPBlockedActions(level)
	if actions == nil {
		return resp
	}

	// Parse the response to inject fields into the JSON data payload.
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		return resp
	}

	text := result.Content[0].Text
	// Find the JSON object within the text (after the summary line).
	jsonStart := -1
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		return resp
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		return resp
	}

	data["blocked_actions"] = actions
	data["blocked_reason"] = reason

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return resp
	}

	result.Content[0].Text = text[:jsonStart] + string(dataJSON)
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}
