// tools_navigation_summary_limitations_test.go — Regression tests that expose
// known navigation-summary limitations in the current implementation.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNavigationSummaryLimitation_NavigateShouldForwardReason(t *testing.T) {
	t.Parallel()

	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"action":"navigate","url":"https://example.com","reason":"investigate checkout flow","sync":false}`)
	if !ok {
		t.Fatal("navigate should return result")
	}
	if result.IsError {
		t.Fatalf("navigate should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("navigate should queue a browser_action query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse browser_action params: %v", err)
	}

	gotReason, _ := params["reason"].(string)
	if gotReason != "investigate checkout flow" {
		t.Fatalf("navigate should preserve reason in browser_action params; got %q", gotReason)
	}
}

func TestNavigationSummaryLimitation_InteractSchemaShouldExposeSummaryToggle(t *testing.T) {
	t.Parallel()

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

	if _, ok := props["summary"]; !ok {
		t.Fatal("interact schema should expose a boolean 'summary' toggle for navigate/refresh/back/forward")
	}
}

func TestNavigationSummaryLimitation_CompactSummaryShouldUseRealCountsForClassification(t *testing.T) {
	t.Parallel()

	script := compactSummaryScript()
	if strings.Contains(script, "classifyPage(forms, interactiveCount, 0, 0,") {
		t.Fatal("compact summary script passes hard-coded link/paragraph counts into classifyPage; should compute real counts")
	}
}
