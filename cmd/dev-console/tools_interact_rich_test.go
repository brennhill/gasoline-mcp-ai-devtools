// tools_interact_rich_test.go — TDD tests for Rich Action Results.
// Tests the daemon-side contract: analyze param parsing, forwarding to
// pending query, and schema presence.
//
// Run: go test ./cmd/dev-console -run "TestRichAction" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/performance"
)

// ============================================
// analyze param: explicit parsing and forwarding
// ============================================

func TestRichAction_AnalyzeInPendingQueryParams(t *testing.T) {
	env := newInteractTestEnv(t)

	// Enable pilot so the request gets queued (not rejected at pilot check)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn","analyze":true}`)
	if !ok {
		t.Fatal("click with analyze:true should return result")
	}
	if result.IsError {
		t.Fatalf("click with analyze:true + pilot enabled should not error. Got: %s",
			result.Content[0].Text)
	}

	// Extract the queued pending query and verify analyze is in params
	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("Failed to parse pending query params: %v", err)
	}

	analyze, exists := params["analyze"]
	if !exists {
		t.Fatal("analyze field missing from pending query params — it was stripped during forwarding")
	}
	if analyze != true {
		t.Errorf("analyze = %v, want true", analyze)
	}
}

func TestRichAction_AnalyzeFalseNotForwarded(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn","analyze":false}`)
	if !ok {
		t.Fatal("click with analyze:false should return result")
	}
	if result.IsError {
		t.Fatalf("click with analyze:false + pilot enabled should not error. Got: %s",
			result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("Failed to parse pending query params: %v", err)
	}

	// analyze:false should either be absent or false — not true
	if analyze, exists := params["analyze"]; exists && analyze == true {
		t.Error("analyze:false should not be forwarded as true")
	}
}

func TestRichAction_AnalyzeOmittedByDefault(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	if !ok {
		t.Fatal("click without analyze should return result")
	}
	if result.IsError {
		t.Fatalf("click + pilot enabled should not error. Got: %s",
			result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("Failed to parse pending query params: %v", err)
	}

	// analyze should not be present when not specified
	if _, exists := params["analyze"]; exists {
		t.Error("analyze should not be in params when not specified by caller")
	}
}

func TestRichAction_AnalyzeOnNavigationAction(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// refresh doesn't use analyze (it always returns perf_diff)
	// but should not reject it either
	result, ok := env.callInteract(t, `{"action":"refresh","analyze":true}`)
	if !ok {
		t.Fatal("refresh with analyze:true should return result")
	}
	if result.IsError {
		t.Fatalf("refresh with analyze:true + pilot enabled should not error. Got: %s",
			result.Content[0].Text)
	}
}

func TestRichAction_FrameSelectorInPendingQueryParams(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"click","selector":"#submit","frame":"iframe[name='payment']","sync":false}`)
	if !ok {
		t.Fatal("click with frame selector should return result")
	}
	if result.IsError {
		t.Fatalf("click with frame selector should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["frame"].(string); !ok || got != "iframe[name='payment']" {
		t.Fatalf("frame selector not forwarded correctly, got %#v", params["frame"])
	}
}

func TestRichAction_FrameIndexInPendingQueryParams(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"click","selector":"#submit","frame":0,"sync":false}`)
	if !ok {
		t.Fatal("click with frame index should return result")
	}
	if result.IsError {
		t.Fatalf("click with frame index should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["frame"].(float64); !ok || got != 0 {
		t.Fatalf("frame index not forwarded correctly, got %#v", params["frame"])
	}
}

// ============================================
// Schema: analyze param in tools/list
// ============================================

func TestRichAction_SchemaHasAnalyze(t *testing.T) {
	env := newInteractTestEnv(t)

	// Get tools list directly (not via embedded MCPHandler which has no toolHandler)
	tools := env.handler.ToolsList()

	// Find interact tool schema
	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}

	analyzeParam, exists := props["analyze"]
	if !exists {
		t.Fatal("interact schema missing 'analyze' property")
	}

	analyzeMap, ok := analyzeParam.(map[string]any)
	if !ok {
		t.Fatal("analyze property is not an object")
	}

	if analyzeMap["type"] != "boolean" {
		t.Errorf("analyze.type = %v, want 'boolean'", analyzeMap["type"])
	}

	desc, ok := analyzeMap["description"].(string)
	if !ok || desc == "" {
		t.Fatal("analyze must have a description")
	}

	// Description must mention profiling/performance/timing
	lower := strings.ToLower(desc)
	if !strings.Contains(lower, "profil") &&
		!strings.Contains(lower, "performance") &&
		!strings.Contains(lower, "timing") &&
		!strings.Contains(lower, "breakdown") {
		t.Errorf("analyze description must mention profiling/performance/timing. Got: %q", desc)
	}
}

