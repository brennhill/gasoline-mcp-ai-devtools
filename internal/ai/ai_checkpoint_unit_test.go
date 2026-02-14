package ai

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/server"
	"github.com/dev-console/dev-console/internal/types"
)

type fakeLogReader struct {
	snapshot   server.LogSnapshot
	timestamps []time.Time
}

func (f *fakeLogReader) GetLogSnapshot() server.LogSnapshot { return f.snapshot }
func (f *fakeLogReader) GetLogCount() int                   { return len(f.snapshot.Entries) }
func (f *fakeLogReader) GetLogTotalAdded() int64            { return f.snapshot.TotalAdded }
func (f *fakeLogReader) GetLogEntries() []types.LogEntry    { return f.snapshot.Entries }
func (f *fakeLogReader) GetLogTimestamps() []time.Time {
	return append([]time.Time(nil), f.timestamps...)
}
func (f *fakeLogReader) GetOldestLogTime() time.Time {
	if len(f.timestamps) == 0 {
		return time.Time{}
	}
	return f.timestamps[0]
}
func (f *fakeLogReader) GetNewestLogTime() time.Time {
	if len(f.timestamps) == 0 {
		return time.Time{}
	}
	return f.timestamps[len(f.timestamps)-1]
}

func TestCheckpointComputeConsoleDiff_DeduplicatesByFingerprint(t *testing.T) {
	t.Parallel()

	now := time.Now()
	fake := &fakeLogReader{
		snapshot: server.LogSnapshot{
			Entries: []types.LogEntry{
				{"level": "error", "message": "Order 1234 failed with id 550e8400-e29b-41d4-a716-446655440000"},
				{"level": "error", "message": "Order 9999 failed with id 550e8400-e29b-41d4-a716-446655440111"},
				{"level": "warning", "message": "Slow response at 2024-01-01T10:00:00Z"},
				{"level": "info", "message": "normal log"},
			},
			TotalAdded: 4,
		},
		timestamps: []time.Time{now.Add(-4 * time.Second), now.Add(-3 * time.Second), now.Add(-2 * time.Second), now.Add(-1 * time.Second)},
	}

	cm := NewCheckpointManager(fake, capture.NewCapture())
	cp := &Checkpoint{LogTotal: 0}

	diff := cm.computeConsoleDiff(cp, "all")
	if diff.TotalNew != 4 {
		t.Fatalf("TotalNew = %d, want 4", diff.TotalNew)
	}
	if len(diff.Errors) != 1 || diff.Errors[0].Count != 2 {
		t.Fatalf("expected one deduped error with count=2, got %+v", diff.Errors)
	}
	if len(diff.Warnings) != 1 || diff.Warnings[0].Count != 1 {
		t.Fatalf("expected one warning, got %+v", diff.Warnings)
	}

	errorsOnly := cm.computeConsoleDiff(cp, "errors_only")
	if len(errorsOnly.Warnings) != 0 {
		t.Fatalf("errors_only should omit warnings, got %+v", errorsOnly.Warnings)
	}
}

func TestCheckpointComputeNetworkWebSocketAndActions(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cap := capture.NewCapture()
	cm := NewCheckpointManager(fake, cap)

	cap.AddNetworkBodies([]capture.NetworkBody{
		{URL: "https://example.com/api", Status: 500, Duration: 120},
		{URL: "https://example.com/slow", Status: 200, Duration: 350},
		{URL: "https://example.com/new-endpoint", Status: 200, Duration: 50},
	})
	cap.AddWebSocketEvents([]capture.WebSocketEvent{
		{Event: "open", URL: "wss://example.com/socket", ID: "ws-1"},
		{Event: "close", URL: "wss://example.com/socket", ID: "ws-1", CloseCode: 1006, CloseReason: "abnormal"},
		{Event: "error", URL: "wss://example.com/socket", ID: "ws-1", Data: "connection reset"},
	})
	cap.AddEnhancedActions([]capture.EnhancedAction{
		{Type: "click", URL: "https://example.com", Timestamp: 1000},
	})

	cp := &Checkpoint{
		NetworkTotal: 0,
		WSTotal:      0,
		ActionTotal:  0,
		KnownEndpoints: map[string]endpointState{
			"/api":  {Status: 200, Duration: 100},
			"/slow": {Status: 200, Duration: 100},
		},
	}

	netDiff := cm.computeNetworkDiff(cp)
	if len(netDiff.Failures) != 1 || netDiff.Failures[0].Path != "/api" {
		t.Fatalf("expected /api failure transition, got %+v", netDiff.Failures)
	}
	if len(netDiff.Degraded) != 1 || netDiff.Degraded[0].Path != "/slow" {
		t.Fatalf("expected /slow degraded latency, got %+v", netDiff.Degraded)
	}
	if len(netDiff.NewEndpoints) != 1 || netDiff.NewEndpoints[0] != "/new-endpoint" {
		t.Fatalf("expected one new endpoint, got %+v", netDiff.NewEndpoints)
	}

	wsDiff := cm.computeWebSocketDiff(cp, "all")
	if len(wsDiff.Connections) != 1 || len(wsDiff.Disconnections) != 1 || len(wsDiff.Errors) != 1 {
		t.Fatalf("unexpected websocket diff: %+v", wsDiff)
	}

	wsErrorsOnly := cm.computeWebSocketDiff(cp, "errors_only")
	if len(wsErrorsOnly.Disconnections) != 0 {
		t.Fatalf("errors_only should skip websocket disconnections, got %+v", wsErrorsOnly.Disconnections)
	}

	actDiff := cm.computeActionsDiff(cp)
	if actDiff.TotalNew != 1 || len(actDiff.Actions) != 1 || actDiff.Actions[0].Type != "click" {
		t.Fatalf("unexpected actions diff: %+v", actDiff)
	}
}

