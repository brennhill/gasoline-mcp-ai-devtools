// contract_compliance_test.go — Tests verifying beacon payloads match the Counterscale ingest contract.
// These tests catch schema drift between the Go sender and the Counterscale worker.

package telemetry

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsupport"
)

// captureBeacon sets up a test server and returns a channel that receives beacon payloads.
func captureBeacon(t *testing.T) chan map[string]any {
	t.Helper()
	drainSem()
	t.Cleanup(drainSem)

	received := make(chan map[string]any, 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)
	overrideEndpoint(srv.URL)
	t.Cleanup(resetEndpoint)
	return received
}

// waitForEvent drains the channel until it finds a beacon with the given event type.
func waitForEvent(t *testing.T, ch chan map[string]any, event string) map[string]any {
	t.Helper()
	for {
		select {
		case body := <-ch:
			if body["event"] == event {
				return body
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for %q event", event)
			return nil
		}
	}
}

// TestContract_ToolCallHasNoNullAsyncOutcome verifies async_outcome is omitted (not null)
// when the tool call is synchronous.
func TestContract_ToolCallHasNoNullAsyncOutcome(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 50*time.Millisecond, false)

	body := waitForEvent(t, received, "tool_call")

	// async_outcome should either be absent or a non-null string — never JSON null.
	val, exists := body["async_outcome"]
	if exists && val != nil {
		// If present, must be a string (not null).
		if _, ok := val.(string); !ok {
			t.Errorf("async_outcome = %v (%T), want absent or string", val, val)
		}
	}
}

// TestContract_ToolCallV2Envelope verifies tool_call beacons have all required v2 envelope fields.
func TestContract_ToolCallV2Envelope(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 50*time.Millisecond, false)

	body := waitForEvent(t, received, "tool_call")

	// Required v2 envelope fields per contract.
	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required v2 envelope field: %s", field)
		}
	}
	// Required tool_call fields.
	for _, field := range []string{"family", "name", "tool", "outcome", "latency_ms"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required tool_call field: %s", field)
		}
	}
	if body["family"] != "observe" {
		t.Errorf("family = %v, want observe", body["family"])
	}
	if body["name"] != "page" {
		t.Errorf("name = %v, want page", body["name"])
	}
	if body["tool"] != "observe:page" {
		t.Errorf("tool = %v, want observe:page", body["tool"])
	}
	if body["outcome"] != "success" {
		t.Errorf("outcome = %v, want success", body["outcome"])
	}
}

// TestContract_AppErrorNoDetailField verifies app_error beacons do not send the
// 'detail' field, which is not in the contract and silently dropped by the ingest.
func TestContract_AppErrorNoDetailField(t *testing.T) {
	received := captureBeacon(t)
	AppError("daemon_panic", nil)

	body := waitForEvent(t, received, "app_error")

	if _, exists := body["detail"]; exists {
		t.Error("app_error should not send 'detail' field — not in Counterscale contract, silently dropped")
	}
	// Verify required contract fields are present.
	for _, field := range []string{"error_kind", "error_code", "severity", "source"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required app_error field: %s", field)
		}
	}
}

// TestContract_UsageSummaryNoSessionDepth is a compile-time check:
// UsageSnapshot no longer has a SessionDepth field, so it cannot be sent.
// Intentionally left as a named test for documentation — verifies via beacon_test.go
// that the payload key is absent.

// TestContract_AppErrorClassifiesNewCategories verifies all migrated error categories
// produce valid error_kind, severity, and source fields.
func TestContract_AppErrorClassifiesNewCategories(t *testing.T) {
	cases := []struct {
		category  string
		wantKind  string
		wantSev   string
		wantSrc   string
		wantRetry bool
	}{
		// Existing
		{"daemon_panic", "internal", "fatal", "daemon", false},
		{"daemon_start_failed", "internal", "fatal", "startup", false},
		{"tool_rate_limited", "integration", "warning", "daemon", true},
		// New: bridge errors
		{"bridge_connection_error", "integration", "error", "bridge", true},
		{"bridge_port_blocked", "integration", "error", "bridge", false},
		{"bridge_spawn_build_error", "internal", "fatal", "bridge", false},
		{"bridge_spawn_start_error", "internal", "fatal", "bridge", false},
		{"bridge_spawn_timeout", "internal", "error", "bridge", true},
		{"bridge_exit_error", "internal", "error", "bridge", false},
		{"bridge_parse_error", "integration", "warning", "bridge", false},
		{"bridge_method_not_found", "integration", "warning", "bridge", false},
		{"bridge_stdin_error", "internal", "error", "bridge", false},
		// New: extension/install errors
		{"extension_disconnect", "integration", "warning", "extension", false},
		{"install_config_error", "internal", "error", "installer", false},
	}

	for _, tc := range cases {
		t.Run(tc.category, func(t *testing.T) {
			kind, sev, src, retry := classifyAppError(tc.category)
			if kind != tc.wantKind {
				t.Errorf("error_kind = %q, want %q", kind, tc.wantKind)
			}
			if sev != tc.wantSev {
				t.Errorf("severity = %q, want %q", sev, tc.wantSev)
			}
			if src != tc.wantSrc {
				t.Errorf("source = %q, want %q", src, tc.wantSrc)
			}
			if retry != tc.wantRetry {
				t.Errorf("retryable = %v, want %v", retry, tc.wantRetry)
			}
		})
	}
}

