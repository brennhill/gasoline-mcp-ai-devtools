// tools_async.go — Async command infrastructure for all MCP tools.
//
// Two async dispatch patterns exist in this codebase:
//
//  1. MaybeWaitForCommand (sync-by-default) — Used by interact and analyze handlers
//     that dispatch commands to the browser extension. The handler queues a command,
//     then blocks up to 20s waiting for completion. If the command finishes in time,
//     the result is returned inline. If not, a "still_processing" handle is returned
//     so the LLM can poll via observe(what="command_result").
//     Use this when: the extension executes an action and returns a result.
//
//  2. WaitForResult (always-blocking) — Used by internal queries (a11y audits,
//     screenshots, DOM queries) where the extension must respond before the tool
//     can return anything useful. There is no "still_processing" fallback.
//     Use this when: the tool's response IS the query result (no partial answer).
//
// All correlation IDs are generated via newCorrelationID(prefix) for consistency.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/performance"
	"github.com/dev-console/dev-console/internal/queries"
)

// newCorrelationID generates a unique correlation ID with the given prefix.
// Format: prefix_timestamp_random (e.g., "nav_1708300000000000000_4821937562").
func newCorrelationID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), randomInt63())
}

// ============================================
// Sync-by-Default Command Dispatch
// ============================================

// MaybeWaitForCommand blocks until an async command completes, or returns a "queued"
// response if background execution is requested or the safety timeout is reached.
// This is the core implementation of "Synchronous-by-Default" mode.
func (h *ToolHandler) MaybeWaitForCommand(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse {
	var params struct {
		Sync       *bool `json:"sync"`
		Wait       *bool `json:"wait"`
		Background bool  `json:"background"`
	}
	lenientUnmarshal(args, &params)

	// Default to sync unless Background is true or Sync/Wait explicitly set to false
	isSync := true
	if params.Background {
		isSync = false
	}
	if params.Sync != nil && !*params.Sync {
		isSync = false
	}
	if params.Wait != nil && !*params.Wait {
		isSync = false
	}

	if !isSync {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(queuedSummary, map[string]any{
			"status":         "queued",
			"correlation_id": correlationID,
			"queued":         true,
			"final":          false,
		})}
	}

	// Check connectivity first to avoid useless waiting
	if !h.capture.IsExtensionConnected() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Extension is not connected", "Ensure the Gasoline extension shows 'Connected' and a tab is tracked.", h.diagnosticHint())}
	}

	// Wait for result (15s default for "Sync-by-Default" pattern).
	// Most DOM actions (click, type) take < 500ms over long-polling.
	// 15s is safe for most navigations while staying well under the 60s IDE timeout.
	const initialWait = 15 * time.Second
	const retryWait = 5 * time.Second
	attempts := 1

	cmd, found := h.capture.WaitForCommand(correlationID, initialWait)
	if !found {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Command not found after queuing", "Internal error — do not retry")}
	}

	// Single retry: if still pending and extension is connected, wait 5s more.
	// This catches commands that complete just after the initial timeout.
	if cmd.Status == "pending" && h.capture.IsExtensionConnected() {
		attempts = 2
		cmd, found = h.capture.WaitForCommand(correlationID, retryWait)
		if !found {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Command not found after retry", "Internal error — do not retry")}
		}
	}

	// If still pending after retry, return a "still_processing" handle so the agent can poll.
	if cmd.Status == "pending" {
		totalWaitMs := initialWait.Milliseconds()
		if attempts > 1 {
			totalWaitMs += retryWait.Milliseconds()
		}
		stillProcessing := map[string]any{
			"status":         "still_processing",
			"correlation_id": correlationID,
			"queued":         false,
			"final":          false,
			"elapsed_ms":     cmd.ElapsedMs(),
			"queue_depth":    h.capture.QueueDepth(),
			"retry_context": map[string]any{
				"attempts":            attempts,
				"total_wait_ms":       totalWaitMs,
				"extension_connected": h.capture.IsExtensionConnected(),
			},
			"suggested_retry_ms": 2000,
			"message":            "Action is taking longer than expected. Polling is now required. Use observe({what:'command_result', correlation_id:'" + correlationID + "'}) to check the result.",
		}
		if pos := h.capture.QueuePosition(correlationID); pos >= 0 {
			stillProcessing["queue_position"] = pos
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Action still processing", stillProcessing)}
	}

	// Result received — format using standard command result formatter
	return h.formatCommandResult(req, *cmd, correlationID)
}

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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.CorrelationID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'correlation_id' is missing", "Add the 'correlation_id' parameter and call again", withParam("correlation_id"))}
	}

	corrID := params.CorrelationID

	// Annotation commands: block up to 55s waiting for the user to finish drawing.
	// This is token-efficient — the LLM makes one call and waits instead of rapid polling.
	// See docs/core/async-tool-pattern.md for the full pattern.
	if strings.HasPrefix(corrID, "ann_") {
		cmd, found := h.capture.WaitForCommand(corrID, annotationCommandWaitTimeout)
		if !found {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrNoData,
				"Annotation command not found: "+corrID,
				"The command may have expired (10 min TTL). Start a new draw mode session.",
				withFinal(true),
				h.diagnosticHint(),
			)}
		}
		return h.formatCommandResult(req, *cmd, corrID)
	}

	cmd, found := h.capture.GetCommandResult(corrID)
	if !found {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrNoData,
			"Command not found: "+corrID,
			"The command may have already completed and been cleaned up (60s TTL), or the correlation_id is invalid. Use observe with what='pending_commands' to see active commands.",
			withFinal(true),
			h.diagnosticHint(),
		)}
	}

	return h.formatCommandResult(req, *cmd, corrID)
}

