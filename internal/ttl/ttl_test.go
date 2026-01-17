package ttl

import (
	"testing"
	"time"
)

// ============================================
// TTL Duration Parsing Tests
// ============================================

func TestTTLParseDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"1 hour", "1h", time.Hour, false},
		{"15 minutes", "15m", 15 * time.Minute, false},
		{"30 seconds rejected by minimum", "30s", 0, true},
		{"2 hours 30 minutes", "2h30m", 2*time.Hour + 30*time.Minute, false},
		{"5 minutes", "5m", 5 * time.Minute, false},
		{"empty string means unlimited", "", 0, false},
		{"invalid duration", "abc", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTTL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tc.input, err)
				return
			}
			if got != tc.expected {
				t.Errorf("ParseTTL(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestTTLMinimumEnforcement(t *testing.T) {
	t.Parallel()
	// TTL values below 1 minute should be rejected
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"59 seconds rejected", "59s", true},
		{"30 seconds rejected", "30s", true},
		{"1 second rejected", "1s", true},
		{"exactly 1 minute accepted", "1m", false},
		{"61 seconds accepted", "61s", false},
		{"2 minutes accepted", "2m", false},
		{"empty (unlimited) accepted", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTTL(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for TTL %q (below minimum), got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for TTL %q: %v", tc.input, err)
			}
		})
	}
}

// ============================================
// TTL=0 Means Unlimited (All Entries Returned)
// ============================================

func TestTTLZeroMeansUnlimited(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 0 // unlimited

	// Add events - both should be returned regardless of age
	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "old message",
		Timestamp: "2020-01-01T00:00:00Z",
	}})

	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "new message",
		Timestamp: "2025-01-01T00:00:00Z",
	}})

	// With TTL=0, all entries should be returned regardless of age
	events := capture.GetWebSocketEvents(WebSocketEventFilter{})
	if len(events) != 2 {
		t.Errorf("TTL=0 should return all entries, got %d, want 2", len(events))
	}
}

// ============================================
// TTL Filtering: Old Entries Excluded
// ============================================

func TestTTLFiltersOldWSEvents(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 1 * time.Minute

	// Add an event, then backdate its addedAt timestamp
	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "old message",
	}})

	// Backdate the first event to 2 minutes ago
	capture.mu.Lock()
	capture.wsAddedAt[0] = time.Now().Add(-2 * time.Minute)
	capture.mu.Unlock()

	// Add a fresh event
	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "new message",
	}})

	// Only the fresh event should be returned
	events := capture.GetWebSocketEvents(WebSocketEventFilter{})
	if len(events) != 1 {
		t.Errorf("expected 1 event after TTL filter, got %d", len(events))
		return
	}
	if events[0].Data != "new message" {
		t.Errorf("expected fresh event data 'new message', got %q", events[0].Data)
	}
}

func TestTTLFiltersOldNetworkBodies(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 1 * time.Minute

	// Add a network body, then backdate it
	capture.AddNetworkBodies([]NetworkBody{{
		URL: "http://example.com/api/old", Method: "GET", Status: 200,
	}})

	capture.mu.Lock()
	capture.networkAddedAt[0] = time.Now().Add(-2 * time.Minute)
	capture.mu.Unlock()

	// Add a fresh one
	capture.AddNetworkBodies([]NetworkBody{{
		URL: "http://example.com/api/new", Method: "GET", Status: 200,
	}})

	bodies := capture.GetNetworkBodies(NetworkBodyFilter{})
	if len(bodies) != 1 {
		t.Errorf("expected 1 body after TTL filter, got %d", len(bodies))
		return
	}
	if bodies[0].URL != "http://example.com/api/new" {
		t.Errorf("expected fresh body URL, got %q", bodies[0].URL)
	}
}

