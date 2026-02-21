// tools_configure_help.go — Compact configure help/cheatsheet output for LLM routing.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type helpModeSpec struct {
	Name    string
	Summary string
	Params  []string
}

type helpToolSpec struct {
	Name         string
	Summary      string
	CommonParams []string
	Modes        []helpModeSpec
}

var helpSpecs = map[string]helpToolSpec{
	"analyze": {
		Name:         "analyze",
		Summary:      "Active analysis",
		CommonParams: []string{"background", "sync", "timeout_ms", "tab_id"},
		Modes: []helpModeSpec{
			{Name: "dom", Summary: "Query DOM elements", Params: []string{"selector", "frame"}},
			{Name: "page_summary", Summary: "Compact page state snapshot", Params: []string{"tab_id", "world"}},
			{Name: "accessibility", Summary: "WCAG accessibility scan", Params: []string{"scope", "selector", "tags", "force_refresh"}},
			{Name: "forms", Summary: "Discover form fields", Params: []string{"selector"}},
			{Name: "computed_styles", Summary: "Read computed CSS values", Params: []string{"selector", "property", "frame"}},
			{Name: "link_validation", Summary: "Validate explicit URLs", Params: []string{"urls"}},
			{Name: "api_validation", Summary: "Validate API calls in capture", Params: []string{"operation", "ignore_endpoints"}},
			{Name: "security_audit", Summary: "Run security checks", Params: []string{"checks", "severity_min"}},
		},
	},
	"observe": {
		Name:         "observe",
		Summary:      "Read captured telemetry buffers",
		CommonParams: []string{"limit", "scope"},
		Modes: []helpModeSpec{
			{Name: "errors", Summary: "Console/runtime errors", Params: []string{"limit", "url", "scope"}},
			{Name: "logs", Summary: "Console logs with filtering/pagination", Params: []string{"level", "min_level", "source", "url", "limit"}},
			{Name: "network_waterfall", Summary: "All network timing entries", Params: []string{"url", "method", "status_min", "status_max", "limit"}},
			{Name: "network_bodies", Summary: "Captured request/response payloads", Params: []string{"url", "method", "status_min", "status_max", "body_key", "body_path", "limit"}},
			{Name: "actions", Summary: "Captured human/AI actions", Params: []string{"url", "limit"}},
			{Name: "error_bundles", Summary: "Error + related context bundle", Params: []string{"limit", "window_seconds", "url"}},
			{Name: "command_result", Summary: "Poll async command lifecycle", Params: []string{"correlation_id"}},
		},
	},
	"interact": {
		Name:         "interact",
		Summary:      "Drive browser actions",
		CommonParams: []string{"tab_id", "background", "sync", "timeout_ms"},
		Modes: []helpModeSpec{
			{Name: "navigate", Summary: "Navigate active tab", Params: []string{"url", "tab_id"}},
			{Name: "click", Summary: "Click element by selector/index", Params: []string{"selector", "index", "frame", "world"}},
			{Name: "type", Summary: "Type text into element", Params: []string{"selector", "index", "text", "clear", "frame", "world"}},
			{Name: "execute_js", Summary: "Execute JavaScript in page", Params: []string{"script", "world", "timeout_ms"}},
			{Name: "list_interactive", Summary: "List interactable elements", Params: []string{"visible_only", "frame"}},
			{Name: "navigate_and_wait_for", Summary: "Navigate then wait for selector", Params: []string{"url", "wait_for", "timeout_ms"}},
		},
	},
	"generate": {
		Name:         "generate",
		Summary:      "Generate artifacts from captured data",
		CommonParams: []string{"save_to"},
		Modes: []helpModeSpec{
			{Name: "reproduction", Summary: "Generate reproduction script", Params: []string{"error_message", "last_n", "base_url", "include_screenshots", "generate_fixtures", "output_format"}},
			{Name: "test", Summary: "Generate Playwright test", Params: []string{"test_name", "last_n", "base_url", "assert_network", "assert_no_errors"}},
			{Name: "sarif", Summary: "Export SARIF report", Params: []string{"scope", "include_passes", "save_to"}},
			{Name: "csp", Summary: "Generate CSP policy", Params: []string{"mode", "include_report_uri", "exclude_origins"}},
			{Name: "har", Summary: "Export HAR from capture", Params: []string{"url", "method", "status_min", "status_max"}},
			{Name: "test_from_context", Summary: "Generate tests from context/error", Params: []string{"context", "error_id", "include_mocks", "output_format"}},
		},
	},
	"configure": {
		Name:         "configure",
		Summary:      "Session controls and diagnostics",
		CommonParams: []string{"what"},
		Modes: []helpModeSpec{
			{Name: "health", Summary: "Connection and readiness snapshot", Params: []string{}},
			{Name: "doctor", Summary: "Actionable setup diagnostics", Params: []string{}},
			{Name: "noise_rule", Summary: "Manage noise filters", Params: []string{"noise_action", "rules", "rule_id"}},
			{Name: "streaming", Summary: "Configure notifications", Params: []string{"streaming_action", "events", "throttle_seconds"}},
			{Name: "tutorial", Summary: "Quickstart snippets", Params: []string{}},
			{Name: "examples", Summary: "Task-oriented workflows", Params: []string{}},
			{Name: "help", Summary: "Compact capability cheat sheet", Params: []string{"tool", "mode"}},
		},
	},
}

