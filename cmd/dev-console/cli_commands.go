// Purpose: Owns cli_commands.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// cli_commands.go — Argument parsers for CLI mode.
// Maps CLI flags to MCP tool arguments for observe, analyze, generate, configure, interact.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// --- Generic flag parser ---

type cliFlagKind int

const (
	flagString cliFlagKind = iota
	flagInt
	flagBool
	flagStringList
	flagJSON
	flagJSONOrString
	flagIntOrString
)

type cliFlagSpec struct {
	mcpKey string
	kind   cliFlagKind
}

func parseFlagsBySpec(args []string, specs map[string]cliFlagSpec) (map[string]any, error) {
	out := make(map[string]any)
	for i := 0; i < len(args); i++ {
		flag := args[i]
		spec, ok := specs[flag]
		if !ok {
			return nil, fmt.Errorf("unknown flag: %s", flag)
		}
		switch spec.kind {
		case flagBool:
			out[spec.mcpKey] = true
		case flagString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.mcpKey] = val
			i = next
		case flagInt:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			n, err := parseIntValue(val)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.mcpKey] = n
			i = next
		case flagStringList:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.mcpKey] = parseCSVList(val)
			i = next
		case flagJSON:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			var parsed any
			if err := json.Unmarshal([]byte(val), &parsed); err != nil {
				return nil, fmt.Errorf("%s: invalid JSON: %w", flag, err)
			}
			out[spec.mcpKey] = parsed
			i = next
		case flagJSONOrString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.mcpKey] = parseJSONOrString(val)
			i = next
		case flagIntOrString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			if n, err := strconv.Atoi(val); err == nil {
				out[spec.mcpKey] = n
			} else {
				out[spec.mcpKey] = val
			}
			i = next
		default:
			return nil, fmt.Errorf("unsupported parser kind for %s", flag)
		}
	}
	return out, nil
}

func requireFlagValue(args []string, idx int) (string, int, error) {
	next := idx + 1
	if next >= len(args) {
		return "", idx, fmt.Errorf("missing value")
	}
	val := args[next]
	if strings.HasPrefix(val, "--") {
		return "", idx, fmt.Errorf("missing value")
	}
	return val, next, nil
}

func parseIntValue(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q", s)
	}
	return n, nil
}

func parseCSVList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

// --- Tool parsers ---

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
		"--session":             {mcpKey: "session", kind: flagString},
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

func parseGenerateArgs(format string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"format": format}
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
		"--session":               {mcpKey: "session", kind: flagString},
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
	mcpArgs := map[string]any{"action": action}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		"--telemetry-mode":         {mcpKey: "telemetry_mode", kind: flagString},
		"--store-action":           {mcpKey: "store_action", kind: flagString},
		"--namespace":              {mcpKey: "namespace", kind: flagString},
		"--key":                    {mcpKey: "key", kind: flagString},
		"--data":                   {mcpKey: "data", kind: flagJSONOrString},
		"--noise-action":           {mcpKey: "noise_action", kind: flagString},
		"--rules":                  {mcpKey: "rules", kind: flagJSON},
		"--rule-id":                {mcpKey: "rule_id", kind: flagString},
		"--pattern":                {mcpKey: "pattern", kind: flagString},
		"--selector":               {mcpKey: "selector", kind: flagString},
		"--category":               {mcpKey: "category", kind: flagString},
		"--reason":                 {mcpKey: "reason", kind: flagString},
		"--buffer":                 {mcpKey: "buffer", kind: flagString},
		"--tab-id":                 {mcpKey: "tab_id", kind: flagInt},
		"--session-action":         {mcpKey: "session_action", kind: flagString},
		"--name":                   {mcpKey: "name", kind: flagString},
		"--compare-a":              {mcpKey: "compare_a", kind: flagString},
		"--compare-b":              {mcpKey: "compare_b", kind: flagString},
		"--recording-id":           {mcpKey: "recording_id", kind: flagString},
		"--session-id":             {mcpKey: "session_id", kind: flagString},
		"--tool-name":              {mcpKey: "tool_name", kind: flagString},
		"--since":                  {mcpKey: "since", kind: flagString},
		"--limit":                  {mcpKey: "limit", kind: flagInt},
		"--url":                    {mcpKey: "url", kind: flagString},
		"--operation":              {mcpKey: "operation", kind: flagString},
		"--streaming-action":       {mcpKey: "streaming_action", kind: flagString},
		"--events":                 {mcpKey: "events", kind: flagStringList},
		"--throttle-seconds":       {mcpKey: "throttle_seconds", kind: flagInt},
		"--severity-min":           {mcpKey: "severity_min", kind: flagString},
		"--test-id":                {mcpKey: "test_id", kind: flagString},
		"--label":                  {mcpKey: "label", kind: flagString},
		"--sensitive-data-enabled": {mcpKey: "sensitive_data_enabled", kind: flagBool},
		"--original-id":            {mcpKey: "original_id", kind: flagString},
		"--replay-id":              {mcpKey: "replay_id", kind: flagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}