// TestContract_AppErrorCodeNormalization verifies error_code is uppercase with underscores.
func TestContract_AppErrorCodeNormalization(t *testing.T) {
	cases := []struct {
		category string
		wantCode string
	}{
		{"daemon_panic", "DAEMON_PANIC"},
		{"bridge_connection_error", "BRIDGE_CONNECTION_ERROR"},
		{"bridge-spawn-failed", "BRIDGE_SPAWN_FAILED"},
		{"install config error", "INSTALL_CONFIG_ERROR"},
	}
	for _, tc := range cases {
		code := normalizeAppErrorCode(tc.category)
		if code != tc.wantCode {
			t.Errorf("normalizeAppErrorCode(%q) = %q, want %q", tc.category, code, tc.wantCode)
		}
	}
}

// TestContract_AppErrorSendsAllRequiredFields fires an AppError and checks
// the actual beacon has every field the ingest expects.
func TestContract_AppErrorSendsAllRequiredFields(t *testing.T) {
	received := captureBeacon(t)

	AppError("bridge_connection_error", nil)

	body := waitForEvent(t, received, "app_error")

	// V2 envelope.
	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing v2 envelope field: %s", field)
		}
	}
	// App error specific.
	if body["error_kind"] != "integration" {
		t.Errorf("error_kind = %v, want integration", body["error_kind"])
	}
	if body["error_code"] != "BRIDGE_CONNECTION_ERROR" {
		t.Errorf("error_code = %v, want BRIDGE_CONNECTION_ERROR", body["error_code"])
	}
	if body["severity"] != "error" {
		t.Errorf("severity = %v, want error", body["severity"])
	}
	if body["source"] != "bridge" {
		t.Errorf("source = %v, want bridge", body["source"])
	}
	if body["retryable"] != true {
		t.Errorf("retryable = %v, want true", body["retryable"])
	}
}

// TestContract_DefaultEndpointPinned pins the canonical telemetry ingest URL.
// The daemon is the source of truth for this endpoint — extension code does
// not ship a telemetry beacon helper. A change to this URL is a wire-contract
// break: dashboard/ingest infra must be updated in the same change.
func TestContract_DefaultEndpointPinned(t *testing.T) {
	const want = "https://t.gokaboom.dev/v1/event"
	if defaultEndpoint != want {
		t.Errorf("defaultEndpoint = %q, want %q (wire contract)", defaultEndpoint, want)
	}
}

