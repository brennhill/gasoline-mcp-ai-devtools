// pagination_websocket_test.go — Unit tests for websocket event cursor-based pagination
package pagination

import (
	"testing"
)

func TestEnrichWebSocketEntries(t *testing.T) {
	tests := []struct {
		name             string
		events           []WebSocketEvent
		wsTotalAdded     int64
		expectedFirstSeq int64
		expectedLastSeq  int64
		expectedCount    int
	}{
		{
			name:          "empty buffer",
			events:        []WebSocketEvent{},
			wsTotalAdded:  0,
			expectedFirstSeq: 0,
			expectedLastSeq:  0,
			expectedCount: 0,
		},
		{
			name: "single event",
			events: []WebSocketEvent{
				{Event: "message", ID: "ws-1", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:15:23Z"},
			},
			wsTotalAdded:     1,
			expectedFirstSeq: 1,
			expectedLastSeq:  1,
			expectedCount:    1,
		},
		{
			name: "multiple events no eviction",
			events: []WebSocketEvent{
				{Event: "open", ID: "ws-1", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:15:20Z"},
				{Event: "message", ID: "ws-1", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:15:21Z"},
				{Event: "close", ID: "ws-1", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:15:22Z"},
			},
			wsTotalAdded:     3,
			expectedFirstSeq: 1,
			expectedLastSeq:  3,
			expectedCount:    3,
		},
		{
			name: "buffer with evictions (wsTotalAdded > len)",
			events: []WebSocketEvent{
				{Event: "message", ID: "ws-2", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:20:00Z"},
				{Event: "close", ID: "ws-2", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:20:01Z"},
			},
			wsTotalAdded:     152, // 150 evicted, 2 remain
			expectedFirstSeq: 151,  // First entry is sequence 151
			expectedLastSeq:  152,  // Last entry is sequence 152
			expectedCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enriched := EnrichWebSocketEntries(tt.events, tt.wsTotalAdded)

			if len(enriched) != tt.expectedCount {
				t.Errorf("EnrichWebSocketEntries() count = %d, want %d", len(enriched), tt.expectedCount)
			}

			if len(enriched) > 0 {
				firstSeq := enriched[0].Sequence
				if firstSeq != tt.expectedFirstSeq {
					t.Errorf("First sequence = %d, want %d", firstSeq, tt.expectedFirstSeq)
				}

				lastSeq := enriched[len(enriched)-1].Sequence
				if lastSeq != tt.expectedLastSeq {
					t.Errorf("Last sequence = %d, want %d", lastSeq, tt.expectedLastSeq)
				}

				// Verify timestamps are preserved (already RFC3339 strings)
				for i, e := range enriched {
					if e.Timestamp == "" && tt.events[i].Timestamp != "" {
						t.Errorf("Entry %d: timestamp was lost", i)
					}
				}
			}
		})
	}
}

func TestApplyWebSocketCursorPagination_NoCursor(t *testing.T) {
	// Create 100 events
	events := make([]WebSocketEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = WebSocketEvent{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://echo.example.com",
			Timestamp: "2026-01-30T10:15:00Z", // Same timestamp for all (batched)
		}
	}
	enriched := EnrichWebSocketEntries(events, 100)

	tests := []struct {
		name          string
		limit         int
		expectedCount int
	}{
		{
			name:          "no limit returns all",
			limit:         0,
			expectedCount: 100,
		},
		{
			name:          "limit 50 returns last 50",
			limit:         50,
			expectedCount: 50,
		},
		{
			name:          "limit exceeds buffer size",
			limit:         200,
			expectedCount: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := ApplyWebSocketCursorPagination(enriched, "", "", "", tt.limit, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Result count = %d, want %d", len(result), tt.expectedCount)
			}

			if metadata.Count != tt.expectedCount {
				t.Errorf("Metadata count = %d, want %d", metadata.Count, tt.expectedCount)
			}

			if metadata.Total != 100 {
				t.Errorf("Metadata total = %d, want 100", metadata.Total)
			}

			// When limit is applied (no cursor), should return LAST N entries
			if tt.limit > 0 && tt.limit < 100 {
				firstSeq := result[0].Sequence
				expectedFirstSeq := int64(100 - tt.expectedCount + 1)
				if firstSeq != expectedFirstSeq {
					t.Errorf("First sequence = %d, want %d (should be last %d entries)", firstSeq, expectedFirstSeq, tt.expectedCount)
				}
			}

			// Verify cursor field is present when results exist
			if len(result) > 0 {
				expectedCursor := BuildCursor(result[len(result)-1].Timestamp, result[len(result)-1].Sequence)
				if metadata.Cursor != expectedCursor {
					t.Errorf("Metadata cursor = %v, want %v", metadata.Cursor, expectedCursor)
				}

				// Verify timestamp fields
				if metadata.OldestTimestamp != result[0].Timestamp {
					t.Errorf("OldestTimestamp = %v, want %v", metadata.OldestTimestamp, result[0].Timestamp)
				}
				if metadata.NewestTimestamp != result[len(result)-1].Timestamp {
					t.Errorf("NewestTimestamp = %v, want %v", metadata.NewestTimestamp, result[len(result)-1].Timestamp)
				}
			} else {
				// Empty results should have empty cursor
				if metadata.Cursor != "" {
					t.Errorf("Metadata cursor should be empty for empty results, got %v", metadata.Cursor)
				}
			}
		})
	}
}

func TestApplyWebSocketCursorPagination_AfterCursor(t *testing.T) {
	// Create 100 events (sequences 1-100)
	events := make([]WebSocketEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = WebSocketEvent{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://echo.example.com",
			Timestamp: "2026-01-30T10:15:00Z",
		}
	}
	enriched := EnrichWebSocketEntries(events, 100)

	// Build cursors from actual enriched data
	cursor50 := BuildCursor(enriched[49].Timestamp, enriched[49].Sequence)   // Sequence 50
	cursor1 := BuildCursor(enriched[0].Timestamp, enriched[0].Sequence)      // Sequence 1
	cursor100 := BuildCursor(enriched[99].Timestamp, enriched[99].Sequence) // Sequence 100

	tests := []struct {
		name             string
		afterCursor      string
		limit            int
		expectedCount    int
		expectedFirstSeq int64
		expectedLastSeq  int64
		expectedHasMore  bool
	}{
		{
			name:             "after cursor gets older entries",
			afterCursor:      cursor50,
			limit:            0,
			expectedCount:    49, // Sequences 1-49
			expectedFirstSeq: 1,
			expectedLastSeq:  49,
			expectedHasMore:  false,
		},
		{
			name:             "after cursor with limit",
			afterCursor:      cursor50,
			limit:            10,
			expectedCount:    10, // Last 10 of sequences 1-49 = sequences 40-49
			expectedFirstSeq: 40,
			expectedLastSeq:  49,
			expectedHasMore:  true,
		},
		{
			name:             "after cursor at beginning",
			afterCursor:      cursor1,
			limit:            0,
			expectedCount:    0, // No entries older than sequence 1
			expectedFirstSeq: 0,
			expectedLastSeq:  0,
			expectedHasMore:  false,
		},
		{
			name:             "after cursor at end",
			afterCursor:      cursor100,
			limit:            0,
			expectedCount:    99, // All entries except sequence 100
			expectedFirstSeq: 1,
			expectedLastSeq:  99,
			expectedHasMore:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := ApplyWebSocketCursorPagination(enriched, tt.afterCursor, "", "", tt.limit, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Result count = %d, want %d", len(result), tt.expectedCount)
			}

			if tt.expectedCount > 0 {
				firstSeq := result[0].Sequence
				if firstSeq != tt.expectedFirstSeq {
					t.Errorf("First sequence = %d, want %d", firstSeq, tt.expectedFirstSeq)
				}

				lastSeq := result[len(result)-1].Sequence
				if lastSeq != tt.expectedLastSeq {
					t.Errorf("Last sequence = %d, want %d", lastSeq, tt.expectedLastSeq)
				}

				// Verify cursor field is present
				expectedCursor := BuildCursor(result[len(result)-1].Timestamp, result[len(result)-1].Sequence)
				if metadata.Cursor != expectedCursor {
					t.Errorf("Metadata cursor = %v, want %v", metadata.Cursor, expectedCursor)
				}

				// Verify timestamp fields
				if metadata.OldestTimestamp != result[0].Timestamp {
					t.Errorf("OldestTimestamp = %v, want %v", metadata.OldestTimestamp, result[0].Timestamp)
				}
				if metadata.NewestTimestamp != result[len(result)-1].Timestamp {
					t.Errorf("NewestTimestamp = %v, want %v", metadata.NewestTimestamp, result[len(result)-1].Timestamp)
				}
			}

			if metadata.HasMore != tt.expectedHasMore {
				t.Errorf("HasMore = %v, want %v", metadata.HasMore, tt.expectedHasMore)
			}
		})
	}
}

func TestApplyWebSocketCursorPagination_CursorExpired(t *testing.T) {
	// Buffer has sequences 101-200 (100 entries evicted)
	events := make([]WebSocketEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = WebSocketEvent{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://echo.example.com",
			Timestamp: "2026-01-30T10:20:00Z",
		}
	}
	enriched := EnrichWebSocketEntries(events, 200) // 200 total added, 100 evicted

	// Build a cursor for an evicted sequence (sequence 50, which is before sequence 101)
	expiredCursor := BuildCursor("2026-01-30T10:15:00Z", 50)

	tests := []struct {
		name                  string
		afterCursor           string
		restartOnEviction     bool
		expectError           bool
		expectedCount         int
		expectedFirstSeq      int64
		expectedCursorRestart bool
	}{
		{
			name:              "expired cursor without restart returns error",
			afterCursor:       expiredCursor,
			restartOnEviction: false,
			expectError:       true,
		},
		{
			name:                  "expired cursor with restart returns oldest available",
			afterCursor:           expiredCursor,
			restartOnEviction:     true,
			expectError:           false,
			expectedCount:         100, // All 100 available entries (no limit)
			expectedFirstSeq:      101, // Oldest available is sequence 101
			expectedCursorRestart: true,
		},
		{
			name:                  "expired cursor with restart and limit",
			afterCursor:           expiredCursor,
			restartOnEviction:     true,
			expectError:           false,
			expectedCount:         10, // Limit applied
			expectedFirstSeq:      101, // After restart, take FIRST 10 entries from oldest
			expectedCursorRestart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := 0
			if tt.name == "expired cursor with restart and limit" {
				limit = 10
			}

			result, metadata, err := ApplyWebSocketCursorPagination(enriched, tt.afterCursor, "", "", limit, tt.restartOnEviction)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Result count = %d, want %d", len(result), tt.expectedCount)
			}

			if tt.expectedCount > 0 {
				firstSeq := result[0].Sequence
				if firstSeq != tt.expectedFirstSeq {
					t.Errorf("First sequence = %d, want %d (oldest after restart)", firstSeq, tt.expectedFirstSeq)
				}

				// Verify cursor field is present
				expectedCursor := BuildCursor(result[len(result)-1].Timestamp, result[len(result)-1].Sequence)
				if metadata.Cursor != expectedCursor {
					t.Errorf("Metadata cursor = %v, want %v", metadata.Cursor, expectedCursor)
				}

				// Verify timestamp fields
				if metadata.OldestTimestamp != result[0].Timestamp {
					t.Errorf("OldestTimestamp = %v, want %v", metadata.OldestTimestamp, result[0].Timestamp)
				}
				if metadata.NewestTimestamp != result[len(result)-1].Timestamp {
					t.Errorf("NewestTimestamp = %v, want %v", metadata.NewestTimestamp, result[len(result)-1].Timestamp)
				}
			}

			if metadata.CursorRestarted != tt.expectedCursorRestart {
				t.Errorf("CursorRestarted = %v, want %v", metadata.CursorRestarted, tt.expectedCursorRestart)
			}

			if tt.expectedCursorRestart && metadata.Warning == "" {
				t.Errorf("Expected warning when cursor restarted, got empty string")
			}
		})
	}
}

func TestSerializeWebSocketEntryWithSequence(t *testing.T) {
	event := WebSocketEntryWithSequence{
		Entry: WebSocketEvent{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://echo.example.com",
			Direction: "incoming",
			Data:      `{"type":"ping"}`,
			Timestamp: "2026-01-30T10:15:23Z",
			TabId:     123,
		},
		Sequence:  5678,
		Timestamp: "2026-01-30T10:15:23Z",
	}

	result := SerializeWebSocketEntryWithSequence(event)

	// Verify required fields
	if result["event"] != "message" {
		t.Errorf("event = %v, want 'message'", result["event"])
	}

	if result["id"] != "ws-1" {
		t.Errorf("id = %v, want 'ws-1'", result["id"])
	}

	if result["url"] != "wss://echo.example.com" {
		t.Errorf("url = %v, want 'wss://echo.example.com'", result["url"])
	}

	if result["timestamp"] != "2026-01-30T10:15:23Z" {
		t.Errorf("timestamp = %v, want '2026-01-30T10:15:23Z'", result["timestamp"])
	}

	if result["sequence"] != int64(5678) {
		t.Errorf("sequence = %v, want 5678", result["sequence"])
	}

	if result["direction"] != "incoming" {
		t.Errorf("direction = %v, want 'incoming'", result["direction"])
	}

	// Verify tabId included
	if result["tab_id"] != 123 {
		t.Errorf("tab_id = %v, want 123", result["tab_id"])
	}
}