// parseJSONOrString attempts to parse s as JSON object; returns the raw string on failure.
func parseJSONOrString(s string) any {
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err == nil {
		return parsed
	}
	return s
}

// interactActionsRequiringSelector lists actions that need --selector.
var interactActionsRequiringSelector = map[string]bool{
	"click":         true,
	"type":          true,
	"select":        true,
	"key_press":     true,
	"get_text":      true,
	"get_value":     true,
	"get_attribute": true,
	"set_attribute": true,
	"wait_for":      true,
	"scroll_to":     true,
	"focus":         true,
	"check":         true,
	"paste":         true,
	"highlight":     true,
}

// interactActionsAllowingIndex lists selector-required actions where --index can be used instead.
var interactActionsAllowingIndex = map[string]bool{
	"click":         true,
	"type":          true,
	"select":        true,
	"check":         true,
	"get_text":      true,
	"get_value":     true,
	"get_attribute": true,
	"set_attribute": true,
	"wait_for":      true,
	"scroll_to":     true,
	"focus":         true,
	"key_press":     true,
	"paste":         true,
}

func parseInteractArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"action": action}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		"--telemetry-mode":        {mcpKey: "telemetry_mode", kind: flagString},
		"--sync":                  {mcpKey: "sync", kind: flagBool},
		"--wait":                  {mcpKey: "wait", kind: flagBool},
		"--background":            {mcpKey: "background", kind: flagBool},
		"--selector":              {mcpKey: "selector", kind: flagString},
		"--index":                 {mcpKey: "index", kind: flagInt},
		"--visible-only":          {mcpKey: "visible_only", kind: flagBool},
		"--frame":                 {mcpKey: "frame", kind: flagIntOrString},
		"--duration-ms":           {mcpKey: "duration_ms", kind: flagInt},
		"--snapshot-name":         {mcpKey: "snapshot_name", kind: flagString},
		"--include-url":           {mcpKey: "include_url", kind: flagBool},
		"--include-content":       {mcpKey: "include_content", kind: flagBool},
		"--script":                {mcpKey: "script", kind: flagString},
		"--timeout-ms":            {mcpKey: "timeout_ms", kind: flagInt},
		"--text":                  {mcpKey: "text", kind: flagString},
		"--subtitle":              {mcpKey: "subtitle", kind: flagString},
		"--value":                 {mcpKey: "value", kind: flagString},
		"--clear":                 {mcpKey: "clear", kind: flagBool},
		"--checked":               {mcpKey: "checked", kind: flagBool},
		"--name":                  {mcpKey: "name", kind: flagString},
		"--audio":                 {mcpKey: "audio", kind: flagString},
		"--fps":                   {mcpKey: "fps", kind: flagInt},
		"--world":                 {mcpKey: "world", kind: flagString},
		"--url":                   {mcpKey: "url", kind: flagString},
		"--tab-id":                {mcpKey: "tab_id", kind: flagInt},
		"--reason":                {mcpKey: "reason", kind: flagString},
		"--correlation-id":        {mcpKey: "correlation_id", kind: flagString},
		"--analyze":               {mcpKey: "analyze", kind: flagBool},
		"--session":               {mcpKey: "session", kind: flagString},
		"--file-path":             {mcpKey: "file_path", kind: flagString},
		"--api-endpoint":          {mcpKey: "api_endpoint", kind: flagString},
		"--submit":                {mcpKey: "submit", kind: flagBool},
		"--escalation-timeout-ms": {mcpKey: "escalation_timeout_ms", kind: flagInt},
		"--fields":                {mcpKey: "fields", kind: flagJSON},
		"--submit-selector":       {mcpKey: "submit_selector", kind: flagString},
		"--submit-index":          {mcpKey: "submit_index", kind: flagInt},
		"--wait-for":              {mcpKey: "wait_for", kind: flagString},
		"--save-to":               {mcpKey: "save_to", kind: flagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	parseInteractFilePath(mcpArgs)

	return mcpArgs, validateInteractArgs(action, mcpArgs)
}

// parseInteractFilePath extracts --file-path and resolves relative paths to absolute.
func parseInteractFilePath(mcpArgs map[string]any) {
	filePath, _ := mcpArgs["file_path"].(string)
	if filePath == "" {
		return
	}
	if !filepath.IsAbs(filePath) {
		if cwd, err := os.Getwd(); err == nil {
			filePath = filepath.Join(cwd, filePath)
		}
	}
	mcpArgs["file_path"] = filePath
}

// validateInteractArgs checks required fields for specific interact actions.
func validateInteractArgs(action string, mcpArgs map[string]any) error {
	selector, _ := mcpArgs["selector"].(string)
	_, hasIndex := mcpArgs["index"]
	if interactActionsRequiringSelector[action] && selector == "" {
		if !interactActionsAllowingIndex[action] || !hasIndex {
			return fmt.Errorf("interact %s: --selector is required", action)
		}
	}
	if action == "upload" && selector == "" && mcpArgs["api_endpoint"] == nil {
		return fmt.Errorf("interact upload: --selector or --api-endpoint is required")
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