func TestRichAction_SchemaHasFrame(t *testing.T) {
	env := newInteractTestEnv(t)
	tools := env.handler.ToolsList()

	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}

	frameParam, exists := props["frame"]
	if !exists {
		t.Fatal("interact schema missing 'frame' property")
	}
	frameMap, ok := frameParam.(map[string]any)
	if !ok {
		t.Fatal("frame property is not an object")
	}
	if _, ok := frameMap["oneOf"]; !ok {
		t.Fatal("frame property should declare oneOf (string | number)")
	}
}

func TestRichAction_SchemaDescriptionMentionsPerf(t *testing.T) {
	env := newInteractTestEnv(t)

	// Get tools list directly
	tools := env.handler.ToolsList()

	var desc string
	for _, tool := range tools {
		if tool.Name == "interact" {
			desc = tool.Description
			break
		}
	}

	if desc == "" {
		t.Fatal("interact tool not found or has empty description")
	}

	lower := strings.ToLower(desc)
	if !strings.Contains(lower, "perf") &&
		!strings.Contains(lower, "timing") &&
		!strings.Contains(lower, "diff") {
		t.Errorf("interact description should mention perf/timing/diff so AI discovers perf_diff. Got: %q", desc)
	}
}

// ============================================
// Correlation ID: DOM actions with analyze
// ============================================

func TestRichAction_CorrelationID_HasAction(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn","analyze":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed with pilot enabled")
	}

	// Extract correlation_id from result
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Fatal("Result must contain correlation_id")
	}
	if !strings.Contains(text, "dom_click_") {
		t.Errorf("Correlation ID should start with dom_click_. Got: %s", text)
	}
}

// ============================================
// PerfDiff enrichment: refresh command_result
// ============================================

// extractJSON extracts JSON from an mcpJSONResponse text (strips "Summary\n" prefix)
func extractJSON(text string) string {
	if i := strings.Index(text, "\n{"); i >= 0 {
		return text[i+1:]
	}
	if strings.HasPrefix(text, "{") {
		return text
	}
	return text
}

func TestRichAction_RefreshStoresBeforeSnapshot(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com/dashboard")

	// Seed a perf snapshot for the tracked URL's path
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing:    performance.PerformanceTiming{TimeToFirstByte: 120, DomContentLoaded: 800, Load: 1500},
	}})

	// Call refresh — should stash the before-snapshot
	result, ok := env.callInteract(t, `{"action":"refresh"}`)
	if !ok {
		t.Fatal("refresh should return result")
	}
	if result.IsError {
		t.Fatalf("refresh with pilot enabled should not error. Got: %s", result.Content[0].Text)
	}

	// Extract correlation_id from result
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}
	corrID, _ := resultData["correlation_id"].(string)
	if corrID == "" {
		t.Fatal("No correlation_id in refresh result")
	}

	// Verify before-snapshot was stored
	snap, ok := env.capture.GetAndDeleteBeforeSnapshot(corrID)
	if !ok {
		t.Fatal("Before-snapshot should have been stored for refresh correlation_id")
	}
	if snap.URL != "/dashboard" {
		t.Errorf("Before-snapshot URL = %q, want /dashboard", snap.URL)
	}
	if snap.Timing.TimeToFirstByte != 120 {
		t.Errorf("Before-snapshot TTFB = %v, want 120", snap.Timing.TimeToFirstByte)
	}
}

