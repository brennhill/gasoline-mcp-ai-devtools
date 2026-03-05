// Purpose: Tool-specific CLI flag-to-MCP argument mapping for observe/analyze.
// Why: Keeps observe/analyze parser contracts isolated from other tool parser families.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

func parseObserveArgs(mode string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": mode}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		// Cross-cutting
		"--telemetry-mode":         {mcpKey: "telemetry_mode", kind: flagString},
		"--limit":                  {mcpKey: "limit", kind: flagInt},
		"--summary":                {mcpKey: "summary", kind: flagBool},
		"--scope":                  {mcpKey: "scope", kind: flagString},
		// Pagination
		"--after-cursor":           {mcpKey: "after_cursor", kind: flagString},
		"--before-cursor":          {mcpKey: "before_cursor", kind: flagString},
		"--since-cursor":           {mcpKey: "since_cursor", kind: flagString},
		"--restart-on-eviction":    {mcpKey: "restart_on_eviction", kind: flagBool},
		// Filtering
		"--min-level":              {mcpKey: "min_level", kind: flagString},
		"--source":                 {mcpKey: "source", kind: flagString},
		"--url":                    {mcpKey: "url", kind: flagString},
		"--method":                 {mcpKey: "method", kind: flagString},
		"--status-min":             {mcpKey: "status_min", kind: flagInt},
		"--status-max":             {mcpKey: "status_max", kind: flagInt},
		"--body-path":              {mcpKey: "body_path", kind: flagString},
		"--connection-id":          {mcpKey: "connection_id", kind: flagString},
		"--direction":              {mcpKey: "direction", kind: flagString},
		"--last-n":                 {mcpKey: "last_n", kind: flagInt},
		"--include":                {mcpKey: "include", kind: flagStringList},
		"--correlation-id":         {mcpKey: "correlation_id", kind: flagString},
		"--recording-id":           {mcpKey: "recording_id", kind: flagString},
		"--window-seconds":         {mcpKey: "window_seconds", kind: flagInt},
		"--original-id":            {mcpKey: "original_id", kind: flagString},
		"--replay-id":              {mcpKey: "replay_id", kind: flagString},
		// Log detail
		"--include-internal":       {mcpKey: "include_internal", kind: flagBool},
		"--include-extension-logs": {mcpKey: "include_extension_logs", kind: flagBool},
		"--extension-limit":        {mcpKey: "extension_limit", kind: flagInt},
		"--min-group-size":         {mcpKey: "min_group_size", kind: flagInt},
		// Screenshot
		"--format":                 {mcpKey: "format", kind: flagString},
		"--quality":                {mcpKey: "quality", kind: flagInt},
		"--full-page":              {mcpKey: "full_page", kind: flagBool},
		"--selector":               {mcpKey: "selector", kind: flagString},
		"--wait-for-stable":        {mcpKey: "wait_for_stable", kind: flagBool},
		"--save-to":                {mcpKey: "save_to", kind: flagString},
		// Storage / IndexedDB
		"--storage-type":           {mcpKey: "storage_type", kind: flagString},
		"--key":                    {mcpKey: "key", kind: flagString},
		"--database":               {mcpKey: "database", kind: flagString},
		"--store":                  {mcpKey: "store", kind: flagString},
		// Transients / Page inventory
		"--classification":         {mcpKey: "classification", kind: flagString},
		"--visible-only":           {mcpKey: "visible_only", kind: flagBool},
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
		// Cross-cutting
		"--telemetry-mode":      {mcpKey: "telemetry_mode", kind: flagString},
		"--background":          {mcpKey: "background", kind: flagBool},
		"--summary":             {mcpKey: "summary", kind: flagBool},
		// Element targeting
		"--selector":            {mcpKey: "selector", kind: flagString},
		"--frame":               {mcpKey: "frame", kind: flagIntOrString},
		"--tab-id":              {mcpKey: "tab_id", kind: flagInt},
		// Analysis control
		"--operation":           {mcpKey: "operation", kind: flagString},
		"--ignore-endpoints":    {mcpKey: "ignore_endpoints", kind: flagStringList},
		"--scope":               {mcpKey: "scope", kind: flagString},
		"--tags":                {mcpKey: "tags", kind: flagStringList},
		"--force-refresh":       {mcpKey: "force_refresh", kind: flagBool},
		"--domain":              {mcpKey: "domain", kind: flagString},
		"--timeout-ms":          {mcpKey: "timeout_ms", kind: flagInt},
		"--world":               {mcpKey: "world", kind: flagString},
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
		// Annotation URL filtering
		"--url":                 {mcpKey: "url", kind: flagString},
		"--url-pattern":         {mcpKey: "url_pattern", kind: flagString},
		// Data table
		"--max-rows":            {mcpKey: "max_rows", kind: flagInt},
		"--max-cols":            {mcpKey: "max_cols", kind: flagInt},
		// Visual regression
		"--name":                {mcpKey: "name", kind: flagString},
		"--baseline":            {mcpKey: "baseline", kind: flagString},
		"--threshold":           {mcpKey: "threshold", kind: flagInt},
		// Audit
		"--categories":          {mcpKey: "categories", kind: flagStringList},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}
