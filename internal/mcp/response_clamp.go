package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