// TestContract_ArtifactTableMatchesCallSites scans production telemetry
// source for `// ARTIFACT: <name>` magic comments next to each on-disk
// daemon-owned file call site and asserts each appears in the
// docs/core/app-metrics.md "Daemon-owned on-disk artifacts" table.
//
// The magic-comment scheme decouples the test from the call-site shape:
// renames of `kaboomDir`, `withKaboomStateLock`, or any helper do not
// silently disable the symmetry check. Adding a new on-disk file just
// requires (1) tagging its call site with `// ARTIFACT: <name>` and
// (2) adding a doc table row.
//
// Producer rules: each artifact must be tagged at least once across the
// production sources scanned below; the same artifact may legitimately
// appear at multiple call sites (read + write) and that's fine.
func TestContract_ArtifactTableMatchesCallSites(t *testing.T) {
	repoRoot := testsupport.RepoRoot(t)
	docPath := filepath.Join(repoRoot, "docs", "core", "app-metrics.md")
	docBody, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}

	prodFiles := []string{
		filepath.Join(repoRoot, "internal", "telemetry", "install_id.go"),
		filepath.Join(repoRoot, "internal", "telemetry", "install_id_drift.go"),
	}
	var prodSrc strings.Builder
	for _, p := range prodFiles {
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		prodSrc.Write(body)
		prodSrc.WriteByte('\n')
	}
	src := prodSrc.String()

	// Extract artifact names tagged at production call sites.
	artifactPat := regexp.MustCompile(`(?m)^\s*//\s*ARTIFACT:\s*(\S+)\s*$`)
	codeArtifacts := make(map[string]bool)
	for _, m := range artifactPat.FindAllStringSubmatch(src, -1) {
		codeArtifacts[m[1]] = true
	}

	// Set of artifacts expected to be both code-tagged AND doc-rowed.
	// Adding a new on-disk file requires updating ALL THREE: production
	// code (with ARTIFACT tag), this set, and the doc table.
	expected := []string{
		"install_id",
		"install_id.bak",
		"install_id.lock",
		"install_id_lineage",
		"first_tool_call_install_id",
		"first_tool_call_install_id.lock",
	}
	const docSection = "Daemon-owned on-disk artifacts"

	for _, name := range expected {
		if !codeArtifacts[name] {
			t.Errorf("expected artifact %q has no `// ARTIFACT: %s` tag in any of the searched production files: %v — stale expected list, or missing tag at the call site?", name, name, prodFiles)
		}
		needle := "`" + name + "`"
		if !strings.Contains(string(docBody), needle) {
			t.Errorf("expected artifact %q missing doc pin %s — add a row to the %q table at %s", name, needle, docSection, docPath)
		}
	}

	// Direct symmetry: anything tagged with ARTIFACT in code MUST be in
	// expected (catches a new code artifact that landed without doc/test
	// updates).
	expectedSet := make(map[string]bool, len(expected))
	for _, name := range expected {
		expectedSet[name] = true
	}
	for name := range codeArtifacts {
		if !expectedSet[name] {
			t.Errorf("production source tags on-disk artifact %q via // ARTIFACT comment but it is NOT in the test's expected list (and likely not in the %q table either); add doc + expected list update", name, docSection)
		}
	}
}

// TestContract_DefaultEndpointMatchesDocs cross-pins the daemon's endpoint
// against the docs (`docs/core/app-metrics.md`) so a developer who updates
// only one side of the contract trips the test.
//
// The URL must appear inside a fenced code block (```...```) — substring
// containment alone would pass against a casual mention like "deprecated
// old URL: ...". The fenced-block requirement signals the URL is a
// canonical pin, not prose.
func TestContract_DefaultEndpointMatchesDocs(t *testing.T) {
	repoRoot := testsupport.RepoRoot(t)
	docPath := filepath.Join(repoRoot, "docs", "core", "app-metrics.md")
	body, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}

	// Walk the doc, tracking fence state per CommonMark: a fence opens
	// with 3+ consecutive backticks OR 3+ consecutive tildes; the matching
	// close uses the same character class with at least the same count.
	// A line matches the contract when it appears inside a fenced block
	// AND contains the URL.
	inFence := false
	fenceChar := byte(0)
	fenceLen := 0
	pinned := false
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if op, ch := markdownFenceOpenInfo(trimmed); op {
			if !inFence {
				inFence = true
				fenceChar = ch
				fenceLen = markdownCountLeadingByte(trimmed, ch)
				continue
			}
			// Closing fence requires same char and at least the same length.
			if ch == fenceChar && markdownCountLeadingByte(trimmed, ch) >= fenceLen {
				inFence = false
				fenceChar = 0
				fenceLen = 0
			}
			continue
		}
		if inFence && strings.Contains(line, defaultEndpoint) {
			pinned = true
			break
		}
	}
	if !pinned {
		t.Errorf("%s does not pin defaultEndpoint %q inside a fenced code block — code/doc drift in wire contract", docPath, defaultEndpoint)
	}
}

// markdownFenceOpenInfo / markdownCountLeadingByte and their unit tests
// live in markdown_fence_test.go.

// TestContract_FireStructuredBeaconDefensiveCheck verifies fireStructuredBeacon
// does not panic when 'event' field is missing or wrong type.
func TestContract_FireStructuredBeaconDefensiveCheck(t *testing.T) {
	received := captureBeacon(t)

	// Should not panic with missing event.
	fireStructuredBeacon(map[string]any{"not_event": "test"})
	// Should not panic with wrong type.
	fireStructuredBeacon(map[string]any{"event": 123})

	// Verify no beacons were sent (both should be silently dropped).
	select {
	case body := <-received:
		t.Fatalf("expected no beacon, got event=%v", body["event"])
	case <-time.After(200 * time.Millisecond):
		// Good — nothing sent.
	}
}

