// tools_configure_sequence_test.go — Tests for macro sequence CRUD and replay.
package main

import (
	"encoding/json"
	"testing"

	"github.com/dev-console/dev-console/internal/ai"
)

// newSequenceTestEnv creates a test env with an isolated session store
// so parallel tests don't interfere with each other's sequences.
func newSequenceTestEnv(t *testing.T) *toolTestEnv {
	t.Helper()
	env := newToolTestEnv(t)
	// Replace session store with one backed by t.TempDir for isolation
	store, err := ai.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create isolated session store: %v", err)
	}
	t.Cleanup(func() { store.Shutdown() })
	env.handler.sessionStoreImpl = store
	return env
}

// ============================================
// Save Sequence Tests
// ============================================

func TestSaveSequence_Valid(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "login-flow",
		"description": "Login to the app",
		"steps": [
			{"action": "navigate", "url": "https://example.com/login"},
			{"action": "click", "selector": "#submit"}
		]
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "save_sequence", result)
	data := extractResultJSON(t, result)
	if data["status"] != "saved" {
		t.Errorf("expected status=saved, got %v", data["status"])
	}
	if data["name"] != "login-flow" {
		t.Errorf("expected name=login-flow, got %v", data["name"])
	}
	stepCount, _ := data["step_count"].(float64)
	if stepCount != 2 {
		t.Errorf("expected step_count=2, got %v", stepCount)
	}
}

func TestSaveSequence_MissingName(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"steps": [{"action": "click", "selector": "#btn"}]
	}`)
	assertIsError(t, resp, "missing_param")
}

func TestSaveSequence_InvalidNameFormat(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "invalid name with spaces!",
		"steps": [{"action": "click", "selector": "#btn"}]
	}`)
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_EmptySteps(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "empty-seq",
		"steps": []
	}`)
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_TooManySteps(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	steps := make([]map[string]any, 51)
	for i := range steps {
		steps[i] = map[string]any{"action": "click", "selector": "#btn"}
	}
	argsMap := map[string]any{
		"action": "save_sequence",
		"name":   "big-seq",
		"steps":  steps,
	}
	argsJSON, _ := json.Marshal(argsMap)
	resp := callConfigureRaw(env.handler, string(argsJSON))
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_StepMissingAction(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "bad-step",
		"steps": [{"selector": "#btn"}]
	}`)
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_Upsert(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	// Save initial
	callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "my-seq",
		"steps": [{"action": "click", "selector": "#a"}]
	}`)
	// Overwrite
	resp := callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "my-seq",
		"steps": [
			{"action": "click", "selector": "#b"},
			{"action": "click", "selector": "#c"}
		]
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "save_sequence upsert", result)
	data := extractResultJSON(t, result)
	stepCount, _ := data["step_count"].(float64)
	if stepCount != 2 {
		t.Errorf("expected upserted step_count=2, got %v", stepCount)
	}
}

func TestSaveSequence_WithTags(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "tagged-seq",
		"tags": ["auth", "setup"],
		"steps": [{"action": "navigate", "url": "https://example.com"}]
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "save_sequence with tags", result)
}

// ============================================
// Get Sequence Tests
// ============================================

func TestGetSequence_Valid(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	// Save
	callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "get-test",
		"description": "A test sequence",
		"steps": [{"action": "navigate", "url": "https://example.com"}]
	}`)
	// Get
	resp := callConfigureRaw(env.handler, `{"action": "get_sequence", "name": "get-test"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "get_sequence", result)
	data := extractResultJSON(t, result)
	if data["name"] != "get-test" {
		t.Errorf("expected name=get-test, got %v", data["name"])
	}
	if data["description"] != "A test sequence" {
		t.Errorf("expected description, got %v", data["description"])
	}
	steps, ok := data["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Errorf("expected 1 step, got %v", data["steps"])
	}
}

func TestGetSequence_NotFound(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"action": "get_sequence", "name": "nonexistent"}`)
	assertIsError(t, resp, "no_data")
}

