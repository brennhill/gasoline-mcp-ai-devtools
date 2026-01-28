// streaming_test.go — Tests for Context Streaming / Active push notifications.
// Covers StreamState configuration, throttling, dedup, rate limiting,
// severity filtering, URL filtering, and MCP notification emission.
package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

// ============================================
// StreamState creation and configuration
// ============================================

func TestStreamState_DefaultConfig(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()

	if ss.Config.Enabled {
		t.Error("expected streaming disabled by default")
	}
	if ss.Config.ThrottleSeconds != 5 {
		t.Errorf("expected default throttle=5, got %d", ss.Config.ThrottleSeconds)
	}
	if ss.Config.SeverityMin != "warning" {
		t.Errorf("expected default severity_min=warning, got %q", ss.Config.SeverityMin)
	}
}

func TestStreamState_Enable(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()

	result := ss.Configure("enable", []string{"errors", "network_errors"}, 10, "", "error")

	if !ss.Config.Enabled {
		t.Error("expected streaming enabled after enable action")
	}
	if len(ss.Config.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(ss.Config.Events))
	}
	if ss.Config.ThrottleSeconds != 10 {
		t.Errorf("expected throttle=10, got %d", ss.Config.ThrottleSeconds)
	}
	if ss.Config.SeverityMin != "error" {
		t.Errorf("expected severity_min=error, got %q", ss.Config.SeverityMin)
	}

	if result["status"] != "enabled" {
		t.Errorf("expected status=enabled, got %v", result["status"])
	}
}

func TestStreamState_Disable(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()

	// Enable first with some pending alerts
	ss.Configure("enable", nil, 0, "", "")
	ss.PendingBatch = append(ss.PendingBatch, Alert{
		Severity: "error", Title: "Test",
	})

	result := ss.Configure("disable", nil, 0, "", "")

	if ss.Config.Enabled {
		t.Error("expected streaming disabled after disable action")
	}
	if len(ss.PendingBatch) != 0 {
		t.Errorf("expected pending batch cleared, got %d", len(ss.PendingBatch))
	}
	cleared, ok := result["pending_cleared"].(int)
	if !ok || cleared != 1 {
		t.Errorf("expected pending_cleared=1, got %v", result["pending_cleared"])
	}
}

func TestStreamState_Status(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", []string{"errors"}, 15, "/api/", "warning")

	result := ss.Configure("status", nil, 0, "", "")

	config, ok := result["config"].(StreamConfig)
	if !ok {
		t.Fatal("expected config in status response")
	}
	if !config.Enabled {
		t.Error("expected config.enabled=true in status")
	}
	if config.ThrottleSeconds != 15 {
		t.Errorf("expected throttle=15, got %d", config.ThrottleSeconds)
	}
}

func TestStreamState_EnableWithDefaults(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()

	// Enable with zero values — should use defaults
	ss.Configure("enable", nil, 0, "", "")

	if !ss.Config.Enabled {
		t.Error("expected streaming enabled")
	}
	if len(ss.Config.Events) != 1 || ss.Config.Events[0] != "all" {
		t.Errorf("expected default events=[all], got %v", ss.Config.Events)
	}
	if ss.Config.ThrottleSeconds != 5 {
		t.Errorf("expected default throttle=5, got %d", ss.Config.ThrottleSeconds)
	}
	if ss.Config.SeverityMin != "warning" {
		t.Errorf("expected default severity_min=warning, got %q", ss.Config.SeverityMin)
	}
}

// ============================================
// Notification filtering
// ============================================

func TestStreamState_SeverityFilter(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", nil, 0, "", "error")

	// Warning should be filtered out
	if ss.shouldEmit(Alert{Severity: "warning", Category: "anomaly"}) {
		t.Error("expected warning to be filtered when severity_min=error")
	}

	// Error should pass
	if !ss.shouldEmit(Alert{Severity: "error", Category: "anomaly"}) {
		t.Error("expected error to pass when severity_min=error")
	}

	// Info should be filtered out
	if ss.shouldEmit(Alert{Severity: "info", Category: "noise"}) {
		t.Error("expected info to be filtered when severity_min=error")
	}
}

