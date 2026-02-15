// cli_commands.go — Argument parsers for CLI mode.
// Maps CLI flags to MCP tool arguments for observe, generate, configure, interact.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// parseCLIArgs dispatches to the correct tool parser based on tool name.
func parseCLIArgs(tool, action string, args []string) (map[string]any, error) {
	action = normalizeAction(action)
	switch tool {
	case "observe":
		return parseObserveArgs(action, args)
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

// --- Flag definition types ---

type stringFlag struct {
	cli string // CLI flag name, e.g. "--url"
	mcp string // MCP arg key, e.g. "url"
}

type intFlag struct {
	cli string
	mcp string
}

type boolFlag struct {
	cli string
	mcp string
}

// applyStringFlags parses all string flags from args into mcpArgs.
func applyStringFlags(args []string, mcpArgs map[string]any, flags []stringFlag) []string {
	for _, f := range flags {
		var val string
		val, args = cliParseFlag(args, f.cli)
		if val != "" {
			mcpArgs[f.mcp] = val
		}
	}
	return args
}

// applyIntFlags parses all integer flags from args into mcpArgs.
func applyIntFlags(args []string, mcpArgs map[string]any, flags []intFlag) []string {
	for _, f := range flags {
		var val int
		var ok bool
		val, ok, args = cliParseFlagInt(args, f.cli)
		if ok {
			mcpArgs[f.mcp] = val
		}
	}
	return args
}

// applyBoolFlags parses all boolean flags from args into mcpArgs.
func applyBoolFlags(args []string, mcpArgs map[string]any, flags []boolFlag) []string {
	for _, f := range flags {
		var val bool
		val, args = cliParseFlagBool(args, f.cli)
		if val {
			mcpArgs[f.mcp] = true
		}
	}
	return args
}

// --- Tool parsers ---

func parseObserveArgs(mode string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": mode}
	args = applyStringFlags(args, mcpArgs, []stringFlag{
		{"--min-level", "min_level"},
		{"--url", "url"},
		{"--method", "method"},
		{"--scope", "scope"},
		{"--connection-id", "connection_id"},
		{"--direction", "direction"},
	})
	applyIntFlags(args, mcpArgs, []intFlag{
		{"--limit", "limit"},
		{"--status-min", "status_min"},
		{"--status-max", "status_max"},
		{"--last-n", "last_n"},
	})
	return mcpArgs, nil
}

func parseGenerateArgs(format string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"format": format}
	args = applyStringFlags(args, mcpArgs, []stringFlag{
		{"--test-name", "test_name"},
		{"--error-message", "error_message"},
		{"--url", "url"},
		{"--method", "method"},
		{"--mode", "mode"},
		{"--save-to", "save_to"},
		{"--base-url", "base_url"},
	})
	args = applyIntFlags(args, mcpArgs, []intFlag{
		{"--status-min", "status_min"},
		{"--status-max", "status_max"},
		{"--last-n", "last_n"},
	})
	applyBoolFlags(args, mcpArgs, []boolFlag{
		{"--assert-network", "assert_network"},
		{"--assert-no-errors", "assert_no_errors"},
		{"--include-screenshots", "include_screenshots"},
	})
	return mcpArgs, nil
}

func parseConfigureArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"action": action}

	// --data needs special handling: try JSON parse, fall back to string.
	var data string
	data, args = cliParseFlag(args, "--data")
	if data != "" {
		mcpArgs["data"] = parseJSONOrString(data)
	}

	applyStringFlags(args, mcpArgs, []stringFlag{
		{"--key", "key"},
		{"--namespace", "namespace"},
		{"--pattern", "pattern"},
		{"--reason", "reason"},
		{"--noise-action", "noise_action"},
		{"--buffer", "buffer"},
		{"--rule-id", "rule_id"},
		{"--store-action", "store_action"},
		{"--selector", "selector"},
	})
	return mcpArgs, nil
}

// parseJSONOrString attempts to parse s as JSON object; returns the raw string on failure.
func parseJSONOrString(s string) any {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(s), &parsed); err == nil {
		return parsed
	}
	return s
}

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

func parseInteractArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"action": action}
	args = applyStringFlags(args, mcpArgs, []stringFlag{
		{"--selector", "selector"},
		{"--text", "text"},
		{"--url", "url"},
		{"--name", "name"},
		{"--value", "value"},
		{"--script", "script"},
		{"--reason", "reason"},
		{"--subtitle", "subtitle"},
	})

	// --file-path needs special handling for relative path resolution.
	args = parseInteractFilePath(args, mcpArgs)

	args = applyIntFlags(args, mcpArgs, []intFlag{
		{"--timeout-ms", "timeout_ms"},
	})
	applyBoolFlags(args, mcpArgs, []boolFlag{
		{"--clear", "clear"},
	})

	return mcpArgs, validateInteractArgs(action, mcpArgs)
}

// parseInteractFilePath extracts --file-path and resolves relative paths to absolute.
func parseInteractFilePath(args []string, mcpArgs map[string]any) []string {
	var filePath string
	filePath, args = cliParseFlag(args, "--file-path")
	if filePath != "" {
		if !filepath.IsAbs(filePath) {
			if cwd, err := os.Getwd(); err == nil {
				filePath = filepath.Join(cwd, filePath)
			}
		}
		mcpArgs["file_path"] = filePath
	}
	return args
}

// validateInteractArgs checks required fields for specific interact actions.
func validateInteractArgs(action string, mcpArgs map[string]any) error {
	selector, _ := mcpArgs["selector"].(string)
	if interactActionsRequiringSelector[action] && selector == "" {
		return fmt.Errorf("interact %s: --selector is required", action)
	}
	if action == "navigate" && mcpArgs["url"] == nil {
		return fmt.Errorf("interact navigate: --url is required")
	}
	if action == "execute_js" && mcpArgs["script"] == nil {
		return fmt.Errorf("interact execute_js: --script is required")
	}
	return nil
}

// --- Low-level flag parsers ---

// cliParseFlag extracts a string flag value from args, returning the value and remaining args.
func cliParseFlag(args []string, flag string) (string, []string) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			val := args[i+1]
			remaining := make([]string, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)
			return val, remaining
		}
	}
	return "", args
}

// cliParseFlagInt extracts an integer flag value from args.
func cliParseFlagInt(args []string, flag string) (int, bool, []string) {
	val, remaining := cliParseFlag(args, flag)
	if val == "" {
		return 0, false, args
	}
	var n int
	for _, c := range val {
		if c < '0' || c > '9' {
			return 0, false, args
		}
		n = n*10 + int(c-'0')
	}
	return n, true, remaining
}

// cliParseFlagBool checks if a boolean flag is present in args.
func cliParseFlagBool(args []string, flag string) (bool, []string) {
	for i, a := range args {
		if a == flag {
			remaining := make([]string, 0, len(args)-1)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return true, remaining
		}
	}
	return false, args
}
