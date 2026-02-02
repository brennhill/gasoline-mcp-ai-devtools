// pagination_test.go — Unit tests for cursor pagination helpers
package pagination

import (
	"fmt"
	"testing"
	"time"
)

func TestEnrichLogEntries(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		entries        []LogEntry
		logTotalAdded  int64
		wantFirstSeq   int64
		wantLastSeq    int64
	}{
		{
			name: "empty buffer",
			entries: []LogEntry{},
			logTotalAdded: 0,
			wantFirstSeq: 0,
			wantLastSeq: 0,
		},
		{
			name: "single entry",
			entries: []LogEntry{
				{"ts": baseTime.Format(time.RFC3339), "message": "Log 1"},
			},
			logTotalAdded: 1,
			wantFirstSeq: 1,
			wantLastSeq: 1,
		},
		{
			name: "multiple entries",
			entries: []LogEntry{
				{"ts": baseTime.Format(time.RFC3339), "message": "Log 1"},
				{"ts": baseTime.Add(1 * time.Second).Format(time.RFC3339), "message": "Log 2"},
				{"ts": baseTime.Add(2 * time.Second).Format(time.RFC3339), "message": "Log 3"},
			},
			logTotalAdded: 3,
			wantFirstSeq: 1,
			wantLastSeq: 3,
		},
		{
			name: "buffer with evictions (logTotalAdded > buffer length)",
			entries: []LogEntry{
				{"ts": baseTime.Format(time.RFC3339), "message": "Log 101"},
				{"ts": baseTime.Add(1 * time.Second).Format(time.RFC3339), "message": "Log 102"},
			},
			logTotalAdded: 102,
			wantFirstSeq: 101, // First 100 logs were evicted
			wantLastSeq: 102,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enriched := EnrichLogEntries(tt.entries, tt.logTotalAdded)

			if len(enriched) != len(tt.entries) {
				t.Errorf("EnrichLogEntries() returned %d entries, want %d", len(enriched), len(tt.entries))
				return
			}

			if len(enriched) == 0 {
				return // Empty case - no sequence to check
			}

			firstSeq := enriched[0].Sequence
			lastSeq := enriched[len(enriched)-1].Sequence

			if firstSeq != tt.wantFirstSeq {
				t.Errorf("First sequence = %d, want %d", firstSeq, tt.wantFirstSeq)
			}
			if lastSeq != tt.wantLastSeq {
				t.Errorf("Last sequence = %d, want %d", lastSeq, tt.wantLastSeq)
			}

			// Verify sequences are monotonically increasing
			for i := 1; i < len(enriched); i++ {
				if enriched[i].Sequence != enriched[i-1].Sequence+1 {
					t.Errorf("Non-monotonic sequence at index %d: %d -> %d", i, enriched[i-1].Sequence, enriched[i].Sequence)
				}
			}

			// Verify timestamps were extracted correctly
			for i, e := range enriched {
				expectedTs := entryStr(tt.entries[i], "ts")
				if e.Timestamp != expectedTs {
					t.Errorf("Entry %d timestamp = %v, want %v", i, e.Timestamp, expectedTs)
				}
			}
		})
	}
}

