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

	// Get tools list
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}
	resp := env.handler.handleToolsList(req)

	if resp.Result == nil {
		t.Fatal("tools/list returned nil result")
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	// Find interact tool schema
	var result struct {
		Tools []struct {
			Name        string         `json:"name"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	var interactSchema map[string]any
	for _, tool := range result.Tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in tools/list")
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

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}
	resp := env.handler.handleToolsList(req)

	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var result struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	var desc string
	for _, tool := range result.Tools {
		if tool.Name == "interact" {
			desc = tool.Description
			break
		}
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
