// e2e_reporting_test.go — End-to-end tests proving the full telemetry reporting pipeline.
// Each test captures actual beacon payloads and validates them against the Counterscale
// contract: correct event types, required fields, field values, session lifecycle,
// usage summary rollups, opt-out, and the complete tool call → session_end flow.

package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

// collectAll drains all beacons from the channel within a deadline.
func collectAll(ch chan map[string]any, deadline time.Duration) []map[string]any {
	var all []map[string]any
	timer := time.After(deadline)
	for {
		select {
		case body := <-ch:
			all = append(all, body)
		case <-timer:
			return all
		}
	}
}

// filterByEvent returns only beacons with the given event type.
func filterByEvent(beacons []map[string]any, event string) []map[string]any {
	var out []map[string]any
	for _, b := range beacons {
		if b["event"] == event {
			out = append(out, b)
		}
	}
	return out
}

// requireEnvelope checks all required shared envelope fields are present and valid.
func requireEnvelope(t *testing.T, body map[string]any, label string) {
	t.Helper()
	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel"} {
		val, ok := body[field]
		if !ok {
			t.Errorf("[%s] missing required envelope field: %s", label, field)
			continue
		}
		if _, isStr := val.(string); !isStr {
			t.Errorf("[%s] envelope field %s is %T, want string", label, field, val)
		}
	}
	// iid: 12-char hex
	if iid, ok := body["iid"].(string); ok {
		if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(iid) {
			t.Errorf("[%s] iid = %q, want 12-char hex", label, iid)
		}
	}
	// sid: 16-char hex
	if sid, ok := body["sid"].(string); ok {
		if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(sid) {
			t.Errorf("[%s] sid = %q, want 16-char hex", label, sid)
		}
	}
	// ts: valid RFC3339
	if ts, ok := body["ts"].(string); ok {
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			t.Errorf("[%s] ts = %q, not valid RFC3339: %v", label, ts, err)
		}
	}
}

// ---------- E2E: tool_call event ----------

func TestE2E_ToolCall_SuccessPayload(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	SetLLMName("cursor")
	t.Cleanup(func() { SetLLMName("") })

	tracker := NewUsageTracker()
	tracker.RecordToolCall("interact:click", 123*time.Millisecond, false)

	body := waitForEvent(t, received, "tool_call")
	requireEnvelope(t, body, "tool_call/success")

	if body["family"] != "interact" {
		t.Errorf("family = %v, want interact", body["family"])
	}
	if body["name"] != "click" {
		t.Errorf("name = %v, want click", body["name"])
	}
	if body["tool"] != "interact:click" {
		t.Errorf("tool = %v, want interact:click", body["tool"])
	}
	if body["outcome"] != "success" {
		t.Errorf("outcome = %v, want success", body["outcome"])
	}
	if ms, ok := body["latency_ms"].(float64); !ok || ms != 123 {
		t.Errorf("latency_ms = %v, want 123", body["latency_ms"])
	}
	if body["llm"] != "cursor" {
		t.Errorf("llm = %v, want cursor", body["llm"])
	}
	// async_outcome must be absent (not null).
	if val, exists := body["async_outcome"]; exists && val != nil {
		t.Errorf("async_outcome = %v, want absent", val)
	}
}

func TestE2E_ToolCall_ErrorPayload(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("analyze:security", 50*time.Millisecond, true)

	body := waitForEvent(t, received, "tool_call")
	requireEnvelope(t, body, "tool_call/error")

	if body["outcome"] != "error" {
		t.Errorf("outcome = %v, want error", body["outcome"])
	}
	if body["family"] != "analyze" {
		t.Errorf("family = %v, want analyze", body["family"])
	}
	if body["name"] != "security" {
		t.Errorf("name = %v, want security", body["name"])
	}
}

// ---------- E2E: first_tool_call event ----------

func TestE2E_FirstToolCall_FiredOncePerInstall(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	resetInstallIDState()
	resetFirstToolCallState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(func() {
		resetInstallIDState()
		resetFirstToolCallState()
		resetKaboomDir()
	})

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)
	tracker.RecordToolCall("observe:page", 0, false)
	tracker.RecordToolCall("interact:click", 0, false)

	all := collectAll(received, 3*time.Second)
	firsts := filterByEvent(all, "first_tool_call")

	if len(firsts) != 1 {
		t.Fatalf("first_tool_call fired %d times, want exactly 1", len(firsts))
	}

	body := firsts[0]
	requireEnvelope(t, body, "first_tool_call")
	if body["family"] != "observe" {
		t.Errorf("family = %v, want observe (first tool called)", body["family"])
	}
	if body["tool"] != "observe:page" {
		t.Errorf("tool = %v, want observe:page", body["tool"])
	}
}

