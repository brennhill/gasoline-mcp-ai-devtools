// Purpose: Tool-specific CLI flag-to-MCP argument mapping for generate/configure.
// Why: Keeps generate/configure parser contracts isolated from observe/analyze/interact parsers.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

func parseGenerateArgs(format string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": format}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		"--telemetry-mode":        {mcpKey: "telemetry_mode", kind: flagString},
		"--error-message":         {mcpKey: "error_message", kind: flagString},
		"--last-n":                {mcpKey: "last_n", kind: flagInt},
		"--base-url":              {mcpKey: "base_url", kind: flagString},
		"--include-screenshots":   {mcpKey: "include_screenshots", kind: flagBool},
		"--generate-fixtures":     {mcpKey: "generate_fixtures", kind: flagBool},
		"--visual-assertions":     {mcpKey: "visual_assertions", kind: flagBool},
		"--test-name":             {mcpKey: "test_name", kind: flagString},
		"--assert-network":        {mcpKey: "assert_network", kind: flagBool},
		"--assert-no-errors":      {mcpKey: "assert_no_errors", kind: flagBool},
		"--assert-response-shape": {mcpKey: "assert_response_shape", kind: flagBool},
		"--scope":                 {mcpKey: "scope", kind: flagString},
		"--include-passes":        {mcpKey: "include_passes", kind: flagBool},
		"--save-to":               {mcpKey: "save_to", kind: flagString},
		"--url":                   {mcpKey: "url", kind: flagString},
		"--method":                {mcpKey: "method", kind: flagString},
		"--status-min":            {mcpKey: "status_min", kind: flagInt},
		"--status-max":            {mcpKey: "status_max", kind: flagInt},
		"--mode":                  {mcpKey: "mode", kind: flagString},
		"--include-report-uri":    {mcpKey: "include_report_uri", kind: flagBool},
		"--exclude-origins":       {mcpKey: "exclude_origins", kind: flagStringList},
		"--resource-types":        {mcpKey: "resource_types", kind: flagStringList},
		"--origins":               {mcpKey: "origins", kind: flagStringList},
		"--annot-session":         {mcpKey: "annot_session", kind: flagString},
		"--context":               {mcpKey: "context", kind: flagString},
		"--action":                {mcpKey: "action", kind: flagString},
		"--test-file":             {mcpKey: "test_file", kind: flagString},
		"--test-dir":              {mcpKey: "test_dir", kind: flagString},
		"--broken-selectors":      {mcpKey: "broken_selectors", kind: flagStringList},
		"--auto-apply":            {mcpKey: "auto_apply", kind: flagBool},
		"--failure":               {mcpKey: "failure", kind: flagJSON},
		"--failures":              {mcpKey: "failures", kind: flagJSON},
		"--error-id":              {mcpKey: "error_id", kind: flagString},
		"--include-mocks":         {mcpKey: "include_mocks", kind: flagBool},
		"--output-format":         {mcpKey: "output_format", kind: flagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}

func parseConfigureArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": action}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		"--telemetry-mode":       {mcpKey: "telemetry_mode", kind: flagString},
		"--store-action":         {mcpKey: "store_action", kind: flagString},
		"--namespace":            {mcpKey: "namespace", kind: flagString},
		"--key":                  {mcpKey: "key", kind: flagString},
		"--data":                 {mcpKey: "data", kind: flagJSONOrString},
		"--noise-action":         {mcpKey: "noise_action", kind: flagString},
		"--rules":                {mcpKey: "rules", kind: flagJSON},
		"--rule-id":              {mcpKey: "rule_id", kind: flagString},
		"--pattern":              {mcpKey: "pattern", kind: flagString},
		"--selector":             {mcpKey: "selector", kind: flagString},
		"--category":             {mcpKey: "category", kind: flagString},
		"--reason":               {mcpKey: "reason", kind: flagString},
		"--buffer":               {mcpKey: "buffer", kind: flagString},
		"--tab-id":               {mcpKey: "tab_id", kind: flagInt},
		"--verif-session-action": {mcpKey: "verif_session_action", kind: flagString},
		"--name":                 {mcpKey: "name", kind: flagString},
		"--compare-a":            {mcpKey: "compare_a", kind: flagString},
		"--compare-b":            {mcpKey: "compare_b", kind: flagString},
		"--recording-id":         {mcpKey: "recording_id", kind: flagString},
		"--audit-session-id":     {mcpKey: "audit_session_id", kind: flagString},
		"--tool-name":            {mcpKey: "tool_name", kind: flagString},
		"--since":                {mcpKey: "since", kind: flagString},
		"--limit":                {mcpKey: "limit", kind: flagInt},
		"--streaming-action":     {mcpKey: "streaming_action", kind: flagString},
		"--events":               {mcpKey: "events", kind: flagStringList},
		"--throttle-seconds":     {mcpKey: "throttle_seconds", kind: flagInt},
		"--severity-min":         {mcpKey: "severity_min", kind: flagString},
		"--test-id":              {mcpKey: "test_id", kind: flagString},
		"--label":                {mcpKey: "label", kind: flagString},
		"--original-id":          {mcpKey: "original_id", kind: flagString},
		"--replay-id":            {mcpKey: "replay_id", kind: flagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}