func TestRichAction_CommandResultEnrichedWithPerfDiff(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com/dashboard")

	// Seed "before" snapshot
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing:    performance.PerformanceTiming{TimeToFirstByte: 200, DomContentLoaded: 1000, Load: 2000},
		Network:   performance.NetworkSummary{TransferSize: 500000, RequestCount: 40},
	}})

	// Call refresh to stash the before-snapshot
	result, _ := env.callInteract(t, `{"action":"refresh"}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Simulate extension sending the "after" snapshot (overwrites the old one)
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:05Z",
		Timing:    performance.PerformanceTiming{TimeToFirstByte: 100, DomContentLoaded: 600, Load: 1200},
		Network:   performance.NetworkSummary{TransferSize: 300000, RequestCount: 30},
	}})

	// Simulate extension completing the command
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"refresh"}`), "")

	// Now observe the command_result — should include perf_diff
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	if resp.Result == nil {
		t.Fatal("No result from toolObserveCommandResult")
	}

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse observe result: %v", err)
	}
	if observeResult.IsError {
		t.Fatalf("observe should not error. Got: %s", observeResult.Content[0].Text)
	}

	// Parse the JSON response to check for perf_diff
	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	perfDiff, exists := responseData["perf_diff"]
	if !exists {
		t.Fatal("perf_diff missing from command_result response")
	}

	diffMap, ok := perfDiff.(map[string]any)
	if !ok {
		t.Fatal("perf_diff is not an object")
	}

	// Verify verdict
	verdict, _ := diffMap["verdict"].(string)
	if verdict != "improved" {
		t.Errorf("verdict = %q, want 'improved' (all metrics got better)", verdict)
	}

	// Verify summary exists
	summary, _ := diffMap["summary"].(string)
	if summary == "" {
		t.Error("perf_diff.summary should not be empty")
	}

	// Verify metrics exist
	metrics, _ := diffMap["metrics"].(map[string]any)
	if metrics == nil {
		t.Fatal("perf_diff.metrics missing")
	}
	if _, hasTTFB := metrics["ttfb"]; !hasTTFB {
		t.Error("perf_diff.metrics should include ttfb")
	}
}

func TestRichAction_CommandResultNoPerfDiffWhenNoSnapshots(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	// No tracking status set, no snapshots

	// Call refresh — no before-snapshot available
	result, _ := env.callInteract(t, `{"action":"refresh"}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Complete the command
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"refresh"}`), "")

	// Observe — should return without perf_diff (no crash)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData)

	if _, exists := responseData["perf_diff"]; exists {
		t.Error("perf_diff should NOT be present when no before-snapshot exists")
	}
}

