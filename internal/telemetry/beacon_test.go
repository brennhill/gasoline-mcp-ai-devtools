// beacon_test.go — Tests for anonymous telemetry beacons.

package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestBeaconError_DisabledByEnv(t *testing.T) {
	t.Setenv("STRUM_TELEMETRY", "off")

	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	BeaconError("test_event", map[string]string{"key": "val"})

	// Give goroutine time to run (if it were going to).
	time.Sleep(50 * time.Millisecond)

	if called {
		t.Fatal("BeaconError should not make HTTP calls when STRUM_TELEMETRY=off")
	}
}

func TestBeaconError_FireAndForget(t *testing.T) {
	// Verify BeaconError returns immediately even when server is unreachable.
	// Use a non-routable address so the goroutine doesn't linger on other test servers.
	overrideEndpoint("http://198.51.100.1:1") // TEST-NET-2, non-routable
	defer resetEndpoint()

	start := time.Now()
	BeaconError("slow_test", nil)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Fatalf("BeaconError blocked for %v, expected fire-and-forget", elapsed)
	}
}

func TestBeaconError_FormatsJSON(t *testing.T) {
	var mu sync.Mutex
	var received map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode JSON body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	BeaconError("test_error", map[string]string{"error_code": "conn_refused", "port": "7890"})

	// Wait for async delivery.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("expected HTTP request, got none")
	}

	if received["event"] != "test_error" {
		t.Errorf("event = %v, want test_error", received["event"])
	}

	props, ok := received["props"].(map[string]any)
	if !ok {
		t.Fatalf("props is not a map: %T", received["props"])
	}
	if props["error_code"] != "conn_refused" {
		t.Errorf("props.error_code = %v, want conn_refused", props["error_code"])
	}
	if props["port"] != "7890" {
		t.Errorf("props.port = %v, want 7890", props["port"])
	}
}

func TestBeaconEvent_IncludesVersion(t *testing.T) {
	var mu sync.Mutex
	var received map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	BeaconEvent("daemon_start", map[string]string{"mode": "bridge"})

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("expected HTTP request, got none")
	}

	if _, ok := received["v"]; !ok {
		t.Error("missing 'v' (version) field in beacon payload")
	}
	if _, ok := received["os"]; !ok {
		t.Error("missing 'os' field in beacon payload")
	}
	if received["event"] != "daemon_start" {
		t.Errorf("event = %v, want daemon_start", received["event"])
	}
}

func TestBeaconError_IgnoresHTTPFailure(t *testing.T) {
	// Point at a closed server — should not panic.
	overrideEndpoint("http://127.0.0.1:1") // nothing listening
	defer resetEndpoint()

	// Should not panic or block.
	BeaconError("unreachable", map[string]string{"key": "val"})
	time.Sleep(100 * time.Millisecond)
	// If we got here without panic, the test passes.
}

func TestBeaconError_NilProps(t *testing.T) {
	var mu sync.Mutex
	var received map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	BeaconError("nil_props_test", nil)
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("expected HTTP request, got none")
	}
	if received["event"] != "nil_props_test" {
		t.Errorf("event = %v, want nil_props_test", received["event"])
	}
}
