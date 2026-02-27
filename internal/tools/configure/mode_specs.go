// mode_specs.go — Per-mode parameter specs for all tools.
// Docs: docs/features/describe_capabilities.md
package configure

// toolModeSpecs maps tool name → mode name → { Hint, Required, Optional }.
// Each mode lists only the params relevant to that mode, preventing
// the full param list from being dumped into every mode's output.
// Hint is a one-line description surfaced in summary mode for discovery.
var toolModeSpecs = map[string]map[string]modeParamSpec{
	// ── configure ──────────────────────────────────────────────
	"configure": {
		"store": {
			Hint:     "Persist/retrieve session key-value data",
			Optional: []string{"store_action", "namespace", "key", "data", "value"},
		},
		"load": {
			Hint: "Load stored session data by namespace",
		},
		"noise_rule": {
			Hint: "Suppress recurring console noise with pattern rules",
			Optional: []string{
				"noise_action", "rules", "rule_id", "pattern", "category", "classification",
				"message_regex", "source_regex", "url_regex", "method", "status_min", "status_max", "level", "reason",
			},
		},
		"clear": {
			Hint:     "Reset capture buffers (network, logs, actions, all)",
			Optional: []string{"buffer"},
		},
		"health": {
			Hint: "Check daemon + extension connection status",
		},
		"tutorial": {
			Hint: "Context-aware usage guidance and best practices",
		},
		"examples": {
			Hint: "Quick code snippets for common operations",
		},
		"streaming": {
			Hint:     "Enable/disable push notifications for browser events",
			Optional: []string{"streaming_action", "events", "throttle_seconds", "severity_min"},
		},
		"test_boundary_start": {
			Hint:     "Mark start of a test boundary for isolated captures",
			Required: []string{"test_id"},
			Optional: []string{"label"},
		},
		"test_boundary_end": {
			Hint:     "Mark end of a test boundary",
			Required: []string{"test_id"},
		},
		"recording_start": {
			Hint:     "Start recording browser session (actions + video)",
			Optional: []string{"name", "tab_id", "sensitive_data_enabled"},
		},
		"recording_stop": {
			Hint:     "Stop an active browser recording",
			Optional: []string{"recording_id"},
		},
		"playback": {
			Hint:     "Replay a saved recording",
			Optional: []string{"recording_id"},
		},
		"log_diff": {
			Hint:     "Compare error logs between original and replay recordings",
			Optional: []string{"original_id", "replay_id"},
		},
		"telemetry": {
			Hint:     "Set telemetry metadata mode (off/auto/full)",
			Optional: []string{"telemetry_mode"},
		},
		"describe_capabilities": {
			Hint:     "List modes and per-mode params; filter by tool and mode",
			Optional: []string{"tool", "mode"},
		},
		"diff_sessions": {
			Hint:     "Compare two session snapshots to find state differences",
			Optional: []string{"verif_session_action", "name", "compare_a", "compare_b", "url"},
		},
		"audit_log": {
			Hint:     "View tool call audit trail with timing and results",
			Optional: []string{"operation", "audit_session_id", "tool_name", "since", "limit"},
		},
		"restart": {
			Hint: "Force-restart daemon when unresponsive",
		},
		"save_sequence": {
			Hint:     "Save a named sequence of interact actions for replay",
			Optional: []string{"name", "description", "steps", "tags"},
		},
		"get_sequence": {
			Hint:     "Retrieve a saved action sequence by name",
			Optional: []string{"name"},
		},
		"list_sequences": {
			Hint: "List all saved action sequences",
		},
		"delete_sequence": {
			Hint:     "Delete a saved action sequence",
			Optional: []string{"name"},
		},
		"replay_sequence": {
			Hint:     "Replay a saved action sequence with optional overrides",
			Optional: []string{"name", "override_steps", "step_timeout_ms", "continue_on_error", "stop_after_step"},
		},
		"doctor": {
			Hint: "System diagnostics: port, state directory, log health",
		},
		"security_mode": {
			Hint:     "Toggle normal/insecure_proxy mode for debug environments",
			Optional: []string{"mode", "confirm"},
		},
	},

	// ── observe ────────────────────────────────────────────────
	"observe": {
		"errors": {
			Hint:     "Raw JavaScript console errors. summary=true returns counts by source + top messages",
			Optional: []string{"scope", "limit", "summary"},
		},
		"logs": {
			Hint:     "Console log messages with level/source filtering. summary=true returns counts by level/source",
			Optional: []string{"min_level", "level", "source", "include_internal", "include_extension_logs", "extension_limit", "limit", "scope", "summary"},
		},
		"extension_logs": {
			Hint:     "Gasoline extension internal debug logs",
			Optional: []string{"limit"},
		},
		"network_waterfall": {
			Hint:     "HTTP request/response timeline with status and timing. summary=true returns compact {url,ms,type} entries",
			Optional: []string{"url", "method", "status_min", "status_max", "limit", "summary", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction"},
		},
		"network_bodies": {
			Hint:     "HTTP response bodies with JSON path extraction. summary=true returns status groups + top URLs",
			Optional: []string{"url", "body_key", "body_path", "method", "status_min", "status_max", "limit", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction", "summary"},
		},
		"websocket_events": {
			Hint:     "WebSocket message frames (incoming/outgoing). summary=true returns direction/event counts",
			Optional: []string{"connection_id", "direction", "limit", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction", "summary"},
		},
		"websocket_status": {
			Hint:     "Active WebSocket connection states",
			Optional: []string{"limit"},
		},
		"actions": {
			Hint:     "User interaction log (clicks, inputs, navigation). summary=true returns counts by type + time range",
			Optional: []string{"limit", "after_cursor", "before_cursor", "since_cursor", "last_n", "restart_on_eviction", "summary"},
		},
		"vitals": {
			Hint:     "Core Web Vitals (LCP, CLS, INP, FCP, TTFB)",
			Optional: []string{"limit"},
		},
		"page": {
			Hint: "Current page URL, title, and tracked tab info",
		},
		"tabs": {
			Hint: "All open browser tabs with URLs",
		},
		"history": {
			Hint:     "Recent page navigation history. summary=true returns counts only",
			Optional: []string{"limit", "summary"},
		},
		"pilot": {
			Hint:     "AI Web Pilot connection status and availability",
			Optional: []string{"limit"},
		},
		"timeline": {
			Hint:     "Merged chronological view of actions, errors, network, and WebSocket events. summary=true returns counts by type",
			Optional: []string{"include", "limit", "summary"},
		},
		"error_bundles": {
			Hint:     "Pre-assembled debug context per error (error + network + actions + logs in time window). summary=true returns bundle counts + unique messages",
			Optional: []string{"window_seconds", "limit", "scope", "summary"},
		},
		"screenshot": {
			Hint:     "Capture page screenshot (full page or element)",
			Optional: []string{"format", "quality", "full_page", "selector", "wait_for_stable"},
		},
		"storage": {
			Hint: "localStorage and sessionStorage contents",
		},
		"indexeddb": {
			Hint:     "IndexedDB database/store contents",
			Optional: []string{"database", "store"},
		},
		"command_result": {
			Hint:     "Poll result of an async command by correlation_id",
			Optional: []string{"correlation_id"},
		},
		"pending_commands": {
			Hint: "List in-flight async commands awaiting results",
		},
		"failed_commands": {
			Hint: "List recently failed or expired async commands",
		},
		"saved_videos": {
			Hint: "List saved browser recording videos",
		},
		"recordings": {
			Hint:     "List captured browser session recordings",
			Optional: []string{"limit"},
		},
		"recording_actions": {
			Hint:     "Action log from a specific recording",
			Optional: []string{"recording_id", "limit"},
		},
		"playback_results": {
			Hint:     "Results from replaying a recording",
			Optional: []string{"recording_id", "limit"},
		},
		"log_diff_report": {
			Hint:     "Compare error logs between original and replay to find regressions",
			Optional: []string{"original_id", "replay_id"},
		},
		"summarized_logs": {
			Hint:     "Console messages grouped by fingerprint for pattern detection",
			Optional: []string{"min_level", "source", "limit", "min_group_size"},
		},
	},

	// ── interact ───────────────────────────────────────────────
	"interact": {
		"click": {
			Hint:     "Click an element by selector, element_id, or coordinates",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame", "reason", "correlation_id", "timeout_ms", "x", "y", "analyze"},
		},
		"type": {
			Hint:     "Type text into an input or textarea",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame", "text", "clear"},
		},
		"select": {
			Hint:     "Choose an option in a <select> dropdown",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame", "value"},
		},
		"check": {
			Hint:     "Toggle a checkbox or radio button",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame", "checked"},
		},
		"get_text": {
			Hint:     "Read text content of an element",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame"},
		},
		"get_value": {
			Hint:     "Read value of an input element",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame"},
		},
		"focus": {
			Hint:     "Focus an element",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame"},
		},
		"scroll_to": {
			Hint:     "Scroll an element into view",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame"},
		},
		"get_attribute": {
			Hint:     "Read an HTML attribute from an element",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame", "name"},
		},
		"set_attribute": {
			Hint:     "Set an HTML attribute on an element",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame", "name", "value"},
		},
		"paste": {
			Hint:     "Paste text into an element via clipboard",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame", "text"},
		},
		"highlight": {
			Hint:     "Visually highlight an element with a colored overlay",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame", "duration_ms"},
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
			Optional: []string{"url", "include_content", "new_tab", "analyze"},
		},
		"navigate_and_wait_for": {
			Hint:     "Navigate to a URL and wait for a selector to appear",
			Optional: []string{"url", "wait_for", "include_content"},
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
		"hover": {
			Hint:     "Trigger hover state on an element for tooltip discovery",
			Optional: []string{"selector", "element_id", "index", "scope_selector", "frame"},
		},
		"activate_tab": {
			Hint: "Bring the tracked tab to the foreground",
		},
	},

	// ── analyze ────────────────────────────────────────────────
	"analyze": {
		"dom": {
			Hint:     "Query DOM structure and element properties",
			Optional: []string{"selector", "frame", "tab_id"},
		},
		"performance": {
			Hint: "Page load performance metrics and bottleneck analysis",
		},
		"accessibility": {
			Hint:     "WCAG/axe accessibility audit with violation details. summary=true returns counts + top issues",
			Optional: []string{"selector", "scope", "tags", "force_refresh", "frame", "summary"},
		},
		"error_clusters": {
			Hint: "Group errors by pattern to identify systemic issues",
		},
		"history": {
			Hint: "Analyze navigation history patterns",
		},
		"security_audit": {
			Hint:     "Check for credential leaks, CSP, cookie, and header risks. summary=true returns counts + top issues",
			Optional: []string{"checks", "severity_min", "summary"},
		},
		"third_party_audit": {
			Hint:     "Audit third-party script origins and data exposure. summary=true returns counts + top origins",
			Optional: []string{"first_party_origins", "include_static", "custom_lists", "summary"},
		},
		"link_health": {
			Hint:     "Check all page links for broken URLs (404s, timeouts)",
			Optional: []string{"domain", "max_workers", "timeout_ms"},
		},
		"link_validation": {
			Hint:     "Validate specific URLs for reachability",
			Optional: []string{"urls"},
		},
		"page_summary": {
			Hint:     "AI-generated summary of page content and structure",
			Optional: []string{"world", "tab_id", "timeout_ms"},
		},
		"annotations": {
			Hint:     "List annotations from a draw/annotation session",
			Optional: []string{"annot_session", "wait", "timeout_ms"},
		},
		"annotation_detail": {
			Hint:     "Full DOM/style details for a specific annotation",
			Optional: []string{"correlation_id"},
		},
		"api_validation": {
			Hint:     "Validate API responses against contract/schema",
			Optional: []string{"operation", "ignore_endpoints"},
		},
		"draw_history": {
			Hint: "List saved annotation/draw sessions",
		},
		"draw_session": {
			Hint:     "Load all annotations from a saved draw session file",
			Optional: []string{"file"},
		},
		"computed_styles": {
			Hint:     "CSS computed styles for an element",
			Optional: []string{"selector", "frame"},
		},
		"forms": {
			Hint:     "Analyze form structure, fields, and validation state",
			Optional: []string{"selector", "frame"},
		},
		"form_validation": {
			Hint:     "Check form validation rules and constraint violations. summary=true returns counts only",
			Optional: []string{"summary"},
		},
		"visual_baseline": {
			Hint:     "Capture a baseline screenshot for visual regression",
			Optional: []string{"name"},
		},
		"visual_diff": {
			Hint:     "Compare current page against a visual baseline",
			Optional: []string{"baseline", "name", "threshold"},
		},
		"visual_baselines": {
			Hint: "List all stored visual regression baselines",
		},
		"navigation": {
			Hint:     "Discover navigable links grouped by page region (nav, header, footer, aside)",
			Optional: []string{"tab_id"},
		},
	},

	// ── generate ───────────────────────────────────────────────
	"generate": {
		"reproduction": {
			Hint:     "Generate Playwright reproduction script from captured actions/errors",
			Optional: []string{"error_message", "last_n", "base_url", "include_screenshots", "generate_fixtures", "visual_assertions"},
		},
		"test": {
			Hint:     "Generate Playwright test from captured actions",
			Optional: []string{"test_name", "assert_network", "assert_no_errors", "assert_response_shape"},
		},
		"pr_summary": {
			Hint: "Generate PR summary from captured session activity",
		},
		"har": {
			Hint:     "Export captured network traffic as HAR file",
			Optional: []string{"url", "method", "status_min", "status_max"},
		},
		"csp": {
			Hint:     "Generate Content-Security-Policy header from observed resources",
			Optional: []string{"mode", "include_report_uri", "exclude_origins"},
		},
		"sri": {
			Hint:     "Generate Subresource Integrity hashes for scripts/styles",
			Optional: []string{"resource_types", "origins"},
		},
		"sarif": {
			Hint:     "Export errors and violations as SARIF for IDE/CI integration",
			Optional: []string{"scope", "include_passes"},
		},
		"visual_test": {
			Hint:     "Generate visual regression test from annotations",
			Optional: []string{"test_name", "annot_session"},
		},
		"annotation_report": {
			Hint:     "Generate markdown report from annotation session",
			Optional: []string{"annot_session"},
		},
		"annotation_issues": {
			Hint:     "Generate structured issue list from annotations",
			Optional: []string{"annot_session"},
		},
		"test_from_context": {
			Hint:     "Generate test from error/interaction/regression context",
			Optional: []string{"context", "error_id", "include_mocks", "output_format"},
		},
		"test_heal": {
			Hint:     "Analyze or repair broken test selectors",
			Optional: []string{"action", "test_file", "test_dir", "broken_selectors", "auto_apply"},
		},
		"test_classify": {
			Hint:     "Classify test failures by root cause",
			Optional: []string{"action", "failure", "failures"},
		},
	},
}
