// configure.go — MCP schema definition for the configure tool.
package schema

import "github.com/dev-console/dev-console/internal/mcp"

// ConfigureToolSchema returns the MCP tool definition for the configure tool.
func ConfigureToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "configure",
		Description: "Session settings and utilities.\n\nKey actions: health (check server/extension status), clear (reset buffers), noise_rule (suppress recurring console noise), store/load (persist/retrieve session data), streaming (enable push notifications), recording_start/recording_stop (capture browser sessions), playback (replay recordings), log_diff (compare error states), restart (force-restart daemon when unresponsive).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{"store", "load", "noise_rule", "clear", "health", "streaming", "test_boundary_start", "test_boundary_end", "recording_start", "recording_stop", "playback", "log_diff", "telemetry", "describe_capabilities", "diff_sessions", "audit_log", "restart"},
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
				"verif_session_action": map[string]any{
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
			},
			"required": []string{"action"},
		},
	}
}
