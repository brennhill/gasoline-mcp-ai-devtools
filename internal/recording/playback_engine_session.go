// Purpose: Manages playback session lifecycle including start, result collection, and completion.
// Why: Separates session orchestration from individual action execution.
package recording

import (
	"fmt"
	"time"
)

func (r *RecordingManager) StartPlayback(recordingID string) (*PlaybackSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	recording, exists := r.recordings[recordingID]
	if !exists {
		loaded, err := r.loadRecordingFromDisk(recordingID)
		if err != nil {
			return nil, fmt.Errorf("playback_recording_not_found: Recording %s not found: %w", recordingID, err)
		}
		recording = loaded
	}

	if recording == nil || len(recording.Actions) == 0 {
		return nil, fmt.Errorf("playback_no_actions: Recording has no actions to replay")
	}

	session := &PlaybackSession{
		RecordingID:      recordingID,
		StartedAt:        time.Now(),
		Results:          make([]PlaybackResult, 0),
		SelectorFailures: make(map[string]int),
	}

	return session, nil
}

func (r *RecordingManager) ExecutePlayback(recordingID string) (*PlaybackSession, error) {
	session, err := r.StartPlayback(recordingID)
	if err != nil {
		return nil, err
	}

	recording := func() *Recording {
		r.mu.Lock()
		defer r.mu.Unlock()
		current, exists := r.recordings[recordingID]
		if !exists {
			current, _ = r.loadRecordingFromDisk(recordingID)
		}
		return current
	}()
	if recording == nil {
		return nil, fmt.Errorf("playback_load_failed: Could not load recording")
	}

	for i, action := range recording.Actions {
		result := r.executeAction(i, action)
		session.Results = append(session.Results, result)

		if result.Status == "error" {
			session.ActionsFailed++
			if action.Selector != "" {
				session.SelectorFailures[action.Selector]++
			}
			continue
		}

		session.ActionsExecuted++
	}

	return session, nil
}

func (r *RecordingManager) GetPlaybackStatus(session *PlaybackSession) map[string]any {
	totalTime := time.Since(session.StartedAt)

	status := "ok"
	if session.ActionsFailed > 0 {
		status = "partial"
	}
	if session.ActionsExecuted == 0 {
		status = "failed"
	}

	return map[string]any{
		"status":            status,
		"actions_executed":  session.ActionsExecuted,
		"actions_failed":    session.ActionsFailed,
		"actions_total":     session.ActionsExecuted + session.ActionsFailed,
		"duration_ms":       totalTime.Milliseconds(),
		"results_count":     len(session.Results),
		"selector_failures": session.SelectorFailures,
	}
}
