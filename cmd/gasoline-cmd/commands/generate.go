// generate.go — CLI argument parser for the generate tool.
// Maps CLI flags to MCP generate tool arguments.
package commands

// GenerateArgs parses CLI args for the generate tool and returns MCP arguments.
func GenerateArgs(format string, args []string) (map[string]any, error) {
	format = NormalizeAction(format)
	mcpArgs := map[string]any{
		"format": format,
	}

	remaining := args

	// Parse --test-name (for test)
	var testName string
	testName, remaining = parseFlag(remaining, "--test-name")
	if testName != "" {
		mcpArgs["test_name"] = testName
	}

	// Parse --error-message (for reproduction)
	var errorMsg string
	errorMsg, remaining = parseFlag(remaining, "--error-message")
	if errorMsg != "" {
		mcpArgs["error_message"] = errorMsg
	}

	// Parse --url (for har)
	var url string
	url, remaining = parseFlag(remaining, "--url")
	if url != "" {
		mcpArgs["url"] = url
	}

	// Parse --method (for har)
	var method string
	method, remaining = parseFlag(remaining, "--method")
	if method != "" {
		mcpArgs["method"] = method
	}

	// Parse --mode (for csp: strict, moderate, report_only)
	var mode string
	mode, remaining = parseFlag(remaining, "--mode")
	if mode != "" {
		mcpArgs["mode"] = mode
	}

	// Parse --save-to (for sarif, har)
	var saveTo string
	saveTo, remaining = parseFlag(remaining, "--save-to")
	if saveTo != "" {
		mcpArgs["save_to"] = saveTo
	}

	// Parse --status-min (for har)
	var statusMin int
	var hasStatusMin bool
	statusMin, hasStatusMin, remaining = parseFlagInt(remaining, "--status-min")
	if hasStatusMin {
		mcpArgs["status_min"] = statusMin
	}

	// Parse --status-max (for har)
	var statusMax int
	var hasStatusMax bool
	statusMax, hasStatusMax, remaining = parseFlagInt(remaining, "--status-max")
	if hasStatusMax {
		mcpArgs["status_max"] = statusMax
	}

	// Parse --base-url (for reproduction, test)
	var baseURL string
	baseURL, remaining = parseFlag(remaining, "--base-url")
	if baseURL != "" {
		mcpArgs["base_url"] = baseURL
	}

	// Parse --last-n (for reproduction)
	var lastN int
	var hasLastN bool
	lastN, hasLastN, remaining = parseFlagInt(remaining, "--last-n")
	if hasLastN {
		mcpArgs["last_n"] = lastN
	}

	// Parse boolean flags
	var assertNetwork bool
	assertNetwork, remaining = parseFlagBool(remaining, "--assert-network")
	if assertNetwork {
		mcpArgs["assert_network"] = true
	}

	var assertNoErrors bool
	assertNoErrors, remaining = parseFlagBool(remaining, "--assert-no-errors")
	if assertNoErrors {
		mcpArgs["assert_no_errors"] = true
	}

	var includeScreenshots bool
	includeScreenshots, remaining = parseFlagBool(remaining, "--include-screenshots")
	if includeScreenshots {
		mcpArgs["include_screenshots"] = true
	}

	_ = remaining

	return mcpArgs, nil
}
