// tools_schema.go — MCP tool schema definitions.
// Contains the ToolsList() function that returns all MCP tool definitions with their schemas.
package main

// toolsList returns all MCP tool definitions.
// Note: _meta field removed to comply with strict MCP spec (fixes Cursor schema errors).
func (h *ToolHandler) ToolsList() []MCPTool {
	// Data counts no longer included in tool definitions (not in MCP spec)
	// Available via configure tool's get_health action instead
	return []MCPTool{
		{
			Name:        "observe",
			Description: "Read current browser state. Call observe() first before interact() or generate().\n\nModes: errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, accessibility, timeline, error_clusters, history, security_audit, third_party_audit, security_diff, command_result, pending_commands, failed_commands.\n\nFilters: limit, url, method, status_min/max, connection_id, direction, last_n, format, severity.\n\nPagination: Pass after_cursor/before_cursor/since_cursor from metadata. Use restart_on_eviction=true if cursor expires.\n\nResponses: JSON format.\n\nNote: network_bodies only captures fetch(). Use network_waterfall for all network requests.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"what": map[string]any{
						"type":        "string",
						"description": "What to observe or analyze",
						"enum":        []string{"errors", "logs", "extension_logs", "network_waterfall", "network_bodies", "websocket_events", "websocket_status", "actions", "vitals", "page", "tabs", "pilot", "performance", "accessibility", "timeline", "error_clusters", "history", "security_audit", "third_party_audit", "security_diff", "command_result", "pending_commands", "failed_commands"},
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "Maximum entries to return (applies to logs, network_waterfall, network_bodies, websocket_events, actions, audit_log)",
					},
					"after_cursor": map[string]any{
						"type":        "string",
						"description": "Return entries older than this cursor (backward pagination). Cursor format: 'timestamp:sequence' from previous response.",
					},
					"before_cursor": map[string]any{
						"type":        "string",
						"description": "Return entries newer than this cursor (forward pagination).",
					},
					"since_cursor": map[string]any{
						"type":        "string",
						"description": "Return ALL entries newer than this cursor (inclusive, no limit).",
					},
					"restart_on_eviction": map[string]any{
						"type":        "boolean",
						"description": "If cursor expired (buffer overflow), automatically restart from oldest available entry.",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"method": map[string]any{
						"type":        "string",
						"description": "Filter by HTTP method (applies to network_bodies)",
					},
					"status_min": map[string]any{
						"type":        "number",
						"description": "Minimum status code (applies to network_bodies)",
					},
					"status_max": map[string]any{
						"type":        "number",
						"description": "Maximum status code (applies to network_bodies)",
					},
					"connection_id": map[string]any{
						"type":        "string",
						"description": "Filter by WebSocket connection ID",
					},
					"direction": map[string]any{
						"type":        "string",
						"description": "Filter by direction",
						"enum":        []string{"incoming", "outgoing"},
					},
					"last_n": map[string]any{
						"type":        "number",
						"description": "Return only the last N items",
					},
					"scope": map[string]any{
						"type":        "string",
						"description": "CSS selector to scope audit (applies to accessibility)",
					},
					"tags": map[string]any{
						"type":        "array",
						"description": "WCAG tags to test (applies to accessibility)",
						"items":       map[string]any{"type": "string"},
					},
					"force_refresh": map[string]any{
						"type":        "boolean",
						"description": "Bypass cache (applies to accessibility)",
					},
					"include": map[string]any{
						"type":        "array",
						"description": "Categories to include (applies to timeline)",
						"items":       map[string]any{"type": "string"},
					},
					"checks": map[string]any{
						"type":        "array",
						"description": "Which checks to run (applies to security_audit)",
						"items": map[string]any{
							"type": "string",
							"enum": []string{"credentials", "pii", "headers", "cookies", "transport", "auth"},
						},
					},
					"severity_min": map[string]any{
						"type":        "string",
						"description": "Minimum severity to report (applies to security_audit)",
						"enum":        []string{"critical", "high", "medium", "low", "info"},
					},
					"first_party_origins": map[string]any{
						"type":        "array",
						"description": "Origins to consider first-party (applies to third_party_audit)",
						"items":       map[string]any{"type": "string"},
					},
					"include_static": map[string]any{
						"type":        "boolean",
						"description": "Include static-only origins (applies to third_party_audit)",
					},
					"custom_lists": map[string]any{
						"type":        "object",
						"description": "Custom allowed/blocked/internal domain lists (applies to third_party_audit)",
					},
					"action": map[string]any{
						"type":        "string",
						"description": "Snapshot action: snapshot, compare, list (applies to security_diff)",
						"enum":        []string{"snapshot", "compare", "list"},
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Snapshot name (applies to security_diff)",
					},
					"compare_from": map[string]any{
						"type":        "string",
						"description": "Baseline snapshot (applies to security_diff)",
					},
					"compare_to": map[string]any{
						"type":        "string",
						"description": "Target snapshot (applies to security_diff)",
					},
					"correlation_id": map[string]any{
						"type":        "string",
						"description": "Correlation ID for async command tracking (applies to command_result)",
					},
				},
				"required": []string{"what"},
			},
		},
		{
			Name:        "generate",
			Description: "CREATE ARTIFACTS. Generates production-ready outputs from captured data: test (Playwright tests), reproduction (bug scripts), pr_summary, csp, sarif, har, sri.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"format": map[string]any{
						"type":        "string",
						"description": "What to generate",
						"enum":        []string{"reproduction", "test", "pr_summary", "sarif", "har", "csp", "sri"},
					},
					"error_message": map[string]any{
						"type":        "string",
						"description": "Error message for context (applies to reproduction)",
					},
					"last_n": map[string]any{
						"type":        "number",
						"description": "Use only the last N actions (applies to reproduction)",
					},
					"base_url": map[string]any{
						"type":        "string",
						"description": "Replace origin in URLs (applies to reproduction, test)",
					},
					"include_screenshots": map[string]any{
						"type":        "boolean",
						"description": "Insert page.screenshot() calls (applies to reproduction)",
					},
					"generate_fixtures": map[string]any{
						"type":        "boolean",
						"description": "Generate fixtures from captured network data (applies to reproduction)",
					},
					"visual_assertions": map[string]any{
						"type":        "boolean",
						"description": "Add toHaveScreenshot() assertions (applies to reproduction)",
					},
					"test_name": map[string]any{
						"type":        "string",
						"description": "Name for the generated test (applies to test)",
					},
					"assert_network": map[string]any{
						"type":        "boolean",
						"description": "Include network response assertions (applies to test)",
					},
					"assert_no_errors": map[string]any{
						"type":        "boolean",
						"description": "Assert no console errors occurred (applies to test)",
					},
					"assert_response_shape": map[string]any{
						"type":        "boolean",
						"description": "Assert response body shape matches (applies to test)",
					},
					"scope": map[string]any{
						"type":        "string",
						"description": "CSS selector to scope (applies to sarif)",
					},
					"include_passes": map[string]any{
						"type":        "boolean",
						"description": "Include passing rules (applies to sarif)",
					},
					"save_to": map[string]any{
						"type":        "string",
						"description": "File path to save output (applies to sarif, har)",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "Filter by URL substring (applies to har)",
					},
					"method": map[string]any{
						"type":        "string",
						"description": "Filter by HTTP method (applies to har)",
					},
					"status_min": map[string]any{
						"type":        "number",
						"description": "Minimum status code (applies to har)",
					},
					"status_max": map[string]any{
						"type":        "number",
						"description": "Maximum status code (applies to har)",
					},
					"mode": map[string]any{
						"type":        "string",
						"description": "CSP strictness mode (applies to csp)",
						"enum":        []string{"strict", "moderate", "report_only"},
					},
					"include_report_uri": map[string]any{
						"type":        "boolean",
						"description": "Include report-uri directive (applies to csp)",
					},
					"exclude_origins": map[string]any{
						"type":        "array",
						"description": "Origins to exclude from CSP (applies to csp)",
						"items":       map[string]any{"type": "string"},
					},
					"resource_types": map[string]any{
						"type":        "array",
						"description": "Filter by resource type: 'script', 'stylesheet' (applies to sri)",
						"items":       map[string]any{"type": "string"},
					},
					"origins": map[string]any{
						"type":        "array",
						"description": "Filter by specific origins (applies to sri)",
						"items":       map[string]any{"type": "string"},
					},
				},
				"required": []string{"format"},
			},
		},
		{
			Name:        "configure",
			Description: "CUSTOMIZE THE SESSION. Actions: noise_rule, store, load, diff_sessions, validate_api, audit_log, streaming, query_dom, capture, record_event, dismiss, clear, health, test_boundary_start, test_boundary_end.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "Configuration action to perform",
						"enum":        []string{"store", "load", "noise_rule", "dismiss", "clear", "capture", "record_event", "query_dom", "diff_sessions", "validate_api", "audit_log", "health", "streaming", "test_boundary_start", "test_boundary_end"},
					},
					"store_action": map[string]any{
						"type":        "string",
						"description": "Store sub-action: save, load, list, delete, stats",
						"enum":        []string{"save", "load", "list", "delete", "stats"},
					},
					"namespace": map[string]any{
						"type":        "string",
						"description": "Logical grouping for store",
					},
					"key": map[string]any{
						"type":        "string",
						"description": "Storage key",
					},
					"data": map[string]any{
						"type":        "object",
						"description": "JSON data to persist",
					},
					"noise_action": map[string]any{
						"type":        "string",
						"description": "Noise sub-action: add, remove, list, reset, auto_detect",
						"enum":        []string{"add", "remove", "list", "reset", "auto_detect"},
					},
					"rules": map[string]any{
						"type":        "array",
						"description": "Noise rules to add",
						"items":       map[string]any{"type": "object"},
					},
					"rule_id": map[string]any{
						"type":        "string",
						"description": "ID of rule to remove",
					},
					"pattern": map[string]any{
						"type":        "string",
						"description": "Regex pattern to dismiss",
					},
					"category": map[string]any{
						"type":        "string",
						"description": "Buffer category",
						"enum":        []string{"console", "network", "websocket"},
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Why this is noise",
					},
					"buffer": map[string]any{
						"type":        "string",
						"description": "Which buffer to clear",
						"enum":        []string{"network", "websocket", "actions", "logs", "all"},
					},
					"selector": map[string]any{
						"type":        "string",
						"description": "CSS selector to query (applies to query_dom)",
					},
					"tab_id": map[string]any{
						"type":        "number",
						"description": "Target tab ID",
					},
					"session_action": map[string]any{
						"type":        "string",
						"description": "Session sub-action: capture, compare, list, delete",
						"enum":        []string{"capture", "compare", "list", "delete"},
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Snapshot name",
					},
					"compare_a": map[string]any{
						"type":        "string",
						"description": "First snapshot for comparison",
					},
					"compare_b": map[string]any{
						"type":        "string",
						"description": "Second snapshot for comparison",
					},
					"operation": map[string]any{
						"type":        "string",
						"description": "API validation operation: analyze, report, clear",
						"enum":        []string{"analyze", "report", "clear"},
					},
					"url": map[string]any{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"ignore_endpoints": map[string]any{
						"type":        "array",
						"description": "URL substrings to exclude",
						"items":       map[string]any{"type": "string"},
					},
					"session_id": map[string]any{
						"type":        "string",
						"description": "Filter by MCP session ID (applies to audit_log)",
					},
					"tool_name": map[string]any{
						"type":        "string",
						"description": "Filter by tool name (applies to audit_log)",
					},
					"since": map[string]any{
						"type":        "string",
						"description": "Only entries after this ISO 8601 timestamp (applies to audit_log)",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "Maximum entries to return (applies to audit_log)",
					},
					"streaming_action": map[string]any{
						"type":        "string",
						"description": "Streaming sub-action: enable, disable, status",
						"enum":        []string{"enable", "disable", "status"},
					},
					"events": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
							"enum": []string{"errors", "network_errors", "performance", "user_frustration", "security", "regression", "anomaly", "ci", "all"},
						},
						"description": "Event categories to stream",
					},
					"throttle_seconds": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     60,
						"description": "Minimum seconds between notifications",
					},
					"severity_min": map[string]any{
						"type":        "string",
						"enum":        []string{"info", "warning", "error"},
						"description": "Minimum severity to stream",
					},
					"test_id": map[string]any{
						"type":        "string",
						"description": "Test ID for boundary marker",
					},
					"label": map[string]any{
						"type":        "string",
						"description": "Human-readable label for test boundary",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "interact",
			Description: "PERFORM BROWSER ACTIONS. Actions: navigate, execute_js, refresh, back, forward, new_tab, highlight, save_state, load_state, list_states, delete_state. Requires AI Web Pilot enabled.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "Browser interaction to perform",
						"enum":        []string{"highlight", "save_state", "load_state", "list_states", "delete_state", "execute_js", "navigate", "refresh", "back", "forward", "new_tab"},
					},
					"selector": map[string]any{
						"type":        "string",
						"description": "CSS selector for element (applies to highlight)",
					},
					"duration_ms": map[string]any{
						"type":        "number",
						"description": "Highlight duration in ms, default 5000",
					},
					"snapshot_name": map[string]any{
						"type":        "string",
						"description": "State snapshot name",
					},
					"include_url": map[string]any{
						"type":        "boolean",
						"description": "Include URL when restoring state",
					},
					"script": map[string]any{
						"type":        "string",
						"description": "JavaScript code to execute (applies to execute_js)",
					},
					"timeout_ms": map[string]any{
						"type":        "number",
						"description": "Execution timeout in ms, default 5000",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "URL to navigate to (required for navigate, new_tab)",
					},
					"tab_id": map[string]any{
						"type":        "number",
						"description": "Target tab ID. Omit for active tab.",
					},
					"correlation_id": map[string]any{
						"type":        "string",
						"description": "Optional ID to link this action to a specific error or investigation.",
					},
				},
				"required": []string{"action"},
			},
		},
	}
}
