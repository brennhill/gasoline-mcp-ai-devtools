package schema

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
