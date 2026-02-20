// playback_logdiff_test.go — Tests for playback status, log diff, and helper functions.
package recording

import (
	"strings"
	"testing"
	"time"
)

// ============================================
// GetPlaybackStatus Tests
// ============================================

func TestNewGetPlaybackStatus_AllOK(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	session := &PlaybackSession{
		StartedAt:        time.Now().Add(-1 * time.Second),
		ActionsExecuted:  5,
		ActionsFailed:    0,
		Results:          make([]PlaybackResult, 5),
		SelectorFailures: map[string]int{},
	}

	status := mgr.GetPlaybackStatus(session)

	if status["status"] != "ok" {
		t.Errorf("status = %v, want ok", status["status"])
	}
	if status["actions_executed"] != 5 {
		t.Errorf("actions_executed = %v, want 5", status["actions_executed"])
	}
	if status["actions_failed"] != 0 {
		t.Errorf("actions_failed = %v, want 0", status["actions_failed"])
	}
	if status["actions_total"] != 5 {
		t.Errorf("actions_total = %v, want 5", status["actions_total"])
	}
	if status["results_count"] != 5 {
		t.Errorf("results_count = %v, want 5", status["results_count"])
	}
}

func TestNewGetPlaybackStatus_Partial(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	session := &PlaybackSession{
		StartedAt:        time.Now().Add(-2 * time.Second),
		ActionsExecuted:  3,
		ActionsFailed:    2,
		Results:          make([]PlaybackResult, 5),
		SelectorFailures: map[string]int{"css": 2},
	}

	status := mgr.GetPlaybackStatus(session)

	if status["status"] != "partial" {
		t.Errorf("status = %v, want partial", status["status"])
	}
	if status["actions_total"] != 5 {
		t.Errorf("actions_total = %v, want 5", status["actions_total"])
	}
}

func TestNewGetPlaybackStatus_Failed(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	session := &PlaybackSession{
		StartedAt:        time.Now().Add(-1 * time.Second),
		ActionsExecuted:  0,
		ActionsFailed:    3,
		Results:          make([]PlaybackResult, 3),
		SelectorFailures: map[string]int{},
	}

	status := mgr.GetPlaybackStatus(session)

	if status["status"] != "failed" {
		t.Errorf("status = %v, want failed", status["status"])
	}
}

func TestNewGetPlaybackStatus_DurationPositive(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	session := &PlaybackSession{
		StartedAt:        time.Now().Add(-100 * time.Millisecond),
		ActionsExecuted:  1,
		SelectorFailures: map[string]int{},
	}

	status := mgr.GetPlaybackStatus(session)

	durationMs, ok := status["duration_ms"].(int64)
	if !ok {
		t.Fatal("duration_ms not int64")
	}
	if durationMs < 0 {
		t.Errorf("duration_ms = %d, want >= 0", durationMs)
	}
}

// ============================================
// LogDiff Tests
// ============================================

func TestNewLogDiffResult_GetRegressionReport(t *testing.T) {
	t.Parallel()

	result := &LogDiffResult{
		Status:            "regression",
		OriginalRecording: "orig-123",
		ReplayRecording:   "replay-456",
		Summary:           "REGRESSION: 1 new errors detected",
		NewErrors: []DiffLogEntry{
			{
				Type:       "error",
				Severity:   "high",
				Level:      "error",
				Message:    "TypeError: undefined is not a function",
				Timestamp:  1500,
				Selector:   "#btn",
				ActionType: "error",
			},
		},
		MissingEvents: []DiffLogEntry{},
		ChangedValues: []ValueChange{
			{
				Field:     "#email",
				FromValue: "old@test.com",
				ToValue:   "new@test.com",
				Timestamp: 2000,
			},
		},
		ActionStats: ActionComparison{
			OriginalCount:     10,
			ReplayCount:       12,
			ErrorsOriginal:    0,
			ErrorsReplay:      1,
			ClicksOriginal:    5,
			ClicksReplay:      5,
			TypesOriginal:     3,
			TypesReplay:       3,
			NavigatesOriginal: 2,
			NavigatesReplay:   3,
		},
	}

	report := result.GetRegressionReport()

	if !strings.Contains(report, "regression") {
		t.Error("report should contain 'regression'")
	}
	if !strings.Contains(report, "TypeError: undefined is not a function") {
		t.Error("report should contain the new error message")
	}
	if !strings.Contains(report, "#email") {
		t.Error("report should contain the changed field")
	}
	if !strings.Contains(report, "old@test.com") {
		t.Error("report should contain the original value")
	}
	if !strings.Contains(report, "new@test.com") {
		t.Error("report should contain the new value")
	}
	if !strings.Contains(report, "Original: 10 actions") {
		t.Error("report should contain original action count")
	}
	if !strings.Contains(report, "Replay: 12 actions") {
		t.Error("report should contain replay action count")
	}
}

