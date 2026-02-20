// playback_unit_test.go — Unit tests for playback engine internals.
package recording

import (
	"strings"
	"testing"
	"time"
)

func TestPlaybackStartAndExecute(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	if _, err := mgr.StartPlayback("missing-recording"); err == nil || !strings.Contains(err.Error(), "playback_recording_not_found") {
		t.Fatalf("StartPlayback(missing) error = %v, want playback_recording_not_found", err)
	}

	mgr.recordings["empty"] = &Recording{ID: "empty", Actions: nil}
	if _, err := mgr.StartPlayback("empty"); err == nil || !strings.Contains(err.Error(), "playback_no_actions") {
		t.Fatalf("StartPlayback(empty) error = %v, want playback_no_actions", err)
	}

	mgr.recordings["flow"] = &Recording{
		ID: "flow",
		Actions: []RecordingAction{
			{Type: "navigate"},
			{Type: "click", Selector: "#submit"},
			{Type: "type", Text: "hello"},
			{Type: "unknown"},
		},
	}

	session, err := mgr.ExecutePlayback("flow")
	if err != nil {
		t.Fatalf("ExecutePlayback(flow) error = %v", err)
	}
	if got := len(session.Results); got != 4 {
		t.Fatalf("len(session.Results) = %d, want 4", got)
	}
	if session.ActionsExecuted != 3 {
		t.Fatalf("ActionsExecuted = %d, want 3", session.ActionsExecuted)
	}
	if session.ActionsFailed != 1 {
		t.Fatalf("ActionsFailed = %d, want 1", session.ActionsFailed)
	}
	if session.Results[3].Status != "error" || !strings.Contains(session.Results[3].Error, "unknown_action_type") {
		t.Fatalf("unexpected unknown action result: %+v", session.Results[3])
	}
}

func TestExecuteClickWithHealingStrategies(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	dataTestID := mgr.executeClickWithHealing(RecordingAction{Type: "click", DataTestID: "login"})
	if dataTestID.Status != "ok" || dataTestID.SelectorUsed != "data-testid" {
		t.Fatalf("data-testid strategy failed: %+v", dataTestID)
	}

	css := mgr.executeClickWithHealing(RecordingAction{Type: "click", Selector: ".primary-button"})
	if css.Status != "ok" || css.SelectorUsed != "css" {
		t.Fatalf("css strategy failed: %+v", css)
	}

	nearby := mgr.executeClickWithHealing(RecordingAction{Type: "click", X: 10, Y: 20})
	if nearby.Status != "ok" || nearby.SelectorUsed != "nearby_xy" {
		t.Fatalf("nearby strategy failed: %+v", nearby)
	}

	lastKnown := mgr.executeClickWithHealing(RecordingAction{Type: "click", ScreenshotPath: "shot.png"})
	if lastKnown.Status != "ok" || lastKnown.SelectorUsed != "last_known" {
		t.Fatalf("last-known strategy failed: %+v", lastKnown)
	}

	failed := mgr.executeClickWithHealing(RecordingAction{Type: "click"})
	if failed.Status != "error" || !strings.Contains(failed.Error, "selector_not_found") {
		t.Fatalf("failed strategy result = %+v, want selector_not_found error", failed)
	}
}

func TestTryClickSelectorValidation(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()
	action := RecordingAction{Type: "click"}

	if mgr.tryClickSelector("", action) {
		t.Fatal("empty selector should fail")
	}
	if !mgr.tryClickSelector("[data-testid=submit]", action) {
		t.Fatal("data-testid selector should pass")
	}
	if !mgr.tryClickSelector(".btn", action) {
		t.Fatal("class selector should pass")
	}
	if !mgr.tryClickSelector("#submit", action) {
		t.Fatal("id selector should pass")
	}
	if !mgr.tryClickSelector("[role=button]", action) {
		t.Fatal("attribute selector should pass")
	}
	if mgr.tryClickSelector("div > button", action) {
		t.Fatal("unsupported selector prefix should fail")
	}
}

func TestDetectFragileSelectorsAndStatus(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	// Needs at least 2 sessions.
	if got := mgr.DetectFragileSelectors([]*PlaybackSession{{}}); len(got) != 0 {
		t.Fatalf("DetectFragileSelectors(single run) = %+v, want empty", got)
	}

	s1 := &PlaybackSession{
		Results: []PlaybackResult{
			{ActionType: "click", SelectorUsed: "css", Status: "error"},
			{ActionType: "click", SelectorUsed: "css", Status: "error"},
			{ActionType: "click", SelectorUsed: "data-testid", Status: "ok"},
		},
	}
	s2 := &PlaybackSession{
		Results: []PlaybackResult{
			{ActionType: "click", SelectorUsed: "css", Status: "ok"},
			{ActionType: "click", SelectorUsed: "data-testid", Status: "ok"},
		},
	}
	fragile := mgr.DetectFragileSelectors([]*PlaybackSession{s1, s2})
	if !fragile["css:css"] {
		t.Fatalf("expected css selector to be marked fragile, got %+v", fragile)
	}
	if fragile["data-testid:data-testid"] {
		t.Fatalf("data-testid selector should not be fragile, got %+v", fragile)
	}

	now := time.Now()
	failedStatus := mgr.GetPlaybackStatus(&PlaybackSession{StartedAt: now, ActionsExecuted: 0, ActionsFailed: 0})
	if failedStatus["status"] != "failed" {
		t.Fatalf("status for zero executed actions = %v, want failed", failedStatus["status"])
	}

	partialStatus := mgr.GetPlaybackStatus(&PlaybackSession{
		StartedAt:       now.Add(-5 * time.Millisecond),
		ActionsExecuted: 3,
		ActionsFailed:   1,
		Results:         []PlaybackResult{{}, {}, {}, {}},
	})
	if partialStatus["status"] != "partial" {
		t.Fatalf("status for mixed results = %v, want partial", partialStatus["status"])
	}
}