func TestTTLFiltersOldActions(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 1 * time.Minute

	capture.AddEnhancedActions([]EnhancedAction{{
		Type: "click", URL: "http://example.com/old",
	}})

	capture.mu.Lock()
	capture.actionAddedAt[0] = time.Now().Add(-2 * time.Minute)
	capture.mu.Unlock()

	capture.AddEnhancedActions([]EnhancedAction{{
		Type: "click", URL: "http://example.com/new",
	}})

	actions := capture.GetEnhancedActions(EnhancedActionFilter{})
	if len(actions) != 1 {
		t.Errorf("expected 1 action after TTL filter, got %d", len(actions))
		return
	}
	if actions[0].URL != "http://example.com/new" {
		t.Errorf("expected fresh action URL, got %q", actions[0].URL)
	}
}

func TestTTLFiltersOldConsoleLogs(t *testing.T) {
	t.Parallel()
	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	server.TTL = 1 * time.Minute

	// Add entries
	server.addEntries([]LogEntry{{"level": "info", "message": "old log"}})

	// Backdate the first entry
	server.mu.Lock()
	server.logAddedAt[0] = time.Now().Add(-2 * time.Minute)
	server.mu.Unlock()

	server.addEntries([]LogEntry{{"level": "info", "message": "new log"}})

	// Read entries with TTL filtering
	entries := server.getEntriesWithTTL()
	if len(entries) != 1 {
		t.Errorf("expected 1 log entry after TTL filter, got %d", len(entries))
		return
	}
	if entries[0]["message"] != "new log" {
		t.Errorf("expected 'new log', got %v", entries[0]["message"])
	}
}

// ============================================
// TTL Filtering: Fresh Entries Returned
// ============================================

func TestTTLReturnsFreshEntries(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 1 * time.Hour

	// Add 5 events, all fresh
	for i := 0; i < 5; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{{
			ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "message",
		}})
	}

	events := capture.GetWebSocketEvents(WebSocketEventFilter{})
	if len(events) != 5 {
		t.Errorf("all fresh events should be returned, got %d, want 5", len(events))
	}
}

// ============================================
// TTL Boundary Case: Entry Exactly at TTL Age
// ============================================

func TestTTLBoundaryExactlyAtTTL(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 1 * time.Minute

	// Add an event and set its addedAt to exactly TTL ago
	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "boundary message",
	}})

	// Set addedAt to exactly 1 minute ago (at the boundary)
	capture.mu.Lock()
	capture.wsAddedAt[0] = time.Now().Add(-1 * time.Minute)
	capture.mu.Unlock()

	// An entry exactly at TTL age should be filtered out (strictly older than cutoff)
	events := capture.GetWebSocketEvents(WebSocketEventFilter{})
	if len(events) != 0 {
		t.Errorf("entry exactly at TTL boundary should be filtered, got %d events", len(events))
	}
}

// ============================================
// TTL Does Not Affect Buffer Write Behavior
// ============================================

func TestTTLDoesNotAffectWrites(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 1 * time.Second // Very short TTL (below minimum for real use, but tests internal behavior)

	// Add events - they should all be added to the buffer regardless of TTL
	for i := 0; i < 10; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{{
			ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "message",
		}})
	}

	// Check internal buffer length (TTL only filters on read, not write)
	capture.mu.RLock()
	bufLen := len(capture.wsEvents)
	capture.mu.RUnlock()

	if bufLen != 10 {
		t.Errorf("TTL should not affect writes, buffer has %d entries, want 10", bufLen)
	}
}

func TestTTLRingBufferStillWorks(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 1 * time.Hour

	// Add more events than buffer capacity to verify ring buffer still rotates
	for i := 0; i < maxWSEvents+10; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{{
			ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "message",
		}})
	}

	capture.mu.RLock()
	bufLen := len(capture.wsEvents)
	capture.mu.RUnlock()

	if bufLen > maxWSEvents {
		t.Errorf("ring buffer should cap at %d, got %d", maxWSEvents, bufLen)
	}
}

// ============================================
// SetTTL Method
// ============================================

