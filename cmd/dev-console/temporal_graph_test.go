package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Event Recording ---

func TestTemporalRecordError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{
		Type:        "error",
		Description: "TypeError: x is undefined at render (app.js:10)",
		Source:      "app.js:10",
		Origin:      "system",
	})

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events.Events))
	}
	if events.Events[0].Type != "error" {
		t.Fatalf("expected type=error, got %s", events.Events[0].Type)
	}
	if events.Events[0].Origin != "system" {
		t.Fatalf("expected origin=system, got %s", events.Events[0].Origin)
	}
}

func TestTemporalRecordRegression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{
		Type:        "regression",
		Description: "LCP regressed: 1200ms → 2400ms on /dashboard",
		Source:      "lcp_ms",
		Origin:      "system",
	})

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events.Events))
	}
	if events.Events[0].Type != "regression" {
		t.Fatalf("expected type=regression, got %s", events.Events[0].Type)
	}
}

func TestTemporalRecordAgentEvent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{
		Type:        "fix",
		Description: "Fixed null user in UserProfile",
		Origin:      "agent",
		Agent:       "claude-code",
	})

	events := tg.Query(TemporalQuery{})
	if events.Events[0].Origin != "agent" {
		t.Fatalf("expected origin=agent, got %s", events.Events[0].Origin)
	}
	if events.Events[0].Agent != "claude-code" {
		t.Fatalf("expected agent=claude-code, got %s", events.Events[0].Agent)
	}
}

func TestTemporalEventGetsID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{
		Type:        "error",
		Description: "Test error",
		Origin:      "system",
	})

	events := tg.Query(TemporalQuery{})
	if events.Events[0].ID == "" {
		t.Fatal("event should have an auto-generated ID")
	}
	if !strings.HasPrefix(events.Events[0].ID, "evt_") {
		t.Fatalf("event ID should start with evt_, got %s", events.Events[0].ID)
	}
}

func TestTemporalEventGetsTimestamp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{
		Type:        "error",
		Description: "Test error",
		Origin:      "system",
	})

	events := tg.Query(TemporalQuery{})
	if events.Events[0].Timestamp == "" {
		t.Fatal("event should have a timestamp")
	}
}

func TestTemporalDuplicateByFingerprint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	// Same error twice — should deduplicate
	for i := 0; i < 3; i++ {
		tg.RecordEvent(TemporalEvent{
			Type:        "error",
			Description: "TypeError: x is undefined at render (app.js:10)",
			Source:      "app.js:10",
			Origin:      "system",
		})
	}

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 1 {
		t.Fatalf("duplicate errors should be deduplicated, got %d events", len(events.Events))
	}
	if events.Events[0].OccurrenceCount != 3 {
		t.Fatalf("expected occurrence_count=3, got %d", events.Events[0].OccurrenceCount)
	}
}

// --- Event Links ---

func TestTemporalEventWithLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{
		Type:        "error",
		Description: "The error",
		Origin:      "system",
	})

	events := tg.Query(TemporalQuery{})
	errorID := events.Events[0].ID

	tg.RecordEvent(TemporalEvent{
		Type:        "fix",
		Description: "Fixed the error",
		Origin:      "agent",
		Links: []EventLink{
			{Target: errorID, Relationship: "resolved_by", Confidence: "explicit"},
		},
	})

	events = tg.Query(TemporalQuery{})
	if len(events.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events.Events))
	}
	fixEvent := events.Events[1]
	if len(fixEvent.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(fixEvent.Links))
	}
	if fixEvent.Links[0].Target != errorID {
		t.Fatalf("link target should be %s, got %s", errorID, fixEvent.Links[0].Target)
	}
}

// --- Querying ---

func TestTemporalQueryByType(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{Type: "error", Description: "An error", Origin: "system"})
	tg.RecordEvent(TemporalEvent{Type: "regression", Description: "A regression", Origin: "system"})
	tg.RecordEvent(TemporalEvent{Type: "error", Description: "Another error", Origin: "system"})

	events := tg.Query(TemporalQuery{Type: "error"})
	if len(events.Events) != 2 {
		t.Fatalf("expected 2 error events, got %d", len(events.Events))
	}
}

