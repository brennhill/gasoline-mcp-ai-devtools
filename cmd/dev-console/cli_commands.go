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

// parseObserveArgs parses CLI args for the observe tool.
func parseObserveArgs(mode string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{
		"what": mode,
	}

	remaining := args

	var limit int
	var hasLimit bool
	limit, hasLimit, remaining = cliParseFlagInt(remaining, "--limit")
	if hasLimit {
		mcpArgs["limit"] = limit
	}

	var minLevel string
	minLevel, remaining = cliParseFlag(remaining, "--min-level")
	if minLevel != "" {
		mcpArgs["min_level"] = minLevel
	}

	var url string
	url, remaining = cliParseFlag(remaining, "--url")
	if url != "" {
		mcpArgs["url"] = url
	}

	var method string
	method, remaining = cliParseFlag(remaining, "--method")
	if method != "" {
		mcpArgs["method"] = method
	}

	var statusMin int
	var hasStatusMin bool
	statusMin, hasStatusMin, remaining = cliParseFlagInt(remaining, "--status-min")
	if hasStatusMin {
		mcpArgs["status_min"] = statusMin
	}

	var statusMax int
	var hasStatusMax bool
	statusMax, hasStatusMax, remaining = cliParseFlagInt(remaining, "--status-max")
	if hasStatusMax {
		mcpArgs["status_max"] = statusMax
	}

	var scope string
	scope, remaining = cliParseFlag(remaining, "--scope")
	if scope != "" {
		mcpArgs["scope"] = scope
	}

	var lastN int
	var hasLastN bool
	lastN, hasLastN, remaining = cliParseFlagInt(remaining, "--last-n")
	if hasLastN {
		mcpArgs["last_n"] = lastN
	}

	var connID string
	connID, remaining = cliParseFlag(remaining, "--connection-id")
	if connID != "" {
		mcpArgs["connection_id"] = connID
	}

	var direction string
	direction, remaining = cliParseFlag(remaining, "--direction")
	if direction != "" {
		mcpArgs["direction"] = direction
	}

	_ = remaining
	return mcpArgs, nil
}

// parseGenerateArgs parses CLI args for the generate tool.
func parseGenerateArgs(format string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{
		"format": format,
	}

	remaining := args

	var testName string
	testName, remaining = cliParseFlag(remaining, "--test-name")
	if testName != "" {
		mcpArgs["test_name"] = testName
	}

	var errorMsg string
	errorMsg, remaining = cliParseFlag(remaining, "--error-message")
	if errorMsg != "" {
		mcpArgs["error_message"] = errorMsg
	}

	var url string
	url, remaining = cliParseFlag(remaining, "--url")
	if url != "" {
		mcpArgs["url"] = url
	}

	var method string
	method, remaining = cliParseFlag(remaining, "--method")
	if method != "" {
		mcpArgs["method"] = method
	}

	var mode string
	mode, remaining = cliParseFlag(remaining, "--mode")
	if mode != "" {
		mcpArgs["mode"] = mode
	}

	var saveTo string
	saveTo, remaining = cliParseFlag(remaining, "--save-to")
	if saveTo != "" {
		mcpArgs["save_to"] = saveTo
	}

	var statusMin int
	var hasStatusMin bool
	statusMin, hasStatusMin, remaining = cliParseFlagInt(remaining, "--status-min")
	if hasStatusMin {
		mcpArgs["status_min"] = statusMin
	}

	var statusMax int
	var hasStatusMax bool
	statusMax, hasStatusMax, remaining = cliParseFlagInt(remaining, "--status-max")
	if hasStatusMax {
		mcpArgs["status_max"] = statusMax
	}

	var baseURL string
	baseURL, remaining = cliParseFlag(remaining, "--base-url")
	if baseURL != "" {
		mcpArgs["base_url"] = baseURL
	}

	var lastN int
	var hasLastN bool
	lastN, hasLastN, remaining = cliParseFlagInt(remaining, "--last-n")
	if hasLastN {
		mcpArgs["last_n"] = lastN
	}

	var assertNetwork bool
	assertNetwork, remaining = cliParseFlagBool(remaining, "--assert-network")
	if assertNetwork {
		mcpArgs["assert_network"] = true
	}

	var assertNoErrors bool
	assertNoErrors, remaining = cliParseFlagBool(remaining, "--assert-no-errors")
	if assertNoErrors {
		mcpArgs["assert_no_errors"] = true
	}

	var includeScreenshots bool
	includeScreenshots, remaining = cliParseFlagBool(remaining, "--include-screenshots")
	if includeScreenshots {
		mcpArgs["include_screenshots"] = true
	}

	_ = remaining
	return mcpArgs, nil
}

