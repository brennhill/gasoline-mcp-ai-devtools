// timeout.go — Per-request timeout logic for MCP tool calls.
package bridge

import (
	"encoding/json"
	"time"
)

// Timeout constants for different tool categories.
const (
	FastTimeout    = 10 * time.Second
	SlowTimeout    = 35 * time.Second
	BlockingPoll   = 65 * time.Second
)

// ToolCallTimeout returns the per-request timeout based on the MCP method and tool name.
// Fast tools (observe, generate, configure, resources/read) get 10s; slow tools
// (analyze, interact) that round-trip to the extension get 35s.
// Annotation observe (observe command_result for ann_*) gets 65s for blocking poll.
//
// method is the JSON-RPC method (e.g. "tools/call", "resources/read").
// params is the raw JSON of the request params.
func ToolCallTimeout(method string, params json.RawMessage) time.Duration {
	if method == "resources/read" {
		return FastTimeout
	}
	if method != "tools/call" {
		return FastTimeout
	}

	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(params, &p) != nil {
		return FastTimeout
	}

	switch p.Name {
	case "analyze", "interact":
		return SlowTimeout
	case "observe":
		var args struct {
			What          string `json:"what"`
			CorrelationID string `json:"correlation_id"`
		}
		if json.Unmarshal(p.Arguments, &args) == nil {
			if args.What == "command_result" &&
				len(args.CorrelationID) > 4 && args.CorrelationID[:4] == "ann_" {
				return BlockingPoll
			}
			if args.What == "screenshot" {
				return SlowTimeout
			}
		}
		return FastTimeout
	default:
		return FastTimeout
	}
}

// ExtractToolAction extracts the tool name and action parameter from a tools/call request.
// Returns empty strings for non-tools/call methods or if parsing fails.
func ExtractToolAction(method string, params json.RawMessage) (toolName, action string) {
	if method != "tools/call" {
		return "", ""
	}
	var p struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(params, &p) != nil {
		return "", ""
	}
	var a struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal(p.Args, &a)
	return p.Name, a.Action
}
