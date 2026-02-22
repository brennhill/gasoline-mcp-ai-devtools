// health_unit_test.go — Unit tests for HealthMetrics counters.
package main

import (
	"sync"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

func TestHealthMetrics_IncrementAndGet(t *testing.T) {
	t.Parallel()

	hm := NewHealthMetrics()

	hm.IncrementRequest("observe")
	hm.IncrementRequest("observe")
	hm.IncrementRequest("configure")
	hm.IncrementError("observe")

	if got := hm.GetRequestCount("observe"); got != 2 {
		t.Fatalf("GetRequestCount(observe) = %d, want 2", got)
	}
	if got := hm.GetRequestCount("configure"); got != 1 {
		t.Fatalf("GetRequestCount(configure) = %d, want 1", got)
	}
	if got := hm.GetRequestCount("unknown"); got != 0 {
		t.Fatalf("GetRequestCount(unknown) = %d, want 0", got)
	}
	if got := hm.GetErrorCount("observe"); got != 1 {
		t.Fatalf("GetErrorCount(observe) = %d, want 1", got)
	}
	if got := hm.GetErrorCount("configure"); got != 0 {
		t.Fatalf("GetErrorCount(configure) = %d, want 0", got)
	}
}

func TestHealthMetrics_Totals(t *testing.T) {
	t.Parallel()

	hm := NewHealthMetrics()

	hm.IncrementRequest("observe")
	hm.IncrementRequest("configure")
	hm.IncrementRequest("interact")
	hm.IncrementError("observe")
	hm.IncrementError("interact")

	if got := hm.GetTotalRequests(); got != 3 {
		t.Fatalf("GetTotalRequests() = %d, want 3", got)
	}
	if got := hm.GetTotalErrors(); got != 2 {
		t.Fatalf("GetTotalErrors() = %d, want 2", got)
	}
}

func TestHealthMetrics_EmptyTotals(t *testing.T) {
	t.Parallel()

	hm := NewHealthMetrics()

	if got := hm.GetTotalRequests(); got != 0 {
		t.Fatalf("GetTotalRequests() on empty = %d, want 0", got)
	}
	if got := hm.GetTotalErrors(); got != 0 {
		t.Fatalf("GetTotalErrors() on empty = %d, want 0", got)
	}
}

func TestHealthMetrics_Uptime(t *testing.T) {
	t.Parallel()

	hm := NewHealthMetrics()
	uptime := hm.GetUptime()
	if uptime < 0 {
		t.Fatalf("GetUptime() = %v, expected positive", uptime)
	}
}

func TestHealthResponseIncludesDroppedCount(t *testing.T) {
	t.Parallel()

	hm := NewHealthMetrics()

	// Create a server with a channel of size 1 and NO async worker,
	// so the channel stays full when we manually fill it.
	srv := &Server{
		maxEntries: 100,
		entries:    make([]LogEntry, 0),
		logChan:    make(chan []LogEntry, 1),
		logDone:    make(chan struct{}),
	}

	// Fill channel (no worker draining it), then trigger two drops
	srv.logChan <- []LogEntry{{"level": "info", "message": "fill"}}
	_ = srv.appendToFile([]LogEntry{{"level": "info", "message": "drop1"}})
	_ = srv.appendToFile([]LogEntry{{"level": "info", "message": "drop2"}})

	resp := hm.GetHealth(nil, srv, "test")

	if resp.Buffers.Console.DroppedCount != 2 {
		t.Fatalf("Console.DroppedCount = %d, want 2", resp.Buffers.Console.DroppedCount)
	}

	// Other buffers should have zero dropped count
	if resp.Buffers.Network.DroppedCount != 0 {
		t.Fatalf("Network.DroppedCount = %d, want 0", resp.Buffers.Network.DroppedCount)
	}
	if resp.Buffers.WebSocket.DroppedCount != 0 {
		t.Fatalf("WebSocket.DroppedCount = %d, want 0", resp.Buffers.WebSocket.DroppedCount)
	}
	if resp.Buffers.Actions.DroppedCount != 0 {
		t.Fatalf("Actions.DroppedCount = %d, want 0", resp.Buffers.Actions.DroppedCount)
	}

	// Clean up: drain channel and close done signal
	<-srv.logChan
	close(srv.logDone)
}

func TestHealthResponseZeroDroppedCount(t *testing.T) {
	t.Parallel()

	hm := NewHealthMetrics()
	srv := &Server{
		maxEntries: 100,
		entries:    make([]LogEntry, 0),
		logChan:    make(chan []LogEntry, 10),
		logDone:    make(chan struct{}),
	}

	resp := hm.GetHealth(nil, srv, "test")

	if resp.Buffers.Console.DroppedCount != 0 {
		t.Fatalf("Console.DroppedCount = %d, want 0 for fresh server", resp.Buffers.Console.DroppedCount)
	}

	close(srv.logDone)
}

func TestBuildPilotInfo_AssumedEnabledStartupState(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	info := buildPilotInfo(cap)

	if !info.Enabled {
		t.Fatalf("enabled = false, want true during startup uncertainty")
	}
	if info.Source != "assumed_startup" {
		t.Fatalf("source = %q, want assumed_startup", info.Source)
	}
}

func TestBuildPilotInfo_ExplicitDisableState(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	cap.SetPilotEnabled(false)

	info := buildPilotInfo(cap)
	if info.Enabled {
		t.Fatalf("enabled = true, want false for explicit disable")
	}
	if info.Source != "explicitly_disabled" {
		t.Fatalf("source = %q, want explicitly_disabled", info.Source)
	}
}

func TestCalcUtilization_Normal(t *testing.T) {
	t.Parallel()

	if got := calcUtilization(50, 100); got != 50.0 {
		t.Fatalf("calcUtilization(50, 100) = %v, want 50.0", got)
	}
	if got := calcUtilization(0, 100); got != 0.0 {
		t.Fatalf("calcUtilization(0, 100) = %v, want 0.0", got)
	}
	if got := calcUtilization(100, 100); got != 100.0 {
		t.Fatalf("calcUtilization(100, 100) = %v, want 100.0", got)
	}
}

func TestCalcUtilization_ZeroCapacity(t *testing.T) {
	t.Parallel()

	if got := calcUtilization(50, 0); got != 0.0 {
		t.Fatalf("calcUtilization(50, 0) = %v, want 0.0", got)
	}
	if got := calcUtilization(0, 0); got != 0.0 {
		t.Fatalf("calcUtilization(0, 0) = %v, want 0.0", got)
	}
}

func TestCalcUtilization_NegativeCapacity(t *testing.T) {
	t.Parallel()

	if got := calcUtilization(50, -1); got != 0.0 {
		t.Fatalf("calcUtilization(50, -1) = %v, want 0.0", got)
	}
}

func TestHealthMetrics_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	hm := NewHealthMetrics()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			hm.IncrementRequest("observe")
		}()
		go func() {
			defer wg.Done()
			hm.IncrementError("observe")
		}()
	}
	wg.Wait()

	if got := hm.GetTotalRequests(); got != 100 {
		t.Fatalf("GetTotalRequests() after 100 concurrent increments = %d, want 100", got)
	}
	if got := hm.GetTotalErrors(); got != 100 {
		t.Fatalf("GetTotalErrors() after 100 concurrent increments = %d, want 100", got)
	}
}
