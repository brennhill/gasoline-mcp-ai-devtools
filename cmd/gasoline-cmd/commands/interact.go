// interact.go — CLI argument parser for the interact tool.
// Maps CLI flags to MCP interact tool arguments.
package commands

import (
	"fmt"
	"os"
	"path/filepath"
)

// interactActionsRequiringSelector lists actions that need --selector.
var interactActionsRequiringSelector = map[string]bool{
	"click":         true,
	"type":          true,
	"get_text":      true,
	"get_value":     true,
	"get_attribute": true,
	"set_attribute": true,
	"wait_for":      true,
	"scroll_to":     true,
	"focus":         true,
	"check":         true,
	"upload":        true,
	"highlight":     true,
}

// InteractArgs parses CLI args for the interact tool and returns MCP arguments.
func InteractArgs(action string, args []string) (map[string]any, error) {
	action = NormalizeAction(action)
	mcpArgs := map[string]any{
		"action": action,
	}

	remaining := args

	// Parse --selector
	var selector string
	selector, remaining = parseFlag(remaining, "--selector")
	if selector != "" {
		mcpArgs["selector"] = selector
	}

	// Parse --text (for type, key_press)
	var text string
	text, remaining = parseFlag(remaining, "--text")
	if text != "" {
		mcpArgs["text"] = text
	}

	// Parse --url (for navigate)
	var url string
	url, remaining = parseFlag(remaining, "--url")
	if url != "" {
		mcpArgs["url"] = url
	}

	// Parse --file-path (for upload)
	var filePath string
	filePath, remaining = parseFlag(remaining, "--file-path")
	if filePath != "" {
		// Convert relative paths to absolute
		if !filepath.IsAbs(filePath) {
			cwd, err := os.Getwd()
			if err == nil {
				filePath = filepath.Join(cwd, filePath)
			}
		}
		mcpArgs["file_path"] = filePath
	}

	// Parse --name (for get_attribute, set_attribute)
	var name string
	name, remaining = parseFlag(remaining, "--name")
	if name != "" {
		mcpArgs["name"] = name
	}

	// Parse --value (for set_attribute, select)
	var value string
	value, remaining = parseFlag(remaining, "--value")
	if value != "" {
		mcpArgs["value"] = value
	}

	// Parse --timeout-ms (for wait_for, execute_js)
	var timeoutMs int
	var hasTimeout bool
	timeoutMs, hasTimeout, remaining = parseFlagInt(remaining, "--timeout-ms")
	if hasTimeout {
		mcpArgs["timeout_ms"] = timeoutMs
	}

	// Parse --script (for execute_js)
	var script string
	script, remaining = parseFlag(remaining, "--script")
	if script != "" {
		mcpArgs["script"] = script
	}

	// Parse --clear (boolean for type)
	var clear bool
	clear, remaining = parseFlagBool(remaining, "--clear")
	if clear {
		mcpArgs["clear"] = true
	}

	// Parse --reason (for action annotation)
	var reason string
	reason, remaining = parseFlag(remaining, "--reason")
	if reason != "" {
		mcpArgs["reason"] = reason
	}

	// Parse --subtitle
	var subtitle string
	subtitle, remaining = parseFlag(remaining, "--subtitle")
	if subtitle != "" {
		mcpArgs["subtitle"] = subtitle
	}

	// Validate required fields
	if interactActionsRequiringSelector[action] && selector == "" {
		return nil, fmt.Errorf("interact %s: --selector is required", action)
	}

	if action == "navigate" && url == "" {
		return nil, fmt.Errorf("interact navigate: --url is required")
	}

	if action == "execute_js" && script == "" {
		return nil, fmt.Errorf("interact execute_js: --script is required")
	}

	_ = remaining // Suppress unused warning; unknown flags are silently ignored

	return mcpArgs, nil
}