func TestGetSequence_MissingName(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"action": "get_sequence"}`)
	assertIsError(t, resp, "missing_param")
}

// ============================================
// List Sequences Tests
// ============================================

func TestListSequences_Empty(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"action": "list_sequences"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "list_sequences empty", result)
	data := extractResultJSON(t, result)
	sequences, ok := data["sequences"].([]any)
	if !ok {
		t.Fatalf("expected sequences array, got %T", data["sequences"])
	}
	if len(sequences) != 0 {
		t.Errorf("expected 0 sequences, got %d", len(sequences))
	}
}

func TestListSequences_Multiple(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	for _, name := range []string{"seq-a", "seq-b"} {
		argsMap := map[string]any{
			"action": "save_sequence",
			"name":   name,
			"steps":  []any{map[string]any{"action": "click", "selector": "#btn"}},
		}
		argsJSON, _ := json.Marshal(argsMap)
		callConfigureRaw(env.handler, string(argsJSON))
	}
	resp := callConfigureRaw(env.handler, `{"action": "list_sequences"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "list_sequences multiple", result)
	data := extractResultJSON(t, result)
	sequences, ok := data["sequences"].([]any)
	if !ok {
		t.Fatalf("expected sequences array, got %T", data["sequences"])
	}
	if len(sequences) != 2 {
		t.Errorf("expected 2 sequences, got %d", len(sequences))
	}
}

func TestListSequences_FilterByTags(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "auth-seq",
		"tags": ["auth"],
		"steps": [{"action": "click", "selector": "#btn"}]
	}`)
	callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "other-seq",
		"tags": ["other"],
		"steps": [{"action": "click", "selector": "#btn"}]
	}`)
	resp := callConfigureRaw(env.handler, `{"action": "list_sequences", "tags": ["auth"]}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "list_sequences filter", result)
	data := extractResultJSON(t, result)
	sequences, ok := data["sequences"].([]any)
	if !ok {
		t.Fatalf("expected sequences array, got %T", data["sequences"])
	}
	if len(sequences) != 1 {
		t.Errorf("expected 1 tagged sequence, got %d", len(sequences))
	}
}

// ============================================
// Delete Sequence Tests
// ============================================

func TestDeleteSequence_Valid(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "to-delete",
		"steps": [{"action": "click", "selector": "#btn"}]
	}`)
	resp := callConfigureRaw(env.handler, `{"action": "delete_sequence", "name": "to-delete"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "delete_sequence", result)
	data := extractResultJSON(t, result)
	if data["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %v", data["status"])
	}
	// Verify it's gone
	getResp := callConfigureRaw(env.handler, `{"action": "get_sequence", "name": "to-delete"}`)
	assertIsError(t, getResp, "no_data")
}

func TestDeleteSequence_NotFound(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"action": "delete_sequence", "name": "nonexistent"}`)
	assertIsError(t, resp, "no_data")
}

func TestDeleteSequence_MissingName(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"action": "delete_sequence"}`)
	assertIsError(t, resp, "missing_param")
}

// ============================================
// Replay Sequence Tests (unit-level, no live browser)
// ============================================

func TestReplaySequence_NotFound(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"action": "replay_sequence", "name": "nonexistent"}`)
	assertIsError(t, resp, "no_data")
}

func TestReplaySequence_MissingName(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"action": "replay_sequence"}`)
	assertIsError(t, resp, "missing_param")
}

func TestReplaySequence_OverrideStepsLengthMismatch(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "two-step",
		"steps": [
			{"action": "click", "selector": "#a"},
			{"action": "click", "selector": "#b"}
		]
	}`)
	resp := callConfigureRaw(env.handler, `{
		"action": "replay_sequence",
		"name": "two-step",
		"override_steps": [null, null, null]
	}`)
	assertIsError(t, resp, "invalid_param")
}

// ============================================
// Persistence Tests
// ============================================

func TestSequence_PersistsAcrossHandlerInstances(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	// Save a sequence
	callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "persist-test",
		"steps": [{"action": "navigate", "url": "https://example.com"}]
	}`)
	// Get with the same handler (verifies session store persistence)
	resp := callConfigureRaw(env.handler, `{"action": "get_sequence", "name": "persist-test"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "persistence", result)
	data := extractResultJSON(t, result)
	if data["name"] != "persist-test" {
		t.Errorf("expected name=persist-test, got %v", data["name"])
	}
}

// ============================================
// Name Validation Edge Cases
// ============================================

func TestSaveSequence_NameTooLong(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	longName := ""
	for i := 0; i < 65; i++ {
		longName += "a"
	}
	argsMap := map[string]any{
		"action": "save_sequence",
		"name":   longName,
		"steps":  []any{map[string]any{"action": "click", "selector": "#btn"}},
	}
	argsJSON, _ := json.Marshal(argsMap)
	resp := callConfigureRaw(env.handler, string(argsJSON))
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_ValidNameChars(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"action": "save_sequence",
		"name": "valid-name_123",
		"steps": [{"action": "click", "selector": "#btn"}]
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "valid name chars", result)
}
