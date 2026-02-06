// tools_observe_contract_test.go — Response shape contracts for observe tool.
// Each test verifies that an observe mode returns the correct JSON fields with
// correct types. Catches field renames, missing fields, and type changes.
//
// Run: go test ./cmd/dev-console -run "TestObserveContract" -v
package main

import (
	"testing"
)

// ============================================
// Tier 1: Core Data Flow Contracts
// ============================================

func TestObserveContract_Errors(t *testing.T) {
	s := newScenario(t)
	s.loadConsoleData(t)

	result, ok := s.callObserve(t, "errors")
	if !ok {
		t.Fatal("observe errors: no result")
	}

	assertResponseShape(t, "errors", result, []fieldSpec{
		required("errors", "array"),
		required("count", "number"),
	})

	// Nested: verify error entry shape
	data := parseResponseJSON(t, result)
	errors := data["errors"].([]any)
	if len(errors) == 0 {
		t.Fatal("errors: expected at least 1 error entry")
	}
	assertObjectShape(t, "errors[0]", errors[0].(map[string]any), []fieldSpec{
		required("message", "string"),
		optional("source", "string"),
		optional("url", "string"),
		optional("line", "number"),
		optional("column", "number"),
		optional("stack", "string"),
		optional("timestamp", "string"),
	})
}

func TestObserveContract_Logs(t *testing.T) {
	s := newScenario(t)
	s.loadConsoleData(t)

	result, ok := s.callObserve(t, "logs")
	if !ok {
		t.Fatal("observe logs: no result")
	}

	assertResponseShape(t, "logs", result, []fieldSpec{
		required("logs", "array"),
		required("count", "number"),
	})

	data := parseResponseJSON(t, result)
	logs := data["logs"].([]any)
	if len(logs) == 0 {
		t.Fatal("logs: expected at least 1 log entry")
	}
	assertObjectShape(t, "logs[0]", logs[0].(map[string]any), []fieldSpec{
		required("level", "string"),
		required("message", "string"),
		optional("source", "string"),
		optional("url", "string"),
		optional("line", "number"),
		optional("column", "number"),
		optional("timestamp", "string"),
	})
}

func TestObserveContract_ExtensionLogs(t *testing.T) {
	s := newScenario(t)
	s.loadExtensionLogs(t)

	result, ok := s.callObserve(t, "extension_logs")
	if !ok {
		t.Fatal("observe extension_logs: no result")
	}

	assertResponseShape(t, "extension_logs", result, []fieldSpec{
		required("logs", "array"),
		required("count", "number"),
	})

	data := parseResponseJSON(t, result)
	logs := data["logs"].([]any)
	if len(logs) == 0 {
		t.Fatal("extension_logs: expected at least 1 entry")
	}
	assertObjectShape(t, "extension_logs[0]", logs[0].(map[string]any), []fieldSpec{
		required("level", "string"),
		required("message", "string"),
		optional("source", "string"),
		optional("category", "string"),
		optional("timestamp", "string"),
	})
}

func TestObserveContract_NetworkWaterfall(t *testing.T) {
	s := newScenario(t)
	s.loadNetworkData(t)

	result, ok := s.callObserve(t, "network_waterfall")
	if !ok {
		t.Fatal("observe network_waterfall: no result")
	}

	assertResponseShape(t, "network_waterfall", result, []fieldSpec{
		required("entries", "array"),
		required("count", "number"),
	})

	data := parseResponseJSON(t, result)
	entries := data["entries"].([]any)
	if len(entries) == 0 {
		t.Fatal("network_waterfall: expected at least 1 entry")
	}
	assertObjectShape(t, "network_waterfall[0]", entries[0].(map[string]any), []fieldSpec{
		required("url", "string"),
		required("initiator_type", "string"),
		required("duration_ms", "number"),
		required("start_time", "number"),
		required("transfer_size", "number"),
		optional("decoded_body_size", "number"),
		optional("encoded_body_size", "number"),
		optional("page_url", "string"),
	})
}

func TestObserveContract_NetworkBodies(t *testing.T) {
	s := newScenario(t)
	s.loadNetworkData(t)

	result, ok := s.callObserve(t, "network_bodies")
	if !ok {
		t.Fatal("observe network_bodies: no result")
	}

	assertResponseShape(t, "network_bodies", result, []fieldSpec{
		required("entries", "array"),
	})
}