// toolObservePendingCommands lists all pending, completed, and failed async commands.
func (h *ToolHandler) toolObservePendingCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	pending := h.capture.GetPendingCommands()
	completed := h.capture.GetCompletedCommands()
	failed := h.capture.GetFailedCommands()

	responseData := map[string]any{
		"pending":   pending,
		"completed": completed,
		"failed":    failed,
	}

	summary := fmt.Sprintf("Pending: %d, Completed: %d, Failed: %d", len(pending), len(completed), len(failed))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

// toolObserveFailedCommands lists recent failed/expired async commands.
func (h *ToolHandler) toolObserveFailedCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	failed := h.capture.GetFailedCommands()

	responseData := map[string]any{
		"status":   "ok",
		"commands": failed,
		"count":    len(failed),
	}

	if len(failed) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No failed commands found", responseData)}
	}

	summary := fmt.Sprintf("Found %d failed/expired commands", len(failed))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

// ============================================
// Command Result Formatting
// ============================================

func (h *ToolHandler) formatCommandResult(req JSONRPCRequest, cmd queries.CommandResult, corrID string) JSONRPCResponse {
	responseData := map[string]any{
		"correlation_id": cmd.CorrelationID,
		"status":         cmd.Status,
		"queued":         false,
		"created_at":     cmd.CreatedAt.Format(time.RFC3339),
		"elapsed_ms":     cmd.ElapsedMs(),
	}

	switch cmd.Status {
	case "complete":
		responseData["final"] = true
		return h.formatCompleteCommand(req, cmd, corrID, responseData)
	case "error":
		responseData["final"] = true
		if cmd.Error == "" {
			cmd.Error = "Command failed in extension"
		}
		responseData["error"] = cmd.Error
		if len(cmd.Result) > 0 {
			responseData["result"] = cmd.Result
		}
		summary := fmt.Sprintf("FAILED — Command %s error: %s", corrID, cmd.Error)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONErrorResponse(summary, responseData)}
	case "expired":
		responseData["final"] = true
		responseData["error"] = ErrExtTimeout
		responseData["message"] = fmt.Sprintf("Command %s expired before the extension could execute it. Error: %s", corrID, cmd.Error)
		responseData["retry"] = "The browser extension may be disconnected or the page is not active. Check observe with what='pilot' to verify extension status, then retry the command."
		responseData["hint"] = h.DiagnosticHintString()
		summary := fmt.Sprintf("FAILED — Command %s expired: %s", corrID, cmd.Error)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONErrorResponse(summary, responseData)}
	case "timeout":
		responseData["final"] = true
		responseData["error"] = ErrExtTimeout
		responseData["message"] = fmt.Sprintf("Command %s timed out waiting for the extension to respond. Error: %s", corrID, cmd.Error)
		responseData["retry"] = "The command took too long. The page may be unresponsive or the action is stuck. Try refreshing the page with interact action='refresh', then retry."
		responseData["hint"] = h.DiagnosticHintString()
		summary := fmt.Sprintf("FAILED — Command %s timed out: %s", corrID, cmd.Error)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONErrorResponse(summary, responseData)}
	default:
		responseData["final"] = false
		summary := fmt.Sprintf("Command %s: %s", corrID, cmd.Status)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
	}
}

