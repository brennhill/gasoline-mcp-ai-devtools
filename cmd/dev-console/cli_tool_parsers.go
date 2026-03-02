// Purpose: Tool-specific CLI flag-to-MCP argument mapping.
// Why: Separates per-tool argument contracts from shared parsing infrastructure.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

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
	"paste":         true,
	"highlight":     true,
}

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

func parseInteractArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": action}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		"--telemetry-mode":        {mcpKey: "telemetry_mode", kind: flagString},
		"--sync":                  {mcpKey: "sync", kind: flagBool},
		"--wait":                  {mcpKey: "wait", kind: flagBool},
		"--background":            {mcpKey: "background", kind: flagBool},
		"--selector":              {mcpKey: "selector", kind: flagString},
		"--frame":                 {mcpKey: "frame", kind: flagIntOrString},
		"--duration-ms":           {mcpKey: "duration_ms", kind: flagInt},
		"--snapshot-name":         {mcpKey: "snapshot_name", kind: flagString},
		"--include-url":           {mcpKey: "include_url", kind: flagBool},
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
		"--annot-session":         {mcpKey: "annot_session", kind: flagString},
		"--file-path":             {mcpKey: "file_path", kind: flagString},
		"--api-endpoint":          {mcpKey: "api_endpoint", kind: flagString},
		"--submit":                {mcpKey: "submit", kind: flagBool},
		"--escalation-timeout-ms": {mcpKey: "escalation_timeout_ms", kind: flagInt},
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
	if interactActionsRequiringSelector[action] && selector == "" {
		return fmt.Errorf("interact %s: --selector is required", action)
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
