// recording_handlers_test.go — Coverage tests for recording handler functions.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// Test Infrastructure
// ============================================

type recordingTestEnv struct {
	handler *ToolHandler
	server  *Server
}

func newRecordingTestEnv(t *testing.T) *recordingTestEnv {
	t.Helper()
	env := newConfigureTestEnv(t)
	return &recordingTestEnv{handler: env.handler, server: env.server}
}

func (e *recordingTestEnv) callHandler(t *testing.T, fn func(JSONRPCRequest, json.RawMessage) JSONRPCResponse, argsJSON string) MCPToolResult {
	t.Helper()
	args := json.RawMessage(argsJSON)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := fn(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return result
}

// ============================================
// toolConfigureRecordingStart — 0% → 80%+
// ============================================

func TestRecordingStart_Success(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureRecordingStart, `{"name":"test recording","url":"https://example.com"}`)
	if result.IsError {
		t.Fatalf("recording start should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
	if _, ok := data["recording_id"]; !ok {
		t.Error("response should contain recording_id")
	}
}

func TestRecordingStart_DefaultURL(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureRecordingStart, `{"name":"test"}`)
	if result.IsError {
		t.Fatalf("recording start without url should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if url, _ := data["url"].(string); url != "about:blank" {
		t.Fatalf("url = %q, want about:blank", url)
	}
}

func TestRecordingStart_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureRecordingStart, `{bad json}`)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// toolConfigureRecordingStop — 0% → 80%+
// ============================================

func TestRecordingStop_MissingID(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureRecordingStop, `{}`)
	if !result.IsError {
		t.Fatal("recording stop without id should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "recording_id") {
		t.Errorf("error should mention recording_id, got: %s", text)
	}
}

func TestRecordingStop_InvalidID(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureRecordingStop, `{"recording_id":"nonexistent"}`)
	if !result.IsError {
		t.Fatal("recording stop with invalid id should return error")
	}
}

func TestRecordingStop_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureRecordingStop, `{bad json}`)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

func TestRecordingStop_AfterStart(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	// Start a recording first
	startResult := env.callHandler(t, env.handler.toolConfigureRecordingStart, `{"name":"stop test","url":"https://example.com"}`)
	if startResult.IsError {
		t.Fatalf("start failed: %s", startResult.Content[0].Text)
	}

	startData := parseResponseJSON(t, startResult)
	recordingID, _ := startData["recording_id"].(string)

	// Stop the recording
	stopResult := env.callHandler(t, env.handler.toolConfigureRecordingStop, `{"recording_id":"`+recordingID+`"}`)
	if stopResult.IsError {
		t.Fatalf("stop should not error, got: %s", stopResult.Content[0].Text)
	}

	stopData := parseResponseJSON(t, stopResult)
	if status, _ := stopData["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
}

// ============================================
// toolGetRecordingActions — 0% → 80%+
// ============================================

func TestGetRecordingActions_MissingID(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetRecordingActions, `{}`)
	if !result.IsError {
		t.Fatal("get actions without id should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "recording_id") {
		t.Errorf("error should mention recording_id, got: %s", text)
	}
}

func TestGetRecordingActions_InvalidID(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetRecordingActions, `{"recording_id":"nonexistent"}`)
	if !result.IsError {
		t.Fatal("get actions with invalid id should return error")
	}
}

func TestGetRecordingActions_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetRecordingActions, `{bad}`)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// toolConfigurePlayback — 0% → 80%+
// ============================================

func TestPlayback_MissingID(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigurePlayback, `{}`)
	if !result.IsError {
		t.Fatal("playback without id should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "recording_id") {
		t.Errorf("error should mention recording_id, got: %s", text)
	}
}

func TestPlayback_InvalidID(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigurePlayback, `{"recording_id":"nonexistent"}`)
	if !result.IsError {
		t.Fatal("playback with invalid id should return error")
	}
}

func TestPlayback_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigurePlayback, `{bad}`)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// toolConfigureLogDiff — 0% → 80%+
// ============================================

func TestLogDiff_MissingIDs(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureLogDiff, `{}`)
	if !result.IsError {
		t.Fatal("log_diff without ids should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "original_id") {
		t.Errorf("error should mention original_id, got: %s", text)
	}
}

func TestLogDiff_MissingReplayID(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureLogDiff, `{"original_id":"abc"}`)
	if !result.IsError {
		t.Fatal("log_diff without replay_id should return error")
	}
}

func TestLogDiff_InvalidIDs(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureLogDiff, `{"original_id":"nonexistent","replay_id":"also_nonexistent"}`)
	if !result.IsError {
		t.Fatal("log_diff with invalid ids should return error")
	}
}

func TestLogDiff_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolConfigureLogDiff, `{bad}`)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// toolGetLogDiffReport — 0% → 80%+
// ============================================

func TestLogDiffReport_MissingIDs(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetLogDiffReport, `{}`)
	if !result.IsError {
		t.Fatal("log_diff_report without ids should return error")
	}
}

func TestLogDiffReport_InvalidIDs(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetLogDiffReport, `{"original_id":"nonexistent","replay_id":"also_nonexistent"}`)
	if !result.IsError {
		t.Fatal("log_diff_report with invalid ids should return error")
	}
}

func TestLogDiffReport_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetLogDiffReport, `{bad}`)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// toolGetPlaybackResults — 0% → 100%
// ============================================

func TestGetPlaybackResults_MissingID(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetPlaybackResults, `{}`)
	if !result.IsError {
		t.Fatal("playback_results without id should return error")
	}
}

func TestGetPlaybackResults_Success(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetPlaybackResults, `{"recording_id":"test123"}`)
	if result.IsError {
		t.Fatalf("playback_results should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if rid, _ := data["recording_id"].(string); rid != "test123" {
		t.Fatalf("recording_id = %q, want test123", rid)
	}
}

func TestGetPlaybackResults_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetPlaybackResults, `{bad}`)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// toolGetRecordings — 0% → 80%+
// ============================================

func TestGetRecordings_DefaultLimit(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetRecordings, `{}`)
	if result.IsError {
		t.Fatalf("get recordings should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	// Default limit should be 10
	limit, _ := data["limit"].(float64)
	if limit != 10 {
		t.Fatalf("limit = %v, want 10 (default)", limit)
	}
}

func TestGetRecordings_WithLimit(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetRecordings, `{"limit":5}`)
	if result.IsError {
		t.Fatalf("get recordings with limit should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	limit, _ := data["limit"].(float64)
	if limit != 5 {
		t.Fatalf("limit = %v, want 5", limit)
	}
}

func TestGetRecordings_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newRecordingTestEnv(t)

	result := env.callHandler(t, env.handler.toolGetRecordings, `{bad}`)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}
