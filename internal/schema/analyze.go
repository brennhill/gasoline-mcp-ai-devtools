// analyze.go — MCP schema definition for the analyze tool.
package schema

import "github.com/dev-console/dev-console/internal/mcp"

// AnalyzeToolSchema returns the MCP tool definition for the analyze tool.
func AnalyzeToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "analyze",
		Description: "Trigger active analysis. Creates async queries the extension executes.\n\nSynchronous Mode (Default): Tools now block until the extension returns a result (up to 15s). Set background:true to return immediately with a correlation_id.\n\nDraw Mode: Use annotations to get all annotations from the last draw mode session. Use annotation_detail with correlation_id to get full computed styles and DOM detail for a specific annotation.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"what": map[string]any{
					"type": "string",
					"enum": []string{"dom", "performance", "accessibility", "error_clusters", "history", "security_audit", "third_party_audit", "link_health", "link_validation", "page_summary", "annotations", "annotation_detail", "api_validation", "draw_history", "draw_session", "computed_styles", "forms", "form_validation", "visual_baseline", "visual_diff", "visual_baselines"},
				},
				"telemetry_mode": map[string]any{
					"type":        "string",
					"description": "Telemetry metadata mode for this call: off, auto, full",
					"enum":        []string{"off", "auto", "full"},
				},
				"selector": map[string]any{
					"type":        "string",
					"description": "CSS selector (dom, accessibility, computed_styles, forms)",
				},
				"frame": map[string]any{
					"description": "Target iframe: CSS selector, 0-based index, or \"all\" (dom, accessibility)",
					"type":        "string",
				},
				"sync": map[string]any{
					"type":        "boolean",
					"description": "Wait for result (default: true).",
				},
				"wait": map[string]any{
					"type":        "boolean",
					"description": "Wait for result (default: true). For annotations: blocks up to 5 min for user to finish drawing.",
				},
				"background": map[string]any{
					"type":        "boolean",
					"description": "Run in background and return a correlation_id immediately.",
				},
				"operation": map[string]any{
					"type":        "string",
					"description": "API validation operation",
					"enum":        []string{"analyze", "report", "clear"},
				},
				"ignore_endpoints": map[string]any{
					"type":        "array",
					"description": "URL substrings to exclude (api_validation)",
					"items":       map[string]any{"type": "string"},
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "CSS selector scope (accessibility)",
				},
				"tags": map[string]any{
					"type":        "array",
					"description": "WCAG tags (accessibility)",
					"items":       map[string]any{"type": "string"},
				},
				"force_refresh": map[string]any{
					"type":        "boolean",
					"description": "Bypass cache (accessibility)",
				},
				"domain": map[string]any{
					"type":        "string",
					"description": "Domain to check (link_health)",
				},
				"timeout_ms": map[string]any{
					"type":        "number",
					"description": "Timeout ms (link_health, page_summary, annotations). For annotations with wait=true: default 300000 (5 min), max 600000 (10 min).",
				},
				"world": map[string]any{
					"type":        "string",
					"description": "Execution world for page_summary script",
					"enum":        []string{"auto", "main", "isolated"},
				},
				"tab_id": map[string]any{
					"type":        "number",
					"description": "Target tab ID (dom, page_summary)",
				},
				"max_workers": map[string]any{
					"type":        "number",
					"description": "Max concurrent workers (link_health)",
				},
				"checks": map[string]any{
					"type":        "array",
					"description": "Checks to run (security_audit)",
					"items": map[string]any{
						"type": "string",
						"enum": []string{"credentials", "pii", "headers", "cookies", "transport", "auth"},
					},
				},
				"severity_min": map[string]any{
					"type":        "string",
					"description": "Min severity (security_audit)",
					"enum":        []string{"critical", "high", "medium", "low", "info"},
				},
				"first_party_origins": map[string]any{
					"type":        "array",
					"description": "First-party origins (third_party_audit)",
					"items":       map[string]any{"type": "string"},
				},
				"include_static": map[string]any{
					"type":        "boolean",
					"description": "Include static-only origins (third_party_audit)",
				},
				"custom_lists": map[string]any{
					"type":        "object",
					"description": "Custom domain allow/block lists (third_party_audit)",
				},
				"correlation_id": map[string]any{
					"type":        "string",
					"description": "Correlation ID for fetching annotation detail (applies to annotation_detail)",
				},
				"annot_session": map[string]any{
					"type":        "string",
					"description": "Named session for multi-page annotation review (applies to annotations). Accumulates annotations across pages.",
				},
				"urls": map[string]any{
					"type":        "array",
					"description": "URLs to validate (link_validation)",
					"items":       map[string]any{"type": "string"},
				},
				"file": map[string]any{
					"type":        "string",
					"description": "Session filename from draw_history results (draw_session)",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Baseline name (visual_baseline, visual_diff)",
				},
				"baseline": map[string]any{
					"type":        "string",
					"description": "Baseline name to compare against (visual_diff)",
				},
				"threshold": map[string]any{
					"type":        "number",
					"description": "Pixel diff threshold 0-255 (visual_diff, default 30)",
				},
			},
			"required": []string{"what"},
		},
	}
}
