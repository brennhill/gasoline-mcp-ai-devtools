package recording

import "fmt"

func (r *RecordingManager) DiffRecordings(originalRecordingID, replayRecordingID string) (*LogDiffResult, error) {
	original, err := r.GetRecording(originalRecordingID)
	if err != nil {
		return nil, fmt.Errorf("logdiff_load_original_failed: Failed to load original recording: %w", err)
	}

	replay, err := r.GetRecording(replayRecordingID)
	if err != nil {
		return nil, fmt.Errorf("logdiff_load_replay_failed: Failed to load replay recording: %w", err)
	}

	result := &LogDiffResult{
		OriginalRecording: originalRecordingID,
		ReplayRecording:   replayRecordingID,
		NewErrors:         make([]DiffLogEntry, 0),
		MissingEvents:     make([]DiffLogEntry, 0),
		ChangedValues:     make([]ValueChange, 0),
	}

	result.ActionStats = r.compareActions(original, replay)
	r.detectRegressions(original, replay, result)
	r.detectFixes(original, replay, result)
	r.detectValueChanges(original, replay, result)
	r.determineStatus(result)

	return result, nil
}

func (r *RecordingManager) compareActions(original, replay *Recording) ActionComparison {
	stats := ActionComparison{
		OriginalCount: original.ActionCount,
		ReplayCount:   replay.ActionCount,
	}
	stats.ErrorsOriginal, stats.ClicksOriginal, stats.TypesOriginal, stats.NavigatesOriginal = CountActionTypes(original.Actions)
	stats.ErrorsReplay, stats.ClicksReplay, stats.TypesReplay, stats.NavigatesReplay = CountActionTypes(replay.Actions)
	return stats
}

func (r *RecordingManager) detectRegressions(original, replay *Recording, result *LogDiffResult) {
	originalErrors := make(map[string]bool)
	for _, action := range original.Actions {
		if action.Type == "error" {
			originalErrors[action.Text] = true
		}
	}

	for _, action := range replay.Actions {
		if action.Type == "error" && !originalErrors[action.Text] {
			result.NewErrors = append(result.NewErrors, DiffLogEntry{
				Type:       "error",
				Severity:   "high",
				Level:      "error",
				Message:    action.Text,
				Timestamp:  action.TimestampMs,
				Selector:   action.Selector,
				ActionType: action.Type,
			})
		}
	}
}

func (r *RecordingManager) detectFixes(original, replay *Recording, result *LogDiffResult) {
	replayErrors := make(map[string]bool)
	for _, action := range replay.Actions {
		if action.Type == "error" {
			replayErrors[action.Text] = true
		}
	}

	for _, action := range original.Actions {
		if action.Type == "error" && !replayErrors[action.Text] {
			result.MissingEvents = append(result.MissingEvents, DiffLogEntry{
				Type:       "error",
				Severity:   "high",
				Level:      "error",
				Message:    action.Text,
				Timestamp:  action.TimestampMs,
				Selector:   action.Selector,
				ActionType: action.Type,
			})
		}
	}
}

func (r *RecordingManager) detectValueChanges(original, replay *Recording, result *LogDiffResult) {
	originalValues := BuildTypeValueMap(original.Actions)

	for _, action := range replay.Actions {
		if action.Type != "type" || action.Selector == "" {
			continue
		}
		originalValue, exists := originalValues[action.Selector]
		if !exists || originalValue == action.Text {
			continue
		}
		result.ChangedValues = append(result.ChangedValues, ValueChange{
			Field:     action.Selector,
			FromValue: originalValue,
			ToValue:   action.Text,
			Timestamp: action.TimestampMs,
		})
	}
}

func (r *RecordingManager) determineStatus(result *LogDiffResult) {
	if len(result.NewErrors) > 0 {
		result.Status = "regression"
		result.Summary = fmt.Sprintf("⚠️ REGRESSION: %d new errors detected", len(result.NewErrors))
		return
	}

	if len(result.MissingEvents) > 0 && len(result.NewErrors) == 0 {
		result.Status = "fixed"
		result.Summary = fmt.Sprintf("✓ FIXED: %d errors no longer appear", len(result.MissingEvents))
		return
	}

	if len(result.ChangedValues) > 0 {
		result.Status = "changed"
		result.Summary = fmt.Sprintf("⚠️ VALUE CHANGES: %d field(s) changed", len(result.ChangedValues))
		return
	}

	result.Status = "match"
	result.Summary = "All logs match (0 new errors, 0 missing events)"
}