func TestNewLogDiffResult_MatchReport(t *testing.T) {
	t.Parallel()

	result := &LogDiffResult{
		Status:        "match",
		Summary:       "All logs match (0 new errors, 0 missing events)",
		NewErrors:     []DiffLogEntry{},
		MissingEvents: []DiffLogEntry{},
		ChangedValues: []ValueChange{},
		ActionStats:   ActionComparison{OriginalCount: 5, ReplayCount: 5},
	}

	report := result.GetRegressionReport()

	if !strings.Contains(report, "match") {
		t.Error("report should contain 'match'")
	}
	if strings.Contains(report, "New Errors") {
		t.Error("match report should not contain 'New Errors' section")
	}
}

func TestNewLogDiffResult_FixedReport(t *testing.T) {
	t.Parallel()

	result := &LogDiffResult{
		Status:  "fixed",
		Summary: "FIXED: 2 errors no longer appear",
		MissingEvents: []DiffLogEntry{
			{Type: "error", Message: "Fixed error 1", Timestamp: 1000},
			{Type: "error", Message: "Fixed error 2", Timestamp: 2000},
		},
		NewErrors:     []DiffLogEntry{},
		ChangedValues: []ValueChange{},
		ActionStats:   ActionComparison{},
	}

	report := result.GetRegressionReport()

	if !strings.Contains(report, "fixed") {
		t.Error("report should contain 'fixed'")
	}
	if !strings.Contains(report, "Fixed error 1") {
		t.Error("report should contain fixed error message")
	}
	if !strings.Contains(report, "Fixed/Missing Events (2)") {
		t.Error("report should show missing events count")
	}
}

// ============================================
// CountActionTypes Tests
// ============================================

func TestNewCountActionTypes(t *testing.T) {
	t.Parallel()

	actions := []RecordingAction{
		{Type: "error"}, {Type: "click"}, {Type: "click"},
		{Type: "type"}, {Type: "navigate"}, {Type: "navigate"},
		{Type: "navigate"}, {Type: "scroll"},
	}

	errors, clicks, types, navigates := CountActionTypes(actions)

	if errors != 1 {
		t.Errorf("errors = %d, want 1", errors)
	}
	if clicks != 2 {
		t.Errorf("clicks = %d, want 2", clicks)
	}
	if types != 1 {
		t.Errorf("types = %d, want 1", types)
	}
	if navigates != 3 {
		t.Errorf("navigates = %d, want 3", navigates)
	}
}

func TestNewCountActionTypes_Empty(t *testing.T) {
	t.Parallel()

	errors, clicks, types, navigates := CountActionTypes([]RecordingAction{})

	if errors != 0 || clicks != 0 || types != 0 || navigates != 0 {
		t.Errorf("all counts should be 0, got errors=%d, clicks=%d, types=%d, navigates=%d",
			errors, clicks, types, navigates)
	}
}

func TestNewCountActionTypes_UnknownTypes(t *testing.T) {
	t.Parallel()

	actions := []RecordingAction{
		{Type: "scroll"}, {Type: "unknown"}, {Type: "custom"},
	}

	errors, clicks, types, navigates := CountActionTypes(actions)

	if errors != 0 || clicks != 0 || types != 0 || navigates != 0 {
		t.Errorf("unknown types should not be counted, got errors=%d, clicks=%d, types=%d, navigates=%d",
			errors, clicks, types, navigates)
	}
}

// ============================================
// BuildTypeValueMap Tests
// ============================================

func TestNewBuildTypeValueMap(t *testing.T) {
	t.Parallel()

	actions := []RecordingAction{
		{Type: "type", Selector: "#email", Text: "user@test.com"},
		{Type: "type", Selector: "#password", Text: "secret123"},
		{Type: "click", Selector: "#submit"},
		{Type: "type", Selector: "", Text: "no-sel"},
	}

	values := BuildTypeValueMap(actions)

	if values["#email"] != "user@test.com" {
		t.Errorf("values[#email] = %q, want user@test.com", values["#email"])
	}
	if values["#password"] != "secret123" {
		t.Errorf("values[#password] = %q, want secret123", values["#password"])
	}
	if _, ok := values["#submit"]; ok {
		t.Error("click action should not be in type value map")
	}
	if len(values) != 2 {
		t.Errorf("values len = %d, want 2", len(values))
	}
}

func TestNewBuildTypeValueMap_Empty(t *testing.T) {
	t.Parallel()

	values := BuildTypeValueMap([]RecordingAction{})
	if len(values) != 0 {
		t.Errorf("values len = %d, want 0", len(values))
	}
}
