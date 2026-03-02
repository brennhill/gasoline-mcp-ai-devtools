// Purpose: Tool-specific CLI flag-to-MCP argument mapping for interact actions.
// Why: Isolates interact parser contracts and validation from other tool parsers.
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
		"--direction":             {mcpKey: "direction", kind: flagString},
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