func TestTemporalQueryBySince(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	// Record an old event (simulate by writing directly)
	oldEvent := TemporalEvent{
		Type:        "error",
		Description: "Old error",
		Origin:      "system",
		Timestamp:   time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
		ID:          "evt_old",
		Status:      "active",
	}
	tg.appendEvent(oldEvent)

	// Record a recent event
	tg.RecordEvent(TemporalEvent{Type: "error", Description: "Recent error", Origin: "system"})

	events := tg.Query(TemporalQuery{Since: "1d"})
	if len(events.Events) != 1 {
		t.Fatalf("expected 1 event within 1d, got %d", len(events.Events))
	}
	if events.Events[0].Description != "Recent error" {
		t.Fatalf("expected recent error, got: %s", events.Events[0].Description)
	}
}

func TestTemporalQueryByRelatedTo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{Type: "error", Description: "Error A", Origin: "system"})
	events := tg.Query(TemporalQuery{})
	errorID := events.Events[0].ID

	tg.RecordEvent(TemporalEvent{
		Type:        "fix",
		Description: "Fix for A",
		Origin:      "agent",
		Links:       []EventLink{{Target: errorID, Relationship: "resolved_by", Confidence: "explicit"}},
	})
	tg.RecordEvent(TemporalEvent{Type: "error", Description: "Unrelated error", Origin: "system"})

	related := tg.Query(TemporalQuery{RelatedTo: errorID})
	if len(related.Events) != 1 {
		t.Fatalf("expected 1 related event, got %d", len(related.Events))
	}
	if related.Events[0].Description != "Fix for A" {
		t.Fatalf("expected fix event, got: %s", related.Events[0].Description)
	}
}

func TestTemporalQueryByPattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{Type: "error", Description: "TypeError in UserProfile", Origin: "system"})
	tg.RecordEvent(TemporalEvent{Type: "error", Description: "Network timeout", Origin: "system"})

	events := tg.Query(TemporalQuery{Pattern: "UserProfile"})
	if len(events.Events) != 1 {
		t.Fatalf("expected 1 matching event, got %d", len(events.Events))
	}
}

func TestTemporalQueryEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events.Events))
	}
	if events.TotalEvents != 0 {
		t.Fatalf("expected total=0, got %d", events.TotalEvents)
	}
}

// --- Persistence ---

func TestTemporalPersistence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	tg.RecordEvent(TemporalEvent{Type: "error", Description: "Persisted error", Origin: "system"})
	tg.Close()

	// Reload
	tg2 := NewTemporalGraph(dir)
	defer tg2.Close()
	events := tg2.Query(TemporalQuery{})
	if len(events.Events) != 1 {
		t.Fatalf("expected 1 persisted event, got %d", len(events.Events))
	}
	if events.Events[0].Description != "Persisted error" {
		t.Fatalf("expected persisted description, got: %s", events.Events[0].Description)
	}
}

func TestTemporalJSONLFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	tg.RecordEvent(TemporalEvent{Type: "error", Description: "Test", Origin: "system"})
	tg.Close()

	data, err := os.ReadFile(filepath.Join(dir, "history", "events.jsonl"))
	if err != nil {
		t.Fatalf("failed to read events file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 JSONL line, got %d", len(lines))
	}

	var event TemporalEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("failed to parse JSONL: %v", err)
	}
}

// --- Retention ---

func TestTemporalEviction90Days(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write an old event directly to the file
	histDir := filepath.Join(dir, "history")
	os.MkdirAll(histDir, 0755)
	oldEvent := TemporalEvent{
		ID:          "evt_old",
		Type:        "error",
		Description: "Very old error",
		Timestamp:   time.Now().Add(-100 * 24 * time.Hour).UTC().Format(time.RFC3339),
		Origin:      "system",
		Status:      "active",
	}
	data, _ := json.Marshal(oldEvent)
	os.WriteFile(filepath.Join(histDir, "events.jsonl"), append(data, '\n'), 0644)

	// Load — should evict the old event
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 0 {
		t.Fatalf("old event should be evicted, got %d events", len(events.Events))
	}
}

