// tools_configure_tutorial.go — Tutorial/examples helpers for configure tool UX.
package main

import "encoding/json"

func (h *ToolHandler) toolConfigureTutorial(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What string `json:"what"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	mode := "tutorial"
	if params.What == "examples" {
		mode = "examples"
	}

	context := h.tutorialContext()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Tutorial", map[string]any{
		"status":     "ok",
		"mode":       mode,
		"message":    "Quickstart snippets and context-aware guidance",
		"context":    context,
		"issues":     tutorialIssues(context),
		"next_steps": tutorialNextSteps(context),
		"snippets":   tutorialSnippets(),
		"best_practices": []string{
			"Start with observe to gather evidence before automating actions",
			"Use configure tutorial/examples and describe_capabilities when argument shape is unclear",
			"When debugging, capture correlation_id from interact/analyze and inspect with observe command_result",
		},
	})}
}

func (h *ToolHandler) tutorialContext() map[string]any {
	ctx := map[string]any{
		"pilot_enabled":       false,
		"extension_connected": false,
		"tracking_enabled":    false,
		"tracked_tab_id":      0,
		"tracked_tab_url":     "",
	}
	if h == nil || h.capture == nil {
		return ctx
	}

	trackingEnabled, tabID, tabURL := h.capture.GetTrackingStatus()
	ctx["pilot_enabled"] = h.capture.IsPilotEnabled()
	ctx["extension_connected"] = h.capture.IsExtensionConnected()
	ctx["tracking_enabled"] = trackingEnabled
	ctx["tracked_tab_id"] = tabID
	ctx["tracked_tab_url"] = tabURL
	return ctx
}

func tutorialIssues(context map[string]any) []map[string]any {
	pilotEnabled, _ := context["pilot_enabled"].(bool)
	extensionConnected, _ := context["extension_connected"].(bool)
	trackingEnabled, _ := context["tracking_enabled"].(bool)
	tabID, _ := context["tracked_tab_id"].(int)
	tabURL, _ := context["tracked_tab_url"].(string)

	issues := make([]map[string]any, 0, 3)
	if !pilotEnabled {
		issues = append(issues, map[string]any{
			"code":     "pilot_disabled",
			"severity": "warning",
			"message":  "AI Web Pilot is disabled; interact actions that require extension control will be skipped.",
			"fix":      "Enable AI Web Pilot in the extension popup, then run configure tutorial again.",
			"example":  `configure({what:"doctor"})`,
		})
		return issues
	}

	if !extensionConnected {
		issues = append(issues, map[string]any{
			"code":     "extension_disconnected",
			"severity": "warning",
			"message":  "Extension is not connected; active browser automation and async command polling may fail.",
			"fix":      "Open the extension and verify status is Connected, then retry.",
			"example":  `configure({what:"doctor"})`,
		})
		return issues
	}

	if !trackingEnabled || tabID <= 0 || tabURL == "" {
		issues = append(issues, map[string]any{
			"code":     "no_tracked_tab",
			"severity": "warning",
			"message":  "No tracked tab is active; observe/interact responses may be empty or stale.",
			"fix":      "Track a tab in the extension and run a simple interact navigate call.",
			"example":  `interact({what:"navigate", url:"https://example.com"})`,
		})
	}

	return issues
}

func tutorialNextSteps(context map[string]any) []string {
	issues := tutorialIssues(context)
	if len(issues) > 0 {
		return []string{
			"Run configure doctor to verify environment status",
			"Resolve the warning shown in issues",
			"Retry with a minimal snippet from tutorial snippets",
		}
	}
	return []string{
		"Run observe errors to inspect current page issues",
		"Run interact navigate to move to a target page",
		"Run analyze page_summary for a compact state summary",
	}
}

func tutorialSnippets() []map[string]any {
	return []map[string]any{
		{
			"tool":      "observe",
			"goal":      "Read recent console errors",
			"snippet":   `observe({what:"errors", limit:20})`,
			"arguments": map[string]any{"what": "errors", "limit": 20},
		},
		{
			"tool":      "observe",
			"goal":      "Get latest command result by correlation id",
			"snippet":   `observe({what:"command_result", correlation_id:"corr_123"})`,
			"arguments": map[string]any{"what": "command_result", "correlation_id": "corr_123"},
		},
		{
			"tool":      "interact",
			"goal":      "Navigate to a URL",
			"snippet":   `interact({what:"navigate", url:"https://example.com"})`,
			"arguments": map[string]any{"what": "navigate", "url": "https://example.com"},
		},
		{
			"tool":      "interact",
			"goal":      "Click a button with semantic selector",
			"snippet":   `interact({what:"click", selector:"text=Submit"})`,
			"arguments": map[string]any{"what": "click", "selector": "text=Submit"},
		},
		{
			"tool":      "analyze",
			"goal":      "Get a high-level page summary",
			"snippet":   `analyze({what:"page_summary"})`,
			"arguments": map[string]any{"what": "page_summary"},
		},
		{
			"tool":      "generate",
			"goal":      "Generate a reproduction script from recent actions",
			"snippet":   `generate({what:"reproduction", last_n:20})`,
			"arguments": map[string]any{"what": "reproduction", "last_n": 20},
		},
		{
			"tool":      "configure",
			"goal":      "Check daemon + extension health",
			"snippet":   `configure({what:"health"})`,
			"arguments": map[string]any{"what": "health"},
		},
		{
			"tool":      "configure",
			"goal":      "Inspect tool/mode metadata at runtime",
			"snippet":   `configure({what:"describe_capabilities"})`,
			"arguments": map[string]any{"what": "describe_capabilities"},
		},
	}
}