func TestRichAction_CommandResultIncludesTimingMs(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Click action
	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	var resultData map[string]any
	json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Simulate extension completing the click after a small delay
	time.Sleep(10 * time.Millisecond)
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"click"}`), "")

	// Observe command_result
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)
	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData)

	timingMs, exists := responseData["timing_ms"]
	if !exists {
		t.Fatal("timing_ms missing from completed command result")
	}
	tm, ok := timingMs.(float64)
	if !ok {
		t.Fatalf("timing_ms should be a number, got %T", timingMs)
	}
	if tm < 0 {
		t.Errorf("timing_ms should be non-negative, got %v", tm)
	}
}

// ============================================
// Bug reproduction: rAF hang → missing dom_summary
// ============================================

// TestRichAction_CompactClickMissingDomSummary_WhenExtensionHangs demonstrates
// the downstream impact of the rAF hang bug in dom-primitives.ts.
// When the extension's withMutationTracking Promise never resolves, the command
// expires and neither timing_ms nor dom_summary are available.
// This is exactly what smoke test 9.2 observes: "timeout waiting for click".
func TestRichAction_CompactClickMissingDomSummary_WhenExtensionHangs(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a click command
	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed with pilot enabled")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// BUG SCENARIO: Extension never responds because withMutationTracking
	// is stuck waiting for requestAnimationFrame in a backgrounded tab.
	// Simulate this by expiring the command (no CompleteCommand call).
	env.capture.ExpireCommand(corrID)

	// Observe the expired command
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// The command expired — IsError should be true
	if !observeResult.IsError {
		t.Error("Expired command (rAF hang) MUST set IsError=true")
	}

	text := observeResult.Content[0].Text

	// KEY ASSERTION: timing_ms and dom_summary are both missing
	// This is exactly what smoke test 9.2 observes as NO_JSON/timeout
	if strings.Contains(text, "timing_ms") {
		t.Error("Expired command should NOT contain timing_ms — extension never completed")
	}
	if strings.Contains(text, "dom_summary") {
		t.Error("Expired command should NOT contain dom_summary — extension never completed")
	}
}

// TestRichAction_CompactClickHasDomSummary_WhenExtensionResponds verifies that
// when the extension DOES respond with dom_summary (rAF works), it passes through.
// This test passes today — the Go server correctly passes through extension fields.
// It exists to confirm the server isn't the problem.
func TestRichAction_CompactClickHasDomSummary_WhenExtensionResponds(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Click without analyze:true (compact mode)
	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Simulate extension responding WITH dom_summary (compact mode, no analyze)
	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"selector": "#btn",
		"dom_summary": "1 added"
	}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	// Observe command_result
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// timing_ms should be at the top level (Go server adds this)
	if _, exists := responseData["timing_ms"]; !exists {
		t.Error("timing_ms should be present at top level (added by Go server)")
	}

	// dom_summary should be inside result (extension provides this)
	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("response should contain 'result' object")
	}

	domSummary, exists := extResult["dom_summary"]
	if !exists {
		t.Fatal("dom_summary should pass through from extension to command result")
	}
	if domSummary != "1 added" {
		t.Errorf("dom_summary = %q, want '1 added'", domSummary)
	}
}

// ============================================
// Top-level surfacing: timing, dom_changes, analysis
// ============================================

func TestRichAction_AnalyzeFieldsSurfacedTopLevel(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Click with analyze:true
	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn","analyze":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Simulate extension result with analyze enrichment fields
	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"timing": {"total_ms": 55, "js_blocking_ms": 12, "render_ms": 8},
		"dom_changes": {"added": 3, "removed": 0, "modified": 1, "summary": "3 added, 1 modified"},
		"analysis": "click completed in 55ms. 3 added, 1 modified."
	}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	// Observe command_result
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// timing must be surfaced at top level
	timing, ok := responseData["timing"].(map[string]any)
	if !ok {
		t.Fatal("timing must be surfaced at top level of command_result response")
	}
	if totalMs, _ := timing["total_ms"].(float64); totalMs != 55 {
		t.Errorf("timing.total_ms = %v, want 55", totalMs)
	}

	// dom_changes must be surfaced at top level
	domChanges, ok := responseData["dom_changes"].(map[string]any)
	if !ok {
		t.Fatal("dom_changes must be surfaced at top level of command_result response")
	}
	if domChanges["summary"] != "3 added, 1 modified" {
		t.Errorf("dom_changes.summary = %v, want '3 added, 1 modified'", domChanges["summary"])
	}

	// analysis must be surfaced at top level
	analysis, ok := responseData["analysis"].(string)
	if !ok || analysis == "" {
		t.Fatal("analysis must be surfaced at top level of command_result response")
	}
	if !strings.Contains(analysis, "55ms") {
		t.Errorf("analysis = %q, should mention 55ms", analysis)
	}

	// Fields must also still be in result (passthrough preserved)
	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("result envelope must still exist")
	}
	if _, exists := extResult["timing"]; !exists {
		t.Error("timing must also remain inside result (passthrough)")
	}
}

func TestRichAction_NoAnalyzeFieldsWhenAbsent(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Extension result WITHOUT analyze fields (compact mode)
	extensionResult := json.RawMessage(`{"success": true, "action": "click", "dom_summary": "1 added"}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// None of the analyze fields should be at top level
	if _, exists := responseData["timing"]; exists {
		t.Error("timing should NOT be at top level when extension doesn't provide it")
	}
	if _, exists := responseData["dom_changes"]; exists {
		t.Error("dom_changes should NOT be at top level when extension doesn't provide it")
	}
	if _, exists := responseData["analysis"]; exists {
		t.Error("analysis should NOT be at top level when extension doesn't provide it")
	}
}

