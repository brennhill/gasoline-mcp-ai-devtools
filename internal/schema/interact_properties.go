package schema

func interactToolProperties() map[string]any {
	props := make(map[string]any)
	mergeProps(props, interactDispatchProperties())
	mergeProps(props, interactTargetingProperties())
	mergeProps(props, interactCoreActionProperties())
	mergeProps(props, interactFormAndWaitProperties())
	mergeProps(props, interactOutputAndBatchProperties())
	return props
}

func mergeProps(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func interactDispatchProperties() map[string]any {
	return map[string]any{
		"what": map[string]any{
			"type": "string",
			"enum": interactActions,
		},
		"action": map[string]any{
			"type":        "string",
			"description": "Deprecated alias for 'what'. Prefer 'what'.",
			"enum":        interactActions,
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
		"reason": map[string]any{
			"type":        "string",
			"description": "Action reason (shown as toast)",
		},
		"correlation_id": map[string]any{
			"type":        "string",
			"description": "Link to error/investigation",
		},
	}
}

func interactTargetingProperties() map[string]any {
	return map[string]any{
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
		"index_generation": map[string]any{
			"type":        "string",
			"description": "Generation token from list_interactive to ensure index resolves against the same element snapshot",
		},
		"nth": map[string]any{
			"type":        "number",
			"description": "Select the Nth matching element when a selector matches multiple. 0 = first visible match, 1 = second, etc. Negative values count from end (-1 = last). Prefers visible elements when available.",
		},
		"x": map[string]any{
			"type":        "number",
			"description": "X coordinate in pixels from left edge (click, hardware_click)",
		},
		"y": map[string]any{
			"type":        "number",
			"description": "Y coordinate in pixels from top edge (click, hardware_click)",
		},
		"visible_only": map[string]any{
			"type":        "boolean",
			"description": "Only return visible elements (list_interactive)",
		},
		"limit": map[string]any{
			"type":        "number",
			"description": "Max elements to return (list_interactive, default all)",
		},
		"text_contains": map[string]any{
			"type":        "string",
			"description": "Filter list_interactive elements whose label contains this substring (case-insensitive)",
		},
		"role": map[string]any{
			"type":        "string",
			"description": "Filter list_interactive elements by element type or ARIA role (e.g., 'button', 'link', 'input', 'tab')",
		},
		"exclude_nav": map[string]any{
			"type":        "boolean",
			"description": "Exclude elements inside navigation containers — nav, header, or role=navigation (list_interactive)",
		},
		"query_type": map[string]any{
			"type":        "string",
			"description": "Query operation type for interact(what='query'): exists, count, text, text_all, attributes",
			"enum":        []string{"exists", "count", "text", "text_all", "attributes"},
		},
		"attribute_names": map[string]any{
			"type":        "array",
			"description": "Attribute names to read for query_type='attributes' (e.g., ['href', 'data-id'])",
			"items":       map[string]any{"type": "string"},
		},
		"frame": map[string]any{
			"description": "Target iframe: CSS selector, 0-based index, or \"all\"",
			"type":        "string",
		},
	}
}

func interactCoreActionProperties() map[string]any {
	return map[string]any{
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
		"set_tracked": map[string]any{
			"type":        "boolean",
			"description": "Controls whether switch_tab updates the tracked tab to the newly activated tab (default: true). Set to false to switch focus without changing which tab the server targets for subsequent commands.",
		},
		"new_tab": map[string]any{
			"type":        "boolean",
			"description": "Open navigation URL in a background tab instead of replacing current tab (navigate)",
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
	}
}

func interactFormAndWaitProperties() map[string]any {
	return map[string]any{
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
		"url_contains": map[string]any{
			"type":        "string",
			"description": "Wait for URL to contain this substring (wait_for)",
		},
		"absent": map[string]any{
			"type":        "boolean",
			"description": "Wait for element to disappear (wait_for)",
		},
		"structured": map[string]any{
			"type":        "boolean",
			"description": "Return nested/hierarchical text extraction (get_text)",
		},
		"save_to": map[string]any{
			"type":        "string",
			"description": "File path to save output (run_a11y_and_export_sarif)",
		},
	}
}

func interactOutputAndBatchProperties() map[string]any {
	return map[string]any{
		"include_screenshot": map[string]any{
			"type":        "boolean",
			"description": "Capture a screenshot after the action completes and return it inline as an image content block",
		},
		"include_interactive": map[string]any{
			"type":        "boolean",
			"description": "Run list_interactive after the action and include results in the response",
		},
		"auto_dismiss": map[string]any{
			"type":        "boolean",
			"description": "After navigation completes, automatically dismiss cookie consent banners and overlays",
		},
		"wait_for_stable": map[string]any{
			"type":        "boolean",
			"description": "Wait for DOM stability (no mutations) before returning. Composable with navigate and click.",
		},
		"stability_ms": map[string]any{
			"type":        "number",
			"description": "Milliseconds of DOM quiet time required for wait_for_stable (default 500)",
		},
		"action_diff": map[string]any{
			"type":        "boolean",
			"description": "After the action completes, capture a structured mutation summary (overlays opened/closed, toasts, form errors, text changes). Composable with click, type, select, and other DOM-mutating actions.",
		},
		"steps": map[string]any{
			"type":        "array",
			"description": "Ordered list of interact actions to execute sequentially (batch)",
			"items":       map[string]any{"type": "object"},
		},
		"step_timeout_ms": map[string]any{
			"type":        "number",
			"description": "Timeout per step during batch execution (default 10000)",
		},
		"continue_on_error": map[string]any{
			"type":        "boolean",
			"description": "Continue executing remaining steps after a failure (default true)",
		},
		"stop_after_step": map[string]any{
			"type":        "number",
			"description": "Stop batch execution after this many steps",
		},
	}
}
