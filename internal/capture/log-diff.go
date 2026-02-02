// log-diff.go — Log diffing and regression detection for recordings.
// Compares original vs replay recordings to detect regressions, fixes, and value changes.
// Categories: Match (no issues), Regression (new errors), Fixed (errors resolved).
package capture

import (
	"fmt"
)

// ============================================================================
// Log Diff Result Types
// ============================================================================

// LogDiffResult represents the comparison of two recordings
type LogDiffResult struct {
	Status            string           // "match", "regression", "fixed", "changed"
	OriginalRecording string
	ReplayRecording   string
	Summary           string
	NewErrors         []DiffLogEntry
	MissingEvents     []DiffLogEntry
	ChangedValues     []ValueChange
	ActionStats       ActionComparison
}

// DiffLogEntry represents a single log entry for diff comparison (action or error)
type DiffLogEntry struct {
	Type        string // "error", "warning", "info"
	Severity    string // "critical", "high", "medium", "low"
	Level       string // log level
	Message     string
	Timestamp   int64
	Selector    string
	ActionType  string
}

// ValueChange represents a field value that changed between recordings
type ValueChange struct {
	Field     string
	FromValue string
	ToValue   string
	Timestamp int64
}

// ActionComparison tracks action counts and types between recordings
type ActionComparison struct {
	OriginalCount   int
	ReplayCount     int
	ErrorsOriginal  int
	ErrorsReplay    int
	ClicksOriginal  int
	ClicksReplay    int
	TypesOriginal   int
	TypesReplay     int
	NavigatesOriginal int
	NavigatesReplay  int
}

// ============================================================================
// Log Diffing
// ============================================================================

// DiffRecordings compares two recordings and detects regressions
func (c *Capture) DiffRecordings(originalRecordingID, replayRecordingID string) (*LogDiffResult, error) {
	// Load both recordings
	original, err := c.GetRecording(originalRecordingID)
	if err != nil {
		return nil, fmt.Errorf("logdiff_load_original_failed: Failed to load original recording: %v", err)
	}

	replay, err := c.GetRecording(replayRecordingID)
	if err != nil {
		return nil, fmt.Errorf("logdiff_load_replay_failed: Failed to load replay recording: %v", err)
	}

	result := &LogDiffResult{
		OriginalRecording: originalRecordingID,
		ReplayRecording:   replayRecordingID,
		NewErrors:         make([]DiffLogEntry, 0),
		MissingEvents:     make([]DiffLogEntry, 0),
		ChangedValues:     make([]ValueChange, 0),
	}

	// Categorize actions by type in both recordings
	result.ActionStats = c.compareActions(original, replay)

	// Detect differences
	c.detectRegressions(original, replay, result)
	c.detectFixes(original, replay, result)
	c.detectValueChanges(original, replay, result)

	// Determine overall status and summary
	c.determineStatus(result)

	return result, nil
}

// compareActions builds action comparison statistics
func (c *Capture) compareActions(original, replay *Recording) ActionComparison {
	stats := ActionComparison{
		OriginalCount: original.ActionCount,
		ReplayCount:   replay.ActionCount,
	}

	// Count action types in original
	for _, action := range original.Actions {
		switch action.Type {
		case "error":
			stats.ErrorsOriginal++
		case "click":
			stats.ClicksOriginal++
		case "type":
			stats.TypesOriginal++
		case "navigate":
			stats.NavigatesOriginal++
		}
	}

	// Count action types in replay
	for _, action := range replay.Actions {
		switch action.Type {
		case "error":
			stats.ErrorsReplay++
		case "click":
			stats.ClicksReplay++
		case "type":
			stats.TypesReplay++
		case "navigate":
			stats.NavigatesReplay++
		}
	}

	return stats
}

// detectRegressions finds new errors in replay
func (c *Capture) detectRegressions(original, replay *Recording, result *LogDiffResult) {
	// Build set of error messages in original
	originalErrors := make(map[string]bool)
	for _, action := range original.Actions {
		if action.Type == "error" {
			originalErrors[action.Text] = true
		}
	}

	// Find new errors in replay
	for _, action := range replay.Actions {
		if action.Type == "error" {
			if !originalErrors[action.Text] {
				entry := DiffLogEntry{
					Type:       "error",
					Severity:   "high",
					Level:      "error",
					Message:    action.Text,
					Timestamp:  action.TimestampMs,
					Selector:   action.Selector,
					ActionType: action.Type,
				}
				result.NewErrors = append(result.NewErrors, entry)
			}
		}
	}
}

