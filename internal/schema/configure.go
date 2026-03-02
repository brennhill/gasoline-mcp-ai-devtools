// Purpose: Returns the MCP tool definition (name, description, input schema) for the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package schema

import "github.com/dev-console/dev-console/internal/mcp"

// ConfigureToolSchema returns the MCP tool definition for the configure tool.
func ConfigureToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "configure",
		Description: "Session settings and utilities.\n\nKey actions: health (check server/extension status), clear (reset buffers), noise_rule (suppress recurring console noise), store/load (persist/retrieve session data), tutorial/examples (quick snippets + context-aware guidance), streaming (enable push notifications), recording_start/recording_stop (capture browser sessions), playback (replay recordings), log_diff (compare error states), security_mode (opt-in altered-environment debug mode), restart (force-restart daemon when unresponsive).\n\nMacro sequences: save_sequence/replay_sequence/get_sequence/list_sequences/delete_sequence — save named interact action sequences and replay them in a single call.\n\nDiscovery: describe_capabilities — list available modes and per-mode parameters for any tool. Filter with tool and mode params, e.g. configure(what:'describe_capabilities', tool:'observe', mode:'errors') returns only the params relevant to that mode.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": configureToolProperties(),
			"required":   []string{"what"},
		},
	}
}