func TestTemporalRetentionKeepsRecent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	histDir := filepath.Join(dir, "history")
	os.MkdirAll(histDir, 0755)
	recentEvent := TemporalEvent{
		ID:          "evt_recent",
		Type:        "error",
		Description: "Recent error",
		Timestamp:   time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339),
		Origin:      "system",
		Status:      "active",
	}
	data, _ := json.Marshal(recentEvent)
	os.WriteFile(filepath.Join(histDir, "events.jsonl"), append(data, '\n'), 0644)

	tg := NewTemporalGraph(dir)
	defer tg.Close()

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 1 {
		t.Fatalf("recent event should be kept, got %d events", len(events.Events))
	}
}

// --- Edge Cases ---

func TestTemporalCorruptedLineSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	histDir := filepath.Join(dir, "history")
	os.MkdirAll(histDir, 0755)

	// Write one valid and one corrupted line
	validEvent := TemporalEvent{
		ID:          "evt_valid",
		Type:        "error",
		Description: "Valid error",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Origin:      "system",
		Status:      "active",
	}
	validData, _ := json.Marshal(validEvent)
	content := string(validData) + "\n" + "this is not json\n"
	os.WriteFile(filepath.Join(histDir, "events.jsonl"), []byte(content), 0644)

	tg := NewTemporalGraph(dir)
	defer tg.Close()

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 1 {
		t.Fatalf("should skip corrupted line, got %d events", len(events.Events))
	}
}

func TestTemporalNoHistoryFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 0 {
		t.Fatalf("no history file should yield 0 events, got %d", len(events.Events))
	}
}

func TestTemporalConcurrentAccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	done := make(chan bool, 100)
	for i := 0; i < 50; i++ {
		go func() {
			tg.RecordEvent(TemporalEvent{Type: "error", Description: "Concurrent", Origin: "system"})
			done <- true
		}()
		go func() {
			tg.Query(TemporalQuery{})
			done <- true
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}

// --- Configure record_event ---

func TestConfigureRecordEventValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	result, errMsg := handleRecordEvent(tg, map[string]interface{}{
		"type":        "fix",
		"description": "Fixed the bug",
	}, "claude-code")

	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !strings.Contains(result, "recorded") {
		t.Fatalf("expected confirmation, got: %s", result)
	}

	events := tg.Query(TemporalQuery{})
	if len(events.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events.Events))
	}
	if events.Events[0].Origin != "agent" {
		t.Fatalf("expected origin=agent, got %s", events.Events[0].Origin)
	}
}

func TestConfigureRecordEventMissingType(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	_, errMsg := handleRecordEvent(tg, map[string]interface{}{
		"description": "No type",
	}, "agent")

	if errMsg == "" {
		t.Fatal("expected error for missing type")
	}
}

func TestConfigureRecordEventWithLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := NewTemporalGraph(dir)
	defer tg.Close()

	tg.RecordEvent(TemporalEvent{Type: "error", Description: "Error", Origin: "system"})
	events := tg.Query(TemporalQuery{})
	errorID := events.Events[0].ID

	handleRecordEvent(tg, map[string]interface{}{
		"type":        "fix",
		"description": "Fixed it",
		"related_to":  errorID,
	}, "claude-code")

	events = tg.Query(TemporalQuery{RelatedTo: errorID})
	if len(events.Events) != 1 {
		t.Fatalf("expected 1 related event, got %d", len(events.Events))
	}
}

// --- Parse Since Duration ---

func TestParseSinceDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1h", time.Hour},
		{"1d", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
		{"", 7 * 24 * time.Hour}, // default
	}

	for _, tt := range tests {
		got := parseSinceDuration(tt.input)
		if got != tt.expected {
			t.Errorf("parseSinceDuration(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
