// capabilities.go — Pure function for building machine-readable tool capability maps.
package configure

import (
	"sort"

	"github.com/dev-console/dev-console/internal/mcp"
)

// BuildCapabilitiesMap transforms a list of MCP tools into a machine-readable
// map of tool metadata. For each tool, it extracts the dispatch parameter
// (first required field), its enum values (modes), and all other parameter names.
func BuildCapabilitiesMap(tools []mcp.MCPTool) map[string]any {
	toolsMap := make(map[string]any, len(tools))
	for _, tool := range tools {
		props, _ := tool.InputSchema["properties"].(map[string]any)
		required, _ := tool.InputSchema["required"].([]string)

		// Extract the dispatch parameter (first required field)
		dispatchParam := ""
		if len(required) > 0 {
			dispatchParam = required[0]
		}

		// Extract enum values for the dispatch parameter
		var modes []string
		if dispatchParam != "" {
			if dp, ok := props[dispatchParam].(map[string]any); ok {
				if enumVals, ok := dp["enum"].([]string); ok {
					modes = enumVals
				}
			}
		}

		// Extract parameter names
		paramNames := make([]string, 0, len(props))
		for k := range props {
			if k != dispatchParam {
				paramNames = append(paramNames, k)
			}
		}
		sort.Strings(paramNames)

		toolsMap[tool.Name] = map[string]any{
			"dispatch_param": dispatchParam,
			"modes":          modes,
			"params":         paramNames,
			"description":    tool.Description,
		}
	}
	return toolsMap
}
