// interact.go — MCP schema definition for the interact tool.
package schema

import "github.com/dev-console/dev-console/internal/mcp"

// InteractToolSchema returns the MCP tool definition for the interact tool.
func InteractToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "interact",
		Description: "Browser actions. Requires AI Web Pilot.\n\nSynchronous Mode (Default): Tools now block until the extension returns a result (up to 15s). Set background:true to return immediately with a correlation_id.\n\nSelectors: CSS or semantic (text=Submit, role=button, placeholder=Email, label=Name, aria-label=Close). subtitle param composable with any action. analyze=true captures perf_diff. navigate/refresh auto-include perf_diff.\n\nDraw Mode: draw_mode_start activates annotation overlay — user draws rectangles and types feedback, presses ESC to finish. Use analyze({what:'annotations'}) to retrieve results.\n\nCompatibility: action='screenshot' is a backward-compatible alias for observe({what:'screenshot'}).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"what": map[string]any{
					"type": "string",
					"enum": []string{
						"highlight", "subtitle", "save_state", "state_save", "load_state", "state_load", "list_states", "state_list", "delete_state", "state_delete",
						"set_storage", "delete_storage", "clear_storage", "set_cookie", "delete_cookie",
						"execute_js", "navigate", "refresh", "back", "forward", "new_tab", "switch_tab", "close_tab", "screenshot",
						"click", "type", "select", "check",
						"get_text", "get_value", "get_attribute",
						"set_attribute", "focus", "scroll_to", "wait_for", "key_press", "paste",
						"open_composer", "submit_active_composer", "confirm_top_dialog", "dismiss_top_overlay",
						"list_interactive",
						"get_readable", "get_markdown",
						"navigate_and_wait_for", "fill_form_and_submit", "fill_form", "run_a11y_and_export_sarif",
						"record_start", "record_stop",
						"upload", "draw_mode_start",
					},
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Deprecated alias for 'what'",
					"enum": []string{
						"highlight", "subtitle", "save_state", "state_save", "load_state", "state_load", "list_states", "state_list", "delete_state", "state_delete",
						"set_storage", "delete_storage", "clear_storage", "set_cookie", "delete_cookie",
						"execute_js", "navigate", "refresh", "back", "forward", "new_tab", "switch_tab", "close_tab", "screenshot",
						"click", "type", "select", "check",
						"get_text", "get_value", "get_attribute",
						"set_attribute", "focus", "scroll_to", "wait_for", "key_press", "paste",
						"open_composer", "submit_active_composer", "confirm_top_dialog", "dismiss_top_overlay",
						"list_interactive",
						"get_readable", "get_markdown",
						"navigate_and_wait_for", "fill_form_and_submit", "fill_form", "run_a11y_and_export_sarif",
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
				"scope_selector": map[string]any{
					"type":        "string",
					"description": "Optional container selector to constrain DOM actions to a specific region",
				},
				"element_id": map[string]any{
					"type":        "string",
					"description": "Stable element handle from list_interactive (preferred for deterministic follow-up actions)",
				},
				"index": map[string]any{
					"type":        "number",
					"description": "Element index from list_interactive results (legacy alternative to selector/element_id)",
				},
				"visible_only": map[string]any{
					"type":        "boolean",
					"description": "Only return visible elements (list_interactive)",
				},
				"frame": map[string]any{
					"description": "Target iframe: CSS selector, 0-based index, or \"all\"",
					"type":        "string",
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
				"storage_type": map[string]any{
					"type":        "string",
					"description": "Storage target for state mutation actions",
					"enum":        []string{"localStorage", "sessionStorage"},
				},
				"key": map[string]any{
					"type":        "string",
					"description": "Storage key for set_storage/delete_storage",
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
					"description": "Attribute, recording, or cookie name",
				},
				"domain": map[string]any{
					"type":        "string",
					"description": "Cookie domain (set_cookie/delete_cookie)",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Cookie path (set_cookie/delete_cookie, default /)",
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
				"include_content": map[string]any{
					"type":        "boolean",
					"description": "Return page content with navigate response (url, title, text_content, vitals)",
				},
				"tab_id": map[string]any{
					"type":        "number",
					"description": "Tab ID (default: active)",
				},
				"tab_index": map[string]any{
					"type":        "number",
					"description": "Tab index in current window ordering (switch_tab)",
				},
				"new_tab": map[string]any{
					"type":        "boolean",
					"description": "Open navigation URL in a background tab instead of replacing current tab (navigate)",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Action reason (shown as toast)",
				},
				"correlation_id": map[string]any{
					"type":        "string",
					"description": "Link to error/investigation",
				},
				"analyze": map[string]any{
					"type":        "boolean",
					"description": "Enable perf profiling (captures perf_diff)",
				},
				"evidence": map[string]any{
					"type":        "string",
					"description": "Optional visual evidence capture mode: off (default), on_mutation, always.",
					"enum":        []string{"off", "on_mutation", "always"},
				},
				"observe_mutations": map[string]any{
					"type":        "boolean",
					"description": "Track element-level DOM mutations during action execution",
				},
				"annot_session": map[string]any{
					"type":        "string",
					"description": "Named session for multi-page annotation review (applies to draw_mode_start). Accumulates annotations across pages under a shared session name.",
				},
				"file_path": map[string]any{
					"type":        "string",
					"description": "Absolute file path for upload action",
				},
				"api_endpoint": map[string]any{
					"type":        "string",
					"description": "API endpoint for direct upload mode",
				},
				"submit": map[string]any{
					"type":        "boolean",
					"description": "Submit form after upload",
				},
				"escalation_timeout_ms": map[string]any{
					"type":        "number",
					"description": "Upload escalation timeout in ms",
				},
				"fields": map[string]any{
					"type":        "array",
					"description": "Form fields to fill (fill_form, fill_form_and_submit)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"selector": map[string]any{"type": "string", "description": "CSS selector for the field"},
							"value":    map[string]any{"type": "string", "description": "Value to type into the field"},
							"index":    map[string]any{"type": "number", "description": "Element index from list_interactive (alternative to selector)"},
						},
						"required": []string{"value"},
					},
				},
				"submit_selector": map[string]any{
					"type":        "string",
					"description": "Submit button selector (fill_form_and_submit)",
				},
				"submit_index": map[string]any{
					"type":        "number",
					"description": "Submit button index from list_interactive (fill_form_and_submit)",
				},
				"wait_for": map[string]any{
					"type":        "string",
					"description": "CSS selector to wait for after navigation (navigate_and_wait_for)",
				},
				"save_to": map[string]any{
					"type":        "string",
					"description": "File path to save output (run_a11y_and_export_sarif)",
				},
			},
			"required": []string{"what"},
		},
	}
}
