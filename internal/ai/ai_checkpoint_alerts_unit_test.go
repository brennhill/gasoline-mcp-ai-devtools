package ai

import (
	"fmt"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/performance"
	"github.com/dev-console/dev-console/internal/server"
	gasTypes "github.com/dev-console/dev-console/internal/types"
)

func float64Ptr(v float64) *float64 {
	return &v
}

func TestCheckpointDetectAndStoreAlertsLifecycle(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())

	// SampleCount < 1 should not alert.
	cm.DetectAndStoreAlerts(performance.PerformanceSnapshot{URL: "https://example.test"}, performance.PerformanceBaseline{SampleCount: 0})
	if got := len(cm.pendingAlerts); got != 0 {
		t.Fatalf("pending alerts = %d, want 0 for first-sample baseline", got)
	}

	baseFCP := 1000.0
	baseLCP := 1200.0
	baseCLS := 0.05
	baseline := performance.PerformanceBaseline{
		SampleCount: 2,
		Timing: performance.BaselineTiming{
			Load:                   1000,
			FirstContentfulPaint:   float64Ptr(baseFCP),
			LargestContentfulPaint: float64Ptr(baseLCP),
			TimeToFirstByte:        200,
		},
		Network: performance.BaselineNetwork{
			TransferSize: 1000,
		},
		CLS: float64Ptr(baseCLS),
	}

	curFCP := 1300.0
	curLCP := 1500.0
	curCLS := 0.20
	snapshot := performance.PerformanceSnapshot{
		URL: "https://example.test/page",
		Timing: performance.PerformanceTiming{
			Load:                   1300,
			FirstContentfulPaint:   float64Ptr(curFCP),
			LargestContentfulPaint: float64Ptr(curLCP),
			TimeToFirstByte:        350,
		},
		Network: performance.NetworkSummary{
			TransferSize: 1400,
		},
		CLS: float64Ptr(curCLS),
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)
	if got := len(cm.pendingAlerts); got != 1 {
		t.Fatalf("pending alerts = %d, want 1", got)
	}
	alert := cm.pendingAlerts[0]
	if alert.ID != 1 {
		t.Fatalf("alert.ID = %d, want 1", alert.ID)
	}
	if alert.Type != "regression" {
		t.Fatalf("alert.Type = %q, want regression", alert.Type)
	}
	if alert.URL != snapshot.URL {
		t.Fatalf("alert.URL = %q, want %q", alert.URL, snapshot.URL)
	}
	if len(alert.Metrics) != 6 {
		t.Fatalf("alert metrics = %d, want 6 regression metrics", len(alert.Metrics))
	}
	if _, ok := alert.Metrics["load"]; !ok {
		t.Fatal("expected load regression metric")
	}
	if _, ok := alert.Metrics["ttfb"]; !ok {
		t.Fatal("expected ttfb regression metric")
	}

	beforeMark := cm.getPendingAlerts(0)
	if len(beforeMark) != 1 {
		t.Fatalf("getPendingAlerts before delivery = %d, want 1", len(beforeMark))
	}
	cm.markAlertsDelivered()
	if cm.pendingAlerts[0].DeliveredAt != cm.alertDelivery {
		t.Fatalf("DeliveredAt = %d, want %d", cm.pendingAlerts[0].DeliveredAt, cm.alertDelivery)
	}
	afterMark := cm.getPendingAlerts(cm.alertDelivery)
	if len(afterMark) != 0 {
		t.Fatalf("getPendingAlerts after delivery = %d, want 0", len(afterMark))
	}

	// No regression for the same URL should resolve existing alerts for that URL.
	noRegression := snapshot
	noRegression.Timing.Load = baseline.Timing.Load
	noRegression.Timing.TimeToFirstByte = baseline.Timing.TimeToFirstByte
	noRegression.Network.TransferSize = baseline.Network.TransferSize
	*noRegression.CLS = *baseline.CLS
	*noRegression.Timing.FirstContentfulPaint = *baseline.Timing.FirstContentfulPaint
	*noRegression.Timing.LargestContentfulPaint = *baseline.Timing.LargestContentfulPaint
	cm.DetectAndStoreAlerts(noRegression, baseline)
	if got := len(cm.pendingAlerts); got != 0 {
		t.Fatalf("pending alerts after resolution = %d, want 0", got)
	}
}