func TestApplyLogCursorPagination_NoСursor(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	entries := make([]LogEntryWithSequence, 100)
	for i := 0; i < 100; i++ {
		entries[i] = LogEntryWithSequence{
			Entry:     LogEntry{"ts": baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339), "message": fmt.Sprintf("Log %d", i)},
			Sequence:  int64(i + 1),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
		}
	}

	tests := []struct {
		name      string
		limit     int
		wantCount int
		wantFirst int64 // Expected first sequence
		wantLast  int64 // Expected last sequence
	}{
		{
			name:      "no limit returns all",
			limit:     0,
			wantCount: 100,
			wantFirst: 1,
			wantLast:  100,
		},
		{
			name:      "limit 50 returns last 50 (newest)",
			limit:     50,
			wantCount: 50,
			wantFirst: 51,
			wantLast:  100,
		},
		{
			name:      "limit exceeds buffer returns all",
			limit:     200,
			wantCount: 100,
			wantFirst: 1,
			wantLast:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := ApplyLogCursorPagination(entries, "", "", "", tt.limit, false)
			if err != nil {
				t.Fatalf("ApplyLogCursorPagination() unexpected error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("Result count = %d, want %d", len(result), tt.wantCount)
			}

			if metadata.Count != tt.wantCount {
				t.Errorf("Metadata count = %d, want %d", metadata.Count, tt.wantCount)
			}

			if metadata.Total != 100 {
				t.Errorf("Metadata total = %d, want 100", metadata.Total)
			}

			if len(result) > 0 {
				if result[0].Sequence != tt.wantFirst {
					t.Errorf("First sequence = %d, want %d", result[0].Sequence, tt.wantFirst)
				}
				if result[len(result)-1].Sequence != tt.wantLast {
					t.Errorf("Last sequence = %d, want %d", result[len(result)-1].Sequence, tt.wantLast)
				}

				// Verify cursor points to last entry
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

func TestApplyLogCursorPagination_AfterCursor(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	entries := make([]LogEntryWithSequence, 100)
	for i := 0; i < 100; i++ {
		entries[i] = LogEntryWithSequence{
			Entry:     LogEntry{"ts": baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339), "message": fmt.Sprintf("Log %d", i)},
			Sequence:  int64(i + 1),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
		}
	}

	tests := []struct {
		name        string
		afterCursor string
		limit       int
		wantCount   int
		wantFirst   int64
		wantLast    int64
	}{
		{
			name:        "after cursor gets older entries",
			afterCursor: BuildCursor(entries[50].Timestamp, entries[50].Sequence), // After entry 51
			limit:       0,
			wantCount:   50, // Entries 1-50 are older
			wantFirst:   1,
			wantLast:    50,
		},
		{
			name:        "after cursor with limit",
			afterCursor: BuildCursor(entries[50].Timestamp, entries[50].Sequence),
			limit:       25,
			wantCount:   25,
			wantFirst:   26, // Last 25 of the older entries
			wantLast:    50,
		},
		{
			name:        "after cursor at beginning returns empty",
			afterCursor: BuildCursor(entries[0].Timestamp, entries[0].Sequence),
			limit:       0,
			wantCount:   0,
		},
		{
			name:        "after cursor at end returns all but last",
			afterCursor: BuildCursor(entries[99].Timestamp, entries[99].Sequence),
			limit:       0,
			wantCount:   99,
			wantFirst:   1,
			wantLast:    99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := ApplyLogCursorPagination(entries, tt.afterCursor, "", "", tt.limit, false)
			if err != nil {
				t.Fatalf("ApplyLogCursorPagination() unexpected error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("Result count = %d, want %d", len(result), tt.wantCount)
			}

			if metadata.Count != tt.wantCount {
				t.Errorf("Metadata count = %d, want %d", metadata.Count, tt.wantCount)
			}

			if len(result) > 0 {
				if result[0].Sequence != tt.wantFirst {
					t.Errorf("First sequence = %d, want %d", result[0].Sequence, tt.wantFirst)
				}
				if result[len(result)-1].Sequence != tt.wantLast {
					t.Errorf("Last sequence = %d, want %d", result[len(result)-1].Sequence, tt.wantLast)
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
		})
	}
}

func TestApplyLogCursorPagination_BeforeCursor(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	entries := make([]LogEntryWithSequence, 100)
	for i := 0; i < 100; i++ {
		entries[i] = LogEntryWithSequence{
			Entry:     LogEntry{"ts": baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339), "message": fmt.Sprintf("Log %d", i)},
			Sequence:  int64(i + 1),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
		}
	}

	tests := []struct {
		name         string
		beforeCursor string
		limit        int
		wantCount    int
		wantFirst    int64
		wantLast     int64
	}{
		{
			name:         "before cursor gets newer entries",
			beforeCursor: BuildCursor(entries[50].Timestamp, entries[50].Sequence), // Before entry 51
			limit:        0,
			wantCount:    49, // Entries 52-100 are newer
			wantFirst:    52,
			wantLast:     100,
		},
		{
			name:         "before cursor with limit takes first N",
			beforeCursor: BuildCursor(entries[50].Timestamp, entries[50].Sequence),
			limit:        25,
			wantCount:    25,
			wantFirst:    52, // First 25 of the newer entries
			wantLast:     76,
		},
		{
			name:         "before cursor at end returns empty",
			beforeCursor: BuildCursor(entries[99].Timestamp, entries[99].Sequence),
			limit:        0,
			wantCount:    0,
		},
		{
			name:         "before cursor at beginning returns all but first",
			beforeCursor: BuildCursor(entries[0].Timestamp, entries[0].Sequence),
			limit:        0,
			wantCount:    99,
			wantFirst:    2,
			wantLast:     100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := ApplyLogCursorPagination(entries, "", tt.beforeCursor, "", tt.limit, false)
			if err != nil {
				t.Fatalf("ApplyLogCursorPagination() unexpected error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("Result count = %d, want %d", len(result), tt.wantCount)
			}

			if metadata.Count != tt.wantCount {
				t.Errorf("Metadata count = %d, want %d", metadata.Count, tt.wantCount)
			}

			if len(result) > 0 {
				if result[0].Sequence != tt.wantFirst {
					t.Errorf("First sequence = %d, want %d", result[0].Sequence, tt.wantFirst)
				}
				if result[len(result)-1].Sequence != tt.wantLast {
					t.Errorf("Last sequence = %d, want %d", result[len(result)-1].Sequence, tt.wantLast)
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
		})
	}
}

func TestApplyLogCursorPagination_CursorExpired(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	// Buffer has entries 101-200 (first 100 were evicted)
	entries := make([]LogEntryWithSequence, 100)
	for i := 0; i < 100; i++ {
		entries[i] = LogEntryWithSequence{
			Entry:     LogEntry{"ts": baseTime.Add(time.Duration(100+i) * time.Second).Format(time.RFC3339), "message": fmt.Sprintf("Log %d", 100+i)},
			Sequence:  int64(101 + i),
			Timestamp: baseTime.Add(time.Duration(100+i) * time.Second).Format(time.RFC3339),
		}
	}

	t.Run("expired cursor without restart returns error", func(t *testing.T) {
		// Cursor points to sequence 50 (which was evicted)
		expiredCursor := BuildCursor(baseTime.Add(50*time.Second).Format(time.RFC3339), 50)

		result, metadata, err := ApplyLogCursorPagination(entries, expiredCursor, "", "", 10, false)
		if err == nil {
			t.Fatal("ApplyLogCursorPagination() expected error for expired cursor, got nil")
		}

		// Should return nil results on error
		if result != nil {
			t.Errorf("Result should be nil on error, got %d entries", len(result))
		}
		if metadata != nil {
			t.Errorf("Metadata should be nil on error, got %+v", metadata)
		}

		wantErrSubstr := "cursor expired"
		if !contains(err.Error(), wantErrSubstr) {
			t.Errorf("Error = %v, want error containing %q", err, wantErrSubstr)
		}
	})

	t.Run("expired cursor with restart returns oldest available", func(t *testing.T) {
		// Cursor points to sequence 50 (which was evicted)
		expiredCursor := BuildCursor(baseTime.Add(50*time.Second).Format(time.RFC3339), 50)

		result, metadata, err := ApplyLogCursorPagination(entries, expiredCursor, "", "", 10, true)
		if err != nil {
			t.Fatalf("ApplyLogCursorPagination() unexpected error: %v", err)
		}

		// Should return limited entries from restart (limit=10)
		if len(result) != 10 {
			t.Errorf("Result count = %d, want 10 (limit applied after restart)", len(result))
		}

		// Should start from oldest available (sequence 101)
		if result[0].Sequence != 101 {
			t.Errorf("First sequence = %d, want 101 (oldest after restart)", result[0].Sequence)
		}

		if !metadata.CursorRestarted {
			t.Error("Metadata.CursorRestarted = false, want true")
		}

		if metadata.OriginalCursor != expiredCursor {
			t.Errorf("Metadata.OriginalCursor = %v, want %v", metadata.OriginalCursor, expiredCursor)
		}

		if metadata.Warning == "" {
			t.Error("Metadata.Warning is empty, want warning message")
		}

		// Verify cursor field is present
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
		}
	})
}

func TestSerializeLogEntryWithSequence(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 15, 23, 0, time.UTC)

	enriched := LogEntryWithSequence{
		Entry: LogEntry{
			"level":   "error",
			"message": "Test error message",
			"source":  "app.js:42",
			"ts":      baseTime.Format(time.RFC3339),
			"tabId":   float64(123),
		},
		Sequence:  5678,
		Timestamp: baseTime.Format(time.RFC3339),
	}

	result := SerializeLogEntryWithSequence(enriched)

	// Check required fields
	if result["level"] != "error" {
		t.Errorf("level = %v, want error", result["level"])
	}
	if result["message"] != "Test error message" {
		t.Errorf("message = %v, want 'Test error message'", result["message"])
	}
	if result["source"] != "app.js:42" {
		t.Errorf("source = %v, want app.js:42", result["source"])
	}
	if result["timestamp"] != baseTime.Format(time.RFC3339) {
		t.Errorf("timestamp = %v, want %v", result["timestamp"], baseTime.Format(time.RFC3339))
	}
	if result["sequence"] != int64(5678) {
		t.Errorf("sequence = %v, want 5678", result["sequence"])
	}
	if result["tab_id"] != "123" {
		t.Errorf("tab_id = %v, want 123", result["tab_id"])
	}
}
