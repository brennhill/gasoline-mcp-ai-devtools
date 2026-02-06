// verify_compute.go — Verification computation and MCP tool dispatch.
// Contains: computeVerification, determineVerdict, HandleTool.
package session

import (
	"encoding/json"
	"fmt"
)

// ============================================
// Verification Computation
// ============================================

// computeVerification compares baseline and after snapshots
func (vm *VerificationManager) computeVerification(before, after *SessionSnapshot) VerificationResult {
	result := VerificationResult{
		Changes:   make([]VerifyChange, 0),
		NewIssues: make([]VerifyChange, 0),
	}

	// Count totals
	beforeConsoleCount := 0
	for _, e := range before.ConsoleErrors {
		beforeConsoleCount += e.Count
	}
	afterConsoleCount := 0
	for _, e := range after.ConsoleErrors {
		afterConsoleCount += e.Count
	}

	result.Before = IssueSummary{
		ConsoleErrors: beforeConsoleCount,
		NetworkErrors: len(before.NetworkErrors),
		TotalIssues:   beforeConsoleCount + len(before.NetworkErrors),
	}
	result.After = IssueSummary{
		ConsoleErrors: afterConsoleCount,
		NetworkErrors: len(after.NetworkErrors),
		TotalIssues:   afterConsoleCount + len(after.NetworkErrors),
	}

	// Compare console errors by normalized message
	beforeMsgs := make(map[string]VerifyError)
	for _, e := range before.ConsoleErrors {
		beforeMsgs[e.Normalized] = e
	}

	afterMsgs := make(map[string]VerifyError)
	for _, e := range after.ConsoleErrors {
		afterMsgs[e.Normalized] = e
	}

	// Resolved = in before but not in after
	for norm, e := range beforeMsgs {
		if _, found := afterMsgs[norm]; !found {
			suffix := ""
			if e.Count > 1 {
				suffix = fmt.Sprintf(" (x%d)", e.Count)
			}
			result.Changes = append(result.Changes, VerifyChange{
				Type:     "resolved",
				Category: "console",
				Before:   e.Message + suffix,
				After:    "(not seen)",
			})
		}
	}

	// New = in after but not in before
	for norm, e := range afterMsgs {
		if _, found := beforeMsgs[norm]; !found {
			result.NewIssues = append(result.NewIssues, VerifyChange{
				Type:     "new",
				Category: "console",
				Before:   "(not seen)",
				After:    e.Message,
			})
		}
	}

	// Compare network errors by method+path
	// Build map of all "after" requests (any status) for checking error resolution
	afterAllNetwork := make(map[string]VerifyNetworkEntry)
	for _, n := range after.AllNetworkRequests {
		key := n.Method + " " + n.Path
		afterAllNetwork[key] = n
	}

	// Build error-only maps
	beforeNetwork := make(map[string]VerifyNetworkEntry)
	for _, n := range before.NetworkErrors {
		key := n.Method + " " + n.Path
		beforeNetwork[key] = n
	}

	afterNetwork := make(map[string]VerifyNetworkEntry)
	for _, n := range after.NetworkErrors {
		key := n.Method + " " + n.Path
		afterNetwork[key] = n
	}

	// Check for resolved network errors
	for key, n := range beforeNetwork {
		if afterN, found := afterNetwork[key]; found {
			// Still an error - check if different
			if afterN.Status != n.Status {
				// Status changed but both are errors
				result.Changes = append(result.Changes, VerifyChange{
					Type:     "changed",
					Category: "network",
					Before:   fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
					After:    fmt.Sprintf("%s %s -> %d", afterN.Method, afterN.URL, afterN.Status),
				})
			}
		} else {
			// Error no longer in error list - check if it succeeded or just wasn't called
			if allN, found := afterAllNetwork[key]; found {
				// Endpoint was called - check if it succeeded
				if allN.Status >= 200 && allN.Status < 400 {
					result.Changes = append(result.Changes, VerifyChange{
						Type:     "resolved",
						Category: "network",
						Before:   fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
						After:    fmt.Sprintf("%s %s -> %d", allN.Method, allN.URL, allN.Status),
					})
				} else {
					// Still an error but different status
					result.Changes = append(result.Changes, VerifyChange{
						Type:     "changed",
						Category: "network",
						Before:   fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
						After:    fmt.Sprintf("%s %s -> %d", allN.Method, allN.URL, allN.Status),
					})
				}
			} else {
				// Endpoint not called - mark as resolved (can't compare)
				result.Changes = append(result.Changes, VerifyChange{
					Type:     "resolved",
					Category: "network",
					Before:   fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
					After:    "(not seen)",
				})
			}
		}
	}

	// New network errors
	for key, n := range afterNetwork {
		if _, found := beforeNetwork[key]; !found {
			result.NewIssues = append(result.NewIssues, VerifyChange{
				Type:     "new",
				Category: "network",
				Before:   "(not seen)",
				After:    fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
			})
		}
	}

	// Performance diff
	if before.Performance != nil && after.Performance != nil {
		result.PerformanceDiff = &VerifyPerfDiff{
			LoadTimeBefore: fmt.Sprintf("%.0fms", before.Performance.Timing.Load),
			LoadTimeAfter:  fmt.Sprintf("%.0fms", after.Performance.Timing.Load),
		}
		if before.Performance.Timing.Load > 0 {
			pctChange := ((after.Performance.Timing.Load - before.Performance.Timing.Load) / before.Performance.Timing.Load) * 100
			if pctChange >= 0 {
				result.PerformanceDiff.Change = fmt.Sprintf("+%.0f%%", pctChange)
			} else {
				result.PerformanceDiff.Change = fmt.Sprintf("%.0f%%", pctChange)
			}
		}
	}

	// Determine verdict
	result.Verdict = vm.determineVerdict(result)

	return result
}

