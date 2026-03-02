// Purpose: Dispatches CLI commands to tool-specific argument parsers.
// Why: Keeps top-level CLI parser intent explicit while detailed parsing lives in focused modules.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

import (
	"fmt"
	"strings"
)

// parseCLIArgs dispatches to the correct tool parser based on tool name.
func parseCLIArgs(tool, action string, args []string) (map[string]any, error) {
	action = normalizeAction(action)
	switch tool {
	case "observe":
		return parseObserveArgs(action, args)
	case "analyze":
		return parseAnalyzeArgs(action, args)
	case "generate":
		return parseGenerateArgs(action, args)
	case "configure":
		return parseConfigureArgs(action, args)
	case "interact":
		return parseInteractArgs(action, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}
}

// normalizeAction converts CLI-style kebab-case to MCP snake_case.
func normalizeAction(action string) string {
	return strings.ReplaceAll(action, "-", "_")
}