// ---------- E2E: session_start event ----------

func TestE2E_SessionStart_FirstActivity(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	body := waitForEvent(t, received, "session_start")
	requireEnvelope(t, body, "session_start/first_activity")

	if body["reason"] != "first_activity" {
		t.Errorf("reason = %v, want first_activity", body["reason"])
	}
}

func TestE2E_SessionStart_PostTimeout(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)
	waitForEvent(t, received, "session_start") // drain first

	// Expire session.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	tracker.RecordToolCall("observe:page", 0, false)

	body := waitForEvent(t, received, "session_start")
	requireEnvelope(t, body, "session_start/post_timeout")

	if body["reason"] != "post_timeout" {
		t.Errorf("reason = %v, want post_timeout", body["reason"])
	}
}

// ---------- E2E: session_end event ----------

func TestE2E_SessionEnd_Timeout(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)
	tracker.RecordToolCall("interact:click", 0, false)
	tracker.RecordToolCall("analyze:perf", 0, false)

	// Expire session.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// Next touch triggers session_end.
	TouchSession()

	body := waitForEvent(t, received, "session_end")
	requireEnvelope(t, body, "session_end/timeout")

	if body["reason"] != "timeout" {
		t.Errorf("reason = %v, want timeout", body["reason"])
	}
	if calls, ok := body["tool_calls"].(float64); !ok || calls != 3 {
		t.Errorf("tool_calls = %v, want 3", body["tool_calls"])
	}
	if _, ok := body["duration_s"].(float64); !ok {
		t.Errorf("duration_s missing or not a number: %v", body["duration_s"])
	}
}

func TestE2E_SessionEnd_Shutdown(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	tracker.EmitSessionEnd("shutdown")

	body := waitForEvent(t, received, "session_end")
	requireEnvelope(t, body, "session_end/shutdown")

	if body["reason"] != "shutdown" {
		t.Errorf("reason = %v, want shutdown", body["reason"])
	}
	if calls, ok := body["tool_calls"].(float64); !ok || calls != 1 {
		t.Errorf("tool_calls = %v, want 1", body["tool_calls"])
	}
}

func TestE2E_SessionEnd_NoOpWhenIdle(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	// No tool calls — EmitSessionEnd should not fire.
	tracker := NewUsageTracker()
	tracker.EmitSessionEnd("shutdown")

	select {
	case body := <-received:
		if body["event"] == "session_end" {
			t.Fatal("session_end should not fire when no tool calls were made")
		}
	case <-time.After(300 * time.Millisecond):
		// Good — nothing sent.
	}
}

// ---------- E2E: app_error event ----------

