// tools_response.go — Response formatting and JSON serialization helpers.
// Constructs MCP tool results with proper formatting (text, markdown, JSON).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// safeMarshal performs defensive JSON marshaling with a fallback value.
func safeMarshal(v any, fallback string) json.RawMessage {
	resultJSON, err := json.Marshal(v)
	if err != nil {
		// This should never happen with simple structs, but handle it defensively
		fmt.Fprintf(os.Stderr, "[gasoline] JSON marshal error: %v\n", err)
		return json.RawMessage(fallback)
	}
	return json.RawMessage(resultJSON)
}

// lenientUnmarshal parses optional JSON params, logging failures to stderr for debugging.
// Behavior is deliberately lenient: malformed optional params are logged but not rejected,
// allowing callers to fall through to defaults.
func lenientUnmarshal(args json.RawMessage, v any) {
	if len(args) == 0 {
		return
	}
	if err := json.Unmarshal(args, v); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] optional param parse: %v (args: %.100s)\n", err, string(args))
	}
}

// mcpTextResponse constructs an MCP tool result containing a single text content block.
func mcpTextResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
	}
	return safeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}]}`)
}

// mcpErrorResponse constructs an MCP tool error result containing a single text content block.
func mcpErrorResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
		IsError: true,
	}
	return safeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// ResponseFormat tags each response for documentation and testing.
type ResponseFormat string

const (
	FormatMarkdown ResponseFormat = "markdown"
	FormatJSON     ResponseFormat = "json"
)

// mcpMarkdownResponse constructs an MCP tool result with a summary line
// followed by markdown-formatted content (typically a table).
// Use for flat, uniform data where columns are consistent across rows.
func mcpMarkdownResponse(summary string, markdown string) json.RawMessage {
	text := summary + "\n\n" + markdown

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
	}
	return safeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// mcpJSONResponse constructs an MCP tool result with a summary line prefix
// followed by compact JSON. Use for nested, irregular, or highly variable data.
func mcpJSONResponse(summary string, data any) json.RawMessage {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return mcpErrorResponse("Failed to serialize response: " + err.Error())
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
	resultJSON, _ := json.Marshal(result)
	return json.RawMessage(resultJSON)
}

// markdownTable converts a slice of items into a markdown table.
// headers defines column names. rows contains cell values for each row.
// Pipe chars in cell values are escaped, newlines are replaced with spaces.
func markdownTable(headers []string, rows [][]string) string {
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

// truncate returns s unchanged if len(s) <= maxLen. Otherwise, it truncates
// and appends "..." so the total output length equals maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// appendWarningsToResponse adds a warnings content block to an MCP response if there are any.
func appendWarningsToResponse(resp JSONRPCResponse, warnings []string) JSONRPCResponse {
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
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}
