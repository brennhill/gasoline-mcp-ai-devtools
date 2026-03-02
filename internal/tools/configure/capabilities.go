// Purpose: Builds describe_capabilities responses by introspecting tool schemas, modes, and per-mode parameter sets.
// Docs: docs/features/feature/config-profiles/index.md
// Docs: docs/features/describe_capabilities.md

package configure

import (
	"sort"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

type modeParamSpec struct {
	Hint     string
	Required []string
	Optional []string
}

// BuildCapabilitiesSummary returns a compact summary: tool name → { description, dispatch_param, modes }.
// modes is a map of mode name → one-line hint for discovery.
// Omits full parameter schemas, reducing output from ~757K to ~10K.
func BuildCapabilitiesSummary(tools []mcp.MCPTool) map[string]any {
	toolsMap := make(map[string]any, len(tools))
	for _, tool := range tools {
		props, _ := tool.InputSchema["properties"].(map[string]any)
		dispatchParam := inferDispatchParam(tool.InputSchema)

		modeNames := extractModes(dispatchParam, props)
		modes := buildModeIndex(tool.Name, modeNames)

		toolsMap[tool.Name] = map[string]any{
			"description":    tool.Description,
			"dispatch_param": dispatchParam,
			"modes":          modes,
		}
	}
	return toolsMap
}

// buildModeIndex returns a mode name → hint map for a tool.
// Falls back to empty string if no hint is defined for a mode.
func buildModeIndex(toolName string, modeNames []string) map[string]string {
	specs := toolModeSpecs[toolName]
	index := make(map[string]string, len(modeNames))
	for _, mode := range modeNames {
		hint := ""
		if specs != nil {
			if spec, ok := specs[mode]; ok {
				hint = spec.Hint
			}
		}
		index[mode] = hint
	}
	return index
}

// buildToolCapEntry builds the full capability entry map for a single MCPTool.
// Shared by BuildCapabilitiesMap and BuildCapabilitiesForTool.
func buildToolCapEntry(tool mcp.MCPTool) map[string]any {
	props, _ := tool.InputSchema["properties"].(map[string]any)
	dispatchParam := inferDispatchParam(tool.InputSchema)

	modes := extractModes(dispatchParam, props)

	paramNames := make([]string, 0, len(props))
	for name := range props {
		if name != dispatchParam {
			paramNames = append(paramNames, name)
		}
	}
	sort.Strings(paramNames)

	paramDetails := buildParamDetails(props)
	modeParams := buildModeParams(tool.Name, modes, dispatchParam, paramNames, paramDetails)

	return map[string]any{
		"dispatch_param": dispatchParam,
		"modes":          modes,
		"params":         paramNames,
		"param_details":  paramDetails,
		"mode_params":    modeParams,
		"description":    tool.Description,
	}
}

// BuildCapabilitiesMap transforms tool schemas into machine-readable capability metadata.
// It preserves legacy fields (dispatch_param, modes, params, description) and adds:
// - param_details: per-parameter type/enum/default metadata
// - mode_params: per-mode required/optional params and metadata
func BuildCapabilitiesMap(tools []mcp.MCPTool) map[string]any {
	toolsMap := make(map[string]any, len(tools))
	for _, tool := range tools {
		toolsMap[tool.Name] = buildToolCapEntry(tool)
	}
	return toolsMap
}

// BuildCapabilitiesForTool returns the full capability map for a single tool by name.
// Returns (toolCapabilities, true) if found, or (nil, false) if the tool name is unknown.
func BuildCapabilitiesForTool(tools []mcp.MCPTool, toolName string) (map[string]any, bool) {
	for _, tool := range tools {
		if tool.Name != toolName {
			continue
		}
		return buildToolCapEntry(tool), true
	}
	return nil, false
}

// FilterToolByMode extracts a single mode's entry from a tool's capability map.
// Returns a flat structure: {tool, mode, required, optional, params}.
// Returns (nil, false) if the mode is not found in mode_params.
func FilterToolByMode(toolCap map[string]any, toolName, mode string) (map[string]any, bool) {
	modeParamsRaw, ok := toolCap["mode_params"].(map[string]any)
	if !ok {
		return nil, false
	}
	modeEntry, ok := modeParamsRaw[mode].(map[string]any)
	if !ok {
		return nil, false
	}
	return map[string]any{
		"tool":     toolName,
		"mode":     mode,
		"required": modeEntry["required"],
		"optional": modeEntry["optional"],
		"params":   modeEntry["params"],
	}, true
}