func TestTTLSetTTL(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	capture.SetTTL(15 * time.Minute)
	if capture.TTL != 15*time.Minute {
		t.Errorf("SetTTL failed: got %v, want 15m", capture.TTL)
	}

	capture.SetTTL(0)
	if capture.TTL != 0 {
		t.Errorf("SetTTL(0) should set unlimited: got %v, want 0", capture.TTL)
	}
}

func TestTTLServerSetTTL(t *testing.T) {
	t.Parallel()
	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	server.SetTTL(30 * time.Minute)
	if server.TTL != 30*time.Minute {
		t.Errorf("Server.SetTTL failed: got %v, want 30m", server.TTL)
	}
}

// ============================================
// TTL Applies Consistently to All Buffer Types
// ============================================

func TestTTLAppliesConsistently(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 5 * time.Minute

	// Add one entry to each buffer, then backdate all of them
	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming",
	}})
	capture.AddNetworkBodies([]NetworkBody{{URL: "http://example.com", Method: "GET", Status: 200}})
	capture.AddEnhancedActions([]EnhancedAction{{Type: "click", URL: "http://example.com"}})

	// Backdate all entries to 10 minutes ago
	capture.mu.Lock()
	oldTime := time.Now().Add(-10 * time.Minute)
	capture.wsAddedAt[0] = oldTime
	capture.networkAddedAt[0] = oldTime
	capture.actionAddedAt[0] = oldTime
	capture.mu.Unlock()

	// All buffers should return empty
	wsEvents := capture.GetWebSocketEvents(WebSocketEventFilter{})
	if len(wsEvents) != 0 {
		t.Errorf("WS events should be filtered, got %d", len(wsEvents))
	}

	bodies := capture.GetNetworkBodies(NetworkBodyFilter{})
	if len(bodies) != 0 {
		t.Errorf("Network bodies should be filtered, got %d", len(bodies))
	}

	actions := capture.GetEnhancedActions(EnhancedActionFilter{})
	if len(actions) != 0 {
		t.Errorf("Actions should be filtered, got %d", len(actions))
	}
}

// ============================================
// TTL with Other Filters (Combined Behavior)
// ============================================

func TestTTLCombinesWithOtherFilters(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.TTL = 5 * time.Minute

	// Add 3 events: 1 old, 2 fresh (one matches URL filter, one doesn't)
	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "old",
	}})
	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://example.com", Event: "message", Direction: "incoming", Data: "fresh-match",
	}})
	capture.AddWebSocketEvents([]WebSocketEvent{{
		ID: "conn-1", URL: "ws://other.com", Event: "message", Direction: "incoming", Data: "fresh-nomatch",
	}})

	// Backdate the first one
	capture.mu.Lock()
	capture.wsAddedAt[0] = time.Now().Add(-10 * time.Minute)
	capture.mu.Unlock()

	// Filter by URL and TTL should both apply
	events := capture.GetWebSocketEvents(WebSocketEventFilter{URLFilter: "example.com"})
	if len(events) != 1 {
		t.Errorf("expected 1 event (fresh + URL match), got %d", len(events))
		return
	}
	if events[0].Data != "fresh-match" {
		t.Errorf("expected 'fresh-match', got %q", events[0].Data)
	}
}

// ============================================
// TTL Filtering of Console Errors (via Server)
// ============================================

func TestTTLFiltersOldErrors(t *testing.T) {
	t.Parallel()
	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	server.TTL = 1 * time.Minute

	// Add an old error and a new error
	server.addEntries([]LogEntry{{"level": "error", "message": "old error"}})
	server.mu.Lock()
	server.logAddedAt[0] = time.Now().Add(-2 * time.Minute)
	server.mu.Unlock()

	server.addEntries([]LogEntry{{"level": "error", "message": "new error"}})

	entries := server.getEntriesWithTTL()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry after TTL filter, got %d", len(entries))
		return
	}
	if entries[0]["message"] != "new error" {
		t.Errorf("expected 'new error', got %v", entries[0]["message"])
	}
}