func TestRichAction_TargetContextSurfacedTopLevel(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn","tab_id":77}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"resolved_tab_id": 77,
		"resolved_url": "https://example.com/form",
		"target_context": {
			"source": "explicit_tab",
			"requested_tab_id": 77,
			"tracked_tab_id": null,
			"use_active_tab": false
		}
	}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if responseData["resolved_tab_id"] != float64(77) {
		t.Fatalf("resolved_tab_id = %v, want 77", responseData["resolved_tab_id"])
	}
	if responseData["resolved_url"] != "https://example.com/form" {
		t.Fatalf("resolved_url = %v, want https://example.com/form", responseData["resolved_url"])
	}

	targetContext, ok := responseData["target_context"].(map[string]any)
	if !ok {
		t.Fatal("target_context missing at top level")
	}
	if targetContext["source"] != "explicit_tab" {
		t.Fatalf("target_context.source = %v, want explicit_tab", targetContext["source"])
	}

	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("result envelope missing")
	}
	if extResult["resolved_tab_id"] != float64(77) {
		t.Fatalf("result.resolved_tab_id = %v, want 77", extResult["resolved_tab_id"])
	}
}

// ============================================
// Command result passthrough: dom_summary, analyze fields
// ============================================

func TestRichAction_DomSummaryPassthrough(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Click with analyze:true
	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn","analyze":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Simulate extension result with dom_summary and analyze fields
	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"dom_summary": "2 added, 1 modified",
		"timing": {"total_ms": 42},
		"dom_changes": {"added": 2, "removed": 0, "modified": 1, "summary": "2 added, 1 modified"},
		"analysis": "click completed in 42ms. 2 added, 1 modified."
	}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	// Observe command_result — verify extension fields pass through
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Extension result fields should be inside the "result" envelope
	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("response should contain 'result' object with extension data")
	}

	domSummary, exists := extResult["dom_summary"]
	if !exists {
		t.Fatal("dom_summary missing from command result — extension field not passed through")
	}
	if domSummary != "2 added, 1 modified" {
		t.Errorf("dom_summary = %q, want '2 added, 1 modified'", domSummary)
	}

	analysis, exists := extResult["analysis"]
	if !exists {
		t.Fatal("analysis missing from command result — extension field not passed through")
	}
	if analysis != "click completed in 42ms. 2 added, 1 modified." {
		t.Errorf("analysis = %q, want 'click completed in 42ms. 2 added, 1 modified.'", analysis)
	}

	timing, exists := extResult["timing"].(map[string]any)
	if !exists {
		t.Fatal("timing missing from command result")
	}
	if totalMs, _ := timing["total_ms"].(float64); totalMs != 42 {
		t.Errorf("timing.total_ms = %v, want 42", totalMs)
	}
}

func TestRichAction_PerfDiffWithFullWebVitals(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com/dashboard")

	fcp := 3500.0
	lcp := 4500.0
	cls := 0.3

	// Seed before snapshot with ALL Web Vitals populated
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: performance.PerformanceTiming{
			TimeToFirstByte:        900,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			DomContentLoaded:       1200,
			Load:                   2500,
		},
		CLS:     &cls,
		Network: performance.NetworkSummary{TransferSize: 800000, RequestCount: 60},
	}})

	// Call refresh to stash before-snapshot
	result, _ := env.callInteract(t, `{"action":"refresh"}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Simulate improved "after" snapshot with all Web Vitals
	fcp2 := 800.0
	lcp2 := 1200.0
	cls2 := 0.02
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:05Z",
		Timing: performance.PerformanceTiming{
			TimeToFirstByte:        150,
			FirstContentfulPaint:   &fcp2,
			LargestContentfulPaint: &lcp2,
			DomContentLoaded:       500,
			Load:                   1000,
		},
		CLS:     &cls2,
		Network: performance.NetworkSummary{TransferSize: 300000, RequestCount: 25},
	}})

	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"refresh"}`), "")

	// Observe command_result
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData)

	perfDiff, exists := responseData["perf_diff"]
	if !exists {
		t.Fatal("perf_diff missing from command_result with full Web Vitals")
	}
	diffMap := perfDiff.(map[string]any)
	metrics := diffMap["metrics"].(map[string]any)

	// FCP must be present with "good" rating
	fcpMetric, hasFCP := metrics["fcp"]
	if !hasFCP {
		t.Fatal("perf_diff.metrics missing 'fcp' — FCP not wired through snapshot→diff pipeline")
	}
	fcpMap := fcpMetric.(map[string]any)
	if fcpMap["rating"] != "good" {
		t.Errorf("FCP 800ms rating = %q, want 'good'", fcpMap["rating"])
	}

	// LCP must be present with "good" rating
	lcpMetric, hasLCP := metrics["lcp"]
	if !hasLCP {
		t.Fatal("perf_diff.metrics missing 'lcp'")
	}
	lcpMap := lcpMetric.(map[string]any)
	if lcpMap["rating"] != "good" {
		t.Errorf("LCP 1200ms rating = %q, want 'good'", lcpMap["rating"])
	}

	// CLS must be present with "good" rating
	clsMetric, hasCLS := metrics["cls"]
	if !hasCLS {
		t.Fatal("perf_diff.metrics missing 'cls' — CLS not wired through snapshot→diff pipeline")
	}
	clsMap := clsMetric.(map[string]any)
	if clsMap["rating"] != "good" {
		t.Errorf("CLS 0.02 rating = %q, want 'good'", clsMap["rating"])
	}

	// TTFB must be present with "good" rating
	ttfbMetric, hasTTFB := metrics["ttfb"]
	if !hasTTFB {
		t.Fatal("perf_diff.metrics missing 'ttfb'")
	}
	ttfbMap := ttfbMetric.(map[string]any)
	if ttfbMap["rating"] != "good" {
		t.Errorf("TTFB 150ms rating = %q, want 'good'", ttfbMap["rating"])
	}

	// Verdict must be "improved"
	if diffMap["verdict"] != "improved" {
		t.Errorf("verdict = %q, want 'improved'", diffMap["verdict"])
	}
}

