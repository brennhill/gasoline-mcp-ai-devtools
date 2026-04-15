// usage_beacon_test.go — Tests for periodic aggregated usage beacon.

package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestUsageBeaconLoop_FiresOnActivity(t *testing.T) {
	received := make(chan map[string]any, 10)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	// Reset install ID state so it generates fresh for this test.
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	counter := NewUsageTracker()
	counter.RecordToolCall("observe:errors", 0, false)
	counter.RecordToolCall("observe:errors", 0, false)
	counter.RecordToolCall("interact:click", 0, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUsageBeaconLoopWithInterval(ctx, counter, 50*time.Millisecond)

	// Drain received until we find the usage_summary event.
	// Other events (e.g. first_tool_call) may arrive first.
	var body map[string]any
	for {
		select {
		case b := <-received:
			if b["event"] == "usage_summary" {
				body = b
				goto found
			}
		case <-time.After(2 * time.Second):
			t.Fatal("usage beacon not received within timeout")
		}
	}
found:

	if body["event"] != "usage_summary" {
		t.Errorf("event = %v, want usage_summary", body["event"])
	}

	// window_m is top-level (JSON number → float64).
	if wm, ok := body["window_m"].(float64); !ok || wm != 0 {
		t.Errorf("window_m = %v, want 0 (sub-minute test interval)", body["window_m"])
	}

	// sid must be present (16-char hex).
	if sid, ok := body["sid"].(string); !ok || len(sid) != 16 {
		t.Errorf("sid = %v, want 16-char hex string", body["sid"])
	}

	// Verify tool_stats is an array with the expected entries.
	toolStats, ok := body["tool_stats"].([]any)
	if !ok {
		t.Fatalf("tool_stats is not an array: %T", body["tool_stats"])
	}
	if len(toolStats) == 0 {
		t.Fatal("tool_stats is empty, expected at least 1 entry")
	}
	// Verify at least one stat has observe:errors
	foundObserve := false
	for _, s := range toolStats {
		stat, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if stat["tool"] == "observe:errors" {
			foundObserve = true
			if stat["count"] != float64(2) {
				t.Errorf("observe:errors count = %v, want 2", stat["count"])
			}
		}
	}
	if !foundObserve {
		t.Errorf("tool_stats missing observe:errors entry, got %v", toolStats)
	}
}

func TestUsageBeaconLoop_SkipsWhenIdle(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	// Use onTick hook to wait for a known number of tick cycles.
	tickCh := make(chan struct{}, 10)
	setOnTick(func() {
		select {
		case tickCh <- struct{}{}:
		default:
		}
	})
	defer setOnTick(nil)

	counter := NewUsageTracker()
	// Don't increment — should skip.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUsageBeaconLoopWithInterval(ctx, counter, 10*time.Millisecond)

	// Wait for 3 tick cycles to complete.
	for i := 0; i < 3; i++ {
		select {
		case <-tickCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for tick")
		}
	}

	cancel()

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Fatalf("beacon fired %d times, want 0 (no activity)", count)
	}
}

func TestUsageBeaconLoop_RespectsOptOut(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")

	var mu sync.Mutex
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	// Use onTick hook to wait for a known number of tick cycles.
	tickCh := make(chan struct{}, 10)
	setOnTick(func() {
		select {
		case tickCh <- struct{}{}:
		default:
		}
	})
	defer setOnTick(nil)

	counter := NewUsageTracker()
	counter.RecordToolCall("observe:errors", 0, false)
	counter.RecordToolCall("interact:click", 0, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUsageBeaconLoopWithInterval(ctx, counter, 10*time.Millisecond)

	// Wait for 3 tick cycles to complete.
	for i := 0; i < 3; i++ {
		select {
		case <-tickCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for tick")
		}
	}

	cancel()

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Fatalf("beacon fired %d times with KABOOM_TELEMETRY=off, want 0", count)
	}
}

func TestUsageBeaconLoop_StopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	counter := NewUsageTracker()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		startUsageBeaconLoopWithInterval(ctx, counter, 50*time.Millisecond)
		close(done)
	}()

	// Cancel context and verify goroutine exits.
	cancel()

	select {
	case <-done:
		// Good — goroutine exited.
	case <-time.After(2 * time.Second):
		t.Fatal("beacon loop did not stop after context cancel")
	}
}
