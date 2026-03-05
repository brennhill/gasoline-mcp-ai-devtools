// Purpose: Implements passive network traffic recording with start/stop lifecycle and snapshot diffing for the configure tool.
// Why: Captures network baseline snapshots so agents can compare traffic patterns before and after code changes.
// Docs: docs/features/feature/backend-log-streaming/index.md

package main

import (
	"encoding/json"
	"time"
)

// toolConfigureNetworkRecording handles configure(what="network_recording").
func (h *ToolHandler) toolConfigureNetworkRecording(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation string `json:"operation"`
		Domain    string `json:"domain"`
		Method    string `json:"method"`
	}
	if len(args) > 0 {
				if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	switch params.Operation {
	case "start":
		startedAt, ok := h.networkRecording.tryStart(params.Domain, params.Method)
		if !ok {
			return fail(req, ErrInvalidParam,
				"Network recording already active",
				"Stop the current recording first with operation='stop'.")
		}
		result := map[string]any{
			"status":     "recording",
			"started_at": startedAt.Format(time.RFC3339),
		}
		if params.Domain != "" {
			result["domain_filter"] = params.Domain
		}
		if params.Method != "" {
			result["method_filter"] = params.Method
		}
		return succeed(req, "Network recording started", result)

	case "stop":
		snap, wasActive := h.networkRecording.stop()
		if !wasActive {
			return fail(req, ErrInvalidParam,
				"No active network recording",
				"Start a recording first with operation='start'.")
		}

		// Collect network bodies captured since start time.
		recorded := collectRecordedRequests(h.capture.GetNetworkBodies(), snap)

		duration := time.Since(snap.StartTime)
		result := map[string]any{
			"status":      "stopped",
			"duration_ms": int(duration.Milliseconds()),
			"requests":    recorded,
			"count":       len(recorded),
		}
		return succeed(req, "Network recording stopped", result)

	case "status", "":
		snap := h.networkRecording.info()
		result := map[string]any{
			"active": snap.Active,
		}
		if snap.Active {
			result["started_at"] = snap.StartTime.Format(time.RFC3339)
			result["duration_ms"] = int(time.Since(snap.StartTime).Milliseconds())
			if snap.Domain != "" {
				result["domain_filter"] = snap.Domain
			}
			if snap.Method != "" {
				result["method_filter"] = snap.Method
			}
		}
		return succeed(req, "Network recording status", result)

	default:
		return fail(req, ErrInvalidParam,
			"Unknown operation: "+params.Operation,
			"Use 'start', 'stop', or 'status'.")
	}
}