// parseConfigureArgs parses CLI args for the configure tool.
func parseConfigureArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{
		"action": action,
	}

	remaining := args

	var key string
	key, remaining = cliParseFlag(remaining, "--key")
	if key != "" {
		mcpArgs["key"] = key
	}

	var data string
	data, remaining = cliParseFlag(remaining, "--data")
	if data != "" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(data), &parsed); err == nil {
			mcpArgs["data"] = parsed
		} else {
			mcpArgs["data"] = data
		}
	}

	var namespace string
	namespace, remaining = cliParseFlag(remaining, "--namespace")
	if namespace != "" {
		mcpArgs["namespace"] = namespace
	}

	var pattern string
	pattern, remaining = cliParseFlag(remaining, "--pattern")
	if pattern != "" {
		mcpArgs["pattern"] = pattern
	}

	var reason string
	reason, remaining = cliParseFlag(remaining, "--reason")
	if reason != "" {
		mcpArgs["reason"] = reason
	}

	var noiseAction string
	noiseAction, remaining = cliParseFlag(remaining, "--noise-action")
	if noiseAction != "" {
		mcpArgs["noise_action"] = noiseAction
	}

	var buffer string
	buffer, remaining = cliParseFlag(remaining, "--buffer")
	if buffer != "" {
		mcpArgs["buffer"] = buffer
	}

	var ruleID string
	ruleID, remaining = cliParseFlag(remaining, "--rule-id")
	if ruleID != "" {
		mcpArgs["rule_id"] = ruleID
	}

	var storeAction string
	storeAction, remaining = cliParseFlag(remaining, "--store-action")
	if storeAction != "" {
		mcpArgs["store_action"] = storeAction
	}

	var selector string
	selector, remaining = cliParseFlag(remaining, "--selector")
	if selector != "" {
		mcpArgs["selector"] = selector
	}

	_ = remaining
	return mcpArgs, nil
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

// parseInteractArgs parses CLI args for the interact tool.
func parseInteractArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{
		"action": action,
	}

	remaining := args

	var selector string
	selector, remaining = cliParseFlag(remaining, "--selector")
	if selector != "" {
		mcpArgs["selector"] = selector
	}

	var text string
	text, remaining = cliParseFlag(remaining, "--text")
	if text != "" {
		mcpArgs["text"] = text
	}

	var url string
	url, remaining = cliParseFlag(remaining, "--url")
	if url != "" {
		mcpArgs["url"] = url
	}

	var filePath string
	filePath, remaining = cliParseFlag(remaining, "--file-path")
	if filePath != "" {
		if !filepath.IsAbs(filePath) {
			cwd, err := os.Getwd()
			if err == nil {
				filePath = filepath.Join(cwd, filePath)
			}
		}
		mcpArgs["file_path"] = filePath
	}

	var name string
	name, remaining = cliParseFlag(remaining, "--name")
	if name != "" {
		mcpArgs["name"] = name
	}

	var value string
	value, remaining = cliParseFlag(remaining, "--value")
	if value != "" {
		mcpArgs["value"] = value
	}

	var timeoutMs int
	var hasTimeout bool
	timeoutMs, hasTimeout, remaining = cliParseFlagInt(remaining, "--timeout-ms")
	if hasTimeout {
		mcpArgs["timeout_ms"] = timeoutMs
	}

	var script string
	script, remaining = cliParseFlag(remaining, "--script")
	if script != "" {
		mcpArgs["script"] = script
	}

	var clear bool
	clear, remaining = cliParseFlagBool(remaining, "--clear")
	if clear {
		mcpArgs["clear"] = true
	}

	var reason string
	reason, remaining = cliParseFlag(remaining, "--reason")
	if reason != "" {
		mcpArgs["reason"] = reason
	}

	var subtitle string
	subtitle, remaining = cliParseFlag(remaining, "--subtitle")
	if subtitle != "" {
		mcpArgs["subtitle"] = subtitle
	}

	_ = remaining

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

	return mcpArgs, nil
}

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
