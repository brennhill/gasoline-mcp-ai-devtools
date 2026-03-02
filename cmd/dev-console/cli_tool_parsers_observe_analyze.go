// Purpose: Tool-specific CLI flag-to-MCP argument mapping for observe/analyze.
// Why: Keeps observe/analyze parser contracts isolated from other tool parser families.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

func parseObserveArgs(mode string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": mode}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		"--telemetry-mode":      {mcpKey: "telemetry_mode", kind: flagString},
		"--limit":               {mcpKey: "limit", kind: flagInt},
		"--after-cursor":        {mcpKey: "after_cursor", kind: flagString},
		"--before-cursor":       {mcpKey: "before_cursor", kind: flagString},
		"--since-cursor":        {mcpKey: "since_cursor", kind: flagString},
		"--restart-on-eviction": {mcpKey: "restart_on_eviction", kind: flagBool},
		"--min-level":           {mcpKey: "min_level", kind: flagString},
		"--level":               {mcpKey: "level", kind: flagString},
		"--source":              {mcpKey: "source", kind: flagString},
		"--url":                 {mcpKey: "url", kind: flagString},
		"--method":              {mcpKey: "method", kind: flagString},
		"--scope":               {mcpKey: "scope", kind: flagString},
		"--status-min":          {mcpKey: "status_min", kind: flagInt},
		"--status-max":          {mcpKey: "status_max", kind: flagInt},
		"--body-key":            {mcpKey: "body_key", kind: flagString},
		"--body-path":           {mcpKey: "body_path", kind: flagString},
		"--connection-id":       {mcpKey: "connection_id", kind: flagString},
		"--direction":           {mcpKey: "direction", kind: flagString},
		"--last-n":              {mcpKey: "last_n", kind: flagInt},
		"--include":             {mcpKey: "include", kind: flagStringList},
		"--correlation-id":      {mcpKey: "correlation_id", kind: flagString},
		"--recording-id":        {mcpKey: "recording_id", kind: flagString},
		"--window-seconds":      {mcpKey: "window_seconds", kind: flagInt},
		"--original-id":         {mcpKey: "original_id", kind: flagString},
		"--replay-id":           {mcpKey: "replay_id", kind: flagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}

func parseAnalyzeArgs(what string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": what}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		"--telemetry-mode":      {mcpKey: "telemetry_mode", kind: flagString},
		"--selector":            {mcpKey: "selector", kind: flagString},
		"--frame":               {mcpKey: "frame", kind: flagIntOrString},
		"--sync":                {mcpKey: "sync", kind: flagBool},
		"--wait":                {mcpKey: "wait", kind: flagBool},
		"--background":          {mcpKey: "background", kind: flagBool},
		"--operation":           {mcpKey: "operation", kind: flagString},
		"--ignore-endpoints":    {mcpKey: "ignore_endpoints", kind: flagStringList},
		"--scope":               {mcpKey: "scope", kind: flagString},
		"--tags":                {mcpKey: "tags", kind: flagStringList},
		"--force-refresh":       {mcpKey: "force_refresh", kind: flagBool},
		"--domain":              {mcpKey: "domain", kind: flagString},
		"--timeout-ms":          {mcpKey: "timeout_ms", kind: flagInt},
		"--world":               {mcpKey: "world", kind: flagString},
		"--tab-id":              {mcpKey: "tab_id", kind: flagInt},
		"--max-workers":         {mcpKey: "max_workers", kind: flagInt},
		"--checks":              {mcpKey: "checks", kind: flagStringList},
		"--severity-min":        {mcpKey: "severity_min", kind: flagString},
		"--first-party-origins": {mcpKey: "first_party_origins", kind: flagStringList},
		"--include-static":      {mcpKey: "include_static", kind: flagBool},
		"--custom-lists":        {mcpKey: "custom_lists", kind: flagJSON},
		"--correlation-id":      {mcpKey: "correlation_id", kind: flagString},
		"--annot-session":       {mcpKey: "annot_session", kind: flagString},
		"--urls":                {mcpKey: "urls", kind: flagStringList},
		"--file":                {mcpKey: "file", kind: flagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}
