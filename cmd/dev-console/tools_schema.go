// tools_schema.go — MCP tool schema definitions.
// Descriptions are kept minimal to reduce token usage — detailed docs live in server
// instructions and the gasoline://guide resource.
package main

// toolsList returns all MCP tool definitions.
// #lizard forgives
func (h *ToolHandler) ToolsList() []MCPTool {
	return []MCPTool{
		{
			Name:        "observe",
			Description: "Read captured browser state from extension buffers.\n\nnetwork_bodies captures fetch() only; use network_waterfall for all requests. extension_logs = internal debug logs (use logs for console). error_bundles = pre-assembled debug context per error. Use body_key/body_path to extract JSON subtrees from network_bodies.\n\nPagination: pass after_cursor/before_cursor/since_cursor from response metadata. restart_on_eviction=true if cursor expired.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"errors", "logs", "extension_logs", "network_waterfall", "network_bodies", "websocket_events", "websocket_status", "actions", "vitals", "page", "tabs", "pilot", "timeline", "error_bundles", "screenshot", "command_result", "pending_commands", "failed_commands", "saved_videos", "recordings", "recording_actions", "log_diff_report"},
					},
					"telemetry_mode": map[string]any{
						"type":        "string",
						"description": "Telemetry metadata mode for this call: off, auto, full",
						"enum":        []string{"off", "auto", "full"},
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "Max entries to return (default 100, max 1000)",
					},
					"after_cursor": map[string]any{
						"type":        "string",
						"description": "Backward pagination cursor from response metadata",
					},
					"before_cursor": map[string]any{
						"type":        "string",
						"description": "Forward pagination cursor",
					},
					"since_cursor": map[string]any{
						"type":        "string",
						"description": "Return all entries newer than cursor (no limit)",
					},
					"restart_on_eviction": map[string]any{
						"type":        "boolean",
						"description": "Auto-restart if cursor expired",
					},
					"min_level": map[string]any{
						"type":        "string",
						"description": "Min log level (logs)",
						"enum":        []string{"debug", "log", "info", "warn", "error"},
					},
					"level": map[string]any{
						"type":        "string",
						"description": "Exact log level filter (logs)",
						"enum":        []string{"debug", "log", "info", "warn", "error"},
					},
					"source": map[string]any{
						"type":        "string",
						"description": "Exact source filter (logs)",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"method": map[string]any{
						"type":        "string",
						"description": "HTTP method filter",
					},
					"status_min": map[string]any{
						"type":        "number",
						"description": "Min HTTP status code",
					},
					"status_max": map[string]any{
						"type":        "number",
						"description": "Max HTTP status code",
					},
					"body_key": map[string]any{
						"type":        "string",
						"description": "Extract values for a JSON key from response_body (network_bodies)",
					},
					"body_path": map[string]any{
						"type":        "string",
						"description": "Extract JSON value from response_body using path, e.g. data.items[0].id (network_bodies)",
					},
					"connection_id": map[string]any{
						"type":        "string",
						"description": "WebSocket connection ID filter",
					},
					"direction": map[string]any{
						"type": "string",
						"enum": []string{"incoming", "outgoing"},
					},
					"last_n": map[string]any{
						"type":        "number",
						"description": "Return last N items only",
					},
					"include": map[string]any{
						"type":        "array",
						"description": "Categories to include (timeline)",
						"items":       map[string]any{"type": "string"},
					},
					"correlation_id": map[string]any{
						"type":        "string",
						"description": "Async command correlation ID",
					},
					"window_seconds": map[string]any{
						"type":        "number",
						"description": "error_bundles lookback seconds (default 3, max 10)",
					},
				},
				"required": []string{"what"},
			},
		},
		{
			Name:        "analyze",
			Description: "Trigger active analysis. Creates async queries the extension executes.\n\nSynchronous Mode (Default): Tools now block until the extension returns a result (up to 15s). Set background:true to return immediately with a correlation_id.\n\nDraw Mode: Use annotations to get all annotations from the last draw mode session. Use annotation_detail with correlation_id to get full computed styles and DOM detail for a specific annotation.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"dom", "performance", "accessibility", "error_clusters", "history", "security_audit", "third_party_audit", "link_health", "link_validation", "page_summary", "annotations", "annotation_detail", "api_validation", "draw_history", "draw_session"},
					},
					"telemetry_mode": map[string]any{
						"type":        "string",
						"description": "Telemetry metadata mode for this call: off, auto, full",
						"enum":        []string{"off", "auto", "full"},
					},
					"selector": map[string]any{
						"type":        "string",
						"description": "CSS selector (dom, accessibility)",
					},
					"frame": map[string]any{
						"description": "Target iframe: CSS selector, 0-based index, or \"all\" (dom, accessibility)",
						"oneOf": []map[string]any{
							{"type": "string"},
							{"type": "number"},
						},
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
					"session": map[string]any{
						"type":        "string",
						"description": "Named session for multi-page annotation review (applies to annotations). Accumulates annotations across pages.",
					},
				},
				"required": []string{"what"},
			},
		},
		{
			Name:        "generate",
			Description: "Generate artifacts from captured data: reproduction (bug script), csp (Content Security Policy), sarif (accessibility report). Test generation: test_from_context, test_heal, test_classify. Annotation formats: visual_test, annotation_report, annotation_issues.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"format": map[string]any{
						"type": "string",
						"enum": []string{"reproduction", "test", "pr_summary", "har", "csp", "sri", "sarif", "visual_test", "annotation_report", "annotation_issues", "test_from_context", "test_heal", "test_classify"},
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
					"session": map[string]any{
						"type":        "string",
						"description": "Named annotation session (applies to visual_test, annotation_report, annotation_issues)",
					},
				},
				"required": []string{"format"},
			},
		},
		{
			Name:        "configure",
			Description: "Session settings and utilities.\n\nKey actions: health (check server/extension status), clear (reset buffers), noise_rule (suppress recurring console noise), store/load (persist/retrieve session data), streaming (enable push notifications), recording_start/recording_stop (capture browser sessions), playback (replay recordings), log_diff (compare error states).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type": "string",
						"enum": []string{"store", "load", "noise_rule", "clear", "health", "streaming", "test_boundary_start", "test_boundary_end", "recording_start", "recording_stop", "playback", "log_diff", "telemetry"},
					},
					"telemetry_mode": map[string]any{
						"type":        "string",
						"description": "Telemetry metadata mode: off, auto, full. configure(action='telemetry') sets global default. Any tools/call may override per request with telemetry_mode.",
						"enum":        []string{"off", "auto", "full"},
					},
					"store_action": map[string]any{
						"type": "string",
						"enum": []string{"save", "load", "list", "delete", "stats"},
					},
					"namespace": map[string]any{
						"type":        "string",
						"description": "Store grouping",
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
						"type": "string",
						"enum": []string{"add", "remove", "list", "reset", "auto_detect"},
					},
					"rules": map[string]any{
						"type":        "array",
						"description": "Noise rules to add",
						"items":       map[string]any{"type": "object"},
					},
					"rule_id": map[string]any{
						"type":        "string",
						"description": "Rule ID to remove",
					},
					"pattern": map[string]any{
						"type":        "string",
						"description": "Regex pattern",
					},
					"category": map[string]any{
						"type": "string",
						"enum": []string{"console", "network", "websocket"},
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Why this is noise",
					},
					"buffer": map[string]any{
						"type": "string",
						"enum": []string{"network", "websocket", "actions", "logs", "all"},
					},
					"tab_id": map[string]any{
						"type":        "number",
						"description": "Target tab ID",
					},
					"session_action": map[string]any{
						"type": "string",
						"enum": []string{"capture", "compare", "list", "delete"},
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Snapshot name",
					},
					"compare_a": map[string]any{
						"type":        "string",
						"description": "First snapshot to compare",
					},
					"compare_b": map[string]any{
						"type":        "string",
						"description": "Second snapshot to compare",
					},
					"recording_id": map[string]any{
						"type":        "string",
						"description": "Recording ID (recording_stop, playback)",
					},
					"session_id": map[string]any{
						"type":        "string",
						"description": "Filter by session ID",
					},
					"tool_name": map[string]any{
						"type":        "string",
						"description": "Filter by tool name",
					},
					"since": map[string]any{
						"type":        "string",
						"description": "Entries after ISO 8601 timestamp",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "Max entries to return (default 100, max 1000)",
					},
					"streaming_action": map[string]any{
						"type": "string",
						"enum": []string{"enable", "disable", "status"},
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
						"description": "Min seconds between notifications",
					},
					"severity_min": map[string]any{
						"type": "string",
						"enum": []string{"info", "warning", "error"},
					},
					"test_id": map[string]any{
						"type":        "string",
						"description": "Test boundary ID",
					},
					"label": map[string]any{
						"type":        "string",
						"description": "Test boundary label",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "interact",
			Description: "Browser actions. Requires AI Web Pilot.\n\nSynchronous Mode (Default): Tools now block until the extension returns a result (up to 15s). Set background:true to return immediately with a correlation_id.\n\nSelectors: CSS or semantic (text=Submit, role=button, placeholder=Email, label=Name, aria-label=Close). subtitle param composable with any action. analyze=true captures perf_diff. navigate/refresh auto-include perf_diff.\n\nDraw Mode: draw_mode_start activates annotation overlay — user draws rectangles and types feedback, presses ESC to finish. Use analyze({what:'annotations'}) to retrieve results.\n\nCompatibility: action='screenshot' is a backward-compatible alias for observe({what:'screenshot'}).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type": "string",
						"enum": []string{
							"highlight", "subtitle", "save_state", "load_state", "list_states", "delete_state",
							"execute_js", "navigate", "refresh", "back", "forward", "new_tab", "screenshot",
							"click", "type", "paste", "select", "check",
							"get_text", "get_value", "get_attribute",
							"set_attribute", "focus", "scroll_to", "wait_for", "key_press",
							"list_interactive",
							"record_start", "record_stop",
							"upload", "draw_mode_start",
						},
					},
					"telemetry_mode": map[string]any{
						"type":        "string",
						"description": "Telemetry metadata mode for this call: off, auto, full",
						"enum":        []string{"off", "auto", "full"},
					},
					"sync": map[string]any{
						"type":        "boolean",
						"description": "Wait for result (default: true).",
					},
					"wait": map[string]any{
						"type":        "boolean",
						"description": "Alias for sync (default: true).",
					},
					"background": map[string]any{
						"type":        "boolean",
						"description": "Run in background and return a correlation_id immediately.",
					},
					"selector": map[string]any{
						"type":        "string",
						"description": "CSS or semantic selector for target element",
					},
					"frame": map[string]any{
						"description": "Target iframe: CSS selector, 0-based index, or \"all\"",
						"oneOf": []map[string]any{
							{"type": "string"},
							{"type": "number"},
						},
					},
					"duration_ms": map[string]any{
						"type":        "number",
						"description": "Highlight duration ms (default 5000)",
					},
					"snapshot_name": map[string]any{
						"type":        "string",
						"description": "State snapshot name",
					},
					"include_url": map[string]any{
						"type":        "boolean",
						"description": "Restore URL with state",
					},
					"script": map[string]any{
						"type":        "string",
						"description": "JS code (execute_js)",
					},
					"timeout_ms": map[string]any{
						"type":        "number",
						"description": "Timeout ms (default 5000)",
					},
					"text": map[string]any{
						"type":        "string",
						"description": "Text for type/subtitle. key_press keys: Enter, Tab, Escape, Backspace, ArrowDown, ArrowUp, Space.",
					},
					"subtitle": map[string]any{
						"type":        "string",
						"description": "Narration text, composable with any action. Empty clears.",
					},
					"value": map[string]any{
						"type":        "string",
						"description": "Value for select/set_attribute",
					},
					"clear": map[string]any{
						"type":        "boolean",
						"description": "Clear before typing",
					},
					"checked": map[string]any{
						"type":        "boolean",
						"description": "Check/uncheck (default true)",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Attribute or recording name",
					},
					"audio": map[string]any{
						"type":        "string",
						"description": "Recording audio: tab, mic, both. Omit for video-only.",
						"enum":        []string{"tab", "mic", "both"},
					},
					"fps": map[string]any{
						"type":        "number",
						"description": "Recording FPS (5-60, default 15)",
					},
					"world": map[string]any{
						"type":        "string",
						"description": "JS world: auto (default), main (page globals), isolated (bypass CSP).",
						"enum":        []string{"auto", "main", "isolated"},
					},
					"url": map[string]any{
						"type":        "string",
						"description": "URL (navigate, new_tab)",
					},
					"tab_id": map[string]any{
						"type":        "number",
						"description": "Tab ID (default: active)",
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Action reason (shown as toast)",
					},
					"summary": map[string]any{
						"type":        "boolean",
						"description": "Auto-include compact page summary (title, type, headings, primary actions, forms) after navigate/refresh/back/forward. Default: true. Set false to skip for speed.",
					},
					"correlation_id": map[string]any{
						"type":        "string",
						"description": "Link to error/investigation",
					},
					"analyze": map[string]any{
						"type":        "boolean",
						"description": "Enable perf profiling (captures perf_diff)",
					},
					"session": map[string]any{
						"type":        "string",
						"description": "Named session for multi-page annotation review (applies to draw_mode_start). Accumulates annotations across pages under a shared session name.",
					},
				},
				"required": []string{"action"},
			},
		},
	}
}
