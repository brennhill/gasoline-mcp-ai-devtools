// server_routes_debug_usage_test.go — Tests for debug telemetry endpoints.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// stubMCPHandler returns an MCPHandler with a ToolHandler that has the given UsageCounter.
// Other ToolHandler fields are left at zero values — only usageCounter is needed for debug endpoints.
func stubMCPHandler(counter *telemetry.UsageCounter) *MCPHandler {
	h := &MCPHandler{}
	if counter != nil {
		th := &ToolHandler{usageCounter: counter}
		h.toolHandler = th
	}
	return h
}

// M5: debugEndpointsEnabled respects KABOOM_DEBUG env var.
func TestDebugEndpointsEnabled_Unset(t *testing.T) {
	t.Setenv("KABOOM_DEBUG", "")
	if debugEndpointsEnabled() {
		t.Fatal("debugEndpointsEnabled() = true, want false when KABOOM_DEBUG is empty")
	}
}

func TestDebugEndpointsEnabled_Set(t *testing.T) {
	t.Setenv("KABOOM_DEBUG", "1")
	if !debugEndpointsEnabled() {
		t.Fatal("debugEndpointsEnabled() = false, want true when KABOOM_DEBUG=1")
	}
}

func TestDebugEndpointsEnabled_WrongValue(t *testing.T) {
	t.Setenv("KABOOM_DEBUG", "true")
	if debugEndpointsEnabled() {
		t.Fatal("debugEndpointsEnabled() = true, want false when KABOOM_DEBUG=true (must be exactly '1')")
	}
}

func TestDebugUsage_GET_EmptyCounter(t *testing.T) {
	t.Parallel()
	mcp := stubMCPHandler(telemetry.NewUsageCounter())
	handler := handleDebugUsage(mcp)

	req := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	counts, ok := body["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts type = %T, want map", body["counts"])
	}
	if len(counts) != 0 {
		t.Errorf("counts = %v, want empty", counts)
	}
}

func TestDebugUsage_GET_PopulatedCounter(t *testing.T) {
	t.Parallel()
	counter := telemetry.NewUsageCounter()
	counter.Increment("observe:page")
	counter.Increment("observe:page")
	counter.Increment("interact:click")

	mcp := stubMCPHandler(counter)
	handler := handleDebugUsage(mcp)

	req := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	counts := body["counts"].(map[string]any)
	if counts["observe:page"] != float64(2) {
		t.Errorf("observe:page = %v, want 2", counts["observe:page"])
	}
	if counts["interact:click"] != float64(1) {
		t.Errorf("interact:click = %v, want 1", counts["interact:click"])
	}
}

func TestDebugUsage_GET_DoesNotReset(t *testing.T) {
	t.Parallel()
	counter := telemetry.NewUsageCounter()
	counter.Increment("observe:page")

	mcp := stubMCPHandler(counter)
	handler := handleDebugUsage(mcp)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	// Second request — counts should still be there (Peek, not SwapAndReset)
	req2 := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	var body map[string]any
	_ = json.Unmarshal(rr2.Body.Bytes(), &body)
	counts := body["counts"].(map[string]any)
	if counts["observe:page"] != float64(1) {
		t.Errorf("observe:page = %v after second GET, want 1 (should not reset)", counts["observe:page"])
	}
}

func TestDebugUsage_POST_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	mcp := stubMCPHandler(telemetry.NewUsageCounter())
	handler := handleDebugUsage(mcp)

	req := httptest.NewRequest(http.MethodPost, "/debug/usage", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestDebugUsage_NilCounter(t *testing.T) {
	t.Parallel()
	mcp := stubMCPHandler(nil)
	handler := handleDebugUsage(mcp)

	req := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	counts := body["counts"].(map[string]any)
	if len(counts) != 0 {
		t.Errorf("counts = %v, want empty for nil counter", counts)
	}
}

func TestDebugBeaconFlush_POST_WithData(t *testing.T) {
	t.Parallel()
	counter := telemetry.NewUsageCounter()
	counter.Increment("observe:errors")
	counter.Increment("configure:health")

	mcp := stubMCPHandler(counter)
	handler := handleDebugBeaconFlush(mcp)

	req := httptest.NewRequest(http.MethodPost, "/debug/beacon-flush", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// flushed >= 2 (may include session_depth key).
	if flushed, ok := body["flushed"].(float64); !ok || flushed < 2 {
		t.Errorf("flushed = %v, want >= 2", body["flushed"])
	}

	payload, ok := body["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map", body["payload"])
	}
	if payload["event"] != "usage_summary" {
		t.Errorf("event = %v, want usage_summary", payload["event"])
	}
	if _, ok := payload["iid"].(string); !ok {
		t.Error("missing iid in payload")
	}
	if _, ok := payload["sid"].(string); !ok {
		t.Error("missing sid in payload")
	}

	props, ok := payload["props"].(map[string]any)
	if !ok {
		t.Fatalf("props type = %T, want map", payload["props"])
	}
	if props["observe:errors"] != float64(1) {
		t.Errorf("observe:errors = %v, want 1", props["observe:errors"])
	}

	// Counter should be empty after flush.
	peek := counter.Peek()
	if len(peek) != 0 {
		t.Errorf("counter has %d keys after flush, want 0", len(peek))
	}
}

func TestDebugBeaconFlush_POST_Empty(t *testing.T) {
	t.Parallel()
	mcp := stubMCPHandler(telemetry.NewUsageCounter())
	handler := handleDebugBeaconFlush(mcp)

	req := httptest.NewRequest(http.MethodPost, "/debug/beacon-flush", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["flushed"] != float64(0) {
		t.Errorf("flushed = %v, want 0", body["flushed"])
	}
	if body["payload"] != nil {
		t.Errorf("payload = %v, want nil for empty counter", body["payload"])
	}
}

func TestDebugBeaconFlush_POST_NilCounter(t *testing.T) {
	t.Parallel()
	mcp := stubMCPHandler(nil)
	handler := handleDebugBeaconFlush(mcp)

	req := httptest.NewRequest(http.MethodPost, "/debug/beacon-flush", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["flushed"] != float64(0) {
		t.Errorf("flushed = %v, want 0", body["flushed"])
	}
}

func TestDebugBeaconFlush_GET_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	mcp := stubMCPHandler(telemetry.NewUsageCounter())
	handler := handleDebugBeaconFlush(mcp)

	req := httptest.NewRequest(http.MethodGet, "/debug/beacon-flush", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}