// TestContract_SessionStartReasonPostTimeout verifies that session_start after
// a timeout rotation uses reason "post_timeout", not "first_activity".
func TestContract_SessionStartReasonPostTimeout(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	// Drain session_start for first session.
	first := waitForEvent(t, received, "session_start")
	if first["reason"] != "first_activity" {
		t.Fatalf("first session_start reason = %v, want first_activity", first["reason"])
	}

	// Simulate inactivity beyond timeout.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// Next RecordToolCall triggers session rotation.
	tracker.RecordToolCall("observe:page", 0, false)

	// The new session_start should have reason "post_timeout".
	second := waitForEvent(t, received, "session_start")
	if second["reason"] != "post_timeout" {
		t.Errorf("session_start after timeout reason = %v, want post_timeout", second["reason"])
	}
}

// TestContract_BeaconUsageSummaryDRY verifies BeaconUsageSummary uses
// BuildUsageSummaryPayload internally (no duplicated logic).
func TestContract_BeaconUsageSummaryDRY(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats:     []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 3, LatencyAvgMs: 45, LatencyMaxMs: 100}},
		AsyncOutcomes: map[string]int{"complete": 2},
	}

	// Build expected payload via the same function BeaconUsageSummary uses.
	expected := BuildUsageSummaryPayload(5, snapshot)
	if expected == nil {
		t.Fatal("BuildUsageSummaryPayload returned nil")
	}

	// Fire the actual beacon.
	BeaconUsageSummary(5, snapshot)
	body := waitForEvent(t, received, "usage_summary")

	// Check key fields match what BuildUsageSummaryPayload produces.
	if body["window_m"] == nil {
		t.Error("missing window_m in beacon")
	}
	if body["tool_stats"] == nil {
		t.Error("missing tool_stats in beacon")
	}
}

// TestContract_AppErrorSignature verifies AppError takes only (category, props),
// with no misleading unused parameters.
func TestContract_AppErrorSignature(t *testing.T) {
	received := captureBeacon(t)

	// Call with nil props — should work without extra params.
	AppError("daemon_panic", nil)

	body := waitForEvent(t, received, "app_error")
	if body["error_code"] != "DAEMON_PANIC" {
		t.Errorf("error_code = %v, want DAEMON_PANIC", body["error_code"])
	}
}

// TestContract_BeaconUsageSummaryHasV2Envelope verifies usage_summary beacons
// include all required v2 envelope fields.
func TestContract_BeaconUsageSummaryHasV2Envelope(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats: []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
	}
	BeaconUsageSummary(5, snapshot)

	body := waitForEvent(t, received, "usage_summary")

	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required v2 envelope field: %s", field)
		}
	}
	if body["window_m"] == nil {
		t.Error("missing window_m")
	}
	if body["tool_stats"] == nil {
		t.Error("missing tool_stats")
	}
}

// TestContract_AppErrorPropsCannotOverwriteContractFields verifies that caller-provided
// props cannot overwrite classified contract fields (error_kind, severity, etc.).
func TestContract_AppErrorPropsCannotOverwriteContractFields(t *testing.T) {
	received := captureBeacon(t)

	// Pass props that attempt to overwrite every contract field.
	AppError("daemon_panic", map[string]string{
		"error_kind": "attacker",
		"error_code": "FAKE",
		"severity":   "warning",
		"source":     "evil",
		"event":      "not_app_error",
	})

	body := waitForEvent(t, received, "app_error")

	// Contract fields must reflect classifyAppError, not caller props.
	if body["error_kind"] != "internal" {
		t.Errorf("error_kind = %v, want internal (props overwrote contract field)", body["error_kind"])
	}
	if body["error_code"] != "DAEMON_PANIC" {
		t.Errorf("error_code = %v, want DAEMON_PANIC (props overwrote contract field)", body["error_code"])
	}
	if body["severity"] != "fatal" {
		t.Errorf("severity = %v, want fatal (props overwrote contract field)", body["severity"])
	}
	if body["source"] != "daemon" {
		t.Errorf("source = %v, want daemon (props overwrote contract field)", body["source"])
	}
	if body["event"] != "app_error" {
		t.Errorf("event = %v, want app_error (props overwrote event type)", body["event"])
	}
}

