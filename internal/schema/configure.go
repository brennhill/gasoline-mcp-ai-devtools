// Purpose: Returns the MCP tool definition (name, description, input schema) for the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package schema

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"

// configureToolSchema returns the MCP tool definition for the configure tool.
func configureToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "configure",
		Description: "Session settings and utilities.\n\nSession: store, load, clear, telemetry, security_mode.\nDiagnostics: health, doctor, restart, audit_log, describe_capabilities, report_issue.\nRecording: event_recording_start/stop, playback, log_diff, network_recording.\nSequences: save/get/list/delete/replay_sequence.\nNoise & streaming: noise_rule, streaming, action_jitter.\nTesting: test_boundary_start/end.\nQuality: setup_quality_gates.\nHelp: tutorial, examples, diff_sessions.\n\nDiscovery: describe_capabilities — list available modes and per-mode parameters for any tool. Filter with tool and mode params, e.g. configure(what:'describe_capabilities', tool:'observe', mode:'errors') returns only the params relevant to that mode.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": configureToolProperties(),
			"required":   []string{"what"},
		},
	}
}