func sortedHelpTools() []string {
	keys := make([]string, 0, len(helpSpecs))
	for key := range helpSpecs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func findHelpMode(spec helpToolSpec, mode string) (helpModeSpec, bool) {
	for _, m := range spec.Modes {
		if m.Name == mode {
			return m, true
		}
	}
	return helpModeSpec{}, false
}

func helpModeToMap(mode helpModeSpec) map[string]any {
	return map[string]any{
		"name":    mode.Name,
		"summary": mode.Summary,
		"params":  mode.Params,
	}
}

func helpToolToMap(tool helpToolSpec) map[string]any {
	modes := make([]map[string]any, 0, len(tool.Modes))
	for _, mode := range tool.Modes {
		modes = append(modes, helpModeToMap(mode))
	}
	return map[string]any{
		"name":          tool.Name,
		"summary":       tool.Summary,
		"common_params": tool.CommonParams,
		"modes":         modes,
	}
}

func renderHelpText(tools []helpToolSpec) string {
	var b strings.Builder
	for i, tool := range tools {
		b.WriteString(fmt.Sprintf("%s - %s\n", tool.Name, tool.Summary))
		for _, mode := range tool.Modes {
			params := strings.Join(mode.Params, ", ")
			if params == "" {
				params = "no specific params"
			}
			b.WriteString(fmt.Sprintf("  %s -> %s (%s)\n", mode.Name, mode.Summary, params))
		}
		common := strings.Join(tool.CommonParams, ", ")
		if common == "" {
			common = "none"
		}
		b.WriteString(fmt.Sprintf("  Common params: %s\n", common))
		if i < len(tools)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (h *ToolHandler) toolConfigureHelp(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Tool string `json:"tool"`
		Mode string `json:"mode"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	selectedTool := strings.TrimSpace(strings.ToLower(params.Tool))
	selectedMode := strings.TrimSpace(strings.ToLower(params.Mode))
	if selectedTool == "" && selectedMode != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Parameter 'mode' requires parameter 'tool'", "Provide tool and mode together, for example configure({what:'help', tool:'analyze', mode:'accessibility'})", withParam("tool"), withParam("mode"))}
	}

	tools := make([]helpToolSpec, 0, len(helpSpecs))
	if selectedTool != "" {
		spec, ok := helpSpecs[selectedTool]
		if !ok {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Unknown tool for help: "+selectedTool, "Use tool one of: analyze, configure, generate, interact, observe", withParam("tool"))}
		}
		if selectedMode != "" {
			modeSpec, ok := findHelpMode(spec, selectedMode)
			if !ok {
				return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Unknown mode for tool '"+selectedTool+"': "+selectedMode, "Use a mode listed in configure({what:'help', tool:'"+selectedTool+"'})", withParam("mode"))}
			}
			spec.Modes = []helpModeSpec{modeSpec}
		}
		tools = append(tools, spec)
	} else {
		for _, name := range sortedHelpTools() {
			tools = append(tools, helpSpecs[name])
		}
	}

	toolPayload := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		toolPayload = append(toolPayload, helpToolToMap(tool))
	}

	payload := map[string]any{
		"status":  "ok",
		"mode":    "help",
		"message": "Compact capability cheat sheet for MCP tool routing",
		"tools":   toolPayload,
		"text":    renderHelpText(tools),
	}
	if selectedTool != "" {
		payload["tool"] = selectedTool
	}
	if selectedMode != "" {
		payload["focus_mode"] = selectedMode
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Capabilities cheat sheet", payload)}
}
