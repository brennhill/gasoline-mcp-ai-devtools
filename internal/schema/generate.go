// generate.go — MCP schema definition for the generate tool.
package schema

import "github.com/dev-console/dev-console/internal/mcp"

// GenerateToolSchema returns the MCP tool definition for the generate tool.
func GenerateToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "generate",
		Description: "Generate artifacts from captured data: reproduction (bug script), csp (Content Security Policy), sarif (accessibility report). Test generation: test_from_context, test_heal, test_classify. Annotation formats: visual_test, annotation_report, annotation_issues.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"what": map[string]any{
					"type": "string",
					"enum": []string{"reproduction", "test", "pr_summary", "har", "csp", "sri", "sarif", "visual_test", "annotation_report", "annotation_issues", "test_from_context", "test_heal", "test_classify"},
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Deprecated alias for 'what'",
					"enum":        []string{"reproduction", "test", "pr_summary", "har", "csp", "sri", "sarif", "visual_test", "annotation_report", "annotation_issues", "test_from_context", "test_heal", "test_classify"},
				},
				"telemetry_mode": map[string]any{
					"type":        "string",
					"description": "Telemetry metadata mode for this call: off, auto, full",
					"enum":        []string{"off", "auto", "full"},
				},
				"error_message": map[string]any{
					"type":        "string",
					"description": "Error context (reproduction)",
				},
				"last_n": map[string]any{
					"type":        "number",
					"description": "Use last N actions (reproduction)",
				},
				"base_url": map[string]any{
					"type":        "string",
					"description": "Replace origin in URLs",
				},
				"include_screenshots": map[string]any{
					"type":        "boolean",
					"description": "Add screenshot calls (reproduction)",
				},
				"generate_fixtures": map[string]any{
					"type":        "boolean",
					"description": "Generate network fixtures (reproduction)",
				},
				"visual_assertions": map[string]any{
					"type":        "boolean",
					"description": "Add visual assertions (reproduction)",
				},
				"test_name": map[string]any{
					"type":        "string",
					"description": "Test name (test, visual_test)",
				},
				"assert_network": map[string]any{
					"type":        "boolean",
					"description": "Assert network responses (test)",
				},
				"assert_no_errors": map[string]any{
					"type":        "boolean",
					"description": "Assert no console errors (test)",
				},
				"assert_response_shape": map[string]any{
					"type":        "boolean",
					"description": "Assert response shape (test)",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "CSS selector scope (sarif)",
				},
				"include_passes": map[string]any{
					"type":        "boolean",
					"description": "Include passing rules (sarif)",
				},
				"save_to": map[string]any{
					"type":        "string",
					"description": "File path to save output",
				},
				"url": map[string]any{
					"type":        "string",
					"description": "URL filter (har)",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP method filter (har)",
				},
				"status_min": map[string]any{
					"type":        "number",
					"description": "Min status code (har)",
				},
				"status_max": map[string]any{
					"type":        "number",
					"description": "Max status code (har)",
				},
				"mode": map[string]any{
					"type": "string",
					"enum": []string{"strict", "moderate", "report_only"},
				},
				"include_report_uri": map[string]any{
					"type":        "boolean",
					"description": "Include report-uri (csp)",
				},
				"exclude_origins": map[string]any{
					"type":        "array",
					"description": "Origins to exclude (csp)",
					"items":       map[string]any{"type": "string"},
				},
				"resource_types": map[string]any{
					"type":        "array",
					"description": "Resource types: script, stylesheet (sri)",
					"items":       map[string]any{"type": "string"},
				},
				"origins": map[string]any{
					"type":        "array",
					"description": "Filter origins (sri)",
					"items":       map[string]any{"type": "string"},
				},
				"annot_session": map[string]any{
					"type":        "string",
					"description": "Named annotation session (applies to visual_test, annotation_report, annotation_issues)",
				},
				"context": map[string]any{
					"type":        "string",
					"description": "Test context (test_from_context)",
					"enum":        []string{"error", "interaction", "regression"},
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Action type (test_heal: analyze/repair/batch, test_classify: failure/batch)",
				},
				"test_file": map[string]any{
					"type":        "string",
					"description": "Test file path (test_heal analyze)",
				},
				"test_dir": map[string]any{
					"type":        "string",
					"description": "Test directory (test_heal batch)",
				},
				"broken_selectors": map[string]any{
					"type":        "array",
					"description": "Broken selectors (test_heal repair)",
					"items":       map[string]any{"type": "string"},
				},
				"auto_apply": map[string]any{
					"type":        "boolean",
					"description": "Auto-apply high-confidence fixes (test_heal repair)",
				},
				"failure": map[string]any{
					"type":        "object",
					"description": "Single test failure (test_classify failure)",
					"properties": map[string]any{
						"test_name":   map[string]any{"type": "string", "description": "Name of the failing test"},
						"error":       map[string]any{"type": "string", "description": "Error message"},
						"screenshot":  map[string]any{"type": "string", "description": "Base64-encoded screenshot (optional)"},
						"trace":       map[string]any{"type": "string", "description": "Stack trace"},
						"duration_ms": map[string]any{"type": "number", "description": "Test duration in milliseconds"},
					},
					"required": []string{"error"},
				},
				"failures": map[string]any{
					"type":        "array",
					"description": "Multiple test failures (test_classify batch)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"test_name":   map[string]any{"type": "string", "description": "Name of the failing test"},
							"error":       map[string]any{"type": "string", "description": "Error message"},
							"screenshot":  map[string]any{"type": "string", "description": "Base64-encoded screenshot (optional)"},
							"trace":       map[string]any{"type": "string", "description": "Stack trace"},
							"duration_ms": map[string]any{"type": "number", "description": "Test duration in milliseconds"},
						},
						"required": []string{"error"},
					},
				},
				"error_id": map[string]any{
					"type":        "string",
					"description": "Specific error ID (test_from_context error)",
				},
				"include_mocks": map[string]any{
					"type":        "boolean",
					"description": "Include network mocks (test_from_context)",
				},
				"output_format": map[string]any{
					"type":        "string",
					"description": "Output format: playwright|gasoline (reproduction), file|inline (test_from_context)",
					"enum":        []string{"playwright", "gasoline", "file", "inline"},
				},
			},
			"required": []string{"what"},
		},
	}
}
