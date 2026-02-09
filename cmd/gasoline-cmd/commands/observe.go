// observe.go — CLI argument parser for the observe tool.
// Maps CLI flags to MCP observe tool arguments.
package commands

// ObserveArgs parses CLI args for the observe tool and returns MCP arguments.
func ObserveArgs(mode string, args []string) (map[string]any, error) {
	mode = NormalizeAction(mode)
	mcpArgs := map[string]any{
		"what": mode,
	}

	remaining := args

	// Parse --limit
	var limit int
	var hasLimit bool
	limit, hasLimit, remaining = parseFlagInt(remaining, "--limit")
	if hasLimit {
		mcpArgs["limit"] = limit
	}

	// Parse --min-level (for logs)
	var minLevel string
	minLevel, remaining = parseFlag(remaining, "--min-level")
	if minLevel != "" {
		mcpArgs["min_level"] = minLevel
	}

	// Parse --url (for network_waterfall, network_bodies)
	var url string
	url, remaining = parseFlag(remaining, "--url")
	if url != "" {
		mcpArgs["url"] = url
	}

	// Parse --method (for network_bodies)
	var method string
	method, remaining = parseFlag(remaining, "--method")
	if method != "" {
		mcpArgs["method"] = method
	}

	// Parse --status-min
	var statusMin int
	var hasStatusMin bool
	statusMin, hasStatusMin, remaining = parseFlagInt(remaining, "--status-min")
	if hasStatusMin {
		mcpArgs["status_min"] = statusMin
	}

	// Parse --status-max
	var statusMax int
	var hasStatusMax bool
	statusMax, hasStatusMax, remaining = parseFlagInt(remaining, "--status-max")
	if hasStatusMax {
		mcpArgs["status_max"] = statusMax
	}

	// Parse --scope (for accessibility)
	var scope string
	scope, remaining = parseFlag(remaining, "--scope")
	if scope != "" {
		mcpArgs["scope"] = scope
	}

	// Parse --last-n
	var lastN int
	var hasLastN bool
	lastN, hasLastN, remaining = parseFlagInt(remaining, "--last-n")
	if hasLastN {
		mcpArgs["last_n"] = lastN
	}

	// Parse --connection-id (for websocket_events)
	var connID string
	connID, remaining = parseFlag(remaining, "--connection-id")
	if connID != "" {
		mcpArgs["connection_id"] = connID
	}

	// Parse --direction (for websocket_events)
	var direction string
	direction, remaining = parseFlag(remaining, "--direction")
	if direction != "" {
		mcpArgs["direction"] = direction
	}

	_ = remaining

	return mcpArgs, nil
}