func TestStreamState_CategoryFilter(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", []string{"errors", "ci"}, 0, "", "info")

	// "errors" category maps to anomaly alerts
	if !ss.shouldEmit(Alert{Severity: "error", Category: "anomaly"}) {
		t.Error("expected anomaly to pass with errors filter")
	}

	// ci should pass
	if !ss.shouldEmit(Alert{Severity: "info", Category: "ci"}) {
		t.Error("expected ci to pass with ci filter")
	}

	// regression should be filtered
	if ss.shouldEmit(Alert{Severity: "error", Category: "regression"}) {
		t.Error("expected regression to be filtered without regression event type")
	}
}

func TestStreamState_AllEventsFilter(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", []string{"all"}, 0, "", "info")

	// Everything should pass with "all"
	for _, cat := range []string{"regression", "anomaly", "ci", "noise", "threshold"} {
		if !ss.shouldEmit(Alert{Severity: "info", Category: cat}) {
			t.Errorf("expected category %q to pass with 'all' filter", cat)
		}
	}
}

func TestStreamState_DisabledEmitsNothing(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	// Not enabled

	if ss.shouldEmit(Alert{Severity: "error", Category: "anomaly"}) {
		t.Error("expected nothing to pass when streaming is disabled")
	}
}

// ============================================
// Throttling and rate limiting
// ============================================

func TestStreamState_Throttling(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", nil, 5, "", "info")

	now := time.Now()
	alert := Alert{Severity: "error", Category: "anomaly", Title: "Test error"}

	// First emission should succeed
	if !ss.canEmitAt(now) {
		t.Error("expected first emission to succeed")
	}
	ss.recordEmission(now, alert)

	// Immediate second emission should be throttled
	if ss.canEmitAt(now.Add(1 * time.Second)) {
		t.Error("expected emission within throttle window to be blocked")
	}

	// After throttle window, should succeed
	if !ss.canEmitAt(now.Add(6 * time.Second)) {
		t.Error("expected emission after throttle window to succeed")
	}
}

func TestStreamState_RateLimit(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", nil, 1, "", "info") // 1 second throttle

	now := time.Now()
	alert := Alert{Severity: "error", Category: "anomaly", Title: "Test"}
	ss.MinuteStart = now

	// Fill up the rate limit (12 per minute)
	for i := 0; i < maxNotificationsPerMinute; i++ {
		emitTime := now.Add(time.Duration(i*2) * time.Second)
		ss.recordEmission(emitTime, alert)
	}

	// Verify count reached the limit
	if ss.NotifyCount < maxNotificationsPerMinute {
		t.Errorf("expected notify count=%d, got %d", maxNotificationsPerMinute, ss.NotifyCount)
	}
}

func TestStreamState_RateLimitResets(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", nil, 1, "", "info")

	now := time.Now()
	ss.MinuteStart = now
	ss.NotifyCount = maxNotificationsPerMinute

	// After a full minute, counter should reset
	nextMinute := now.Add(61 * time.Second)
	ss.checkRateReset(nextMinute)

	if ss.NotifyCount != 0 {
		t.Errorf("expected notify count reset to 0, got %d", ss.NotifyCount)
	}
}

// ============================================
// Deduplication
// ============================================

func TestStreamState_Dedup(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", nil, 1, "", "info")

	now := time.Now()
	dedupKey := "POST:/api/users:500"

	// First message with this key should not be a duplicate
	if ss.isDuplicate(dedupKey, now) {
		t.Error("expected first occurrence of key to not be a duplicate")
	}
	// Record it
	ss.recordDedupKey(dedupKey, now)

	// Same key within dedup window (30s) should be duplicate
	if !ss.isDuplicate(dedupKey, now.Add(10*time.Second)) {
		t.Error("expected same key within 30s to be detected as duplicate")
	}

	// After dedup window, should not be duplicate
	if ss.isDuplicate(dedupKey, now.Add(31*time.Second)) {
		t.Error("expected same key after 30s to not be duplicate")
	}
}

// ============================================
// MCP Notification Format
// ============================================

