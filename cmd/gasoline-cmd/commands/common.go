// common.go — Shared utilities for command argument parsing.
// Contains action normalization, result building, and text extraction.
package commands

import (
	"encoding/json"
	"strings"

	"github.com/dev-console/dev-console/cmd/gasoline-cmd/output"
)

// NormalizeAction converts CLI-style kebab-case actions to MCP snake_case.
// e.g., "get-text" -> "get_text", "network-waterfall" -> "network_waterfall".
func NormalizeAction(action string) string {
	return strings.ReplaceAll(action, "-", "_")
}

// BuildResult constructs an output.Result from MCP response content.
func BuildResult(tool, action, textContent string, isError bool) *output.Result {
	result := &output.Result{
		Success:     !isError,
		Tool:        tool,
		Action:      action,
		TextContent: textContent,
	}

	if isError {
		result.Error = textContent
		return result
	}

	// Try to parse text content as JSON for structured data
	var data map[string]any
	if err := json.Unmarshal([]byte(textContent), &data); err == nil {
		result.Data = data
	}

	return result
}

// ExtractText concatenates text from multiple MCP content blocks.
func ExtractText(blocks []json.RawMessage) string {
	var parts []string
	for _, block := range blocks {
		var cb struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(block, &cb); err == nil && cb.Text != "" {
			parts = append(parts, cb.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// parseFlag extracts a flag value from an args slice.
// Returns the value and remaining args (with the flag pair removed).
func parseFlag(args []string, flag string) (string, []string) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			val := args[i+1]
			remaining := make([]string, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)
			return val, remaining
		}
	}
	return "", args
}

// parseFlagInt extracts an integer flag value from an args slice.
func parseFlagInt(args []string, flag string) (int, bool, []string) {
	val, remaining := parseFlag(args, flag)
	if val == "" {
		return 0, false, args
	}

	var n int
	for _, c := range val {
		if c < '0' || c > '9' {
			return 0, false, args
		}
		n = n*10 + int(c-'0')
	}
	return n, true, remaining
}

// parseFlagBool checks if a boolean flag is present in args.
func parseFlagBool(args []string, flag string) (bool, []string) {
	for i, a := range args {
		if a == flag {
			remaining := make([]string, 0, len(args)-1)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return true, remaining
		}
	}
	return false, args
}
