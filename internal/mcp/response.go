// Purpose: Provides SafeMarshal, JSON response writers, and stdout-safe MCP result formatting helpers.
// Docs: docs/features/feature/query-service/index.md

// Layout:
// - response_json.go: JSON marshal helpers and text/json response constructors
// - response_content.go: image/warning response content utilities
// - response_markdown.go: markdown table and text truncation helpers
// - response_clamp.go: payload-size clamping with JSON-aware boundary truncation
package mcp
