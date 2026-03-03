// mode_specs_interact.go — interact tool per-mode parameter specs.
// Docs: docs/features/describe_capabilities.md
package configure

var interactModeSpecs = map[string]modeParamSpec{
	"click": {
		Hint:     "Click an element by selector, element_id, or coordinates",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "reason", "correlation_id", "timeout_ms", "x", "y", "analyze", "wait_for_stable", "stability_ms"},
	},
	"type": {
		Hint:     "Type text into an input or textarea",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "text", "clear"},
	},
	"select": {
		Hint:     "Choose an option in a <select> dropdown",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "value"},
	},
	"check": {
		Hint:     "Toggle a checkbox or radio button",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "checked"},
	},
	"get_text": {
		Hint:     "Read text content of an element",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "structured"},
	},
	"get_value": {
		Hint:     "Read value of an input element",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"},
	},
	"focus": {
		Hint:     "Focus an element",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"},
	},
	"scroll_to": {
		Hint:     "Scroll an element into view, or scroll container directionally (direction='top'|'bottom'|'up'|'down')",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "direction", "value"},
	},
	"get_attribute": {
		Hint:     "Read an HTML attribute from an element",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "name"},
	},
	"set_attribute": {
		Hint:     "Set an HTML attribute on an element",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "name", "value"},
	},
	"paste": {
		Hint:     "Paste text into an element via clipboard",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "text"},
	},
	"highlight": {
		Hint:     "Visually highlight an element with a colored overlay",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "duration_ms"},
	},
	"wait_for": {
		Hint:     "Wait until a selector appears in the DOM",
		Optional: []string{"selector", "timeout_ms", "frame"},
	},
	"key_press": {
		Hint:     "Send keyboard keys (Enter, Tab, Escape, shortcuts)",
		Optional: []string{"text"},
	},
	"subtitle": {
		Hint:     "Display a status subtitle in the extension UI",
		Optional: []string{"text"},
	},
	"navigate": {
		Hint:     "Navigate to a URL",
		Optional: []string{"url", "include_content", "new_tab", "analyze", "auto_dismiss", "wait_for_stable", "stability_ms"},
	},
	"navigate_and_wait_for": {
		Hint:     "Navigate to a URL and wait for a selector to appear",
		Optional: []string{"url", "wait_for", "include_content"},
	},
	"navigate_and_document": {
		Hint:     "Click to navigate, optionally wait for URL change/stability, then return page context",
		Optional: []string{"selector", "element_id", "index", "index_generation", "nth", "scope_selector", "scope_rect", "frame", "tab_id", "reason", "timeout_ms", "wait_for_url_change", "wait_for_stable", "stability_ms", "include_screenshot", "include_interactive"},
	},
	"refresh": {
		Hint:     "Reload the current page",
		Optional: []string{"analyze"},
	},
	"back": {
		Hint: "Browser back button",
	},
	"forward": {
		Hint: "Browser forward button",
	},
	"new_tab": {
		Hint:     "Open a new browser tab",
		Optional: []string{"url"},
	},
	"switch_tab": {
		Hint:     "Switch to a different browser tab",
		Optional: []string{"tab_id", "tab_index"},
	},
	"close_tab": {
		Hint:     "Close a browser tab",
		Optional: []string{"tab_id"},
	},
	"screenshot": {
		Hint: "Capture page screenshot (alias for observe/screenshot)",
	},
	"execute_js": {
		Hint:     "Run JavaScript in the page context",
		Optional: []string{"script", "world", "timeout_ms"},
	},
	"list_interactive": {
		Hint:     "List all clickable/typeable elements on the page. Use limit to cap results",
		Optional: []string{"visible_only", "frame", "scope_selector", "limit"},
	},
	"get_readable": {
		Hint:     "Extract readable text content from the page",
		Optional: []string{"frame"},
	},
	"get_markdown": {
		Hint:     "Extract page content as markdown",
		Optional: []string{"frame"},
	},
	"fill_form": {
		Hint:     "Fill multiple form fields at once",
		Optional: []string{"fields", "scope_selector", "frame"},
	},
	"fill_form_and_submit": {
		Hint:     "Fill form fields and click the submit button",
		Optional: []string{"fields", "submit_selector", "submit_index", "scope_selector", "frame"},
	},
	"save_state": {
		Hint:     "Snapshot cookies/storage/URL for later restore",
		Optional: []string{"snapshot_name", "storage_type", "include_url"},
	},
	"state_save": {
		Hint:     "Snapshot cookies/storage/URL (alias for save_state)",
		Optional: []string{"snapshot_name", "storage_type", "include_url"},
	},
	"load_state": {
		Hint:     "Restore a previously saved state snapshot",
		Optional: []string{"snapshot_name", "storage_type"},
	},
	"state_load": {
		Hint:     "Restore a saved state snapshot (alias for load_state)",
		Optional: []string{"snapshot_name", "storage_type"},
	},
	"list_states": {
		Hint: "List all saved state snapshots",
	},
	"state_list": {
		Hint: "List saved state snapshots (alias for list_states)",
	},
	"delete_state": {
		Hint:     "Delete a saved state snapshot",
		Optional: []string{"snapshot_name"},
	},
	"state_delete": {
		Hint:     "Delete a state snapshot (alias for delete_state)",
		Optional: []string{"snapshot_name"},
	},
	"set_storage": {
		Hint:     "Set a localStorage or sessionStorage key",
		Optional: []string{"storage_type", "key", "value"},
	},
	"delete_storage": {
		Hint:     "Delete a storage key",
		Optional: []string{"storage_type", "key"},
	},
	"clear_storage": {
		Hint:     "Clear all keys from a storage type",
		Optional: []string{"storage_type"},
	},
	"set_cookie": {
		Hint:     "Set a browser cookie",
		Optional: []string{"name", "value", "domain", "path"},
	},
	"delete_cookie": {
		Hint:     "Delete a browser cookie",
		Optional: []string{"name", "domain", "path"},
	},
	"record_start": {
		Hint:     "Start recording browser session with video capture",
		Optional: []string{"name", "audio", "fps"},
	},
	"record_stop": {
		Hint:     "Stop recording and save the session",
		Optional: []string{"name"},
	},
	"upload": {
		Hint:     "Upload a file to a file input or API endpoint",
		Optional: []string{"file_path", "api_endpoint", "submit", "escalation_timeout_ms"},
	},
	"draw_mode_start": {
		Hint:     "Activate annotation overlay for drawing rectangles and adding feedback",
		Optional: []string{"annot_session", "timeout_ms"},
	},
	"hardware_click": {
		Hint:     "CDP-level click at x/y coordinates for isTrusted events",
		Optional: []string{"x", "y"},
	},
	"run_a11y_and_export_sarif": {
		Hint:     "Run accessibility audit and export results as SARIF",
		Optional: []string{"save_to", "scope_selector", "frame"},
	},
	"open_composer": {
		Hint: "Open the Claude composer interface",
	},
	"submit_active_composer": {
		Hint: "Submit the active Claude composer message",
	},
	"confirm_top_dialog": {
		Hint: "Accept/confirm the top-most dialog or modal",
	},
	"dismiss_top_overlay": {
		Hint: "Dismiss/close the top-most overlay or popover",
	},
	"auto_dismiss_overlays": {
		Hint:     "Auto-dismiss cookie consent banners and overlays using known framework selectors",
		Optional: []string{"timeout_ms"},
	},
	"wait_for_stable": {
		Hint:     "Wait for DOM stability (no mutations for stability_ms). Returns stable/timed_out status",
		Optional: []string{"stability_ms", "timeout_ms"},
	},
	"hover": {
		Hint:     "Trigger hover state on an element for tooltip discovery",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"},
	},
	"activate_tab": {
		Hint: "Bring the tracked tab to the foreground",
	},
	"explore_page": {
		Hint:     "Composite page exploration: screenshot, interactive elements, readable text, navigation links, and metadata in one call",
		Optional: []string{"url", "visible_only", "limit"},
	},
	"batch": {
		Hint:     "Execute a sequence of interact actions in one call",
		Optional: []string{"steps", "step_timeout_ms", "continue_on_error", "stop_after_step"},
	},
	"clipboard_read": {
		Hint: "Read current clipboard text content",
	},
	"clipboard_write": {
		Hint:     "Write text to the clipboard",
		Optional: []string{"text"},
	},
	"query": {
		Hint:     "Query DOM elements: check existence, count, read text or attributes without screenshots",
		Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "query_type", "attribute_names"},
	},
}
