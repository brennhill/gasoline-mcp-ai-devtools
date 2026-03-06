// Purpose: Defines MCP content block and tool result types duplicated to avoid circular imports.
// Why: Provides local type definitions so the redaction engine can re-marshal responses without importing cmd/.
package redaction

import "regexp"

// MCPContentBlock represents a single content block in an MCP tool response.
// This is duplicated from cmd/dev-console/tools_core.go to avoid circular imports.
// IMPORTANT: Must stay in sync with the main package's MCPContentBlock.
// Note: text is NOT omitempty here because the redaction engine re-marshals
// content blocks and must preserve empty text fields (B2 regression guard).
type MCPContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Data     string `json:"data,omitempty"`     // SPEC:MCP — base64-encoded image data (type="image")
	MimeType string `json:"mimeType,omitempty"` // SPEC:MCP — MIME type for image content
}

// MCPToolResult represents the result of an MCP tool call.
// This is duplicated from cmd/dev-console/tools_core.go to avoid circular imports.
// IMPORTANT: Must stay in sync with the main package's MCPToolResult.
type MCPToolResult struct {
	Content  []MCPContentBlock `json:"content"`
	IsError  bool              `json:"isError,omitempty"` // SPEC:MCP
	Metadata map[string]any    `json:"metadata,omitempty"`
}

// RedactionPattern represents a single redaction rule.
type RedactionPattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement,omitempty"`
}

// RedactionConfig represents the JSON configuration file structure.
type RedactionConfig struct {
	Patterns []RedactionPattern `json:"patterns"`
}

// compiledPattern holds a pre-compiled regex and its replacement string.
type compiledPattern struct {
	name        string
	regex       *regexp.Regexp
	replacement string
	validate    func(match string) bool // optional post-match validation (e.g., Luhn)
}

// RedactionEngine applies a set of compiled patterns to text.
// It is safe for concurrent use after construction.
type RedactionEngine struct {
	patterns []compiledPattern
}
