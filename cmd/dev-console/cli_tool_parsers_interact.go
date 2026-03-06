// Purpose: Tool-specific CLI flag-to-MCP argument mapping for interact actions.
// Why: Isolates interact parser contracts and validation from other tool parsers.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// interactActionsRequiringTarget lists actions that need at least one targeting param
// (--selector, --element-id, --index, or --x/--y for click).
var interactActionsRequiringTarget = map[string]bool{
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

func parseInteractArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": action}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		// Cross-cutting
		"--telemetry-mode":        {mcpKey: "telemetry_mode", kind: flagString},
		"--background":            {mcpKey: "background", kind: flagBool},
		// Element targeting
		"--selector":              {mcpKey: "selector", kind: flagString},
		"--element-id":            {mcpKey: "element_id", kind: flagString},
		"--index":                 {mcpKey: "index", kind: flagInt},
		"--index-generation":      {mcpKey: "index_generation", kind: flagString},
		"--nth":                   {mcpKey: "nth", kind: flagInt},
		"--scope-selector":        {mcpKey: "scope_selector", kind: flagString},
		"--scope-rect":            {mcpKey: "scope_rect", kind: flagJSON},
		"--frame":                 {mcpKey: "frame", kind: flagIntOrString},
		"--x":                     {mcpKey: "x", kind: flagInt},
		"--y":                     {mcpKey: "y", kind: flagInt},
		// List/query filters
		"--visible-only":          {mcpKey: "visible_only", kind: flagBool},
		"--limit":                 {mcpKey: "limit", kind: flagInt},
		"--text-contains":         {mcpKey: "text_contains", kind: flagString},
		"--role":                  {mcpKey: "role", kind: flagString},
		"--exclude-nav":           {mcpKey: "exclude_nav", kind: flagBool},
		"--query-type":            {mcpKey: "query_type", kind: flagString},
		"--attribute-names":       {mcpKey: "attribute_names", kind: flagStringList},
		// Core action params
		"--text":                  {mcpKey: "text", kind: flagString},
		"--value":                 {mcpKey: "value", kind: flagString},
		"--name":                  {mcpKey: "name", kind: flagString},
		"--clear":                 {mcpKey: "clear", kind: flagBool},
		"--checked":               {mcpKey: "checked", kind: flagBool},
		"--direction":             {mcpKey: "direction", kind: flagString},
		"--structured":            {mcpKey: "structured", kind: flagBool},
		"--script":                {mcpKey: "script", kind: flagString},
		"--world":                 {mcpKey: "world", kind: flagString},
		"--timeout-ms":            {mcpKey: "timeout_ms", kind: flagInt},
		"--duration-ms":           {mcpKey: "duration_ms", kind: flagInt},
		"--subtitle":              {mcpKey: "subtitle", kind: flagString},
		// Navigation
		"--url":                   {mcpKey: "url", kind: flagString},
		"--tab-id":                {mcpKey: "tab_id", kind: flagInt},
		"--tab-index":             {mcpKey: "tab_index", kind: flagInt},
		"--set-tracked":           {mcpKey: "set_tracked", kind: flagBool},
		"--new-tab":               {mcpKey: "new_tab", kind: flagBool},
		"--include-content":       {mcpKey: "include_content", kind: flagBool},
		"--analyze":               {mcpKey: "analyze", kind: flagBool},
		// Wait / stability
		"--wait-for":              {mcpKey: "wait_for", kind: flagString},
		"--url-contains":          {mcpKey: "url_contains", kind: flagString},
		"--absent":                {mcpKey: "absent", kind: flagBool},
		"--wait-for-stable":       {mcpKey: "wait_for_stable", kind: flagBool},
		"--wait-for-url-change":   {mcpKey: "wait_for_url_change", kind: flagBool},
		"--stability-ms":          {mcpKey: "stability_ms", kind: flagInt},
		"--auto-dismiss":          {mcpKey: "auto_dismiss", kind: flagBool},
		// Output enrichments
		"--include-screenshot":    {mcpKey: "include_screenshot", kind: flagBool},
		"--include-interactive":   {mcpKey: "include_interactive", kind: flagBool},
		"--observe-mutations":     {mcpKey: "observe_mutations", kind: flagBool},
		"--action-diff":           {mcpKey: "action_diff", kind: flagBool},
		"--evidence":              {mcpKey: "evidence", kind: flagString},
		"--reason":                {mcpKey: "reason", kind: flagString},
		"--correlation-id":        {mcpKey: "correlation_id", kind: flagString},
		// State management
		"--snapshot-name":         {mcpKey: "snapshot_name", kind: flagString},
		"--include-url":           {mcpKey: "include_url", kind: flagBool},
		"--storage-type":          {mcpKey: "storage_type", kind: flagString},
		"--key":                   {mcpKey: "key", kind: flagString},
		"--domain":                {mcpKey: "domain", kind: flagString},
		"--path":                  {mcpKey: "path", kind: flagString},
		// Form filling
		"--fields":                {mcpKey: "fields", kind: flagJSON},
		"--submit-selector":       {mcpKey: "submit_selector", kind: flagString},
		"--submit-index":          {mcpKey: "submit_index", kind: flagInt},
		// Recording
		"--audio":                 {mcpKey: "audio", kind: flagString},
		"--fps":                   {mcpKey: "fps", kind: flagInt},
		"--annot-session":         {mcpKey: "annot_session", kind: flagString},
		// Upload
		"--file-path":             {mcpKey: "file_path", kind: flagString},
		"--api-endpoint":          {mcpKey: "api_endpoint", kind: flagString},
		"--submit":                {mcpKey: "submit", kind: flagBool},
		"--escalation-timeout-ms": {mcpKey: "escalation_timeout_ms", kind: flagInt},
		// Batch
		"--steps":                 {mcpKey: "steps", kind: flagJSON},
		"--step-timeout-ms":       {mcpKey: "step_timeout_ms", kind: flagInt},
		"--continue-on-error":     {mcpKey: "continue_on_error", kind: flagBool},
		"--stop-after-step":       {mcpKey: "stop_after_step", kind: flagInt},
		// Save output
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
	if interactActionsRequiringTarget[action] && !hasTargetingParam(mcpArgs) {
		return fmt.Errorf("interact %s: requires a targeting param (--selector, --element-id, --index, or --x/--y)", action)
	}
	if action == "upload" && mcpArgs["selector"] == nil && mcpArgs["element_id"] == nil && mcpArgs["api_endpoint"] == nil {
		return fmt.Errorf("interact upload: --selector, --element-id, or --api-endpoint is required")
	}
	if action == "navigate" && mcpArgs["url"] == nil {
		return fmt.Errorf("interact navigate: --url is required")
	}
	if action == "execute_js" && mcpArgs["script"] == nil {
		return fmt.Errorf("interact execute_js: --script is required")
	}
	return nil
}

// hasTargetingParam checks if at least one element targeting param is present.
func hasTargetingParam(mcpArgs map[string]any) bool {
	for _, key := range []string{"selector", "element_id", "index", "x", "y"} {
		if mcpArgs[key] != nil {
			return true
		}
	}
	return false
}
