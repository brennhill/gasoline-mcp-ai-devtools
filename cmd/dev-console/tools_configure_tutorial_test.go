// tools_configure_tutorial_test.go — Tests for configure tutorial/examples mode.
package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func seedSyncSettings(t *testing.T, env *configureTestEnv, settingsJSON string) {
	t.Helper()
	reqBody := `{"ext_session_id":"tutorial-test","settings":` + settingsJSON + `}`
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(reqBody))
	httpReq.Header.Set("X-Gasoline-Client", "test-client")
	env.capture.HandleSync(httptest.NewRecorder(), httpReq)
}

func TestToolsConfigureTutorial_ResponseShape(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if data["status"] != "ok" {
		t.Fatalf("status = %v, want ok", data["status"])
	}
	if data["mode"] != "tutorial" {
		t.Fatalf("mode = %v, want tutorial", data["mode"])
	}
	if _, ok := data["snippets"].([]any); !ok {
		t.Fatalf("snippets type = %T, want []any", data["snippets"])
	}
	if _, ok := data["context"].(map[string]any); !ok {
		t.Fatalf("context type = %T, want map[string]any", data["context"])
	}
	if _, ok := data["issues"].([]any); !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
}

func TestToolsConfigureExamples_Alias(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"examples"}`)
	if !ok {
		t.Fatal("examples should return result")
	}
	if result.IsError {
		t.Fatalf("examples should not error, got: %s", result.Content[0].Text)
	}
	data := parseResponseJSON(t, result)
	if data["mode"] != "examples" {
		t.Fatalf("mode = %v, want examples", data["mode"])
	}
}

func TestToolsConfigureExamples_DiffersFromTutorialWithTaskWorkflows(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	tutorialResult, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if tutorialResult.IsError {
		t.Fatalf("tutorial should not error, got: %s", tutorialResult.Content[0].Text)
	}
	tutorialData := parseResponseJSON(t, tutorialResult)

	examplesResult, ok := env.callConfigure(t, `{"what":"examples"}`)
	if !ok {
		t.Fatal("examples should return result")
	}
	if examplesResult.IsError {
		t.Fatalf("examples should not error, got: %s", examplesResult.Content[0].Text)
	}
	examplesData := parseResponseJSON(t, examplesResult)

	if examplesData["message"] == tutorialData["message"] {
		t.Fatal("examples message should be task-oriented and differ from tutorial")
	}

	if _, hasWorkflows := tutorialData["workflows"]; hasWorkflows {
		t.Fatal("tutorial should not include task workflows")
	}
	workflows, ok := examplesData["workflows"].([]any)
	if !ok || len(workflows) == 0 {
		t.Fatalf("examples should include non-empty task workflows, got: %#v", examplesData["workflows"])
	}
}

func TestToolsConfigureTutorial_ContextAware_PilotDisabled(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}
	data := parseResponseJSON(t, result)

	issues, ok := data["issues"].([]any)
	if !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
	if len(issues) == 0 {
		t.Fatal("expected at least one context issue when pilot is disabled")
	}
	first, _ := issues[0].(map[string]any)
	if first["code"] != "pilot_disabled" {
		t.Fatalf("first issue code = %v, want pilot_disabled", first["code"])
	}
}

func TestToolsConfigureTutorial_ContextAware_NoTrackedTab(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)
	env.capture.SetPilotEnabled(true)
	seedSyncSettings(t, env, `{"pilot_enabled":true,"tracking_enabled":false,"tracked_tab_id":0,"tracked_tab_url":"","tracked_tab_title":""}`)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}
	data := parseResponseJSON(t, result)

	issues, ok := data["issues"].([]any)
	if !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
	found := false
	for _, issueRaw := range issues {
		issue, _ := issueRaw.(map[string]any)
		if issue["code"] == "no_tracked_tab" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected no_tracked_tab issue, got: %+v", issues)
	}
}

func TestToolsConfigureHelp_ResponseShape(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"help"}`)
	if !ok {
		t.Fatal("help should return result")
	}
	if result.IsError {
		t.Fatalf("help should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if data["status"] != "ok" {
		t.Fatalf("status = %v, want ok", data["status"])
	}
	if data["mode"] != "help" {
		t.Fatalf("mode = %v, want help", data["mode"])
	}
	tools, ok := data["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools should be non-empty []any, got: %#v", data["tools"])
	}
	text, _ := data["text"].(string)
	if !strings.Contains(text, "analyze - Active analysis") {
		t.Fatalf("help text should include analyze summary, got: %q", text)
	}
}

func TestToolsConfigureCheatsheet_AliasAndDrilldown(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"cheatsheet","tool":"analyze","mode":"accessibility"}`)
	if !ok {
		t.Fatal("cheatsheet should return result")
	}
	if result.IsError {
		t.Fatalf("cheatsheet should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if data["tool"] != "analyze" {
		t.Fatalf("tool = %v, want analyze", data["tool"])
	}
	if data["focus_mode"] != "accessibility" {
		t.Fatalf("focus_mode = %v, want accessibility", data["focus_mode"])
	}
	tools := data["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected exactly one tool in filtered cheatsheet, got %d", len(tools))
	}
	toolMap := tools[0].(map[string]any)
	modes := toolMap["Modes"]
	if modes != nil {
		t.Fatalf("tool payload should use json field names (lowercase), got unexpected key 'Modes': %#v", toolMap)
	}
	modeList := toolMap["modes"].([]any)
	if len(modeList) != 1 {
		t.Fatalf("expected exactly one mode in drilldown response, got %d", len(modeList))
	}
	modeMap := modeList[0].(map[string]any)
	if modeMap["name"] != "accessibility" {
		t.Fatalf("mode.name = %v, want accessibility", modeMap["name"])
	}
}

func TestToolsConfigureHelp_InvalidToolReturnsError(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"help","tool":"not_a_tool"}`)
	if !ok {
		t.Fatal("help should return result")
	}
	if !result.IsError {
		t.Fatalf("help with invalid tool should return error, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Fatalf("expected invalid_param error, got: %s", result.Content[0].Text)
	}
}
