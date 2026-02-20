// logdiff_unit_test.go — Unit tests for log diff internals.
package recording

import (
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

func persistRecordingForLogDiff(t *testing.T, mgr *RecordingManager, id string, actions []RecordingAction) {
	t.Helper()

	rec := &Recording{
		ID:          id,
		Name:        id,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		StartURL:    "https://app.example.com",
		ActionCount: len(actions),
		Actions:     actions,
	}
	if err := mgr.persistRecordingToDisk(rec); err != nil {
		t.Fatalf("persistRecordingToDisk(%q) error = %v", id, err)
	}
}

func TestDiffRecordingsRegressionAndValueChanges(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	mgr := NewRecordingManager()

	persistRecordingForLogDiff(t, mgr, "orig", []RecordingAction{
		{Type: "click", Selector: "#submit", TimestampMs: 10},
		{Type: "type", Selector: "#email", Text: "before@example.com", TimestampMs: 20},
		{Type: "error", Text: "E1 existing", TimestampMs: 30},
	})
	persistRecordingForLogDiff(t, mgr, "replay", []RecordingAction{
		{Type: "click", Selector: "#submit", TimestampMs: 15},
		{Type: "type", Selector: "#email", Text: "after@example.com", TimestampMs: 25},
		{Type: "error", Text: "E1 existing", TimestampMs: 35},
		{Type: "error", Text: "E2 new regression", TimestampMs: 45},
	})

	result, err := mgr.DiffRecordings("orig", "replay")
	if err != nil {
		t.Fatalf("DiffRecordings() error = %v", err)
	}

	if result.Status != "regression" {
		t.Fatalf("result.Status = %q, want regression", result.Status)
	}
	if len(result.NewErrors) != 1 || result.NewErrors[0].Message != "E2 new regression" {
		t.Fatalf("unexpected NewErrors: %+v", result.NewErrors)
	}
	if len(result.ChangedValues) != 1 || result.ChangedValues[0].Field != "#email" {
		t.Fatalf("unexpected ChangedValues: %+v", result.ChangedValues)
	}
	if result.ActionStats.OriginalCount != 3 || result.ActionStats.ReplayCount != 4 {
		t.Fatalf("unexpected ActionStats counts: %+v", result.ActionStats)
	}
	if result.ActionStats.ErrorsOriginal != 1 || result.ActionStats.ErrorsReplay != 2 {
		t.Fatalf("unexpected ActionStats error counts: %+v", result.ActionStats)
	}

	report := result.GetRegressionReport()
	if !strings.Contains(report, "Status: regression") {
		t.Fatalf("report missing status: %s", report)
	}
	if !strings.Contains(report, "New Errors (1)") {
		t.Fatalf("report missing new errors section: %s", report)
	}
	if !strings.Contains(report, "Changed Values (1)") {
		t.Fatalf("report missing changed values section: %s", report)
	}
}

func TestDiffRecordingsLoadError(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	mgr := NewRecordingManager()
	if _, err := mgr.DiffRecordings("missing-original", "missing-replay"); err == nil || !strings.Contains(err.Error(), "logdiff_load_original_failed") {
		t.Fatalf("DiffRecordings(missing, missing) error = %v, want logdiff_load_original_failed", err)
	}
}

func TestLogDiffStatusBranchesAndHelpers(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	r := &LogDiffResult{NewErrors: []DiffLogEntry{{Message: "new"}}}
	mgr.determineStatus(r)
	if r.Status != "regression" {
		t.Fatalf("determineStatus(regression) = %q, want regression", r.Status)
	}

	r = &LogDiffResult{MissingEvents: []DiffLogEntry{{Message: "fixed"}}}
	mgr.determineStatus(r)
	if r.Status != "fixed" {
		t.Fatalf("determineStatus(fixed) = %q, want fixed", r.Status)
	}

	r = &LogDiffResult{ChangedValues: []ValueChange{{Field: "#email"}}}
	mgr.determineStatus(r)
	if r.Status != "changed" {
		t.Fatalf("determineStatus(changed) = %q, want changed", r.Status)
	}

	r = &LogDiffResult{}
	mgr.determineStatus(r)
	if r.Status != "match" {
		t.Fatalf("determineStatus(match) = %q, want match", r.Status)
	}

	counts := mgr.CategorizeActionTypes(&Recording{
		Actions: []RecordingAction{
			{Type: "click"},
			{Type: "click"},
			{Type: "type"},
		},
	})
	if counts["click"] != 2 || counts["type"] != 1 {
		t.Fatalf("CategorizeActionTypes() = %+v, want click=2,type=1", counts)
	}

	stats := mgr.compareActions(
		&Recording{
			ActionCount: 3,
			Actions: []RecordingAction{
				{Type: "error"},
				{Type: "click"},
				{Type: "navigate"},
			},
		},
		&Recording{
			ActionCount: 2,
			Actions: []RecordingAction{
				{Type: "type"},
				{Type: "click"},
			},
		},
	)
	if stats.OriginalCount != 3 || stats.ReplayCount != 2 {
		t.Fatalf("compareActions counts = %+v", stats)
	}
	if stats.ErrorsOriginal != 1 || stats.ClicksOriginal != 1 || stats.NavigatesOriginal != 1 {
		t.Fatalf("compareActions original breakdown = %+v", stats)
	}
	if stats.TypesReplay != 1 || stats.ClicksReplay != 1 {
		t.Fatalf("compareActions replay breakdown = %+v", stats)
	}
}
