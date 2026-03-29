// Purpose: Generic JSON argument parsing, schema validation, unknown field warnings, and log quality checking for tool inputs.
// Why: Centralizes input validation so all tools reject malformed parameters with consistent structured errors.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func unmarshalWithWarnings(data json.RawMessage, v any) ([]string, error) {
	return mcp.UnmarshalWithWarnings(data, v)
}

func validateParamsAgainstSchema(data json.RawMessage, schema map[string]any) []string {
	return mcp.ValidateParamsAgainstSchema(data, schema)
}
