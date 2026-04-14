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
	t.Setenv("KABOOM_TELEMETRY", "off")

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
		t.Fatal("BeaconError should not make HTTP calls when Kaboom_TELEMETRY=off")
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

	// Verify session ID is present and is 16-char hex.
	sid, ok := body["sid"].(string)
	if !ok {
		t.Fatalf("missing or non-string 'sid' field in beacon payload")
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(sid) {
		t.Errorf("sid = %q, want 16-char hex string", sid)
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

// drainSem empties the semaphore so tests start from a clean state.
func drainSem() {
	for {
		select {
		case <-sem:
		default:
			return
		}
	}
}

func TestBeacon_SemaphoreBackpressure(t *testing.T) {
	// Ensure clean semaphore state on entry and exit.
	drainSem()
	t.Cleanup(drainSem)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	// Fill all semaphore slots.
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

	// Drain and verify beacon works again after slots are freed.
	drainSem()

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
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	BeaconEvent("post_drain", nil)
	select {
	case <-received:
		// Good — beacon fired after slots were freed.
	case <-time.After(2 * time.Second):
		t.Fatal("beacon did not fire after semaphore slots were freed")
	}
}

// #5: SetLLMName inclusion/omission in beacon payload.
func TestBeacon_IncludesLLMName(t *testing.T) {
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
	SetLLMName("claude-code")
	defer SetLLMName("")

	BeaconEvent("llm_test", nil)

	var body map[string]any
	select {
	case body = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("beacon not received within timeout")
	}

	if body["llm"] != "claude-code" {
		t.Errorf("llm = %v, want claude-code", body["llm"])
	}
}

func TestBeacon_OmitsLLMNameWhenEmpty(t *testing.T) {
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
	SetLLMName("")

	BeaconEvent("no_llm_test", nil)

	var body map[string]any
	select {
	case body = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("beacon not received within timeout")
	}

	if _, exists := body["llm"]; exists {
		t.Errorf("llm key should be absent when empty, got %v", body["llm"])
	}
}

// #14: Opt-out tests for BeaconEvent and BeaconUsageSummary.
func TestBeaconEvent_DisabledByEnv(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")

	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	BeaconEvent("should_not_fire", nil)
	time.Sleep(50 * time.Millisecond)

	if called {
		t.Fatal("BeaconEvent should not fire when KABOOM_TELEMETRY=off")
	}
}

func TestBeaconUsageSummary_DisabledByEnv(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")

	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	BeaconUsageSummary(5, map[string]int{"observe:page": 1})
	time.Sleep(50 * time.Millisecond)

	if called {
		t.Fatal("BeaconUsageSummary should not fire when KABOOM_TELEMETRY=off")
	}
}

// #13: BuildUsageSummaryPayload structure test.
func TestBuildUsageSummaryPayload_Structure(t *testing.T) {
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()
	resetSessionState()
	TouchSession()

	props := map[string]int{"observe:page": 3, "interact:click": 1}
	payload := BuildUsageSummaryPayload(5, props)

	if payload["event"] != "usage_summary" {
		t.Errorf("event = %v, want usage_summary", payload["event"])
	}
	if payload["window_m"] != 5 {
		t.Errorf("window_m = %v, want 5", payload["window_m"])
	}
	if _, ok := payload["iid"].(string); !ok {
		t.Error("missing iid field")
	}
	if _, ok := payload["sid"].(string); !ok {
		t.Error("missing sid field")
	}
	if _, ok := payload["v"].(string); !ok {
		t.Error("missing v field")
	}
	if _, ok := payload["os"].(string); !ok {
		t.Error("missing os field")
	}
	p, ok := payload["props"].(map[string]int)
	if !ok {
		t.Fatalf("props type = %T, want map[string]int", payload["props"])
	}
	if p["observe:page"] != 3 {
		t.Errorf("props observe:page = %d, want 3", p["observe:page"])
	}
}

// #6: Semaphore cleanup safety — drainSem prevents leaked slots from poisoning subsequent tests.
func TestBeacon_SemaphoreCleanupOnFailure(t *testing.T) {
	drainSem()
	t.Cleanup(drainSem)

	// Fill sem completely.
	for i := 0; i < maxConcurrentBeacons; i++ {
		sem <- struct{}{}
	}

	// Verify beacon is dropped (not blocked) when semaphore is full.
	done := make(chan struct{})
	go func() {
		BeaconEvent("overflow", nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("beacon blocked on full semaphore")
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