func TestCheckpointDetectAndStoreAlertsCapsPendingBuffer(t *testing.T) {
	t.Parallel()

	cm := NewCheckpointManager(&fakeLogReader{}, capture.NewCapture())
	baseline := performance.PerformanceBaseline{
		SampleCount: 1,
		Timing: performance.BaselineTiming{
			Load: 1000,
		},
	}

	for i := 0; i < maxPendingAlerts+2; i++ {
		snapshot := performance.PerformanceSnapshot{
			URL: fmt.Sprintf("https://example.test/%d", i),
			Timing: performance.PerformanceTiming{
				Load: 1300,
			},
		}
		cm.DetectAndStoreAlerts(snapshot, baseline)
	}

	if got := len(cm.pendingAlerts); got != maxPendingAlerts {
		t.Fatalf("pending alerts = %d, want cap %d", got, maxPendingAlerts)
	}
	if cm.pendingAlerts[0].URL != "https://example.test/2" {
		t.Fatalf("oldest retained alert URL = %q, want /2 after cap eviction", cm.pendingAlerts[0].URL)
	}
	if cm.pendingAlerts[len(cm.pendingAlerts)-1].URL != "https://example.test/11" {
		t.Fatalf("newest alert URL = %q, want /11", cm.pendingAlerts[len(cm.pendingAlerts)-1].URL)
	}
}

func TestCheckpointGetChangesSinceAutoNamedAndSeverityFilter(t *testing.T) {
	t.Parallel()

	t0 := time.Now().Add(-10 * time.Second).UTC()
	t1 := t0.Add(2 * time.Second)

	logReader := &fakeLogReader{
		snapshot: serverSnapshotForTests([]map[string]any{
			{"level": "warn", "message": "first warning"},
			{"level": "error", "message": "first error"},
		}),
		timestamps: []time.Time{t0, t1},
	}
	cap := capture.NewCapture()
	cap.AddWebSocketEvents([]capture.WebSocketEvent{
		{Event: "close", URL: "wss://example.test/ws", CloseCode: 1001, CloseReason: "closed"},
	})

	cm := NewCheckpointManager(logReader, cap)
	cm.alertDelivery = 2
	cm.pendingAlerts = []gasTypes.PerformanceAlert{
		{ID: 1, URL: "https://example.test", DeliveredAt: 0},
	}

	resp := cm.GetChangesSince(GetChangesSinceParams{
		Severity: "errors_only",
		Include:  []string{"console", "websocket"},
	}, "")
	if resp.Console == nil || len(resp.Console.Errors) != 1 {
		t.Fatalf("console diff errors = %+v, want one error", resp.Console)
	}
	if len(resp.Console.Warnings) != 0 {
		t.Fatalf("errors_only should strip warnings, got %+v", resp.Console.Warnings)
	}
	if resp.WebSocket != nil {
		t.Fatalf("errors_only should omit ws-only warnings/disconnections, got %+v", resp.WebSocket)
	}
	if len(resp.PerformanceAlerts) != 1 {
		t.Fatalf("performance alerts = %d, want 1 on first auto poll", len(resp.PerformanceAlerts))
	}
	if cm.pendingAlerts[0].DeliveredAt != 2 {
		t.Fatalf("DeliveredAt after auto poll = %d, want 2", cm.pendingAlerts[0].DeliveredAt)
	}

	autoBefore := cm.autoCheckpoint
	if autoBefore == nil {
		t.Fatal("auto checkpoint should be initialized after first auto poll")
	}

	second := cm.GetChangesSince(GetChangesSinceParams{
		Severity: "errors_only",
		Include:  []string{"console"},
	}, "")
	if second.Console != nil {
		t.Fatalf("second auto poll should have no new console entries, got %+v", second.Console)
	}
	if len(second.PerformanceAlerts) != 0 {
		t.Fatalf("second auto poll alerts = %d, want 0 after delivery", len(second.PerformanceAlerts))
	}
	autoBefore = cm.autoCheckpoint

	if err := cm.CreateCheckpoint("named", "client-a"); err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}

	logReader.snapshot.Entries = append(logReader.snapshot.Entries, map[string]any{
		"level":   "error",
		"message": "new named error",
	})
	logReader.snapshot.TotalAdded++
	logReader.timestamps = append(logReader.timestamps, t1.Add(2*time.Second))

	named := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "named",
		Include:    []string{"console"},
	}, "client-a")
	if named.Console == nil || named.Console.TotalNew != 1 {
		t.Fatalf("named checkpoint diff = %+v, want one new entry", named.Console)
	}
	if cm.autoCheckpoint != autoBefore {
		t.Fatal("named checkpoint query should not advance auto checkpoint")
	}
}

