// Purpose: Defines MCP protocol types, validation, and structured error response helpers.
// Why: Gives all tools consistent protocol validation and machine-readable error semantics.
// Docs: docs/features/feature/query-service/index.md

package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

// ErrorResponse constructs an MCP tool error result containing a single text content block.
func ErrorResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
		IsError: true,
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// MarkdownResponse constructs an MCP tool result with a summary line
// followed by markdown-formatted content (typically a table).
// Use for flat, uniform data where columns are consistent across rows.
func MarkdownResponse(summary string, markdown string) json.RawMessage {
	text := summary + "\n\n" + markdown

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// JSONErrorResponse constructs an MCP tool error result with a summary line
// followed by compact JSON. Sets IsError: true so LLMs recognize the failure.
func JSONErrorResponse(summary string, data any) json.RawMessage {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return ErrorResponse("Failed to serialize response: " + err.Error())
	}

	var text string
	if summary != "" {
		text = summary + "\n" + string(dataJSON)
	} else {
		text = string(dataJSON)
	}

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
		IsError: true,
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// JSONResponse constructs an MCP tool result with a summary line prefix
// followed by compact JSON. Use for nested, irregular, or highly variable data.
func JSONResponse(summary string, data any) json.RawMessage {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return ErrorResponse("Failed to serialize response: " + err.Error())
	}

	var text string
	if summary != "" {
		text = summary + "\n" + string(dataJSON)
	} else {
		text = string(dataJSON)
	}

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return json.RawMessage(resultJSON)
}

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

// MarkdownTable converts a slice of items into a markdown table.
// headers defines column names. rows contains cell values for each row.
// Pipe chars in cell values are escaped, newlines are replaced with spaces.
func MarkdownTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder

	// Header row
	b.WriteString("| ")
	b.WriteString(strings.Join(headers, " | "))
	b.WriteString(" |\n")

	// Separator
	b.WriteString("|")
	for range headers {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")

	// Data rows
	for _, row := range rows {
		escaped := make([]string, len(row))
		for i, cell := range row {
			// Replace newlines with spaces
			cell = strings.ReplaceAll(cell, "\n", " ")
			// Escape pipe characters
			cell = strings.ReplaceAll(cell, "|", `\|`)
			escaped[i] = cell
		}
		b.WriteString("| ")
		b.WriteString(strings.Join(escaped, " | "))
		b.WriteString(" |\n")
	}
	return b.String()
}

// Truncate returns s unchanged if len(s) <= maxLen. Otherwise, it truncates
// and appends "..." so the total output length equals maxLen.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// MaxResponseBytes is the safety-net limit for MCP tool result payloads.
// Responses exceeding this size are truncated with a note.
const MaxResponseBytes = 100_000

// ClampResponseSize truncates the text content of the first content block
// if the marshaled response exceeds MaxResponseBytes.
// Image content blocks are excluded from the byte count to prevent
// screenshots from triggering truncation of text data (#9.4).
// JSON-aware truncation ensures the truncated text is still valid JSON (#9.4.1).
func ClampResponseSize(result json.RawMessage) json.RawMessage {
	if len(result) <= MaxResponseBytes {
		return result
	}

	var toolResult MCPToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil || len(toolResult.Content) == 0 {
		return result
	}

	// Calculate the byte size of image content blocks (excluded from limit).
	// This prevents inline screenshots from causing text truncation.
	imageBytes := 0
	for _, block := range toolResult.Content {
		if block.Type == "image" {
			// Estimate serialized size: base64 data + JSON envelope
			imageBytes += len(block.Data) + 100
		}
	}

	// Effective limit for text content only
	effectiveLimit := MaxResponseBytes + imageBytes
	originalSize := len(result)
	if originalSize <= effectiveLimit {
		return result
	}

	text := toolResult.Content[0].Text
	if text == "" {
		return result
	}

	// Calculate overhead (everything except the first text content)
	overhead := originalSize - len(text)
	if overhead < 0 {
		overhead = 200
	}

	// Target text size to fit within effective limit
	truncNote := fmt.Sprintf("\n\n[truncated: original %d bytes, limit %d bytes. Use pagination parameters (limit, offset, last_n) to page through results.]", originalSize, MaxResponseBytes)
	targetTextLen := effectiveLimit - overhead - len(truncNote) - 100 // 100 bytes margin
	if targetTextLen < 100 {
		targetTextLen = 100
	}

	if len(text) > targetTextLen {
		// JSON-aware truncation: find the last valid JSON boundary to avoid
		// producing malformed JSON that agents cannot parse.
		truncatedText := text[:targetTextLen]
		truncatedText = truncateAtJSONBoundary(truncatedText)
		toolResult.Content[0].Text = truncatedText + truncNote
	}

	clamped, err := json.Marshal(toolResult)
	if err != nil {
		return result
	}
	return json.RawMessage(clamped)
}

// truncateAtJSONBoundary finds the last safe truncation point in a JSON string.
// It looks for the last closing bracket/brace that could form valid JSON,
// or falls back to the last comma boundary to avoid mid-value truncation.
// Uses a stack-based approach to close open structures in correct order (#9.R2).
func truncateAtJSONBoundary(text string) string {
	if len(text) == 0 {
		return text
	}

	// If text starts with { or [, try to truncate at a JSON-safe boundary
	trimmed := strings.TrimSpace(text)
	if len(trimmed) == 0 {
		return text
	}

	firstChar := trimmed[0]
	if firstChar != '{' && firstChar != '[' {
		// Not JSON, truncate as-is
		return text
	}

	// Find the last comma, closing bracket, or closing brace.
	// Truncate just before the last incomplete value.
	lastSafe := len(text)
	for i := len(text) - 1; i >= 0; i-- {
		ch := text[i]
		if ch == ',' || ch == '}' || ch == ']' {
			lastSafe = i + 1
			break
		}
	}

	truncated := text[:lastSafe]

	// Strip trailing comma that would produce invalid JSON (e.g., [1,] or {"a":1,}).
	// The lastSafe search may land on a comma boundary, leaving a dangling comma
	// before the auto-appended closers (#9.R3.1).
	truncated = strings.TrimRight(truncated, ",")

	// Track open structures with a stack so closers are emitted in correct
	// reverse order (e.g., [{ → }] not ]}). (#9.R2)
	var stack []byte
	inString := false
	escaped := false
	for i := 0; i < len(truncated); i++ {
		ch := truncated[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{', '[':
			stack = append(stack, ch)
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		}
	}

	// Close remaining open structures in reverse order
	closers := make([]byte, len(stack))
	for i, opener := range stack {
		if opener == '{' {
			closers[len(stack)-1-i] = '}'
		} else {
			closers[len(stack)-1-i] = ']'
		}
	}

	return truncated + string(closers)
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
