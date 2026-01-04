// audit_trail_test.go — TDD tests for Enterprise Audit Trail (Tier 1).
// Tests cover: tool invocation logging, client identification, session ID
// assignment, parameter redaction, redaction event logging, query/filter,
// FIFO eviction, concurrent safety, and disabled mode.
package main

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================
// Test: Recording a tool call creates an entry
// ============================================

func TestAuditTrail_RecordEntry(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	entry := AuditEntry{
		SessionID:    "sess-001",
		ClientID:     "claude-code",
		ToolName:     "observe",
		Parameters:   `{"mode":"console"}`,
		ResponseSize: 2048,
		Duration:     15,
		Success:      true,
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(results))
	}

	got := results[0]
	if got.ID == "" {
		t.Error("expected entry to have an ID assigned")
	}
	if got.Timestamp.IsZero() {
		t.Error("expected entry to have a timestamp assigned")
	}
	if got.SessionID != "sess-001" {
		t.Errorf("expected session_id 'sess-001', got %q", got.SessionID)
	}
	if got.ClientID != "claude-code" {
		t.Errorf("expected client_id 'claude-code', got %q", got.ClientID)
	}
	if got.ToolName != "observe" {
		t.Errorf("expected tool_name 'observe', got %q", got.ToolName)
	}
	if got.Parameters != `{"mode":"console"}` {
		t.Errorf("expected parameters preserved, got %q", got.Parameters)
	}
	if got.ResponseSize != 2048 {
		t.Errorf("expected response_size 2048, got %d", got.ResponseSize)
	}
	if got.Duration != 15 {
		t.Errorf("expected duration 15, got %d", got.Duration)
	}
	if !got.Success {
		t.Error("expected success to be true")
	}
	if got.ErrorMessage != "" {
		t.Errorf("expected empty error_message, got %q", got.ErrorMessage)
	}
}

// ============================================
// Test: Entry with error message
// ============================================

func TestAuditTrail_RecordErrorEntry(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	entry := AuditEntry{
		SessionID:    "sess-002",
		ClientID:     "cursor",
		ToolName:     "query_dom",
		Parameters:   `{"selector":".btn"}`,
		ResponseSize: 0,
		Duration:     5,
		Success:      false,
		ErrorMessage: "no active tab",
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(results))
	}
	if results[0].Success {
		t.Error("expected success to be false")
	}
	if results[0].ErrorMessage != "no active tab" {
		t.Errorf("expected error_message 'no active tab', got %q", results[0].ErrorMessage)
	}
}

// ============================================
// Test: Query with no filter returns latest (default limit 100)
// ============================================

func TestAuditTrail_QueryDefaultLimit(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   10000,
		Enabled:      true,
		RedactParams: false,
	})

	// Record 150 entries
	for i := 0; i < 150; i++ {
		trail.Record(AuditEntry{
			SessionID: "sess-default",
			ClientID:  "claude-code",
			ToolName:  "observe",
			Success:   true,
		})
	}

	results := trail.Query(AuditFilter{})
	if len(results) != 100 {
		t.Fatalf("expected default limit of 100, got %d", len(results))
	}
}

// ============================================
// Test: Query by session_id
// ============================================

func TestAuditTrail_QueryBySessionID(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{SessionID: "sess-A", ClientID: "claude-code", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: "sess-B", ClientID: "cursor", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{SessionID: "sess-A", ClientID: "claude-code", ToolName: "generate", Success: true})
	trail.Record(AuditEntry{SessionID: "sess-B", ClientID: "cursor", ToolName: "observe", Success: true})

	results := trail.Query(AuditFilter{SessionID: "sess-A"})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for sess-A, got %d", len(results))
	}
	for _, r := range results {
		if r.SessionID != "sess-A" {
			t.Errorf("expected session_id 'sess-A', got %q", r.SessionID)
		}
	}
}

// ============================================
// Test: Query by tool_name
// ============================================

func TestAuditTrail_QueryByToolName(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{SessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "generate", Success: true})

	results := trail.Query(AuditFilter{ToolName: "observe"})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for tool 'observe', got %d", len(results))
	}
	for _, r := range results {
		if r.ToolName != "observe" {
			t.Errorf("expected tool_name 'observe', got %q", r.ToolName)
		}
	}
}

// ============================================
// Test: Query with 'since' timestamp filter
// ============================================

