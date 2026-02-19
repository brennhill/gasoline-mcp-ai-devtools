// schema.go — MCP schema definition for the observe tool.
package observe

import "github.com/dev-console/dev-console/internal/mcp"

// Schema returns the MCP tool definition for the observe tool.
func Schema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "observe",
		Description: "Read captured browser state from extension buffers.\n\nnetwork_bodies captures fetch() only; use network_waterfall for all requests. extension_logs = internal debug logs (use logs for console). error_bundles = pre-assembled debug context per error. Use body_key/body_path to extract JSON subtrees from network_bodies.\n\nPagination: pass after_cursor/before_cursor/since_cursor from response metadata. restart_on_eviction=true if cursor expired.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"what": map[string]any{
					"type": "string",
					"enum": []string{"errors", "logs", "extension_logs", "network_waterfall", "network_bodies", "websocket_events", "websocket_status", "actions", "vitals", "page", "tabs", "pilot", "timeline", "error_bundles", "screenshot", "storage", "command_result", "pending_commands", "failed_commands", "saved_videos", "recordings", "recording_actions", "playback_results", "log_diff_report"},
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
				"recording_id": map[string]any{
					"type":        "string",
					"description": "Recording ID (recording_actions, playback_results)",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "Filter scope: current_page (default) filters by tracked tab, all returns everything (errors, logs)",
					"enum":        []string{"current_page", "all"},
				},
				"window_seconds": map[string]any{
					"type":        "number",
					"description": "error_bundles lookback seconds (default 3, max 10)",
				},
				"original_id": map[string]any{
					"type":        "string",
					"description": "Original recording ID (log_diff_report)",
				},
				"replay_id": map[string]any{
					"type":        "string",
					"description": "Replay recording ID (log_diff_report)",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Screenshot format (screenshot)",
					"enum":        []string{"png", "jpeg"},
				},
				"quality": map[string]any{
					"type":        "number",
					"description": "Screenshot JPEG quality 1-100 (screenshot)",
				},
				"full_page": map[string]any{
					"type":        "boolean",
					"description": "Capture full scrollable page (screenshot)",
				},
				"selector": map[string]any{
					"type":        "string",
					"description": "Capture specific element by CSS selector (screenshot)",
				},
				"wait_for_stable": map[string]any{
					"type":        "boolean",
					"description": "Wait for layout to stabilize before capture (screenshot)",
				},
			},
			"required": []string{"what"},
		},
	}
}
