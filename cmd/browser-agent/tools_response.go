// Purpose: Provides MCP response builders (text, markdown, JSON, error) and safe marshal/unmarshal helpers for tool results.
// Why: Standardizes response shaping across all five tools through a single set of formatting functions.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func safeMarshal(v any, fallback string) json.RawMessage {
	return mcp.SafeMarshal(v, fallback)
}

// buildQueryParams marshals a string-keyed map into JSON for query dispatch.
// Falls back to `{}` on marshal failure (impossible for map[string]any with primitive values).
func buildQueryParams(fields map[string]any) json.RawMessage {
	return safeMarshal(fields, "{}")
}

func lenientUnmarshal(args json.RawMessage, v any) {
	mcp.LenientUnmarshal(args, v)
}

func mcpTextResponse(text string) json.RawMessage {
	return mcp.TextResponse(text)
}

// succeed builds a success JSONRPCResponse with a JSON summary + data payload.
func succeed(req JSONRPCRequest, summary string, data any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: mcpJSONResponse(summary, data)}
}

// succeedText builds a success JSONRPCResponse with a plain text payload.
func succeedText(req JSONRPCRequest, text string) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: mcpTextResponse(text)}
}

// succeedRaw builds a success JSONRPCResponse with a pre-built Result payload.
func succeedRaw(req JSONRPCRequest, result json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: result}
}

// fail builds an error JSONRPCResponse with a structured error payload (isError=true).
func fail(req JSONRPCRequest, code, message, retry string, opts ...func(*StructuredError)) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: mcpStructuredError(code, message, retry, opts...)}
}

// failJSON builds an error JSONRPCResponse with a JSON data payload (isError=true).
func failJSON(req JSONRPCRequest, summary string, data any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: mcpJSONErrorResponse(summary, data)}
}

// parseArgs unmarshals JSON args into v. Returns (resp, true) if parsing failed.
func parseArgs(req JSONRPCRequest, args json.RawMessage, v any) (JSONRPCResponse, bool) {
	if err := json.Unmarshal(args, v); err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"), true
	}
	return JSONRPCResponse{}, false
}

func mcpJSONErrorResponse(summary string, data any) json.RawMessage {
	return mcp.JSONErrorResponse(summary, data)
}

func mcpJSONResponse(summary string, data any) json.RawMessage {
	return mcp.JSONResponse(summary, data)
}

func appendWarningsToResponse(resp JSONRPCResponse, warnings []string) JSONRPCResponse {
	return mcp.AppendWarningsToResponse(resp, warnings)
}

// mutateToolResult unmarshals the response result into MCPToolResult, applies the
// mutation function, and remarshals. Returns the original response unchanged if
// unmarshal or remarshal fails.
func mutateToolResult(resp JSONRPCResponse, fn func(*MCPToolResult)) JSONRPCResponse {
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

	return mutateToolResult(resp, func(r *MCPToolResult) {
		if len(r.Content) == 0 {
			return
		}

		text := r.Content[0].Text
		// Find the JSON object within the text (after the summary line).
		jsonStart := -1
		for i := 0; i < len(text); i++ {
			if text[i] == '{' {
				jsonStart = i
				break
			}
		}
		if jsonStart < 0 {
			return
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
			return
		}

		data["blocked_actions"] = actions
		data["blocked_reason"] = reason

		dataJSON, err := json.Marshal(data)
		if err != nil {
			return
		}

		r.Content[0].Text = text[:jsonStart] + string(dataJSON)
	})
}