// detectFixes finds errors that disappeared in replay
func (c *Capture) detectFixes(original, replay *Recording, result *LogDiffResult) {
	// Build set of error messages in replay
	replayErrors := make(map[string]bool)
	for _, action := range replay.Actions {
		if action.Type == "error" {
			replayErrors[action.Text] = true
		}
	}

	// Find fixed errors (in original but not in replay)
	for _, action := range original.Actions {
		if action.Type == "error" {
			if !replayErrors[action.Text] {
				entry := DiffLogEntry{
					Type:       "error",
					Severity:   "high",
					Level:      "error",
					Message:    action.Text,
					Timestamp:  action.TimestampMs,
					Selector:   action.Selector,
					ActionType: action.Type,
				}
				result.MissingEvents = append(result.MissingEvents, entry)
			}
		}
	}
}

// detectValueChanges finds changed field values between recordings
func (c *Capture) detectValueChanges(original, replay *Recording, result *LogDiffResult) {
	// Build map of type actions from original by selector
	originalValues := make(map[string]string)
	for _, action := range original.Actions {
		if action.Type == "type" && action.Selector != "" {
			originalValues[action.Selector] = action.Text
		}
	}

	// Find changed values in replay
	for _, action := range replay.Actions {
		if action.Type == "type" && action.Selector != "" {
			if originalValue, exists := originalValues[action.Selector]; exists {
				if originalValue != action.Text {
					valueChange := ValueChange{
						Field:     action.Selector,
						FromValue: originalValue,
						ToValue:   action.Text,
						Timestamp: action.TimestampMs,
					}
					result.ChangedValues = append(result.ChangedValues, valueChange)
				}
			}
		}
	}
}

// determineStatus sets overall status and summary based on findings
func (c *Capture) determineStatus(result *LogDiffResult) {
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

// ============================================================================
// Categorization
// ============================================================================

// CategorizeActionTypes returns counts of each action type
func (c *Capture) CategorizeActionTypes(recording *Recording) map[string]int {
	counts := make(map[string]int)

	for _, action := range recording.Actions {
		counts[action.Type]++
	}

	return counts
}

// GetRegressionReport generates a human-readable regression report
func (result *LogDiffResult) GetRegressionReport() string {
	report := fmt.Sprintf("Log Diff Report\n")
	report += fmt.Sprintf("===============\n")
	report += fmt.Sprintf("Status: %s\n", result.Status)
	report += fmt.Sprintf("Summary: %s\n\n", result.Summary)

	// Action statistics
	report += fmt.Sprintf("Action Statistics:\n")
	report += fmt.Sprintf("  Original: %d actions\n", result.ActionStats.OriginalCount)
	report += fmt.Sprintf("    - Errors: %d\n", result.ActionStats.ErrorsOriginal)
	report += fmt.Sprintf("    - Clicks: %d\n", result.ActionStats.ClicksOriginal)
	report += fmt.Sprintf("    - Types: %d\n", result.ActionStats.TypesOriginal)
	report += fmt.Sprintf("    - Navigates: %d\n", result.ActionStats.NavigatesOriginal)

	report += fmt.Sprintf("  Replay: %d actions\n", result.ActionStats.ReplayCount)
	report += fmt.Sprintf("    - Errors: %d\n", result.ActionStats.ErrorsReplay)
	report += fmt.Sprintf("    - Clicks: %d\n", result.ActionStats.ClicksReplay)
	report += fmt.Sprintf("    - Types: %d\n", result.ActionStats.TypesReplay)
	report += fmt.Sprintf("    - Navigates: %d\n", result.ActionStats.NavigatesReplay)

	if len(result.NewErrors) > 0 {
		report += fmt.Sprintf("\nNew Errors (%d):\n", len(result.NewErrors))
		for i, err := range result.NewErrors {
			report += fmt.Sprintf("  %d. %s (at %dms)\n", i+1, err.Message, err.Timestamp)
		}
	}

	if len(result.MissingEvents) > 0 {
		report += fmt.Sprintf("\nFixed/Missing Events (%d):\n", len(result.MissingEvents))
		for i, event := range result.MissingEvents {
			report += fmt.Sprintf("  %d. %s (was at %dms)\n", i+1, event.Message, event.Timestamp)
		}
	}

	if len(result.ChangedValues) > 0 {
		report += fmt.Sprintf("\nChanged Values (%d):\n", len(result.ChangedValues))
		for i, change := range result.ChangedValues {
			report += fmt.Sprintf("  %d. %s: '%s' → '%s'\n", i+1, change.Field, change.FromValue, change.ToValue)
		}
	}

	return report
}
