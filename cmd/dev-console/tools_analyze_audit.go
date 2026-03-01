// Purpose: Aggregates performance, accessibility, security, and best-practices analyzers into a single scored audit report.
// Why: Provides a Lighthouse-style combined score without requiring agents to call each analyzer separately.
// Docs: docs/features/feature/best-practices-audit/index.md

package main

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/tools/observe"
)

// auditCategory defines a category for the combined audit.
type auditCategory struct {
	Name    string
	Handler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
	Weight  float64
}

// defaultAuditCategories returns the available audit categories.
func defaultAuditCategories() []auditCategory {
	return []auditCategory{
		{Name: "performance", Handler: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return observe.CheckPerformance(h, req, args)
		}, Weight: 1.0},
		{Name: "accessibility", Handler: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return observe.RunA11yAudit(h, req, args)
		}, Weight: 1.0},
		{Name: "security", Handler: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return h.toolSecurityAudit(req, args)
		}, Weight: 1.0},
		{Name: "best_practices", Handler: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return h.toolAuditThirdParties(req, args)
		}, Weight: 1.0},
	}
}

// validAuditCategories is the set of valid category names.
var validAuditCategories = map[string]bool{
	"performance":    true,
	"accessibility":  true,
	"security":       true,
	"best_practices": true,
}

// auditCategoryResult holds the result for one audit category.
type auditCategoryResult struct {
	Score    int            `json:"score"`
	Findings []any          `json:"findings"`
	Summary  string         `json:"summary"`
	Error    string         `json:"error,omitempty"`
}

func (h *ToolHandler) toolAnalyzeAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Categories []string `json:"categories"`
		Summary    bool     `json:"summary"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Validate categories
	requestedCategories := params.Categories
	if len(requestedCategories) == 0 {
		requestedCategories = []string{"performance", "accessibility", "security", "best_practices"}
	}
	for _, cat := range requestedCategories {
		if !validAuditCategories[cat] {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"Unknown audit category: "+cat,
				"Use valid categories: performance, accessibility, security, best_practices",
				withParam("categories"),
			)}
		}
	}

	// Build category set for filtering
	categorySet := make(map[string]bool, len(requestedCategories))
	for _, c := range requestedCategories {
		categorySet[c] = true
	}

	allCategories := defaultAuditCategories()
	categoryResults := make(map[string]auditCategoryResult)
	var totalScore float64
	var totalWeight float64

	// Run categories sequentially (avoids correlation_id collision)
	for _, cat := range allCategories {
		if !categorySet[cat.Name] {
			continue
		}

		catResult := runAuditCategory(h, req, args, cat)
		categoryResults[cat.Name] = catResult
		totalScore += float64(catResult.Score) * cat.Weight
		totalWeight += cat.Weight
	}

	overallScore := 0
	if totalWeight > 0 {
		overallScore = int(totalScore / totalWeight)
	}

	_, _, trackedURL := h.capture.GetTrackingStatus()

	// When summary=true, strip findings to reduce output size
	catOutput := make(map[string]any, len(categoryResults))
	for name, cr := range categoryResults {
		if params.Summary {
			m := map[string]any{
				"score":          cr.Score,
				"summary":        cr.Summary,
				"findings_count": len(cr.Findings),
			}
			if cr.Error != "" {
				m["error"] = cr.Error
			}
			catOutput[name] = m
		} else {
			catOutput[name] = cr
		}
	}

	responseData := map[string]any{
		"categories":    catOutput,
		"overall_score": overallScore,
		"url":           trackedURL,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Combined audit report", responseData)}
}

// runAuditCategory runs a single audit category and extracts score/findings.
func runAuditCategory(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, cat auditCategory) auditCategoryResult {
	resp := cat.Handler(h, req, args)

	// Parse the response to extract findings
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return auditCategoryResult{Score: 0, Findings: []any{}, Summary: "Failed to parse result", Error: err.Error()}
	}

	if result.IsError {
		errMsg := "unknown error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		return auditCategoryResult{Score: 0, Findings: []any{}, Summary: "Category failed", Error: errMsg}
	}

	// Extract JSON data from response
	if len(result.Content) == 0 {
		return auditCategoryResult{Score: 0, Findings: []any{}, Summary: "No data available", Error: "no content returned"}
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
		return auditCategoryResult{Score: 0, Findings: []any{}, Summary: "No structured data", Error: "could not parse response"}
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		return auditCategoryResult{Score: 0, Findings: []any{}, Summary: "Could not parse audit data", Error: "malformed JSON in response"}
	}

	return scoreAuditCategory(cat.Name, data)
}

// scoreAuditCategory maps raw analyzer output to a 0-100 score.
func scoreAuditCategory(name string, data map[string]any) auditCategoryResult {
	result := auditCategoryResult{Score: 100, Findings: []any{}}

	switch name {
	case "performance":
		result = scorePerformance(data)
	case "accessibility":
		result = scoreAccessibility(data)
	case "security":
		result = scoreSecurity(data)
	case "best_practices":
		result = scoreBestPractices(data)
	}

	return result
}

func scorePerformance(data map[string]any) auditCategoryResult {
	findings := extractFindings(data, "issues", "warnings")
	score := 100 - len(findings)*10
	if score < 0 {
		score = 0
	}
	summary := "Performance analysis"
	if len(findings) == 0 {
		summary = "No performance issues detected"
	}
	return auditCategoryResult{Score: score, Findings: findings, Summary: summary}
}

func scoreAccessibility(data map[string]any) auditCategoryResult {
	findings := extractFindings(data, "violations", "issues")
	score := 100 - len(findings)*5
	if score < 0 {
		score = 0
	}
	summary := "Accessibility audit"
	if len(findings) == 0 {
		summary = "No accessibility violations detected"
	}
	return auditCategoryResult{Score: score, Findings: findings, Summary: summary}
}

func scoreSecurity(data map[string]any) auditCategoryResult {
	findings := extractFindings(data, "findings", "issues", "vulnerabilities")
	score := 100
	for _, f := range findings {
		if fm, ok := f.(map[string]any); ok {
			switch fm["severity"] {
			case "critical":
				score -= 25
			case "high":
				score -= 15
			case "medium":
				score -= 10
			case "low":
				score -= 5
			default:
				score -= 5
			}
		} else {
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	}
	summary := "Security audit"
	if len(findings) == 0 {
		summary = "No security issues detected"
	}
	return auditCategoryResult{Score: score, Findings: findings, Summary: summary}
}

func scoreBestPractices(data map[string]any) auditCategoryResult {
	findings := extractFindings(data, "third_parties", "issues", "findings")
	score := 100 - len(findings)*3
	if score < 0 {
		score = 0
	}
	summary := "Best practices audit"
	if len(findings) == 0 {
		summary = "No best practices issues detected"
	}
	return auditCategoryResult{Score: score, Findings: findings, Summary: summary}
}

// extractFindings extracts array findings from data using multiple candidate keys.
func extractFindings(data map[string]any, keys ...string) []any {
	for _, key := range keys {
		if arr, ok := data[key].([]any); ok && len(arr) > 0 {
			return arr
		}
	}
	return []any{}
}