func TestAuditTrail_QuerySince(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	// Record entries with slight delays to ensure distinct timestamps
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "observe", Success: true})
	time.Sleep(10 * time.Millisecond)

	cutoff := time.Now()
	time.Sleep(10 * time.Millisecond)

	trail.Record(AuditEntry{SessionID: "s1", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "generate", Success: true})

	results := trail.Query(AuditFilter{Since: &cutoff})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries after cutoff, got %d", len(results))
	}
}

// ============================================
// Test: Query with custom limit
// ============================================

func TestAuditTrail_QueryWithLimit(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	for i := 0; i < 50; i++ {
		trail.Record(AuditEntry{SessionID: "s1", ToolName: "observe", Success: true})
	}

	results := trail.Query(AuditFilter{Limit: 10})
	if len(results) != 10 {
		t.Fatalf("expected 10 entries with limit=10, got %d", len(results))
	}
}

// ============================================
// Test: Max entries enforced (FIFO eviction)
// ============================================

func TestAuditTrail_FIFOEviction(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   5,
		Enabled:      true,
		RedactParams: false,
	})

	// Record 8 entries; only last 5 should remain
	for i := 0; i < 8; i++ {
		trail.Record(AuditEntry{
			SessionID: "s1",
			ToolName:  "tool-" + string(rune('A'+i)),
			Success:   true,
		})
	}

	results := trail.Query(AuditFilter{Limit: 10})
	if len(results) != 5 {
		t.Fatalf("expected 5 entries (max), got %d", len(results))
	}

	// The oldest entries (tool-A, tool-B, tool-C) should be evicted
	// The remaining entries should be tool-D through tool-H
	for _, r := range results {
		if r.ToolName == "tool-A" || r.ToolName == "tool-B" || r.ToolName == "tool-C" {
			t.Errorf("expected old entry %q to be evicted", r.ToolName)
		}
	}
}

// ============================================
// Test: Concurrent recording is safe
// ============================================

func TestAuditTrail_ConcurrentSafety(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   10000,
		Enabled:      true,
		RedactParams: false,
	})

	var wg sync.WaitGroup
	numGoroutines := 50
	entriesPerGoroutine := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				trail.Record(AuditEntry{
					SessionID: "concurrent-sess",
					ClientID:  "test-client",
					ToolName:  "observe",
					Success:   true,
				})
			}
		}(g)
	}

	wg.Wait()

	results := trail.Query(AuditFilter{Limit: 10000})
	expected := numGoroutines * entriesPerGoroutine
	if len(results) != expected {
		t.Errorf("expected %d entries after concurrent writes, got %d", expected, len(results))
	}
}

// ============================================
// Test: Client identification from initialize message
// ============================================

func TestAuditTrail_ClientIdentification(t *testing.T) {
	tests := []struct {
		name     string
		input    ClientIdentifier
		expected string
	}{
		{"claude-code", ClientIdentifier{Name: "claude-code", Version: "1.0.0"}, "claude-code"},
		{"Cursor uppercase", ClientIdentifier{Name: "Cursor", Version: "0.45"}, "cursor"},
		{"Windsurf uppercase", ClientIdentifier{Name: "Windsurf", Version: "2.0"}, "windsurf"},
		{"cline lowercase", ClientIdentifier{Name: "cline", Version: "3.1"}, "cline"},
		{"unknown client preserved", ClientIdentifier{Name: "my-custom-tool", Version: "0.1"}, "my-custom-tool"},
		{"empty name", ClientIdentifier{Name: "", Version: "1.0"}, "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			trail := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})
			clientID := trail.IdentifyClient(tc.input)
			if clientID != tc.expected {
				t.Errorf("expected client ID %q, got %q", tc.expected, clientID)
			}
		})
	}
}

// ============================================
// Test: Session ID is unique per connection
// ============================================

func TestAuditTrail_SessionIDUnique(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		sess := trail.CreateSession(ClientIdentifier{Name: "claude-code", Version: "1.0"})
		if seen[sess.ID] {
			t.Fatalf("duplicate session ID: %s", sess.ID)
		}
		seen[sess.ID] = true
	}
}

// ============================================
// Test: Session ID format (hex-encoded, 32 chars)
// ============================================

func TestAuditTrail_SessionIDFormat(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})
	sess := trail.CreateSession(ClientIdentifier{Name: "cursor", Version: "1.0"})

	if len(sess.ID) != 32 {
		t.Errorf("expected session ID length 32, got %d: %q", len(sess.ID), sess.ID)
	}

	// Verify it's valid hex
	for _, c := range sess.ID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("session ID contains non-hex character: %c in %q", c, sess.ID)
			break
		}
	}
}