func TestCheckpointResolveTimestampAndHelpers(t *testing.T) {
	t.Parallel()

	t0 := time.Now().Add(-20 * time.Second).UTC()
	t1 := t0.Add(5 * time.Second)
	t2 := t1.Add(5 * time.Second)

	logReader := &fakeLogReader{
		snapshot: serverSnapshotForTests([]map[string]any{
			{"level": "info", "message": "a"},
			{"level": "info", "message": "b"},
			{"level": "info", "message": "c"},
		}),
		timestamps: []time.Time{t0, t1, t2},
	}
	cap := capture.NewCapture()
	cap.AddNetworkBodies([]capture.NetworkBody{
		{URL: "https://example.test/api", Status: 200, Duration: 80},
	})
	cm := NewCheckpointManager(logReader, cap)

	cp := cm.resolveTimestampCheckpoint(t1.Format(time.RFC3339Nano))
	if cp == nil {
		t.Fatal("resolveTimestampCheckpoint returned nil for valid RFC3339Nano")
	}
	if cp.LogTotal != 2 {
		t.Fatalf("cp.LogTotal = %d, want 2 entries at/under t1", cp.LogTotal)
	}

	cpRFC3339 := cm.resolveTimestampCheckpoint(t1.Format(time.RFC3339))
	if cpRFC3339 == nil {
		t.Fatal("resolveTimestampCheckpoint returned nil for valid RFC3339")
	}

	if bad := cm.resolveTimestampCheckpoint("not-a-time"); bad != nil {
		t.Fatalf("resolveTimestampCheckpoint on invalid value = %+v, want nil", bad)
	}

	if got := cm.findPositionAtTime([]time.Time{}, 7, t1); got != 7 {
		t.Fatalf("findPositionAtTime empty addedAt = %d, want 7", got)
	}
	if got := cm.findPositionAtTime([]time.Time{t0, t1, t2}, 1, t0.Add(-1*time.Second)); got != 0 {
		t.Fatalf("findPositionAtTime should floor to zero, got %d", got)
	}

	existing := map[string]endpointState{
		"/old": {Status: 204, Duration: 10},
	}
	known := cm.buildKnownEndpoints(existing)
	if known["/old"].Status != 204 {
		t.Fatalf("expected existing endpoint to be retained, got %+v", known["/old"])
	}
	if got := known["/api"]; got.Status != 200 || got.Duration != 80 {
		t.Fatalf("expected /api endpoint from capture, got %+v", got)
	}

	if !cm.shouldInclude(nil, "console") {
		t.Fatal("shouldInclude(nil, category) should default to true")
	}
	if cm.shouldInclude([]string{"network"}, "console") {
		t.Fatal("shouldInclude should return false for non-listed category")
	}
}

func serverSnapshotForTests(entries []map[string]any) server.LogSnapshot {
	typed := make([]gasTypes.LogEntry, 0, len(entries))
	for _, entry := range entries {
		typed = append(typed, entry)
	}
	return server.LogSnapshot{
		Entries:    typed,
		TotalAdded: int64(len(typed)),
	}
}
