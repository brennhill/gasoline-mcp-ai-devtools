// tools_configure_tutorial.go — Tutorial/examples helpers for configure tool UX.
package main

import "encoding/json"

const (
	cspRetryNavigationGuidance = "This page blocks script execution (CSP/restricted context). Use interact navigate/refresh/back/forward/new_tab/switch_tab/close_tab to move to another page."
	cspFallbackStatusPattern   = "Error: MAIN world execution FAILED. Fallback in ISOLATED is SUCCESS|ERROR"
)

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
		"status":                     "ok",
		"mode":                       mode,
		"message":                    "Quickstart snippets and context-aware guidance",
		"context":                    context,
		"issues":                     tutorialIssues(context),
		"next_steps":                 tutorialNextSteps(context),
		"snippets":                   tutorialSnippets(),
		"safe_automation_loop":       tutorialSafeAutomationLoop(),
		"csp_fallback_playbook":      tutorialCSPFallbackPlaybook(),
		"failure_recovery_playbooks": tutorialFailureRecoveryPlaybooks(),
		"best_practices": []string{
			"Start with observe to gather evidence before automating actions",
			"Use configure tutorial/examples and describe_capabilities when argument shape is unclear",
			"When debugging, capture correlation_id from interact/analyze and inspect with observe command_result",
			"Use scope + list_interactive + post-action verification to avoid wrong-target clicks",
		},
	})}
}

func (h *ToolHandler) tutorialContext() map[string]any {
	ctx := map[string]any{
		"pilot_enabled":       true,
		"pilot_state":         "assumed_enabled",
		"pilot_authoritative": false,
		"extension_connected": false,
		"tracking_enabled":    false,
		"tracked_tab_id":      0,
		"tracked_tab_url":     "",
	}
	if h == nil || h.capture == nil {
		return ctx
	}

	trackingEnabled, tabID, tabURL := h.capture.GetTrackingStatus()
	if status, ok := h.capture.GetPilotStatus().(map[string]any); ok {
		if v, ok := status["enabled"].(bool); ok {
			ctx["pilot_enabled"] = v
		}
		if v, ok := status["state"].(string); ok && v != "" {
			ctx["pilot_state"] = v
		}
		if v, ok := status["authoritative"].(bool); ok {
			ctx["pilot_authoritative"] = v
		}
	}
	ctx["extension_connected"] = h.capture.IsExtensionConnected()
	ctx["tracking_enabled"] = trackingEnabled
	ctx["tracked_tab_id"] = tabID
	ctx["tracked_tab_url"] = tabURL
	return ctx
}