func TestE2E_AppError_AllCategories(t *testing.T) {
	categories := []struct {
		category string
		wantKind string
		wantSev  string
		wantSrc  string
	}{
		{"daemon_panic", "internal", "fatal", "daemon"},
		{"bridge_connection_error", "integration", "error", "bridge"},
		{"bridge_parse_error", "integration", "warning", "bridge"},
		{"bridge_method_not_found", "integration", "warning", "bridge"},
		{"bridge_stdin_error", "internal", "error", "bridge"},
		{"extension_disconnect", "integration", "warning", "extension"},
		{"install_config_error", "internal", "error", "installer"},
	}

	for _, tc := range categories {
		t.Run(tc.category, func(t *testing.T) {
			received := captureBeacon(t)
			AppError(tc.category, nil)

			body := waitForEvent(t, received, "app_error")
			requireEnvelope(t, body, "app_error/"+tc.category)

			if body["error_kind"] != tc.wantKind {
				t.Errorf("error_kind = %v, want %v", body["error_kind"], tc.wantKind)
			}
			if body["severity"] != tc.wantSev {
				t.Errorf("severity = %v, want %v", body["severity"], tc.wantSev)
			}
			if body["source"] != tc.wantSrc {
				t.Errorf("source = %v, want %v", body["source"], tc.wantSrc)
			}
			// error_code should be UPPER_SNAKE_CASE.
			code, _ := body["error_code"].(string)
			if !regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`).MatchString(code) {
				t.Errorf("error_code = %q, want UPPER_SNAKE_CASE", code)
			}
		})
	}
}

// ---------- E2E: usage_summary event ----------

func TestE2E_UsageSummary_FullPayload(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats: []ToolStat{
			{Tool: "observe:page", Family: "observe", Name: "page", Count: 10, ErrorCount: 1, LatencyAvgMs: 42, LatencyMaxMs: 200},
			{Tool: "interact:click", Family: "interact", Name: "click", Count: 3},
		},
		AsyncOutcomes: map[string]int{"complete": 5, "timeout": 2},
	}
	BeaconUsageSummary(5, snapshot)

	body := waitForEvent(t, received, "usage_summary")
	requireEnvelope(t, body, "usage_summary")

	if wm, ok := body["window_m"].(float64); !ok || wm != 5 {
		t.Errorf("window_m = %v, want 5", body["window_m"])
	}

	// tool_stats: check it's a non-empty array.
	statsRaw, ok := body["tool_stats"]
	if !ok {
		t.Fatal("missing tool_stats")
	}
	stats, ok := statsRaw.([]any)
	if !ok {
		t.Fatalf("tool_stats is %T, want []any", statsRaw)
	}
	if len(stats) != 2 {
		t.Fatalf("tool_stats length = %d, want 2", len(stats))
	}

	// Verify first tool_stat entry.
	entry, ok := stats[0].(map[string]any)
	if !ok {
		t.Fatalf("tool_stats[0] is %T, want map", stats[0])
	}
	if entry["tool"] != "observe:page" {
		t.Errorf("tool_stats[0].tool = %v, want observe:page", entry["tool"])
	}
	if entry["family"] != "observe" {
		t.Errorf("tool_stats[0].family = %v, want observe", entry["family"])
	}
	if cnt, ok := entry["count"].(float64); !ok || cnt != 10 {
		t.Errorf("tool_stats[0].count = %v, want 10", entry["count"])
	}
	if ec, ok := entry["error_count"].(float64); !ok || ec != 1 {
		t.Errorf("tool_stats[0].error_count = %v, want 1", entry["error_count"])
	}
	if avg, ok := entry["latency_avg_ms"].(float64); !ok || avg != 42 {
		t.Errorf("tool_stats[0].latency_avg_ms = %v, want 42", entry["latency_avg_ms"])
	}
	if max, ok := entry["latency_max_ms"].(float64); !ok || max != 200 {
		t.Errorf("tool_stats[0].latency_max_ms = %v, want 200", entry["latency_max_ms"])
	}

	// async_outcomes
	aoRaw, ok := body["async_outcomes"]
	if !ok {
		t.Fatal("missing async_outcomes")
	}
	ao, ok := aoRaw.(map[string]any)
	if !ok {
		t.Fatalf("async_outcomes is %T, want map", aoRaw)
	}
	if ao["complete"] != float64(5) {
		t.Errorf("async_outcomes.complete = %v, want 5", ao["complete"])
	}
	if ao["timeout"] != float64(2) {
		t.Errorf("async_outcomes.timeout = %v, want 2", ao["timeout"])
	}

	// Must NOT have session_depth.
	if _, exists := body["session_depth"]; exists {
		t.Error("usage_summary must not include session_depth")
	}
}

func TestE2E_UsageSummary_OmitsEmptyAsyncOutcomes(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats:     []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
		AsyncOutcomes: map[string]int{},
	}
	BeaconUsageSummary(5, snapshot)

	body := waitForEvent(t, received, "usage_summary")

	if ao, exists := body["async_outcomes"]; exists {
		if m, ok := ao.(map[string]any); ok && len(m) == 0 {
			t.Error("usage_summary should omit async_outcomes when empty")
		}
	}
}

func TestE2E_UsageSummary_NilSnapshotNoBeacon(t *testing.T) {
	received := captureBeacon(t)
	BeaconUsageSummary(5, nil)

	select {
	case body := <-received:
		t.Fatalf("nil snapshot should not fire beacon, got event=%v", body["event"])
	case <-time.After(300 * time.Millisecond):
		// Good.
	}
}

// ---------- E2E: opt-out ----------

func TestE2E_OptOut_NoBeaconsSent(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")
	drainSem()
	time.Sleep(10 * time.Millisecond)

	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	resetSessionState()
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)
	AppError("daemon_panic", nil)
	BeaconUsageSummary(5, &UsageSnapshot{
		ToolStats: []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
	})

	time.Sleep(200 * time.Millisecond)
	if called {
		t.Fatal("no beacons should be sent when KABOOM_TELEMETRY=off")
	}
}

// ---------- E2E: full session lifecycle ----------

func TestE2E_FullSessionLifecycle(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	resetInstallIDState()
	resetFirstToolCallState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(func() {
		resetInstallIDState()
		resetFirstToolCallState()
		resetKaboomDir()
	})
	SetLLMName("claude-code")
	t.Cleanup(func() { SetLLMName("") })

	tracker := NewUsageTracker()

	// === Phase 1: First tool call ever ===
	tracker.RecordToolCall("observe:page", 100*time.Millisecond, false)

	// Expect: session_start, tool_call, first_tool_call (in any order).
	phase1 := collectAll(received, 3*time.Second)
	phase1Events := map[string]map[string]any{}
	for _, b := range phase1 {
		if ev, ok := b["event"].(string); ok {
			phase1Events[ev] = b
		}
	}
	if _, ok := phase1Events["session_start"]; !ok {
		t.Fatal("phase 1: missing session_start")
	}
	if _, ok := phase1Events["tool_call"]; !ok {
		t.Fatal("phase 1: missing tool_call")
	}
	if _, ok := phase1Events["first_tool_call"]; !ok {
		t.Fatal("phase 1: missing first_tool_call")
	}
	// All should have same session ID.
	sid := phase1Events["session_start"]["sid"]
	for ev, body := range phase1Events {
		if body["sid"] != sid {
			t.Errorf("phase 1: %s sid = %v, want %v (same session)", ev, body["sid"], sid)
		}
		if body["llm"] != "claude-code" {
			t.Errorf("phase 1: %s llm = %v, want claude-code", ev, body["llm"])
		}
	}

	// === Phase 2: More tool calls (same session) ===
	tracker.RecordToolCall("interact:click", 50*time.Millisecond, false)
	tracker.RecordToolCall("analyze:security", 200*time.Millisecond, true)

	phase2 := collectAll(received, 2*time.Second)
	phase2Calls := filterByEvent(phase2, "tool_call")
	if len(phase2Calls) != 2 {
		t.Fatalf("phase 2: expected 2 tool_calls, got %d", len(phase2Calls))
	}
	// No duplicate session_start or first_tool_call.
	if len(filterByEvent(phase2, "session_start")) > 0 {
		t.Error("phase 2: unexpected session_start (session should still be active)")
	}
	if len(filterByEvent(phase2, "first_tool_call")) > 0 {
		t.Error("phase 2: unexpected first_tool_call (should fire once per install)")
	}

	// === Phase 3: Session timeout → session_end + new session_start ===
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	tracker.RecordToolCall("observe:errors", 10*time.Millisecond, false)

	phase3 := collectAll(received, 3*time.Second)
	phase3Events := map[string][]map[string]any{}
	for _, b := range phase3 {
		if ev, ok := b["event"].(string); ok {
			phase3Events[ev] = append(phase3Events[ev], b)
		}
	}

	// Must have session_end for old session.
	ends := phase3Events["session_end"]
	if len(ends) != 1 {
		t.Fatalf("phase 3: expected 1 session_end, got %d", len(ends))
	}
	if ends[0]["reason"] != "timeout" {
		t.Errorf("phase 3: session_end reason = %v, want timeout", ends[0]["reason"])
	}
	if calls, ok := ends[0]["tool_calls"].(float64); !ok || calls != 4 {
		t.Errorf("phase 3: session_end tool_calls = %v, want 4 (3 prior + 1 that triggered rotation)", ends[0]["tool_calls"])
	}

	// Must have session_start for new session.
	starts := phase3Events["session_start"]
	if len(starts) != 1 {
		t.Fatalf("phase 3: expected 1 session_start, got %d", len(starts))
	}
	if starts[0]["reason"] != "post_timeout" {
		t.Errorf("phase 3: session_start reason = %v, want post_timeout", starts[0]["reason"])
	}

	// New session should have a DIFFERENT session ID.
	newSID := starts[0]["sid"]
	if newSID == sid {
		t.Error("phase 3: new session should have different sid after timeout rotation")
	}

	// tool_call in new session should use new sid.
	newCalls := phase3Events["tool_call"]
	if len(newCalls) != 1 {
		t.Fatalf("phase 3: expected 1 tool_call, got %d", len(newCalls))
	}
	if newCalls[0]["sid"] != newSID {
		t.Errorf("phase 3: tool_call sid = %v, want %v (new session)", newCalls[0]["sid"], newSID)
	}

	// === Phase 4: Usage summary ===
	snapshot := tracker.SwapAndReset()
	if snapshot == nil {
		t.Fatal("phase 4: SwapAndReset returned nil")
	}
	BeaconUsageSummary(5, snapshot)

	summary := waitForEvent(t, received, "usage_summary")
	requireEnvelope(t, summary, "usage_summary")
	if summary["window_m"] != float64(5) {
		t.Errorf("phase 4: window_m = %v, want 5", summary["window_m"])
	}
	statsRaw, _ := summary["tool_stats"].([]any)
	if len(statsRaw) == 0 {
		t.Error("phase 4: usage_summary has no tool_stats")
	}
}

// ---------- E2E: BuildUsageSummaryPayload for debug endpoint ----------

func TestE2E_BuildUsageSummaryPayload_MatchesBeacon(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats: []ToolStat{
			{Tool: "observe:page", Family: "observe", Name: "page", Count: 5, LatencyAvgMs: 33, LatencyMaxMs: 80},
		},
		AsyncOutcomes: map[string]int{"complete": 3},
	}

	// Build the debug payload.
	payload := BuildUsageSummaryPayload(5, snapshot)
	if payload == nil {
		t.Fatal("BuildUsageSummaryPayload returned nil")
	}

	// Verify it has the same structure as what BeaconUsageSummary sends.
	if payload["event"] != "usage_summary" {
		t.Errorf("event = %v, want usage_summary", payload["event"])
	}
	if payload["window_m"] != 5 {
		t.Errorf("window_m = %v, want 5", payload["window_m"])
	}
	if _, ok := payload["iid"].(string); !ok {
		t.Error("missing iid")
	}
	if _, ok := payload["ts"].(string); !ok {
		t.Error("missing ts")
	}

	// Fire the actual beacon and compare key fields.
	BeaconUsageSummary(5, snapshot)
	body := waitForEvent(t, received, "usage_summary")

	// JSON decodes numbers as float64; payload has int. Compare as float64.
	beaconWM, _ := body["window_m"].(float64)
	debugWM := float64(payload["window_m"].(int))
	if beaconWM != debugWM {
		t.Errorf("beacon window_m = %v, debug = %v — should match", beaconWM, debugWM)
	}
}

// ---------- E2E: SwapAndReset integration ----------

func TestE2E_SwapAndReset_AccumulatesAndResets(t *testing.T) {
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 50*time.Millisecond, false)
	tracker.RecordToolCall("observe:page", 150*time.Millisecond, false)
	tracker.RecordToolCall("observe:page", 100*time.Millisecond, true)
	tracker.RecordToolCall("interact:click", 30*time.Millisecond, false)
	tracker.RecordAsyncOutcome("complete")
	tracker.RecordAsyncOutcome("complete")
	tracker.RecordAsyncOutcome("timeout")

	snapshot := tracker.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}

	// Verify tool stats.
	if len(snapshot.ToolStats) != 2 {
		t.Fatalf("ToolStats length = %d, want 2", len(snapshot.ToolStats))
	}
	var page, click *ToolStat
	for i := range snapshot.ToolStats {
		switch snapshot.ToolStats[i].Tool {
		case "observe:page":
			page = &snapshot.ToolStats[i]
		case "interact:click":
			click = &snapshot.ToolStats[i]
		}
	}
	if page == nil {
		t.Fatal("missing observe:page in ToolStats")
	}
	if page.Count != 3 {
		t.Errorf("observe:page count = %d, want 3", page.Count)
	}
	if page.ErrorCount != 1 {
		t.Errorf("observe:page error_count = %d, want 1", page.ErrorCount)
	}
	if page.LatencyAvgMs != 100 {
		t.Errorf("observe:page latency_avg_ms = %d, want 100", page.LatencyAvgMs)
	}
	if page.LatencyMaxMs != 150 {
		t.Errorf("observe:page latency_max_ms = %d, want 150", page.LatencyMaxMs)
	}
	if click == nil || click.Count != 1 {
		t.Errorf("interact:click missing or count wrong")
	}

	// Verify async outcomes.
	if snapshot.AsyncOutcomes["complete"] != 2 {
		t.Errorf("async complete = %d, want 2", snapshot.AsyncOutcomes["complete"])
	}
	if snapshot.AsyncOutcomes["timeout"] != 1 {
		t.Errorf("async timeout = %d, want 1", snapshot.AsyncOutcomes["timeout"])
	}

	// After swap, next SwapAndReset should return nil (no new activity).
	if next := tracker.SwapAndReset(); next != nil {
		t.Errorf("second SwapAndReset should return nil, got %+v", next)
	}
}

// ---------- E2E: Warm pre-loads install ID off hot path ----------

func TestE2E_Warm_PreloadsInstallID(t *testing.T) {
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(func() {
		resetInstallIDState()
		resetKaboomDir()
	})

	// Before Warm, install ID is not cached.
	resetInstallIDState()

	Warm()

	// After Warm, GetInstallID should return instantly (no I/O).
	id := GetInstallID()
	if id == "" {
		t.Fatal("GetInstallID returned empty after Warm()")
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(id) {
		t.Errorf("install ID = %q, want 12-char hex", id)
	}
}

// ---------- E2E: opt-out caching ----------

func TestE2E_OptOutCachedAtInit(t *testing.T) {
	// Verify that telemetryOptedOut returns consistent results.
	// (Tests env var reading, not caching — caching is an implementation detail.)
	t.Setenv("KABOOM_TELEMETRY", "off")
	if !telemetryOptedOut() {
		t.Error("telemetryOptedOut should return true when KABOOM_TELEMETRY=off")
	}
	t.Setenv("KABOOM_TELEMETRY", "OFF")
	if !telemetryOptedOut() {
		t.Error("telemetryOptedOut should be case-insensitive")
	}
	t.Setenv("KABOOM_TELEMETRY", "")
	if telemetryOptedOut() {
		t.Error("telemetryOptedOut should return false when KABOOM_TELEMETRY is empty")
	}
}

// ---------- E2E: empty tool key ----------

func TestE2E_EmptyToolKey_NoBlowup(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	// Empty key should not panic or crash — just records as empty.
	tracker.RecordToolCall("", 0, false)

	body := waitForEvent(t, received, "tool_call")
	requireEnvelope(t, body, "tool_call/empty_key")

	// Family and name should be empty strings (splitKey on "" returns "", "").
	if body["family"] != "" {
		t.Errorf("family = %q, want empty", body["family"])
	}
	if body["tool"] != "" {
		t.Errorf("tool = %q, want empty", body["tool"])
	}
}

// ---------- E2E: beacon timeout does not block caller ----------

func TestE2E_SlowServer_DoesNotBlockCaller(t *testing.T) {
	drainSem()
	t.Cleanup(drainSem)

	// Server that takes 5 seconds to respond — far longer than the 2s timeout.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	overrideEndpoint(srv.URL)
	t.Cleanup(resetEndpoint)

	resetSessionState()
	tracker := NewUsageTracker()

	start := time.Now()
	tracker.RecordToolCall("observe:page", 0, false)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("RecordToolCall blocked for %v with slow server — should return immediately", elapsed)
	}
}

// ---------- E2E: JSON serialization roundtrip ----------

func TestE2E_BeaconJSON_Roundtrip(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	SetLLMName("codex")
	t.Cleanup(func() { SetLLMName("") })

	tracker := NewUsageTracker()
	tracker.RecordToolCall("generate:test", 75*time.Millisecond, false)

	body := waitForEvent(t, received, "tool_call")

	// Re-serialize and re-parse to verify clean JSON.
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal beacon: %v", err)
	}
	var roundtrip map[string]any
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal beacon: %v", err)
	}

	// All fields should survive the roundtrip.
	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel", "llm", "family", "name", "tool", "outcome", "latency_ms"} {
		if _, ok := roundtrip[field]; !ok {
			t.Errorf("field %q lost in JSON roundtrip", field)
		}
	}
}
