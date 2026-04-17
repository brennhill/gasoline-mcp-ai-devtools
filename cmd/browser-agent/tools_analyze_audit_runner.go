// Purpose: Runs audit category handlers and normalizes raw MCP output to scoreable data.
// Why: Keeps toolAnalyzeAudit focused on category orchestration and weighted aggregation.
// Docs: docs/features/feature/best-practices-audit/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
)

// runAuditCategory runs a single audit category and extracts score/findings.
func runAuditCategory(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, cat auditCategory) toolanalyze.AuditCategoryResult {
	resp := cat.Handler(h, req, args)

	// Parse the response to extract findings.
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "Failed to parse result", Error: err.Error()}
	}

	if result.IsError {
		errMsg := "unknown error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "Category failed", Error: errMsg}
	}

	// Extract JSON data from response.
	if len(result.Content) == 0 {
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "No data available", Error: "no content returned"}
	}

	text := result.Content[0].Text
	jsonStart := -1
	for i, ch := range text {
		if ch == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "No structured data", Error: "could not parse response"}
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "Could not parse audit data", Error: "malformed JSON in response"}
	}

	return toolanalyze.ScoreAuditCategory(cat.Name, data)
}
