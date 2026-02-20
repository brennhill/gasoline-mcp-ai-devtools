// rewrite_test.go — Tests for argument rewriting functions.
package configure

import (
	"encoding/json"
	"testing"
)

// helper to parse rewritten JSON and extract a field.
func extractField(t *testing.T, data json.RawMessage, field string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to parse rewritten JSON: %v", err)
	}
	v, _ := m[field].(string)
	return v
}

// ============================================
// RewriteNoiseRuleArgs tests
// ============================================

func TestRewriteNoiseRuleArgs_SetsAction(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteNoiseRuleArgs(json.RawMessage(`{"noise_action":"add","pattern":"test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "add" {
		t.Errorf("action = %q, want %q", got, "add")
	}
}

func TestRewriteNoiseRuleArgs_DefaultsToList(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteNoiseRuleArgs(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "list" {
		t.Errorf("action = %q, want %q", got, "list")
	}
}

func TestRewriteNoiseRuleArgs_NilArgs(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteNoiseRuleArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "list" {
		t.Errorf("action = %q, want %q", got, "list")
	}
}

func TestRewriteNoiseRuleArgs_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := RewriteNoiseRuleArgs(json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRewriteNoiseRuleArgs_FlattensSingleRule_DefaultCategory(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteNoiseRuleArgs(json.RawMessage(`{"noise_action":"add","pattern":"test.*noise"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(rewritten, &raw); err != nil {
		t.Fatalf("unmarshal rewritten: %v", err)
	}
	if raw["action"] != "add" {
		t.Fatalf("action = %v, want add", raw["action"])
	}

	rules, ok := raw["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("rules = %v, want single rule", raw["rules"])
	}
	rule, ok := rules[0].(map[string]any)
	if !ok {
		t.Fatalf("rules[0] type = %T, want map[string]any", rules[0])
	}
	if rule["category"] != "console" {
		t.Fatalf("category = %v, want console", rule["category"])
	}
	matchSpec, ok := rule["match_spec"].(map[string]any)
	if !ok {
		t.Fatalf("match_spec type = %T, want map[string]any", rule["match_spec"])
	}
	if matchSpec["message_regex"] != "test.*noise" {
		t.Fatalf("message_regex = %v, want test.*noise", matchSpec["message_regex"])
	}
}

func TestRewriteNoiseRuleArgs_FlattenPreservesExplicitCategory(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteNoiseRuleArgs(json.RawMessage(`{"noise_action":"add","pattern":"api-noise","category":"network"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(rewritten, &raw); err != nil {
		t.Fatalf("unmarshal rewritten: %v", err)
	}
	rules, ok := raw["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("rules = %v, want single rule", raw["rules"])
	}
	rule, _ := rules[0].(map[string]any)
	if rule["category"] != "network" {
		t.Fatalf("category = %v, want network", rule["category"])
	}
}

// ============================================
// RewriteStreamingArgs tests
// ============================================

func TestRewriteStreamingArgs_SetsAction(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteStreamingArgs(json.RawMessage(`{"streaming_action":"enable"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "enable" {
		t.Errorf("action = %q, want %q", got, "enable")
	}
}

func TestRewriteStreamingArgs_NoStreamingAction(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteStreamingArgs(json.RawMessage(`{"other":"value"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// action should not be set if streaming_action wasn't provided
	if got := extractField(t, rewritten, "action"); got != "" {
		t.Errorf("action = %q, want empty", got)
	}
}

func TestRewriteStreamingArgs_NilArgs(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteStreamingArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rewritten == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRewriteStreamingArgs_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := RewriteStreamingArgs(json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ============================================
// RewriteDiffSessionsArgs tests
// ============================================

func TestRewriteDiffSessionsArgs_SetsAction(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteDiffSessionsArgs(json.RawMessage(`{"verif_session_action":"compare"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "compare" {
		t.Errorf("action = %q, want %q", got, "compare")
	}
}

func TestRewriteDiffSessionsArgs_DefaultsToList(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteDiffSessionsArgs(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "list" {
		t.Errorf("action = %q, want %q", got, "list")
	}
}

func TestRewriteDiffSessionsArgs_DiffSessionsDefaultsToList(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteDiffSessionsArgs(json.RawMessage(`{"action":"diff_sessions"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "list" {
		t.Errorf("action = %q, want %q", got, "list")
	}
}

func TestRewriteDiffSessionsArgs_EmptyVerifSessionAction(t *testing.T) {
	t.Parallel()

	// Empty string verif_session_action should not override action
	rewritten, err := RewriteDiffSessionsArgs(json.RawMessage(`{"verif_session_action":"","action":"capture"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "capture" {
		t.Errorf("action = %q, want %q", got, "capture")
	}
}

func TestRewriteDiffSessionsArgs_NilArgs(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteDiffSessionsArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := extractField(t, rewritten, "action"); got != "list" {
		t.Errorf("action = %q, want %q", got, "list")
	}
}

func TestRewriteDiffSessionsArgs_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := RewriteDiffSessionsArgs(json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