// determineVerdict determines the overall verdict based on changes
func (vm *VerificationManager) determineVerdict(result VerificationResult) string {
	beforeTotal := result.Before.TotalIssues
	afterTotal := result.After.TotalIssues
	hasResolved := len(result.Changes) > 0
	hasNew := len(result.NewIssues) > 0

	// Count actual resolved changes (not just "changed")
	resolvedCount := 0
	for _, c := range result.Changes {
		if c.Type == "resolved" {
			resolvedCount++
		}
	}

	switch {
	case beforeTotal == 0 && afterTotal == 0:
		return "no_issues_detected"
	case resolvedCount > 0 && !hasNew && afterTotal == 0:
		return "fixed"
	case resolvedCount > 0 && !hasNew:
		return "improved"
	case hasResolved && hasNew:
		return "different_issue"
	case hasNew && afterTotal > beforeTotal:
		return "regressed"
	case hasNew:
		return "regressed"
	default:
		return "unchanged"
	}
}

// ============================================
// MCP Tool Handler
// ============================================

// verifyFixParams defines the MCP tool input schema
type verifyFixParams struct {
	Action    string `json:"action"`
	SessionID string `json:"session_id,omitempty"`
	Label     string `json:"label,omitempty"`
	URLFilter string `json:"url,omitempty"`
}

// HandleTool dispatches the verify_fix MCP tool call
func (vm *VerificationManager) HandleTool(params json.RawMessage) (any, error) {
	var p verifyFixParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	switch p.Action {
	case "start":
		result, err := vm.Start(p.Label, p.URLFilter)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"session_id": result.SessionID,
			"status":     result.Status,
			"label":      result.Label,
			"baseline":   result.Baseline,
		}, nil

	case "watch":
		if p.SessionID == "" {
			return nil, fmt.Errorf("'session_id' is required for watch action")
		}
		result, err := vm.Watch(p.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"session_id": result.SessionID,
			"status":     result.Status,
			"message":    result.Message,
		}, nil

	case "compare":
		if p.SessionID == "" {
			return nil, fmt.Errorf("'session_id' is required for compare action")
		}
		result, err := vm.Compare(p.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"session_id": result.SessionID,
			"status":     result.Status,
			"label":      result.Label,
			"result": map[string]any{
				"verdict":          result.Result.Verdict,
				"before":           result.Result.Before,
				"after":            result.Result.After,
				"changes":          result.Result.Changes,
				"new_issues":       result.Result.NewIssues,
				"performance_diff": result.Result.PerformanceDiff,
			},
		}, nil

	case "status":
		if p.SessionID == "" {
			return nil, fmt.Errorf("'session_id' is required for status action")
		}
		result, err := vm.Status(p.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"session_id": result.SessionID,
			"status":     result.Status,
			"label":      result.Label,
			"created_at": result.CreatedAt,
		}, nil

	case "cancel":
		if p.SessionID == "" {
			return nil, fmt.Errorf("'session_id' is required for cancel action")
		}
		result, err := vm.Cancel(p.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"session_id": result.SessionID,
			"status":     result.Status,
		}, nil

	default:
		if p.Action == "" {
			return nil, fmt.Errorf("'action' is required (start, watch, compare, status, cancel)")
		}
		return nil, fmt.Errorf("unknown action %q (valid: start, watch, compare, status, cancel)", p.Action)
	}
}