func TestObserveContract_WebSocketEvents(t *testing.T) {
	s := newScenario(t)
	s.loadWebSocketData(t)

	result, ok := s.callObserve(t, "websocket_events")
	if !ok {
		t.Fatal("observe websocket_events: no result")
	}

	assertResponseShape(t, "websocket_events", result, []fieldSpec{
		required("entries", "array"),
	})
}

func TestObserveContract_Actions(t *testing.T) {
	s := newScenario(t)
	s.loadActionData(t)

	result, ok := s.callObserve(t, "actions")
	if !ok {
		t.Fatal("observe actions: no result")
	}

	assertResponseShape(t, "actions", result, []fieldSpec{
		required("entries", "array"),
	})
}

// ============================================
// Tier 2: Analysis & Status Contracts
// ============================================

func TestObserveContract_WebSocketStatus(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "websocket_status")
	if !ok {
		t.Fatal("observe websocket_status: no result")
	}

	assertResponseShape(t, "websocket_status", result, []fieldSpec{
		required("connections", "array"),
		required("closed", "array"),
		required("active_count", "number"),
		required("closed_count", "number"),
	})
}

func TestObserveContract_Vitals(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "vitals")
	if !ok {
		t.Fatal("observe vitals: no result")
	}

	assertResponseShape(t, "vitals", result, []fieldSpec{
		required("metrics", "object"),
	})
}

func TestObserveContract_Page(t *testing.T) {
	s := newScenario(t)
	s.loadTrackingState(t)

	result, ok := s.callObserve(t, "page")
	if !ok {
		t.Fatal("observe page: no result")
	}

	assertResponseShape(t, "page", result, []fieldSpec{
		required("url", "string"),
		required("title", "string"),
	})
}

func TestObserveContract_Tabs(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "tabs")
	if !ok {
		t.Fatal("observe tabs: no result")
	}

	assertResponseShape(t, "tabs", result, []fieldSpec{
		required("tabs", "array"),
		required("tracking_active", "bool"),
	})
}

func TestObserveContract_Performance(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "performance")
	if !ok {
		t.Fatal("observe performance: no result")
	}

	assertResponseShape(t, "performance", result, []fieldSpec{
		required("snapshots", "array"),
		required("count", "number"),
	})
}

func TestObserveContract_Timeline(t *testing.T) {
	s := newScenario(t)
	s.loadFullScenario(t)

	result, ok := s.callObserve(t, "timeline")
	if !ok {
		t.Fatal("observe timeline: no result")
	}

	assertResponseShape(t, "timeline", result, []fieldSpec{
		required("entries", "array"),
		required("count", "number"),
	})
}

func TestObserveContract_ErrorClusters(t *testing.T) {
	s := newScenario(t)
	s.loadConsoleData(t)

	result, ok := s.callObserve(t, "error_clusters")
	if !ok {
		t.Fatal("observe error_clusters: no result")
	}

	assertResponseShape(t, "error_clusters", result, []fieldSpec{
		required("clusters", "array"),
		required("total_count", "number"),
	})
}

func TestObserveContract_History(t *testing.T) {
	s := newScenario(t)
	s.loadActionData(t)

	result, ok := s.callObserve(t, "history")
	if !ok {
		t.Fatal("observe history: no result")
	}

	assertResponseShape(t, "history", result, []fieldSpec{
		required("entries", "array"),
		required("count", "number"),
	})
}

// ============================================
// Tier 2b: Pilot & Security Contracts
// ============================================

func TestObserveContract_Pilot(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "pilot")
	if !ok {
		t.Fatal("observe pilot: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "pilot", data, []fieldSpec{
		required("enabled", "bool"),
		required("source", "string"),
		required("extension_connected", "bool"),
	})
}

func TestObserveContract_SecurityAudit(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "security_audit")
	if !ok {
		t.Fatal("observe security_audit: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "security_audit", data, []fieldSpec{
		optional("findings", "array"),   // null when no findings
		required("summary", "object"),
		required("scanned_at", "string"),
	})
}

