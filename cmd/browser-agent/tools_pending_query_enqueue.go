// Purpose: Shared pending-query enqueue helper with consistent queue saturation errors.
// Why: Prevents duplicated CreatePendingQueryWithTimeout error handling across tools.

package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// EnqueuePendingQuery submits a command for extension pickup and returns a
// structured error response when queueing fails.
// Satisfies mcp.PendingQueryEnqueuer.
func (h *ToolHandler) EnqueuePendingQuery(req JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (JSONRPCResponse, bool) {
	_, err := h.capture.CreatePendingQueryWithTimeout(query, timeout, req.ClientID)
	if err == nil {
		return JSONRPCResponse{}, false
	}

	if errors.Is(err, queries.ErrQueueFull) {
		return fail(req, ErrQueueFull,
			fmt.Sprintf("Command queue is full; unable to enqueue action type=%q", query.Type),
			"Wait for in-flight commands to complete, then retry.",
			withRetryable(true), withRetryAfterMs(1000), h.diagnosticHint(),
			withRecoveryToolCall(map[string]any{
				"tool":      "observe",
				"arguments": map[string]any{"what": "pending_commands"},
			}),
		), true
	}

	return fail(req, ErrInternal,
		fmt.Sprintf("Failed to enqueue command type=%q: %v", query.Type, err),
		"Internal error — do not retry until server health is restored.",
		h.diagnosticHint(),
	), true
}