func TestStreamState_FormatNotification(t *testing.T) {
	t.Parallel()
	alert := Alert{
		Severity:  "error",
		Category:  "ci",
		Title:     "CI failure (github-actions)",
		Detail:    "2 tests failed",
		Timestamp: "2025-01-25T14:30:00.000Z",
		Source:    "ci_webhook",
	}

	notification := formatMCPNotification(alert)

	if notification.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %q", notification.JSONRPC)
	}
	if notification.Method != "notifications/message" {
		t.Errorf("expected method=notifications/message, got %q", notification.Method)
	}
	if notification.Params.Level != "error" {
		t.Errorf("expected level=error, got %q", notification.Params.Level)
	}
	if notification.Params.Logger != "gasoline" {
		t.Errorf("expected logger=gasoline, got %q", notification.Params.Logger)
	}

	// Verify serialization
	data, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("failed to marshal notification: %v", err)
	}
	if !bytes.Contains(data, []byte(`"notifications/message"`)) {
		t.Error("expected notifications/message in JSON output")
	}
	if !bytes.Contains(data, []byte(`"CI failure (github-actions)"`)) {
		t.Error("expected alert title in JSON output")
	}
}

// ============================================
// EmitNotification integration
// ============================================

func TestStreamState_EmitToWriter(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", nil, 1, "", "info")

	var buf bytes.Buffer
	ss.writer = &buf

	alert := Alert{
		Severity:  "warning",
		Category:  "anomaly",
		Title:     "Error frequency spike",
		Detail:    "5 errors in 10s",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "anomaly_detector",
	}

	ss.EmitAlert(alert)

	output := buf.String()
	if output == "" {
		t.Fatal("expected notification output, got empty")
	}

	// Should be valid JSON
	var notification MCPNotification
	if err := json.Unmarshal([]byte(output), &notification); err != nil {
		t.Fatalf("expected valid JSON notification: %v", err)
	}

	if notification.Method != "notifications/message" {
		t.Errorf("expected method=notifications/message, got %q", notification.Method)
	}
}

func TestStreamState_EmitRespectsEnabled(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	// NOT enabled

	var buf bytes.Buffer
	ss.writer = &buf

	alert := Alert{
		Severity:  "error",
		Category:  "anomaly",
		Title:     "Test",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "test",
	}

	ss.EmitAlert(alert)

	if buf.Len() != 0 {
		t.Error("expected no output when streaming is disabled")
	}
}

// TestStreamState_PendingBatchOverflow verifies that when PendingBatch is at
// maxPendingBatch, additional throttled alerts are silently dropped. (CH4)
func TestStreamState_PendingBatchOverflow(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", nil, 1, "", "info")

	var buf bytes.Buffer
	ss.writer = &buf

	// First: emit one alert to set LastNotified
	firstAlert := Alert{
		Severity:  "warning",
		Category:  "anomaly",
		Title:     "first",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "test",
	}
	ss.EmitAlert(firstAlert)
	buf.Reset()

	// Now emit maxPendingBatch+10 alerts rapidly (will be throttled)
	for i := 0; i < maxPendingBatch+10; i++ {
		ss.EmitAlert(Alert{
			Severity:  "warning",
			Category:  "anomaly",
			Title:     "throttled-" + time.Now().Format(time.RFC3339Nano),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Source:    "test",
		})
	}

	ss.mu.Lock()
	batchLen := len(ss.PendingBatch)
	ss.mu.Unlock()

	if batchLen > maxPendingBatch {
		t.Errorf("PendingBatch should be capped at %d, got %d", maxPendingBatch, batchLen)
	}
}

// TestStreamState_CategoryMatchesAllEvents verifies all 8 event-to-category
// mappings in categoryMatchesEvent. (CH6)
func TestStreamState_CategoryMatchesAllEvents(t *testing.T) {
	t.Parallel()
	tests := []struct {
		event    string
		category string
		want     bool
	}{
		// "errors" matches anomaly and threshold
		{"errors", "anomaly", true},
		{"errors", "threshold", true},
		{"errors", "ci", false},
		// "network_errors" matches anomaly
		{"network_errors", "anomaly", true},
		{"network_errors", "ci", false},
		// "performance" matches regression
		{"performance", "regression", true},
		{"performance", "anomaly", false},
		// "regression" matches regression
		{"regression", "regression", true},
		{"regression", "ci", false},
		// "anomaly" matches anomaly
		{"anomaly", "anomaly", true},
		{"anomaly", "regression", false},
		// "ci" matches ci
		{"ci", "ci", true},
		{"ci", "anomaly", false},
		// "security" matches threshold
		{"security", "threshold", true},
		{"security", "ci", false},
		// "user_frustration" matches anomaly
		{"user_frustration", "anomaly", true},
		{"user_frustration", "regression", false},
		// unknown event matches nothing
		{"unknown_event", "anomaly", false},
		{"unknown_event", "ci", false},
	}

	for _, tt := range tests {
		got := categoryMatchesEvent(tt.category, tt.event)
		if got != tt.want {
			t.Errorf("categoryMatchesEvent(%q, %q) = %v, want %v", tt.category, tt.event, got, tt.want)
		}
	}
}