func tutorialIssues(context map[string]any) []map[string]any {
	pilotEnabled, _ := context["pilot_enabled"].(bool)
	pilotState, _ := context["pilot_state"].(string)
	extensionConnected, _ := context["extension_connected"].(bool)
	trackingEnabled, _ := context["tracking_enabled"].(bool)
	tabID, _ := context["tracked_tab_id"].(int)
	tabURL, _ := context["tracked_tab_url"].(string)

	issues := make([]map[string]any, 0, 3)
	if pilotState == "explicitly_disabled" || (!pilotEnabled && pilotState == "") {
		issues = append(issues, map[string]any{
			"code":     "pilot_disabled",
			"severity": "warning",
			"message":  "AI Web Pilot is explicitly disabled; interact actions that require extension control will be skipped.",
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
			"goal":      "Click a button with scoped selector (deterministic)",
			"snippet":   `interact({what:"click", selector:"role=button[name='Submit']", scope_selector:"form[aria-label='Checkout']"})`,
			"arguments": map[string]any{"what": "click", "selector": "role=button[name='Submit']", "scope_selector": "form[aria-label='Checkout']"},
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

func tutorialSafeAutomationLoop() map[string]any {
	return map[string]any{
		"title": "Deterministic safe automation loop",
		"steps": []map[string]any{
			{"step": 1, "name": "scope_selection", "instruction": `Pick a scope first (scope_selector or frame) before targeting controls.`},
			{"step": 2, "name": "list_interactive_in_scope", "instruction": `Run interact({what:"list_interactive", ...scope...}) and capture candidate indices/labels.`},
			{"step": 3, "name": "candidate_verification", "instruction": `Verify candidate role/name/label matches intent before acting.`},
			{"step": 4, "name": "action_execution", "instruction": `Execute action using element_id/index when available; avoid broad global selectors.`},
			{"step": 5, "name": "post_action_verification", "instruction": `Verify state changed (DOM/text/url) and optionally capture screenshot evidence.`},
		},
		"bad_vs_good": []map[string]any{
			{
				"action": "submit_post",
				"bad":    `interact({what:"click", selector:"text=Post"})`,
				"good":   `interact({what:"list_interactive", scope_selector:"[role='dialog'][aria-label*='Create post']"}) -> verify candidate -> interact({what:"click", index:2}) -> observe({what:"screenshot", selector:"[data-test='feed-post']"})`,
				"reason": "Global text selectors are ambiguous on complex pages with multiple dialogs/buttons.",
			},
		},
		"scenarios": []map[string]any{
			{
				"id":          "multi_dialog",
				"name":        "Multi-dialog composer flow",
				"description": "When two dialogs are present, always scope to the active composer container before selecting a button.",
				"snippet":     `interact({what:"list_interactive", scope_selector:"[role='dialog'][aria-modal='true']"})`,
			},
			{
				"id":          "iframe",
				"name":        "Iframe-scoped interaction flow",
				"description": "When controls are inside an iframe, set frame first, then list/verify/click in that frame.",
				"snippet":     `interact({what:"list_interactive", frame:"iframe.editor", scope_selector:"form"}) -> interact({what:"click", frame:"iframe.editor", index:1})`,
			},
			{
				"id":          "csp_restricted_page",
				"name":        "CSP-restricted execute_js fallback flow",
				"description": "If execute_js reports CSP failure, switch to DOM primitives + scope + screenshot verification instead of retrying arbitrary JS.",
				"snippet":     `interact({what:"execute_js", script:"document.title", world:"main", background:true}) -> observe({what:"command_result", correlation_id:"<corr>"}) -> interact({what:"list_interactive", scope_selector:"main"}) -> interact({what:"click", index:1}) -> observe({what:"screenshot"})`,
			},
		},
	}
}

func tutorialCSPFallbackPlaybook() map[string]any {
	return map[string]any{
		"title":                   "CSP-safe automation playbook (execute_js fallback)",
		"detect_signals":          []string{"error=csp_blocked_all_worlds", "failure_cause=csp", "csp_blocked=true"},
		"fallback_status_pattern": cspFallbackStatusPattern,
		"exact_retry_guidance":    cspRetryNavigationGuidance,
		"what_is_possible": []string{
			"Pre-compiled DOM primitives (click/type/select/check/focus/list_interactive/highlight)",
			"DOM inspection and screenshot checkpoints",
			"Navigation escape actions (navigate/refresh/back/forward/new_tab/switch_tab/close_tab)",
		},
		"what_is_not_possible": []string{
			"Arbitrary page-context JS eval when CSP/Trusted Types blocks dynamic script execution",
			"Assuming MAIN world and ISOLATED world have identical capabilities on every page",
		},
		"fallback_sequence": []map[string]any{
			{"step": 1, "instruction": `Detect CSP in observe(command_result): error=csp_blocked_all_worlds or failure_cause=csp.`},
			{"step": 2, "instruction": `Stop retrying execute_js on the same page context. Switch to list_interactive + scoped DOM primitives.`},
			{"step": 3, "instruction": `Run the action with scope/frame constraints and verify the intended target before clicking.`},
			{"step": 4, "instruction": `Capture screenshot evidence after action to prove post-condition.`},
		},
		"command_examples": []map[string]any{
			{
				"goal":     "Detect CSP failure and capture retry guidance",
				"snippet":  `observe({what:"command_result", correlation_id:"<corr>"})`,
				"expected": `error=csp_blocked_all_worlds | failure_cause=csp | retry=` + cspRetryNavigationGuidance,
			},
			{
				"goal":     "Run CSP-safe fallback flow",
				"snippet":  `interact({what:"list_interactive", scope_selector:"main"}) -> interact({what:"click", index:1}) -> observe({what:"screenshot"})`,
				"expected": "Deterministic target selection and visual checkpoint without arbitrary JS eval",
			},
		},
	}
}
