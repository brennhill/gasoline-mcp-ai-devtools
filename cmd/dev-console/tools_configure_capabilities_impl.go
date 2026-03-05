// Purpose: Implements configure capabilities introspection response shaping.
// Why: Keeps capability filtering/reporting isolated from configure routing logic.

package main

import (
	"encoding/json"
	"sort"
	"strings"

	cfg "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/configure"
)

// configureDescribeCapabilitiesImpl returns machine-readable tool metadata derived from ToolsList().
// Supports filtering by tool name and mode to reduce payload size.
// When summary=true, returns only tool name -> { description, dispatch_param, modes }.
func (h *ToolHandler) configureDescribeCapabilitiesImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Summary bool   `json:"summary"`
		Tool    string `json:"tool"`
		Mode    string `json:"mode"`
	}
	lenientUnmarshal(args, &params)

	tools := h.ToolsList()

	// mode without tool is an error.
	if params.Mode != "" && params.Tool == "" {
		return fail(req, ErrInvalidParam,
			"'mode' requires 'tool' to be set",
			"Add the 'tool' parameter to filter by mode",
			withParam("tool"))
	}

	// Filter by tool + mode.
	if params.Tool != "" {
		toolCap, ok := cfg.BuildCapabilitiesForTool(tools, params.Tool)
		if !ok {
			validNames := make([]string, 0, len(tools))
			for _, t := range tools {
				validNames = append(validNames, t.Name)
			}
			sort.Strings(validNames)
			return fail(req, ErrInvalidParam,
				"Unknown tool: "+params.Tool,
				"Use a valid tool name",
				withParam("tool"),
				withHint("Valid tools: "+strings.Join(validNames, ", ")))
		}

		if params.Mode != "" {
			modeCap, ok := cfg.FilterToolByMode(toolCap, params.Tool, params.Mode)
			if !ok {
				modes, _ := toolCap["modes"].([]string)
				if modes == nil {
					if modesAny, ok := toolCap["modes"].([]any); ok {
						for _, m := range modesAny {
							if s, ok := m.(string); ok {
								modes = append(modes, s)
							}
						}
					}
				}
				return fail(req, ErrInvalidParam,
					"Unknown mode '"+params.Mode+"' for tool '"+params.Tool+"'",
					"Use a valid mode for this tool",
					withParam("mode"),
					withHint("Valid modes: "+strings.Join(modes, ", ")))
			}
			return succeed(req, "Capabilities", modeCap)
		}

		return succeed(req, "Capabilities", map[string]any{
			"version":          version,
			"protocol_version": "2024-11-05",
			"tools":            map[string]any{params.Tool: toolCap},
		})
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
