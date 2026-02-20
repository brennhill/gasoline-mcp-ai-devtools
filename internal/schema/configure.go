// configure.go — MCP schema definition for the configure tool.
package schema

import "github.com/dev-console/dev-console/internal/mcp"

// ConfigureToolSchema returns the MCP tool definition for the configure tool.
func ConfigureToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "configure",
		Description: "Session settings and utilities.\n\nKey actions: health (check server/extension status), clear (reset buffers), noise_rule (suppress recurring console noise), store/load (persist/retrieve session data), tutorial/examples (quick snippets + context-aware guidance), streaming (enable push notifications), recording_start/recording_stop (capture browser sessions), playback (replay recordings), log_diff (compare error states), restart (force-restart daemon when unresponsive).\n\nMacro sequences: save_sequence/replay_sequence/get_sequence/list_sequences/delete_sequence — save named interact action sequences and replay them in a single call.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"what": map[string]any{
					"type": "string",
					"enum": []string{"store", "load", "noise_rule", "clear", "health", "tutorial", "examples", "streaming", "test_boundary_start", "test_boundary_end", "recording_start", "recording_stop", "playback", "log_diff", "telemetry", "describe_capabilities", "diff_sessions", "audit_log", "restart", "save_sequence", "get_sequence", "list_sequences", "delete_sequence", "replay_sequence", "doctor"},
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Deprecated alias for 'what'",
					"enum":        []string{"store", "load", "noise_rule", "clear", "health", "tutorial", "examples", "streaming", "test_boundary_start", "test_boundary_end", "recording_start", "recording_stop", "playback", "log_diff", "telemetry", "describe_capabilities", "diff_sessions", "audit_log", "restart", "save_sequence", "get_sequence", "list_sequences", "delete_sequence", "replay_sequence", "doctor"},
				},
				"telemetry_mode": map[string]any{
					"type":        "string",
					"description": "Telemetry metadata mode: off, auto, full. configure(action='telemetry') sets global default. Any tools/call may override per request with telemetry_mode.",
					"enum":        []string{"off", "auto", "full"},
				},
				"store_action": map[string]any{
					"type":        "string",
					"description": "Store operation (default: list)",
					"enum":        []string{"save", "load", "list", "delete", "stats"},
					"default":     "list",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Store grouping (default: session)",
					"default":     "session",
				},
				"key": map[string]any{
					"type":        "string",
					"description": "Storage key",
				},
				"data": map[string]any{
					"type":        "object",
					"description": "JSON data to persist",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "Flat value alias for save action; treated as data when provided",
				},
				"noise_action": map[string]any{
					"type":        "string",
					"description": "Noise operation (default: list)",
					"enum":        []string{"add", "remove", "list", "reset", "auto_detect"},
					"default":     "list",
				},
				"rules": map[string]any{
					"type":        "array",
					"description": "Noise rules to add",
					"items":       map[string]any{"type": "object"},
				},
				"classification": map[string]any{
					"type":        "string",
					"description": "Single-rule flattening helper for noise_action=add",
				},
				"message_regex": map[string]any{
					"type":        "string",
					"description": "Single-rule flattening helper for noise_action=add",
				},
				"source_regex": map[string]any{
					"type":        "string",
					"description": "Single-rule flattening helper for noise_action=add",
				},
				"url_regex": map[string]any{
					"type":        "string",
					"description": "Single-rule flattening helper for noise_action=add",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "Single-rule flattening helper for noise_action=add",
				},
				"status_min": map[string]any{
					"type":        "integer",
					"description": "Single-rule flattening helper for noise_action=add",
				},
				"status_max": map[string]any{
					"type":        "integer",
					"description": "Single-rule flattening helper for noise_action=add",
				},
				"level": map[string]any{
					"type":        "string",
					"description": "Single-rule flattening helper for noise_action=add",
				},
				"rule_id": map[string]any{
					"type":        "string",
					"description": "Rule ID to remove",
				},
				"pattern": map[string]any{
					"type":        "string",
					"description": "Regex pattern (single-rule flattening helper for noise_action=add)",
				},
				"category": map[string]any{
					"type":        "string",
					"description": "Noise category (default: console for flattened add)",
					"enum":        []string{"console", "network", "websocket"},
					"default":     "console",
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
				"verif_session_action": map[string]any{
					"type": "string",
					"enum": []string{"capture", "compare", "list", "delete"},
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Snapshot name, or sequence name for save_sequence/get_sequence/delete_sequence/replay_sequence",
				},
				"compare_a": map[string]any{
					"type":        "string",
					"description": "First snapshot to compare",
				},
				"compare_b": map[string]any{
					"type":        "string",
					"description": "Second snapshot to compare",
				},
				"url": map[string]any{
					"type":        "string",
					"description": "URL filter for snapshot capture (diff_sessions)",
				},
				"recording_id": map[string]any{
					"type":        "string",
					"description": "Recording ID (recording_stop, playback)",
				},
				"sensitive_data_enabled": map[string]any{
					"type":        "boolean",
					"description": "Include sensitive data in recording capture",
				},
				"operation": map[string]any{
					"type":        "string",
					"description": "Action-specific operation key",
					"enum":        []string{"analyze", "report", "clear"},
				},
				"audit_session_id": map[string]any{
					"type":        "string",
					"description": "Filter by audit session ID",
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
				"original_id": map[string]any{
					"type":        "string",
					"description": "Original recording ID (log_diff)",
				},
				"replay_id": map[string]any{
					"type":        "string",
					"description": "Replay recording ID (log_diff)",
				},
				"steps": map[string]any{
					"type":        "array",
					"description": "Ordered list of interact action objects (save_sequence, replay_sequence override)",
					"items":       map[string]any{"type": "object"},
				},
				"tags": map[string]any{
					"type":        "array",
					"description": "Labels for sequence categorization",
					"items":       map[string]any{"type": "string"},
				},
				"override_steps": map[string]any{
					"type":        "array",
					"description": "Sparse array of step overrides for replay (null = use saved)",
					"items":       map[string]any{},
				},
				"step_timeout_ms": map[string]any{
					"type":        "number",
					"description": "Timeout per step during replay (default 10000)",
				},
				"continue_on_error": map[string]any{
					"type":        "boolean",
					"description": "Continue replay if a step fails (default true)",
				},
				"stop_after_step": map[string]any{
					"type":        "number",
					"description": "Stop replay after executing this many steps",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Human-readable description for saved sequence",
				},
			},
			"required": []string{"what"},
		},
	}
}
