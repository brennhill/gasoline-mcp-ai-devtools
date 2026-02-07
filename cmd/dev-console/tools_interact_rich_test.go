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
	json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
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
	json.Unmarshal([]byte(extractJSON(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	// Complete the command
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"refresh"}`), "")

	// Observe — should return without perf_diff (no crash)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	json.Unmarshal([]byte(extractJSON(observeResult.Content[0].Text)), &responseData)

	if _, exists := responseData["perf_diff"]; exists {
		t.Error("perf_diff should NOT be present when no before-snapshot exists")
	}
}