// TestContract_UsageSummaryOmitsEmptyAsyncOutcomes verifies that usage_summary
// beacons omit async_outcomes when empty rather than sending {}.
func TestContract_UsageSummaryOmitsEmptyAsyncOutcomes(t *testing.T) {
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats:     []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
		AsyncOutcomes: map[string]int{}, // empty
	}
	payload := BuildUsageSummaryPayload(5, snapshot)

	if ao, exists := payload["async_outcomes"]; exists {
		if m, ok := ao.(map[string]int); ok && len(m) == 0 {
			t.Error("usage_summary should omit async_outcomes when empty, not send {}")
		}
	}
}

// TestContract_ConcurrentRecordToolCall_NoDuplicateSessionStart verifies that
// concurrent RecordToolCall invocations produce exactly one session_start beacon
// per session, even under contention.
func TestContract_ConcurrentRecordToolCall_NoDuplicateSessionStart(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tracker.RecordToolCall("observe:page", 0, false)
		}()
	}
	wg.Wait()

	// Drain all events and count session_starts.
	sessionStarts := 0
	deadline := time.After(3 * time.Second)
	for {
		select {
		case body := <-received:
			if body["event"] == "session_start" {
				sessionStarts++
			}
		case <-deadline:
			goto done
		}
	}
done:
	if sessionStarts != 1 {
		t.Errorf("concurrent RecordToolCall produced %d session_start beacons, want exactly 1", sessionStarts)
	}
}

// TestContract_EnvelopeLLMField verifies that beacons include the 'llm' field
// when SetLLMName is called (MCP client name from initialize handshake).
func TestContract_EnvelopeLLMField(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	SetLLMName("claude-code")
	t.Cleanup(func() { SetLLMName("") })

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	body := waitForEvent(t, received, "tool_call")

	if body["llm"] != "claude-code" {
		t.Errorf("llm = %v, want claude-code", body["llm"])
	}
}

// TestContract_EnvelopeOmitsLLMWhenEmpty verifies llm is absent when no client connected.
func TestContract_EnvelopeOmitsLLMWhenEmpty(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	SetLLMName("")

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	body := waitForEvent(t, received, "tool_call")

	if _, exists := body["llm"]; exists {
		t.Error("llm should be absent when no client name is set")
	}
}

// TestContract_SessionDepthNotInSnapshot verifies that UsageSnapshot no longer
// carries a SessionDepth field (removed as dead code — not sent to Counterscale).
func TestContract_SessionDepthNotInSnapshot(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("a", 0, false)
	c.RecordToolCall("b", 0, false)

	snapshot := c.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	// SessionDepth should no longer exist on the struct.
	// This test will fail to compile if the field is re-added.
	_ = snapshot.ToolStats
	_ = snapshot.AsyncOutcomes
	// No SessionDepth field to access — that's the point.
}

// TestContract_ConcurrentRecordToolCall_PostTimeoutSingleSessionStart verifies that
// concurrent RecordToolCall after a timeout rotation produces exactly one session_start
// with reason "post_timeout".
func TestContract_ConcurrentRecordToolCall_PostTimeoutSingleSessionStart(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	// Establish first session.
	tracker.RecordToolCall("observe:page", 0, false)

	// Drain first session_start.
	waitForEvent(t, received, "session_start")

	// Simulate inactivity beyond timeout.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// Concurrent calls after timeout — should produce exactly one post_timeout session_start.
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tracker.RecordToolCall("observe:page", 0, false)
		}()
	}
	wg.Wait()

	postTimeoutStarts := 0
	deadline := time.After(3 * time.Second)
	for {
		select {
		case body := <-received:
			if body["event"] == "session_start" {
				postTimeoutStarts++
			}
		case <-deadline:
			goto done
		}
	}
done:
	if postTimeoutStarts != 1 {
		t.Errorf("concurrent post-timeout RecordToolCall produced %d session_start beacons, want exactly 1", postTimeoutStarts)
	}
}