func (h *ToolHandler) formatCompleteCommand(req JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) JSONRPCResponse {
	responseData["result"] = cmd.Result
	responseData["completed_at"] = cmd.CompletedAt.Format(time.RFC3339)
	responseData["timing_ms"] = cmd.CompletedAt.Sub(cmd.CreatedAt).Milliseconds()

	if embeddedErr, hasEmbeddedErr := enrichCommandResponseData(cmd.Result, responseData); cmd.Error == "" && hasEmbeddedErr {
		cmd.Error = embeddedErr
	}

	if beforeSnap, ok := h.capture.GetAndDeleteBeforeSnapshot(corrID); ok {
		// The "after" perf snapshot arrives ~2.5s after page load (2s content script
		// delay + 500ms batcher debounce). Poll briefly for a snapshot newer than
		// the "before" baseline. Without this wait, we'd compare the same snapshot
		// to itself (zero diff) or find nothing.
		afterSnap, found := h.capture.GetPerformanceSnapshotByURL(beforeSnap.URL)
		if !found || afterSnap.Timestamp == beforeSnap.Timestamp {
			for retry := 0; retry < 5; retry++ {
				time.Sleep(500 * time.Millisecond)
				afterSnap, found = h.capture.GetPerformanceSnapshotByURL(beforeSnap.URL)
				if found && afterSnap.Timestamp != beforeSnap.Timestamp {
					break // Found a genuinely new snapshot
				}
			}
		}
		if found && afterSnap.Timestamp != beforeSnap.Timestamp {
			before := performance.SnapshotToPageLoadMetrics(beforeSnap)
			after := performance.SnapshotToPageLoadMetrics(afterSnap)
			responseData["perf_diff"] = performance.ComputePerfDiff(before, after)
		}
	}

	if cmd.Error != "" {
		responseData["error"] = cmd.Error
		summary := fmt.Sprintf("FAILED — Command %s completed with error: %s", corrID, cmd.Error)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONErrorResponse(summary, responseData)}
	}

	summary := fmt.Sprintf("Command %s: complete", corrID)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

func enrichCommandResponseData(result json.RawMessage, responseData map[string]any) (embeddedErr string, hasEmbeddedErr bool) {
	if len(result) == 0 {
		return "", false
	}

	var extResult map[string]any
	if err := json.Unmarshal(result, &extResult); err != nil {
		return "", false
	}

	// Surface extension enrichment fields to top-level for easier LLM consumption.
	for _, key := range []string{"timing", "dom_changes", "dom_summary", "analysis", "content_script_status", "resolved_tab_id", "resolved_url", "target_context", "effective_tab_id", "effective_url", "effective_title", "final_url", "title"} {
		if v, ok := extResult[key]; ok {
			responseData[key] = v
		}
	}

	if success, ok := extResult["success"].(bool); ok && !success {
		msg := embeddedCommandError(extResult)
		if msg == "" {
			msg = "Command reported success=false"
		}
		return msg, true
	}

	if _, ok := extResult["error"]; ok {
		msg := embeddedCommandError(extResult)
		if msg != "" {
			return msg, true
		}
	}

	return "", false
}

func embeddedCommandError(extResult map[string]any) string {
	if msg, ok := extResult["error"].(string); ok && msg != "" {
		return msg
	}
	if msg, ok := extResult["message"].(string); ok && msg != "" {
		return msg
	}
	return ""
}
