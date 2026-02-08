// tool-handler.go — Tool handler for diff_sessions MCP tool.
// HandleTool function dispatches capture, compare, list, delete actions.
package session

import (
	"encoding/json"
	"fmt"
)

// diffSessionsParams defines the MCP tool input schema.
type diffSessionsParams struct {
	Action    string `json:"action"`
	Name      string `json:"name,omitempty"`
	CompareA  string `json:"compare_a,omitempty"`
	CompareB  string `json:"compare_b,omitempty"`
	URLFilter string `json:"url,omitempty"`
}

// HandleTool dispatches the diff_sessions MCP tool call.
func (sm *SessionManager) HandleTool(params json.RawMessage) (any, error) {
	var p diffSessionsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	switch p.Action {
	case "capture":
		if p.Name == "" {
			return nil, fmt.Errorf("'name' is required for capture action")
		}
		snap, err := sm.Capture(p.Name, p.URLFilter)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"action": "captured",
			"name":   snap.Name,
			"snapshot": map[string]any{
				"captured_at":      snap.CapturedAt,
				"console_errors":   len(snap.ConsoleErrors),
				"console_warnings": len(snap.ConsoleWarnings),
				"network_requests": len(snap.NetworkRequests),
				"page_url":         snap.PageURL,
			},
		}, nil

	case "compare":
		if p.CompareA == "" || p.CompareB == "" {
			return nil, fmt.Errorf("'compare_a' and 'compare_b' are required for compare action")
		}
		diff, err := sm.Compare(p.CompareA, p.CompareB)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"action":  "compared",
			"a":       diff.A,
			"b":       diff.B,
			"diff":    diff,
			"summary": diff.Summary,
		}, nil

	case "list":
		entries := sm.List()
		return map[string]any{
			"action":    "listed",
			"snapshots": entries,
		}, nil

	case "delete":
		if p.Name == "" {
			return nil, fmt.Errorf("'name' is required for delete action")
		}
		if err := sm.Delete(p.Name); err != nil {
			return nil, err
		}
		return map[string]any{
			"action": "deleted",
			"name":   p.Name,
		}, nil

	default:
		if p.Action == "" {
			return nil, fmt.Errorf("'action' is required (capture, compare, list, delete)")
		}
		return nil, fmt.Errorf("unknown action %q (valid: capture, compare, list, delete)", p.Action)
	}
}
