// Purpose: Recording log-diff comparison and report generation handlers.
// Why: Keeps diff/report concerns separated from recording control and playback flows.
// Docs: docs/features/feature/flow-recording/index.md

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// toolConfigureLogDiff compares two recordings and returns summary delta counts.
//
// Failure semantics:
// - Comparison errors are surfaced directly; no partial diff payload is returned.
func (h *ToolHandler) toolConfigureLogDiff(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		OriginalID string `json:"original_id"`
		ReplayID   string `json:"replay_id"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if resp, blocked := requireString(req, params.OriginalID, "original_id", "Provide the original recording ID"); blocked {
		return resp
	}
	if resp, blocked := requireString(req, params.ReplayID, "replay_id", "Provide the replay recording ID"); blocked {
		return resp
	}

	// Compare recordings
	result, err := h.capture.DiffRecordings(params.OriginalID, params.ReplayID)
	if err != nil {
		return fail(req, ErrInternal,
			fmt.Sprintf("Failed to diff recordings: %v", err),
			"Ensure both recording IDs are valid")
	}

	h.appendServerLog(LogEntry{
		"timestamp":   time.Now().Format(time.RFC3339Nano),
		"level":       "info",
		"message":     fmt.Sprintf("[LOG_DIFF] Comparison complete: %s", result.Summary),
		"category":    "LOG_DIFF",
		"original_id": params.OriginalID,
		"replay_id":   params.ReplayID,
		"status":      result.Status,
	})

	responseData := map[string]any{
		"status":         result.Status,
		"summary":        result.Summary,
		"original_id":    params.OriginalID,
		"replay_id":      params.ReplayID,
		"new_errors":     len(result.NewErrors),
		"missing_events": len(result.MissingEvents),
		"changed_values": len(result.ChangedValues),
		"action_stats":   result.ActionStats,
	}

	return succeed(req, result.Summary, responseData)
}

// toolGetLogDiffReport returns human-readable regression report text for two recordings.
//
// Failure semantics:
// - Underlying diff errors short-circuit response with structured internal error.
func (h *ToolHandler) toolGetLogDiffReport(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		OriginalID string `json:"original_id"`
		ReplayID   string `json:"replay_id"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if resp, blocked := requireString(req, params.OriginalID, "original_id", "Provide the original recording ID"); blocked {
		return resp
	}
	if resp, blocked := requireString(req, params.ReplayID, "replay_id", "Provide the replay recording ID"); blocked {
		return resp
	}

	// Compare recordings
	result, err := h.capture.DiffRecordings(params.OriginalID, params.ReplayID)
	if err != nil {
		return fail(req, ErrInternal,
			fmt.Sprintf("Failed to generate report: %v", err),
			"Ensure both recording IDs are valid")
	}

	// Generate report text
	report := result.GetRegressionReport()

	responseData := map[string]any{
		"status":  result.Status,
		"report":  report,
		"summary": result.Summary,
		"stats":   result.ActionStats,
	}

	return succeed(req, "Log diff report", responseData)
}
