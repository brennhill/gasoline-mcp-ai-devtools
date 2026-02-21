// noise_autorun_test.go — Tests for automatic noise detection after navigation.
package main

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// noiseAutoRunner Tests
// ============================================

func TestNoiseAutoRunner_ScheduleRunsOnce(t *testing.T) {
	t.Parallel()

	var runCount atomic.Int32
	runner := newNoiseAutoRunner(func() {
		runCount.Add(1)
	}, 50*time.Millisecond)

	runner.schedule()

	// Wait for debounce + execution
	time.Sleep(150 * time.Millisecond)

	if got := runCount.Load(); got != 1 {
		t.Errorf("run count = %d, want 1", got)
	}
}

func TestNoiseAutoRunner_DebouncesRapidSchedules(t *testing.T) {
	t.Parallel()

	var runCount atomic.Int32
	runner := newNoiseAutoRunner(func() {
		runCount.Add(1)
	}, 100*time.Millisecond)

	// Schedule 5 times rapidly — should only run once within debounce window
	for i := 0; i < 5; i++ {
		runner.schedule()
	}

	// Wait for debounce + execution
	time.Sleep(250 * time.Millisecond)

	if got := runCount.Load(); got != 1 {
		t.Errorf("run count after rapid schedules = %d, want 1", got)
	}
}

func TestNoiseAutoRunner_RunsAgainAfterDebounceExpires(t *testing.T) {
	t.Parallel()

	var runCount atomic.Int32
	runner := newNoiseAutoRunner(func() {
		runCount.Add(1)
	}, 50*time.Millisecond)

	runner.schedule()
	time.Sleep(100 * time.Millisecond) // Wait for first run

	runner.schedule()
	time.Sleep(100 * time.Millisecond) // Wait for second run

	if got := runCount.Load(); got != 2 {
		t.Errorf("run count = %d, want 2 (one per debounce window)", got)
	}
}

func TestNoiseAutoRunner_NilFuncDoesNotPanic(t *testing.T) {
	t.Parallel()

	// Should not panic with nil function
	runner := newNoiseAutoRunner(nil, 50*time.Millisecond)
	runner.schedule() // Should be a no-op
	time.Sleep(100 * time.Millisecond)
}

func TestNoiseAutoDetectEnabled_DefaultOff(t *testing.T) {
	t.Setenv(noiseAutoDetectEnvVar, "")

	if noiseAutoDetectEnabled() {
		t.Fatal("noise auto-detect should default to off")
	}
}

func TestNoiseAutoDetectEnabled_TruthyValues(t *testing.T) {
	for _, val := range []string{"1", "true", "TRUE", "on", "yes"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv(noiseAutoDetectEnvVar, val)
			if !noiseAutoDetectEnabled() {
				t.Fatalf("expected %q to enable noise auto-detect", val)
			}
		})
	}
}

func TestNoiseAutoDetect_RunsOnceOnFirstInitialize(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/noise-init.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.toolHandler.(*ToolHandler)

	initReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-06-18"}`),
	}

	resp1 := mcpHandler.HandleRequest(initReq)
	if resp1 == nil || resp1.Error != nil {
		t.Fatalf("first initialize should succeed, got response: %+v", resp1)
	}

	initReq.ID = 2
	resp2 := mcpHandler.HandleRequest(initReq)
	if resp2 == nil || resp2.Error != nil {
		t.Fatalf("second initialize should succeed, got response: %+v", resp2)
	}

	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadUint32(&h.noiseAutoInitRuns); got != 1 {
		t.Fatalf("noise auto-detect init runs = %d, want 1", got)
	}
}
