// streaming_emit_test.go — Unit tests for EmitAlert and checkRateReset.
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

func TestEmitAlert_Disabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.writer = &buf
	s.Config.Enabled = false

	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "test"})

	if buf.Len() != 0 {
		t.Fatal("EmitAlert should not write when disabled")
	}
}

func TestEmitAlert_SeverityFilter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "error"
	s.Config.Events = []string{"all"}

	// Info alert should be filtered out (below error threshold)
	s.EmitAlert(types.Alert{Severity: "info", Category: "regression", Title: "low severity"})
	if buf.Len() != 0 {
		t.Fatal("info alert should be filtered when severity_min=error")
	}

	// Error alert should pass
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "high severity"})
	if buf.Len() == 0 {
		t.Fatal("error alert should pass when severity_min=error")
	}
}

func TestEmitAlert_CategoryFilter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"ci"} // Only CI events

	// "regression" category should not match "ci" event filter
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "regression alert"})
	if buf.Len() != 0 {
		t.Fatal("regression should not match 'ci' event filter")
	}
}

func TestEmitAlert_Dedup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}

	alert := types.Alert{Severity: "error", Category: "regression", Title: "dup alert"}

	s.EmitAlert(alert)
	first := buf.Len()
	if first == 0 {
		t.Fatal("first EmitAlert should write")
	}

	// Same alert again should be deduped
	s.EmitAlert(alert)
	if buf.Len() != first {
		t.Fatal("duplicate alert within dedup window should be suppressed")
	}
}

func TestEmitAlert_WritesValidJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}

	s.EmitAlert(types.Alert{Severity: "warning", Category: "regression", Title: "test alert"})

	var notif MCPNotification
	if err := json.Unmarshal(buf.Bytes(), &notif); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, buf.String())
	}
	if notif.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", notif.JSONRPC)
	}
	if notif.Method != "notifications/message" {
		t.Errorf("method = %q, want notifications/message", notif.Method)
	}
}

func TestEmitAlert_Throttle(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}
	s.Config.ThrottleSeconds = 60 // very long throttle

	// First alert goes through
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "first"})
	first := buf.Len()
	if first == 0 {
		t.Fatal("first alert should be emitted")
	}

	// Second alert (different title to avoid dedup) should be throttled
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "second"})
	if buf.Len() != first {
		t.Fatal("second alert should be throttled")
	}

	// Should be in pending batch
	s.mu.Lock()
	pending := len(s.PendingBatch)
	s.mu.Unlock()
	if pending != 1 {
		t.Fatalf("expected 1 pending alert, got %d", pending)
	}
}

func TestEmitAlert_NilWriter(t *testing.T) {
	t.Parallel()

	s := NewStreamState()
	s.writer = nil
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}

	// Should not panic with nil writer
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "test"})
}

func TestCheckRateReset(t *testing.T) {
	t.Parallel()

	s := NewStreamState()
	s.MinuteStart = time.Now().Add(-2 * time.Minute)
	s.NotifyCount = 50

	s.checkRateReset(time.Now())

	s.mu.Lock()
	count := s.NotifyCount
	s.mu.Unlock()

	if count != 0 {
		t.Fatalf("checkRateReset should reset count after minute boundary, got %d", count)
	}
}

func TestEmitAlert_RateLimit(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}
	s.Config.ThrottleSeconds = 0 // no throttle

	// Exhaust the rate limit
	for i := 0; i < maxNotificationsPerMinute; i++ {
		// Use unique titles to avoid dedup
		s.EmitAlert(types.Alert{
			Severity: "error",
			Category: "regression",
			Title:    strings.Repeat("x", i+1),
		})
	}

	prevLen := buf.Len()

	// Next one should be rate-limited
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "rate limited"})
	if buf.Len() != prevLen {
		t.Fatal("alert beyond rate limit should be suppressed")
	}
}
