// Purpose: Shared recording/playback response and logging helpers for MCP handler methods.
// Why: Keeps common formatting and bounded log writes DRY across recording-related handlers.
// Docs: docs/features/feature/flow-recording/index.md

package main

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// buildPlaybackResult constructs canonical playback completion payload.
//
// Failure semantics:
// - Session timing is computed from session.StartedAt; clock skew only affects duration text.
func (h *ToolHandler) buildPlaybackResult(req JSONRPCRequest, recordingID string, session *capture.PlaybackSession) JSONRPCResponse {
	status := "ok"
	if session.ActionsFailed > 0 {
		status = "partial"
	}
	total := session.ActionsExecuted + session.ActionsFailed
	responseData := map[string]any{
		"status":            status,
		"recording_id":      recordingID,
		"actions_executed":  session.ActionsExecuted,
		"actions_failed":    session.ActionsFailed,
		"actions_total":     total,
		"duration_ms":       time.Since(session.StartedAt).Milliseconds(),
		"results_count":     len(session.Results),
		"selector_failures": session.SelectorFailures,
	}
	message := fmt.Sprintf("Playback complete: %d/%d actions executed", session.ActionsExecuted, total)
	return succeed(req, message, responseData)
}

// appendServerLog appends one entry to bounded in-memory daemon logs.
//
// Invariants:
// - h.server.entries is always size-limited to h.server.maxEntries under h.server.mu.
//
// Failure semantics:
// - Oldest entries are evicted first when capacity is exceeded.
func (h *ToolHandler) appendServerLog(entry LogEntry) {
	h.server.mu.Lock()
	defer h.server.mu.Unlock()
	h.server.entries = append(h.server.entries, entry)
	if len(h.server.entries) > h.server.maxEntries {
		h.server.entries = h.server.entries[1:]
	}
}
