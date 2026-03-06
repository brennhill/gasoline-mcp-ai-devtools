// Purpose: Observe-mode accessors for async command state/results.

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ============================================
// Command Result Observation
// ============================================

// annotationCommandWaitTimeout is how long observe blocks for pending annotation commands.
// Fits within the bridge's 65s timeout for blocking observe calls.
const annotationCommandWaitTimeout = 55 * time.Second

// toolObserveCommandResult retrieves the result of an async command by correlation_id.
// For annotation commands (ann_*), blocks up to 55s waiting for the user to finish drawing.
func (h *ToolHandler) toolObserveCommandResult(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil && len(args) > 0 {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	if params.CorrelationID == "" {
		return fail(req, ErrMissingParam, "Required parameter 'correlation_id' is missing", "Add the 'correlation_id' parameter and call again", withParam("correlation_id"))
	}

	corrID := params.CorrelationID

	// Annotation commands: block up to 55s waiting for the user to finish drawing.
	// This is token-efficient — the LLM makes one call and waits instead of rapid polling.
	// See docs/core/async-tool-pattern.md for the full pattern.
	if strings.HasPrefix(corrID, "ann_") {
		cmd, found := h.capture.WaitForCommand(corrID, annotationCommandWaitTimeout)
		if !found {
			return fail(req, ErrNoData,
				"Annotation command not found: "+corrID,
				"The command may have expired (10 min TTL). Start a new draw mode session.",
				withFinal(true),
				h.diagnosticHint())
		}
		return h.formatCommandResult(req, *cmd, corrID)
	}

	cmd, found := h.capture.GetCommandResult(corrID)
	if !found {
		return fail(req, ErrNoData,
			"Command not found: "+corrID,
			"The command may have already completed and been cleaned up (60s TTL), or the correlation_id is invalid. Use observe with what='pending_commands' to see active commands.",
			withFinal(true),
			h.diagnosticHint())
	}

	return h.formatCommandResult(req, *cmd, corrID)
}

// toolObservePendingCommands lists all pending, completed, and failed async commands.
func (h *ToolHandler) toolObservePendingCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	pending := h.capture.GetPendingCommands()
	completed := h.capture.GetCompletedCommands()
	failed := h.capture.GetFailedCommands()
	inProgress := h.capture.GetInProgressCommands()

	responseData := map[string]any{
		"pending":                     pending,
		"completed":                   completed,
		"failed":                      failed,
		"extension_in_progress":       inProgress,
		"extension_in_progress_count": len(inProgress),
	}

	summary := fmt.Sprintf(
		"Pending: %d, Completed: %d, Failed: %d, Extension in-progress: %d",
		len(pending),
		len(completed),
		len(failed),
		len(inProgress),
	)
	return succeed(req, summary, responseData)
}

// toolObserveFailedCommands lists recent failed/expired async commands.
func (h *ToolHandler) toolObserveFailedCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	failed := h.capture.GetFailedCommands()

	responseData := map[string]any{
		"commands": failed,
		"count":    len(failed),
	}

	if len(failed) == 0 {
		return succeed(req, "No failed commands found", responseData)
	}

	summary := fmt.Sprintf("Found %d failed/expired commands", len(failed))
	return succeed(req, summary, responseData)
}
