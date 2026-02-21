// tools_param_hints.go — Inline valid-parameter hints for structured validation errors.
package main

import (
	"encoding/json"
	"sort"
	"strings"
)

// appendValidParamsHintOnError enriches structured validation errors with an inline
// "Valid params" hint so LLMs can recover without a second discovery call.
func (h *ToolHandler) appendValidParamsHintOnError(resp JSONRPCResponse, toolName string, args json.RawMessage) JSONRPCResponse {
	if len(resp.Result) == 0 {
		return resp
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || !result.IsError || len(result.Content) == 0 {
		return resp
	}

	var serr StructuredError
	if err := json.Unmarshal([]byte(extractJSONFromTextBlock(result.Content[0].Text)), &serr); err != nil {
		return resp
	}
	if !isParamValidationErrorCode(serr.Error) {
		return resp
	}

	hint := buildValidParamsHint(toolName, args, h.getToolSchema(toolName))
	if hint == "" {
		return resp
	}
	if strings.Contains(strings.ToLower(serr.Hint), "valid params") {
		return resp
	}
	if serr.Hint == "" {
		serr.Hint = hint
	} else {
		serr.Hint += " " + hint
	}

	opts := []func(*StructuredError){
		withRetryable(serr.Retryable),
		withHint(serr.Hint),
	}
	if serr.Param != "" {
		opts = append(opts, withParam(serr.Param))
	}
	if serr.RetryAfterMs > 0 {
		opts = append(opts, withRetryAfterMs(serr.RetryAfterMs))
	}
	if serr.Final {
		opts = append(opts, withFinal(true))
	}
	resp.Result = mcpStructuredError(serr.Error, serr.Message, serr.Retry, opts...)
	return resp
}

func isParamValidationErrorCode(code string) bool {
	return code == ErrMissingParam || code == ErrInvalidParam || code == ErrUnknownMode
}

func buildValidParamsHint(toolName string, args json.RawMessage, schema map[string]any) string {
	if toolName == "generate" {
		if hint := generateModeValidParamsHint(args); hint != "" {
			return hint
		}
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return ""
	}
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return "Valid params: " + strings.Join(keys, ", ")
}

func generateModeValidParamsHint(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}

	var raw map[string]any
	if err := json.Unmarshal(args, &raw); err != nil {
		return ""
	}

	mode, _ := raw["what"].(string)
	if mode == "" {
		mode, _ = raw["format"].(string)
	}
	valid, ok := generateValidParams[mode]
	if !ok || len(valid) == 0 {
		return ""
	}

	keys := make([]string, 0, len(valid))
	for key := range valid {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return "Valid params for '" + mode + "': " + strings.Join(keys, ", ")
}

func extractJSONFromTextBlock(text string) string {
	for i, ch := range text {
		if ch == '{' || ch == '[' {
			return text[i:]
		}
	}
	return text
}