// ============================================
// Failed command visibility: IsError signaling
// ============================================

func TestCommandResult_ExpiredSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate expiry — extension never responded
	env.capture.ExpireCommand(corrID)

	// Observe the expired command
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Error("Expired command MUST set IsError=true so LLMs recognize failure")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "extension_timeout") {
		t.Errorf("Expired command should include error code 'extension_timeout', got: %s", text)
	}
	if !strings.Contains(text, "retry") {
		t.Errorf("Expired command should include retry instructions, got: %s", text)
	}
}

func TestCommandResult_CompleteWithErrorSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate extension completing with an error
	env.capture.CompleteCommand(corrID, json.RawMessage(`null`), "Element not found: #btn")

	// Observe the failed command
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Error("Command completed with error MUST set IsError=true so LLMs recognize failure")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "FAILED") {
		t.Errorf("Failed command summary should include 'FAILED', got: %s", text)
	}
	if !strings.Contains(text, "Element not found") {
		t.Errorf("Failed command should include error message, got: %s", text)
	}
}

func TestCommandResult_EmbeddedFailureSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Extension reported failure inside the result payload without setting command error.
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":false,"error":"selector_not_found","message":"#btn not found"}`), "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Fatal("Embedded success=false MUST set IsError=true")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "FAILED") {
		t.Fatalf("Expected FAILED summary, got: %s", text)
	}
	if !strings.Contains(text, "selector_not_found") {
		t.Fatalf("Expected embedded error to surface, got: %s", text)
	}
}

func TestCommandResult_SuccessDoesNotSetIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate successful completion
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true}`), "")

	// Observe the successful command
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if observeResult.IsError {
		t.Error("Successful command should NOT set IsError")
	}

	text := observeResult.Content[0].Text
	if strings.Contains(text, "FAILED") {
		t.Errorf("Successful command should not contain 'FAILED', got: %s", text)
	}
}

// ============================================
// Issue #92: queued/final markers on async responses
// ============================================

func TestQueuedResponse_HasQueuedAndFinalMarkers(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn","background":true}`)
	var responseData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &responseData)

	if responseData["status"] != "queued" {
		t.Fatalf("status = %v, want queued", responseData["status"])
	}
	if queued, _ := responseData["queued"].(bool); !queued {
		t.Fatalf("queued response should have queued=true, got %v", responseData["queued"])
	}
	if final, _ := responseData["final"].(bool); final {
		t.Fatalf("queued response should have final=false, got %v", responseData["final"])
	}
}

