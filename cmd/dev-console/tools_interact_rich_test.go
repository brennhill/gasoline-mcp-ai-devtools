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