// TestStreamState_MultipleEventFilter verifies OR-matching with multiple events. (G2)
func TestStreamState_MultipleEventFilter(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", []string{"errors", "ci"}, 1, "", "info")

	var buf bytes.Buffer
	ss.writer = &buf

	// anomaly matches "errors" event → should emit
	ss.EmitAlert(Alert{
		Severity:  "warning",
		Category:  "anomaly",
		Title:     "anomaly-test",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "test",
	})
	if buf.Len() == 0 {
		t.Error("anomaly alert should emit when 'errors' event is subscribed")
	}
	buf.Reset()

	// ci matches "ci" event → should emit (after throttle window)
	time.Sleep(2 * time.Millisecond)
	ss.mu.Lock()
	ss.LastNotified = time.Time{} // reset throttle
	ss.mu.Unlock()
	ss.EmitAlert(Alert{
		Severity:  "warning",
		Category:  "ci",
		Title:     "ci-test",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "test",
	})
	if buf.Len() == 0 {
		t.Error("ci alert should emit when 'ci' event is subscribed")
	}
	buf.Reset()

	// regression does NOT match "errors" or "ci" → should not emit
	ss.mu.Lock()
	ss.LastNotified = time.Time{} // reset throttle
	ss.mu.Unlock()
	ss.EmitAlert(Alert{
		Severity:  "warning",
		Category:  "regression",
		Title:     "regression-test",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "test",
	})
	if buf.Len() != 0 {
		t.Error("regression alert should NOT emit when only 'errors' and 'ci' events are subscribed")
	}
}

// TestStreamState_RecordEmission verifies recordEmission updates LastNotified
// and increments NotifyCount. (G3)
func TestStreamState_RecordEmission(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()

	now := time.Now()
	alert := Alert{
		Severity:  "error",
		Category:  "ci",
		Title:     "test",
		Timestamp: now.Format(time.RFC3339),
		Source:    "test",
	}

	ss.recordEmission(now, alert)

	ss.mu.Lock()
	if ss.LastNotified.IsZero() {
		t.Error("expected LastNotified to be set")
	}
	if ss.NotifyCount != 1 {
		t.Errorf("expected NotifyCount=1, got %d", ss.NotifyCount)
	}
	ss.mu.Unlock()

	// Second emission
	ss.recordEmission(now.Add(time.Second), alert)

	ss.mu.Lock()
	if ss.NotifyCount != 2 {
		t.Errorf("expected NotifyCount=2, got %d", ss.NotifyCount)
	}
	ss.mu.Unlock()
}

// TestStreamState_CheckRateReset verifies the public checkRateReset method
// resets the counter when a new minute starts. (G1)
func TestStreamState_CheckRateReset(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()

	// Set up state: 5 notifications in the current minute
	now := time.Now()
	ss.mu.Lock()
	ss.MinuteStart = now
	ss.NotifyCount = 5
	ss.mu.Unlock()

	// Within the same minute: count should NOT reset
	ss.checkRateReset(now.Add(30 * time.Second))
	ss.mu.Lock()
	if ss.NotifyCount != 5 {
		t.Errorf("expected count=5 within same minute, got %d", ss.NotifyCount)
	}
	ss.mu.Unlock()

	// After 1 minute: count should reset
	ss.checkRateReset(now.Add(61 * time.Second))
	ss.mu.Lock()
	if ss.NotifyCount != 0 {
		t.Errorf("expected count=0 after minute reset, got %d", ss.NotifyCount)
	}
	ss.mu.Unlock()
}

func TestStreamState_EmitRespectsSeverityFilter(t *testing.T) {
	t.Parallel()
	ss := NewStreamState()
	ss.Configure("enable", nil, 1, "", "error")

	var buf bytes.Buffer
	ss.writer = &buf

	// Warning should not emit
	ss.EmitAlert(Alert{
		Severity:  "warning",
		Category:  "anomaly",
		Title:     "Should not emit",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "test",
	})

	if buf.Len() != 0 {
		t.Error("expected no output for warning when severity_min=error")
	}
}
