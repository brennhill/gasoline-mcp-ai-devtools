// Purpose: Playback execution and playback-result retrieval handlers.
// Why: Isolates replay lifecycle and session projection from recording/log-diff handlers.
// Docs: docs/features/feature/flow-recording/index.md

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// toolConfigurePlayback executes playback and stores session for later observe retrieval.
//
// Invariants:
// - playbackSessions map is written only under playbackMu.
//
// Failure semantics:
// - Invalid/missing recording IDs return explicit structured errors.
func (h *ToolHandler) toolConfigurePlayback(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if resp, blocked := requireString(req, params.RecordingID, "recording_id", "Provide a recording_id from a previous recording"); blocked {
		return resp
	}

	// Execute playback
	session, err := h.capture.ExecutePlayback(params.RecordingID)
	if err != nil {
		return fail(req, ErrInternal,
			fmt.Sprintf("Failed to execute playback: %v", err),
			"Ensure the recording_id is valid")
	}

	// Store session for later retrieval via observe(what:"playback_results")
	func() {
		h.playbackMu.Lock()
		defer h.playbackMu.Unlock()
		h.playbackSessions[params.RecordingID] = session
	}()

	total := session.ActionsExecuted + session.ActionsFailed
	h.appendServerLog(LogEntry{
		"timestamp":        time.Now().Format(time.RFC3339Nano),
		"level":            "info",
		"message":          fmt.Sprintf("[PLAYBACK_COMPLETE] Recording replayed: %d/%d actions succeeded", session.ActionsExecuted, total),
		"category":         "PLAYBACK",
		"recording_id":     params.RecordingID,
		"actions_executed": session.ActionsExecuted,
		"actions_failed":   session.ActionsFailed,
	})

	return h.buildPlaybackResult(req, params.RecordingID, session)
}

// toolGetPlaybackResults reads stored playback session snapshots by recording ID.
//
// Invariants:
// - playbackSessions map is read under playbackMu and transformed into detached response maps.
//
// Failure semantics:
// - Missing session returns ErrNoData without attempting replay.
func (h *ToolHandler) toolGetPlaybackResults(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if resp, blocked := requireString(req, params.RecordingID, "recording_id", "Provide the recording_id from playback"); blocked {
		return resp
	}

	// Look up stored playback session
	h.playbackMu.RLock()
	session, found := h.playbackSessions[params.RecordingID]
	h.playbackMu.RUnlock()

	if !found {
		return fail(req, ErrNoData,
			fmt.Sprintf("No playback results for recording_id %s", params.RecordingID),
			"Run configure(action:'playback', recording_id:'...') first")
	}

	// Build per-action results
	actions := make([]map[string]any, 0, len(session.Results))
	for _, r := range session.Results {
		action := map[string]any{
			"status":           r.Status,
			"action_index":     r.ActionIndex,
			"action_type":      r.ActionType,
			"selector_used":    r.SelectorUsed,
			"duration_ms":      r.DurationMs,
			"error":            r.Error,
			"selector_fragile": r.SelectorFragile,
		}
		if r.Coordinates != nil {
			action["coordinates"] = map[string]any{"x": r.Coordinates.X, "y": r.Coordinates.Y}
		}
		actions = append(actions, action)
	}

	total := session.ActionsExecuted + session.ActionsFailed
	responseData := map[string]any{
		"recording_id":      params.RecordingID,
		"status":            "ok",
		"actions_executed":  session.ActionsExecuted,
		"actions_failed":    session.ActionsFailed,
		"actions_total":     total,
		"duration_ms":       time.Since(session.StartedAt).Milliseconds(),
		"results":           actions,
		"selector_failures": session.SelectorFailures,
	}

	summary := fmt.Sprintf("Playback results: %d/%d actions executed", session.ActionsExecuted, total)
	return succeed(req, summary, responseData)
}
