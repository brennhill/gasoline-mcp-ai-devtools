// configure.go — CLI argument parser for the configure tool.
// Maps CLI flags to MCP configure tool arguments.
package commands

import "encoding/json"

// ConfigureArgs parses CLI args for the configure tool and returns MCP arguments.
func ConfigureArgs(action string, args []string) (map[string]any, error) {
	action = NormalizeAction(action)
	mcpArgs := map[string]any{
		"action": action,
	}

	remaining := args

	// Parse --key (for store, load)
	var key string
	key, remaining = parseFlag(remaining, "--key")
	if key != "" {
		mcpArgs["key"] = key
	}

	// Parse --data (for store — JSON string)
	var data string
	data, remaining = parseFlag(remaining, "--data")
	if data != "" {
		// Parse data as JSON object
		var parsed map[string]any
		if err := json.Unmarshal([]byte(data), &parsed); err == nil {
			mcpArgs["data"] = parsed
		} else {
			// If not JSON, store as string
			mcpArgs["data"] = data
		}
	}

	// Parse --namespace (for store)
	var namespace string
	namespace, remaining = parseFlag(remaining, "--namespace")
	if namespace != "" {
		mcpArgs["namespace"] = namespace
	}

	// Parse --pattern (for noise_rule)
	var pattern string
	pattern, remaining = parseFlag(remaining, "--pattern")
	if pattern != "" {
		mcpArgs["pattern"] = pattern
	}

	// Parse --reason (for noise_rule)
	var reason string
	reason, remaining = parseFlag(remaining, "--reason")
	if reason != "" {
		mcpArgs["reason"] = reason
	}

	// Parse --noise-action (for noise_rule: add, remove, list, reset)
	var noiseAction string
	noiseAction, remaining = parseFlag(remaining, "--noise-action")
	if noiseAction != "" {
		mcpArgs["noise_action"] = noiseAction
	}

	// Parse --buffer (for clear: network, websocket, actions, logs, all)
	var buffer string
	buffer, remaining = parseFlag(remaining, "--buffer")
	if buffer != "" {
		mcpArgs["buffer"] = buffer
	}

	// Parse --rule-id (for noise_rule remove)
	var ruleID string
	ruleID, remaining = parseFlag(remaining, "--rule-id")
	if ruleID != "" {
		mcpArgs["rule_id"] = ruleID
	}

	// Parse --store-action (for store: save, load, list, delete, stats)
	var storeAction string
	storeAction, remaining = parseFlag(remaining, "--store-action")
	if storeAction != "" {
		mcpArgs["store_action"] = storeAction
	}

	// Parse --selector (for query_dom)
	var selector string
	selector, remaining = parseFlag(remaining, "--selector")
	if selector != "" {
		mcpArgs["selector"] = selector
	}

	_ = remaining

	return mcpArgs, nil
}