func TestCheckpointCreateAndEvictNamedCheckpoints(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	if err := cm.CreateCheckpoint("", "client-a"); err == nil {
		t.Fatal("CreateCheckpoint should reject empty name")
	}
	if err := cm.CreateCheckpoint(strings.Repeat("x", maxCheckpointNameLen+1), "client-a"); err == nil {
		t.Fatal("CreateCheckpoint should reject overly long names")
	}

	for i := 0; i < maxNamedCheckpoints+3; i++ {
		if err := cm.CreateCheckpoint(fmt.Sprintf("cp-%02d", i), "client-a"); err != nil {
			t.Fatalf("CreateCheckpoint(%d) failed: %v", i, err)
		}
	}

	if got := cm.GetNamedCheckpointCount(); got != maxNamedCheckpoints {
		t.Fatalf("GetNamedCheckpointCount() = %d, want %d", got, maxNamedCheckpoints)
	}

	// Oldest should be evicted after capacity is exceeded.
	if _, exists := cm.namedCheckpoints["client-a:cp-00"]; exists {
		t.Fatal("expected oldest checkpoint to be evicted")
	}
	if _, exists := cm.namedCheckpoints["client-a:cp-22"]; !exists {
		t.Fatal("expected newest checkpoint to be retained")
	}
}

func TestCheckpointUtilityHelpers(t *testing.T) {
	t.Parallel()

	msg := "user=123456 at 2024-01-01T10:00:00Z id=550e8400-e29b-41d4-a716-446655440000"
	fp := FingerprintMessage(msg)
	if strings.Contains(fp, "123456") || strings.Contains(fp, "550e8400") || strings.Contains(fp, "2024-01-01") {
		t.Fatalf("FingerprintMessage did not normalize dynamic values: %q", fp)
	}
	if !strings.Contains(fp, "{n}") || !strings.Contains(fp, "{uuid}") || !strings.Contains(fp, "{ts}") {
		t.Fatalf("FingerprintMessage missing placeholders: %q", fp)
	}

	long := strings.Repeat("a", maxMessageLen) + "🙂"
	truncated := truncateMessage(long)
	if len(truncated) > maxMessageLen {
		t.Fatalf("truncateMessage length = %d, want <= %d", len(truncated), maxMessageLen)
	}
	if !utf8.ValidString(truncated) {
		t.Fatalf("truncateMessage returned invalid UTF-8: %q", truncated)
	}

	if !containsString([]string{"a", "b", "c"}, "b") {
		t.Fatal("containsString should find existing element")
	}
	if containsString([]string{"a", "b", "c"}, "z") {
		t.Fatal("containsString should return false for missing element")
	}

	cm := &CheckpointManager{}
	severity := cm.determineSeverity(DiffResponse{
		Console: &ConsoleDiff{Errors: []ConsoleEntry{{Message: "boom", Count: 1}}},
	})
	if severity != "error" {
		t.Fatalf("determineSeverity() = %s, want error", severity)
	}

	summary := cm.buildSummary(DiffResponse{
		Severity: "warning",
		Console:  &ConsoleDiff{Warnings: []ConsoleEntry{{Message: "warn", Count: 2}}},
	})
	if !strings.Contains(summary, "2 new console warning(s)") {
		t.Fatalf("buildSummary unexpected output: %q", summary)
	}

	cleanSummary := cm.buildSummary(DiffResponse{Severity: "clean"})
	if cleanSummary != "No significant changes." {
		t.Fatalf("clean summary = %q, want default clean text", cleanSummary)
	}
}
