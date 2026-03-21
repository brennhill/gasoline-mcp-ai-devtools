// beacon_test.go — Tests for anonymous telemetry beacons.
// Tests in this package must NOT use t.Parallel() due to shared package-level state.

package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	received := make(chan map[string]any, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode JSON body: %v", err)
		}
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	BeaconError("test_error", map[string]string{"error_code": "conn_refused", "port": "7890"})

	var body map[string]any
	select {
	case body = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("beacon not received within timeout")
	}

	if body["event"] != "test_error" {
		t.Errorf("event = %v, want test_error", body["event"])
	}

	props, ok := body["props"].(map[string]any)
	if !ok {
		t.Fatalf("props is not a map: %T", body["props"])
	}
	if props["error_code"] != "conn_refused" {
		t.Errorf("props.error_code = %v, want conn_refused", props["error_code"])
	}
	if props["port"] != "7890" {
		t.Errorf("props.port = %v, want 7890", props["port"])
	}

	// Verify install ID is present and is 12-char hex.
	iid, ok := body["iid"].(string)
	if !ok {
		t.Fatalf("missing or non-string 'iid' field in beacon payload")
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(iid) {
		t.Errorf("iid = %q, want 12-char hex string", iid)
	}
}

func TestBeaconEvent_IncludesVersion(t *testing.T) {
	received := make(chan map[string]any, 1)

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

	BeaconEvent("daemon_start", map[string]string{"mode": "bridge"})

	var body map[string]any
	select {
	case body = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("beacon not received within timeout")
	}

	if _, ok := body["v"]; !ok {
		t.Error("missing 'v' (version) field in beacon payload")
	}
	if _, ok := body["os"]; !ok {
		t.Error("missing 'os' field in beacon payload")
	}
	if body["event"] != "daemon_start" {
		t.Errorf("event = %v, want daemon_start", body["event"])
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

func TestBeacon_SemaphoreBackpressure(t *testing.T) {
	// Point at a real server so beacons would succeed if they ran.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	// Fill all semaphore slots with blocking goroutines.
	blockers := make(chan struct{})
	for i := 0; i < maxConcurrentBeacons; i++ {
		sem <- struct{}{}
	}

	// Fire a 51st beacon — should be silently dropped (no panic, no block).
	done := make(chan struct{})
	go func() {
		BeaconEvent("overflow_test", map[string]string{"seq": "51"})
		close(done)
	}()

	select {
	case <-done:
		// Good — beacon was dropped without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("BeaconEvent blocked when semaphore was full — should drop silently")
	}

	// Release all slots and verify cleanup.
	close(blockers)
	for i := 0; i < maxConcurrentBeacons; i++ {
		<-sem
	}

	// Verify beacon works again after draining.
	received := make(chan struct{}, 1)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()
	overrideEndpoint(srv2.URL)

	resetInstallIDState()
	dir := t.TempDir()
	overrideStrumDir(dir)
	defer resetStrumDir()

	BeaconEvent("post_drain", nil)
	select {
	case <-received:
		// Good — beacon fired after slots were freed.
	case <-time.After(2 * time.Second):
		t.Fatal("beacon did not fire after semaphore slots were freed")
	}
}

func TestBeaconError_NilProps(t *testing.T) {
	received := make(chan map[string]any, 1)

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

	BeaconError("nil_props_test", nil)

	var body map[string]any
	select {
	case body = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("beacon not received within timeout")
	}

	if body["event"] != "nil_props_test" {
		t.Errorf("event = %v, want nil_props_test", body["event"])
	}
}