func TestObserveContract_ThirdPartyAudit(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "third_party_audit")
	if !ok {
		t.Fatal("observe third_party_audit: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "third_party_audit", data, []fieldSpec{
		required("first_party_origin", "string"),
		optional("third_parties", "array"),    // null when no third parties
		required("summary", "object"),
		optional("recommendations", "array"),  // null when no recommendations
	})
}

// ============================================
// Tier 3: Security Diff, Async, Recording Contracts
// ============================================

func TestObserveContract_SecurityDiff(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "security_diff")
	if !ok {
		t.Fatal("observe security_diff: no result")
	}

	assertResponseShape(t, "security_diff", result, []fieldSpec{
		required("status", "string"),
		required("differences", "array"),
	})
}

func TestObserveContract_PendingCommands(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "pending_commands")
	if !ok {
		t.Fatal("observe pending_commands: no result")
	}

	assertResponseShape(t, "pending_commands", result, []fieldSpec{
		required("pending", "array"),
		required("completed", "array"),
		required("failed", "array"),
	})
}

func TestObserveContract_FailedCommands(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "failed_commands")
	if !ok {
		t.Fatal("observe failed_commands: no result")
	}

	assertResponseShape(t, "failed_commands", result, []fieldSpec{
		required("status", "string"),
		required("commands", "array"),
		required("count", "number"),
	})
}

func TestObserveContract_Recordings(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "recordings")
	if !ok {
		t.Fatal("observe recordings: no result")
	}

	assertResponseShape(t, "recordings", result, []fieldSpec{
		required("recordings", "array"),
		required("count", "number"),
		required("limit", "number"),
	})
}

func TestObserveContract_API(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "api")
	if !ok {
		t.Fatal("observe api: no result")
	}

	assertResponseShape(t, "api", result, []fieldSpec{
		required("status", "string"),
		required("message", "string"),
	})
}

func TestObserveContract_Changes(t *testing.T) {
	s := newScenario(t)

	result, ok := s.callObserve(t, "changes")
	if !ok {
		t.Fatal("observe changes: no result")
	}

	assertResponseShape(t, "changes", result, []fieldSpec{
		required("status", "string"),
		required("message", "string"),
	})
}

// ============================================
// Parameter-Required Modes (error path contracts)
// ============================================

func TestObserveContract_CommandResult_MissingParam(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserve(t, "command_result")
	if !ok {
		t.Fatal("observe command_result: no result")
	}
	assertStructuredError(t, "command_result (missing correlation_id)", result)
}

func TestObserveContract_RecordingActions_MissingParam(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserve(t, "recording_actions")
	if !ok {
		t.Fatal("observe recording_actions: no result")
	}
	assertStructuredError(t, "recording_actions (missing recording_id)", result)
}

func TestObserveContract_PlaybackResults_MissingParam(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserve(t, "playback_results")
	if !ok {
		t.Fatal("observe playback_results: no result")
	}
	assertStructuredError(t, "playback_results (missing recording_id)", result)
}

func TestObserveContract_LogDiffReport_MissingParam(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserve(t, "log_diff_report")
	if !ok {
		t.Fatal("observe log_diff_report: no result")
	}
	assertStructuredError(t, "log_diff_report (missing original_id+replay_id)", result)
}

// ============================================
// Extension-Required Modes
// ============================================

func TestObserveContract_Accessibility_NoTracking(t *testing.T) {
	s := newScenario(t)
	// No loadTrackingState — should return structured error
	result, ok := s.callObserve(t, "accessibility")
	if !ok {
		t.Fatal("observe accessibility: no result")
	}
	assertStructuredError(t, "accessibility (no tracking)", result)
}

// ============================================
// Structured Error Contract (universal)
// ============================================

func TestObserveContract_UnknownMode_StructuredError(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserve(t, "completely_invalid_mode_xyz")
	if !ok {
		t.Fatal("observe unknown mode: no result")
	}
	assertStructuredErrorCode(t, "unknown_mode", result, "unknown_mode")
}

func TestObserveContract_MissingWhat_StructuredError(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserve(t, "")
	if !ok {
		t.Fatal("observe empty what: no result")
	}
	assertStructuredErrorCode(t, "missing_what", result, "missing_param")
}
