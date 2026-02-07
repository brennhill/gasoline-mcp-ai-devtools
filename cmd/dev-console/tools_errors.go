// tools_errors.go — Structured error handling and error codes for MCP tools.
// Defines error constants, StructuredError type, and error response construction.
package main

import (
	"encoding/json"
	"fmt"
)

// Error codes are self-describing snake_case strings.
// Every code tells the LLM what went wrong.
const (
	// Input errors — LLM can fix arguments and retry immediately
	ErrInvalidJSON    = "invalid_json"
	ErrMissingParam   = "missing_param"
	ErrInvalidParam   = "invalid_param"
	ErrUnknownMode    = "unknown_mode"
	ErrPathNotAllowed = "path_not_allowed"

	// State errors — LLM must change state before retrying
	ErrNotInitialized    = "not_initialized"
	ErrNoData            = "no_data"
	ErrCodePilotDisabled = "pilot_disabled" // Named ErrCodePilotDisabled to avoid collision with var ErrCodePilotDisabled in pilot.go
	ErrRateLimited       = "rate_limited"
	ErrCursorExpired     = "cursor_expired" // Cursor pagination: buffer overflow evicted cursor position

	// Communication errors — retry with backoff
	ErrExtTimeout = "extension_timeout"
	ErrExtError   = "extension_error"

	// Internal errors — do not retry
	ErrInternal      = "internal_error"
	ErrMarshalFailed = "marshal_failed"
	ErrExportFailed  = "export_failed"
)

// StructuredError is embedded in MCP text content. Every field is
// self-describing so an LLM can act on it without a lookup table.
type StructuredError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Retry   string `json:"retry"`
	Param   string `json:"param,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

// mcpStructuredError constructs an MCP error response. Format:
//
//	Error: missing_param — Add the 'what' parameter and call again
//	{"error":"missing_param","message":"...","retry":"Add the 'what' parameter and call again","hint":"..."}
//
// The retry string is a plain-English instruction the LLM can follow directly.
func mcpStructuredError(code, message, retry string, opts ...func(*StructuredError)) json.RawMessage {
	se := StructuredError{Error: code, Message: message, Retry: retry}
	for _, opt := range opts {
		opt(&se)
	}

	// Error impossible: StructuredError is a simple struct with no circular refs or unsupported types
	seJSON, _ := json.Marshal(se)
	text := fmt.Sprintf("Error: %s — %s\n%s", code, retry, string(seJSON))

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
		IsError: true,
	}
	return safeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// withParam is an option function to add param field to StructuredError.
func withParam(p string) func(*StructuredError) {
	return func(se *StructuredError) { se.Param = p }
}

// withHint is an option function to add hint field to StructuredError.
func withHint(h string) func(*StructuredError) {
	return func(se *StructuredError) { se.Hint = h }
}