// ============================================
// Test: Session correlates entries
// ============================================

func TestAuditTrail_SessionCorrelation(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{MaxEntries: 1000, Enabled: true, RedactParams: false})

	sess := trail.CreateSession(ClientIdentifier{Name: "claude-code", Version: "1.0"})

	trail.Record(AuditEntry{SessionID: sess.ID, ClientID: "claude-code", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: sess.ID, ClientID: "claude-code", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{SessionID: "other-session", ClientID: "cursor", ToolName: "observe", Success: true})

	results := trail.Query(AuditFilter{SessionID: sess.ID})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for session, got %d", len(results))
	}
	for _, r := range results {
		if r.SessionID != sess.ID {
			t.Errorf("expected session ID %q, got %q", sess.ID, r.SessionID)
		}
	}
}

// ============================================
// Test: Session tracks tool call count
// ============================================

func TestAuditTrail_SessionToolCallCount(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{MaxEntries: 1000, Enabled: true, RedactParams: false})

	sess := trail.CreateSession(ClientIdentifier{Name: "claude-code", Version: "1.0"})

	trail.Record(AuditEntry{SessionID: sess.ID, ClientID: "claude-code", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: sess.ID, ClientID: "claude-code", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{SessionID: sess.ID, ClientID: "claude-code", ToolName: "generate", Success: true})

	info := trail.GetSession(sess.ID)
	if info == nil {
		t.Fatal("expected session to exist")
	}
	if info.ToolCalls != 3 {
		t.Errorf("expected 3 tool calls, got %d", info.ToolCalls)
	}
}

// ============================================
// Test: Parameter redaction - bearer tokens
// ============================================

func TestAuditTrail_RedactBearerToken(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"}`
	entry := AuditEntry{
		SessionID:  "s1",
		ClientID:   "claude-code",
		ToolName:   "observe",
		Parameters: params,
		Success:    true,
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(results))
	}

	if strings.Contains(results[0].Parameters, "eyJhbGci") {
		t.Error("expected bearer token to be redacted")
	}
	if !strings.Contains(results[0].Parameters, "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder in parameters")
	}
}

// ============================================
// Test: Parameter redaction - API keys
// ============================================

func TestAuditTrail_RedactAPIKey(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"config": "api_key=sk-abc123xyz456 something"}`
	entry := AuditEntry{
		SessionID:  "s1",
		ClientID:   "cursor",
		ToolName:   "configure",
		Parameters: params,
		Success:    true,
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	if strings.Contains(results[0].Parameters, "sk-abc123xyz456") {
		t.Error("expected API key to be redacted")
	}
	if !strings.Contains(results[0].Parameters, "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder in parameters")
	}
}

// ============================================
// Test: Parameter redaction - regular params preserved
// ============================================

func TestAuditTrail_RegularParamsPreserved(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"mode": "console", "limit": 50, "selector": ".btn-primary"}`
	entry := AuditEntry{
		SessionID:  "s1",
		ClientID:   "claude-code",
		ToolName:   "observe",
		Parameters: params,
		Success:    true,
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	// Regular params should be preserved as-is
	if results[0].Parameters != params {
		t.Errorf("expected regular params to be preserved.\nGot:      %q\nExpected: %q", results[0].Parameters, params)
	}
}

// ============================================
// Test: Redaction events logged separately
// ============================================

func TestAuditTrail_RedactionEventLogging(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	event := RedactionEvent{
		Timestamp:   time.Now(),
		SessionID:   "sess-100",
		ToolName:    "get_network_bodies",
		FieldPath:   "entries[0].response.headers.authorization",
		PatternName: "bearer_token",
	}

	trail.RecordRedaction(event)

	events := trail.QueryRedactions(AuditFilter{SessionID: "sess-100"})
	if len(events) != 1 {
		t.Fatalf("expected 1 redaction event, got %d", len(events))
	}

	got := events[0]
	if got.ToolName != "get_network_bodies" {
		t.Errorf("expected tool_name 'get_network_bodies', got %q", got.ToolName)
	}
	if got.FieldPath != "entries[0].response.headers.authorization" {
		t.Errorf("expected field_path, got %q", got.FieldPath)
	}
	if got.PatternName != "bearer_token" {
		t.Errorf("expected pattern_name 'bearer_token', got %q", got.PatternName)
	}
}

// ============================================
// Test: Redaction events - no content stored
// ============================================

