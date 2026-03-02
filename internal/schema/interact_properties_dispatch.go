package schema

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
