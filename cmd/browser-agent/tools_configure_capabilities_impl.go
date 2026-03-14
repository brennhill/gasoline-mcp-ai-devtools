// Purpose: Implements configure capabilities introspection response shaping.
// Why: Keeps capability filtering/reporting isolated from configure routing logic.

package main

import (
	"encoding/json"
	"sort"
	"strings"

	cfg "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/configure"
)

// extractModesFromCap extracts the list of valid mode names from a tool capability map.
// Handles both []string and []any representations from JSON.
func extractModesFromCap(toolCap map[string]any) []string {
	if modes, ok := toolCap["modes"].([]string); ok {
		return modes
	}
	modesAny, ok := toolCap["modes"].([]any)
	if !ok {
		return nil
	}
	modes := make([]string, 0, len(modesAny))
	for _, m := range modesAny {
		if s, ok := m.(string); ok {
			modes = append(modes, s)
		}
	}
	return modes
}

// capabilitiesForSingleTool builds the response for a single tool, optionally filtered by mode.
func (h *ToolHandler) capabilitiesForSingleTool(req JSONRPCRequest, tools []MCPTool, toolName, mode string) JSONRPCResponse {
	toolCap, ok := cfg.BuildCapabilitiesForTool(tools, toolName)
	if !ok {
		validNames := make([]string, 0, len(tools))
		for _, t := range tools {
			validNames = append(validNames, t.Name)
		}
		sort.Strings(validNames)
		return fail(req, ErrInvalidParam,
			"Unknown tool: "+toolName,
			"Use a valid tool name",
			withParam("tool"),
			withHint("Valid tools: "+strings.Join(validNames, ", ")))
	}

	if mode != "" {
		modeCap, ok := cfg.FilterToolByMode(toolCap, toolName, mode)
		if !ok {
			return fail(req, ErrInvalidParam,
				"Unknown mode '"+mode+"' for tool '"+toolName+"'",
				"Use a valid mode for this tool",
				withParam("mode"),
				withHint("Valid modes: "+strings.Join(extractModesFromCap(toolCap), ", ")))
		}
		return succeed(req, "Capabilities", modeCap)
	}

	result := map[string]any{
		"version":          version,
		"protocol_version": "2024-11-05",
		"tools":            map[string]any{toolName: toolCap},
	}
	if module, ok := h.toolModules.get(toolName); ok {
		if examples := module.Examples(); len(examples) > 0 {
			result["examples"] = examples
		}
	}
	return succeed(req, "Capabilities", result)
}

// toolConfigureDescribeCapabilities returns machine-readable tool metadata derived from ToolsList().
// Supports filtering by tool name and mode to reduce payload size.
// When summary=true, returns only tool name -> { description, dispatch_param, modes }.
func (h *ToolHandler) toolConfigureDescribeCapabilities(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Summary bool   `json:"summary"`
		Tool    string `json:"tool"`
		Mode    string `json:"mode"`
	}
	lenientUnmarshal(args, &params)

	tools := h.ToolsList()

	if params.Mode != "" && params.Tool == "" {
		return fail(req, ErrInvalidParam,
			"'mode' requires 'tool' to be set",
			"Add the 'tool' parameter to filter by mode",
			withParam("tool"))
	}

	if params.Tool != "" {
		return h.capabilitiesForSingleTool(req, tools, params.Tool, params.Mode)
	}

	// Full or summary response (no filters).
	var toolsMap map[string]any
	if params.Summary {
		toolsMap = cfg.BuildCapabilitiesSummary(tools)
	} else {
		toolsMap = cfg.BuildCapabilitiesMap(tools)
	}

	return succeed(req, "Capabilities", map[string]any{
		"version":          version,
		"protocol_version": "2024-11-05",
		"tools":            toolsMap,
		"deprecated":       []string{},
	})
}