func TestAuditTrail_RedactionNoContent(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{MaxEntries: 1000, Enabled: true})

	event := RedactionEvent{
		Timestamp:   time.Now(),
		SessionID:   "sess-200",
		ToolName:    "observe",
		FieldPath:   "entries[5].message",
		PatternName: "credit_card",
	}

	trail.RecordRedaction(event)

	// Serialize the event and verify no sensitive content fields
	events := trail.QueryRedactions(AuditFilter{})
	data, err := json.Marshal(events[0])
	if err != nil {
		t.Fatal(err)
	}

	// The JSON should only contain the known fields, nothing else
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	allowed := map[string]bool{
		"timestamp": true, "session_id": true, "tool_name": true,
		"field_path": true, "pattern_name": true,
	}
	for key := range raw {
		if !allowed[key] {
			t.Errorf("unexpected field %q in redaction event (potential content leak)", key)
		}
	}
}

// ============================================
// Test: Disabled audit trail silently drops entries
// ============================================

func TestAuditTrail_DisabledDropsEntries(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      false,
		RedactParams: false,
	})

	trail.Record(AuditEntry{
		SessionID: "s1",
		ClientID:  "claude-code",
		ToolName:  "observe",
		Success:   true,
	})

	results := trail.Query(AuditFilter{})
	if len(results) != 0 {
		t.Errorf("expected 0 entries when disabled, got %d", len(results))
	}
}

// ============================================
// Test: Empty filter returns all entries up to limit
// ============================================

func TestAuditTrail_EmptyFilterReturnsAll(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{SessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: "s2", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{SessionID: "s3", ToolName: "generate", Success: true})

	results := trail.Query(AuditFilter{})
	if len(results) != 3 {
		t.Errorf("expected 3 entries with empty filter, got %d", len(results))
	}
}

// ============================================
// Test: Query returns reverse chronological order
// ============================================

func TestAuditTrail_ReverseChronologicalOrder(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{SessionID: "s1", ToolName: "first", Success: true})
	time.Sleep(5 * time.Millisecond)
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "second", Success: true})
	time.Sleep(5 * time.Millisecond)
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "third", Success: true})

	results := trail.Query(AuditFilter{})
	if len(results) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(results))
	}

	// Reverse chronological: newest first
	if results[0].ToolName != "third" {
		t.Errorf("expected first result to be 'third', got %q", results[0].ToolName)
	}
	if results[2].ToolName != "first" {
		t.Errorf("expected last result to be 'first', got %q", results[2].ToolName)
	}
}

// ============================================
// Test: Duration correctly captured
// ============================================

func TestAuditTrail_DurationCaptured(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{
		SessionID: "s1",
		ToolName:  "observe",
		Duration:  42,
		Success:   true,
	})

	results := trail.Query(AuditFilter{})
	if results[0].Duration != 42 {
		t.Errorf("expected duration 42, got %d", results[0].Duration)
	}
}

// ============================================
// Test: Response size correctly captured
// ============================================

func TestAuditTrail_ResponseSizeCaptured(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{
		SessionID:    "s1",
		ToolName:     "get_network_bodies",
		ResponseSize: 15360,
		Success:      true,
	})

	results := trail.Query(AuditFilter{})
	if results[0].ResponseSize != 15360 {
		t.Errorf("expected response_size 15360, got %d", results[0].ResponseSize)
	}
}

// ============================================
// Test: handleGetAuditLog MCP tool handler
// ============================================

func TestAuditTrail_HandleGetAuditLog(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{SessionID: "s1", ClientID: "claude-code", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: "s1", ClientID: "claude-code", ToolName: "analyze", Success: true})

	// Call the MCP handler with empty params
	params := json.RawMessage(`{}`)
	result, err := trail.HandleGetAuditLog(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be serializable
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	if !strings.Contains(string(data), "observe") {
		t.Error("expected result to contain 'observe' tool entry")
	}
	if !strings.Contains(string(data), "analyze") {
		t.Error("expected result to contain 'analyze' tool entry")
	}
}

// ============================================
// Test: handleGetAuditLog with filter params
// ============================================

func TestAuditTrail_HandleGetAuditLogFiltered(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{SessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: "s2", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "generate", Success: true})

	params := json.RawMessage(`{"session_id": "s1"}`)
	result, err := trail.HandleGetAuditLog(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	if strings.Contains(string(data), "analyze") {
		t.Error("expected result to NOT contain session s2 entry 'analyze'")
	}
}

// ============================================
// Test: Multiple combined filters (session + tool)
// ============================================