func TestCommandResult_CompleteHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue async to avoid sync-wait-for-extension
	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true}`), "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("complete command should have final=true, got %v", responseData["final"])
	}
}

func TestCommandResult_ErrorHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	env.capture.CompleteCommand(corrID, nil, "element_not_found")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("error command should have final=true, got %v", responseData["final"])
	}
}

// ============================================
// Issue #91: effective_tab_id and effective_url surfaced
// ============================================

func TestCommandResult_EffectiveContextSurfaced(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"action":"click","selector":"#btn","tab_id":42,"background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"resolved_tab_id": 42,
		"resolved_url": "https://example.com/page1",
		"effective_tab_id": 42,
		"effective_url": "https://example.com/page2"
	}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if responseData["effective_tab_id"] != float64(42) {
		t.Fatalf("effective_tab_id = %v, want 42", responseData["effective_tab_id"])
	}
	if responseData["effective_url"] != "https://example.com/page2" {
		t.Fatalf("effective_url = %v, want https://example.com/page2", responseData["effective_url"])
	}
	if responseData["resolved_url"] != "https://example.com/page1" {
		t.Fatalf("resolved_url = %v, want https://example.com/page1", responseData["resolved_url"])
	}
}

func TestCommandResult_ExpiredIncludesDiagnosticHint(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID
	env.capture.ExpireCommand(corrID)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	text := observeResult.Content[0].Text

	// Diagnostic hint must include pilot and tracking state
	if !strings.Contains(text, "pilot=") {
		t.Errorf("Error response must include pilot status in diagnostic hint, got: %s", text)
	}
	if !strings.Contains(text, "tracked_tab=") {
		t.Errorf("Error response must include tracking status in diagnostic hint, got: %s", text)
	}
}

// ============================================
// Subtitle: correlation_id contract
// ============================================
// handleSubtitle() creates a PendingQuery with a correlationID but never
// returns it in the MCP response. This makes it impossible for callers to
// poll observe(command_result) for completion, causing race conditions in
// smoke tests (test 11.1: "Subtitle still visible after clear").

func TestSubtitle_SetResponse_HasCorrelationID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"subtitle","text":"hello world"}`)
	if !ok {
		t.Fatal("subtitle set should return a result")
	}
	if result.IsError {
		t.Fatal("subtitle set should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("Subtitle SET response must contain correlation_id so callers can poll for completion.\n"+
			"Without it, callers cannot wait for the extension to process the command.\n"+
			"Every other async handler (click, navigate, execute_js, highlight) returns correlation_id.\n"+
			"Got: %s", text)
	}
	if !strings.Contains(text, "subtitle_") {
		t.Errorf("Subtitle correlation_id should have subtitle_ prefix. Got: %s", text)
	}
}

func TestSubtitle_ClearResponse_HasCorrelationID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"subtitle","text":""}`)
	if !ok {
		t.Fatal("subtitle clear should return a result")
	}
	if result.IsError {
		t.Fatal("subtitle clear should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("Subtitle CLEAR response must contain correlation_id so callers can poll for completion.\n"+
			"Without it, the smoke test's interact_and_wait returns immediately and checks DOM\n"+
			"before the extension has processed the clear — causing 'still visible after clear'.\n"+
			"Got: %s", text)
	}
}

func TestSubtitle_CorrelationID_MatchesPendingQuery(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"action":"subtitle","text":"test"}`)
	if !ok || result.IsError {
		t.Fatal("subtitle should succeed")
	}

	// The PendingQuery IS created with a correlationID
	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query created for subtitle")
	}
	if pq.CorrelationID == "" {
		t.Fatal("PendingQuery has empty correlationID")
	}

	// But the MCP response must also contain it
	text := result.Content[0].Text
	if !strings.Contains(text, pq.CorrelationID) {
		t.Errorf("MCP response must contain the same correlation_id as the PendingQuery.\n"+
			"PendingQuery has: %s\n"+
			"Response text: %s", pq.CorrelationID, text)
	}
}

func TestCommandResult_PilotDisabledIncludesDiagnosticHint(t *testing.T) {
	env := newInteractTestEnv(t)
	// Pilot is disabled by default in test env

	result, ok := env.callInteract(t, `{"action":"click","selector":"#btn"}`)
	if !ok {
		t.Fatal("should return result")
	}
	if !result.IsError {
		t.Fatal("pilot disabled should return error")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "pilot=DISABLED") {
		t.Errorf("pilot_disabled error must include 'pilot=DISABLED' in hint, got: %s", text)
	}
	if !strings.Contains(text, "tracked_tab=") {
		t.Errorf("pilot_disabled error must include tracking status in hint, got: %s", text)
	}
}
