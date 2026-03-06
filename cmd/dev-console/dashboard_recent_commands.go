// Purpose: Parses and compacts recent MCP HTTP debug entries for dashboard display.
// Why: Keeps command-summary shaping separate from dashboard route handlers.

package main

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// recentCommand is a simplified view of an HTTP debug entry for the dashboard.
type recentCommand struct {
	Timestamp  time.Time `json:"timestamp"`
	Tool       string    `json:"tool"`
	Params     string    `json:"params"`
	Status     int       `json:"status"`
	DurationMs int64     `json:"duration_ms"`
}

// buildRecentCommands filters and sorts HTTP debug entries for the dashboard.
// Returns the most recent entries (newest first), excluding empty circular buffer slots.
func buildRecentCommands(entries []capture.HTTPDebugEntry) []recentCommand {
	var result []recentCommand
	for _, e := range entries {
		if e.Timestamp.IsZero() {
			continue
		}
		tool, params := parseMCPCommand(e.RequestBody)
		result = append(result, recentCommand{
			Timestamp:  e.Timestamp,
			Tool:       tool,
			Params:     params,
			Status:     e.ResponseStatus,
			DurationMs: e.DurationMs,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if len(result) > 15 {
		result = result[:15]
	}
	return result
}

// parseMCPCommand extracts the tool name and key parameters from a JSON-RPC request body.
// Returns (tool, params) where params is a compact summary like "what=errors" or "action=navigate url=example.com".
func parseMCPCommand(body string) (string, string) {
	if body == "" {
		return "unknown", ""
	}

	// Minimal JSON-RPC parsing without allocating a full struct.
	// Request shape: {"method":"tools/call","params":{"name":"observe","arguments":{...}}}
	var req struct {
		Method string `json:"method"`
		Params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return "unknown", ""
	}

	// Non-tool-call methods (initialize, notifications, etc.)
	if req.Method != "tools/call" || req.Params.Name == "" {
		return req.Method, ""
	}

	tool := req.Params.Name
	args := req.Params.Arguments
	if len(args) == 0 {
		return tool, ""
	}

	// Extract the primary key parameter for each tool, plus secondary context.
	var parts []string
	switch tool {
	case "observe":
		appendParam(&parts, args, "what")
	case "interact":
		appendParam(&parts, args, "what")
		appendParam(&parts, args, "url")
		appendParam(&parts, args, "selector")
	case "analyze":
		appendParam(&parts, args, "what")
		appendParam(&parts, args, "selector")
	case "generate":
		appendParam(&parts, args, "what")
	case "configure":
		appendParam(&parts, args, "what")
		appendParam(&parts, args, "buffer")
		appendParam(&parts, args, "noise_action")
	default:
		// Generic: show first string-valued key
		for k, v := range args {
			if s, ok := v.(string); ok && s != "" {
				parts = append(parts, k+"="+truncateDashParam(s))
				break
			}
		}
	}

	return tool, strings.Join(parts, " ")
}

// appendParam adds "key=value" to parts if the key exists in args with a non-empty string value.
func appendParam(parts *[]string, args map[string]any, key string) {
	v, ok := args[key]
	if !ok {
		return
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return
	}
	*parts = append(*parts, key+"="+truncateDashParam(s))
}

// truncateDashParam shortens a parameter value for dashboard display.
func truncateDashParam(s string) string {
	if len(s) > 40 {
		return s[:37] + "..."
	}
	return s
}