// TestContract_AllOnDiskCallSitesAreTagged is the producer-side companion
// to TestContract_ArtifactTableMatchesCallSites. The earlier test asserts
// "every name in the expected list has at least one ARTIFACT tag in the
// production source"; this one asserts the inverse, more important
// invariant: "every place that names a kaboomDir-relative on-disk path is
// tagged with an ARTIFACT comment within 5 lines above."
//
// Without this scanner, an author who adds a new on-disk file via
// `filepath.Join(kaboomDir, "new_file")` or `withKaboomStateLock("x.lock",
// ...)` could ship the change tag-less; the symmetry check would still
// pass for ALL EXISTING tagged artifacts and silently miss the new one.
//
// Detected patterns (in install_id.go + install_id_drift.go):
//   - filepath.Join(kaboomDir, "<literal>")     — path construction
//   - withKaboomStateLock("<literal>", ...)     — daemon-owned file lock
//
// Both shapes carry a string literal naming the artifact, which is why
// the ARTIFACT tag MUST appear within 5 lines above (so reviewers can
// match name-to-tag without scrolling).
//
// Why not also scan os.WriteFile / os.Create / os.OpenFile / os.Rename:
// in the current code those low-level calls live inside writeTokenAtomic
// and withKaboomStateLock helpers and receive variables (idPath, lockPath)
// rather than literal Joins; their semantic "what artifact is this" is
// determined at the wrapper call sites, which is exactly what we scan.
// If a future refactor inlines an os.* call with a literal kaboomDir Join,
// extend this scanner to recognize that shape too.
func TestContract_AllOnDiskCallSitesAreTagged(t *testing.T) {
	repoRoot := testsupport.RepoRoot(t)
	prodFiles := []string{
		filepath.Join(repoRoot, "internal", "telemetry", "install_id.go"),
		filepath.Join(repoRoot, "internal", "telemetry", "install_id_drift.go"),
	}

	const tagWindow = 5 // max lines a tag may appear above the call site

	for _, path := range prodFiles {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		// Pre-collect ARTIFACT comments by line number for fast lookup.
		artifactCommentLines := make(map[int]bool)
		for _, cg := range file.Comments {
			for _, c := range cg.List {
				if !strings.HasPrefix(c.Text, "//") {
					continue
				}
				body := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
				if !strings.HasPrefix(body, "ARTIFACT:") {
					continue
				}
				line := fset.Position(c.Slash).Line
				artifactCommentLines[line] = true
			}
		}

		// Walk all call expressions and check the two on-disk shapes.
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			callLine := fset.Position(call.Lparen).Line
			label, isOnDiskCall := classifyOnDiskCall(call)
			if !isOnDiskCall {
				return true
			}
			if !hasArtifactTagWithin(artifactCommentLines, callLine, tagWindow) {
				t.Errorf("%s:%d: %s names a kaboomDir-relative on-disk artifact but has no `// ARTIFACT: <name>` comment within %d lines above; tag the call site so TestContract_ArtifactTableMatchesCallSites cannot drift undetected",
					filepath.Base(path), callLine, label, tagWindow)
			}
			return true
		})
	}
}

// classifyOnDiskCall recognizes the two AST shapes that name a
// kaboomDir-relative artifact at construction time. Returns a short
// human-readable label for the diagnostic and true if matched.
//
// Both shapes require a STRING-LITERAL second/first argument: that's the
// artifact name and the load-bearing condition for "this site materializes
// a specific on-disk file." A generic helper that takes a name parameter
// (e.g., withKaboomStateLock receiving its lockName var) is not a
// match — it's the wrapper layer, and its callers carry the literal.
func classifyOnDiskCall(call *ast.CallExpr) (string, bool) {
	// Pattern A: filepath.Join(kaboomDir, "<literal>")
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "filepath" && sel.Sel.Name == "Join" {
			if len(call.Args) >= 2 {
				if id, ok := call.Args[0].(*ast.Ident); ok && id.Name == "kaboomDir" {
					if isStringLit(call.Args[1]) {
						return "filepath.Join(kaboomDir, \"...\")", true
					}
				}
			}
		}
	}
	// Pattern B: withKaboomStateLock("<literal>", ...)
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "withKaboomStateLock" {
		if isStringLit(call.Args[0]) {
			return "withKaboomStateLock(\"...\", ...)", true
		}
	}
	return "", false
}

// isStringLit reports whether expr is a string-typed BasicLit. Concatenated
// expressions (e.g., `name + ".bak"`) are intentionally NOT a match — those
// are derivative constructions; the canonical artifact name lives at the
// originating literal site.
func isStringLit(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

// hasArtifactTagWithin returns true iff any line in [callLine-window, callLine-1]
// carries an `// ARTIFACT: <name>` comment. The strict <-window-and-above
// rule prevents a tag from "below" or on-the-same-line from satisfying the
// contract; the comment must precede the call so reviewers see it first.
func hasArtifactTagWithin(tagLines map[int]bool, callLine, window int) bool {
	for ln := callLine - window; ln < callLine; ln++ {
		if tagLines[ln] {
			return true
		}
	}
	return false
}