func TestAuditTrail_CombinedFilters(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{SessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{SessionID: "s2", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{SessionID: "s2", ToolName: "analyze", Success: true})

	results := trail.Query(AuditFilter{SessionID: "s1", ToolName: "observe"})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry with combined filter, got %d", len(results))
	}
	if results[0].SessionID != "s1" || results[0].ToolName != "observe" {
		t.Errorf("unexpected entry: session=%q tool=%q", results[0].SessionID, results[0].ToolName)
	}
}

// ============================================
// Test: Redaction of JWT tokens in parameters
// ============================================

func TestAuditTrail_RedactJWT(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	params := `{"token": "` + jwt + `"}`
	trail.Record(AuditEntry{
		SessionID:  "s1",
		ToolName:   "observe",
		Parameters: params,
		Success:    true,
	})

	results := trail.Query(AuditFilter{})
	if strings.Contains(results[0].Parameters, "eyJhbGci") {
		t.Error("expected JWT to be redacted from parameters")
	}
	if !strings.Contains(results[0].Parameters, "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder")
	}
}

// ============================================
// Test: Redaction of GitHub tokens
// ============================================

func TestAuditTrail_RedactGitHubToken(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"auth": "Bearer ghp_ABCDEFghijklMNOPQRSTuvwxyz0123456789"}`
	trail.Record(AuditEntry{
		SessionID:  "s1",
		ToolName:   "observe",
		Parameters: params,
		Success:    true,
	})

	results := trail.Query(AuditFilter{})
	if strings.Contains(results[0].Parameters, "ghp_") {
		t.Error("expected GitHub token to be redacted")
	}
}

// ============================================
// Test: Session info stores client identity
// ============================================

func TestAuditTrail_SessionStoresClientIdentity(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})

	sess := trail.CreateSession(ClientIdentifier{Name: "Windsurf", Version: "2.5.0"})

	info := trail.GetSession(sess.ID)
	if info == nil {
		t.Fatal("expected session to exist")
	}
	if info.ClientID != "windsurf" {
		t.Errorf("expected client_id 'windsurf', got %q", info.ClientID)
	}
	if info.StartedAt.IsZero() {
		t.Error("expected started_at to be set")
	}
}

// ============================================
// Test: Default config values
// ============================================

func TestAuditTrail_DefaultConfig(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{})

	// Default should be: MaxEntries=10000, Enabled=true, RedactParams=true
	// Record something to verify it works with defaults
	trail.Record(AuditEntry{SessionID: "s1", ToolName: "observe", Success: true})

	results := trail.Query(AuditFilter{})
	if len(results) != 1 {
		t.Errorf("expected default config to allow recording, got %d entries", len(results))
	}
}

// ============================================
// Test: Concurrent session creation is safe
// ============================================

func TestAuditTrail_ConcurrentSessionCreation(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{MaxEntries: 10000, Enabled: true})

	var wg sync.WaitGroup
	sessions := make([]string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sess := trail.CreateSession(ClientIdentifier{Name: "test", Version: "1.0"})
			sessions[idx] = sess.ID
		}(i)
	}

	wg.Wait()

	// All session IDs should be unique
	seen := make(map[string]bool)
	for _, id := range sessions {
		if id == "" {
			t.Error("got empty session ID")
			continue
		}
		if seen[id] {
			t.Errorf("duplicate session ID: %s", id)
		}
		seen[id] = true
	}
}

// ============================================
// Test: Redaction events bounded by max entries
// ============================================

func TestAuditTrail_RedactionEventsBounded(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{MaxEntries: 5, Enabled: true})

	for i := 0; i < 10; i++ {
		trail.RecordRedaction(RedactionEvent{
			Timestamp:   time.Now(),
			SessionID:   "s1",
			ToolName:    "observe",
			FieldPath:   "field",
			PatternName: "bearer_token",
		})
	}

	events := trail.QueryRedactions(AuditFilter{Limit: 20})
	if len(events) > 5 {
		t.Errorf("expected at most 5 redaction events (bounded), got %d", len(events))
	}
}

// ============================================
// Test: Redaction of session cookies
// ============================================

func TestAuditTrail_RedactSessionCookie(t *testing.T) {
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"cookie": "session=ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"}`
	trail.Record(AuditEntry{
		SessionID:  "s1",
		ToolName:   "observe",
		Parameters: params,
		Success:    true,
	})

	results := trail.Query(AuditFilter{})
	if strings.Contains(results[0].Parameters, "ABCDEFGHIJKLMNOP") {
		t.Error("expected session cookie to be redacted")
	}
}
